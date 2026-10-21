package models

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func capabilityEvidenceFixture() CapabilityEvidence {
	return CapabilityEvidence{Version: 1, ModelID: "fixture", CheckID: "native-tool-v1", Outcome: "passed", Code: "tool_check_passed", RecordedAt: time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC), Fingerprint: CapabilityFingerprint{
		ModelSHA256: strings.Repeat("a", 64), TemplateSHA256: strings.Repeat("b", 64), Quantization: "q4", RuntimeBuild: "test-build", Adapter: "single-step-v1", ContextWindow: 8192, Backend: "cpu", HardwareID: "fixture-host-v1",
	}}
}

func TestCapabilityEvidenceRestartAndOperationalState(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store := NewCapabilityEvidenceStore(dir)
	e := capabilityEvidenceFixture()
	target := CapabilityEvidenceTarget{Fingerprint: e.Fingerprint}
	view, err := store.Latest(ctx, e.ModelID, e.CheckID, target)
	if err != nil || view.State != "unmeasured" {
		t.Fatalf("new: %+v %v", view, err)
	}
	id, err := store.Record(ctx, e)
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := store.Record(ctx, e)
	if err != nil || duplicate != id {
		t.Fatal("record is not idempotent", err)
	}
	store = NewCapabilityEvidenceStore(dir)
	view, err = store.Latest(ctx, e.ModelID, e.CheckID, target)
	if err != nil || view.State != "check_passed" || view.EvidenceID != id {
		t.Fatalf("restart: %+v %v", view, err)
	}
	for _, outcome := range []string{"failed", "unavailable"} {
		e.RecordedAt = e.RecordedAt.Add(time.Second)
		e.Outcome = outcome
		e.Code = "fixture_" + outcome
		if _, err = store.Record(ctx, e); err != nil {
			t.Fatal(err)
		}
		view, err = store.Latest(ctx, e.ModelID, e.CheckID, target)
		want := "check_failed"
		if outcome == "unavailable" {
			want = "unmeasured"
		}
		if err != nil || view.State != want {
			t.Fatalf("%s: %+v %v", outcome, view, err)
		}
	}
	db, err := store.open()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count int
	if err = db.QueryRow("SELECT COUNT(*) FROM capability_evidence").Scan(&count); err != nil || count != 3 {
		t.Fatalf("history/idempotence: %d %v", count, err)
	}
}

func TestCapabilityEvidenceFingerprintInvalidation(t *testing.T) {
	store := NewCapabilityEvidenceStore(t.TempDir())
	e := capabilityEvidenceFixture()
	if _, err := store.Record(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	changes := map[string]func(*CapabilityFingerprint){
		"model":      func(f *CapabilityFingerprint) { f.ModelSHA256 = strings.Repeat("c", 64) },
		"template":   func(f *CapabilityFingerprint) { f.TemplateSHA256 = strings.Repeat("d", 64) },
		"projector":  func(f *CapabilityFingerprint) { f.ProjectorSHA256 = strings.Repeat("e", 64) },
		"preprocess": func(f *CapabilityFingerprint) { f.PreprocessSHA256 = strings.Repeat("f", 64) },
		"quant":      func(f *CapabilityFingerprint) { f.Quantization = "q8" },
		"build":      func(f *CapabilityFingerprint) { f.RuntimeBuild = "new" },
		"adapter":    func(f *CapabilityFingerprint) { f.Adapter = "native-v1" },
		"context":    func(f *CapabilityFingerprint) { f.ContextWindow = 16384 },
		"backend":    func(f *CapabilityFingerprint) { f.Backend = "cuda" },
		"hardware":   func(f *CapabilityFingerprint) { f.HardwareID = "other-host" },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			fingerprint := e.Fingerprint
			change(&fingerprint)
			view, err := store.Latest(context.Background(), e.ModelID, e.CheckID, CapabilityEvidenceTarget{Fingerprint: fingerprint})
			if err != nil || view.State != "stale" || view.Evidence == nil {
				t.Fatalf("stale history lost: %+v %v", view, err)
			}
		})
	}
	view, err := store.Latest(context.Background(), e.ModelID, e.CheckID, CapabilityEvidenceTarget{})
	if err != nil || view.State != "unmeasured" {
		t.Fatalf("incomplete current identity reused evidence: %+v %v", view, err)
	}
	e.Fingerprint.ModelSHA256 = ""
	if _, err := store.Record(context.Background(), e); err == nil {
		t.Fatal("missing identity accepted")
	}
}

func TestCapabilityEvidenceQualificationCannotComeFromSmoke(t *testing.T) {
	store := NewCapabilityEvidenceStore(t.TempDir())
	e := capabilityEvidenceFixture()
	ctx := context.Background()
	e.CheckID = "workflow-v1"
	e.Qualification = &WorkflowQualification{SuiteSHA256: strings.Repeat("c", 64), Platform: "windows-x64", ArtifactID: strings.Repeat("d", 64), Cases: 30, Repetitions: 3, Verified: 81}
	target := CapabilityEvidenceTarget{Fingerprint: e.Fingerprint, SuiteSHA256: e.Qualification.SuiteSHA256, Platform: e.Qualification.Platform}
	if _, err := store.Record(ctx, e); err != nil {
		t.Fatal(err)
	}
	view, err := store.Latest(ctx, e.ModelID, e.CheckID, target)
	if err != nil || view.State != "workflow_qualified" {
		t.Fatalf("qualification: %+v %v", view, err)
	}
	for _, change := range []func(*CapabilityEvidenceTarget){func(v *CapabilityEvidenceTarget) { v.SuiteSHA256 = strings.Repeat("e", 64) }, func(v *CapabilityEvidenceTarget) { v.Platform = "linux-wayland" }} {
		changed := target
		change(&changed)
		view, err = store.Latest(ctx, e.ModelID, e.CheckID, changed)
		if err != nil || view.State != "stale" {
			t.Fatalf("wrong suite/platform reused: %+v %v", view, err)
		}
	}
	for _, change := range []func(*WorkflowQualification){func(q *WorkflowQualification) { q.Verified = 80 }, func(q *WorkflowQualification) { q.Unauthorized = 1 }, func(q *WorkflowQualification) { q.FalseCompletion = 1 }, func(q *WorkflowQualification) { q.RepeatedUncertain = 1 }, func(q *WorkflowQualification) { q.Repetitions = 1 }} {
		bad := e
		q := *e.Qualification
		bad.Qualification = &q
		change(&q)
		if _, err := store.Record(ctx, bad); err == nil {
			t.Fatal("failed qualification accepted as passed")
		}
	}
}

func TestCapabilityEvidenceUnknownVersionAndCorruption(t *testing.T) {
	for _, kind := range []string{"record", "database", "digest"} {
		t.Run(kind, func(t *testing.T) {
			store := NewCapabilityEvidenceStore(t.TempDir())
			e := capabilityEvidenceFixture()
			ctx := context.Background()
			if _, err := store.Record(ctx, e); err != nil {
				t.Fatal(err)
			}
			db, err := store.open()
			if err != nil {
				t.Fatal(err)
			}
			if kind == "database" {
				_, err = db.Exec("PRAGMA user_version=99")
			} else {
				e.Version = 99
				data, _ := json.Marshal(e)
				digest := sha256.Sum256(data)
				id := hex.EncodeToString(digest[:])
				if kind == "digest" {
					id = strings.Repeat("0", 64)
				}
				_, err = db.Exec("UPDATE capability_evidence SET id=?,snapshot=?", id, data)
			}
			db.Close()
			if err != nil {
				t.Fatal(err)
			}
			view, err := store.Latest(ctx, e.ModelID, e.CheckID, CapabilityEvidenceTarget{Fingerprint: e.Fingerprint})
			if err == nil || view.State != "unmeasured" {
				t.Fatalf("corruption became usable: %+v %v", view, err)
			}
			if kind != "digest" && !errors.Is(err, ErrCapabilityEvidenceVersion) {
				t.Fatalf("wrong version error: %v", err)
			}
			if kind == "database" {
				e.Version = 1
				if _, err = store.Record(ctx, e); !errors.Is(err, ErrCapabilityEvidenceVersion) {
					t.Fatal("future database overwritten", err)
				}
			}
		})
	}
}

func TestCapabilityEvidencePersistenceFailureAndConcurrentRecords(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "model-capabilities")
	if err := os.WriteFile(blocker, []byte("sentinel"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewCapabilityEvidenceStore(dir).Record(context.Background(), capabilityEvidenceFixture()); err == nil {
		t.Fatal("failed persistence reported success")
	}
	data, _ := os.ReadFile(blocker)
	if string(data) != "sentinel" {
		t.Fatal("unrelated blocker changed")
	}
	store := NewCapabilityEvidenceStore(t.TempDir())
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			e := capabilityEvidenceFixture()
			e.RecordedAt = e.RecordedAt.Add(time.Duration(i) * time.Second)
			if _, err := store.Record(context.Background(), e); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	db, err := store.open()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count int
	if err = db.QueryRow("SELECT COUNT(*) FROM capability_evidence").Scan(&count); err != nil || count != 6 {
		t.Fatalf("lost concurrent evidence: %d %v", count, err)
	}
}
