package execution

import "context"

type ProcessPage struct {
	ProcessResult
	Offset        int  `json:"offset"`
	NextOffset    *int `json:"next_offset"`
	CapturedBytes int  `json:"captured_bytes"`
	Complete      bool `json:"complete"`
}

// Output offsets refer to an append-only captured byte stream, not rendered
// lines. Retrieval never repeats the command and does not invent empty output
// after the retention limit. Truncation remains explicit on every page.
func (m *ProcessManager) Output(ctx context.Context, b Binding, handle string, offset, limit int) (ProcessPage, error) {
	r, err := m.Inspect(ctx, b, handle)
	if err != nil {
		return ProcessPage{}, err
	}
	if offset < 0 || offset > len(r.Output) || limit < 1 || limit > MaxPageBytes {
		return ProcessPage{}, ErrLimit
	}
	total := len(r.Output)
	end := min(total, offset+limit)
	r.Output = r.Output[offset:end]
	next := nextOffset(end, total)
	complete := r.State != "running" && end == total
	if !complete && next == nil {
		next = &end
	}
	return ProcessPage{ProcessResult: r, Offset: offset, NextOffset: next, CapturedBytes: total, Complete: complete}, nil
}
