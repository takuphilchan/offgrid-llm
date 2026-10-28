package execution

import (
	"context"
	"errors"
	"io"
	"net/url"
	"path/filepath"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

// NativeHostRequest is a bounded one-shot PRIVATE native-supervisor message,
// not a renderer method, HTTP API or model tool. Only native main may supply
// Folder/Permissions after its own consent. There is no arbitrary spawn field.
type NativeHostRequest struct {
	Protocol    int         `json:"protocol"`
	Service     string      `json:"service"`
	Pairing     Pairing     `json:"pairing"`
	Folder      string      `json:"folder"`
	Permissions Permissions `json:"permissions"`
}

type NativeHostState struct {
	Protocol int       `json:"protocol"`
	State    string    `json:"state"`
	Resource *Resource `json:"resource,omitempty"`
}

func (r NativeHostRequest) Validate() error {
	u, err := url.Parse(r.Service)
	if err != nil || u.Scheme != "http" || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || (u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost" && u.Hostname() != "::1") || r.Protocol != ProtocolVersion || !filepath.IsAbs(r.Folder) || !r.Permissions.Read || r.Permissions.Processes && !r.Permissions.Write || r.Pairing.Protocol != ProtocolVersion || !validCredential(r.Pairing.Code) || !validIdentity(r.Pairing.Actor) || !validIdentity(r.Pairing.Workspace) {
		return ErrAuthority
	}
	return nil
}

// ServeNativeHost owns only the approved host lease. EOF/any extra input closes
// it; there is no command channel and no saved-grant restoration on restart.
func ServeNativeHost(ctx context.Context, input io.ReadCloser, output io.Writer, packDirectory, stateDirectory, version string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stopInput := context.AfterFunc(ctx, func() { input.Close() })
	defer stopInput()
	var request NativeHostRequest
	seed := make(chan error, 1)
	go func() { seed <- ReadFrame(input, &request) }()
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	select {
	case err := <-seed:
		if err != nil {
			return err
		}
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		cancel()
		return ErrHostUnavailable
	}
	if err := request.Validate(); err != nil {
		return err
	}
	go func() { var extra [1]byte; _, _ = input.Read(extra[:]); cancel() }()
	client, err := serviceclient.New(request.Service, "", nil)
	if err != nil {
		return err
	}
	host, err := StartPairedHost(ctx, client, LocalHostOptions{Folder: request.Folder, Permissions: request.Permissions, PackDirectory: packDirectory, StateDirectory: stateDirectory, Version: version}, request.Pairing)
	if err != nil {
		return err
	}
	resource := host.Resource()
	if err = WriteFrame(output, NativeHostState{Protocol: ProtocolVersion, State: "ready", Resource: &resource}); err != nil {
		return errors.Join(err, host.Close())
	}
	select {
	case <-ctx.Done():
	case <-host.Done():
	}
	if err = host.Close(); err != nil {
		return err
	}
	return WriteFrame(output, NativeHostState{Protocol: ProtocolVersion, State: "stopped"})
}
