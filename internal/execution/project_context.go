package execution

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
)

type ProjectContextSnapshot struct {
	Identity      string `json:"identity"`
	Name          string `json:"name"`
	Present       bool   `json:"present"`
	Configuration []byte `json:"configuration,omitempty"`
	SHA256        string `json:"sha256,omitempty"`
}

type ProjectContextEndpoint interface {
	ProjectContext(context.Context, Binding) (ProjectContextSnapshot, error)
}

// Only the private supervisor can inspect this fixed project-metadata path.
// It does not admit a job, execute hooks or reinterpret a service-side path.
func (s *Session) projectContext(ctx context.Context, binding Binding) (ProjectContextSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.authorize(ctx, binding); err != nil {
		return ProjectContextSnapshot{}, err
	}
	if binding != s.config.Binding || !s.config.Permissions.Read || s.config.Backend != "local" || !validCredential(s.config.ProjectHostID) {
		return ProjectContextSnapshot{}, ErrUnsupported
	}
	identity, err := folderIdentity(s.config.Root)
	if err != nil {
		return ProjectContextSnapshot{}, err
	}
	key, _ := json.Marshal([]string{"project-folder-v1", s.config.ProjectHostID, identity})
	result := ProjectContextSnapshot{Identity: digest(key), Name: filepath.Base(s.config.Root)}
	const name = ".offgrid/project.toml"
	info, err := s.root.Lstat(name)
	if os.IsNotExist(err) {
		return result, s.authorize(ctx, binding)
	}
	if err != nil {
		return ProjectContextSnapshot{}, err
	}
	if !info.Mode().IsRegular() || info.Size() > 64<<10 {
		return ProjectContextSnapshot{}, ErrLimit
	}
	f, err := openWorkingCopyFile(s.root, name, info)
	if err != nil {
		return ProjectContextSnapshot{}, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(copyContextReader{ctx, f}, 64<<10+1))
	if err != nil {
		return ProjectContextSnapshot{}, err
	}
	after, err := f.Stat()
	if err != nil || int64(len(data)) != info.Size() || !after.ModTime().Equal(info.ModTime()) || after.Size() != info.Size() {
		return ProjectContextSnapshot{}, ErrRevision
	}
	if err = s.authorize(ctx, binding); err != nil {
		return ProjectContextSnapshot{}, err
	}
	result.Present, result.Configuration, result.SHA256 = true, data, digest(data)
	return result, nil
}

func (e *LocalEndpoint) ProjectContext(ctx context.Context, binding Binding) (ProjectContextSnapshot, error) {
	e.mu.Lock()
	closed := e.closed
	e.mu.Unlock()
	if closed {
		return ProjectContextSnapshot{}, ErrHostUnavailable
	}
	return e.Worker.Session.projectContext(ctx, binding)
}

func (e *StdioEndpoint) ProjectContext(ctx context.Context, binding Binding) (ProjectContextSnapshot, error) {
	reply, err := e.control(ctx, binding, "supervisor_project_context")
	if err != nil {
		return ProjectContextSnapshot{}, err
	}
	var result ProjectContextSnapshot
	err = decodeStrict(reply.Receipt.Result, &result)
	return result, err
}

func (p *Peer) ProjectContext(ctx context.Context, binding Binding) (ProjectContextSnapshot, error) {
	id, err := opaqueToken()
	if err != nil {
		return ProjectContextSnapshot{}, err
	}
	r := Request{Version: ProtocolVersion, ID: id, OperationID: id, Binding: binding, Operation: "supervisor_project_context", Arguments: json.RawMessage(`{}`)}
	reply, err := p.exchange(ctx, "project", binding, &r)
	if err != nil {
		return ProjectContextSnapshot{}, err
	}
	var result ProjectContextSnapshot
	if reply.Response == nil || reply.Response.Receipt == nil || reply.Response.Receipt.State != "complete" {
		return result, ErrProtocol
	}
	err = decodeStrict(reply.Response.Receipt.Result, &result)
	return result, err
}

func (h *Hosts) ProjectContext(ctx context.Context, actor, workspace string, ref Reference) (ProjectContextSnapshot, error) {
	entry, err := h.lookup(actor, workspace, ref)
	if err != nil {
		return ProjectContextSnapshot{}, err
	}
	if entry.parent != nil || !entry.config.Permissions.Read || !validCredential(entry.config.ProjectHostID) {
		return ProjectContextSnapshot{}, ErrUnsupported
	}
	if _, err = h.Resolve(ctx, actor, workspace, ref); err != nil {
		return ProjectContextSnapshot{}, err
	}
	endpoint, ok := entry.endpoint.(ProjectContextEndpoint)
	if !ok {
		return ProjectContextSnapshot{}, ErrUnsupported
	}
	result, err := endpoint.ProjectContext(ctx, entry.config.Binding)
	if err != nil {
		return ProjectContextSnapshot{}, err
	}
	if !validCopyID(result.Identity) || !validIdentity(result.Name) || len(result.Configuration) > 64<<10 || result.Present && result.SHA256 != digest(result.Configuration) || !result.Present && (len(result.Configuration) != 0 || result.SHA256 != "") {
		return ProjectContextSnapshot{}, ErrProtocol
	}
	if _, err = h.Resolve(ctx, actor, workspace, ref); err != nil {
		return ProjectContextSnapshot{}, err
	}
	return result, nil
}
