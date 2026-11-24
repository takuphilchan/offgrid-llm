package server

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"slices"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
)

type endpointCredentialReference struct {
	ID          string   `json:"id"`
	Environment string   `json:"environment"`
	Origins     []string `json:"origins"`
}

type endpointManagedTemplate struct {
	ID    string                      `json:"id"`
	Entry inferenceEndpointEnrollment `json:"entry"`
}

// This file is selected only by installation configuration and is never
// writable through the endpoint administration API. Remote credentials are
// constrained to exact origins, not merely to a list of environment names.
type endpointOwnerHost struct {
	Version          int                           `json:"version"`
	EditableFile     string                        `json:"editable_file,omitempty"`
	Credentials      []endpointCredentialReference `json:"credentials"`
	ManagedTemplates []endpointManagedTemplate     `json:"managed_templates"`
}

func readEndpointOwnerHost(path string) (endpointOwnerHost, string, error) {
	host := endpointOwnerHost{Version: 1}
	if path == "" {
		return host, endpointOwnerDigest([]byte("no-host-endpoint-policy")), nil
	}
	raw, err := readEndpointOwnerJSON(path, 64<<10, &host)
	if err != nil {
		return host, "", errors.New("endpoint owner installation policy is unavailable")
	}
	if host.Version != 1 || len(host.Credentials) > 32 || len(host.ManagedTemplates) > 32 || host.EditableFile != "" && !filepath.IsAbs(host.EditableFile) {
		return host, "", errors.New("endpoint owner installation policy is invalid")
	}
	seen := map[string]bool{}
	for _, credential := range host.Credentials {
		if !inferenceEndpointName.MatchString(credential.ID) || !inferenceCredentialName.MatchString(credential.Environment) || seen[credential.ID] || len(credential.Origins) == 0 || len(credential.Origins) > 16 {
			return host, "", errors.New("endpoint credential reference is invalid")
		}
		for _, origin := range credential.Origins {
			u, err := inference.ParseHTTPEndpointOrigin(origin)
			if err != nil || u.Scheme != "https" {
				return host, "", errors.New("endpoint credential origin is invalid")
			}
		}
		seen[credential.ID] = true
	}
	seen = map[string]bool{}
	for _, template := range host.ManagedTemplates {
		if !inferenceEndpointName.MatchString(template.ID) || seen[template.ID] || template.Entry.ManagedOllama == nil {
			return host, "", errors.New("managed endpoint template is invalid")
		}
		if err := validateEndpointOwnerRoute(template.Entry); err != nil {
			return host, "", errors.New("managed endpoint template is unavailable")
		}
		seen[template.ID] = true
	}
	return host, endpointOwnerDigest(raw), nil
}

type endpointOwnerInput struct {
	Expected           string                `json:"expected_revision"`
	Replace            string                `json:"replace,omitempty"`
	ID                 string                `json:"id"`
	Revision           string                `json:"revision"`
	Provider           inference.ProviderID  `json:"provider,omitempty"`
	URL                string                `json:"url,omitempty"`
	AllowedIPs         []string              `json:"allowed_ips,omitempty"`
	Actors             []string              `json:"actors"`
	Credential         string                `json:"credential_reference,omitempty"`
	ManagedTemplate    string                `json:"managed_template,omitempty"`
	Model              string                `json:"model,omitempty"`
	ModelDigest        string                `json:"model_digest,omitempty"`
	Format             inference.ModelFormat `json:"format,omitempty"`
	RuntimeVersion     string                `json:"runtime_version,omitempty"`
	ContextLength      int                   `json:"context_length,omitempty"`
	DeploymentAttested bool                  `json:"deployment_attested"`
}

func (input endpointOwnerInput) entry(host endpointOwnerHost) (inferenceEndpointEnrollment, error) {
	entry := inferenceEndpointEnrollment{ID: input.ID, Revision: input.Revision, Provider: input.Provider, URL: input.URL, AllowedIPs: slices.Clone(input.AllowedIPs), Actors: slices.Clone(input.Actors), Model: input.Model, ModelDigest: input.ModelDigest, Format: input.Format, RuntimeVersion: input.RuntimeVersion, ContextLength: input.ContextLength, DeploymentAttested: input.DeploymentAttested}
	if input.ManagedTemplate != "" {
		if input.Credential != "" || input.URL != "" || input.Provider != "" || len(input.AllowedIPs) > 0 || input.Model != "" || input.ModelDigest != "" || input.Format != "" || input.RuntimeVersion != "" || input.ContextLength != 0 {
			return entry, errors.New("managed endpoint fields come only from its installation template")
		}
		index := slices.IndexFunc(host.ManagedTemplates, func(value endpointManagedTemplate) bool { return value.ID == input.ManagedTemplate })
		if index < 0 {
			return entry, errors.New("managed endpoint template is unavailable")
		}
		template := host.ManagedTemplates[index].Entry
		for _, actor := range input.Actors {
			if !slices.Contains(template.Actors, actor) {
				return entry, errors.New("managed endpoint actor exceeds template authority")
			}
		}
		// JSON detaches nested managed inventories from caller-owned slices/maps.
		raw, _ := json.Marshal(template)
		if err := json.Unmarshal(raw, &entry); err != nil {
			return entry, err
		}
		entry.ID, entry.Revision, entry.Actors = input.ID, input.Revision, slices.Clone(input.Actors)
		entry.DeploymentAttested = input.DeploymentAttested
	} else if input.Credential != "" {
		index := slices.IndexFunc(host.Credentials, func(value endpointCredentialReference) bool { return value.ID == input.Credential })
		if index < 0 || !slices.Contains(host.Credentials[index].Origins, input.URL) {
			return entry, errors.New("credential reference is not approved for this exact origin")
		}
		entry.CredentialEnv = host.Credentials[index].Environment
	}
	if err := validateEndpointOwnerRoute(entry); err != nil {
		return entry, err
	}
	return entry, nil
}

func validateEndpointOwnerRoute(entry inferenceEndpointEnrollment) error {
	if !inferenceEndpointName.MatchString(entry.ID) || !inferenceEndpointName.MatchString(entry.Revision) || len(entry.Actors) == 0 || len(entry.Actors) > 128 || !entry.DeploymentAttested || strings.TrimSpace(entry.Model) == "" || len(entry.Model) > 512 || strings.TrimSpace(entry.RuntimeVersion) == "" || len(entry.RuntimeVersion) > 256 || entry.ContextLength < 512 || entry.ContextLength > 1<<20 {
		return errors.New("endpoint owner entry is incomplete")
	}
	if entry.Provider != inference.ProviderOllama && entry.Provider != inference.ProviderVLLM || entry.ManagedOllama != nil && (entry.Provider != inference.ProviderOllama || entry.CredentialEnv != "" || len(entry.Actors) != 1) {
		return errors.New("endpoint provider or managed authority is invalid")
	}
	seen := map[string]bool{}
	for _, actor := range entry.Actors {
		if actor == "" || actor == "*" || len(actor) > 256 || seen[actor] {
			return errors.New("endpoint actor scope is invalid")
		}
		seen[actor] = true
	}
	_, err := inference.NewHTTPEndpointRoutes(map[string]inference.HTTPEndpoint{endpointKey(entry): {URL: entry.URL, Model: entry.Model, RuntimeVersion: entry.RuntimeVersion, AllowedIPs: entry.AllowedIPs, Remote: entry.ManagedOllama == nil, ManagedOllama: entry.ManagedOllama, Descriptor: endpointDescriptor(entry), Authorize: func(context.Context) (string, error) { return "", errors.New("validation grants no authority") }}})
	if err != nil {
		return errors.New("endpoint destination, pins or runtime contract is invalid")
	}
	return nil
}
