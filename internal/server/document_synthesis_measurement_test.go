package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
)

func TestDocumentSynthesisMeasuresOwnedArtifacts(t *testing.T) {
	dir := t.TempDir()
	modelPath, runtimePath := filepath.Join(dir, "model.gguf"), filepath.Join(dir, "runtime")
	if err := os.WriteFile(modelPath, []byte("isolated model"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(runtimePath, []byte("isolated runtime"), 0600); err != nil {
		t.Fatal(err)
	}
	owner := &observedAgentRuntime{observation: inference.ToolRuntimeObservation{ModelID: "model", Generation: 1, ModelPath: modelPath, ExecutablePath: runtimePath, ConfigSHA256: strings.Repeat("b", 64), Info: inference.ToolRuntimeInfo{Build: "fixture", Context: 8192, TemplateSHA256: strings.Repeat("c", 64)}}}
	p := &localDocumentSynthesis{server: &Server{llamaProcess: owner}}
	i := document.SynthesisModelIdentity{ModelID: "model", Language: "en", ContextTokens: 8192, ResponseTokens: 2048, Protocol: document.SynthesisProtocol}
	first, err := p.measure(context.Background(), i)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256([]byte("isolated model"))
	if first.ModelSHA256 != hex.EncodeToString(hash[:]) || first.Runtime.SHA256 == "" {
		t.Fatal("unmeasured identity", first)
	}
	if err = os.WriteFile(modelPath, []byte("replacement model"), 0600); err != nil {
		t.Fatal(err)
	}
	changed, err := p.measure(context.Background(), i)
	if err != nil || changed.ModelSHA256 == first.ModelSHA256 {
		t.Fatal("model replacement not detected", err)
	}
	if err = os.WriteFile(runtimePath, []byte("replacement runtime"), 0600); err != nil {
		t.Fatal(err)
	}
	changed, err = p.measure(context.Background(), i)
	if err != nil || changed.Runtime.SHA256 == first.Runtime.SHA256 {
		t.Fatal("runtime replacement not detected", err)
	}
	owner.observation.Generation = 0
	if _, err = p.measure(context.Background(), i); err == nil {
		t.Fatal("unowned process admitted")
	}
	owner.observation.Generation = 1
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = p.measure(ctx, i); err == nil {
		t.Fatal("cancelled measurement accepted")
	}
}

func TestDocumentSynthesisRequestConstrainsCountAndGeneration(t *testing.T) {
	s := &localDocumentSynthesisSession{identity: document.SynthesisModelIdentity{ModelID: "local", ResponseTokens: 1024}}
	prompt := document.SynthesisPrompt{System: "instruction", User: "untrusted evidence", Schema: map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{"text": map[string]any{"type": "string"}}, "required": []string{"text"}}}
	r := s.request(prompt)
	if r.ResponseFormat == nil || !r.ResponseFormat.Constrained() || r.ResponseFormat.Validate() != nil || r.ResponseFormat.JSONSchema.Strict == nil || !*r.ResponseFormat.JSONSchema.Strict || len(r.Tools) != 0 || r.ToolChoice != "none" || r.MaxTokens == nil || *r.MaxTokens != 1024 || r.Messages[1].Content != prompt.User {
		t.Fatal("document request weakened", r)
	}
}
