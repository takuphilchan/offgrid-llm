package server

import (
	"context"
	"errors"
	"reflect"

	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/users"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type documentSynthesisRuntime interface {
	document.SynthesisLocalProvider
	Prepare(context.Context, string, string) (document.SynthesisModelIdentity, error)
}

type localDocumentSynthesis struct {
	server      *Server
	actor, work string
}

func (p *localDocumentSynthesis) admit(ctx context.Context, model string, window int) (*inference.LegacyGenerationLease, error) {
	if !p.server.scheduleActorPermission(p.actor, users.PermissionChat) || !p.server.scheduleActorPermission(p.actor, users.PermissionRAG) {
		return nil, rag.ErrAnalysisUnavailable
	}
	if p.server.registry == nil || p.server.engine == nil || p.server.llamaProcess == nil {
		return nil, rag.ErrAnalysisUnavailable
	}
	metadata, err := p.server.registry.GetChatModel(model)
	if err != nil || metadata.Format != "gguf" {
		return nil, rag.ErrAnalysisUnavailable
	}
	if _, ok := p.server.engine.(inference.PromptTokenCounter); !ok {
		return nil, rag.ErrAnalysisUnavailable
	}
	ctx = inference.WithSchedulingScope(inference.WithLocalRuntimeOnly(ctx), inference.SchedulingScope{Actor: p.actor, Work: p.work, Background: true})
	return p.server.acquireInferenceLease(ctx, model, window, nil)
}

func (p *localDocumentSynthesis) Prepare(ctx context.Context, model, language string) (document.SynthesisModelIdentity, error) {
	window := p.server.modelContextWindow(model)
	identity := document.SynthesisModelIdentity{ModelID: model, Language: language, ContextTokens: window, ResponseTokens: min(2048, window/3), Protocol: document.SynthesisProtocol}
	// Validate the language and context before loading a model. The digest and
	// runtime fields below are temporary admission placeholders, never stored.
	probe := identity
	probe.ModelSHA256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	probe.Runtime = document.RuntimeIdentity{Name: "pending", Revision: "pending"}
	if probe.Validate() != nil {
		return identity, rag.ErrAnalysisInvalid
	}
	lease, err := p.admit(ctx, model, window)
	if err != nil {
		return identity, err
	}
	defer lease.Release()
	return p.measure(ctx, identity)
}

func (p *localDocumentSynthesis) WithModel(ctx context.Context, expected document.SynthesisModelIdentity, run func(document.SynthesisLocalSession) error) error {
	if expected.Validate() != nil || run == nil {
		return rag.ErrAnalysisInvalid
	}
	lease, err := p.admit(ctx, expected.ModelID, expected.ContextTokens)
	if err != nil {
		return err
	}
	defer lease.Release()
	measured, err := p.measure(ctx, expected)
	if err != nil {
		return err
	}
	if measured != expected {
		return errors.New("document synthesis runtime identity changed")
	}
	observation, err := p.observe(ctx, expected)
	if err != nil {
		return err
	}
	session := &localDocumentSynthesisSession{provider: p, lease: lease, identity: expected, observation: observation, ctx: ctx}
	if err = run(session); err != nil {
		return err
	}
	measured, err = p.measure(ctx, expected)
	if err != nil || measured != expected {
		return errors.New("document synthesis artifacts changed during generation")
	}
	return session.check(ctx)
}

type localDocumentSynthesisSession struct {
	provider    *localDocumentSynthesis
	lease       *inference.LegacyGenerationLease
	identity    document.SynthesisModelIdentity
	observation *inference.ToolRuntimeObservation
	ctx         context.Context
}

func (s *localDocumentSynthesisSession) check(ctx context.Context) error {
	p := s.provider
	if !p.server.scheduleActorPermission(p.actor, users.PermissionChat) || !p.server.scheduleActorPermission(p.actor, users.PermissionRAG) {
		return rag.ErrAnalysisUnavailable
	}
	current, err := p.observe(ctx, s.identity)
	if err != nil || !reflect.DeepEqual(current, s.observation) {
		return errors.New("document synthesis process changed")
	}
	return nil
}
func (s *localDocumentSynthesisSession) Identity() document.SynthesisModelIdentity {
	if s.check(s.ctx) != nil {
		return document.SynthesisModelIdentity{}
	}
	return s.identity
}
func (s *localDocumentSynthesisSession) request(prompt document.SynthesisPrompt) *api.ChatCompletionRequest {
	temperature := float32(0)
	tokens := s.identity.ResponseTokens
	strict := true
	return &api.ChatCompletionRequest{Model: s.identity.ModelID, Messages: []api.ChatMessage{{Role: "system", Content: prompt.System}, {Role: "user", Content: prompt.User}}, Temperature: &temperature, MaxTokens: &tokens, ToolChoice: "none", ResponseFormat: &api.ResponseFormat{Type: "json_schema", JSONSchema: &api.ResponseJSONSchema{Name: "document_candidates_v1", Strict: &strict, Schema: prompt.Schema}}}
}
func (s *localDocumentSynthesisSession) Count(ctx context.Context, prompt document.SynthesisPrompt) (int, error) {
	if err := s.check(ctx); err != nil {
		return 0, err
	}
	counter, ok := s.provider.server.engine.(inference.PromptTokenCounter)
	if !ok {
		return 0, inference.ErrPromptCountUnavailable
	}
	var count int
	err := s.lease.Run(ctx, func(ctx context.Context) error {
		var err error
		count, err = counter.CountPromptTokens(ctx, s.request(prompt))
		return err
	})
	return count, err
}
func (s *localDocumentSynthesisSession) Generate(ctx context.Context, prompt document.SynthesisPrompt, tokens int) ([]byte, string, error) {
	if tokens != s.identity.ResponseTokens {
		return nil, "", rag.ErrAnalysisInvalid
	}
	if err := s.check(ctx); err != nil {
		return nil, "", err
	}
	var response *api.ChatCompletionResponse
	err := s.lease.Run(ctx, func(ctx context.Context) error {
		var err error
		response, err = s.provider.server.engine.ChatCompletion(inference.WithBoundedDocumentGeneration(ctx), s.request(prompt))
		return err
	})
	if err != nil {
		return nil, "", err
	}
	if err = s.check(ctx); err != nil {
		return nil, "", err
	}
	if response == nil || len(response.Choices) != 1 || len(response.Choices[0].Message.ToolCalls) != 0 {
		return nil, "", errors.New("invalid document synthesis response")
	}
	content, ok := response.Choices[0].Message.Content.(string)
	if !ok || len(content) > 128<<10 {
		return nil, "", errors.New("invalid document synthesis content")
	}
	return []byte(content), response.Choices[0].FinishReason, nil
}
