package api

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// UnsupportedControlError never includes prompts or caller-supplied values.
type UnsupportedControlError struct{ Control string }

func (e *UnsupportedControlError) Error() string {
	return fmt.Sprintf("inference control %q is not supported by OffGrid; it was not applied", e.Control)
}

func rejectUnsupportedControls(data []byte, chat bool) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for _, name := range []string{"logprobs", "top_logprobs", "prompt_logprobs", "response_format", "top_k", "min_p", "typical_p", "mirostat", "mirostat_tau", "mirostat_eta", "repetition_penalty", "repeat_penalty", "grammar", "json_schema"} {
		if chat && name == "response_format" {
			continue
		}
		if value, found := fields[name]; found && !bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			if name == "logprobs" && bytes.Equal(bytes.TrimSpace(value), []byte("false")) {
				continue
			}
			return &UnsupportedControlError{Control: name}
		}
	}
	return nil
}

func (r *ChatCompletionRequest) UnmarshalJSON(data []byte) error {
	if err := rejectUnsupportedControls(data, true); err != nil {
		return err
	}
	type wire ChatCompletionRequest
	var value wire
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	if err := value.ResponseFormat.Validate(); err != nil {
		return err
	}
	if value.ResponseFormat.Constrained() && len(value.Tools) > 0 {
		return &UnsupportedControlError{Control: "response_format with tools"}
	}
	*r = ChatCompletionRequest(value)
	return nil
}

func (r *CompletionRequest) UnmarshalJSON(data []byte) error {
	if err := rejectUnsupportedControls(data, false); err != nil {
		return err
	}
	type wire CompletionRequest
	var value wire
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*r = CompletionRequest(value)
	return nil
}
