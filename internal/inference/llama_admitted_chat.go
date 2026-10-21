package inference

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// StreamAdmittedLlamaChat adapts a service-owned llama engine to the normalized
// provider stream after its caller has acquired the shared scheduler/model
// lifecycle lease. It must not admit a second generation or own the cache: the
// existing GGUF process and projector lifecycle remain with that lease.
// legacyIdentity identifies only the loaded compatibility instance; it is not
// artifact qualification evidence or a cryptographic hash of model weights.
func StreamAdmittedLlamaChat(ctx context.Context, engine Engine, legacyIdentity string, contextLength int, request *api.ChatCompletionRequest, callback func(ProviderEvent) error) error {
	if engine == nil || legacyIdentity == "" || contextLength < 1 || request == nil || request.Model == "" || len(request.Messages) == 0 || callback == nil {
		return errors.New("admitted llama generation requires a loaded identity, request, and callback")
	}
	if len(request.Tools) != 0 {
		return errors.New("admitted llama text stream does not support a tool catalog")
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	id := hex.EncodeToString(nonce[:])
	descriptor := ProviderDescriptor{
		ProtocolRevision: ProviderProtocolRevision,
		Provider:         ProviderLlamaCPP,
		RunnerRevision:   "legacy-llama-http-v1",
		ModelDigest:      legacyIdentity,
		Format:           FormatGGUF,
		Backend:          "unmeasured",
		Capabilities:     []ProviderCapability{CapabilityText, CapabilityStreaming},
		ContextLength:    contextLength,
	}
	provider, err := NewLlamaCPPProviderForEngine(engine, descriptor)
	if err != nil {
		return err
	}
	defer provider.Close()
	if err := provider.Health(ctx); err != nil {
		return err
	}
	prompt, err := json.Marshal(request)
	if err != nil {
		return err
	}
	providerRequest := ProviderRequest{RequestID: id, CancellationID: id, ModelDigest: legacyIdentity, Prompt: prompt, ContextLength: contextLength}
	gate := &providerStreamGate{descriptor: descriptor, request: providerRequest, callback: callback}
	if err := provider.Generate(ctx, providerRequest, gate.accept); err != nil {
		return err
	}
	return gate.finish()
}
