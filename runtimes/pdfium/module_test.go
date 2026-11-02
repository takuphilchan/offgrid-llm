package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/tetratelabs/wazero"
)

func TestEngineMemoryAndEnvironmentBoundaries(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	e, err := newPDFEngine(ctx, wazero.NewRuntimeConfig().WithMemoryLimitPages(8192).WithCloseOnContextDone(true))
	if err != nil {
		t.Fatal(err)
	}
	defer e.close()
	ptr, err := e.allocate(32)
	if err != nil {
		t.Fatal(err)
	}
	defer e.call("free", ptr)
	if !e.module.Memory().Write(uint32(ptr), []byte("abcdef")) {
		t.Fatal("write failed")
	}
	copyMemory(ctx, e.module, []uint64{ptr + 1, ptr, 5})
	got, ok := e.module.Memory().Read(uint32(ptr), 6)
	if !ok || string(got) != "aabcde" {
		t.Fatalf("overlapping copy: %q", got)
	}
	old, _ := e.module.Memory().Grow(0)
	request := []uint64{512<<20 + 1}
	resizeHeap(ctx, e.module, request)
	after, _ := e.module.Memory().Grow(0)
	if request[0] != 0 || old != after {
		t.Fatal("memory ceiling bypassed")
	}
	t.Setenv("OFFGRID_PRIVATE_TEST", "must-not-enter-parser")
	stack := []uint64{ptr, ptr + 4}
	emptyEnvironment(ctx, e.module, stack)
	count, _ := e.module.Memory().ReadUint32Le(uint32(ptr))
	bytes, _ := e.module.Memory().ReadUint32Le(uint32(ptr + 4))
	if stack[0] != 0 || count != 0 || bytes != 0 {
		t.Fatal("host environment exposed")
	}
	t.Run("out-of-bounds-copy-traps", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("out-of-bounds copy accepted")
			}
		}()
		copyMemory(ctx, e.module, []uint64{ptr, 0xffffffff, 16})
	})
	if !strings.Contains(engineNotices, "licenses/pdfium.txt") || !strings.Contains(engineNotices, "licenses/freetype.txt") {
		t.Fatal("redistribution notices missing")
	}
}

func TestCancelledEngineCannotInitialize(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	e, err := newPDFEngine(ctx, wazero.NewRuntimeConfig().WithCloseOnContextDone(true))
	if e != nil {
		e.close()
		t.Fatal("cancelled engine initialized")
	}
	if err == nil {
		t.Fatal("cancellation lost")
	}
	if time.Since(start) > time.Second {
		t.Fatal("cancelled request compiled the engine")
	}
}
