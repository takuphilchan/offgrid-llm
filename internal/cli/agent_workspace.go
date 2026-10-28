package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

type ExecutionLease interface {
	Resource() execution.Resource
	Done() <-chan struct{}
	Err() error
	Close() error
}
type ExecutionDependencies struct {
	Trust         func() (*execution.FolderTrustStore, error)
	OnDemand      func(context.Context, *serviceclient.Client) (bool, error)
	CurrentFolder func() (string, error)
	CaptureFolder func(string) (string, func() error, error)
	Interactive   func() bool
	Available     func(context.Context, *serviceclient.Client) (bool, error)
	Open          func(context.Context, *serviceclient.Client, string, execution.Permissions, func(execution.LocalApproval) bool) (ExecutionLease, error)
	OpenIsolated  func(context.Context, *serviceclient.Client, string, execution.Permissions, execution.IsolationPolicy, func(execution.LocalApproval) bool) (ExecutionLease, error)
}

func executionPermissions(access string) (execution.Permissions, error) {
	switch access {
	case "", "edit":
		return execution.Permissions{Read: true, Write: true}, nil
	case "read":
		return execution.Permissions{Read: true}, nil
	case "trusted":
		return execution.Permissions{Read: true, Write: true, Processes: true}, nil
	default:
		return execution.Permissions{}, Usage("Use --access read, edit or trusted")
	}
}

func openCLIWorkspace(ctx context.Context, client *serviceclient.Client, folder, access string, input *bufio.Reader, out io.Writer, env Environment) (ExecutionLease, error) {
	return openCLIWorkspacePolicy(ctx, client, folder, access, execution.IsolationPolicy{}, input, out, env)
}

func openCLIWorkspacePolicy(ctx context.Context, client *serviceclient.Client, folder, access string, isolation execution.IsolationPolicy, input *bufio.Reader, out io.Writer, env Environment) (ExecutionLease, error) {
	if env.Options.JSON {
		return nil, Usage("Approve a local host interactively first, then select its --workspace SESSION for JSON or unattended work")
	}
	permissions, err := executionPermissions(access)
	if err != nil {
		return nil, err
	}
	isolated := isolation != (execution.IsolationPolicy{})
	if !isolated && env.Execution.Open == nil || isolated && env.Execution.OpenIsolated == nil {
		return nil, fmt.Errorf("trusted native execution supervisor unavailable; no folder grant was created")
	}
	approve := func(approval execution.LocalApproval) bool {
		var trust *execution.FolderTrustStore
		if env.Execution.Trust != nil && approval.Actor != "" && approval.Backend == "local" {
			store, storeErr := env.Execution.Trust()
			if storeErr == nil {
				remembered, lookupErr := store.Lookup(approval)
				if lookupErr == nil {
					trust = store
				}
				if remembered && lookupErr == nil {
					fmt.Fprintf(out, "Using remembered folder access: %s · %s\n", TerminalSafe(approval.Root), TerminalSafe(approval.Host))
					return ctx.Err() == nil
				}
			}
		}
		mode := "read files"
		if approval.Permissions.Write {
			mode = "read and edit files"
		}
		if approval.Permissions.Processes {
			mode = "read/edit files and run trusted project commands with your OS account's authority (not a sandbox)"
		}
		if approval.Backend == "isolated_linux" {
			mode = fmt.Sprintf("access this folder and run commands in isolated Linux containers (image %s; network %s; %d MiB; %d CPU millis; %d processes). Only this folder is mounted at /workspace; writes to it affect your host files", TerminalSafe(approval.Isolation.Image), TerminalSafe(approval.Isolation.Network), approval.Isolation.MemoryBytes>>20, approval.Isolation.CPUMillis, approval.Isolation.Pids)
		}
		choices := "[y/N]"
		if trust != nil {
			choices = "[y/remember/N]"
			fmt.Fprintln(out, "Choose remember to trust only this exact folder, account, host, service and access level. Inspect/revoke with offgrid agent workspace trust.")
		}
		fmt.Fprintf(out, "Local workspace: %s\nHost: %s · Service: %s\nAllow OffGrid to %s for this host session? %s ", TerminalSafe(approval.Root), TerminalSafe(approval.Host), TerminalSafe(approval.Service), mode, choices)
		line, err := input.ReadString('\n')
		if err != nil {
			return false
		}
		answer := strings.ToLower(strings.TrimSpace(line))
		if answer == "remember" && trust != nil && ctx.Err() == nil {
			if err := trust.Remember(approval); err != nil {
				fmt.Fprintln(out, "Could not safely remember folder access. No access granted.")
				return false
			}
			return true
		}
		if answer != "y" && answer != "yes" {
			return false
		}
		fmt.Fprintln(out, "Preparing the approved local executor…")
		return ctx.Err() == nil
	}
	if isolated {
		return env.Execution.OpenIsolated(ctx, client, folder, permissions, isolation, approve)
	}
	return env.Execution.Open(ctx, client, folder, permissions, approve)
}

func agentWorkspaceCommand(ctx context.Context, args []string, env Environment) (result error) {
	stopInput := context.AfterFunc(ctx, func() {
		if input, ok := env.Stdin.(io.ReadCloser); ok {
			_ = input.Close()
		}
	})
	defer stopInput()
	if len(args) == 1 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h") {
		agentWorkspaceHelp(env)
		return nil
	}
	if len(args) == 0 {
		return Usage("agent workspace list | start FOLDER [--access read|edit|trusted] | stop SESSION")
	}
	client, err := serviceClient(env)
	if err != nil {
		return err
	}
	switch args[0] {
	case "trust":
		if env.Execution.Trust == nil {
			return fmt.Errorf("remembered folder trust unavailable")
		}
		store, err := env.Execution.Trust()
		if err != nil {
			return err
		}
		if len(args) == 3 && args[1] == "revoke" {
			if err = store.Revoke(args[2]); err != nil {
				return err
			}
			fmt.Fprintln(env.Stdout, "Remembered trust removed. Stop any already connected host with agent workspace stop SESSION; completed actions are not undone.")
			return nil
		}
		if len(args) > 2 || len(args) == 2 && args[1] != "list" {
			return Usage("agent workspace trust [list] | trust revoke ID")
		}
		records, err := store.List()
		if err != nil {
			return err
		}
		if env.Options.JSON {
			return json.NewEncoder(env.Stdout).Encode(records)
		}
		for _, r := range records {
			fmt.Fprintf(env.Stdout, "%s\n  %s · %s · %s\n  %s · %s · read=%t edit=%t commands=%t\n", r.ID, TerminalSafe(r.Root), TerminalSafe(r.Host), TerminalSafe(r.Actor), TerminalSafe(r.Service), TerminalSafe(r.Workspace), r.Permissions.Read, r.Permissions.Write, r.Permissions.Processes)
		}
		if len(records) == 0 {
			fmt.Fprintln(env.Stdout, "No remembered folder trust.")
		}
		return nil
	case "list":
		if len(args) != 1 {
			return Usage("agent workspace list")
		}
		var catalog struct {
			Workspaces []execution.Resource `json:"workspaces"`
			Protocol   int                  `json:"protocol"`
		}
		if err = client.JSON(ctx, http.MethodGet, "/api/v2/execution/workspaces", nil, &catalog); err != nil {
			return err
		}
		if catalog.Protocol != execution.ProtocolVersion {
			return fmt.Errorf("execution protocol mismatch")
		}
		if env.Options.JSON {
			return json.NewEncoder(env.Stdout).Encode(catalog)
		}
		if len(catalog.Workspaces) == 0 {
			fmt.Fprintln(env.Stdout, "No approved local execution hosts are connected.")
		}
		for _, resource := range catalog.Workspaces {
			fmt.Fprintf(env.Stdout, "%s  %s  %s  %s\n", TerminalSafe(resource.Reference.Session), TerminalSafe(resource.Reference.Host), TerminalSafe(resource.Backend), TerminalSafe(resource.Root))
		}
		return nil
	case "start":
		folder, access, isolation, err := workspaceStartOptions(args[1:])
		if err != nil {
			return err
		}
		lease, err := openCLIWorkspacePolicy(ctx, client, folder, access, isolation, bufio.NewReader(env.Stdin), env.Stdout, env)
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, closeExecutionLease(lease)) }()
		resource := lease.Resource()
		fmt.Fprintf(env.Stdout, "Execution host ready: %s\nFolder: %s\nUse --workspace %s in Chat/agent commands. Keep this host running; Ctrl+C revokes its local session.\n", TerminalSafe(resource.Reference.Session), TerminalSafe(resource.Root), TerminalSafe(resource.Reference.Session))
		select {
		case <-ctx.Done():
			return nil
		case <-lease.Done():
			return lease.Err()
		}
	case "stop":
		if len(args) != 2 {
			return Usage("agent workspace stop SESSION")
		}
		ref, _, err := resolveExecutionWorkspace(ctx, client, args[1])
		if err != nil {
			return err
		}
		if err = client.JSON(ctx, http.MethodPost, "/api/v2/execution/revoke", map[string]any{"reference": ref}, nil); err != nil {
			return err
		}
		fmt.Fprintln(env.Stdout, "Execution authority revoked. Inspect in-flight work before retrying; revocation does not undo file changes.")
		return nil
	default:
		return Usage("Unknown workspace operation")
	}
}

func workspaceStartOptions(args []string) (string, string, execution.IsolationPolicy, error) {
	var zero execution.IsolationPolicy
	fail := func() (string, string, execution.IsolationPolicy, error) {
		return "", "", zero, Usage("agent workspace start FOLDER [--access read|edit|trusted] [--backend isolated_linux --image sha256:DIGEST --network none|bridge --docker-socket PATH --memory-mib N --cpu-millis N --pids N]")
	}
	if len(args) < 1 || strings.HasPrefix(args[0], "--") {
		return fail()
	}
	access, backend := "edit", "local"
	p := execution.IsolationPolicy{Socket: "/var/run/docker.sock", Network: "none", MemoryBytes: 512 << 20, CPUMillis: 1000, Pids: 64}
	seen := map[string]bool{}
	for n := 1; n < len(args); n += 2 {
		if n+1 >= len(args) || seen[args[n]] {
			return fail()
		}
		seen[args[n]] = true
		value := args[n+1]
		switch args[n] {
		case "--access":
			access = value
		case "--backend":
			backend = value
		case "--image":
			p.Image = value
		case "--network":
			p.Network = value
		case "--docker-socket":
			p.Socket = value
		case "--memory-mib", "--cpu-millis", "--pids":
			v, err := strconv.ParseInt(value, 10, 32)
			if err != nil || v < 1 {
				return fail()
			}
			if args[n] == "--memory-mib" {
				p.MemoryBytes = v << 20
			} else if args[n] == "--cpu-millis" {
				p.CPUMillis = int(v)
			} else {
				p.Pids = v
			}
		default:
			return fail()
		}
	}
	if _, err := executionPermissions(access); err != nil {
		return fail()
	}
	if backend == "local" {
		for key := range seen {
			if key != "--access" && key != "--backend" {
				return fail()
			}
		}
		return args[0], access, zero, nil
	}
	// The native adapter supplies the current non-root UID/GID; neither model
	// arguments nor saved profiles can nominate a more privileged container user.
	check := p
	check.UID, check.GID = 1, 1
	if backend != "isolated_linux" || access != "trusted" || check.Validate() != nil {
		return fail()
	}
	return args[0], access, p, nil
}

func closeExecutionLease(lease ExecutionLease) error {
	if err := lease.Close(); err != nil {
		return fmt.Errorf("local executor stop is not confirmed; inspect owned work before retrying: %w", err)
	}
	return nil
}
