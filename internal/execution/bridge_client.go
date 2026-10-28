package execution

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

type HostConnection struct {
	Reference Reference
	client    *serviceclient.Client
	endpoint  HostEndpoint
	approved  Context
	token     string
	ctx       context.Context
	cancel    context.CancelFunc
	ready     chan struct{}
	done      chan struct{}
	mu        sync.Mutex
	err       error
	stopErr   error
}

// ConnectApproved is called only after native approval and pack verification.
// The selected endpoint already owns the approved OS objects. Nothing from the
// service constructs a local grant or changes the approved root/permissions.
func ConnectApproved(ctx context.Context, client *serviceclient.Client, endpoint HostEndpoint, approved Context, pack Pack, code, token string) (*HostConnection, error) {
	if endpoint == nil || !validCredential(token) || !validCredential(code) || pack.Validate(pack.Version) != nil || !validPublishedContext(approved, pack) {
		return nil, ErrAuthority
	}
	actual, err := endpoint.Check(ctx, approved.Binding)
	if err != nil || actual != approved {
		return nil, ErrAuthority
	}
	bridge := client.HostBridge()
	var response struct {
		Reference Reference `json:"reference"`
		State     string    `json:"state"`
	}
	input := struct {
		Code    string  `json:"code"`
		Token   string  `json:"token"`
		Context Context `json:"context"`
		Pack    Pack    `json:"pack"`
	}{code, token, approved, pack}
	if err = bridge.StrictJSON(ctx, http.MethodPost, "/api/v2/execution/host/enroll", input, &response, 4096); err != nil {
		return nil, err
	}
	if response.Reference != reference(approved.Binding) || response.State != "awaiting_host_check" {
		return nil, ErrProtocol
	}
	workctx, cancel := context.WithCancel(ctx)
	c := &HostConnection{Reference: response.Reference, client: bridge, endpoint: endpoint, approved: approved, token: token, ctx: workctx, cancel: cancel, ready: make(chan struct{}), done: make(chan struct{})}
	go c.run()
	startup := time.NewTimer(10 * time.Second)
	defer startup.Stop()
	select {
	case <-c.ready:
		return c, nil
	case <-c.done:
		return nil, c.Err()
	case <-ctx.Done():
		c.Close()
		return nil, ctx.Err()
	case <-startup.C:
		c.Close()
		return nil, ErrHostUnavailable
	}
}

func (c *HostConnection) body(reply *BridgeReply) any {
	return struct {
		Session string       `json:"session"`
		Token   string       `json:"token"`
		Reply   *BridgeReply `json:"reply,omitempty"`
	}{c.Reference.Session, c.token, reply}
}

func (c *HostConnection) poll() error {
	ready := false
	for {
		var message BridgeMessage
		if err := c.client.StrictJSON(c.ctx, http.MethodPost, "/api/v2/execution/host/poll", c.body(nil), &message, MaxFrameBytes); err != nil {
			return err
		}
		if message.Version != ProtocolVersion {
			return ErrProtocol
		}
		if message.Kind == "heartbeat" {
			if message.ID != "" || message.Request != nil {
				return ErrProtocol
			}
			continue
		}
		operationctx, cancel := context.WithTimeout(c.ctx, 15*time.Second)
		reply := DispatchBridgeMessage(operationctx, c.endpoint, c.approved, message)
		cancel()
		var acknowledgment struct {
			Ready bool `json:"ready"`
		}
		if err := c.client.StrictJSON(c.ctx, http.MethodPost, "/api/v2/execution/host/reply", c.body(&reply), &acknowledgment, 4096); err != nil {
			return err
		}
		if !ready && acknowledgment.Ready {
			close(c.ready)
			ready = true
		}
	}
}

func (c *HostConnection) heartbeat() error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-c.ctx.Done():
			return c.ctx.Err()
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(c.ctx, 10*time.Second)
			current, err := c.endpoint.Check(ctx, c.approved.Binding)
			if err == nil && (current.Root != c.approved.Root || current.Permissions != c.approved.Permissions || current.Binding != c.approved.Binding || current.Backend != c.approved.Backend || current.Isolation != c.approved.Isolation) {
				err = ErrAuthority
			}
			var result struct {
				Success bool `json:"success"`
			}
			if err == nil {
				err = c.client.StrictJSON(ctx, http.MethodPost, "/api/v2/execution/host/heartbeat", c.body(nil), &result, 4096)
				if err == nil && !result.Success {
					err = ErrProtocol
				}
			}
			cancel()
			if err != nil {
				return err
			}
		}
	}
}

func (c *HostConnection) run() {
	defer close(c.done)
	results := make(chan error, 2)
	go func() { results <- c.poll() }()
	go func() { results <- c.heartbeat() }()
	err := <-results
	c.cancel()
	stopErr := c.endpoint.Close()
	<-results
	// Revocation is cleanup, never replay. It uses only this host's narrow
	// credential and a fresh bounded context, even after Ctrl+C.
	cleanup, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	var revoked struct {
		Success bool `json:"success"`
	}
	_ = c.client.StrictJSON(cleanup, http.MethodPost, "/api/v2/execution/host/revoke", c.body(nil), &revoked, 4096)
	cancel()
	c.mu.Lock()
	c.err = errors.Join(err, stopErr)
	c.stopErr = stopErr
	c.mu.Unlock()
}

func (c *HostConnection) Err() error            { c.mu.Lock(); defer c.mu.Unlock(); return c.err }
func (c *HostConnection) Done() <-chan struct{} { return c.done }
func (c *HostConnection) Close() error {
	c.cancel()
	select {
	case <-c.done:
		c.mu.Lock()
		defer c.mu.Unlock()
		return c.stopErr
	case <-time.After(3 * time.Second):
		return ErrHostUnavailable
	}
}
