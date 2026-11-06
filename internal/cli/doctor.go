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
	p.Heading("OffGrid / Diagnostics")
	p.Note("Observed checks only; model generation and storage writes are not tested.")
	problems := 0
	check := func(name, value string, ok bool) {
		state := "ready"
		if !ok {
			state = "needs attention"
			problems++
		}
		p.Detail(name, state)
		p.Note(value)
	}
	if snapshot.ResourcesError != nil {
		check("Resources", snapshot.ResourcesError.Error(), false)
	} else {
		check("Resources", fmt.Sprintf("%s/%s | %d cores | %s RAM", snapshot.OS, snapshot.Arch, snapshot.CPUCores, FormatBytes(snapshot.RAMBytes)), true)
		if snapshot.GPUName != "" {
			p.Detail("GPU", fmt.Sprintf("%s (%s)", snapshot.GPUName, FormatBytes(snapshot.GPUMemoryBytes)))
		}
	}
	if snapshot.ConfigPresent {
		p.Detail("Configuration", snapshot.ConfigPath)
	} else {
		p.Detail("Configuration", "Using defaults")
	}
	if snapshot.StoragePresent {
		check("Storage", fmt.Sprintf("%s (present; writability not tested)", snapshot.ModelsDir), true)
		if snapshot.ModelScanError != nil {
			check("Model scan", snapshot.ModelScanError.Error(), false)
		} else {
			p.Note(fmt.Sprintf("%d model(s) installed", snapshot.ModelCount))
		}
	} else {
		check("Storage", "Missing "+snapshot.ModelsDir, false)
	}
	if snapshot.NetworkOnline {
		p.Detail("Model downloads", "Hugging Face reachable")
	} else {
		p.Detail("Model downloads", "Offline mode; local work does not require downloads")
	}
	if snapshot.ServerOnline {
		check("Service", fmt.Sprintf("Responding on port %d", snapshot.ServerPort), true)
	} else {
		check("Service", "Not reachable. Check the configured endpoint; open OffGrid Desktop or start the intended service with offgrid serve.", false)
	}
	if problems == 0 {
		p.Heading("Observed checks ready")
	} else {
		p.Heading(fmt.Sprintf("%d check(s) need attention", problems))
	}
}
