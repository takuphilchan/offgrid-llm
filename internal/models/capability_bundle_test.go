package models

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCapabilityBundleMeasuresContentRolesAndMembership(t *testing.T) {
	open := func(name, content string) *os.File {
		t.Helper()
		path := filepath.Join(t.TempDir(), name)
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = file.Close() })
		return file
	}
	a, b := open("runtime", "binary-a"), open("library", "library-a")
	measure := func(files map[string]*os.File) string {
		t.Helper()
		id, err := MeasureCapabilityBundle(context.Background(), files)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	want := measure(map[string]*os.File{"executable": a, "library": b})
	if got := measure(map[string]*os.File{"library": b, "executable": a}); got != want {
		t.Fatal("map order changed identity")
	}
	if got := measure(map[string]*os.File{"executable": open("relocated", "binary-a"), "library": b}); got != want {
		t.Fatal("path changed content identity")
	}
	for _, changed := range []map[string]*os.File{
		{"executable": open("runtime", "binary-b"), "library": b},
		{"executable": a, "library": open("library", "library-b")},
		{"executable": a},
		{"executable": b, "library": a},
	} {
		if got := measure(changed); got == want {
			t.Fatal("changed bundle retained identity")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := MeasureCapabilityBundle(ctx, map[string]*os.File{"executable": a}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	for _, invalid := range []map[string]*os.File{nil, {"executable": nil}, {"../path": a}} {
		if _, err := MeasureCapabilityBundle(context.Background(), invalid); err == nil {
			t.Fatal("invalid bundle accepted")
		}
	}
}
