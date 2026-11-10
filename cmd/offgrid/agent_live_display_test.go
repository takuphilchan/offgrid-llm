package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/takuphilchan/offgrid-llm/internal/cli"
)

func TestAgentProgressUsesAvailableCellsWithoutWrapping(t *testing.T) {
	label := "Waiting for a command to finish"
	if got := agentProgressLine("*", label, 12*time.Second, 80); !strings.Contains(got, label) {
		t.Fatal(got)
	}
	for _, width := range []int{8, 20, 32, 80} {
		got := agentProgressLine("*", "Reading 日本語 document\n\x1b[2J", time.Hour, width)
		if ansi.StringWidth(got) >= width || strings.ContainsAny(got, "\n\r\x1b") {
			t.Fatalf("width %d: %q", width, got)
		}
	}
}

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

func TestAnimatedAgentProgressKeepsTransientPhasesOutOfTranscript(t *testing.T) {
	stream := strings.Join([]string{
		`data: {"type":"status","run_id":"run-1","status":"running","progress":{"phase":"queued","iteration":1}}`,
		`data: {"type":"status","run_id":"run-1","status":"running","progress":{"phase":"loading","iteration":1}}`,
		`data: {"type":"status","run_id":"run-1","status":"running","progress":{"phase":"tool","iteration":1,"tool":"workspace_list"}}`,
		`data: {"type":"done","run_id":"run-1","status":"completed","output":"Files listed."}`,
	}, "\n") + "\n"
	var out, progress bytes.Buffer
	if err := renderAgentStreamMode(&out, &progress, strings.NewReader(stream), true, true, cli.PresentationStyle{}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(progress.String(), "Waiting for inference") || strings.Contains(progress.String(), "Loading model") {
		t.Fatal("transient phases were printed as durable history", progress.String())
	}
	if !strings.Contains(progress.String(), "Listing files") || !strings.Contains(out.String(), "Files listed.") {
		t.Fatal(out.String(), progress.String())
	}
}

func TestDelegatedProgressShowsTransitionsWithoutRepeatingTheWholeSnapshot(t *testing.T) {
	stream := strings.Join([]string{
		`data: {"type":"status","run_id":"run-parent","status":"waiting_for_children","children":[{"id":"run-child","status":"pending","spec":{"key":"research","goal":"Find sources"}}]}`,
		`data: {"type":"status","run_id":"run-parent","status":"waiting_for_children","children":[{"id":"run-child","status":"running","phase":"loading","spec":{"key":"research","goal":"Find sources"}}]}`,
		`data: {"type":"status","run_id":"run-parent","status":"waiting_for_children","children":[{"id":"run-child","status":"running","phase":"generating","spec":{"key":"research","goal":"Find sources"}}]}`,
		`data: {"type":"status","run_id":"run-parent","status":"waiting_for_children","children":[{"id":"run-child","status":"completed","spec":{"key":"research","goal":"Find sources"}}]}`,
		`data: {"type":"done","run_id":"run-parent","status":"completed","output":"Reviewed sources."}`,
	}, "\n") + "\n"
	var out, progress bytes.Buffer
	if err := renderAgentStreamMode(&out, &progress, strings.NewReader(stream), true, false, cli.PresentationStyle{}); err != nil {
		t.Fatal(err)
	}
	shown := progress.String()
	for _, state := range []string{"research · pending", "research · running", "research · completed"} {
		if strings.Count(shown, state) != 1 {
			t.Fatal("child transition missing or repeated", state, shown)
		}
	}
	if strings.Contains(shown, "Status: waiting_for_children") || !strings.Contains(out.String(), "Reviewed sources.") {
		t.Fatal("graph progress displaced the final result", shown, out.String())
	}
}

func TestAgentProgressAnimatesInPlace(t *testing.T) {
	t.Setenv("OFFGRID_UNICODE", "1")
	var out bytes.Buffer
	d := newAgentLiveDisplay(&out, true)
	d.phase("Loading model", true)
	deadline := time.Now().Add(2 * time.Second)
	for {
		d.mu.Lock()
		shown := strings.Contains(out.String(), "Loading model")
		d.mu.Unlock()
		if shown {
			break
		}
		if time.Now().After(deadline) {
			d.close()
			t.Fatal("loading animation did not render")
		}
		time.Sleep(10 * time.Millisecond)
	}
	d.close()
	if !strings.Contains(out.String(), "⠋ Loading model") || !strings.HasSuffix(out.String(), "\r\x1b[2K") {
		t.Fatal("animation did not clear before returning to the prompt", out.String())
	}
}
