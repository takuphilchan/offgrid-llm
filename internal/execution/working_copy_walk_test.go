package execution

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

type generatedCopyDirectory struct {
	left, read, maxRequest int
	afterRead              func()
}

func (d *generatedCopyDirectory) Readdir(n int) ([]os.FileInfo, error) {
	if n <= 0 || n > 256 {
		return nil, fmt.Errorf("unbounded directory request: %d", n)
	}
	d.maxRequest = max(d.maxRequest, n)
	count := min(n, d.left)
	entries := make([]os.FileInfo, count)
	for i := range entries {
		entries[i] = generatedCopyEntry(fmt.Sprintf("entry-%09d", d.left-i))
	}
	d.left -= count
	d.read += count
	if d.afterRead != nil {
		d.afterRead()
	}
	if count == 0 {
		return entries, io.EOF
	}
	return entries, nil
}

type generatedCopyEntry string

func (e generatedCopyEntry) Name() string       { return string(e) }
func (e generatedCopyEntry) IsDir() bool        { return false }
func (e generatedCopyEntry) Size() int64        { return 0 }
func (e generatedCopyEntry) Mode() os.FileMode  { return 0 }
func (e generatedCopyEntry) ModTime() time.Time { return time.Time{} }
func (e generatedCopyEntry) Sys() any           { return nil }

func TestWorkingCopyDirectoryStopsBeforeUnboundedAllocation(t *testing.T) {
	directory := &generatedCopyDirectory{left: 10_000_000}
	remaining := 300
	entries, err := readWorkingCopyDirectory(context.Background(), directory, &remaining)
	if !errors.Is(err, ErrLimit) || entries != nil || directory.read != 301 || directory.maxRequest != 256 {
		t.Fatalf("entries=%d error=%v read=%d largest batch=%d", len(entries), err, directory.read, directory.maxRequest)
	}
	ctx, cancel := context.WithCancel(context.Background())
	directory = &generatedCopyDirectory{left: 10_000_000, afterRead: cancel}
	remaining = 1000
	if _, err = readWorkingCopyDirectory(ctx, directory, &remaining); !errors.Is(err, context.Canceled) || directory.read != 256 {
		t.Fatal("cancellation did not stop enumeration between bounded batches", directory.read, err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	directory = &generatedCopyDirectory{afterRead: cancel}
	remaining = 0
	if _, err = readWorkingCopyDirectory(ctx, directory, &remaining); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation during final empty read was ignored", err)
	}
}

func TestWorkingCopyInventoryOrderAndSeparateExclusionLimit(t *testing.T) {
	directory := t.TempDir()
	for name, content := range map[string]string{"z.txt": "z", "a/b.txt": "bb", ".git/objects/deep/ignored": "excluded", "a/.git/config": "excluded"} {
		full := filepath.Join(directory, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	policy := DefaultWorkingCopyPolicy()
	policy.MaxEntries = 3
	req := WorkingCopyRequest{MaxBytes: 3}
	one, err := inventoryWorkingCopy(context.Background(), root, req, policy)
	if err != nil {
		t.Fatal(err)
	}
	var names, excluded []string
	for _, entry := range one.Entries {
		names = append(names, entry.Path)
	}
	for _, entry := range one.Exclusions {
		excluded = append(excluded, entry.Path)
	}
	if !reflect.DeepEqual(names, []string{"a", "a/b.txt", "z.txt"}) || !reflect.DeepEqual(excluded, []string{".git", "a/.git"}) || one.Bytes != 3 {
		t.Fatal(names, excluded, one.Bytes)
	}
	two, err := inventoryWorkingCopy(context.Background(), root, req, policy)
	if err != nil || !reflect.DeepEqual(one, two) {
		t.Fatal("unchanged tree changed deterministic manifest", err)
	}
	policy.MaxEntries = 2
	if _, err = inventoryWorkingCopy(context.Background(), root, req, policy); !errors.Is(err, ErrLimit) {
		t.Fatal("included entry limit was widened by exclusions", err)
	}
}

func TestWorkingCopyMeasurementExactBudgetAndCancellation(t *testing.T) {
	directory := t.TempDir()
	if err := os.Mkdir(filepath.Join(directory, "a"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a/b", "c"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte("12"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if bytes, err := measureCopyTree(context.Background(), root, 3); err != nil || bytes != 4 {
		t.Fatal("exact budget rejected", bytes, err)
	}
	if _, err = measureCopyTree(context.Background(), root, 2); !errors.Is(err, ErrLimit) {
		t.Fatal("parent siblings and descendants did not share one budget", err)
	}
	visited := 0
	err = walkWorkingCopy(context.Background(), root, 1, func(string, os.FileInfo) error { visited++; return nil })
	if !errors.Is(err, ErrLimit) || visited != 0 {
		t.Fatal("oversized directory reached visitor", visited, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	err = walkWorkingCopy(ctx, root, 3, func(string, os.FileInfo) error { cancel(); return nil })
	if !errors.Is(err, context.Canceled) {
		t.Fatal("visitor cancellation was ignored", err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	err = walkWorkingCopy(ctx, root, 3, func(name string, _ os.FileInfo) error {
		if name == "c" {
			cancel()
		}
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation at final entry was ignored", err)
	}
}

func TestWorkingCopyOversizedMeasurementRetainsFullReservation(t *testing.T) {
	endpoint, store, binding, _ := workingCopyFixture(t)
	child, err := endpoint.PrepareWorker(context.Background(), workingCopyRequest(binding, "overflow"))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i <= store.policy.MaxEntries; i++ {
		if err = os.WriteFile(filepath.Join(child.Context.Root, fmt.Sprintf("file-%03d", i)), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = store.Measure(context.Background(), child.Record.ID, binding); !errors.Is(err, ErrLimit) {
		t.Fatal("oversized measurement was accepted", err)
	}
	record, err := store.Inspect(context.Background(), child.Record.ID, binding)
	if err != nil || record.State != "uncertain" || record.UsageMeasured || record.ReservedBytes < store.policy.MaxTotalBytes {
		t.Fatal("incomplete inventory released storage capacity", record, err)
	}
	if _, err = endpoint.PrepareWorker(context.Background(), workingCopyRequest(binding, "blocked-overflow")); !errors.Is(err, ErrLimit) {
		t.Fatal("incomplete measurement admitted another worker", err)
	}
}
