package audio

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// One owned worker holds at most one model; changing kind/model or cancelling
// terminates and reaps it before another request can run.
type managedWorker struct {
	cfg    RuntimeConfig
	gate   chan struct{}
	proc   *workerProcess
	closed bool
}
type workerProcess struct {
	cmd              *exec.Cmd
	stdin            io.WriteCloser
	reader           *bufio.Reader
	dir, kind, model string
}

func newManagedWorker(cfg RuntimeConfig) *managedWorker {
	return &managedWorker{cfg: cfg, gate: make(chan struct{}, 1)}
}

// SupportsQwenDirectSpeech inspects data only; Base is not a direct voice model.
func SupportsQwenDirectSpeech(modelDir string) bool {
	b, err := os.ReadFile(filepath.Join(modelDir, "config.json"))
	if err != nil {
		return false
	}
	var cfg struct {
		ModelType string `json:"tts_model_type"`
	}
	return json.Unmarshal(b, &cfg) == nil && strings.EqualFold(cfg.ModelType, "custom_voice")
}
func (w *managedWorker) available(kind, modelDir string) bool {
	if strings.TrimSpace(modelDir) == "" || w.cfg.PythonPath == "" {
		return false
	}
	info, err := os.Stat(modelDir)
	if err != nil || !info.IsDir() {
		return false
	}
	if _, err := exec.LookPath(w.cfg.PythonPath); err != nil {
		return false
	}
	return kind != "tts" || SupportsQwenDirectSpeech(modelDir)
}

type workerRequest struct {
	Op, Kind, ModelDir, InputPath, Filename, Language, Prompt, ResponseFormat, Voice string
	Text                                                                             string
	Speed                                                                            float64
}
type workerResult struct {
	OK         bool      `json:"ok"`
	Text       string    `json:"text,omitempty"`
	Language   string    `json:"language,omitempty"`
	Duration   float64   `json:"duration,omitempty"`
	Segments   []Segment `json:"segments,omitempty"`
	Error      string    `json:"error,omitempty"`
	OutputPath string    `json:"output_path,omitempty"`
}

func (w *managedWorker) stop() {
	if w.proc == nil {
		return
	}
	p := w.proc
	w.proc = nil
	_ = p.stdin.Close()
	_ = p.cmd.Process.Kill()
	_ = p.cmd.Wait()
	// Only the private directory made by start, never a caller-supplied path.
	_ = os.RemoveAll(p.dir)
}
func (w *managedWorker) close() {
	w.gate <- struct{}{}
	defer func() { <-w.gate }()
	w.closed = true
	w.stop()
}
func (w *managedWorker) start(req workerRequest) error {
	dir, err := os.MkdirTemp("", "offgrid-speech-worker-*")
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		if !ok {
			_ = os.RemoveAll(dir)
		}
	}()
	script := filepath.Join(dir, "worker.py")
	if err := os.WriteFile(script, qwenWorkerScript, 0600); err != nil {
		return err
	}
	cmd := exec.Command(w.cfg.PythonPath, "-u", script)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return err
	}
	// Third-party diagnostics may contain private text. Return safe protocol
	// errors instead of copying inference output into service logs.
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return err
	}
	w.proc = &workerProcess{cmd: cmd, stdin: stdin, reader: bufio.NewReader(stdout), dir: dir, kind: req.Kind, model: req.ModelDir}
	ok = true
	return nil
}
func (w *managedWorker) call(ctx context.Context, req workerRequest) (workerResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	select {
	case w.gate <- struct{}{}:
	case <-ctx.Done():
		return workerResult{}, ctx.Err()
	}
	defer func() { <-w.gate }()
	if err := ctx.Err(); err != nil {
		return workerResult{}, err
	}
	if w.closed {
		return workerResult{}, fmt.Errorf("speech runtime was closed; retry the request")
	}
	if w.proc != nil && (w.proc.kind != req.Kind || w.proc.model != req.ModelDir) {
		w.stop()
	}
	if w.proc == nil {
		if err := w.start(req); err != nil {
			return workerResult{}, err
		}
	}
	b, err := json.Marshal(req)
	if err != nil {
		return workerResult{}, err
	}
	p := w.proc
	type response struct {
		out workerResult
		err error
	}
	done := make(chan response, 1)
	go func() {
		if _, err := p.stdin.Write(append(b, '\n')); err != nil {
			done <- response{err: err}
			return
		}
		scanner := bufio.NewScanner(p.reader)
		scanner.Buffer(make([]byte, 4096), 1<<20)
		if !scanner.Scan() {
			err := scanner.Err()
			if err == nil {
				err = io.ErrUnexpectedEOF
			}
			done <- response{err: err}
			return
		}
		var out workerResult
		err := json.Unmarshal(scanner.Bytes(), &out)
		done <- response{out: out, err: err}
	}()
	select {
	case <-ctx.Done():
		w.stop()
		<-done
		return workerResult{}, ctx.Err()
	case result := <-done:
		if result.err != nil {
			w.stop()
			return workerResult{}, fmt.Errorf("speech worker stopped or returned an invalid response; retry or repair the speech runtime: %w", result.err)
		}
		if !result.out.OK {
			message := strings.TrimSpace(result.out.Error)
			if message == "" {
				message = "speech runtime failed; repair the runtime or select a compatible model"
			}
			return result.out, fmt.Errorf("speech worker: %s", message)
		}
		return result.out, nil
	}
}
func (w *managedWorker) transcribe(ctx context.Context, req TranscriptionRequest, modelDir string) (*TranscriptionResponse, error) {
	suffix := filepath.Ext(req.Filename)
	if suffix == "" {
		suffix = ".wav"
	}
	tmp, err := os.CreateTemp("", "offgrid-audio-*"+suffix)
	if err != nil {
		return nil, err
	}
	path := tmp.Name()
	defer os.Remove(path)
	if _, err = io.Copy(tmp, req.File); err != nil {
		tmp.Close()
		return nil, err
	}
	if err = tmp.Close(); err != nil {
		return nil, err
	}
	out, err := w.call(ctx, workerRequest{Op: "transcribe", Kind: "asr", ModelDir: modelDir, InputPath: path, Filename: req.Filename, Language: req.Language, Prompt: req.Prompt, ResponseFormat: req.ResponseFormat})
	if err != nil {
		return nil, err
	}
	return &TranscriptionResponse{Text: out.Text, Language: out.Language, Duration: out.Duration, Segments: out.Segments}, nil
}
func (w *managedWorker) speak(ctx context.Context, req SpeechRequest, modelDir string) (io.Reader, error) {
	if !SupportsQwenDirectSpeech(modelDir) {
		return nil, fmt.Errorf("Qwen response playback requires a CustomVoice model; Base requires reference audio and is not supported for read-aloud")
	}
	if req.ResponseFormat != "" && req.ResponseFormat != "wav" {
		return nil, fmt.Errorf("this speech runtime supports WAV output only")
	}
	if req.Speed != 0 && req.Speed != 1 {
		return nil, fmt.Errorf("this speech runtime supports playback speed 1 only")
	}
	if len(strings.TrimSpace(req.Input)) == 0 || len(req.Input) > 12000 {
		return nil, fmt.Errorf("speech input must contain 1 to 12000 bytes of text")
	}
	tmp, err := os.CreateTemp("", "offgrid-speech-*.wav")
	if err != nil {
		return nil, err
	}
	path := tmp.Name()
	tmp.Close()
	defer os.Remove(path)
	out, err := w.call(ctx, workerRequest{Op: "synthesize", Kind: "tts", ModelDir: modelDir, InputPath: path, Text: req.Input, Voice: req.Voice, ResponseFormat: "wav", Speed: req.Speed})
	if err != nil {
		return nil, err
	}
	if out.OutputPath != path {
		return nil, fmt.Errorf("speech worker returned an unexpected output path")
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > 64<<20 {
		return nil, fmt.Errorf("speech output exceeds the size limit")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(b) <= 44 || string(b[:4]) != "RIFF" || string(b[8:12]) != "WAVE" {
		return nil, fmt.Errorf("speech runtime did not produce WAV audio")
	}
	return bytes.NewReader(b), nil
}
