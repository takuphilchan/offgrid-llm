package inference

import (
	"math"
	"os/exec"

	"github.com/shirou/gopsutil/v3/process"
)

// ProcessMemorySnapshot observes the cache-owned llama children without
// holding the cache lock across OS calls. RSS is advisory host memory only:
// mmap sharing, CUDA/Metal allocations, and device limits are not inferred.
func (mc *ModelCache) ProcessMemorySnapshot() LegacyProcessMemory {
	type candidate struct {
		modelID         string
		cmd             *exec.Cmd
		processCreateMS int64
	}
	mc.mu.RLock()
	candidates := make([]candidate, 0, len(mc.instances))
	for modelID, instance := range mc.instances {
		candidates = append(candidates, candidate{modelID: modelID, cmd: instance.Cmd, processCreateMS: instance.ProcessCreateMS})
	}
	mc.mu.RUnlock()
	report := LegacyProcessMemory{Residents: len(candidates)}
	for _, candidate := range candidates {
		if candidate.cmd == nil || candidate.cmd.Process == nil || candidate.cmd.ProcessState != nil || candidate.processCreateMS == 0 {
			report.RSSUnmeasured++
			continue
		}
		proc, err := process.NewProcess(int32(candidate.cmd.Process.Pid))
		if err != nil {
			report.RSSUnmeasured++
			continue
		}
		created, err := proc.CreateTime()
		if err != nil || created != candidate.processCreateMS {
			report.RSSUnmeasured++
			continue
		}
		info, err := proc.MemoryInfo()
		mc.mu.RLock()
		current := mc.instances[candidate.modelID]
		unchanged := current != nil && current.Cmd == candidate.cmd && current.ProcessCreateMS == candidate.processCreateMS && candidate.cmd.ProcessState == nil
		mc.mu.RUnlock()
		if err != nil || info == nil || info.RSS == 0 || info.RSS > math.MaxInt64 || !unchanged || int64(info.RSS) > math.MaxInt64-report.RSSBytes {
			report.RSSUnmeasured++
			continue
		}
		report.RSSBytes += int64(info.RSS)
		report.RSSMeasured++
	}
	return report
}

func observedProcessCreateTime(pid int) int64 {
	proc, err := process.NewProcess(int32(pid))
	if err != nil {
		return 0
	}
	created, err := proc.CreateTime()
	if err != nil || created <= 0 {
		return 0
	}
	return created
}
