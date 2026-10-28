package models_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/internal/storage"
)

// Bootstrap only an explicitly disposable, stopped evaluator service. This
// synthetic record exercises the packaged service's real measurement gate; it
// is NOT qualification evidence and must never be copied into installed state.
// The test binary is not shipped in the application or exposed through an API.
func TestSeedIsolatedNativeEvaluation(t *testing.T) {
	dir := os.Getenv("OFFGRID_TEST_ISOLATED_EVALUATION_DIR")
	if dir == "" {
		t.Skip("requires a stopped disposable evaluation service")
	}
	if !filepath.IsAbs(dir) || filepath.Clean(dir) != dir {
		t.Fatal("absolute clean isolated data directory required")
	}
	marker, err := os.ReadFile(filepath.Join(dir, ".isolated-native-evaluation"))
	if err != nil || string(marker) != "isolated-candidate-only-v1\n" {
		t.Fatal("explicit disposable evaluator marker missing")
	}
	owner, err := storage.AcquireOwnership(dir)
	if err != nil {
		t.Fatal("evaluation service must be stopped", err)
	}
	defer owner.Close()
	data, err := os.ReadFile(filepath.Join(dir, "measured-runtime.json"))
	if err != nil {
		t.Fatal(err)
	}
	var measured struct {
		ModelID       string                       `json:"model_id"`
		Fingerprint   models.CapabilityFingerprint `json:"fingerprint"`
		RuntimeFiles  map[string]string            `json:"runtime_files"`
		Platform      string                       `json:"platform"`
		Qualification bool                         `json:"qualification"`
	}
	if json.Unmarshal(data, &measured) != nil || measured.Qualification || measured.ModelID == "" || measured.Platform == "" {
		t.Fatal("invalid measurement-only fixture")
	}
	if _, err := measured.Fingerprint.ID(); err != nil || measured.Fingerprint.Adapter != inference.NativeAgentToolAdapter {
		t.Fatal("invalid measured native adapter identity", err)
	}
	const scope = models.NativeWorkspaceContractScope
	cases, _ := models.NativeWorkspaceCasesForScope(scope)
	sum := sha256.Sum256([]byte("SYNTHETIC ISOLATED EVALUATION BOOTSTRAP; NOT QUALIFICATION\n" + string(data)))
	artifact := hex.EncodeToString(sum[:])
	q := &models.WorkflowQualification{Scope: scope, SuiteSHA256: models.NativeWorkspaceSuiteSHA256ForScope(scope), CapabilitySHA256: agents.NativeWorkspaceCapabilitySHA256(), Platform: measured.Platform, ArtifactID: artifact, Repetitions: 3, Cases: len(cases), Verified: len(cases) * 3}
	for _, c := range cases {
		for repetition := 1; repetition <= 3; repetition++ {
			q.Results = append(q.Results, models.WorkflowCaseResult{CaseID: c.ID, Repetition: repetition, ArtifactID: artifact, Verified: true})
		}
	}
	inventoryDir := filepath.Join(dir, "model-capabilities")
	if _, err := os.Stat(inventoryDir); !os.IsNotExist(err) {
		t.Fatal("refusing to overwrite existing capability state")
	}
	store := models.NewCapabilityEvidenceStore(dir)
	evidence, err := store.Record(context.Background(), models.CapabilityEvidence{Version: models.CapabilityEvidenceVersion, ModelID: measured.ModelID, CheckID: inference.NativeAgentToolAdapter, Fingerprint: measured.Fingerprint, Outcome: "passed", Code: "isolated_candidate_fixture_not_qualification", RecordedAt: time.Now(), Qualification: q})
	if err != nil {
		t.Fatal(err)
	}
	inventory := map[string]any{"version": 1, "models": []any{map[string]any{"model_id": measured.ModelID, "fingerprint": measured.Fingerprint, "runtime_files": measured.RuntimeFiles, "scope": scope, "suite_sha256": q.SuiteSHA256, "capability_sha256": q.CapabilitySHA256, "platform": measured.Platform}}}
	for name, value := range map[string]any{"model-capabilities/native-agent-installations.json": inventory, "evaluation-suite.json": cases, "synthetic-bootstrap.json": map[string]any{"qualification": false, "fixture": evidence, "warning": "NEVER COPY THIS STATE INTO AN INSTALLED WORKSPACE"}} {
		encoded, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		file, err := os.OpenFile(filepath.Join(dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			t.Fatal(err)
		}
		_, writeErr := file.Write(append(encoded, '\n'))
		closeErr := file.Close()
		if writeErr != nil || closeErr != nil {
			t.Fatal(writeErr, closeErr)
		}
	}
	t.Log("Synthetic candidate-only bootstrap installed in marked isolated state; no real qualification recorded")
}
