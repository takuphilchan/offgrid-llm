package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/cli"
)

func TestAgentDraftStreamsBeforeCompletionWithoutRepeatingAnswer(t *testing.T) {
	reader, writer := io.Pipe()
	output := make(chan string, 32)
	w := agentCaptureWriter{output}
	done := make(chan error, 1)
	go func() { done <- renderAgentStreamMode(w, io.Discard, reader, true, false, cli.PresentationStyle{}) }()
	defer reader.Close()
	defer writer.Close()
	fmt.Fprintln(writer, `data: {"type":"status","run_id":"run-1","status":"running","progress":{"phase":"generating","iteration":1,"preview":"Hello 日本語"}}`)
	var text strings.Builder
	deadline := time.After(time.Second)
	for !strings.Contains(text.String(), "Hello 日本語") {
		select {
		case chunk := <-output:
			text.WriteString(chunk)
		case <-deadline:
			t.Fatal("response buffered until completion")
		}
	}
	for _, preview := range []string{"Hello 日本語", "Hello 日本語!"} {
		data, _ := json.Marshal(map[string]any{"type": "status", "status": "running", "progress": map[string]any{"phase": "generating", "iteration": 1, "preview": preview}})
		fmt.Fprintf(writer, "data: %s\n", data)
	}
	fmt.Fprintln(writer, `data: {"type":"done","run_id":"run-1","status":"completed","output":"Hello 日本語!"}`)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	close(output)
	for chunk := range output {
		text.WriteString(chunk)
	}
	if strings.Count(text.String(), "Hello 日本語") != 1 || !strings.Contains(text.String(), "Hello 日本語!") {
		t.Fatal(text.String())
	}
}

type agentCaptureWriter struct{ chunks chan<- string }

func (w agentCaptureWriter) Write(p []byte) (int, error) { w.chunks <- string(p); return len(p), nil }

func TestAgentDraftFailureAndReplacementNeverLookCompleted(t *testing.T) {
	for _, terminal := range []string{"error", "eof", "replacement"} {
		stream := `data: {"type":"status","run_id":"run-1","status":"running","progress":{"phase":"generating","iteration":1,"preview":"partial"}}` + "\n"
		if terminal == "error" {
			stream += `data: {"type":"error","run_id":"run-1","status":"failed","error":"model stopped"}` + "\n"
		}
		if terminal == "replacement" {
			stream += `data: {"type":"done","run_id":"run-1","status":"completed","output":"Different final answer"}` + "\n"
		}
		var out, progress bytes.Buffer
		err := renderAgentStreamMode(&out, &progress, strings.NewReader(stream), true, false, cli.PresentationStyle{})
		if terminal != "replacement" && (err == nil || strings.Contains(progress.String(), "Completed")) {
			t.Fatal(terminal, err, progress.String())
		}
		if !strings.Contains(out.String(), "partial") || !strings.Contains(out.String(), "[Draft") && !strings.Contains(out.String(), "[Stream") {
			t.Fatal(out.String())
		}
	}
}

func TestAgentSpinnerClearsBeforeOtherOutputAndStops(t *testing.T) {
	var out bytes.Buffer
	d := newAgentLiveDisplay(&out, false)
	d.active = true
	d.tick()
	w := agentDisplayWriter{d, &out}
	fmt.Fprint(w, "Approval required\n")
	d.close()
	before := out.String()
	d.tick()
	if before != out.String() || !strings.Contains(before, "\r\x1b[2KApproval required\n") {
		t.Fatal(out.String())
	}
	quiet := newAgentLiveDisplay(io.Discard, true)
	quiet.close()
}
