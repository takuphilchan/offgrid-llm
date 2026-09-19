package api

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestInferenceControlsAreNotSilentlyDiscarded(t *testing.T) {
	for _, raw := range []string{`{"logprobs":true}`, `{"response_format":{"type":"json_object"}}`, `{"top_k":5}`} {
		for _, request := range []any{&ChatCompletionRequest{}, &CompletionRequest{}} {
			err := json.Unmarshal([]byte(raw), request)
			var unsupported *UnsupportedControlError
			if !errors.As(err, &unsupported) {
				t.Fatalf("accepted unsupported control %s: %v", raw, err)
			}
		}
	}
	var request ChatCompletionRequest
	if err := json.Unmarshal([]byte(`{"seed":0,"logprobs":false,"response_format":null}`), &request); err != nil {
		t.Fatal(err)
	}
	if request.Seed == nil || *request.Seed != 0 {
		t.Fatal("seed zero lost")
	}
}
