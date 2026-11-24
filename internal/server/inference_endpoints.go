package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"slices"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/users"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type inferenceEndpointEnrollment struct {
	ID             string                `json:"id"`
	Revision       string                `json:"revision"`
	Provider       inference.ProviderID  `json:"provider"`
	URL            string                `json:"url"`
	AllowedIPs     []string              `json:"allowed_ips"`
	Actors         []string              `json:"actors"`
	CredentialEnv  string                `json:"credential_env,omitempty"`
	Model          string                `json:"model"`
	ModelDigest    string                `json:"model_digest"`
	Format         inference.ModelFormat `json:"format"`
	RuntimeVersion string                `json:"runtime_version"`
	ContextLength  int                   `json:"context_length"`
	// This is an operator attestation over their deployment, not evidence that
	// /v1/models cryptographically verifies vLLM's weight bytes.
	DeploymentAttested bool                           `json:"deployment_attested"`
	ManagedOllama      *inference.ManagedOllamaConfig `json:"managed_ollama,omitempty"`
}

type inferenceEndpointFile struct {
	Version   int                           `json:"version"`
	Endpoints []inferenceEndpointEnrollment `json:"endpoints"`
}

var inferenceEndpointName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,127}$`)
var inferenceCredentialName = regexp.MustCompile(`^[A-Z_][A-Z0-9_]{0,127}$`)

type endpointActorKey struct{}

// Background jobs retain a persisted actor ID, never a mutable User pointer or
// a synthetic HTTP credential. Only server-owned task/schedule paths set this.
func withEndpointActor(ctx context.Context, actor string) context.Context {
	return context.WithValue(ctx, endpointActorKey{}, actor)
}

func (s *Server) currentEndpointActor(ctx context.Context) (string, error) {
	actor, durable := ctx.Value(endpointActorKey{}).(string)
	user := users.UserFromContext(ctx)
	if durable {
		if actor == "" || user != nil && user.ID != actor {
			return "", errors.New("HTTP endpoint task actor is unavailable")
		}
	} else if s.config.RequireAuth {
		if user == nil {
			return "", errors.New("HTTP endpoint actor is unavailable")
		}
		actor = user.ID
	} else {
		actor = "local-admin"
	}
	// Read current permissions under the store lock. A stale request identity
	// cannot preserve authority after deletion, disablement or a role change.
	if !s.scheduleActorPermission(actor, users.PermissionChat) {
		return "", errors.New("HTTP endpoint actor authority is unavailable")
	}
	return actor, nil
}

func isEndpointModel(model string) bool { return strings.HasPrefix(model, "endpoint:") }

// Public endpoint identities are a separate namespace. A historical local
// registry record with the same bytes must never redirect their inference.
// The local record and its files remain intact.
func (s *Server) localChatMetadata(model string) (*api.ModelMetadata, error) {
	if isEndpointModel(model) {
		return nil, errors.New("explicit HTTP endpoint cannot use the local model runtime")
	}
	return s.registry.GetChatModel(model)
}

func (s *Server) listedLocalModels() []api.Model {
	installed := s.registry.ListModels()
	visible := installed[:0]
	for _, model := range installed {
		if !isEndpointModel(model.ID) {
			visible = append(visible, model)
		}
	}
	return visible
}

func readInferenceEndpoints(path string) (inferenceEndpointFile, error) {
	var config inferenceEndpointFile
	if _, err := readEndpointOwnerJSON(path, 64<<10, &config); err != nil {
		return config, errors.New("inference endpoint configuration is invalid")
	}
	return config, validateInferenceEndpointFile(config)
}

func validateInferenceEndpointFile(config inferenceEndpointFile) error {
	if config.Version != 1 || len(config.Endpoints) > 64 {
		return errors.New("inference endpoint configuration version or entries are invalid")
	}
	seen := map[string]bool{}
	for _, endpoint := range config.Endpoints {
		if _, err := inference.ParseHTTPEndpointOrigin(endpoint.URL); err != nil {
			return errors.New("inference endpoint origin is invalid")
		}
		key := endpoint.ID + "@" + endpoint.Revision
		if !inferenceEndpointName.MatchString(endpoint.ID) || !inferenceEndpointName.MatchString(endpoint.Revision) || seen[key] || len(endpoint.Actors) == 0 || len(endpoint.Actors) > 128 || strings.TrimSpace(endpoint.Model) == "" || len(endpoint.Model) > 512 || strings.TrimSpace(endpoint.RuntimeVersion) == "" || len(endpoint.RuntimeVersion) > 256 || endpoint.ContextLength < 512 || endpoint.ContextLength > 1<<20 || !endpoint.DeploymentAttested || endpoint.CredentialEnv != "" && !inferenceCredentialName.MatchString(endpoint.CredentialEnv) {
			return errors.New("inference endpoint enrollment is incomplete or ambiguous")
		}
		if endpoint.Provider != inference.ProviderOllama && endpoint.Provider != inference.ProviderVLLM {
			return errors.New("inference endpoint provider is unsupported")
		}
		if endpoint.ManagedOllama != nil && (endpoint.Provider != inference.ProviderOllama || endpoint.CredentialEnv != "" || len(endpoint.Actors) != 1) {
			return errors.New("managed Ollama requires one explicitly authorized actor and no upstream credential")
		}
		seen[key] = true
		for _, actor := range endpoint.Actors {
			if actor == "" || actor == "*" {
				return errors.New("inference endpoint actor scope must be explicit")
			}
		}
	}
	return nil
}

func endpointDescriptor(entry inferenceEndpointEnrollment) inference.ProviderDescriptor {
	identity, _ := json.Marshal(entry)
	digest := sha256.Sum256(identity)
	backend := "remote"
	if entry.ManagedOllama != nil {
		backend = "local-ollama"
	}
	return inference.ProviderDescriptor{ProtocolRevision: inference.ProviderProtocolRevision, Provider: entry.Provider, RunnerRevision: entry.RuntimeVersion + "/enrollment/" + hex.EncodeToString(digest[:]), AdapterRevision: inference.HTTPEndpointAdapterRevision, ModelDigest: entry.ModelDigest, Format: entry.Format, Backend: backend, ContextLength: entry.ContextLength, Capabilities: []inference.ProviderCapability{inference.CapabilityText, inference.CapabilityStreaming}}
}

// InstallConfiguredHTTPEndpoints installs routes without contacting a server or
// loading a model. Remote endpoints require HTTPS; Windows managed Ollama needs
// explicit single-user enrollment and acquires its owned tree only on admission.
func (s *Server) InstallConfiguredHTTPEndpoints() error {
	if s == nil || s.config == nil {
		return errors.New("HTTP inference enrollment requires a configured service")
	}
	if s.endpointConfigurationPath() == "" {
		return nil
	}
	s.nativeRuntimeMu.Lock()
	defer s.nativeRuntimeMu.Unlock()
	if s.nativeInstallSealed || s.endpointChatSelector != nil || s.providerRegistry == nil {
		return errors.New("HTTP inference enrollment is startup-only")
	}
	path := s.endpointConfigurationPath()
	configuration, _, err := s.readEndpointConfiguration()
	if err != nil {
		return err
	}
	policy, err := newEndpointPolicy(s.config.DataDir)
	if err != nil {
		return err
	}
	policy.guard = func() error { return endpointOwnerJournalReady(s.config.DataDir) }
	s.endpointPolicy = policy
	s.endpointOwner = &endpointOwner{drafts: map[string]endpointOwnerDraft{}}
	if len(configuration.Endpoints) == 0 {
		return nil
	}
	endpoints := map[string]inference.HTTPEndpoint{}
	for _, entry := range configuration.Endpoints {
		pinned := entry
		endpoints[entry.ID+"@"+entry.Revision] = inference.HTTPEndpoint{URL: entry.URL, Model: entry.Model, RuntimeVersion: entry.RuntimeVersion, AllowedIPs: entry.AllowedIPs, Remote: entry.ManagedOllama == nil, ManagedOllama: entry.ManagedOllama, Descriptor: endpointDescriptor(entry), Authorize: func(ctx context.Context) (string, error) {
			if err := ctx.Err(); err != nil {
				return "", err
			}
			if err := policy.allowed(pinned); err != nil {
				return "", err
			}
			actor, err := s.currentEndpointActor(ctx)
			if err != nil {
				return "", err
			}
			current, err := readInferenceEndpoints(path)
			if err != nil {
				return "", err
			}
			for _, candidate := range current.Endpoints {
				if reflect.DeepEqual(candidate, pinned) && slices.Contains(candidate.Actors, actor) {
					if candidate.CredentialEnv == "" {
						return "", nil
					}
					token, ok := os.LookupEnv(candidate.CredentialEnv)
					if !ok || token == "" {
						return "", errors.New("HTTP endpoint credential is unavailable")
					}
					return token, nil
				}
			}
			return "", errors.New("HTTP endpoint enrollment changed or access was revoked")
		}, AcquireRequest: func(ctx context.Context) (context.Context, func(), error) { return policy.acquire(ctx, pinned) }}
	}
	routes, err := inference.NewHTTPEndpointRoutes(endpoints)
	if err != nil {
		return err
	}
	if err = routes.Register(s.providerRegistry); err != nil {
		return err
	}
	s.endpointChatSelector = routes
	s.endpointPolicy = policy
	s.installedEndpoints = configuration.Endpoints
	return nil
}

func (s *Server) selectEndpointChat(ctx context.Context, model string) (inference.ProviderDescriptor, bool, error) {
	s.nativeRuntimeMu.RLock()
	selector := s.endpointChatSelector
	s.nativeRuntimeMu.RUnlock()
	id, revision, ok := strings.Cut(model, "@")
	if !ok || id == "" || revision == "" || strings.Contains(revision, "@") || selector == nil {
		return inference.ProviderDescriptor{}, true, newCodedServiceError(http.StatusServiceUnavailable, "endpoint_model_unavailable", "The selected HTTP model endpoint is unavailable", nil)
	}
	descriptor, err := selector.Select(ctx, id, revision)
	if err != nil {
		return inference.ProviderDescriptor{}, true, newCodedServiceError(http.StatusForbidden, "endpoint_model_unavailable", "The selected HTTP model endpoint is unavailable or access was revoked", err)
	}
	return descriptor, true, nil
}

// Endpoint menus show only currently authorized pins. Listing never contacts a
// provider or treats installation configuration as a smoke/qualification pass.
func (s *Server) listedEndpointModels(ctx context.Context) []api.Model {
	if s.config == nil || s.endpointConfigurationPath() == "" {
		return nil
	}
	configuration, _, err := s.readEndpointConfiguration()
	if err != nil {
		return nil
	}
	var listed []api.Model
	for _, entry := range configuration.Endpoints {
		descriptor, _, err := s.selectEndpointChat(ctx, entry.ID+"@"+entry.Revision)
		if err != nil {
			continue
		}
		listed = append(listed, api.Model{ID: "endpoint:" + entry.ID + "@" + entry.Revision, Object: "model", OwnedBy: "enrolled-endpoint", Type: "llm", ContextWindow: descriptor.ContextLength, ContextLength: descriptor.ContextLength, Engine: string(descriptor.Provider), Format: string(descriptor.Format), Backend: descriptor.Backend, RunnerRevision: descriptor.RunnerRevision, AdapterRevision: descriptor.AdapterRevision, Capabilities: []string{"chat", "streaming"}, CapabilityStatus: map[string]string{"chat": "declared", "streaming": "declared", "tools": "unsupported"}})
	}
	return listed
}
