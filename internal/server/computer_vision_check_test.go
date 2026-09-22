package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"image/png"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type visionProbeEngine struct {
	inference.Engine
	expected string
	wrong    bool
	seen     bool
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
	return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "tool_calls", Message: api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{ID: "vision-probe", Type: "function", Function: api.FunctionCall{Name: "browser_verify", Arguments: string(arguments)}}}}}}}, nil
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
