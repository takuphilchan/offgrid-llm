package resource

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/shirou/gopsutil/v3/cpu"
)

// QualificationHardwareID measures a hardware profile, not a user/machine ID.
// Do not include serial numbers, hostnames, free memory, clocks or temperature.
// Runtime launch configuration separately binds thread/device selection.
func QualificationHardwareID(ctx context.Context, resources *SystemResources) (string, error) {
	info, err := cpu.InfoWithContext(ctx)
	if err != nil {
		return "", err
	}
	return qualificationHardwareID(ctx, resources, info)
}

func qualificationHardwareID(ctx context.Context, r *SystemResources, processors []cpu.InfoStat) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if r == nil || r.TotalRAM <= 0 || r.CPUCores <= 0 || r.OS == "" || r.Arch == "" || len(processors) == 0 {
		return "", errors.New("hardware qualification identity is incomplete")
	}
	if r.GPUAvailable && (r.GPUName == "" || r.GPUMemory <= 0) {
		return "", errors.New("GPU qualification identity is incomplete")
	}
	unique := map[string]bool{}
	for _, p := range processors {
		if strings.TrimSpace(p.ModelName) == "" {
			return "", errors.New("CPU model identity is unavailable")
		}
		flags := append([]string(nil), p.Flags...)
		sort.Strings(flags)
		data, err := json.Marshal(struct {
			Vendor, Family, Model, Name string
			Flags                       []string
		}{p.VendorID, p.Family, p.Model, p.ModelName, flags})
		if err != nil {
			return "", err
		}
		unique[string(data)] = true
	}
	identities := make([]string, 0, len(unique))
	for identity := range unique {
		identities = append(identities, identity)
	}
	sort.Strings(identities)
	gpuName, gpuMemory := "", int64(0)
	if r.GPUAvailable {
		gpuName, gpuMemory = r.GPUName, r.GPUMemory
	}
	data, err := json.Marshal(struct {
		Version    int
		OS, Arch   string
		RAM        int64
		CPUCores   int
		Processors []string
		GPUName    string
		GPUMemory  int64
	}{1, r.OS, r.Arch, r.TotalRAM, r.CPUCores, identities, gpuName, gpuMemory})
	if err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}
