package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"runtime"

	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/models"
)

func (p *localDocumentSynthesis) observe(ctx context.Context, i document.SynthesisModelIdentity) (*inference.ToolRuntimeObservation, error) {
	owner, ok := p.server.llamaProcess.(interface {
		ObserveToolRuntime(context.Context, string, int) (*inference.ToolRuntimeObservation, error)
	})
	if !ok {
		return nil, errors.New("owned local synthesis runtime unavailable")
	}
	return owner.ObserveToolRuntime(ctx, i.ModelID, i.ContextTokens)
}

// Measure the selected owned process's model, executable, projector, service,
// template and launch configuration. This records execution identity; it is not
// model quality qualification or a complete inventory of OS dynamic libraries.
func (p *localDocumentSynthesis) measure(ctx context.Context, i document.SynthesisModelIdentity) (document.SynthesisModelIdentity, error) {
	before, err := p.observe(ctx, i)
	if err != nil {
		return i, err
	}
	if before == nil || before.ModelID != i.ModelID || before.Generation == 0 || before.Info.Context < i.ContextTokens || before.ConfigSHA256 == "" {
		return i, errors.New("invalid synthesis process identity")
	}
	service, err := os.Executable()
	if err != nil {
		return i, err
	}
	if runtime.GOOS == "linux" {
		service = "/proc/self/exe"
	}
	paths := map[string]string{"model": before.ModelPath, "executable": before.ExecutablePath, "offgrid-service": service}
	if before.ProjectorPath != "" {
		paths["projector"] = before.ProjectorPath
	}
	digests := map[string]string{"template": before.Info.TemplateSHA256, "configuration": before.ConfigSHA256}
	for role, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			return i, err
		}
		digest, err := models.MeasureCapabilityFile(ctx, file)
		held, heldErr := file.Stat()
		current, currentErr := os.Stat(path)
		file.Close()
		if err != nil || heldErr != nil || currentErr != nil || !os.SameFile(held, current) {
			return i, errors.New("synthesis artifact changed during measurement")
		}
		digests[role] = digest
	}
	after, err := p.observe(ctx, i)
	if err != nil || !reflect.DeepEqual(before, after) {
		return i, errors.New("synthesis runtime changed during measurement")
	}
	i.ModelSHA256 = digests["model"]
	delete(digests, "model")
	raw, err := json.Marshal(digests)
	if err != nil {
		return i, err
	}
	hash := sha256.Sum256(raw)
	i.Runtime = document.RuntimeIdentity{Name: "llama.cpp", Revision: before.Info.Build, SHA256: hex.EncodeToString(hash[:])}
	return i, i.Validate()
}
