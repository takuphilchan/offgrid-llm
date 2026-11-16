package server

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/testsupport/tifffixture"
	"image"
	"image/png"
	"net/http/httptest"
	"testing"
)

func runtimeTIFF(t *testing.T, data []byte) []byte {
	t.Helper()
	im, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return tifffixture.Encode([]image.Image{im, im}, []uint16{1, 6}, binary.LittleEndian)
}

func verifyTIFFKnowledgePreview(t *testing.T, s *Server, ctx context.Context, id string) {
	t.Helper()
	w := httptest.NewRecorder()
	s.handleDocumentSource(w, httptest.NewRequest("GET", "/v1/documents/source?id="+id+"&format=preview", nil).WithContext(ctx))
	var preview struct {
		Pages int              `json:"pages"`
		State string           `json:"state"`
		Units []rag.SourceUnit `json:"units"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &preview) != nil || preview.Pages != 2 || preview.State != "complete" || len(preview.Units) != 2 {
		t.Fatal(w.Code, w.Body.String())
	}
	for i, u := range preview.Units {
		if u.Index != i+1 || u.Text != "Scanned receipt total 42" {
			t.Fatal("wrong page evidence", u)
		}
	}
	w = httptest.NewRecorder()
	s.handleDocumentSource(w, httptest.NewRequest("GET", "/v1/documents/source?id="+id+"&format=preview&page=2", nil).WithContext(ctx))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if _, err := png.Decode(bytes.NewReader(w.Body.Bytes())); err != nil {
		t.Fatal(err)
	}
}
