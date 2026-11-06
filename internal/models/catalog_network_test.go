package models

import (
	"context"
	"net/url"
	"os"
	"path"
	"strings"
	"testing"
)

// Explicit maintenance check: public metadata only, never downloads model data.
func TestCurrentCatalogUpstreamMetadata(t *testing.T) {
	if os.Getenv("OFFGRID_CATALOG_NETWORK_CHECK") != "1" {
		t.Skip("opt-in public metadata check")
	}
	hf := NewHuggingFaceClient()
	for _, model := range currentSmallModels() {
		t.Run(model.ID, func(t *testing.T) {
			v := model.Variants[0]
			u, _ := url.Parse(v.Sources[0].URL)
			parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
			b, err := hf.ResolveGGUFBundle(context.Background(), parts[0]+"/"+parts[1], parts[3], path.Base(u.Path))
			if err != nil {
				t.Fatal(err)
			}
			if b.Revision != parts[3] || b.Weights.SHA256 != v.SHA256 || b.Weights.Size != v.Size {
				t.Fatal("catalog no longer matches pinned source")
			}
			vision := model.ID == "qwen3.5-2b" || model.ID == "gemma-4-e2b-it"
			if (b.Projector != nil) != vision {
				t.Fatalf("unexpected projector: %+v", b.Projector)
			}
			t.Logf("reviewed %s: %d artifacts, %d bytes", b.Revision, len(b.Artifacts()), b.TotalBytes())
		})
	}
}
