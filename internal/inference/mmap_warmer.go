package inference

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/resource"
)

const warmByteLimit int64 = 512 << 20
const warmHistoryLimit = 32

// MmapWarmer performs bounded speculative reads. The OS owns page-cache
// residency; reading a file cannot guarantee that its pages remain resident.
type MmapWarmer struct {
	modelsDir    string
	mu           sync.RWMutex
	warmedModels map[string]WarmStatus
	busy         atomic.Bool
	paused       atomic.Bool
	epoch        atomic.Uint64
	totalWarmed  atomic.Int64
	memory       func() (*resource.SystemResources, error)
}

type WarmStatus struct {
	ModelPath   string        `json:"model_path"`
	ModelName   string        `json:"model_name"`
	SizeBytes   int64         `json:"size_bytes"`
	SizeMB      int64         `json:"size_mb"`
	WarmedAt    time.Time     `json:"warmed_at"`
	WarmTime    time.Duration `json:"warm_time"`
	InPageCache bool          `json:"in_page_cache"` // Compatibility hint: full read completed, not pinned RAM.
	BytesRead   int64         `json:"bytes_read"`
	Complete    bool          `json:"complete"`
	Error       string        `json:"error,omitempty"`
	modifiedAt  time.Time
	sequence    int64
}

func NewMmapWarmer(modelsDir string) *MmapWarmer {
	return &MmapWarmer{modelsDir: modelsDir, warmedModels: make(map[string]WarmStatus), memory: resource.DetectMemoryResources}
}
func (w *MmapWarmer) Pause()         { w.paused.Store(true); w.epoch.Add(1) }
func (w *MmapWarmer) Resume()        { w.paused.Store(false) }
func (w *MmapWarmer) IsPaused() bool { return w.paused.Load() }
func (w *MmapWarmer) begin() (uint64, error) {
	epoch := w.epoch.Load()
	if w.IsPaused() {
		return 0, fmt.Errorf("warming paused during active model loading")
	}
	if !w.busy.CompareAndSwap(false, true) {
		return 0, fmt.Errorf("warming already active")
	}
	return epoch, nil
}
func (w *MmapWarmer) budget() (int64, error) {
	r, err := w.memory()
	if err != nil || r == nil || r.TotalRAM <= 0 {
		return 0, fmt.Errorf("cannot measure memory for speculative warming")
	}
	spare := r.AvailableHostMemoryMB() - max(int64(1024), r.TotalRAM/10)
	if spare < 256 {
		return 0, fmt.Errorf("warming skipped: insufficient memory headroom")
	}
	return min(warmByteLimit, (spare/4)<<20), nil
}

// warm runs only while the caller holds busy. One buffer and no producer
// goroutine means cancellation cannot strand a sender or retain a read buffer.
func (w *MmapWarmer) warm(path string, epoch uint64, remaining *int64, deadline time.Time) (*WarmStatus, error) {
	start := time.Now()
	status := &WarmStatus{ModelPath: path, ModelName: filepath.Base(path)}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("warming requires a regular model file")
	}
	status.SizeBytes, status.SizeMB = info.Size(), info.Size()>>20
	status.modifiedAt = info.ModTime()
	if previous := w.GetStatus(path); previous != nil && previous.Error == "" &&
		previous.SizeBytes == info.Size() && previous.modifiedAt.Equal(info.ModTime()) && time.Since(previous.WarmedAt) < 10*time.Minute {
		return previous, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := make([]byte, 1<<20)
	for status.BytesRead < status.SizeBytes {
		if w.IsPaused() || w.epoch.Load() != epoch {
			err = fmt.Errorf("warming interrupted")
			break
		}
		if time.Now().After(deadline) {
			err = fmt.Errorf("warming time budget reached")
			break
		}
		if *remaining <= 0 {
			break
		}
		if _, pressureErr := w.budget(); pressureErr != nil {
			err = pressureErr
			break
		}
		n, readErr := f.Read(buf[:min(int64(len(buf)), *remaining)])
		status.BytesRead += int64(n)
		*remaining -= int64(n)
		if readErr != nil {
			if readErr != io.EOF || status.BytesRead != status.SizeBytes {
				err = fmt.Errorf("warming read failed: %w", readErr)
			}
			break
		}
		if n == 0 {
			err = io.ErrNoProgress
			break
		}
	}
	status.WarmTime, status.WarmedAt = time.Since(start), time.Now()
	if w.IsPaused() || w.epoch.Load() != epoch {
		err = fmt.Errorf("warming interrupted")
	}
	status.Complete = err == nil && status.BytesRead == status.SizeBytes
	status.InPageCache = status.Complete
	if err != nil {
		status.Error = err.Error()
	}
	if status.BytesRead > 0 {
		w.remember(*status)
	}
	return status, err
}
func (w *MmapWarmer) remember(status WarmStatus) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, exists := w.warmedModels[status.ModelPath]; !exists && len(w.warmedModels) >= warmHistoryLimit {
		oldest := ""
		var sequence int64
		for key, value := range w.warmedModels {
			if oldest == "" || value.sequence < sequence {
				oldest, sequence = key, value.sequence
			}
		}
		delete(w.warmedModels, oldest)
	}
	status.sequence = w.totalWarmed.Add(1)
	w.warmedModels[status.ModelPath] = status
}
func (w *MmapWarmer) WarmModel(path string) (*WarmStatus, error) {
	epoch, err := w.begin()
	if err != nil {
		return nil, err
	}
	defer w.busy.Store(false)
	budget, err := w.budget()
	if err != nil {
		return nil, err
	}
	return w.warm(path, epoch, &budget, time.Now().Add(30*time.Second))
}

// FastWarmModel uses the same limits; speculative speed never bypasses pressure.
func (w *MmapWarmer) FastWarmModel(path string) (*WarmStatus, error) { return w.WarmModel(path) }
func (w *MmapWarmer) WarmModelAsync(path string)                     { w.WarmModelsAsync([]string{path}) }
func (w *MmapWarmer) WarmOnDemand(path string)                       { w.WarmModelAsync(path) }

// Admission happens before spawning, so repeated hover requests cannot create
// an unbounded goroutine queue. A batch shares one byte/time budget.
func (w *MmapWarmer) WarmModelsAsync(paths []string) {
	epoch, err := w.begin()
	if err != nil {
		return
	}
	paths = append([]string(nil), paths[:min(len(paths), warmHistoryLimit)]...)
	go func() {
		defer w.busy.Store(false)
		budget, err := w.budget()
		if err != nil {
			return
		}
		deadline := time.Now().Add(30 * time.Second)
		for _, path := range paths {
			if _, err := w.warm(path, epoch, &budget, deadline); err != nil || budget <= 0 {
				return
			}
		}
	}()
}

// WarmAllModels is retained for callers, but visits at most 32 files and reads
// at most the shared budget. It does not populate RAM with an entire library.
func (w *MmapWarmer) WarmAllModels() ([]*WarmStatus, error) {
	epoch, err := w.begin()
	if err != nil {
		return nil, err
	}
	defer w.busy.Store(false)
	budget, err := w.budget()
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(30 * time.Second)
	var results []*WarmStatus
	err = filepath.WalkDir(w.modelsDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if w.IsPaused() || w.epoch.Load() != epoch {
			return fmt.Errorf("warming interrupted")
		}
		if time.Now().After(deadline) || len(results) >= warmHistoryLimit || budget <= 0 {
			return filepath.SkipAll
		}
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".gguf") {
			return nil
		}
		status, err := w.warm(path, epoch, &budget, deadline)
		if status != nil {
			results = append(results, status)
		}
		return err
	})
	return results, err
}
func (w *MmapWarmer) WarmPriorityModels(defaultModel string, others []string) {
	names := append([]string{defaultModel}, others[:min(len(others), warmHistoryLimit-1)]...)
	paths := make([]string, 0, len(names))
	for _, name := range names {
		if name != "" {
			if path := w.findModelPath(name); path != "" {
				paths = append(paths, path)
			}
		}
	}
	w.WarmModelsAsync(paths)
}
func (w *MmapWarmer) findModelPath(name string) string {
	for _, path := range []string{name, filepath.Join(w.modelsDir, name), filepath.Join(w.modelsDir, name+".gguf")} {
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			return path
		}
	}
	var found string
	_ = filepath.WalkDir(w.modelsDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		base := entry.Name()
		if strings.TrimSuffix(base, filepath.Ext(base)) == name || base == name {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	return found
}
func (w *MmapWarmer) GetStatus(path string) *WarmStatus {
	w.mu.RLock()
	defer w.mu.RUnlock()
	status, ok := w.warmedModels[path]
	if !ok {
		return nil
	}
	return &status
}
func (w *MmapWarmer) GetAllStatus() []*WarmStatus {
	w.mu.RLock()
	defer w.mu.RUnlock()
	var results []*WarmStatus
	for _, status := range w.warmedModels {
		copy := status
		results = append(results, &copy)
	}
	return results
}
func (w *MmapWarmer) IsWarmed(path string) bool {
	status := w.GetStatus(path)
	return status != nil && status.Complete && time.Since(status.WarmedAt) < 10*time.Minute
}
func (w *MmapWarmer) InvalidateCache(path string) {
	w.mu.Lock()
	delete(w.warmedModels, path)
	w.mu.Unlock()
}
func (w *MmapWarmer) Stats() map[string]interface{} {
	statuses := w.GetAllStatus()
	var bytes int64
	for _, s := range statuses {
		bytes += s.BytesRead
	}
	return map[string]interface{}{"warmed_models": len(statuses), "total_warmed_mb": bytes >> 20, "total_warmed_gb": float64(bytes) / (1 << 30), "lifetime_warmups": w.totalWarmed.Load(), "warming": w.busy.Load()}
}
