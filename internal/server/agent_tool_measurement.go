package server

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/internal/resource"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type agentToolMeasurements struct {
	measurements  models.CapabilityMeasurements
	installations []agentToolInstallation
	model         func(string) (*api.ModelMetadata, error)
	hardware      func(context.Context) (string, error)
	installation  func(context.Context) (func(), error)
	live          func(context.Context, string, int, inference.ToolRuntimeArtifacts) (*inference.MeasuredToolRuntime, error)
}

var errAgentQualificationStale = errors.New("native agent qualification is stale")

// InstallMeasuredAgentTools connects the production owner before runner binding.
// No inventory means compatibility behavior. Explicit invalid inventories return
// an error; callers may leave policy disabled but cannot activate partial data.
func (s *Server) InstallMeasuredAgentTools() error {
	if s.config == nil || s.agentRunner != nil || s.agentToolPolicy != nil {
		return errors.New("native agent measurements require startup ownership")
	}
	entries, err := readAgentToolInstallations(filepath.Join(s.config.DataDir, "model-capabilities", "native-agent-installations.json"))
	if err != nil || len(entries) == 0 {
		return err
	}
	owner, ok := s.llamaProcess.(interface {
		MeasureToolRuntimeArtifacts(context.Context, string, int, inference.ToolRuntimeArtifacts) (*inference.MeasuredToolRuntime, error)
	})
	if !ok || s.registry == nil || s.systemResources == nil {
		return errors.New("native agent runtime measurement owner is unavailable")
	}
	resources := *s.systemResources
	m := &agentToolMeasurements{installations: entries, model: s.registry.GetChatModel,
		hardware: func(ctx context.Context) (string, error) { return resource.QualificationHardwareID(ctx, &resources) },
		installation: func(ctx context.Context) (func(), error) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			// Cold selection cannot acquire an inference lease (that could load
			// a model), but must exclude service-owned removal/replacement.
			// Do not wait behind a slow model load or block cancellation.
			if !s.modelMutex.TryLock() {
				return nil, inference.ErrRuntimeBusy
			}
			return s.modelMutex.Unlock, nil
		},
		live: owner.MeasureToolRuntimeArtifacts}
	return s.InstallQualifiedAgentTools(m.target(false), m.target(true))
}

func (m *agentToolMeasurements) target(live bool) inference.AgentToolTarget {
	return func(ctx context.Context, model string, window int) (*models.CapabilityEvidenceTarget, error) {
		target, err := m.measure(ctx, model, window, live)
		// New work can retain the compatibility adapter when old evidence no
		// longer matches. Validate still rejects an existing native pin because
		// Select returns no current pin; it never changes adapters mid-task.
		if !live && errors.Is(err, errAgentQualificationStale) {
			return nil, nil
		}
		return target, err
	}
}

func (m *agentToolMeasurements) measure(ctx context.Context, model string, window int, live bool) (*models.CapabilityEvidenceTarget, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var installation *agentToolInstallation
	for index := range m.installations {
		i := &m.installations[index]
		if i.ModelID == model && i.Fingerprint.ContextWindow == window {
			installation = i
			break
		}
	}
	if installation == nil {
		return nil, nil
	}
	if !live && m.installation != nil {
		release, err := m.installation(ctx)
		if err != nil {
			return nil, err
		}
		defer release()
	}
	i := *installation
	metadata, err := m.model(model)
	if err != nil {
		return nil, err
	}
	if metadata == nil || metadata.Quantization != i.Fingerprint.Quantization || metadata.ProjectorPath != "" || metadata.Format != "gguf" {
		return nil, fmt.Errorf("%w: model installation changed", errAgentQualificationStale)
	}
	hardware, err := m.hardware(ctx)
	if err != nil {
		return nil, err
	}
	if hardware != i.Fingerprint.HardwareID {
		return nil, fmt.Errorf("%w: hardware changed", errAgentQualificationStale)
	}
	files, closeFiles, err := openAgentToolArtifacts(metadata.Path, i.RuntimeFiles)
	if err != nil {
		return nil, err
	}
	defer closeFiles()
	files.Measurements = &m.measurements
	var measured *inference.MeasuredToolRuntime
	if live {
		measured, err = m.live(ctx, model, window, files)
	} else {
		measured = &inference.MeasuredToolRuntime{}
		measured.ModelSHA256, err = m.measurements.File(ctx, files.Model)
		if err == nil {
			measured.RuntimeSHA256, err = m.measurements.Bundle(ctx, files.Runtime)
		}
	}
	if err != nil {
		return nil, err
	}
	if measured == nil || measured.ModelSHA256 != i.Fingerprint.ModelSHA256 || measured.RuntimeSHA256 != i.Fingerprint.RuntimeSHA256 || measured.ProjectorSHA256 != i.Fingerprint.ProjectorSHA256 {
		return nil, fmt.Errorf("%w: artifact bytes changed", errAgentQualificationStale)
	}
	if live {
		o := measured.Observation
		if o.ModelID != model || o.Generation == 0 || o.Info.Context != window || o.Info.Build != i.Fingerprint.RuntimeBuild || o.Info.TemplateSHA256 != i.Fingerprint.TemplateSHA256 || o.ConfigSHA256 != i.Fingerprint.RuntimeConfigSHA256 {
			return nil, fmt.Errorf("%w: live runtime changed", errAgentQualificationStale)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	target := i.target()
	return &target, nil
}

func openAgentToolArtifacts(model string, runtimeFiles map[string]string) (inference.ToolRuntimeArtifacts, func(), error) {
	files := inference.ToolRuntimeArtifacts{Runtime: map[string]*os.File{}}
	closeFiles := func() {
		if files.Model != nil {
			_ = files.Model.Close()
		}
		for _, file := range files.Runtime {
			_ = file.Close()
		}
	}
	var err error
	files.Model, err = os.Open(model)
	if err == nil {
		for role, path := range runtimeFiles {
			file, openErr := os.Open(path)
			if openErr != nil {
				err = openErr
				break
			}
			files.Runtime[role] = file
		}
	}
	if err == nil {
		servicePath, pathErr := os.Executable()
		if runtime.GOOS == "linux" {
			servicePath = "/proc/self/exe"
		}
		service := files.Runtime["offgrid-service"]
		if pathErr != nil || service == nil {
			err = errors.New("running service identity is unavailable")
		} else {
			held, heldErr := service.Stat()
			current, currentErr := os.Stat(servicePath)
			if heldErr != nil || currentErr != nil || !held.Mode().IsRegular() || !os.SameFile(held, current) {
				err = errors.New("native agent inventory does not identify the running service")
			}
		}
	}
	if err != nil {
		closeFiles()
		return inference.ToolRuntimeArtifacts{}, func() {}, err
	}
	return files, closeFiles, nil
}
