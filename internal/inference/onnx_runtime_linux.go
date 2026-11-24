package inference

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/models"
	"golang.org/x/sys/unix"
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
	if err := managedLinuxUnprivileged(); err != nil {
		return nil, settings, "", err
	}
	if config.NamespaceLauncher == "" || config.RunnerRevision != model.Manifest.RunnerRevision {
		return nil, settings, "", errors.New("Linux ONNX requires the pinned namespace launcher and matching runtime revision")
	}
	if threads < 1 {
		threads = 1
	}
	if threads > 256 {
		return nil, settings, "", errors.New("ONNX thread budget exceeds supported bounds")
	}
	for _, directory := range []string{config.RuntimeDirectory, config.StateDirectory, model.ModelDir} {
		for _, other := range []string{config.RuntimeDirectory, config.StateDirectory, model.ModelDir} {
			if directory != other && strings.HasPrefix(directory+"/", other+"/") {
				return nil, settings, "", errors.New("ONNX runtime, model and state must be separate trees")
			}
		}
	}
	if config.RuntimeDirectory == model.ModelDir || config.StateDirectory == model.ModelDir {
		return nil, settings, "", errors.New("ONNX model and runtime state must differ")
	}
	if err := files.openState(config.StateDirectory); err != nil {
		return nil, settings, "", err
	}
	if _, err := files.open(config.NamespaceLauncher, false); err != nil {
		return nil, settings, "", err
	}
	if err := files.onnxRuntimeTree(ctx, config.RuntimeDirectory, config.RuntimeFiles); err != nil {
		return nil, settings, "", err
	}
	settingsRaw, err := readLinuxONNXSettings(ctx, model)
	if err != nil {
		return nil, settings, "", err
	}
	settings, err = models.DecodeONNXEmbeddingSettings(settingsRaw)
	if err != nil {
		return nil, settings, "", err
	}
	if settings.MaxTokens != model.Manifest.ContextLength {
		return nil, settings, "", errors.New("ONNX context does not match package settings")
	}
	descriptor := onnxEmbeddingDescriptor(model, settingsRaw)
	args, err := onnxWorkerArgs(model, config, descriptor, threads)
	if err != nil {
		return nil, settings, "", err
	}
	env, err := nativeWorkerEnvironment(nil)
	if err != nil {
		return nil, settings, "", err
	}
	env = append(env, "PATH="+config.RuntimeDirectory+":/usr/bin:/bin", "HOME="+config.StateDirectory, "XDG_CONFIG_HOME="+config.StateDirectory, "XDG_CACHE_HOME="+config.StateDirectory, "TMPDIR="+config.StateDirectory, "TMP="+config.StateDirectory, "TEMP="+config.StateDirectory)
	start := func(cmd *exec.Cmd) (func(*exec.Cmd) error, error) {
		job, err := startProviderNamespace(ctx, cmd, providerNamespaceLauncher{Path: config.NamespaceLauncher, SHA256: config.NamespaceLauncherSHA256})
		if err != nil {
			return nil, err
		}
		return func(*exec.Cmd) error { return job.close() }, nil
	}
	provider, err := startProcessProviderUsing(ctx, filepath.Join(config.RuntimeDirectory, filepath.FromSlash(config.Python)), args, descriptor, env, config.RuntimeDirectory, start)
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

// Model directories remain managed by the ordinary package store. The worker
// verifies every graph/tokenizer/settings byte snapshot against the lease pins
// before parsing; it never trusts mutable paths as model identity.
func readLinuxONNXSettings(ctx context.Context, model *models.ONNXEmbeddingLease) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var expected models.PackageArtifact
	for _, artifact := range model.Manifest.Artifacts {
		if artifact.Path == "embedding.json" {
			expected = artifact
		}
	}
	if expected.Size < 1 || expected.Size > 64<<10 {
		return nil, errors.New("ONNX settings artifact is invalid")
	}
	path := filepath.Join(model.ModelDir, "embedding.json")
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NONBLOCK|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() != expected.Size {
		return nil, errors.New("ONNX settings snapshot is not the declared regular file")
	}
	raw, err := io.ReadAll(io.LimitReader(file, expected.Size+1))
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(raw)
	if int64(len(raw)) != expected.Size || hex.EncodeToString(digest[:]) != expected.SHA256 {
		return nil, errors.New("ONNX settings snapshot digest changed")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return raw, nil
}

func checkONNXRuntimePath(path string, directory bool) error {
	return managedLinuxRootPath(path, directory)
}

func validateONNXRuntimeImportPath(_ string, path string, _ *os.File) error {
	if filepath.Base(path) == "pyvenv.cfg" {
		return errors.New("ONNX runtime requires a self-contained immutable Python installation")
	}
	return nil
}
