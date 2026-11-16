package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
)

func (h *SessionHandlers) sessionAttachmentStructure(w http.ResponseWriter, r *http.Request, name, id string) {
	q := r.URL.Query()
	values := []int{0, 100, 0, 0}
	for i, key := range []string{"offset", "limit", "relationship_offset", "navigation_offset"} {
		if q.Has(key) {
			value, err := strconv.Atoi(q.Get(key))
			if err != nil {
				writeError(w, "Invalid structure pagination", 400)
				return
			}
			values[i] = value
		}
	}
	if q.Get("revision") == "" || len(q.Get("revision")) > 128 {
		writeError(w, "A pinned attachment revision is required", 400)
		return
	}
	read := h.scoped(r).AttachmentStructure
	if q.Get("corrections") == "true" {
		read = h.scoped(r).AttachmentCorrectionStructure
	}
	page, err := read(r.Context(), name, sessions.AttachmentReference{ID: id, Revision: q.Get("revision")}, q.Get("unit"), values[0], values[1], values[2], values[3])
	if errors.Is(err, document.ErrStructureQuery) {
		writeError(w, "Invalid structure pagination", 400)
		return
	}
	if err != nil {
		writeSessionError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(page)
}
