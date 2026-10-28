package models

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

func scopedEvidenceFixture() CapabilityEvidence {
	e := capabilityEvidenceFixture()
	q := &WorkflowQualification{Scope: NativeWorkspaceScope, SuiteSHA256: NativeWorkspaceSuiteSHA256(), CapabilitySHA256: strings.Repeat("e", 64), ArtifactID: strings.Repeat("d", 64), Platform: "fixture", Cases: len(NativeWorkspaceCases()), Repetitions: 3, Verified: len(NativeWorkspaceCases()) * 3}
	for _, c := range NativeWorkspaceCases() {
		for n := 1; n <= 3; n++ {
			q.Results = append(q.Results, WorkflowCaseResult{CaseID: c.ID, Repetition: n, ArtifactID: strings.Repeat("f", 64), Verified: true})
		}
	}
	e.Qualification = q
	return e
}

func TestNativeWorkspaceSuiteFrozen(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range NativeWorkspaceCases() {
		if seen[c.ID] || c.ID == "" || c.Prompt == "" || c.Setup == "" || c.Check == "" {
			t.Fatal("incomplete frozen case", c.ID)
		}
		seen[c.ID] = true
	}
	if len(seen) != 19 {
		t.Fatal("frozen case count changed")
	}
	const frozen = "6e29f39924b673210d73b4e1910d5e02a99f24a52a28ae4a32b0581eabc26b38"
	if NativeWorkspaceSuiteSHA256() != frozen {
		t.Fatal("suite changed; publish a new scope version instead of reusing evidence")
	}
}

func TestNativeWorkspaceContractSuiteDoesNotReuseLegacyEvidence(t *testing.T) {
	const frozen = "28363d33e08946f33df2d770a168fce82b5f24e35b4c905a3d458cf2b44bbd13"
	if got := NativeWorkspaceSuiteSHA256ForScope(NativeWorkspaceContractScope); got != frozen {
		t.Fatalf("new suite identity: %s", got)
	}
	if got := NativeWorkspaceSuiteSHA256ForScope("unknown"); got != "" {
		t.Fatal("unknown scope gained identity")
	}
	cases, ok := NativeWorkspaceCasesForScope(NativeWorkspaceContractScope)
	if !ok || len(cases) != 19 || cases[18].Setup == NativeWorkspaceCases()[18].Setup {
		t.Fatal("new revision not distinguished")
	}
	e := scopedEvidenceFixture()
	e.Qualification.Scope = NativeWorkspaceContractScope
	if e.validate() == nil {
		t.Fatal("v1 suite qualified v2")
	}
	e.Qualification.SuiteSHA256 = NativeWorkspaceSuiteSHA256ForScope(NativeWorkspaceContractScope)
	if err := e.validate(); err != nil {
		t.Fatal(err)
	}
	s := NewCapabilityEvidenceStore(t.TempDir())
	if _, err := s.Record(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	q := e.Qualification
	target := CapabilityEvidenceTarget{Fingerprint: e.Fingerprint, SuiteSHA256: q.SuiteSHA256, Scope: q.Scope, Platform: q.Platform, CapabilitySHA256: q.CapabilitySHA256}
	view, err := s.Latest(context.Background(), e.ModelID, e.CheckID, target)
	if err != nil || view.State != "scope_qualified" {
		t.Fatal(view, err)
	}
	target.Scope, target.SuiteSHA256 = NativeWorkspaceScope, NativeWorkspaceSuiteSHA256()
	view, err = s.Latest(context.Background(), e.ModelID, e.CheckID, target)
	if err != nil || view.State != "stale" {
		t.Fatal("contracted evidence qualified legacy suite", view, err)
	}
}

func TestNativeWorkspaceQualificationRequiresCompleteIndependentCases(t *testing.T) {
	e := scopedEvidenceFixture()
	if err := e.validate(); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*WorkflowQualification){
		"unknown scope":      func(q *WorkflowQualification) { q.Scope = "native-everything" },
		"changed suite":      func(q *WorkflowQualification) { q.SuiteSHA256 = strings.Repeat("a", 64) },
		"no inventory":       func(q *WorkflowQualification) { q.CapabilitySHA256 = "" },
		"missing case":       func(q *WorkflowQualification) { q.Results = q.Results[1:] },
		"duplicate":          func(q *WorkflowQualification) { q.Results[1] = q.Results[0] },
		"unknown case":       func(q *WorkflowQualification) { q.Results[0].CaseID = "desktop" },
		"fourth repeat":      func(q *WorkflowQualification) { q.Results[0].Repetition = 4 },
		"no evidence":        func(q *WorkflowQualification) { q.Results[0].ArtifactID = "" },
		"aggregate mismatch": func(q *WorkflowQualification) { q.Verified-- },
		"unresolved failure": func(q *WorkflowQualification) { q.Results[0].Verified = false; q.Verified-- },
		"false completion":   func(q *WorkflowQualification) { q.Results[0].FalseCompletion = 1; q.FalseCompletion = 1 },
		"unauthorized":       func(q *WorkflowQualification) { q.Results[0].Unauthorized = 1; q.Unauthorized = 1 },
		"replay":             func(q *WorkflowQualification) { q.Results[0].RepeatedUncertain = 1; q.RepeatedUncertain = 1 },
	} {
		t.Run(name, func(t *testing.T) {
			bad := scopedEvidenceFixture()
			change(bad.Qualification)
			if bad.validate() == nil {
				t.Fatal("invalid scoped evidence accepted")
			}
		})
	}
	// Failed comparisons are preserved, not relabeled as successful qualification.
	e.Outcome = "failed"
	e.Qualification.Results[0].Verified = false
	e.Qualification.Verified--
	if err := e.validate(); err != nil {
		t.Fatal("failed evidence rejected", err)
	}
}

func TestScopedEvidenceCannotQualifyGeneralOrDifferentCatalog(t *testing.T) {
	e := scopedEvidenceFixture()
	s := NewCapabilityEvidenceStore(t.TempDir())
	ctx := context.Background()
	if _, err := s.Record(ctx, e); err != nil {
		t.Fatal(err)
	}
	q := e.Qualification
	target := CapabilityEvidenceTarget{Fingerprint: e.Fingerprint, SuiteSHA256: q.SuiteSHA256, Platform: q.Platform, Scope: q.Scope, CapabilitySHA256: q.CapabilitySHA256}
	view, err := s.Latest(ctx, e.ModelID, e.CheckID, target)
	if err != nil || view.State != "scope_qualified" {
		t.Fatal(view, err)
	}
	for _, change := range []func(*CapabilityEvidenceTarget){func(x *CapabilityEvidenceTarget) { x.Scope = "" }, func(x *CapabilityEvidenceTarget) { x.CapabilitySHA256 = strings.Repeat("a", 64) }} {
		changed := target
		change(&changed)
		view, err = s.Latest(ctx, e.ModelID, e.CheckID, changed)
		if err != nil || view.State != "stale" || view.Evidence == nil {
			t.Fatal(view, err)
		}
	}
}

func TestCapabilityFingerprintHistoricalSerializationAndRuntimeContent(t *testing.T) {
	f := capabilityEvidenceFixture().Fingerprint
	// Exact historical field order/omissions determine persisted fingerprint IDs.
	old := `{"model_sha256":"` + f.ModelSHA256 + `","quantization":"q4","runtime_build":"test-build","template_sha256":"` + f.TemplateSHA256 + `","adapter":"single-step-v1","context_window":8192,"backend":"cpu","hardware_id":"fixture-host-v1"}`
	encoded, _ := json.Marshal(f)
	if string(encoded) != old {
		t.Fatal("historical serialization changed", string(encoded))
	}
	want := sha256.Sum256([]byte(old))
	id, err := f.ID()
	if err != nil || id != hex.EncodeToString(want[:]) {
		t.Fatal(id, err)
	}
	for _, change := range []func(*CapabilityFingerprint){func(x *CapabilityFingerprint) { x.RuntimeSHA256 = strings.Repeat("e", 64) }, func(x *CapabilityFingerprint) { x.RuntimeConfigSHA256 = strings.Repeat("f", 64) }} {
		changed := f
		change(&changed)
		other, err := changed.ID()
		if err != nil || id == other {
			t.Fatal("runtime identity ignored", err)
		}
	}
	for _, bad := range []string{"name", "ABC", strings.Repeat("A", 64)} {
		changed := f
		changed.RuntimeSHA256 = bad
		if _, err := changed.ID(); err == nil {
			t.Fatal("invalid runtime digest accepted")
		}
		changed = f
		changed.RuntimeConfigSHA256 = bad
		if _, err := changed.ID(); err == nil {
			t.Fatal("invalid configuration digest accepted")
		}
	}
}
