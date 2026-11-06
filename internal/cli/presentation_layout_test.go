package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestPresentationRowsKeepExactIDsAndSeparateDetails(t *testing.T) {
	longID := "NVIDIA-Nemotron-3.5-Bolt-E4B-UD-Q4_K_XL-47d072be609f"
	for _, columns := range []int{32, 60, 100} {
		t.Run(fmt.Sprint(columns), func(t *testing.T) {
			var out bytes.Buffer
			p := Presenter{Stdout: &out, Style: PresentationStyle{Columns: columns}}
			p.rows([]presentationRow{{longID, "4.7 GB"}, {"日本語 café", "2.0 GB"}})
			got := out.String()
			if !strings.Contains(got, longID) || strings.Contains(got, longID+"4.7") || strings.Contains(got, "\x1b") {
				t.Fatal(got)
			}
			if columns < 70 && !strings.Contains(got, longID+"\n    4.7 GB") {
				t.Fatal("details must move below long IDs", got)
			}
			if columns == 100 {
				lines := strings.Split(strings.TrimSuffix(got, "\n"), "\n")
				left := strings.Split(lines[0], "4.7 GB")[0]
				right := strings.Split(lines[1], "2.0 GB")[0]
				if ansi.StringWidth(left) != ansi.StringWidth(right) {
					t.Fatal("Unicode columns misaligned", got)
				}
			}
		})
	}
}

func TestInfoHumanExplainsConfiguredValuesWithoutDimming(t *testing.T) {
	var out, diagnostic bytes.Buffer
	p := Presenter{Stdout: &out, Stderr: &diagnostic, Style: PresentationStyle{Dim: "<dim>", Muted: "<muted>", Columns: 60}}
	snapshot := InfoSnapshot{Version: "test", MaxContextSize: 65536, MaxMemoryMB: 4096,
		ModelsDir: "fixture", Models: []InfoModel{{ID: "z-Q4_K_M", Quantization: "Q4_K_M", Size: 1024}, {ID: "a-model", Size: 2048}}}
	if err := p.InfoSnapshot(snapshot); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	readable := strings.Join(strings.Fields(got), " ")
	for _, want := range []string{"Local configuration", "live service allocations may differ", "Automatic", "65,536 tokens (configured)", "4,096 MB (configured)", "offgrid doctor", "offgrid agent"} {
		if !strings.Contains(readable, want) {
			t.Fatalf("missing %q: %s", want, got)
		}
	}
	if strings.Contains(got, "<dim>") || strings.Contains(got, "<muted>") || strings.Count(got, "Q4_K_M") != 1 {
		t.Fatal(got)
	}
	if strings.Index(got, "a-model") > strings.Index(got, "z-Q4_K_M") || snapshot.Models[0].ID != "z-Q4_K_M" {
		t.Fatal("sort changed snapshot or order", got)
	}
}

func TestModelListHumanAndJSONShareExactServiceIdentity(t *testing.T) {
	const id = "model-with-a-very-long-identity-and-specific-quantization-Q4_K_M-123456789"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"data":[{"id":%q,"size":1024}]}`, id)
	}))
	defer server.Close()
	for _, machine := range []bool{false, true} {
		var out bytes.Buffer
		p := Presenter{Stdout: &out, JSON: machine, Style: PresentationStyle{Columns: 32}}
		if err := RunList(context.Background(), nil, p, server.URL, "", server.Client()); err != nil {
			t.Fatal(err)
		}
		if machine {
			var result struct {
				Models []struct{ Name string }
				Count  int
			}
			if err := json.Unmarshal(out.Bytes(), &result); err != nil || result.Count != 1 || result.Models[0].Name != id {
				t.Fatal(err, out.String())
			}
		} else if !strings.Contains(out.String(), id+"\n    1.0 KB") || !strings.Contains(out.String(), server.URL) {
			t.Fatal(out.String())
		}
	}
}

func TestLayoutSanitizesNamesAndSuppressesDecorationsForJSON(t *testing.T) {
	var out bytes.Buffer
	p := Presenter{Stdout: &out, Style: PresentationStyle{Columns: 32}}
	p.Heading("unsafe\x1b[2J\nname")
	p.Detail("Model", "a\tmodel\x00")
	p.rows([]presentationRow{{"line\nspoof", "detail\x1b[2J"}})
	if strings.ContainsAny(out.String(), "\x1b\t\x00") || strings.Contains(out.String(), "\nspoof") {
		t.Fatal(out.String())
	}
	out.Reset()
	p.JSON = true
	p.Heading("hidden")
	p.Note("hidden")
	p.Detail("hidden", "hidden")
	p.rows([]presentationRow{{"hidden", "hidden"}})
	if out.Len() != 0 {
		t.Fatal(out.String())
	}
}

func TestRecoveryCopyDoesNotInventPlatformOrTimeoutCause(t *testing.T) {
	var out, diagnostic bytes.Buffer
	p := Presenter{Stdout: &out, Stderr: &diagnostic}
	for _, message := range []string{"connection refused", "context deadline exceeded", "permission denied", "out of memory", "429"} {
		p.HelpfulError(errors.New(message), "Request")
	}
	for _, forbidden := range []string{"systemctl", "chmod", "lsof", "HuggingFace servers overloaded", "docs/4GB_RAM", "HuggingFace rate limit"} {
		if strings.Contains(out.String(), forbidden) {
			t.Fatal(forbidden, out.String())
		}
	}
	if !strings.Contains(out.String(), "Inspect saved work before retrying") {
		t.Fatal(out.String())
	}
}

func TestBoxLongUnicodeTitleFitsWithoutPanic(t *testing.T) {
	var out bytes.Buffer
	p := Presenter{Stdout: &out, Style: PresentationStyle{Columns: 32, BoxTL: "+", BoxTR: "+", BoxBL: "+", BoxBR: "+", BoxVR: "+", BoxVL: "+", BoxV: "|", BoxH: "-"}}
	p.Box(strings.Repeat("日本語 ", 25), strings.Repeat("long content ", 20))
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if ansi.StringWidth(line) > 32 {
			t.Fatalf("box overflow: %q", line)
		}
	}
	out.Reset()
	p.JSON = true
	p.Box("hidden", "hidden")
	if out.Len() != 0 {
		t.Fatal(out.String())
	}
}
