package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type computerVisionCheck struct {
	Installed bool   `json:"installed"`
	Passed    bool   `json:"passed"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

var errComputerVision = errors.New("model did not pass the local vision check")

// visionCheckKey binds an in-memory pass to the exact model/projector files.
// A service restart, replacement, or modification invalidates it. This is a
// runtime smoke result, not a persisted qualification claim.
func (s *Server) visionCheckKey(model string) (string, bool) {
	if s.registry == nil {
		return "", false
	}
	metadata, err := s.registry.GetModel(model)
	if err != nil || metadata.ProjectorPath == "" {
		return "", false
	}
	modelInfo, modelErr := os.Stat(metadata.Path)
	projectorInfo, projectorErr := os.Stat(metadata.ProjectorPath)
	if modelErr != nil || projectorErr != nil || !modelInfo.Mode().IsRegular() || !projectorInfo.Mode().IsRegular() {
		return "", false
	}
	return strings.Join([]string{
		model, metadata.Path, modelInfo.ModTime().UTC().Format(time.RFC3339Nano), strconv.FormatInt(modelInfo.Size(), 10),
		metadata.ProjectorPath, projectorInfo.ModTime().UTC().Format(time.RFC3339Nano), strconv.FormatInt(projectorInfo.Size(), 10),
	}, "\x00"), true
}

func (s *Server) computerVisionReady(model string) bool {
	key, ok := s.visionCheckKey(model)
	if !ok {
		return false
	}
	_, passed := s.computerVisionChecks.Load(key)
	return passed
}

func (s *Server) checkComputerVision(ctx context.Context, model string) computerVisionCheck {
	result := computerVisionCheck{Code: "computer_vision_unavailable", Message: "Local vision is not installed for this model. Structured controls remain available.", Retryable: false}
	key, installed := s.visionCheckKey(model)
	result.Installed = installed
	if !installed {
		return result
	}
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	release, err := s.acquireInference(ctx, model)
	if err != nil {
		result.Retryable = true
		result.Message = "The local vision runtime is busy or unavailable. Structured controls remain available; retry the check later."
		return result
	}
	defer release()
	if err := probeComputerVision(ctx, s.engine, model); err != nil {
		if errors.Is(err, errComputerVision) {
			result.Code = "computer_vision_check_failed"
			result.Message = "The installed model/projector did not read the synthetic image and return the required governed tool call. Screenshot tools remain disabled; structured controls still work."
			return result
		}
		result.Retryable = true
		result.Message = "The local vision check could not complete. Screenshot tools remain disabled; structured controls still work."
		return result
	}
	s.computerVisionChecks.Range(func(existing, _ any) bool {
		if value, ok := existing.(string); ok && strings.HasPrefix(value, model+"\x00") && value != key {
			s.computerVisionChecks.Delete(existing)
		}
		return true
	})
	s.computerVisionChecks.Store(key, struct{}{})
	result.Passed = true
	result.Code = "computer_vision_check_passed"
	result.Message = "Local image transport and one governed vision tool call passed for this loaded runtime. This is not full visual-grounding qualification."
	return result
}

func probeComputerVision(ctx context.Context, engine inference.Engine, model string) error {
	var nonce [6]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	return probeComputerVisionCode(ctx, engine, model, strings.ToUpper(hex.EncodeToString(nonce[:])))
}

func probeComputerVisionCode(ctx context.Context, engine inference.Engine, model, expected string) error {
	imageURL, err := visionProbeImage(expected)
	if err != nil {
		return err
	}
	zero, tokens, parallel := float32(0), 128, false
	var verify api.Tool
	for _, tool := range browserToolsForVision(false) {
		if tool.Function.Name == "browser_verify" {
			verify = tool
			break
		}
	}
	if verify.Function.Name != "browser_verify" {
		return errors.New("vision probe tool contract changed")
	}
	request := &api.ChatCompletionRequest{
		Model: model, Tools: []api.Tool{verify}, ToolChoice: "auto", ParallelToolCalls: &parallel,
		Temperature: &zero, MaxTokens: &tokens,
		Messages: []api.ChatMessage{{Role: "system", Content: "This is a synthetic local vision transport check. Read only the large hexadecimal code in the image. Call browser_verify exactly once with that code as text. Do not describe it or call another tool."}, {Role: "user", Content: []api.ChatContentPart{
			{Type: "text", Text: "Read the large hexadecimal code in this synthetic image and pass it unchanged to browser_verify."},
			{Type: "image_url", ImageURL: &api.ChatImageURL{URL: imageURL, Detail: "high"}},
		}}},
	}
	if _, err := api.ValidateChatMessages(request.Messages); err != nil {
		return err
	}
	var response *api.ChatCompletionResponse
	if raw, ok := engine.(inference.RawStreamingEngine); ok {
		request.Stream = true
		acc := agentStreamAccumulator{calls: make(map[int]*api.ToolCall)}
		err = raw.ChatCompletionStreamRaw(ctx, request, func(data json.RawMessage) error { _, _, addErr := acc.add(data); return addErr })
		response = acc.response()
	} else {
		response, err = engine.ChatCompletion(ctx, request)
	}
	if err != nil {
		return err
	}
	if response == nil || len(response.Choices) != 1 || response.Choices[0].FinishReason != "tool_calls" || len(response.Choices[0].Message.ToolCalls) != 1 {
		return errComputerVision
	}
	call := response.Choices[0].Message.ToolCalls[0]
	var arguments struct {
		Text string `json:"text"`
	}
	if call.ID == "" || call.Type != "function" || call.Function.Name != "browser_verify" || json.Unmarshal([]byte(call.Function.Arguments), &arguments) != nil || arguments.Text != expected {
		return errComputerVision
	}
	return nil
}

// A deliberately tiny embedded glyph set avoids fonts, network access, user
// content, and platform rendering differences in the runtime smoke check.
var visionGlyphs = map[rune][7]byte{
	'0': {0x0e, 0x11, 0x13, 0x15, 0x19, 0x11, 0x0e}, '1': {0x04, 0x0c, 0x04, 0x04, 0x04, 0x04, 0x0e},
	'2': {0x0e, 0x11, 0x01, 0x02, 0x04, 0x08, 0x1f}, '3': {0x1e, 0x01, 0x01, 0x0e, 0x01, 0x01, 0x1e},
	'4': {0x02, 0x06, 0x0a, 0x12, 0x1f, 0x02, 0x02}, '5': {0x1f, 0x10, 0x10, 0x1e, 0x01, 0x01, 0x1e},
	'6': {0x0e, 0x10, 0x10, 0x1e, 0x11, 0x11, 0x0e}, '7': {0x1f, 0x01, 0x02, 0x04, 0x08, 0x08, 0x08},
	'8': {0x0e, 0x11, 0x11, 0x0e, 0x11, 0x11, 0x0e}, '9': {0x0e, 0x11, 0x11, 0x0f, 0x01, 0x01, 0x0e},
	'A': {0x0e, 0x11, 0x11, 0x1f, 0x11, 0x11, 0x11}, 'B': {0x1e, 0x11, 0x11, 0x1e, 0x11, 0x11, 0x1e},
	'C': {0x0f, 0x10, 0x10, 0x10, 0x10, 0x10, 0x0f}, 'D': {0x1e, 0x11, 0x11, 0x11, 0x11, 0x11, 0x1e},
	'E': {0x1f, 0x10, 0x10, 0x1e, 0x10, 0x10, 0x1f}, 'F': {0x1f, 0x10, 0x10, 0x1e, 0x10, 0x10, 0x10},
}

func visionProbeImage(value string) (string, error) {
	const scale, margin = 10, 20
	width := margin*2 + len(value)*6*scale
	height := margin*2 + 7*scale
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	white, black := color.RGBA{255, 255, 255, 255}, color.RGBA{0, 0, 0, 255}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			canvas.SetRGBA(x, y, white)
		}
	}
	for index, character := range value {
		glyph, ok := visionGlyphs[character]
		if !ok {
			return "", errComputerVision
		}
		for row, bits := range glyph {
			for column := 0; column < 5; column++ {
				if bits&(1<<uint(4-column)) == 0 {
					continue
				}
				for dy := 0; dy < scale; dy++ {
					for dx := 0; dx < scale; dx++ {
						canvas.SetRGBA(margin+(index*6+column)*scale+dx, margin+row*scale+dy, black)
					}
				}
			}
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, canvas); err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(encoded.Bytes()), nil
}
