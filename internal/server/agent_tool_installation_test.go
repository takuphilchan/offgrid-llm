package server

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func measuredInstallationFixture(t *testing.T) (agentToolInstallation, *agentToolMeasurements, string) {
	t.Helper()
	ctx := context.Background()
	model := filepath.Join(t.TempDir(), "model.gguf")
	if err := os.WriteFile(model, []byte("fixture model"), 0600); err != nil {
		t.Fatal(err)
	}
	service, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	paths := map[string]string{"executable": service, "offgrid-service": service}
	files, closeFiles, err := openAgentToolArtifacts(model, paths)
	if err != nil {
		t.Fatal(err)
	}
	defer closeFiles()
	modelSHA, err := models.MeasureCapabilityFile(ctx, files.Model)
	if err != nil {
		t.Fatal(err)
	}
	bundleSHA, err := models.MeasureCapabilityBundle(ctx, files.Runtime)
	if err != nil {
		t.Fatal(err)
	}
	f := models.CapabilityFingerprint{ModelSHA256: modelSHA, RuntimeSHA256: bundleSHA, RuntimeConfigSHA256: strings.Repeat("a", 64), TemplateSHA256: strings.Repeat("b", 64), RuntimeBuild: "fixture", Adapter: inference.NativeAgentToolAdapter, ContextWindow: 8192, Quantization: "fixture", Backend: "cpu", HardwareID: "hardware"}
	i := agentToolInstallation{ModelID: "fixture", Fingerprint: f, Scope: models.NativeWorkspaceContractScope, SuiteSHA256: models.NativeWorkspaceSuiteSHA256ForScope(models.NativeWorkspaceContractScope), CapabilitySHA256: agents.NativeWorkspaceCapabilitySHA256(), Platform: runtime.GOOS + "-" + runtime.GOARCH, RuntimeFiles: paths}
	m := &agentToolMeasurements{installations: []agentToolInstallation{i}, model: func(string) (*api.ModelMetadata, error) {
		return &api.ModelMetadata{Path: model, Quantization: "fixture", Format: "gguf"}, nil
	}, hardware: func(context.Context) (string, error) { return "hardware", nil }}
	m.live = func(_ context.Context, id string, window int, _ inference.ToolRuntimeArtifacts) (*inference.MeasuredToolRuntime, error) {
		return &inference.MeasuredToolRuntime{ModelSHA256: modelSHA, RuntimeSHA256: bundleSHA, Observation: inference.ToolRuntimeObservation{ModelID: id, Generation: 1, ConfigSHA256: f.RuntimeConfigSHA256, Info: inference.ToolRuntimeInfo{Build: f.RuntimeBuild, TemplateSHA256: f.TemplateSHA256, Context: window}}}, nil
	}
	return i, m, model
}

func TestAgentToolInstallationInventory(t *testing.T) {
	i, _, _ := measuredInstallationFixture(t)
	path := filepath.Join(t.TempDir(), "inventory.json")
	if entries, err := readAgentToolInstallations(path); err != nil || entries != nil {
		t.Fatal(entries, err)
	}
	valid, _ := json.Marshal(map[string]any{"version": 1, "models": []agentToolInstallation{i}})
	for _, tc := range []struct {
		name  string
		data  []byte
		valid bool
	}{
		{"valid", valid, true},
		{"duplicate", []byte(`{"version":1,"version":1,"models":[]}`), false},
		{"case alias", []byte(`{"version":2,"Version":1,"models":[]}`), false},
		{"unknown", []byte(`{"version":1,"models":[],"enable":true}`), false},
		{"future", []byte(`{"version":2,"models":[]}`), false},
		{"trailing", append(append([]byte(nil), valid...), []byte(`{}`)...), false},
		{"oversize", []byte(strings.Repeat(" ", (256<<10)+1)), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(path, tc.data, 0600); err != nil {
				t.Fatal(err)
			}
			_, err := readAgentToolInstallations(path)
			if (err == nil) != tc.valid {
				t.Fatal(err)
			}
		})
	}
	for _, mutate := range []func(*agentToolInstallation){
		func(i *agentToolInstallation) { i.Scope = "all-tools" },
		func(i *agentToolInstallation) { i.CapabilitySHA256 = strings.Repeat("f", 64) },
		func(i *agentToolInstallation) { i.Platform = "other" },
		func(i *agentToolInstallation) { i.Fingerprint.RuntimeSHA256 = "" },
		func(i *agentToolInstallation) { i.Fingerprint.ProjectorSHA256 = strings.Repeat("f", 64) },
		func(i *agentToolInstallation) {
			i.RuntimeFiles = map[string]string{"executable": "relative", "offgrid-service": "relative"}
		},
	} {
		bad := i
		mutate(&bad)
		data, _ := json.Marshal(map[string]any{"version": 1, "models": []agentToolInstallation{bad}})
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := readAgentToolInstallations(path); err == nil {
			t.Fatal("invalid inventory accepted")
		}
	}
	duplicate, _ := json.Marshal(map[string]any{"version": 1, "models": []agentToolInstallation{i, i}})
	if err := os.WriteFile(path, duplicate, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readAgentToolInstallations(path); err == nil {
		t.Fatal("duplicate installation accepted")
	}
	if _, err := readAgentToolInstallations(t.TempDir()); err == nil {
		t.Fatal("directory accepted as inventory")
	}
}

func TestAgentToolMeasuredTargets(t *testing.T) {
	_, m, path := measuredInstallationFixture(t)
	ctx := context.Background()
	for _, live := range []bool{false, true} {
		if target, err := m.target(live)(ctx, "fixture", 8192); err != nil || target == nil {
			t.Fatal(target, err)
		}
		if target, err := m.target(live)(ctx, "other", 8192); err != nil || target != nil {
			t.Fatal(target, err)
		}
		if target, err := m.target(live)(ctx, "fixture", 4096); err != nil || target != nil {
			t.Fatal(target, err)
		}
	}
	live := m.live
	m.live = func(context.Context, string, int, inference.ToolRuntimeArtifacts) (*inference.MeasuredToolRuntime, error) {
		t.Fatal("cold selection loaded model")
		return nil, nil
	}
	if _, err := m.target(false)(ctx, "fixture", 8192); err != nil {
		t.Fatal(err)
	}
	m.live = func(ctx context.Context, id string, window int, files inference.ToolRuntimeArtifacts) (*inference.MeasuredToolRuntime, error) {
		result, err := live(ctx, id, window, files)
		result.Observation.ConfigSHA256 = strings.Repeat("f", 64)
		return result, err
	}
	if _, err := m.target(true)(ctx, "fixture", 8192); err == nil {
		t.Fatal("changed live configuration accepted")
	}
	m.hardware = func(context.Context) (string, error) { return "other", nil }
	if target, err := m.target(false)(ctx, "fixture", 8192); err != nil || target != nil {
		t.Fatal("changed hardware selected native tools", target, err)
	}
	m.hardware = func(context.Context) (string, error) { return "hardware", nil }
	if err := os.WriteFile(path, []byte("changed model"), 0600); err != nil {
		t.Fatal(err)
	}
	if target, err := m.target(false)(ctx, "fixture", 8192); err != nil || target != nil {
		t.Fatal("changed model bytes selected native tools", target, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := m.target(false)(cancelled, "fixture", 8192); err != context.Canceled {
		t.Fatal(err)
	}
}

func TestAgentToolInstallationDoesNotQualifyItself(t *testing.T) {
	_, m, _ := measuredInstallationFixture(t)
	policy, err := inference.NewAgentToolPolicy(models.NewCapabilityEvidenceStore(t.TempDir()), m.target(false), m.target(true))
	if err != nil {
		t.Fatal(err)
	}
	if pin, err := policy.Select(context.Background(), "fixture", 8192); err != nil || pin != nil {
		t.Fatal("inventory manufactured qualification", pin, err)
	}
	s := &Server{config: &config.Config{DataDir: t.TempDir()}}
	if err := s.InstallMeasuredAgentTools(); err != nil || s.agentToolPolicy != nil {
		t.Fatal("missing inventory changed compatibility", err)
	}
}
