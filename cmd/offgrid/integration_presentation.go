package main

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"github.com/takuphilchan/offgrid-llm/internal/integrations"
)

func renderIntegrationReport(p cli.Presenter, action string, r integrations.HostReport, o integrations.HostOptions) {
	name := r.ID
	switch name {
	case "hermes":
		name = "Hermes"
	case "openclaw":
		name = "OpenClaw"
	}
	state := "Setup needed"
	if r.Ready {
		state = "Ready to test"
	}
	if action == "run" && r.Phase != "inspect" {
		state = strings.ReplaceAll(r.Phase, "_", " ")
	}
	p.Heading(name + " · " + state)
	p.Detail("Environment", runtime.GOOS+" · "+runtime.GOARCH)
	p.Detail("Service", o.ServerURL)
	if r.ModelID != "" {
		p.Detail("Model", r.ModelID)
	}
	if r.Runtime.Version != "" {
		p.Detail("Runtime", r.Runtime.Version)
	}
	labels := map[string]string{"runtime": "Runtime", "service": "Service", "plugin": "Provider", "model": "Model", "configuration": "Connection", "context": "Context"}
	blocked := false
	setupNeeded := false
	capacityBlocked := false
	serviceBlocked := false
	for _, c := range r.Checks {
		if c.State != "blocked" {
			continue
		}
		if !blocked {
			p.Heading("Needs attention")
			blocked = true
		}
		label := labels[c.ID]
		if label == "" {
			label = c.ID
		}
		p.Note(label + ": " + c.Message)
		capacityBlocked = capacityBlocked || c.ID == "model" || c.ID == "context"
		serviceBlocked = serviceBlocked || c.ID == "service"
		if c.ID == "runtime" || c.ID == "plugin" || c.ID == "configuration" {
			setupNeeded = true
		}
	}
	if setupNeeded || capacityBlocked || serviceBlocked {
		p.Heading("Next step")
		if serviceBlocked {
			p.Note("Restore the OffGrid service connection, then check status again.")
		} else if capacityBlocked {
			p.Note("Choose an installed model with sufficient context before setup or launch. Setup cannot increase available memory.")
			p.Detail("Model option", "--model MODEL")
		} else if setupNeeded {
			p.Note("Run " + integrationNextCommand(r.ID, "configure") + " in this environment to review setup.")
		}
		if runtime.GOOS == "windows" && !r.Runtime.Installed {
			p.Note("If you installed the agent in WSL, run OffGrid from that WSL terminal instead.")
		}
	}
	if r.Ready {
		for _, warning := range r.Service.Warnings {
			p.Note(warning)
		}
		if action != "run" {
			p.Detail("Model reply", r.Smoke.State)
			p.Detail("Tool workflow", r.Workflow.State)
			p.Note("Verify a reply: " + integrationNextCommand(r.ID, "test"))
		}
	}
	if r.Verification != nil {
		p.Detail("File contract", r.Verification.State+" ("+r.Verification.Code+")")
	}
	if r.RecoveryPath != "" {
		p.Detail("Config backup", r.RecoveryPath)
	}
	if o.DryRun {
		p.Note("Setup preview only. No files changed.")
	}
	fmt.Fprintln(p.Stdout)
}

func integrationNextCommand(id, action string) string {
	if id == "hermes" || id == "openclaw" {
		if action == "configure" {
			action = "install"
		}
		return "offgrid " + id + " " + action
	}
	return "offgrid integrations " + action + " " + id
}
