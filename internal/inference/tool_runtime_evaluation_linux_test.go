//go:build linux && !llama

package inference

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/shirou/gopsutil/v3/process"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/internal/resource"
)

// Explicit, read-only developer measurement inside an isolated candidate
// container. This does not manufacture workflow evidence, install a policy,
// submit inference or change the service. The production owner independently
// remeasures the resulting inventory before any qualified native decision.
func TestMeasureIsolatedNativeRuntime(t *testing.T) {
	input := os.Getenv("OFFGRID_TEST_RUNTIME_MEASUREMENT")
	if input == "" {
		t.Skip("requires explicit isolated candidate measurement")
	}
	var request struct {
		ModelID    string `json:"model_id"`
		ModelsDir  string `json:"models_dir"`
		Backend    string `json:"backend"`
		ServicePID int    `json:"service_pid"`
		RunnerPID  int    `json:"runner_pid"`
		Context    int    `json:"context_window"`
		Port       int    `json:"port"`
		Output     string `json:"output"`
	}
	data, err := os.ReadFile(input)
	if err != nil || len(data) > 8192 || json.Unmarshal(data, &request) != nil || request.ServicePID < 1 || request.RunnerPID < 1 || request.ServicePID == request.RunnerPID || request.ModelID == "" || request.Context < 1 || request.Port < 1 || request.Port > 65535 || !filepath.IsAbs(request.Output) {
		t.Fatal("invalid explicit runtime measurement request")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	serviceStart, runnerStart := observedProcessCreateTime(request.ServicePID), observedProcessCreateTime(request.RunnerPID)
	if serviceStart <= 0 || runnerStart <= 0 {
		t.Fatal("process identity unavailable")
	}
	proc, err := process.NewProcess(int32(request.RunnerPID))
	if err != nil {
		t.Fatal(err)
	}
	parent, err := proc.PpidWithContext(ctx)
	if err != nil || int(parent) != request.ServicePID {
		t.Fatal("runner is not owned by the selected service")
	}
	readVector := func(name string) []string {
		b, err := os.ReadFile(fmt.Sprintf("/proc/%d/%s", request.RunnerPID, name))
		if err != nil {
			t.Fatal(err)
		}
		return strings.Split(strings.TrimSuffix(string(b), "\x00"), "\x00")
	}
	args, env := readVector("cmdline"), readVector("environ")
	argument := func(names ...string) string {
		value, found := "", false
		for i, arg := range args {
			for _, name := range names {
				if arg == name {
					if found || i+1 >= len(args) {
						t.Fatal("ambiguous runtime argument", name)
					}
					value, found = args[i+1], true
				}
			}
		}
		return value
	}
	modelPath, projectorPath := argument("-m", "--model"), argument("--mmproj")
	if !filepath.IsAbs(modelPath) || projectorPath != "" {
		t.Fatal("measurement requires an owned text model")
	}
	if !filepath.IsAbs(request.ModelsDir) {
		t.Fatal("measurement requires an explicit model registry root")
	}
	registry := models.NewRegistry(request.ModelsDir)
	if err := registry.ScanModelsReadOnly(); err != nil {
		t.Fatal(err)
	}
	metadata, err := registry.GetChatModel(request.ModelID)
	if err != nil || metadata.Path != modelPath || metadata.ProjectorPath != "" || metadata.Format != "gguf" {
		t.Fatal("selected model does not match the owned runtime", err)
	}
	if argument("--port") != strconv.Itoa(request.Port) {
		t.Fatal("runtime port does not match selected process")
	}
	resources, err := resource.DetectResources()
	if err != nil {
		t.Fatal(err)
	}
	layers := argument("-ngl", "--n-gpu-layers", "--gpu-layers")
	if request.Backend == "cpu" {
		if layers != "0" {
			t.Fatal("CPU measurement requires explicit zero GPU layers")
		}
	} else if request.Backend == "cuda" {
		if layers == "" || layers == "0" || !resources.GPUAvailable {
			t.Fatal("CUDA launch/hardware unavailable")
		}
	} else {
		t.Fatal("this evaluator does not attest the requested backend")
	}
	engine := NewLlamaHTTPEngine(fmt.Sprintf("http://127.0.0.1:%d", request.Port))
	defer engine.httpClient.CloseIdleConnections()
	engine.httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	before, err := engine.ToolRuntimeInfo(ctx)
	if err != nil || before == nil || before.Context != request.Context {
		t.Fatal("runtime observation unavailable or context changed", err)
	}
	configuration, err := toolRuntimeConfiguration(args, env, modelPath, "", request.Port, before.TemplateSHA256)
	if err != nil {
		t.Fatal(err)
	}
	paths := map[string]string{}
	for role, pid := range map[string]int{"executable": request.RunnerPID, "offgrid-service": request.ServicePID} {
		path, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
		if err != nil || strings.HasSuffix(path, " (deleted)") {
			t.Fatal("process executable unavailable", err)
		}
		paths[role] = path
	}
	// Capture actual mapped dependency files. Do not infer them from a package
	// label or run ldd on untrusted model artifacts. Model data is hashed apart.
	maps, err := os.ReadFile(fmt.Sprintf("/proc/%d/maps", request.RunnerPID))
	if err != nil {
		t.Fatal(err)
	}
	parseMaps := func(data []byte) map[string]bool {
		dependencies := map[string]bool{}
		for _, line := range strings.Split(string(data), "\n") {
			path, err := evaluatedRuntimeMappingPath(line)
			if err != nil {
				t.Fatal(err)
			}
			if path == "" {
				continue
			}
			if path == modelPath || path == paths["executable"] {
				continue
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().IsRegular() {
				dependencies[path] = true
			}
		}
		return dependencies
	}
	dependencies := parseMaps(maps)
	ordered := make([]string, 0, len(dependencies))
	for path := range dependencies {
		ordered = append(ordered, path)
	}
	sort.Strings(ordered)
	for index, path := range ordered {
		paths[fmt.Sprintf("dependency-%03d", index+1)] = path
	}
	files := map[string]*os.File{}
	for role, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		files[role] = file
	}
	model, err := os.Open(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	defer model.Close()
	modelSHA, err := models.MeasureCapabilityFile(ctx, model)
	if err != nil {
		t.Fatal(err)
	}
	bundleSHA, err := models.MeasureCapabilityBundle(ctx, files)
	if err != nil {
		t.Fatal(err)
	}
	hardware, err := resource.QualificationHardwareID(ctx, resources)
	if err != nil {
		t.Fatal(err)
	}
	after, err := engine.ToolRuntimeInfo(ctx)
	if err != nil || !reflect.DeepEqual(before, after) || observedProcessCreateTime(request.ServicePID) != serviceStart || observedProcessCreateTime(request.RunnerPID) != runnerStart || !reflect.DeepEqual(args, readVector("cmdline")) || !reflect.DeepEqual(env, readVector("environ")) {
		t.Fatal("runtime changed during measurement", err)
	}
	afterMaps, err := os.ReadFile(fmt.Sprintf("/proc/%d/maps", request.RunnerPID))
	if err != nil {
		t.Fatal(err)
	}
	// Anonymous heaps may change during observation; mapped artifact paths may not.
	if !reflect.DeepEqual(dependencies, parseMaps(afterMaps)) {
		t.Fatal("runtime dependency mapping changed")
	}
	fingerprint := models.CapabilityFingerprint{ModelSHA256: modelSHA, Quantization: metadata.Quantization, RuntimeBuild: before.Build, TemplateSHA256: before.TemplateSHA256, Adapter: NativeAgentToolAdapter, ContextWindow: request.Context, Backend: request.Backend, HardwareID: hardware, RuntimeSHA256: bundleSHA, RuntimeConfigSHA256: configuration}
	if _, err := fingerprint.ID(); err != nil {
		t.Fatal(err)
	}
	result := struct {
		ModelID       string                       `json:"model_id"`
		Fingerprint   models.CapabilityFingerprint `json:"fingerprint"`
		RuntimeFiles  map[string]string            `json:"runtime_files"`
		Platform      string                       `json:"platform"`
		Qualification bool                         `json:"qualification"`
	}{request.ModelID, fingerprint, paths, resources.OS + "-" + resources.Arch, false}
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	output, err := os.OpenFile(request.Output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := output.Write(append(encoded, '\n')); err != nil {
		output.Close()
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
	t.Log("Measured owned runtime artifacts and launch identity; no qualification or policy installed")
}

func evaluatedRuntimeMappingPath(line string) (string, error) {
	fields := strings.Fields(line)
	if len(fields) < 6 || !strings.HasPrefix(fields[5], "/") {
		return "", nil
	}
	path := strings.Join(fields[5:], " ")
	// Linux represents shared anonymous, non-executable GPU buffers this way.
	// It is not an unlinked library. Do not generalize this exception to other
	// deleted paths, executable mappings, or memfd code.
	if path == "/dev/zero (deleted)" && fields[1] == "rw-s" {
		return "", nil
	}
	if strings.Contains(path, "\\") || strings.HasSuffix(path, " (deleted)") {
		return "", fmt.Errorf("unmeasurable mapped file: permissions=%q path=%q", fields[1], path)
	}
	return path, nil
}

func TestEvaluatedRuntimeMappingsKeepExecutableEvidence(t *testing.T) {
	for _, tc := range []struct {
		line, path string
		rejected   bool
	}{
		{"100-200 rw-s 0000 00:01 1 /dev/zero (deleted)", "", false},
		{"100-200 r-xp 0000 00:01 1 /dev/zero (deleted)", "", true},
		{"100-200 r-xp 0000 08:01 1 /lib/libcuda.so (deleted)", "", true},
		{"100-200 r-xp 0000 08:01 1 /lib/libcuda.so", "/lib/libcuda.so", false},
		{"100-200 r--p 0000 08:01 1 /models/model.gguf", "/models/model.gguf", false},
		{"100-200 rw-p 0000 00:00 0 [heap]", "", false},
		{"100-200 r-xp 0000 00:01 1 /memfd:code (deleted)", "", true},
	} {
		got, err := evaluatedRuntimeMappingPath(tc.line)
		if got != tc.path || (err != nil) != tc.rejected {
			t.Fatalf("%s: path=%q err=%v", tc.line, got, err)
		}
	}
}
