package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

type hostProtectionInspectionOptions struct {
	status *bool
	after  *string
	limit  *int
}

func registerHostProtectionInspection(flags *flag.FlagSet) hostProtectionInspectionOptions {
	return hostProtectionInspectionOptions{
		flags.Bool("host-protection-status", false, "Inspect this account's retained host protection without changing it"),
		flags.String("host-protection-after", "", "Continue host protection inspection after this opaque record ID"),
		flags.Int("host-protection-limit", 32, "Maximum host protection records to inspect (1-64)"),
	}
}

func (options hostProtectionInspectionOptions) run(flags *flag.FlagSet, stdio, connect bool, grant, state string) (bool, error) {
	requested := *options.status
	flags.Visit(func(option *flag.Flag) {
		if option.Name == "host-protection-after" || option.Name == "host-protection-limit" {
			requested = true
		}
	})
	if !requested {
		return false, nil
	}
	if !*options.status || stdio || connect || grant != "" || state != "" || flags.NArg() != 0 {
		return true, execution.ErrAuthority
	}
	return true, inspectHostProtection(os.Stdout, *options.after, *options.limit)
}

// A trusted local diagnostic only: no caller-selected registry, file mutation,
// model tool, enrollment or execution capability is accepted by this operation.
func inspectHostProtection(output io.Writer, after string, limit int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	registry, err := execution.AccountHostProtectionRegistry()
	if err != nil {
		return err
	}
	report, err := registry.Inspect(ctx, after, limit)
	if err == nil {
		return json.NewEncoder(output).Encode(report)
	}
	reason := "inspection_failed"
	var snapshot *execution.HostProtectionSnapshotUnavailableError
	switch {
	case errors.As(err, &snapshot):
		reason = snapshot.Reason
	case errors.Is(err, execution.ErrHostProtectionBusy):
		reason = "owner_busy"
	case errors.Is(err, execution.ErrHostProtectionCorrupt):
		reason = "registry_corrupt"
	case errors.Is(err, execution.ErrAuthority):
		reason = "private_authority_unavailable"
	case errors.Is(err, execution.ErrHostProtectionLimit):
		reason = "inspection_limit"
	}
	writeErr := json.NewEncoder(output).Encode(struct {
		Version int    `json:"version"`
		Status  string `json:"status"`
		Reason  string `json:"reason"`
	}{1, "unavailable", reason})
	return errors.Join(err, writeErr)
}
