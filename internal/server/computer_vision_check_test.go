package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestVisionEvidenceIdentityChangesWithCacheProcess(t *testing.T) {
	dir := t.TempDir()
	model, projector := filepath.Join(dir, "model.gguf"), filepath.Join(dir, "projector.gguf")
	for _, path := range []string{model, projector} {
		if err := os.WriteFile(path, []byte("fixture"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	modelInfo, err := os.Stat(model)
	if err != nil {
		t.Fatal(err)
	}
	projectorInfo, err := os.Stat(projector)
	if err != nil {
		t.Fatal(err)
	}
	first := visionEvidenceIdentity("fixture", model, projector, modelInfo, projectorInfo, 1)
	restarted := visionEvidenceIdentity("fixture", model, projector, modelInfo, projectorInfo, 2)
	if first == restarted || !strings.HasPrefix(first, "fixture\x00") {
		t.Fatal("vision smoke pass survived a different llama process")
	}
}

func TestVisionProbeBudgetStartsAfterInferenceAdmission(t *testing.T) {
	dir := t.TempDir()
	model := "qwen2.5vl-7b-test"
	projector := models.GetProjectorFilename(model, model+".gguf")
	if projector == "" {
		t.Fatal("fixture has no projector mapping")
	}
	for _, name := range []string{model + ".gguf", projector} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("fixture"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	registry := models.NewRegistry(dir)
	if err := registry.ScanModels(); err != nil {
		t.Fatal(err)
	}
	gate := inference.NewLifecycleGate(1)
	engine := &jobVisionEngine{}
	process := &llamaLifecycleFixture{generation: 1}
	s := &Server{config: &config.Config{MaxContextSize: 8192, ModelLoadTimeout: 2}, registry: registry, engine: engine, inferenceLifecycle: gate, llamaProcess: process}
	window := s.modelContextWindow(model)
	s.currentModelID, s.currentContext, s.currentPort, s.currentGeneration = model, window, 12345, 1
	release, err := gate.AcquireWithContext(context.Background(), model, window, func(context.Context, string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	go func() { time.Sleep(200 * time.Millisecond); release() }()
	result := s.checkComputerVisionWithBudget(context.Background(), model, 100*time.Millisecond)
	if !result.Passed || engine.visionCalls != 1 || gate.Status().Active != 0 {
		t.Fatalf("admission consumed perception budget or leaked lease: result=%+v calls=%d gate=%+v", result, engine.visionCalls, gate.Status())
	}
}

type visionProbeEngine struct {
	inference.Engine
	expected   string
	wrong      bool
	overBudget bool
	seen       bool
}

func (engine *visionProbeEngine) ChatCompletion(_ context.Context, request *api.ChatCompletionRequest) (*api.ChatCompletionResponse, error) {
	if len(request.Tools) != 1 || request.Tools[0].Function.Name != "browser_verify" || request.ParallelToolCalls == nil || *request.ParallelToolCalls || len(request.Messages) != 2 {
		return nil, errComputerVision
	}
	hasImage, err := api.ValidateChatMessages(request.Messages)
	if err != nil || !hasImage || strings.Contains(request.Messages[0].StringContent()+request.Messages[1].StringContent(), engine.expected) {
		return nil, errComputerVision
	}
	engine.seen = true
	value := engine.expected
	if engine.wrong {
		value = "INVENTED"
	}
	arguments, _ := json.Marshal(map[string]string{"text": value})
	response := &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "tool_calls", Message: api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{ID: "vision-probe", Type: "function", Function: api.FunctionCall{Name: "browser_verify", Arguments: string(arguments)}}}}}}}
	if engine.overBudget {
		response.Usage.CompletionTokens = 129
	}
	return response, nil
}

func TestComputerVisionProbeRequiresImageGroundedExactToolCall(t *testing.T) {
	const expected = "A10B22C30D4E"
	for _, wrong := range []bool{false, true} {
		engine := &visionProbeEngine{expected: expected, wrong: wrong}
		err := probeComputerVisionCode(context.Background(), engine, "fixture", expected)
		if engine.seen != true {
			t.Fatal("probe did not send a bounded typed image")
		}
		if (err == nil) == wrong {
			t.Fatalf("wrong=%v err=%v", wrong, err)
		}
	}
}

func TestComputerVisionProbeRejectsReportedOutputOveruse(t *testing.T) {
	engine := &visionProbeEngine{expected: "A10B22C30D4E", overBudget: true}
	if err := probeComputerVisionCode(context.Background(), engine, "fixture", engine.expected); !errors.Is(err, errProbeOutputBudget) {
		t.Fatalf("over-budget vision response was accepted: %v", err)
	}
}

func TestVisionProbeImageIsDeterministicBoundedPNG(t *testing.T) {
	first, err := visionProbeImage("A10B22C30D4E")
	if err != nil {
		t.Fatal(err)
	}
	second, err := visionProbeImage("A10B22C30D4E")
	if err != nil || first != second || !strings.HasPrefix(first, "data:image/png;base64,") {
		t.Fatal("vision fixture was not deterministic")
	}
	content, err := base64.StdEncoding.Strict().DecodeString(strings.TrimPrefix(first, "data:image/png;base64,"))
	if err != nil {
		t.Fatal(err)
	}
	configuration, err := png.DecodeConfig(strings.NewReader(string(content)))
	if err != nil || configuration.Width > 1024 || configuration.Height > 256 || configuration.Width < 1 || configuration.Height < 1 {
		t.Fatalf("invalid bounded fixture: %+v %v", configuration, err)
	}
	if _, err := visionProbeImage("NOT-HEX"); err == nil {
		t.Fatal("unsupported fixture glyph was accepted")
	}
}
