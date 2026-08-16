package artifacts

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestStoreIsContentAddressed(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := store.Put(context.Background(), strings.NewReader("artifact payload"), Metadata{Name: "result.txt"})
	if err != nil {
		t.Fatal(err)
	}
	reader, loaded, err := store.Open(metadata.Digest)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	content, _ := io.ReadAll(reader)
	if string(content) != "artifact payload" || loaded.Digest != metadata.Digest {
		t.Fatalf("unexpected artifact: %q %#v", content, loaded)
	}
}

func TestStoreRepeatedPutKeepsOriginalMetadata(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.Put(context.Background(), strings.NewReader("same"), Metadata{Name: "first.txt"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Put(context.Background(), strings.NewReader("same"), Metadata{Name: "second.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if first.Digest != second.Digest || second.Name != "first.txt" {
		t.Fatalf("content metadata changed: first=%#v second=%#v", first, second)
	}
}
