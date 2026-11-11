package inference

import (
	"context"
	"errors"
	"os"
	"runtime"
	"strings"
	"time"
)

var ErrRuntimePreparationBusy = errors.New("inference runtime preparation is already running")

// RuntimeReadiness describes engine preparation, not model or workflow quality.
// No machine paths or secrets are exposed by this public representation.
type RuntimeReadiness struct {
	State            string `json:"state"`
	Mode             string `json:"mode"`
	GPUDetected      bool   `json:"gpu_detected"`
	DownloadRequired bool   `json:"download_required"`
	DownloadBytes    int64  `json:"download_bytes"`
	DownloadedBytes  int64  `json:"downloaded_bytes"`
	Revision         string `json:"revision"`
	Message          string `json:"message,omitempty"`
}

func (bm *BinaryManager) runtimeMode() string {
	if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
		return "metal"
	}
	if bm.gpuRequested() {
		return bm.GPUType()
	}
	return "cpu"
}

// Published b10516 archive sizes, paired with the digest-pinned asset table.
func runtimeDownloadBytes() int64 {
	return map[string]int64{
		"linux/amd64": 16667775, "linux/arm64": 13529264,
		"darwin/amd64": 11395897, "darwin/arm64": 11089823,
		"windows/amd64": 18506923, "windows/arm64": 12256210,
	}[runtime.GOOS+"/"+runtime.GOARCH]
}

func (bm *BinaryManager) RuntimeStatus() RuntimeReadiness {
	bm.readinessMu.Lock()
	defer bm.readinessMu.Unlock()
	status := bm.readiness
	if status.State != "" && status.Mode == bm.runtimeMode() && (status.State != "ready" || regularFile(bm.readyPath)) {
		return status
	}
	status = RuntimeReadiness{State: "unchecked", Mode: bm.runtimeMode(), GPUDetected: bm.HasGPU(), Revision: bm.version}
	if !bm.gpuRequested() {
		status.DownloadBytes = runtimeDownloadBytes()
	}
	explicit := strings.TrimSpace(os.Getenv("OFFGRID_LLAMA_SERVER_PATH"))
	if explicit == "" && bm.GetInstalledPath() == "" {
		status.State = "missing"
		status.DownloadRequired = !bm.gpuRequested()
		if status.DownloadRequired {
			status.DownloadBytes = runtimeDownloadBytes()
		}
	}
	if bm.gpuRequested() {
		status.Message = "GPU mode requires a compatible installed runtime. Preparation checks it; it does not download CUDA or GPU drivers."
	}
	return status
}

func (bm *BinaryManager) updateReadiness(update func(*RuntimeReadiness)) {
	bm.readinessMu.Lock()
	defer bm.readinessMu.Unlock()
	update(&bm.readiness)
}

func (bm *BinaryManager) PrepareRuntime(ctx context.Context) (string, error) {
	return bm.prepareRuntime(ctx, true)
}

func (bm *BinaryManager) prepareRuntime(parent context.Context, verify bool) (path string, err error) {
	if !bm.prepareMu.TryLock() {
		return "", ErrRuntimePreparationBusy
	}
	defer bm.prepareMu.Unlock()
	ctx, cancel := context.WithTimeout(parent, 5*time.Minute)
	defer cancel()
	initial := bm.RuntimeStatus()
	initial.State, initial.Message, initial.DownloadedBytes = "checking", "", 0
	initial.Mode, initial.GPUDetected = bm.runtimeMode(), bm.HasGPU()
	bm.updateReadiness(func(state *RuntimeReadiness) { *state = initial })
	defer func() {
		bm.updateReadiness(func(state *RuntimeReadiness) {
			if err != nil {
				state.State = "failed"
				state.Message = "Runtime preparation failed. Check connectivity, disk space and runtime configuration, then retry. Existing models and work are preserved."
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					state.State = "cancelled"
					state.Message = "Preparation stopped. You can retry."
				}
				if bm.gpuRequested() {
					state.Message = "GPU runtime is unavailable or incompatible. Install a compatible GPU runtime or choose CPU mode."
				}
			} else if initial.Mode != bm.runtimeMode() {
				state.State, state.Message = "unchecked", "Runtime settings changed. Prepare the selected mode again."
			} else {
				state.State, state.Message, state.DownloadRequired = "ready", "Engine ready. Send a prompt to verify your selected model.", false
				bm.readyPath = path
				if !verify {
					state.State = "unchecked"
				}
			}
		})
	}()
	if err = ctx.Err(); err != nil {
		return "", err
	}
	path, err = bm.resolveLlamaServer(ctx)
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err != nil {
		return "", err
	}
	if verify && !llamaServerSupportsRequiredFlagsContext(ctx, path) {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", errors.New("installed runtime lacks required inference flags")
	}
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	return path, nil
}

type runtimeProgress struct{ manager *BinaryManager }

func (p runtimeProgress) Write(data []byte) (int, error) {
	p.manager.updateReadiness(func(state *RuntimeReadiness) { state.DownloadedBytes += int64(len(data)) })
	return len(data), nil
}

func (mc *ModelCache) RuntimeStatus() RuntimeReadiness { return mc.binManager.RuntimeStatus() }
func (mc *ModelCache) PrepareRuntime(ctx context.Context) error {
	_, err := mc.binManager.PrepareRuntime(ctx)
	return err
}
