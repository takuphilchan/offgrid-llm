package serviceclient

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/takuphilchan/offgrid-llm/internal/jsonwire"
	"io"
)

// StrictJSON is for versioned control envelopes, not an opt-in parser of
// assistant prose. Ordinary additive public responses retain JSON compatibility.
func (c *Client) StrictJSON(ctx context.Context, method, path string, input, result any, limit int64) error {
	if limit < 1 || limit > 16<<20 {
		return &Error{Code: "invalid_response_limit", Message: "Invalid control response limit."}
	}
	data, err := json.Marshal(input)
	if err != nil {
		return err
	}
	response, err := c.Do(ctx, method, path, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil || int64(len(body)) > limit || jsonwire.Validate(body) != nil || len(bytes.TrimSpace(body)) == 0 || bytes.TrimSpace(body)[0] != '{' {
		return &Error{Code: "invalid_response", Message: "OffGrid returned an invalid control envelope."}
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if decoder.Decode(result) != nil || decoder.Decode(new(any)) != io.EOF {
		return &Error{Code: "upgrade_required", Message: "The execution control contract changed. Update the matching local host; no operation was replayed."}
	}
	return nil
}
