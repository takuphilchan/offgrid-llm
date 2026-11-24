package inference

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/models"
)

func startOwnedONNXEmbedding(ctx context.Context, model *models.ONNXEmbeddingLease, config ONNXEmbeddingRuntimeConfig, threads int) (ProviderRuntime, models.ONNXEmbeddingSettings, string, error) {
	var settings models.ONNXEmbeddingSettings
	files := &managedOllamaFiles{}
	retained := false
	defer func() {
		if !retained {
			files.close()
			model.Release()
		}
	}()
	if err := config.Validate(); err != nil {
		return nil, settings, "", err
	}
	if config.NamespaceLauncher != "" || config.NamespaceLauncherSHA256 != "" {
		return nil, settings, "", errors.New("Windows ONNX ownership does not use an external namespace launcher")
	}
	if err := checkONNXUnprivilegedToken(); err != nil {
		return nil, settings, "", err
	}
	if err := checkONNXRuntimeAncestors(config.RuntimeDirectory); err != nil {
		return nil, settings, "", err
	}
	if config.RunnerRevision != model.Manifest.RunnerRevision {
		return nil, settings, "", errors.New("installed ONNX runtime does not match the package runner revision")
	}
	if threads < 1 {
		threads = 1
	}
	if threads > 256 {
		return nil, settings, "", errors.New("ONNX thread budget exceeds supported bounds")
	}
	for _, directory := range []string{config.RuntimeDirectory, config.StateDirectory, model.ModelDir} {
		for _, other := range []string{config.RuntimeDirectory, config.StateDirectory, model.ModelDir} {
			if directory != other && strings.HasPrefix(strings.ToLower(filepath.Clean(directory))+`\`, strings.ToLower(filepath.Clean(other))+`\`) {
				return nil, settings, "", errors.New("ONNX runtime, model and state directories must be separate")
			}
		}
	}
	if strings.EqualFold(config.RuntimeDirectory, config.StateDirectory) || strings.EqualFold(config.RuntimeDirectory, model.ModelDir) || strings.EqualFold(config.StateDirectory, model.ModelDir) {
		return nil, settings, "", errors.New("ONNX runtime, model and state directories must differ")
	}
	if _, err := files.open(config.StateDirectory, true); err != nil {
		return nil, settings, "", err
	}
	if err := files.acquireStateLock(config.StateDirectory); err != nil {
		return nil, settings, "", err
	}
	if err := files.onnxRuntimeTree(ctx, config.RuntimeDirectory, config.RuntimeFiles); err != nil {
		return nil, settings, "", err
	}
	if _, err := files.open(model.ModelDir, true); err != nil {
		return nil, settings, "", err
	}
	manifestFile, err := files.open(filepath.Join(model.ModelDir, "manifest.json"), false)
	if err != nil {
		return nil, settings, "", err
	}
	lockedManifest, err := models.DecodePackageManifest(manifestFile)
	if err != nil || models.ManifestDigest(lockedManifest) != models.ManifestDigest(model.Manifest) {
		return nil, settings, "", errors.New("ONNX package manifest changed before file ownership")
	}
	var settingsRaw []byte
	for _, artifact := range model.Manifest.Artifacts {
		file, err := files.open(filepath.Join(model.ModelDir, artifact.Path), false)
		if err != nil {
			return nil, settings, "", err
		}
		if err := managedHash(ctx, file, "sha256:"+artifact.SHA256, artifact.Size); err != nil {
			return nil, settings, "", err
		}
		if artifact.Path == "embedding.json" {
			settingsRaw, err = io.ReadAll(io.LimitReader(file, (64<<10)+1))
			if err != nil {
				return nil, settings, "", err
			}
		}
	}
	settings, err = models.DecodeONNXEmbeddingSettings(settingsRaw)
	if err != nil {
		return nil, settings, "", err
	}
	if settings.MaxTokens != model.Manifest.ContextLength {
		return nil, settings, "", errors.New("ONNX context does not match package settings")
	}
	descriptor := onnxEmbeddingDescriptor(model, settingsRaw)
	workerEnv, err := nativeWorkerEnvironment(nil)
	if err != nil {
		return nil, settings, "", err
	}
	workerEnv = append(workerEnv, "HOME="+config.StateDirectory, "USERPROFILE="+config.StateDirectory, "TEMP="+config.StateDirectory, "TMP="+config.StateDirectory, "LOCALAPPDATA="+config.StateDirectory, "APPDATA="+config.StateDirectory)
	workerEnv = append(workerEnv, "PATH="+config.RuntimeDirectory)
	args, err := onnxWorkerArgs(model, config, descriptor, threads)
	if err != nil {
		return nil, settings, "", err
	}
	provider, err := startProcessProviderInDirectory(ctx, filepath.Join(config.RuntimeDirectory, filepath.FromSlash(config.Python)), args, descriptor, workerEnv, config.RuntimeDirectory)
	if err != nil {
		if errors.Is(err, errProviderStopUncertain) {
			retained = true
			files.retainUncertain()
		}
		return nil, settings, "", err
	}
	retained = true
	provider.release = func() { files.close(); model.Release() }
	return provider, settings, onnxEmbeddingIdentity(descriptor, config), nil
}

func validateONNXPythonSearchPaths(root, filename string, raw []byte) error {
	if len(raw) > 64<<10 {
		return errors.New("ONNX Python search path is oversized")
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(strings.SplitN(line, "#", 2)[0])
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "import ") || strings.ContainsRune(line, 0) || filepath.IsAbs(line) || strings.Contains(line, ":") {
			return errors.New("ONNX Python search path cannot execute site initialization or select external paths")
		}
		resolved := filepath.Clean(filepath.Join(filepath.Dir(filename), line))
		relative, err := filepath.Rel(root, resolved)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return errors.New("ONNX Python search path escapes pinned runtime")
		}
	}
	return nil
}

func checkONNXRuntimePath(path string, _ bool) error { return checkONNXRuntimeNamespace(path, false) }

func validateONNXRuntimeImportPath(root, path string, file *os.File) error {
	if strings.EqualFold(filepath.Base(path), "pyvenv.cfg") {
		return errors.New("ONNX runtime cannot delegate Python to an external virtual-environment base")
	}
	if strings.HasSuffix(strings.ToLower(path), "._pth") {
		raw, err := io.ReadAll(io.LimitReader(file, (64<<10)+1))
		if err != nil {
			return err
		}
		return validateONNXPythonSearchPaths(root, path, raw)
	}
	return nil
}
