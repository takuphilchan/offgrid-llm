package server

import (
	"context"
	"encoding/json"
	"errors"
	"runtime"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
)

// Recognize foreign desktop paths without opening them, resolving a network
// share or translating them into WSL/container mounts. This is a consent hint,
// never a filesystem grant or permission to execute the original operation.
func foreignFileApplication(path, serviceOS string) string {
	p := strings.TrimSpace(path)
	lower := strings.ToLower(p)
	windows := len(p) >= 2 && ((p[0] >= 'a' && p[0] <= 'z') || (p[0] >= 'A' && p[0] <= 'Z')) && p[1] == ':'
	if serviceOS != "windows" && (windows || strings.HasPrefix(p, `\\`) || strings.HasPrefix(lower, "file:") || strings.Contains(p, `\`)) {
		return "File Explorer"
	}
	if serviceOS != "darwin" && (strings.HasPrefix(p, "/Users/") || strings.HasPrefix(p, "/Volumes/")) {
		return "Finder"
	}
	if serviceOS == "windows" && strings.HasPrefix(p, "/home/") {
		return "Files"
	}
	return ""
}

func (b *browserRunTools) hostFileAccess(ctx context.Context, task *agents.Task, name string, args json.RawMessage, execution agents.ToolExecution) error {
	if name != "read_file" && name != "list_files" && name != "write_file" {
		return nil
	}
	d, ok := b.RunTools.Capability(name)
	if !ok || d.Source != "builtin" || d.Namespace != "tools" {
		return nil
	}
	var request struct {
		Path string `json:"path"`
	}
	if json.Unmarshal(args, &request) != nil {
		return nil
	}
	target := foreignFileApplication(request.Path, runtime.GOOS)
	if target == "" {
		return nil
	}
	// A disabled/replaced/forbidden tool cannot acquire authority by being routed
	// to the companion. An approval-required write may request access only; it
	// does not consume or transfer that write grant to the selected application.
	if err := b.RunTools.Authorize(ctx, name, args, execution); err != nil && !errors.Is(err, capabilities.ErrApprovalRequired) {
		return err
	}
	if task.ParentID != "" {
		return &agents.ToolAuthorizationError{Code: "host_access_required", Message: "This file is on another execution device. A read-only subtask cannot obtain local computer access; the parent task must request it."}
	}
	return &agents.InputRequired{Request: agents.InputRequest{Kind: "computer", Mode: "app", Target: target}}
}
