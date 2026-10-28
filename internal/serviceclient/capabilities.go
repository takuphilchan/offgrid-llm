package serviceclient

import (
	"context"
	"net/http"
)

type Identity struct {
	Product      string   `json:"product"`
	Version      string   `json:"version"`
	WorkspaceID  string   `json:"workspace_id"`
	Capabilities []string `json:"capabilities"`
}

func (c *Client) Identity(ctx context.Context) (Identity, error) {
	var identity Identity
	err := c.JSON(ctx, http.MethodGet, "/api/v2/system", nil, &identity)
	return identity, err
}
func (i Identity) Supports(capability string) bool {
	if i.Product != "offgrid" {
		return false
	}
	for _, available := range i.Capabilities {
		if available == capability {
			return true
		}
	}
	return false
}

// RequireCapability runs before a new-protocol mutation. Older services may
// ignore unknown fields, which must not turn an agent turn into plain Chat.
func (c *Client) RequireCapability(ctx context.Context, capability string) error {
	identity, err := c.Identity(ctx)
	if err != nil {
		return err
	}
	if identity.Supports(capability) {
		return nil
	}
	return &Error{Code: "upgrade_required", Message: "This service does not support the requested agent workflow. Update the service; no work was submitted."}
}
