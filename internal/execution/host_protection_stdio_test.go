package execution

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/storage"
)

func TestStdioProtectionAcknowledgmentFencesRejectedStartup(t *testing.T) {
	for _, kind := range []string{"valid", "missing", "foreign lease", "foreign token", "partial", "oversized", "dispatch before ack"} {
		t.Run(kind, func(t *testing.T) {
			w, lease, registry, _ := protectionHandoffFixture(t)
			var input bytes.Buffer
			ack := protectionStartupAck{Version: 1, Kind: "host_protection_ack", Lease: lease.id, Token: w.Token}
			switch kind {
			case "valid":
				_ = WriteFrame(&input, ack)
			case "foreign lease":
				ack.Lease = strings.Repeat("a", 64)
				_ = WriteFrame(&input, ack)
			case "foreign token":
				ack.Token = strings.Repeat("c", 64)
				_ = WriteFrame(&input, ack)
			case "partial":
				_ = WriteFrame(&input, map[string]any{"version": 1, "kind": "host_protection_ack"})
			case "oversized":
				var header [4]byte
				binary.BigEndian.PutUint32(header[:], 1025)
				input.Write(header[:])
			case "dispatch before ack":
				_ = WriteFrame(&input, Request{Version: 1, ID: "premature", OperationID: "premature", Binding: w.Session.config.Binding, Token: w.Token, Operation: "file_write", Arguments: json.RawMessage(`{"path":"must-not-exist","data":"eA=="}`)})
			}
			var output bytes.Buffer
			err := w.ServeProtectedRecoverable(context.Background(), io.NopCloser(&input), &output, lease)
			if kind == "valid" && err != nil || kind != "valid" && !errors.Is(err, ErrHostProtectionUncertain) {
				t.Fatal("startup/cleanup outcome", err)
			}
			_, err = registry.RegisterBrowserRoot(context.Background(), w.Session.config.Root)
			if kind == "valid" && err != nil || kind != "valid" && !errors.Is(err, ErrHostProtectionConflict) {
				t.Fatal("rejected startup discarded fence", err)
			}
			if _, err = os.Stat(filepath.Join(w.Session.config.Root, "must-not-exist")); !os.IsNotExist(err) {
				t.Fatal("dispatched before acknowledgment", err)
			}
		})
	}
}

func protectionHandoffFixture(t *testing.T) (*Worker, *HostProtectionLease, *HostProtectionRegistry, *protectionHandoff) {
	t.Helper()
	if !nativeCleanupVerified(nil) {
		t.Skip("native cleanup proof unavailable on this platform")
	}
	w, lease, registry := protectedWorkerFixture(t)
	data, err := protectedStartup(w, lease)
	if err != nil {
		t.Fatal(err)
	}
	var startup protectionStartup
	if err = decodeStrict(data, &startup); err != nil {
		t.Fatal(err)
	}
	return w, lease, registry, startup.Protection
}

func TestStdioProtectionHandoffRejectsForeignAuthorityWithoutMutation(t *testing.T) {
	w, _, registry, handoff := protectionHandoffFixture(t)
	before := protectionInspectFiles(t, registry.directory)
	cases := map[string]func(*protectionHandoff){
		"foreign capability":    func(h *protectionHandoff) { h.Capability = strings.Repeat("c", 64) },
		"foreign reservation":   func(h *protectionHandoff) { h.Lease = strings.Repeat("d", 64) },
		"foreign configuration": func(h *protectionHandoff) { h.ConfigHash = strings.Repeat("e", 64) },
		"changed journal":       func(h *protectionHandoff) { h.JournalIdentity += "-changed" },
		"partial":               func(h *protectionHandoff) { h.Capability = "" },
		"unsupported version":   func(h *protectionHandoff) { h.Version++ },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			value := *handoff
			mutate(&value)
			if _, err := registry.acceptProtectionHandoff(context.Background(), &value, w.Session.config, w.Journal.directory); err == nil {
				t.Fatal("foreign/partial authority accepted")
			}
		})
	}
	if after := protectionInspectFiles(t, registry.directory); !reflect.DeepEqual(before, after) {
		t.Fatal("rejected handoff mutated registry")
	}
	if _, err := registry.RegisterBrowserRoot(context.Background(), w.Session.config.Root); !errors.Is(err, ErrHostProtectionConflict) {
		t.Fatal("handoff released reservation", err)
	}
}

func TestStdioProtectionStartupPreservesLegacyAndRejectsPresentMalformed(t *testing.T) {
	identity := workerIdentity("development")
	data, _ := json.Marshal(identity)
	endpoint := &StdioEndpoint{}
	if err := endpoint.acceptProtectionStartup(context.Background(), data, Context{}, ""); err != nil || endpoint.identity != identity || endpoint.protection != nil {
		t.Fatal("legacy startup changed", err)
	}
	for _, extra := range []string{`"host_protection":null`, `"host_protection":{}`, `"host_protection":{"version":1,"unknown":true}`} {
		value := append(append([]byte{}, data[:len(data)-1]...), []byte(","+extra+"}")...)
		if err := (&StdioEndpoint{}).acceptProtectionStartup(context.Background(), value, Context{}, ""); err == nil {
			t.Fatal("malformed present handoff accepted")
		}
	}
	if err := endpoint.RetryProtectionSettlement(context.Background()); !errors.Is(err, ErrUnsupported) {
		t.Fatal("legacy metadata reconstructed authority", err)
	}
}

func TestStdioProtectionSettlementNeedsProofAndRetriesOnlyPublication(t *testing.T) {
	w, _, registry, handoff := protectionHandoffFixture(t)
	owner, err := registry.acceptProtectionHandoff(context.Background(), handoff, w.Session.config, w.Journal.directory)
	if err != nil {
		t.Fatal(err)
	}
	endpoint := &StdioEndpoint{protection: owner}
	if err = owner.lease.CancelUnactivated(context.Background()); !errors.Is(err, ErrAuthority) {
		t.Fatal("handoff acquired unactivated refund", err)
	}
	for _, proofErr := range []error{nil, ErrHostUnavailable} {
		if proofErr != nil {
			endpoint.recordProtectionCleanup(proofErr)
		}
		if err = endpoint.RetryProtectionSettlement(context.Background()); !errors.Is(err, ErrHostProtectionUncertain) {
			t.Fatal("absence/failure of cleanup was proof", err)
		}
	}
	db, err := storage.OpenSQLite(filepath.Join(registry.directory, "protection.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TRIGGER reject_release BEFORE UPDATE ON host_protection_records BEGIN SELECT RAISE(ABORT,'fixture publication failure'); END`); err != nil {
		t.Fatal(err)
	}
	endpoint.recordProtectionCleanup(nil) // Unit proof seam; native proof is tested separately.
	if err = endpoint.RetryProtectionSettlement(context.Background()); !errors.Is(err, ErrHostProtectionStorage) {
		t.Fatal("failed publication accepted", err)
	}
	if _, err = registry.RegisterBrowserRoot(context.Background(), w.Session.config.Root); !errors.Is(err, ErrHostProtectionConflict) {
		t.Fatal("failed publication removed fence", err)
	}
	if _, err = db.Exec(`DROP TRIGGER reject_release`); err != nil {
		t.Fatal(err)
	}
	if err = endpoint.RetryProtectionSettlement(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Reclamation cannot make an old endpoint act on a newly admitted lease.
	foreignRoot := protectionTestRoot(t)
	if _, err = registry.ReserveExecution(context.Background(), protectionTestConfig(foreignRoot, "foreign-survivor"), protectionTestRoot(t)); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`DELETE FROM host_protection_records WHERE id=?`, handoff.Lease); err != nil {
		t.Fatal(err)
	}
	if err = endpoint.RetryProtectionSettlement(context.Background()); err != nil {
		t.Fatal("retired success lost", err)
	}
	if _, err = registry.RegisterBrowserRoot(context.Background(), foreignRoot); !errors.Is(err, ErrHostProtectionConflict) {
		t.Fatal("old capability released foreign row", err)
	}
	if _, err = registry.RegisterBrowserRoot(context.Background(), w.Session.config.Root); err != nil {
		t.Fatal("verified settlement did not unblock original workspace", err)
	}
}

func TestStdioProtectionMissingOrReplacedJournalDoesNotRelease(t *testing.T) {
	for _, replace := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing", true: "replacement"}[replace], func(t *testing.T) {
			w, _, registry, handoff := protectionHandoffFixture(t)
			owner, err := registry.acceptProtectionHandoff(context.Background(), handoff, w.Session.config, w.Journal.directory)
			if err != nil {
				t.Fatal(err)
			}
			endpoint := &StdioEndpoint{protection: owner}
			endpoint.recordProtectionCleanup(nil)
			if err = w.Journal.Close(); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(w.Journal.directory, "execution-dispatch.sqlite")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.Rename(path, path+".original"); err != nil {
				t.Fatal(err)
			}
			if replace {
				if err = os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
				if err = ProtectPrivatePath(path); err != nil {
					t.Fatal(err)
				}
			}
			if err = endpoint.RetryProtectionSettlement(context.Background()); !errors.Is(err, ErrHostProtectionChanged) {
				t.Fatal("lost journal accepted", err)
			}
			if !replace {
				if _, err = os.Lstat(path); !os.IsNotExist(err) {
					t.Fatal("settlement created missing database", err)
				}
			}
			if _, err = registry.RegisterBrowserRoot(context.Background(), w.Session.config.Root); !errors.Is(err, ErrHostProtectionConflict) {
				t.Fatal("lost journal removed fence", err)
			}
		})
	}
}

func TestStdioProtectionSecretsRemainPrivate(t *testing.T) {
	w, _, registry, handoff := protectionHandoffFixture(t)
	owner, err := registry.acceptProtectionHandoff(context.Background(), handoff, w.Session.config, w.Journal.directory)
	if err != nil {
		t.Fatal(err)
	}
	report, err := registry.Inspect(context.Background(), "", 64)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []any{report, workerIdentity("development"), &StdioEndpoint{protection: owner}} {
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		for _, secret := range []string{handoff.Capability, handoff.ConfigHash, handoff.JournalIdentity} {
			if strings.Contains(string(data), secret) {
				t.Fatal("private handoff leaked in public projection")
			}
		}
	}
	t.Setenv(protectionRecoveryEnvironment, "1")
	for _, entry := range processEnvironment() {
		if strings.HasPrefix(entry, protectionRecoveryEnvironment+"=") {
			t.Fatal("private startup mode forwarded to project command")
		}
	}
	isolated := w.Session.config
	isolated.Backend = "isolated_linux"
	if _, err = registry.acceptProtectionHandoff(context.Background(), handoff, isolated, w.Journal.directory); !errors.Is(err, ErrAuthority) {
		t.Fatal("isolated worker acquired local proof", err)
	}
}
