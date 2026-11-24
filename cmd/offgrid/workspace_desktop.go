package main

import (
	"context"
	"errors"
	"io"

	"github.com/takuphilchan/offgrid-llm/internal/storage"
)

type desktopMaintenanceSummary struct {
	Files              int    `json:"files"`
	Bytes              int64  `json:"bytes"`
	ApplicationVersion string `json:"application_version"`
	Encryption         string `json:"encryption"`
	Authenticated      bool   `json:"authenticated"`
	Published          bool   `json:"published"`
	DatabaseChecked    bool   `json:"database_checked"`
}

type desktopMaintenanceState struct {
	Protocol int                          `json:"protocol"`
	ID       string                       `json:"id"`
	Phase    string                       `json:"phase"`
	Outcome  string                       `json:"outcome,omitempty"`
	Code     string                       `json:"code,omitempty"`
	Summary  *desktopMaintenanceSummary   `json:"summary,omitempty"`
	Retained []desktopMaintenanceRetained `json:"retained,omitempty"`
}

type desktopMaintenanceRetained struct {
	Path              string `json:"path"`
	Directory         bool   `json:"directory"`
	ContainsPlaintext bool   `json:"contains_plaintext"`
}

func runDesktopMaintenance(parent context.Context, input io.ReadCloser, output io.Writer, version string) int {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	defer input.Close()
	var request desktopMaintenanceRequest
	if err := readDesktopMaintenanceStart(ctx, input, &request); err != nil {
		return 2
	}
	defer func() { request.KeyMaterial, request.Passphrase = "", "" }()
	write, read, err := request.options(ctx)
	if err != nil {
		if validMaintenanceID(request.ID) {
			_ = desktopWriteFrame(output, desktopMaintenanceState{Protocol: 1, ID: request.ID, Phase: "finished", Outcome: "failed", Code: "invalid_request"})
			return 1
		}
		return 2
	}
	request.KeyMaterial, request.Passphrase = "", ""
	control := make(chan error, 1)
	stopInput := context.AfterFunc(ctx, func() { interruptDesktopInput(input) })
	defer stopInput()
	go func() {
		var command struct {
			Protocol int    `json:"protocol"`
			Kind     string `json:"kind"`
			ID       string `json:"id"`
		}
		err := desktopReadFrame(desktopPrivateInput(ctx, input), 4096, &command)
		if ctx.Err() == nil && (err != nil && err != io.EOF || err == nil && (command.Protocol != 1 || command.Kind != "cancel" || command.ID != request.ID)) {
			err = errDesktopProtocol
		}
		control <- err
		cancel()
	}()
	state := desktopMaintenanceState{Protocol: 1, ID: request.ID, Phase: "working"}
	if err = desktopWriteFrame(output, state); err != nil {
		cancel()
		interruptDesktopInput(input)
		<-control
		return 1
	}
	var manifest *storage.BackupManifest
	switch request.Operation {
	case "backup":
		manifest, err = storage.BackupWorkspaceWithOptions(ctx, request.DataDir, request.Destination, version, write)
	case "verify":
		manifest, err = storage.VerifyBackupWithOptions(ctx, request.Source, read)
	case "restore":
		manifest, err = storage.RestoreWorkspaceWithOptions(ctx, request.Source, request.Destination, version, read)
	}
	cancel()
	interruptDesktopInput(input)
	controlErr := <-control
	state, exitCode := desktopMaintenanceOutcome(request, manifest, err, controlErr)
	if desktopWriteFrame(output, state) != nil {
		return 1
	}
	return exitCode
}

// Projection is shared with the isolated post-publication subprocess fixture.
// A storage error may still carry a verified, published archive; never discard
// that evidence or infer that cancellation prevented publication.
func desktopMaintenanceOutcome(request desktopMaintenanceRequest, manifest *storage.BackupManifest, err, controlErr error) (desktopMaintenanceState, int) {
	state := desktopMaintenanceState{Protocol: 1, ID: request.ID, Phase: "finished", Outcome: "succeeded"}
	exitCode := 0
	if err != nil {
		state.Outcome, state.Code, exitCode = "failed", "operation_failed", 1
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			state.Outcome, state.Code, exitCode = "cancelled", "cancelled", 130
		}
	}
	if errors.Is(controlErr, errDesktopProtocol) {
		state.Outcome, state.Code, exitCode = "failed", "invalid_control", 1
	}
	var retained *storage.BackupRetainedError
	if errors.As(err, &retained) && maintenancePath(retained.Path) {
		state.Retained = []desktopMaintenanceRetained{{Path: retained.Path, Directory: retained.Directory, ContainsPlaintext: retained.Directory || request.Encryption == "legacy"}}
	}
	if manifest != nil {
		state.Summary = &desktopMaintenanceSummary{Files: len(manifest.Files), Bytes: manifest.Bytes, ApplicationVersion: manifest.ApplicationVersion,
			Encryption: manifest.Encryption, Authenticated: manifest.Encryption == storage.BackupEncryptionAge,
			Published: request.Operation != "verify", DatabaseChecked: request.Operation == "restore"}
	}
	return state, exitCode
}
