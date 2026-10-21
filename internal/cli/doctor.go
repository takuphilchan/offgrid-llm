package cli

import (
	"context"
	"errors"
	"fmt"
)

// DoctorSnapshot is observed by the OS adapter without modifying installed
// model storage. Rendering it is deterministic and can be tested in isolation.
type DoctorSnapshot struct {
	ResourcesError error
	OS, Arch       string
	CPUCores       int
	RAMBytes       int64
	GPUName        string
	GPUMemoryBytes int64
	ConfigPath     string
	ConfigPresent  bool
	ModelsDir      string
	StoragePresent bool
	ModelCount     int
	ModelScanError error
	NetworkOnline  bool
	ServerOnline   bool
	ServerPort     int
}

func doctorCommand(ctx context.Context, args []string, env Environment) error {
	if env.Doctor == nil {
		return errors.New("missing doctor observer")
	}
	snapshot, err := env.Doctor(ctx)
	if err != nil {
		return err
	}
	p := Presenter{Stdout: env.Stdout, Stderr: env.Stderr, JSON: env.Options.JSON, Style: env.Style}
	p.Doctor(snapshot)
	return nil
}

func (p Presenter) Doctor(snapshot DoctorSnapshot) {
	s := p.Style
	_, _ = fmt.Fprintln(p.Stdout)
	_, _ = fmt.Fprintf(p.Stdout, "  %s%s System Diagnostics%s\n\n", s.Primary+s.Bold, s.Search, s.Reset)
	allPassed, checks, passed := true, 0, 0
	printCheck := func(success, optional bool, name, detail string) {
		checks++
		icon, color := s.Check, s.Success
		if !success {
			if optional {
				icon, color = "○", s.Muted
			} else {
				icon, color = s.Cross, s.Error
				allPassed = false
			}
		} else {
			passed++
		}
		_, _ = fmt.Fprintf(p.Stdout, "    %s%s%s  %-12s %s%s%s\n", color, icon, s.Reset, name, s.Muted, detail, s.Reset)
	}
	if snapshot.ResourcesError != nil {
		printCheck(false, false, "Resources", fmt.Sprintf("Failed (%v)", snapshot.ResourcesError))
	} else {
		printCheck(true, false, "Resources", fmt.Sprintf("%s/%s · %d cores · %s RAM", snapshot.OS, snapshot.Arch, snapshot.CPUCores, FormatBytes(snapshot.RAMBytes)))
		if snapshot.GPUName != "" {
			_, _ = fmt.Fprintf(p.Stdout, "              %s└─ GPU: %s (%s)%s\n", s.Muted, snapshot.GPUName, FormatBytes(snapshot.GPUMemoryBytes), s.Reset)
		}
	}
	if snapshot.ConfigPresent {
		printCheck(true, false, "Config", snapshot.ConfigPath)
	} else {
		printCheck(true, true, "Config", "Using defaults")
	}
	if snapshot.StoragePresent {
		printCheck(true, false, "Storage", fmt.Sprintf("%s (present; writability not tested)", snapshot.ModelsDir))
		if snapshot.ModelScanError == nil {
			_, _ = fmt.Fprintf(p.Stdout, "              %s└─ %d model(s) installed%s\n", s.Muted, snapshot.ModelCount, s.Reset)
		}
	} else {
		printCheck(false, false, "Storage", fmt.Sprintf("Missing %s", snapshot.ModelsDir))
	}
	if snapshot.NetworkOnline {
		printCheck(true, false, "Network", "Online (HuggingFace reachable)")
	} else {
		printCheck(true, true, "Network", "Offline mode")
	}
	if snapshot.ServerOnline {
		printCheck(true, false, "Server", fmt.Sprintf("Running on port %d", snapshot.ServerPort))
	} else {
		printCheck(true, true, "Server", "Not running")
	}
	_, _ = fmt.Fprintln(p.Stdout)
	if allPassed {
		_, _ = fmt.Fprintf(p.Stdout, "    %s%s All checks passed%s (%d/%d)\n", s.Success, s.Check, s.Reset, passed, checks)
	} else {
		_, _ = fmt.Fprintf(p.Stdout, "    %s%s Some checks failed%s (%d/%d passed)\n", s.Error, s.Cross, s.Reset, passed, checks)
	}
	_, _ = fmt.Fprintln(p.Stdout)
}
