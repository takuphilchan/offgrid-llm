package agents

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/takuphilchan/offgrid-llm/internal/jsonwire"
)

// CanonicalArguments preserves integer precision and rejects extra JSON values.
func CanonicalArguments(args json.RawMessage) (json.RawMessage, error) {
	if len(args) == 0 {
		args = json.RawMessage(`{}`)
	}
	if jsonwire.Validate(args) != nil {
		return nil, fmt.Errorf("tool arguments contain malformed Unicode or repeated fields")
	}
	decoder := json.NewDecoder(bytes.NewReader(args))
	decoder.UseNumber()
	var value map[string]any
	if err := decoder.Decode(&value); err != nil || value == nil {
		return nil, fmt.Errorf("tool arguments must be a JSON object")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("tool arguments contain trailing data")
	}
	return json.Marshal(value)
}
