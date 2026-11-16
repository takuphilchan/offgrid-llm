package server

import (
	"context"
	"encoding/json"
	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/internal/users"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
	"net/http"
	"slices"
)

// Resolve through the owning session and canonical import manifest. A session
// name supplied by a client is never sufficient authority to attach its files.
func (s *Server) conversationAttachmentSources(r *http.Request, conversation string, input api.ConversationInboxInput) ([]sessions.AttachmentText, error) {
	if len(input.Attachments) == 0 {
		return nil, nil
	}
	if len(input.Attachments) > 8 || s.sessionHandlers == nil {
		return nil, agents.ErrControlCommand
	}
	manifest, err := s.agentManager.ImportedConversation(s.controlScope(r), input.SourceSession)
	if err != nil || manifest == nil || manifest.ConversationID != conversation {
		return nil, agents.ErrControlNotFound
	}
	refs := make([]sessions.AttachmentReference, len(input.Attachments))
	for i, ref := range input.Attachments {
		refs[i] = sessions.AttachmentReference{ID: ref.ID, Revision: ref.Revision}
	}
	sources, err := s.sessionHandlers.scoped(r).AttachmentSources(input.SourceSession, refs)
	if err != nil {
		return nil, attachmentServiceError(err)
	}
	for i, ref := range input.Attachments {
		if ref.Page > 0 {
			if _, err = sessions.SelectAttachmentPage(sources[i], ref.Page, sessions.MaxAttachmentContextBytes); err != nil {
				return nil, attachmentServiceError(err)
			}
		}
	}
	return sources, nil
}

func (s *Server) validateConversationSources(r *http.Request, c *agents.Conversation, input api.ConversationInboxInput) error {
	if len(input.KnowledgeSelections) > 0 && (len(input.KnowledgeDocuments) > 0 || input.UseKnowledge) {
		return newCodedServiceError(400, "knowledge_scope_unavailable", "Explicit source scope cannot be combined with broad retrieval", nil)
	}
	if _, err := s.conversationAttachmentSources(r, c.ID, input); err != nil {
		return err
	}
	if err := api.ValidateSessionImages(input.Images); err != nil {
		return newServiceError(400, err.Error(), nil)
	}
	for _, selected := range input.KnowledgeSelections {
		input.KnowledgeDocuments = append(input.KnowledgeDocuments, selected.DocumentID)
	}
	if len(input.Images) == 0 && len(input.KnowledgeDocuments) == 0 && !input.UseKnowledge {
		return nil
	}
	if (input.UseKnowledge || len(input.KnowledgeDocuments) > 0) && !s.scheduleActorPermission(s.agentActor(r), users.PermissionRAG) {
		return agents.ErrSourceAccess
	}
	effective, err := s.effectiveConversationPreferences(r, c, input.Model)
	if err != nil {
		return err
	}
	if len(input.Images) > 0 {
		if s.registry == nil {
			return newServiceError(503, "Model catalog unavailable", nil)
		}
		if err = s.sessionImageReady(effective.Model); err != nil {
			return err
		}
	}
	if len(input.KnowledgeDocuments) > 16 {
		return agents.ErrControlCommand
	}
	if effective.ProfileID != "" && len(input.KnowledgeDocuments) > 0 {
		profile, err := s.agentManager.GetProfile(s.workspaceID, s.agentActor(r), effective.ProfileID, effective.ProfileRevision)
		if err != nil {
			return err
		}
		for _, id := range input.KnowledgeDocuments {
			if !slices.Contains(profile.Settings.Knowledge, id) {
				return agents.ErrSourceAccess
			}
		}
	}
	for _, id := range input.KnowledgeDocuments {
		if effective.Restrictions.Knowledge != nil && !slices.Contains(*effective.Restrictions.Knowledge, id) {
			return agents.ErrSourceAccess
		}
	}
	if !s.savedKnowledgeAllowed(s.agentActor(r), input.KnowledgeDocuments) {
		return agents.ErrSourceAccess
	}
	for _, selected := range input.KnowledgeSelections {
		if _, err := s.ragEngine.ForActor(modelActor(r)).SelectEvidence(r.Context(), selected.DocumentID, selected.Revision, selected.Page, selected.BlockIDs, 64<<10); err != nil {
			return newCodedServiceError(422, "knowledge_scope_unavailable", "Selected source scope is unavailable or exceeds the evidence budget", err)
		}
	}
	return nil
}

func (s *Server) attachConversationInputSources(r *http.Request, c *agents.Conversation, input api.ConversationInboxInput, config *agents.AgentConfig) error {
	if err := s.validateConversationSources(r, c, input); err != nil {
		return err
	}
	sources, err := s.conversationAttachmentSources(r, c.ID, input)
	if err != nil {
		return err
	}
	if len(input.KnowledgeDocuments) > 0 {
		ids := append([]string(nil), input.KnowledgeDocuments...)
		config.KnowledgeSelection = &ids
		response := &preparationResponse{header: http.Header{}}
		if !s.attachProfileKnowledge(response, r, input.Content, config) {
			return response.preparationError()
		}
	}
	if input.UseKnowledge && len(input.KnowledgeDocuments) == 0 && config.Profile == nil && config.EffectivePreferences != nil && config.EffectivePreferences.Restrictions.Knowledge == nil {
		if s.ragEngine == nil || !s.ragEngine.IsEnabled() {
			return newServiceError(503, "Knowledge is unavailable", nil)
		}
		ctx := withKnowledgeScope(r.Context(), nil, &config.KnowledgeSources)
		material, err := retrieveKnowledge(ctx, s.ragEngine.ForActor(modelActor(r)), input.Content)
		if err != nil {
			return err
		}
		if material != nil {
			material.TruncateContext(6000)
			config.KnowledgeContext = material.Context
			captureKnowledge(ctx, material)
		}
	}
	if len(sources) > 0 {
		// Conservative byte admission; the runner measures the complete prompt again
		// before generation and rejects overflow rather than dropping current evidence.
		budget := min(sessions.MaxAttachmentContextBytes, max(600, config.ContextWindow-config.MaxTokens-2048))
		if config.ContextWindow == 0 {
			budget = 6000
		}
		selected := make([]sessions.AttachmentText, len(sources))
		for i, source := range sources {
			share := budget / len(sources)
			if input.Attachments[i].Page > 0 {
				selected[i], err = sessions.SelectAttachmentPage(source, input.Attachments[i].Page, share)
			} else {
				var results []sessions.AttachmentText
				results, err = sessions.SelectAttachmentText([]sessions.AttachmentText{source}, input.Content, share)
				if err == nil {
					selected[i] = results[0]
				}
			}
			if err != nil {
				return attachmentServiceError(err)
			}
		}
		config.InputEvidence = sessions.AttachmentPrompt(input.Content, selected)
		config.AttachmentSession = input.SourceSession
		config.InputAttachments = append([]api.ConversationAttachment(nil), input.Attachments...)
		config.AttachmentSelections, err = json.Marshal(sessions.AttachmentSelections(selected))
		if err != nil {
			return err
		}
	}
	if len(input.KnowledgeSelections) > 0 {
		// An explicit scope cannot also request broad retrieval of the same document.
		material := &rag.RAGContext{RetrievalMode: "selected-evidence"}
		config.KnowledgeContext = ""
		config.KnowledgeSources = nil
		config.KnowledgeDocuments = nil
		selectedIDs := make([]string, 0, len(input.KnowledgeSelections))
		for _, selected := range input.KnowledgeSelections {
			part, err := s.ragEngine.ForActor(modelActor(r)).SelectEvidence(r.Context(), selected.DocumentID, selected.Revision, selected.Page, selected.BlockIDs, 6000)
			if err != nil {
				return newCodedServiceError(422, "knowledge_scope_unavailable", "Selected source scope is unavailable or exceeds the evidence budget", err)
			}
			material.Results = append(material.Results, part.Results...)
			selectedIDs = append(selectedIDs, selected.DocumentID)
			config.KnowledgeDocuments = append(config.KnowledgeDocuments, agents.KnowledgeDocument{ID: selected.DocumentID, Name: part.Results[0].DocName})
		}
		material.FormatContext()
		if len(material.Context) > 6000 {
			return newCodedServiceError(422, "knowledge_scope_unavailable", "Selected source scope exceeds the evidence budget", nil)
		}
		config.KnowledgeContext = material.Context
		config.KnowledgeSelection = &selectedIDs
		captureKnowledge(withKnowledgeScope(r.Context(), nil, &config.KnowledgeSources), material)
	}
	config.InputImages = append([]api.SessionImage(nil), input.Images...)
	return nil
}

func (s *Server) taskAttachmentsAllowed(actor string, task *agents.Task) bool {
	if len(task.Config.InputAttachments) == 0 {
		return true
	}
	if task.Origin == nil || task.Actor != actor {
		return false
	}
	scope := agents.ControlScope{Actor: actor, Workspace: s.workspaceID}
	ctx, err := s.conversationActorContext(context.Background(), scope)
	if err != nil {
		return false
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	if err != nil {
		return false
	}
	_, err = s.conversationAttachmentSources(request, task.Origin.Conversation, api.ConversationInboxInput{SourceSession: task.Config.AttachmentSession, Attachments: task.Config.InputAttachments})
	return err == nil
}
