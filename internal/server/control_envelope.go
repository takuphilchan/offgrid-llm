package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/jsonwire"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func decodeControlEnvelope(w http.ResponseWriter, r *http.Request, target any) error {
	limit := int64(128 << 10)
	if _, ok := target.(*api.ConversationInputCommand); ok {
		limit = api.MaxSessionImageRequestBytes
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
	if err != nil || jsonwire.Validate(raw) != nil {
		return agents.ErrControlCommand
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return agents.ErrControlCommand
	}
	for _, key := range []string{"request_id", "target_id", "expected_revision", "kind", "payload"} {
		if value, ok := fields[key]; !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return agents.ErrControlCommand
		}
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(target) != nil || d.Decode(new(any)) != io.EOF {
		return agents.ErrControlCommand
	}
	return nil
}
