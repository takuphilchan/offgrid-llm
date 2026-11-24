//go:build windows || linux

package inference

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/takuphilchan/offgrid-llm/internal/jsonwire"
)

// Platform openers retain artifact identities: Windows denies writes/deletion;
// restricted Linux requires administrator-owned immutable-to-service paths.
// Directories are preprovisioned; their permissions are never changed here.
type managedOllamaFiles struct{ files []*os.File }

// Failed retirement retains OS file leases even if startup never returned a
// provider to the scheduler. They are released by service exit, not finalizers.
var uncertainManagedOllamaLeases = struct {
	sync.Mutex
	leases map[*managedOllamaFiles]struct{}
}{leases: map[*managedOllamaFiles]struct{}{}}

func (lease *managedOllamaFiles) retainUncertain() {
	uncertainManagedOllamaLeases.Lock()
	uncertainManagedOllamaLeases.leases[lease] = struct{}{}
	uncertainManagedOllamaLeases.Unlock()
}

func (lease *managedOllamaFiles) close() {
	for i := len(lease.files) - 1; i >= 0; i-- {
		_ = lease.files[i].Close()
	}
	lease.files = nil
}

func managedHash(ctx context.Context, file *os.File, digest string, limit int64) error {
	info, err := file.Stat()
	if err != nil || info.Size() < 0 || info.Size() > limit || !completeSHA256Digest(digest) {
		return errors.New("managed Ollama file exceeds its pin bounds")
	}
	h := sha256.New()
	buffer := make([]byte, 256<<10)
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, err := file.Read(buffer)
		total += int64(n)
		if total > limit {
			return errors.New("managed Ollama file exceeds byte limit")
		}
		_, _ = h.Write(buffer[:n])
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
	}
	if total != info.Size() || "sha256:"+hex.EncodeToString(h.Sum(nil)) != digest {
		return errors.New("managed Ollama file digest changed")
	}
	_, err = file.Seek(0, io.SeekStart)
	return err
}

// Walk has a bounded entry count and rejects every unpinned regular file. The
// directory handles prevent replacement of the paths retained for the runtime.
func (lease *managedOllamaFiles) tree(ctx context.Context, root string, pins map[string]string, limit int64) error {
	count, found := 0, 0
	var walk func(string, bool, int) error
	walk = func(path string, directory bool, depth int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		count++
		if count > 1024 || depth > 32 {
			return errors.New("managed Ollama file inventory is oversized")
		}
		file, err := lease.open(path, directory)
		if err != nil {
			return err
		}
		if directory {
			// ReadDir(-1)/WalkDir would allocate all attacker-controlled entries
			// before the visitor's count check. Read only the remaining budget.
			entries, err := file.ReadDir(1025 - count)
			if err != nil && !errors.Is(err, io.EOF) {
				return err
			}
			if len(entries) > 1024-count {
				return errors.New("managed Ollama file inventory is oversized")
			}
			for _, entry := range entries {
				if err := walk(filepath.Join(path, entry.Name()), entry.IsDir(), depth+1); err != nil {
					return err
				}
			}
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		digest := pins[filepath.ToSlash(relative)]
		if digest == "" {
			return errors.New("managed Ollama directory contains an unpinned file")
		}
		if err := managedHash(ctx, file, digest, limit); err != nil {
			return err
		}
		found++
		return nil
	}
	if err := walk(root, true, 0); err != nil {
		return err
	}
	if found != len(pins) {
		return errors.New("managed Ollama pinned files are missing")
	}
	return nil
}

type managedOllamaLayer struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
	From      string `json:"from,omitempty"`
	Name      string `json:"name,omitempty"`
}

func (lease *managedOllamaFiles) model(ctx context.Context, endpoint HTTPEndpoint) error {
	c := endpoint.ManagedOllama
	name, tag, _ := strings.Cut(endpoint.Model, ":")
	manifestPath := "manifests/registry.ollama.ai/library/" + name + "/" + tag
	for _, directory := range []string{".", "manifests", "manifests/registry.ollama.ai", "manifests/registry.ollama.ai/library", "manifests/registry.ollama.ai/library/" + name, "blobs"} {
		if _, err := lease.open(filepath.Join(c.ModelDirectory, filepath.FromSlash(directory)), true); err != nil {
			return err
		}
	}
	manifest, err := lease.open(filepath.Join(c.ModelDirectory, filepath.FromSlash(manifestPath)), false)
	if err != nil {
		return err
	}
	if err := managedHash(ctx, manifest, endpoint.Descriptor.ModelDigest, 2<<20); err != nil {
		return err
	}
	raw, err := io.ReadAll(io.LimitReader(manifest, (2<<20)+1))
	if err != nil || len(raw) > 2<<20 || jsonwire.Validate(raw) != nil {
		return errors.New("managed Ollama manifest is malformed")
	}
	var model struct {
		SchemaVersion int                  `json:"schemaVersion"`
		MediaType     string               `json:"mediaType"`
		Config        managedOllamaLayer   `json:"config"`
		Layers        []managedOllamaLayer `json:"layers"`
		Runner        string               `json:"runner,omitempty"`
		Format        string               `json:"format,omitempty"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&model) != nil || model.SchemaVersion != 2 || model.MediaType != "application/vnd.docker.distribution.manifest.v2+json" || len(model.Layers) == 0 || len(model.Layers) > 256 || model.Config.MediaType != "application/vnd.docker.container.image.v1+json" || model.Format != "" && model.Format != "gguf" || model.Runner != "" && model.Runner != "llamacpp" && model.Runner != "ggml" {
		return errors.New("managed Ollama requires a legacy GGUF manifest; manifest lists and newer layouts are unsupported")
	}
	pins := map[string]string{manifestPath: endpoint.Descriptor.ModelDigest}
	var total int64
	hasModel := false
	for i, layer := range append([]managedOllamaLayer{model.Config}, model.Layers...) {
		if !completeSHA256Digest(layer.Digest) || layer.Size < 0 || layer.Size > 512<<30 || layer.Name != "" {
			return errors.New("managed Ollama layer is unsupported")
		}
		if i > 0 {
			switch layer.MediaType {
			case "application/vnd.ollama.image.model":
				hasModel = true
			case "application/vnd.ollama.image.template", "application/vnd.ollama.image.system", "application/vnd.ollama.image.license", "application/vnd.ollama.image.params", "application/vnd.ollama.image.messages":
			default:
				return errors.New("managed Ollama model layer is not text GGUF")
			}
		}
		total += layer.Size
		if total > 1<<40 {
			return errors.New("managed Ollama model inventory exceeds byte limit")
		}
		path := "blobs/" + strings.Replace(layer.Digest, ":", "-", 1)
		pins[path] = layer.Digest
		file, err := lease.open(filepath.Join(c.ModelDirectory, filepath.FromSlash(path)), false)
		if err != nil {
			return err
		}
		info, err := file.Stat()
		if err != nil || info.Size() != layer.Size {
			return errors.New("managed Ollama layer size changed")
		}
		if i == 0 {
			if info.Size() > 2<<20 {
				return errors.New("managed Ollama configuration is oversized")
			}
			data, err := io.ReadAll(io.LimitReader(file, (2<<20)+1))
			var config struct {
				Format      string `json:"model_format"`
				RemoteHost  string `json:"remote_host"`
				RemoteModel string `json:"remote_model"`
			}
			if err != nil || jsonwire.Validate(data) != nil || json.Unmarshal(data, &config) != nil || config.Format != "gguf" || config.RemoteHost != "" || config.RemoteModel != "" {
				return errors.New("managed Ollama configuration is not local GGUF")
			}
		}
	}
	if !hasModel {
		return errors.New("managed Ollama model layer is missing")
	}
	return lease.tree(ctx, c.ModelDirectory, pins, 512<<30)
}
