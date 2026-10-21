package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
)

// ResolveVersion retains the existing release-tag and development-file rules.
func ResolveVersion(buildVersion string, readFile func(string) ([]byte, error)) string {
	if buildVersion != "dev" {
		if len(buildVersion) > 1 && buildVersion[0] == 'v' && buildVersion[1] >= '0' && buildVersion[1] <= '9' {
			return buildVersion[1:]
		}
		return buildVersion
	}
	if readFile != nil {
		for _, path := range []string{"VERSION", "/var/lib/offgrid/VERSION", "/usr/local/share/offgrid/VERSION"} {
			if data, err := readFile(path); err == nil {
				return strings.TrimSpace(string(data))
			}
		}
	}
	return buildVersion
}

// RunVersion writes to supplied streams only. The process adapter owns the
// release version, file access, device probe, and terminal style.
func RunVersion(_ context.Context, _ []string, env Environment, presenter Presenter) error {
	version := ResolveVersion(env.Version, env.ReadFile)
	device := DeviceInfo{}
	if env.Device != nil {
		device = env.Device()
	}
	if env.Options.JSON {
		data := map[string]interface{}{
			"version": version,
			"go":      runtime.Version(),
			"os":      runtime.GOOS,
			"arch":    runtime.GOARCH,
		}
		if device.GPUName != "" {
			data["gpu"] = device.GPUName
			data["gpu_memory_mb"] = device.GPUMemoryMB
		}
		encoder := json.NewEncoder(env.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(data)
	}
	_, _ = fmt.Fprintln(env.Stdout)
	_, _ = fmt.Fprintf(env.Stdout, "  %s%s OffGrid LLM%s\n", presenter.Style.Primary+presenter.Style.Bold, presenter.Style.Bolt, presenter.Style.Reset)
	_, _ = fmt.Fprintln(env.Stdout)
	presenter.KeyValue("Version", version)
	presenter.KeyValue("Platform", fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH))
	if device.GPUName != "" {
		presenter.KeyValue("GPU", fmt.Sprintf("%s (%dMB)", device.GPUName, device.GPUMemoryMB))
	} else {
		presenter.KeyValue("Device", "CPU only")
	}
	_, _ = fmt.Fprintln(env.Stdout)
	return nil
}
