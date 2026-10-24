package integrations

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// ServiceClient is shared by managed and provider-only CLI paths. Credentials
// stay on the selected service origin; serviceclient rejects redirects.
type ServiceClient struct {
	client      *serviceclient.Client
	preparation *serviceclient.Client
}

func NewServiceClient(origin, key string, httpClient *http.Client) (*ServiceClient, error) {
	c, err := serviceclient.New(origin, key, httpClient)
	if err != nil {
		return nil, err
	}
	long := &http.Client{}
	if httpClient != nil {
		*long = *httpClient
	}
	long.Timeout = 5 * time.Minute
	preparation, err := serviceclient.New(origin, key, long)
	if err != nil {
		return nil, err
	}
	return &ServiceClient{client: c, preparation: preparation}, nil
}

// Prepare loads only the explicitly selected model, through normal authenticated
// inference admission. It is not a smoke test or a tool qualification.
func (c *ServiceClient) Prepare(ctx context.Context, model string) error {
	var result struct {
		Model    string `json:"model"`
		Prepared bool   `json:"prepared"`
	}
	err := c.preparation.JSON(ctx, http.MethodPost, "/v1/inference/prepare", map[string]string{"model": model}, &result)
	if err != nil {
		return err
	}
	if !result.Prepared || result.Model != model {
		return fmt.Errorf("service did not confirm preparation of the selected model")
	}
	return nil
}

func (c *ServiceClient) Setup(ctx context.Context, id, model, baseURL string) (SetupResponse, error) {
	var result SetupResponse
	q := url.Values{"base_url": {baseURL}, "model": {model}}
	err := c.client.JSON(ctx, http.MethodGet, "/v1/integrations/"+url.PathEscape(id)+"/setup?"+q.Encode(), nil, &result)
	return result, err
}

func (c *ServiceClient) List(ctx context.Context, model string) (ListResponse, error) {
	var result ListResponse
	err := c.client.JSON(ctx, http.MethodGet, "/v1/integrations?"+(url.Values{"model": {model}}).Encode(), nil, &result)
	return result, err
}

func (c *ServiceClient) Models(ctx context.Context) ([]api.Model, error) {
	var result api.ModelListResponse
	err := c.client.JSON(ctx, http.MethodGet, "/v1/models", nil, &result)
	return result.Data, err
}
