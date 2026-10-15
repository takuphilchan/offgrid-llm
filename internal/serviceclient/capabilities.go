package serviceclient

import (
	"context"
	"net/http"
)

// RequireCapability runs before a new-protocol mutation. Older services may
// ignore unknown fields, which must not turn an agent turn into plain Chat.
func (c *Client) RequireCapability(ctx context.Context, capability string) error {
	var identity struct {
		Product      string   `json:"product"`
		Capabilities []string `json:"capabilities"`
	}
	if err := c.JSON(ctx, http.MethodGet, "/api/v2/system", nil, &identity); err != nil {
		return err
	}
	if identity.Product == "offgrid" {
		for _, available := range identity.Capabilities {
			if available == capability {
				return nil
			}
		}
	}
	return &Error{Code: "upgrade_required", Message: "This service does not support the requested agent workflow. Update the service; no work was submitted."}
}
