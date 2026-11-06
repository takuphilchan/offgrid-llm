package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

func TestCommandHelpLayoutsAtNarrowAndWideWidths(t *testing.T) {
	for _, width := range []int{40, 80, 120} {
		for name, render := range map[string]func(Presenter){
			"knowledge": Presenter.knowledgeHelp, "models": Presenter.modelPackageHelp,
			"speech": Presenter.audioHelp, "users": Presenter.usersHelp,
			"audit": Presenter.auditHelp, "discovery": Presenter.searchHelp,
		} {
			var out bytes.Buffer
			render(Presenter{Stdout: &out, Style: PresentationStyle{Columns: width}})
			if !utf8.Valid(out.Bytes()) || strings.ContainsAny(out.String(), "\x1b\r") {
				t.Fatalf("%s invalid plain output", name)
			}
			for _, line := range strings.Split(out.String(), "\n") {
				// Exact commands soft-wrap; prose must fit the selected viewport.
				if ansi.StringWidth(line) > width && !strings.Contains(line, "offgrid ") {
					t.Errorf("%s width %d: %q", name, width, line)
				}
			}
		}
	}
}

func TestDoctorDoesNotDeclareUnreachableServiceOrFailedScanReady(t *testing.T) {
	var out bytes.Buffer
	p := Presenter{Stdout: &out}
	p.Doctor(DoctorSnapshot{StoragePresent: true, ModelScanError: errors.New("scan failed")})
	for _, want := range []string{"scan failed", "Not reachable", "2 check(s) need attention", "not tested"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q: %s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "All checks passed") {
		t.Fatal("false readiness")
	}
}

func TestHelpDoesNotRequireStoresOrExecuteActions(t *testing.T) {
	for _, command := range []string{"users", "model", "kb", "audio", "integration", "batch", "search", "logs"} {
		var out, errOut bytes.Buffer
		env := testEnvironment(&out, &errOut, &fakeRunner{})
		if code := Execute(context.Background(), []string{command, "--help"}, env); code != 0 || out.Len() == 0 {
			t.Errorf("%s: code %d, output %q, error %q", command, code, out.String(), errOut.String())
		}
	}
}

func TestAllSharedDecorativeHelpersSuppressJSON(t *testing.T) {
	var out bytes.Buffer
	p := Presenter{Stdout: &out, JSON: true}
	p.Item("private", "data")
	p.Option("--flag", "description")
	p.Example("command")
	p.ModernSection("heading", [][]string{{"key", "value"}})
	p.Divider()
	p.KeyValue("key", "value")
	if out.Len() != 0 {
		t.Fatalf("decorations in machine output: %q", out.String())
	}
}
