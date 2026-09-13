package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestHermesConfigurationCommandsPersistProvider(t *testing.T) {
	setup := cliIntegrationSetupResponse{Integration: cliIntegrationStatus{
		ModelID:       "tool-model",
		ContextWindow: 16384,
	}}
	commands := hermesConfigurationCommands(setup, "local-token", "http://127.0.0.1:11611/v1")
	want := [][]string{
		{"config", "set", "OFFGRID_API_KEY", "local-token"},
		{"config", "set", "OFFGRID_BASE_URL", "http://127.0.0.1:11611/v1", "--force"},
		{"config", "set", "model.default", "tool-model"},
		{"config", "set", "model.provider", "offgrid"},
		{"config", "set", "model.base_url", "http://127.0.0.1:11611/v1"},
		{"config", "set", "model.context_length", "16384"},
	}
	if !reflect.DeepEqual(commands, want) {
		t.Fatalf("unexpected Hermes configuration commands: %#v", commands)
	}
}

func TestMergedEnvironmentReplacesExistingValues(t *testing.T) {
	got := mergedEnvironment([]string{"PATH=/bin", "OFFGRID_API_KEY=old"}, map[string]string{
		"OFFGRID_API_KEY":  "new",
		"OFFGRID_BASE_URL": "http://offgrid:11611/v1",
	})
	for _, unwanted := range []string{"OFFGRID_API_KEY=old"} {
		if slices.Contains(got, unwanted) {
			t.Fatalf("stale environment value retained: %q in %#v", unwanted, got)
		}
	}
	for _, expected := range []string{"PATH=/bin", "OFFGRID_API_KEY=new", "OFFGRID_BASE_URL=http://offgrid:11611/v1"} {
		if !slices.Contains(got, expected) {
			t.Fatalf("missing environment value %q in %#v", expected, got)
		}
	}
}

func TestTrustedHermesInstallerHosts(t *testing.T) {
	for _, host := range []string{"hermes-agent.nousresearch.com", "raw.githubusercontent.com"} {
		if !trustedHermesInstallerHost(host) {
			t.Fatalf("expected %s to be trusted", host)
		}
	}
	for _, host := range []string{"example.com", "nousresearch.example.com"} {
		if trustedHermesInstallerHost(host) {
			t.Fatalf("expected %s to be rejected", host)
		}
	}
}

func TestValidateHermesPlatform(t *testing.T) {
	for _, platform := range [][2]string{{"linux", "amd64"}, {"linux", "arm64"}, {"windows", "amd64"}, {"windows", "arm64"}, {"darwin", "arm64"}} {
		if err := validateHermesPlatform(platform[0], platform[1]); err != nil {
			t.Fatalf("supported platform %s/%s rejected: %v", platform[0], platform[1], err)
		}
	}
	for _, platform := range [][2]string{{"darwin", "amd64"}, {"linux", "386"}, {"freebsd", "amd64"}} {
		if err := validateHermesPlatform(platform[0], platform[1]); err == nil {
			t.Fatalf("unsupported platform %s/%s accepted", platform[0], platform[1])
		}
	}
}

func TestHermesInstallDirHonorsOverride(t *testing.T) {
	override := filepath.Join(t.TempDir(), "custom-hermes")
	t.Setenv("HERMES_INSTALL_DIR", override)
	if got := hermesInstallDir(t.TempDir()); got != override {
		t.Fatalf("hermesInstallDir() = %q, want %q", got, override)
	}
}

func TestContainsExactOutputLine(t *testing.T) {
	if !containsExactOutputLine("starting\n\x1b[32mHERMES_OFFGRID_OK\x1b[0m\ndone\n", "HERMES_OFFGRID_OK") {
		t.Fatal("expected exact ANSI-decorated response line to match")
	}
	if containsExactOutputLine("Query: Reply with exactly: HERMES_OFFGRID_OK\nfailed\n", "HERMES_OFFGRID_OK") {
		t.Fatal("prompt echo must not be accepted as a model response")
	}
}

func TestHermesReadinessAdviceExplainsActualContext(t *testing.T) {
	status := cliIntegrationStatus{ContextWindow: 8192, MinimumContext: 64000, ModelID: "small-model"}
	advice := hermesReadinessAdvice(status)
	for _, part := range []string{"64000", "RAM/VRAM", "actually allocates"} {
		if !strings.Contains(advice, part) {
			t.Errorf("readiness advice %q missing %q", advice, part)
		}
	}
}

func TestHermesCoreStagesExcludeOptionalDependencies(t *testing.T) {
	for _, platform := range []string{"linux", "darwin", "windows"} {
		stages := hermesCoreStages(platform)
		if len(stages) == 0 {
			t.Fatalf("%s has no core stages", platform)
		}
		for _, optional := range []string{"prerequisites", "node", "node-deps", "system-packages", "platform-sdks"} {
			if slices.Contains(stages, optional) {
				t.Fatalf("%s includes optional stage %s", platform, optional)
			}
		}
	}
}

func TestHermesStageArgsKeepBrowserOptInAndCustomWindowsPath(t *testing.T) {
	if !slices.Contains(hermesStageArgs("linux", "venv"), "--skip-browser") {
		t.Fatal("Unix core stages must not pull optional browser dependencies")
	}
	if slices.Contains(hermesStageArgs("linux", "node-deps"), "--skip-browser") {
		t.Fatal("explicit browser stage must be allowed to install Chromium")
	}
	customDir := filepath.Join(t.TempDir(), "custom-hermes")
	t.Setenv("HERMES_INSTALL_DIR", customDir)
	args := hermesStageArgs("windows", "dependencies")
	if !slices.Contains(args, customDir) || !slices.Contains(args, "-NonInteractive") {
		t.Fatalf("Windows stage arguments missing custom path or non-interactive mode: %#v", args)
	}
}

func TestHermesOutputTailBoundsCapturedOutput(t *testing.T) {
	var captured hermesOutputTail
	data := strings.Repeat("x", hermesOutputLimit+50) + "\nHERMES_OFFGRID_OK\n"
	if length, err := captured.Write([]byte(data)); err != nil || length != len(data) {
		t.Fatalf("Write() = %d, %v", length, err)
	}
	if len(captured.String()) > hermesOutputLimit || !containsExactOutputLine(captured.String(), "HERMES_OFFGRID_OK") {
		t.Fatalf("bounded output lost expected line or exceeded limit")
	}
}

func TestHermesSmokeTestRequiresActualReply(t *testing.T) {
	original := hermesExecCommand
	t.Cleanup(func() { hermesExecCommand = original })
	hermesExecCommand = func(string, ...string) *exec.Cmd {
		return exec.Command(os.Args[0], "-test.run=TestHermesSmokeHelperProcess")
	}
	for _, scenario := range []struct {
		name    string
		wantErr bool
	}{
		{"prompt", true},
		{"error", true},
		{"reply", false},
	} {
		environment := mergedEnvironment(os.Environ(), map[string]string{"OFFGRID_HERMES_TEST_HELPER": scenario.name})
		err := runHermesSmokeTest("unused", nil, environment, "HERMES_OFFGRID_OK")
		if (err != nil) != scenario.wantErr {
			t.Errorf("scenario %s: error = %v, want error %t", scenario.name, err, scenario.wantErr)
		}
	}
}

func TestHermesSmokeHelperProcess(t *testing.T) {
	switch os.Getenv("OFFGRID_HERMES_TEST_HELPER") {
	case "prompt":
		fmt.Println("Query: Reply with exactly: HERMES_OFFGRID_OK")
	case "error":
		fmt.Println("Failed to initialize agent: context too small")
	case "reply":
		fmt.Println("HERMES_OFFGRID_OK")
	default:
		return
	}
	os.Exit(0)
}
