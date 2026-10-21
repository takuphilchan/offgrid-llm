package inference

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/takuphilchan/offgrid-llm/internal/jsonwire"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// ValidateToolDecision is the final transport-independent gate for tool-enabled
// agent responses. Native calls and decoded single-step calls use this same
// contract. This is structural validation, not authorization or effect evidence.
// The runner still checks live provider identity, policy and dispatch readiness.
func ValidateToolDecision(req *api.ChatCompletionRequest, response *api.ChatCompletionResponse) (err error) {
	defer func() {
		if err != nil {
			if _, unavailable := err.(*DecisionContractError); !unavailable {
				err = &DecisionValidationError{reason: err.Error()}
			}
		}
	}()
	if req == nil {
		return &DecisionContractError{reason: "model decision request is unavailable"}
	}
	if response == nil || len(response.Choices) != 1 {
		return fmt.Errorf("model did not return one complete decision")
	}
	choice := response.Choices[0]
	if choice.Index != 0 || (choice.Message.Role != "" && choice.Message.Role != "assistant") {
		return fmt.Errorf("model returned invalid decision metadata")
	}
	calls := choice.Message.ToolCalls
	selected, required, err := decisionToolChoice(req)
	if err != nil {
		return &DecisionContractError{reason: "configured tool choice is invalid"}
	}
	if len(calls) == 0 {
		if choice.FinishReason != "stop" || required || strings.TrimSpace(choice.Message.StringContent()) == "" {
			return fmt.Errorf("model did not finish a complete answer or required tool call")
		}
		// Ordinary assistant prose is never parsed for embedded executable calls.
		return nil
	}
	if choice.FinishReason != "tool_calls" || req.ToolChoice == "none" || len(calls) > 16 || (req.ParallelToolCalls != nil && !*req.ParallelToolCalls && len(calls) != 1) {
		return fmt.Errorf("model did not finish a supported tool decision")
	}
	catalog := make(map[string]api.Tool, len(req.Tools))
	for _, tool := range req.Tools {
		if tool.Type != "function" || tool.Function.Name == "" {
			return &DecisionContractError{reason: "invalid available tool catalog"}
		}
		if _, exists := catalog[tool.Function.Name]; exists {
			return &DecisionContractError{reason: "ambiguous available tool identity"}
		}
		catalog[tool.Function.Name] = tool
	}
	ids := make(map[string]bool, len(calls))
	for _, call := range calls {
		tool, known := catalog[call.Function.Name]
		if !known || (selected != "" && selected != call.Function.Name) || call.Type != "function" || strings.TrimSpace(call.ID) == "" || len(call.ID) > 512 || ids[call.ID] {
			return fmt.Errorf("model returned an unavailable tool or ambiguous call identity")
		}
		ids[call.ID] = true
		raw := []byte(call.Function.Arguments)
		if len(raw) > 256*1024 || jsonwire.Validate(raw) != nil {
			return fmt.Errorf("model returned ambiguous or malformed tool arguments")
		}
		var args map[string]any
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		if decoder.Decode(&args) != nil || args == nil {
			return fmt.Errorf("model tool arguments must be an object")
		}
		validationArgs, err := decisionSchemaValue(args)
		if err != nil {
			return err
		}
		params := tool.Function.Parameters
		if params == nil {
			params = map[string]any{"type": "object", "additionalProperties": false}
		}
		encoded, err := json.Marshal(params)
		if err != nil || len(encoded) > 1<<20 {
			return &DecisionContractError{reason: "available tool schema is invalid or exceeds limit"}
		}
		var schema jsonschema.Schema
		if json.Unmarshal(encoded, &schema) != nil {
			return &DecisionContractError{reason: "available tool schema is invalid"}
		}
		// Never fetch a remote schema, read a local path, or expose URI details.
		resolved, err := schema.Resolve(&jsonschema.ResolveOptions{Loader: func(*url.URL) (*jsonschema.Schema, error) {
			return nil, fmt.Errorf("external schema references are unavailable")
		}})
		if err != nil {
			return &DecisionContractError{reason: "available tool schema cannot be resolved locally"}
		}
		if resolved.Validate(validationArgs) != nil {
			// Validation errors can contain private argument values; keep them out
			// of logs and user-visible runtime failures.
			return fmt.Errorf("model tool arguments do not match the available schema")
		}
	}
	return nil
}

// DecisionValidationError can only be constructed by the owned validation gate.
// It certifies rejection before dispatch, not that earlier task effects are known.
// Providers' arbitrary errors must never be treated as this retry classification.
type DecisionValidationError struct{ reason string }

func (e *DecisionValidationError) Error() string { return e.reason }

// A missing or invalid provider contract cannot be repaired by model guessing.
// Keep it outside the pre-effect model-correction classification.
type DecisionContractError struct{ reason string }

func (e *DecisionContractError) Error() string { return e.reason }

// jsonschema-go accepts Go numeric values but classifies json.Number as a
// string in its type validator. Convert only the validation copy, preserving
// exact integer IDs and leaving the original dispatch arguments untouched.
func decisionSchemaValue(value any) (any, error) {
	switch v := value.(type) {
	case json.Number:
		if n, err := v.Int64(); err == nil {
			return n, nil
		}
		if !strings.ContainsAny(string(v), ".eE") {
			return nil, fmt.Errorf("tool argument integer exceeds supported validation range")
		}
		if len(v) > 128 {
			return nil, fmt.Errorf("tool argument number exceeds supported validation range")
		}
		if at := strings.IndexAny(string(v), "eE"); at >= 0 {
			exponent, err := strconv.Atoi(string(v)[at+1:])
			if err != nil || exponent < -324 || exponent > 308 {
				return nil, fmt.Errorf("tool argument exponent exceeds supported validation range")
			}
		}
		n, err := v.Float64()
		if err != nil || math.IsInf(n, 0) || math.IsNaN(n) || math.Abs(n) >= 1<<53 {
			return nil, fmt.Errorf("tool argument number exceeds supported validation range")
		}
		// Reject lost decimal precision (including fractions rounded into an
		// integer). Common decimals such as 0.1 retain their round-trip spelling.
		exact, ok := new(big.Rat).SetString(string(v))
		roundTrip, roundOK := new(big.Rat).SetString(strconv.FormatFloat(n, 'g', -1, 64))
		if !ok || !roundOK || exact.Cmp(roundTrip) != 0 {
			return nil, fmt.Errorf("tool argument number cannot be validated without precision loss")
		}
		return n, nil
	case map[string]any:
		copy := make(map[string]any, len(v))
		for key, child := range v {
			converted, err := decisionSchemaValue(child)
			if err != nil {
				return nil, err
			}
			copy[key] = converted
		}
		return copy, nil
	case []any:
		copy := make([]any, len(v))
		for i, child := range v {
			converted, err := decisionSchemaValue(child)
			if err != nil {
				return nil, err
			}
			copy[i] = converted
		}
		return copy, nil
	default:
		return value, nil
	}
}

func decisionToolChoice(req *api.ChatCompletionRequest) (string, bool, error) {
	if req.ToolChoice == nil || req.ToolChoice == "auto" || req.ToolChoice == "none" {
		return "", false, nil
	}
	if req.ToolChoice == "required" {
		return "", true, nil
	}
	raw, err := json.Marshal(req.ToolChoice)
	if err != nil {
		return "", false, fmt.Errorf("invalid tool choice")
	}
	var choice struct {
		Type     string `json:"type"`
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&choice) != nil || choice.Type != "function" || choice.Function.Name == "" {
		return "", false, fmt.Errorf("invalid tool choice")
	}
	return choice.Function.Name, true, nil
}
