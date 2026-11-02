package rag

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParserLimitsAndCancellation(t *testing.T) {
	p := NewDocumentParser()
	p.maxFileSize, p.maxExtractedSize = 4096, 32
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.ParseContext(ctx, []byte("text"), "test.txt", ".txt"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled parse: %v", err)
	}
	for _, test := range []struct {
		name, ext string
		data      []byte
	}{
		{"input", ".txt", bytes.Repeat([]byte("a"), 4097)},
		{"text", ".txt", bytes.Repeat([]byte("a"), 33)},
		{"office XML", ".docx", parserZip(t, map[string]string{"word/document.xml": strings.Repeat("x", 33)})},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("PATH", t.TempDir())
			if _, err := p.Parse(test.data, "test"+test.ext, test.ext); !isParseLimit(err) {
				t.Fatalf("expected bounded extraction, got %v", err)
			}
		})
	}
}

func TestParserArchiveExpansionLimit(t *testing.T) {
	data := parserZip(t, map[string]string{"word/document.xml": strings.Repeat("x", (64<<20)+1)})
	if _, err := NewDocumentParser().Parse(data, "large.docx", ".docx"); !isParseLimit(err) {
		t.Fatalf("expanded archive was not rejected: %v", err)
	}
}

func TestParserFileIngestBoundsBeforeAllocatingOrIndexing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oversized.txt")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate((100 << 20) + 1); err != nil {
		file.Close()
		t.Fatal(err)
	}
	file.Close()
	if _, err := (&Engine{}).IngestFile(context.Background(), path, nil); !isParseLimit(err) {
		t.Fatalf("oversized input reached embedding/storage: %v", err)
	}
	if _, err := (&Engine{}).IngestFile(context.Background(), t.TempDir(), nil); err == nil {
		t.Fatal("directory accepted as a document")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (&Engine{}).IngestReader(ctx, "cancelled", nil, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled reader was touched: %v", err)
	}
	if _, err := (&Engine{}).IngestReader(context.Background(), "invalid", bytes.NewReader([]byte{0xff}), nil); err == nil {
		t.Fatal("invalid UTF-8 reader reached indexing")
	}
}

func TestParserDoesNotLeakInternalErrors(t *testing.T) {
	err := classifyParseError(fmt.Errorf("private temp path /secret/document; process stderr"))
	var problem *ParseError
	if !errors.As(err, &problem) || problem.Code != "document_invalid" || strings.Contains(err.Error(), "secret") {
		t.Fatalf("unsafe error: %v", err)
	}
}

// A child copy of this test executable exercises actual process cancellation and
// output bounds on every platform, without relying on a shell or installed tools.
func TestParserWorkerHelper(t *testing.T) {
	mode := os.Getenv("OFFGRID_TEST_PARSER_WORKER")
	if mode == "" {
		return
	}
	switch mode {
	case "wait":
		time.Sleep(time.Minute)
	case "overflow":
		fmt.Print(strings.Repeat("x", 8192))
	case "text":
		fmt.Print("local extracted text")
	}
	os.Exit(0)
}

func TestParserWorkerBounds(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"text", "overflow", "wait"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, exe, "-test.run=^TestParserWorkerHelper$")
			cmd.Env = append(os.Environ(), "OFFGRID_TEST_PARSER_WORKER="+mode)
			start := time.Now()
			text, err := runParserCommand(ctx, cmd, 128)
			switch mode {
			case "text":
				if err != nil || text != "local extracted text" {
					t.Fatalf("text=%q err=%v", text, err)
				}
			case "overflow":
				if !isParseLimit(err) || text != "" {
					t.Fatalf("output limit: text=%q err=%v", text, err)
				}
			case "wait":
				if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > 5*time.Second {
					t.Fatalf("deadline not enforced: %v", err)
				}
			}
			if cmd.ProcessState == nil || !cmd.ProcessState.Exited() && mode != "wait" {
				t.Fatal("worker was not reaped")
			}
		})
	}
}
