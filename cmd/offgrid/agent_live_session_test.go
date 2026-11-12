package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLiveEditorCompactProgressAndCorrection(t *testing.T) {
	t.Setenv("OFFGRID_ANIMATIONS", "0")
	var output bytes.Buffer
	d := &agentLiveEditor{out: &output, width: 90, height: 24, started: time.Now()}
	d.begin()
	if strings.Contains(output.String(), "> ") {
		t.Fatal("live work must not look like a second idle prompt", output.String())
	}
	if !strings.Contains(output.String(), "Working") || !strings.Contains(output.String(), "  Type a correction") || strings.Contains(output.String(), "\x1b[21;1H") {
		t.Fatal("progress and input must be adjacent", output.String())
	}
	d.AgentProgressPhase("Loading model")
	if !strings.Contains(output.String(), "Loading model") {
		t.Fatal("missing real phase")
	}
	d.editor.insert("shorter please")
	output.Reset()
	d.draw()
	if !strings.Contains(output.String(), "shorter please") || strings.Contains(output.String(), "Type a correction") {
		t.Fatal(output.String())
	}
	d.resize(48, 16)
	if d.width != 48 || d.height != 16 {
		t.Fatal("resize did not update editor bounds")
	}
	d.close()
	if !strings.Contains(output.String(), "\x1b[?2004l") {
		t.Fatal("paste mode not restored")
	}
}

func TestLiveEditorReceivesStreamPhasesAndAnswer(t *testing.T) {
	t.Setenv("OFFGRID_ANIMATIONS", "0")
	var output bytes.Buffer
	d := &agentLiveEditor{out: &output, width: 90, height: 24, started: time.Now()}
	stream := "data: {\"type\":\"status\",\"progress\":{\"phase\":\"loading\",\"iteration\":1}}\n" +
		"data: {\"type\":\"status\",\"progress\":{\"phase\":\"generating\",\"iteration\":1,\"preview\":\"Hello\"}}\n" +
		"data: {\"type\":\"done\",\"status\":\"completed\",\"output\":\"Hello there\"}\n"
	if err := renderAgentStreamMode(d, d, strings.NewReader(stream), true, false, legacyPresenter().Style); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Loading model", "Responding", "Hello", " there", "Completed in"} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("missing %q", want)
		}
	}
}

func TestLiveKeysSplitUnicodePasteNeverSubmits(t *testing.T) {
	k := agentLiveKeys{}
	input := "\x1b[200~café 日本語\r\n/stop\x1b[201~"
	var keys []string
	for _, b := range []byte(input) {
		k.data = append(k.data, b)
		key, paste, ok := k.next(time.Now())
		if ok {
			if !paste {
				t.Fatal("paste became a control", key)
			}
			keys = append(keys, livePaste(key))
		}
	}
	if len(keys) != 1 || keys[0] != "café 日本語\n/stop" {
		t.Fatal(keys)
	}
}
func TestLiveKeysOversizePasteDrainsBeforeNextKey(t *testing.T) {
	k := agentLiveKeys{data: []byte("\x1b[200~" + strings.Repeat("x", 32001) + "\x1b[201~\r")}
	text, paste, ok := k.next(time.Now())
	if !ok || !paste || text != "" || !k.overflow {
		t.Fatal("oversized paste accepted")
	}
	key, _, ok := k.next(time.Now())
	if !ok || key != "\r" {
		t.Fatal("lost next key", key)
	}
}

// Only the explicit native fixture driver enables this. No real model, service,
// credentials, grants or workspace operations are involved.
func TestAgentLivePTYInput(t *testing.T) {
	if os.Getenv("OFFGRID_TEST_SESSION_PTY") != "1" {
		t.Skip("isolated PTY fixture only")
	}
	reader, writer := io.Pipe()
	stopping := os.Getenv("OFFGRID_TEST_LIVE_STOP") == "1"
	steered := make(chan struct{}, 1)
	go func() {
		defer writer.Close()
		fmt.Fprintln(writer, `data: {"type":"status","run_id":"run-fixture","status":"running","progress":{"phase":"loading","iteration":1}}`)
		<-steered
		if stopping {
			fmt.Fprintln(writer, `data: {"type":"done","run_id":"run-fixture","status":"cancelled"}`)
			return
		}
		fmt.Fprintln(writer, `data: {"type":"status","run_id":"run-fixture","status":"running","progress":{"phase":"generating","iteration":1,"preview":"Incremental fixture"}}`)
		time.Sleep(900 * time.Millisecond)
		fmt.Fprintln(writer, `data: {"type":"done","run_id":"run-fixture","status":"completed","output":"Incremental fixture answer."}`)
	}()
	input := bufio.NewReader(os.Stdin)
	if os.Getenv("OFFGRID_TEST_LIVE_PREFILL") == "1" {
		for i := 0; i < 19; i++ {
			fmt.Fprintf(os.Stdout, "Saved conversation line %02d\n", i+1)
		}
	}
	calls := 0
	drafts, err := followAgentLive(context.Background(), input, os.Stdout, reader, func(text string) (string, bool) {
		calls++
		expected := "shorter café"
		if stopping {
			expected = "/stop"
		}
		if text != expected {
			t.Errorf("unexpected correction: %q", text)
		}
		steered <- struct{}{}
		return "CORRECTION_ACCEPTED", true
	})
	if (!stopping && err != nil) || calls != 1 {
		t.Fatal(err, calls)
	}
	data, _ := json.Marshal(drafts)
	fmt.Printf("\nLIVE_DRAFTS:%s\nAPPROVAL_READY\n", data)
	approval, err := input.ReadString('\n')
	if err != nil || (approval != "y\n" && approval != "y\r\n") {
		t.Fatal("approval input lost", approval, err)
	}
	fmt.Println("APPROVAL_RECEIVED")
}
