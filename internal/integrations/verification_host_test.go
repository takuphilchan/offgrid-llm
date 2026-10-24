package integrations

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/integrations/verification"
)

type verifiedFixture struct {
	*fixtureHost
	events bool
	seen   *verification.Connection
}

func (*verifiedFixture) CheckVerification(context.Context, HostOptions, HostState, []string) (string, error) {
	return verification.HermesAdapter, nil
}
func (f *verifiedFixture) Run(ctx context.Context, o HostOptions, _ HostState, _ SetupResponse, _ []string) error {
	f.seen = o.Verification
	if f.events {
		for _, path := range []string{"/bind", "/end"} {
			c := o.Verification
			event := map[string]any{"version": 1, "adapter": c.Adapter, "run": c.RunID, "session": "s"}
			if path == "/end" {
				event["status"] = "completed"
			}
			body, _ := json.Marshal(event)
			r, _ := http.NewRequestWithContext(ctx, "POST", c.URL+path, bytes.NewReader(body))
			r.Header.Set("Authorization", "Bearer "+c.Token)
			r.Header.Set("Content-Type", "application/json")
			res, err := http.DefaultClient.Do(r)
			if err != nil {
				return err
			}
			res.Body.Close()
		}
	}
	return f.runError
}

func TestExplicitVerificationFailsClosedAndDoesNotPersistSecrets(t *testing.T) {
	for _, mode := range []string{"missing", "passed", "external_error", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			m, host, _, o := managerFixture(t)
			f := &verifiedFixture{fixtureHost: host, events: mode == "passed"}
			m.Adapters["fixture-third"] = f
			if mode == "external_error" {
				f.runError = errors.New("child exit")
			}
			if mode == "cancelled" {
				f.runError = context.Canceled
			}
			dir := t.TempDir()
			contract := verification.Contract{Version: 1, Kind: verification.NoChanges, Workspace: dir}
			data, _ := json.Marshal(contract)
			file := filepath.Join(t.TempDir(), "contract.json")
			if err := os.WriteFile(file, data, 0600); err != nil {
				t.Fatal(err)
			}
			o.VerifyContract = file
			r, err := m.Run(context.Background(), "fixture-third", o, []string{"hello"})
			if (err == nil) != (mode == "passed") || r.Verification == nil {
				t.Fatalf("%+v %v", r, err)
			}
			if f.seen == nil || f.seen.Workspace != dir {
				t.Fatal("missing scoped invocation")
			}
			report, _ := json.Marshal(r)
			if strings.Contains(string(report), f.seen.Token) || strings.Contains(string(report), f.seen.URL) {
				t.Fatal("secret leaked")
			}
			if mode == "cancelled" && !errors.Is(err, context.Canceled) {
				t.Fatal("lost cancellation")
			}
		})
	}
}

func TestVerificationOptionParsing(t *testing.T) {
	o, args, err := ParseHostOptions([]string{"hello", "--verify-contract=contract.json"}, HostOptions{}, true)
	if err != nil || o.VerifyContract != "contract.json" || len(args) != 1 {
		t.Fatalf("%+v %v %v", o, args, err)
	}
	for _, args := range [][]string{{"--verify-contract"}, {"--verify-contract", "a", "--verify-contract", "b"}} {
		if _, _, err := ParseHostOptions(args, HostOptions{}, true); err == nil {
			t.Fatal("accepted", args)
		}
	}
	if _, _, err := ParseHostOptions([]string{"--verify-contract", "a"}, HostOptions{}, false); err == nil {
		t.Fatal("allowed non-run contract")
	}
}
