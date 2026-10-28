//go:build !llama

package inference

import (
	"context"
	"errors"
	"os"
	"reflect"
	"runtime"
)

// The installer owns ToolRuntimeArtifacts' open handles. It
// supplies the complete runtime inventory, including dependencies, with the
// selected program under "executable" and this running service under
// "offgrid-service". Adapter/prompt code changes must invalidate qualification
// even when the llama.cpp build and the tool schema remain unchanged.
// It owns handle closure and must retain
// the installation and inference leases through measurement and generation.
// No paths or handles originate in an agent/tool request.
// MeasureToolRuntimeArtifacts binds content hashes to the actual selected child.
// Metadata alone, a same-named binary elsewhere, or a stale open file handle
// cannot attest the files currently selected by that process owner.
func (p *LlamaCPPProcess) MeasureToolRuntimeArtifacts(ctx context.Context, model string, window int, files ToolRuntimeArtifacts) (*MeasuredToolRuntime, error) {
	before, err := p.ObserveToolRuntime(ctx, model, window)
	if err != nil {
		return nil, err
	}
	if err := files.bind(before); err != nil {
		return nil, err
	}
	modelSHA, err := files.Measurements.File(ctx, files.Model)
	if err != nil {
		return nil, err
	}
	projectorSHA := ""
	if files.Projector != nil {
		projectorSHA, err = files.Measurements.File(ctx, files.Projector)
		if err != nil {
			return nil, err
		}
	}
	runtimeSHA, err := files.Measurements.Bundle(ctx, files.Runtime)
	if err != nil {
		return nil, err
	}
	after, err := p.ObserveToolRuntime(ctx, model, window)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(before, after) {
		return nil, errors.New("owned runtime changed during artifact measurement")
	}
	if err := files.bind(after); err != nil {
		return nil, err
	}
	return &MeasuredToolRuntime{Observation: *after, ModelSHA256: modelSHA, ProjectorSHA256: projectorSHA, RuntimeSHA256: runtimeSHA}, nil
}

func (f ToolRuntimeArtifacts) bind(observation *ToolRuntimeObservation) error {
	if observation == nil {
		return errors.New("missing owned runtime identity")
	}
	type binding struct {
		file *os.File
		path string
	}
	servicePath, err := os.Executable()
	if err != nil {
		return errors.New("running service executable identity is unavailable")
	}
	if runtime.GOOS == "linux" {
		// Unlike the executable's installation pathname, this link identifies
		// the live image even if an installer renames/replaces that pathname.
		servicePath = "/proc/self/exe"
	}
	pairs := []binding{{f.Model, observation.ModelPath}, {f.Runtime["executable"], observation.ExecutablePath}, {f.Runtime["offgrid-service"], servicePath}, {f.Projector, observation.ProjectorPath}}
	for _, file := range f.Runtime {
		if file == nil {
			return errors.New("missing runtime dependency handle")
		}
		pairs = append(pairs, binding{file, file.Name()})
	}
	for _, pair := range pairs {
		if pair.file == nil && pair.path == "" {
			continue
		}
		if pair.file == nil || pair.path == "" {
			return errors.New("owned runtime artifact is missing or unexpected")
		}
		held, err := pair.file.Stat()
		if err != nil {
			return err
		}
		current, err := os.Stat(pair.path)
		if err != nil {
			return err
		}
		if !held.Mode().IsRegular() || !current.Mode().IsRegular() || !os.SameFile(held, current) {
			return errors.New("measured artifact does not belong to the selected runtime")
		}
	}
	return nil
}
