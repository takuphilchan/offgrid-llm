//go:build !llama

package inference

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// httpEmbedding owns only the child it started. Lifecycle and requests are
// serialized by EmbeddingEngine; request cancellation does not kill the worker.
type httpEmbedding struct {
	binDir     string
	url        string
	key        string
	client     *http.Client
	cmd        *exec.Cmd
	done       chan struct{}
	exitErr    error
	dimensions int
	usage      int
	normalize  bool
	identity   string
}

func newEmbeddingImpl(binDir string) (EmbeddingImpl, error) {
	return &httpEmbedding{binDir: binDir, client: &http.Client{
		Timeout:   2 * time.Minute,
		Transport: &http.Transport{Proxy: nil},
	}}, nil
}

func embeddingFileDigest(ctx context.Context, path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return "", fmt.Errorf("embedding file must be a nonempty regular file")
	}
	h := sha256.New()
	buf := make([]byte, 1024*1024)
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		n, err := f.Read(buf)
		_, _ = h.Write(buf[:n])
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (e *httpEmbedding) Load(ctx context.Context, modelPath string, opts EmbeddingOptions) error {
	modelDigest, err := embeddingFileDigest(ctx, modelPath)
	if err != nil {
		return fmt.Errorf("read embedding model: %w", err)
	}
	if opts.ContextSize <= 0 || opts.NumThreads < 0 || opts.NumGPULayers < 0 {
		return fmt.Errorf("invalid embedding runtime options")
	}
	switch opts.PoolingMethod {
	case "", "mean", "cls", "last":
	default:
		return fmt.Errorf("unsupported embedding pooling %q", opts.PoolingMethod)
	}
	binDir := e.binDir
	if opts.BinDir != "" {
		binDir = opts.BinDir
	}
	if binDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		binDir = filepath.Join(home, ".offgrid-llm", "bin")
	}
	binary := opts.RuntimePath
	if binary == "" {
		binary, err = NewBinaryManager(binDir).GetLlamaServer()
		if err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	runtimeDigest, err := embeddingFileDigest(ctx, binary)
	if err != nil {
		return err
	}
	// Reserve an available loopback port; never evict another process. A bind
	// race fails startup rather than killing whatever acquired the port.
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	var key [32]byte
	if _, err := rand.Read(key[:]); err != nil {
		return err
	}
	e.key = hex.EncodeToString(key[:])
	e.url = "http://127.0.0.1:" + strconv.Itoa(port)
	e.normalize = opts.NormalizeL2
	args := []string{"-m", modelPath, "--host", "127.0.0.1", "--port", strconv.Itoa(port),
		"--embedding", "--api-key", e.key, "-c", strconv.Itoa(opts.ContextSize),
		"-b", strconv.Itoa(opts.ContextSize), "-ub", strconv.Itoa(opts.ContextSize),
		"-np", "1", "-ngl", strconv.Itoa(opts.NumGPULayers)}
	if opts.NumThreads > 0 {
		args = append(args, "-t", strconv.Itoa(opts.NumThreads))
	}
	if opts.PoolingMethod != "" {
		args = append(args, "--pooling", opts.PoolingMethod)
	}
	if !opts.UseMmap {
		args = append(args, "--no-mmap")
	}
	if opts.UseMlock {
		args = append(args, "--mlock")
	}
	e.cmd = exec.Command(binary, args...)
	configureBackgroundProcess(e.cmd)
	// Do not copy raw runtime output (which can contain source text) into logs.
	e.cmd.Stdout, e.cmd.Stderr = io.Discard, io.Discard
	if err := startOwnedProcess(e.cmd); err != nil {
		e.cmd = nil
		return fmt.Errorf("start embedding runtime: %w", err)
	}
	e.done = make(chan struct{})
	go func() { e.exitErr = e.cmd.Wait(); close(e.done) }()
	readyCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-readyCtx.Done():
			return readyCtx.Err()
		case <-e.done:
			return fmt.Errorf("embedding runtime exited during startup: %v", e.exitErr)
		case <-ticker.C:
			probeCtx, done := context.WithTimeout(readyCtx, time.Second)
			req, _ := http.NewRequestWithContext(probeCtx, http.MethodGet, e.url+"/health", nil)
			req.Header.Set("Authorization", "Bearer "+e.key)
			resp, err := e.client.Do(req)
			ready := err == nil && resp.StatusCode == http.StatusOK
			if resp != nil {
				_ = resp.Body.Close()
			}
			done()
			if !ready {
				continue
			}
			vectors, err := e.Embed(readyCtx, []string{"OffGrid embedding readiness check"})
			if err != nil {
				return fmt.Errorf("embedding readiness check: %w", err)
			}
			e.dimensions = len(vectors[0])
			e.identity = fmt.Sprintf("llama-embeddings-v1:%s:%s:%s:%t:%d", modelDigest, runtimeDigest, opts.PoolingMethod, opts.NormalizeL2, e.dimensions)
			return nil
		}
	}
}

func (e *httpEmbedding) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if e.done != nil {
		select {
		case <-e.done:
			return nil, fmt.Errorf("embedding runtime stopped; reload the model")
		default:
		}
	}
	body, err := json.Marshal(map[string]any{"input": texts, "encoding_format": "float"})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.url+"/v1/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.key)
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding runtime request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedding runtime returned HTTP %d; check model, pooling, and input context limit", resp.StatusCode)
	}
	var result api.EmbeddingResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 32<<20)).Decode(&result); err != nil {
		return nil, fmt.Errorf("invalid embedding runtime response: %w", err)
	}
	if len(result.Data) != len(texts) || len(texts) == 0 {
		return nil, fmt.Errorf("embedding response count mismatch")
	}
	vectors := make([][]float32, len(texts))
	dimensions := e.dimensions
	for _, item := range result.Data {
		if item.Index < 0 || item.Index >= len(texts) || vectors[item.Index] != nil {
			return nil, fmt.Errorf("invalid embedding response index")
		}
		if dimensions == 0 {
			dimensions = len(item.Embedding)
		}
		if dimensions == 0 || len(item.Embedding) != dimensions {
			return nil, fmt.Errorf("embedding dimension mismatch")
		}
		var norm float64
		for _, v := range item.Embedding {
			if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
				return nil, fmt.Errorf("non-finite embedding")
			}
			norm += float64(v) * float64(v)
		}
		if norm == 0 {
			return nil, fmt.Errorf("zero embedding vector")
		}
		if e.normalize {
			scale := math.Sqrt(norm)
			for i := range item.Embedding {
				item.Embedding[i] = float32(float64(item.Embedding[i]) / scale)
			}
		}
		vectors[item.Index] = item.Embedding
	}
	e.usage = result.Usage.PromptTokens
	return vectors, nil
}

func (e *httpEmbedding) GetDimensions() int { return e.dimensions }
func (e *httpEmbedding) UsageTokens() int   { return e.usage }
func (e *httpEmbedding) Identity() string   { return e.identity }
func (e *httpEmbedding) Unload() error {
	if e.cmd != nil && e.cmd.Process != nil {
		select {
		case <-e.done:
		default:
			_ = e.cmd.Process.Kill()
			<-e.done
		}
	}
	e.cmd = nil
	e.client.CloseIdleConnections()
	e.dimensions = 0
	return nil
}
