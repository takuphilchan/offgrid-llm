package inference

import (
	"encoding/base64"
	"encoding/json"
	"path/filepath"
	"strconv"

	"github.com/takuphilchan/offgrid-llm/internal/models"
)

func onnxWorkerArgs(model *models.ONNXEmbeddingLease, config ONNXEmbeddingRuntimeConfig, descriptor ProviderDescriptor, threads int) ([]string, error) {
	identity, err := json.Marshal(descriptor)
	if err != nil {
		return nil, err
	}
	dependencies, err := json.Marshal(config.DependencyVersions)
	if err != nil {
		return nil, err
	}
	artifactPins := map[string]string{}
	for _, artifact := range model.Manifest.Artifacts {
		artifactPins[artifact.Path] = artifact.SHA256
	}
	artifacts, err := json.Marshal(artifactPins)
	if err != nil {
		return nil, err
	}
	args := isolatedPythonWorkerArgs(filepath.Join(config.RuntimeDirectory, filepath.FromSlash(config.Script)), model.ModelDir, descriptor, identity, dependencies)
	args = append([]string{"-S"}, args...)
	return append(args, "--threads", strconv.Itoa(threads), "--runtime-root", config.RuntimeDirectory, "--artifacts", base64.RawURLEncoding.EncodeToString(artifacts)), nil
}
