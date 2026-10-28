package execution

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

type LocalHostOptions struct {
	Folder         string
	PackDirectory  string
	StateDirectory string
	Version        string
	Permissions    Permissions
	Isolation      IsolationPolicy
}

type LocalApproval struct {
	Actor       string
	Service     string
	Root        string
	Host        string
	Workspace   string
	Backend     string
	Permissions Permissions
	Isolation   IsolationPolicy
}

type OwnedHost struct {
	Connection     *HostConnection
	Context        Context
	StateDirectory string
}

func (h *OwnedHost) Close() error { return h.Connection.Close() }
func (h *OwnedHost) Resource() Resource {
	return Resource{reference(h.Context.Binding), h.Context.Backend, h.Context.Root, h.Context.Directory, h.Context.Permissions}
}
func (h *OwnedHost) Done() <-chan struct{} { return h.Connection.Done() }
func (h *OwnedHost) Err() error            { return h.Connection.Err() }

// StartLocalHost is a native-only authority constructor. The caller supplies
// trusted asset/state locations and a local approval callback. Folder names,
// stored profiles and service messages alone cannot create approval.
func StartLocalHost(ctx context.Context, client *serviceclient.Client, options LocalHostOptions, approve func(LocalApproval) bool) (*OwnedHost, error) {
	return startLocalHost(ctx, client, options, approve, nil, func(ctx context.Context) (Pairing, error) {
		var pair Pairing
		err := client.StrictJSON(ctx, http.MethodPost, "/api/v2/execution/pairing", struct{}{}, &pair, 4096)
		return pair, err
	})
}

// StartPairedHost is only for the trusted native application supervisor after
// its local folder/authority dialog. The pairing ticket pins the signed-in
// actor without giving cookies or user API credentials to the helper. Never
// expose this constructor through an HTTP route or renderer-shaped context.
func StartPairedHost(ctx context.Context, client *serviceclient.Client, options LocalHostOptions, pair Pairing) (*OwnedHost, error) {
	return startLocalHost(ctx, client, options, func(LocalApproval) bool { return true }, &pair, func(context.Context) (Pairing, error) { return pair, nil })
}

func startLocalHost(ctx context.Context, client *serviceclient.Client, options LocalHostOptions, approve func(LocalApproval) bool, paired *Pairing, pairing func(context.Context) (Pairing, error)) (*OwnedHost, error) {
	if approve == nil || !filepath.IsAbs(options.Folder) || !filepath.IsAbs(options.PackDirectory) || !filepath.IsAbs(options.StateDirectory) {
		return nil, ErrAuthority
	}
	root, err := filepath.EvalSymlinks(options.Folder)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil, ErrPath
	}
	if err = PrivateStateOutsideRoot(root, options.PackDirectory, options.StateDirectory); err != nil {
		return nil, err
	}
	pack, _, err := LoadPack(options.PackDirectory, options.Version)
	if err != nil {
		return nil, fmt.Errorf("matching execution pack unavailable: %w", err)
	}
	var identity serviceclient.Identity
	if paired == nil {
		identity, err = client.Identity(ctx)
		if err != nil {
			return nil, err
		}
		if !identity.Supports("native-execution-preview-v1") {
			return nil, &serviceclient.Error{Code: "upgrade_required", Message: "Enable native execution only on a matching service with an approved local host. No folder grant was created."}
		}
	} else {
		// Native main already performed its authenticated identity check. The
		// helper receives only its short-lived ticket, never user credentials.
		// Enrollment independently verifies product-specific protocol/version,
		// actor/workspace, preview readiness and ticket freshness at the service.
		identity.Version, identity.WorkspaceID = paired.Version, paired.Workspace
	}
	if !validIdentity(identity.WorkspaceID) {
		return nil, ErrProtocol
	}
	if pack.Validate(identity.Version) != nil {
		return nil, ErrProtocol
	}
	host, err := os.Hostname()
	if err != nil || !validIdentity(host) {
		return nil, ErrAuthority
	}
	backend := "local"
	if options.Isolation != (IsolationPolicy{}) {
		if options.Isolation.Validate() != nil {
			return nil, ErrAuthority
		}
		backend = "isolated_linux"
	}
	actor := ""
	if paired != nil {
		actor = paired.Actor
	} else {
		var account struct {
			AuthRequired  *bool `json:"auth_required"`
			Authenticated bool  `json:"authenticated"`
			User          *struct {
				ID string `json:"id"`
			} `json:"user"`
		}
		// Actor identity is needed before offering remembered trust. An older
		// service without this route may still use session-only approval.
		if accountErr := client.JSON(ctx, http.MethodGet, "/v1/users/me", nil, &account); accountErr == nil && account.AuthRequired != nil {
			if !*account.AuthRequired {
				actor = "local-admin"
			} else if account.Authenticated && account.User != nil {
				actor = account.User.ID
			}
		}
	}
	if !approve(LocalApproval{Actor: actor, Service: client.Origin(), Root: root, Host: host, Workspace: identity.WorkspaceID, Backend: backend, Permissions: options.Permissions, Isolation: options.Isolation}) {
		return nil, ErrAuthority
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	current, statErr := os.Stat(root)
	if statErr != nil || !os.SameFile(info, current) {
		return nil, ErrPath
	}
	// Request the short-lived ticket only AFTER the local person has finished
	// reviewing. Waiting at a consent prompt must not silently expire setup.
	pair, err := pairing(ctx)
	if err != nil {
		return nil, err
	}
	if pair.Protocol != ProtocolVersion || !validCredential(pair.Code) || !validIdentity(pair.Actor) || actor != "" && pair.Actor != actor || pair.Workspace != identity.WorkspaceID || pack.Validate(pair.Version) != nil {
		return nil, ErrProtocol
	}
	nonce, err := opaqueToken()
	if err != nil {
		return nil, err
	}
	hostToken, err := opaqueToken()
	if err != nil {
		return nil, err
	}
	config := Context{Version: ProtocolVersion, Binding: Binding{Actor: pair.Actor, Workspace: pair.Workspace, Host: host, Session: "executor-" + nonce[:32], Grant: "grant-" + nonce[32:], Revision: 1, Job: BootstrapJob}, Backend: backend, Root: root, Directory: ".", Permissions: options.Permissions, Isolation: options.Isolation}
	directory := filepath.Join(options.StateDirectory, config.Binding.Session)
	if err = os.MkdirAll(options.StateDirectory, 0700); err != nil {
		return nil, err
	}
	if err = ProtectPrivatePath(options.StateDirectory); err != nil {
		return nil, err
	}
	if err = os.Mkdir(directory, 0700); err != nil {
		return nil, err
	}
	if err = ProtectPrivatePath(directory); err != nil {
		return nil, err
	}
	data, _ := json.Marshal(config)
	grant := filepath.Join(directory, "grant.json")
	file, err := os.OpenFile(grant, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	_, writeErr := file.Write(data)
	syncErr := file.Sync()
	closeErr := file.Close()
	if writeErr != nil {
		return nil, writeErr
	}
	if syncErr != nil {
		return nil, syncErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if err = ProtectPrivatePath(grant); err != nil {
		return nil, err
	}
	endpoint, actualPack, err := StartManaged(ctx, options.PackDirectory, options.Version, grant, filepath.Join(directory, "journal"))
	if err != nil {
		return nil, err
	}
	connection, err := ConnectApproved(ctx, client, endpoint, config, actualPack, pair.Code, hostToken)
	if err != nil {
		endpoint.Close()
		return nil, err
	}
	return &OwnedHost{Connection: connection, Context: config, StateDirectory: directory}, nil
}
