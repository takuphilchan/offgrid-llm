package models

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/jsonwire"
)

const ONNXEmbeddingAdapterRevision = "bert-symmetric-v1"
const ONNXEmbeddingPrefix = "embedding:"

// A selector routes to installed package metadata; it never establishes model
// integrity, provider readiness, or permission by itself.
func ONNXEmbeddingSelector(id, revision string) string {
	return ONNXEmbeddingPrefix + id + "@" + revision
}

func ParseONNXEmbeddingSelector(selector string) (string, string, error) {
	if !strings.HasPrefix(selector, ONNXEmbeddingPrefix) {
		return "", "", errors.New("not an embedding package selector")
	}
	id, revision, ok := strings.Cut(strings.TrimPrefix(selector, ONNXEmbeddingPrefix), "@")
	if !ok || !validPackageComponent(id) || !validPackageComponent(revision) || strings.Contains(revision, "@") {
		return "", "", errors.New("invalid embedding package identity")
	}
	return id, revision, nil
}

func validateONNXManifest(m ModelPackageManifest) error {
	if m.Engine != EngineONNX || m.Format != FormatONNX || m.Backend != "cpu" || m.ModelFamily != "bert-symmetric" || m.AdapterRevision != ONNXEmbeddingAdapterRevision || m.Runtime.Revision != ONNXEmbeddingAdapterRevision || m.ContextLength < 1 || m.ContextLength > 8192 || m.TemplateDigest != "" || len(m.Voices) != 0 || len(m.Preprocessing) != 0 || m.ToolCalling != "unsupported" || !slices.Equal(m.Modalities, []string{"text"}) {
		return errors.New("ONNX embeddings require the reviewed CPU BERT adapter, text-only metadata and bounded context")
	}
	required := map[string]struct {
		role  string
		limit int64
	}{
		"model.onnx": {"weights", 512 << 20}, "tokenizer.json": {"tokenizer", 16 << 20}, "embedding.json": {"config", 64 << 10},
	}
	if len(m.Artifacts) != len(required) {
		return errors.New("ONNX embeddings require exactly graph, tokenizer and settings artifacts")
	}
	for _, artifact := range m.Artifacts {
		expected, ok := required[artifact.Path]
		if !ok || artifact.Role != expected.role || artifact.Size > expected.limit {
			return errors.New("ONNX embedding artifact layout exceeds the reviewed adapter")
		}
	}
	return nil
}

type ONNXEmbeddingSettings struct {
	SchemaVersion int      `json:"schema_version"`
	Family        string   `json:"family"`
	Inputs        []string `json:"inputs"`
	Output        string   `json:"output"`
	Pooling       string   `json:"pooling"`
	NormalizeL2   bool     `json:"normalize_l2"`
	Dimensions    int      `json:"dimensions"`
	MaxTokens     int      `json:"max_tokens"`
	PadToken      string   `json:"pad_token"`
	PadTokenID    uint32   `json:"pad_token_id"`
}

var onnxOutputName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_./-]{0,127}$`)

func DecodeONNXEmbeddingSettings(raw []byte) (ONNXEmbeddingSettings, error) {
	var settings ONNXEmbeddingSettings
	if len(raw) > 64<<10 || jsonwire.Validate(raw) != nil {
		return settings, errors.New("invalid ONNX embedding settings JSON")
	}
	// Every field is required, including false and zero-valued policy fields.
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || len(fields) != 10 {
		return settings, errors.New("ONNX embedding settings require all reviewed fields")
	}
	for _, name := range []string{"schema_version", "family", "inputs", "output", "pooling", "normalize_l2", "dimensions", "max_tokens", "pad_token", "pad_token_id"} {
		if len(fields[name]) == 0 || bytes.Equal(fields[name], []byte("null")) {
			return settings, fmt.Errorf("ONNX embedding settings require %s", name)
		}
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&settings); err != nil {
		return settings, err
	}
	if settings.SchemaVersion != 1 || settings.Family != "bert-symmetric" || (!slices.Equal(settings.Inputs, []string{"input_ids", "attention_mask"}) && !slices.Equal(settings.Inputs, []string{"input_ids", "attention_mask", "token_type_ids"})) || !onnxOutputName.MatchString(settings.Output) || !slices.Contains([]string{"mean", "cls", "last", "none"}, settings.Pooling) || settings.Dimensions < 1 || settings.Dimensions > 4096 || settings.MaxTokens < 1 || settings.MaxTokens > 8192 || settings.PadToken == "" || len(settings.PadToken) > 128 {
		return settings, errors.New("ONNX embedding settings exceed the reviewed adapter")
	}
	return settings, nil
}

type ONNXEmbeddingLease struct {
	Manifest    ModelPackageManifest
	ModelDir    string
	ModelDigest string
	release     func()
}

func (l *ONNXEmbeddingLease) Release() {
	if l != nil && l.release != nil {
		l.release()
	}
}

func (s *PackageStore) AcquireONNXEmbedding(ctx context.Context, id, revision, consumer string) (*ONNXEmbeddingLease, error) {
	state, release, err := s.Acquire(ctx, id, revision, consumer)
	if err != nil {
		return nil, err
	}
	if state.Manifest == nil || state.Manifest.Architecture != "onnx-embedding" {
		release()
		return nil, errors.New("package is not an ONNX embedding model")
	}
	manifest := *state.Manifest
	if err := manifest.Validate(); err != nil {
		release()
		return nil, err
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		release()
		return nil, err
	}
	digest := sha256.Sum256(encoded)
	base, err := filepath.Abs(s.modelsDir)
	if err != nil {
		release()
		return nil, err
	}
	return &ONNXEmbeddingLease{Manifest: manifest, ModelDir: filepath.Join(base, PackageDirectory, id, revision), ModelDigest: "sha256:" + hex.EncodeToString(digest[:]), release: release}, nil
}
