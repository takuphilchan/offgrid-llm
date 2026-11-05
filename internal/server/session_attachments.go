package server

import (
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func attachmentServiceError(err error) error {
	switch {
	case errors.Is(err, sessions.ErrAttachmentContextLimit):
		return newCodedServiceError(422, "attachment_context_limit", "Attached text exceeds the direct-chat limit. Choose smaller files or use Knowledge retrieval; no source was silently truncated.", nil)
	case errors.Is(err, sessions.ErrAttachmentLimit):
		return newCodedServiceError(413, "attachment_limit", "Use up to eight files, 50 MiB each, 100 MiB per conversation and 500 MiB per account.", nil)
	case errors.Is(err, sessions.ErrAttachmentUnavailable):
		return newCodedServiceError(422, "attachment_unavailable", "An attached file is unavailable, changed or not prepared. Review the attachment before sending; your draft is retained.", nil)
	case errors.Is(err, sessions.ErrAttachmentInUse):
		return newCodedServiceError(409, "attachment_in_use", "This file is used by saved or active work. Removing a draft attachment does not delete its saved source.", nil)
	default:
		return err
	}
}

func (h *SessionHandlers) handleSessionAttachments(w http.ResponseWriter, r *http.Request, name, action string) {
	w.Header().Set("Cache-Control", "no-store")
	scope := h.scoped(r)
	if _, err := scope.Load(name); err != nil {
		writeSessionError(w, err)
		return
	}
	parts := strings.Split(strings.TrimPrefix(action, "attachments"), "/")
	id := ""
	subaction := ""
	if len(parts) > 1 {
		id = parts[1]
	}
	if len(parts) > 2 {
		subaction = parts[2]
	}
	if len(parts) > 3 {
		writeError(w, "Not found", 404)
		return
	}
	if r.Method == http.MethodGet && id == "" {
		items, err := scope.ListAttachments(name)
		if err != nil {
			writeSessionError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"attachments": items})
		return
	}
	if r.Method == http.MethodGet && id != "" && subaction == "original" {
		a, data, err := scope.AttachmentOriginal(name, id)
		if err != nil {
			writeSessionError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": a.Name}))
		_, _ = w.Write(data)
		return
	}
	if r.Method == http.MethodGet && id != "" && subaction == "passage" {
		start, startErr := strconv.Atoi(r.URL.Query().Get("start"))
		end, endErr := strconv.Atoi(r.URL.Query().Get("end"))
		revision := r.URL.Query().Get("revision")
		if startErr != nil || endErr != nil || revision == "" {
			writeError(w, "A pinned source revision and valid passage range are required", 400)
			return
		}
		passage, err := scope.ReadAttachmentPassage(name, sessions.AttachmentReference{ID: id, Revision: revision}, start, end)
		if err != nil {
			writeSessionError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(passage)
		return
	}
	if r.Method == http.MethodGet && id != "" && subaction == "preview" {
		a, data, err := scope.AttachmentOriginal(name, id)
		if err != nil {
			writeSessionError(w, err)
			return
		}
		parser := rag.NewDocumentParser()
		if h.agentServer != nil && h.agentServer.ragEngine != nil {
			parser = h.agentServer.ragEngine.DocumentParser()
		}
		serveDocumentPreview(w, r, parser, a.Name, data, func() bool {
			current, _, err := scope.AttachmentOriginal(name, id)
			return err == nil && current.SHA256 == a.SHA256
		})
		return
	}
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		writeError(w, "Method not allowed", 405)
		return
	}
	unlock, ok := h.tryLockSession(name)
	if !ok {
		writeSessionError(w, sessions.ErrTurnConflict)
		return
	}
	defer unlock()
	if r.Method == http.MethodDelete && id != "" && subaction == "" {
		if err := scope.DeleteAttachment(name, id); err != nil {
			writeSessionError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"removed": true})
		return
	}
	var a sessions.Attachment
	var data []byte
	var err error
	if r.Method == http.MethodPost && id == "" {
		var filename, mediaType string
		data, filename, mediaType, err = readDocumentUpload(r)
		if err != nil {
			writeDocumentIngestError(w, err)
			return
		}
		if !rag.IsSupportedExtension(strings.ToLower(filepath.Ext(filename))) {
			writeErrorWithCode(w, "This file format is not supported for document attachment.", 422, "document_unsupported_format")
			return
		}
		a, err = scope.CreateAttachment(name, filename, mediaType, data)
	} else if r.Method == http.MethodPost && id != "" && subaction == "prepare" {
		a, data, err = scope.AttachmentOriginal(name, id)
	} else {
		writeError(w, "Not found", 404)
		return
	}
	if err != nil {
		writeSessionError(w, err)
		return
	}
	if a.Status == "ready" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"attachment": a})
		return
	}
	parser := rag.NewDocumentParser()
	if h.agentServer != nil && h.agentServer.ragEngine != nil {
		parser = h.agentServer.ragEngine.DocumentParser()
	}
	textOnly := r.URL.Query().Get("text_only") == "true"
	var result *rag.ParseResult
	var parseErr error
	if textOnly {
		result, parseErr = parser.PreviewContext(r.Context(), data, a.Name, filepath.Ext(a.Name))
		var problem *rag.ParseError
		if result != nil && strings.TrimSpace(result.Content) != "" && errors.As(parseErr, &problem) && problem.Code == "document_ocr_required" {
			parseErr = nil
		}
	} else {
		// Inspect native PDF text first. Usable text with visual limitations does
		// not need a full-page OCR pass, but remains explicitly text-only.
		if strings.EqualFold(filepath.Ext(a.Name), ".pdf") {
			result, parseErr = parser.PreviewContext(r.Context(), data, a.Name, filepath.Ext(a.Name))
			var problem *rag.ParseError
			if errors.As(parseErr, &problem) && problem.Code == "document_ocr_required" && rag.NativeTextUsable(result) {
				textOnly, parseErr = true, nil
			}
		}
		var problem *rag.ParseError
		if !strings.EqualFold(filepath.Ext(a.Name), ".pdf") || errors.As(parseErr, &problem) && problem.Code == "document_ocr_required" {
			result, parseErr = parser.ParseContext(r.Context(), data, a.Name, filepath.Ext(a.Name))
		}
	}
	if parseErr == nil && (result == nil || strings.TrimSpace(result.Content) == "") {
		parseErr = &rag.ParseError{Code: "document_invalid", Message: "This file has no readable text."}
	}
	text, code := "", ""
	if result != nil {
		text = result.Content
	}
	if parseErr != nil {
		code = "document_invalid"
		var typed *rag.ParseError
		if errors.As(parseErr, &typed) {
			code = typed.Code
		}
		if r.Context().Err() != nil {
			code = "document_cancelled"
		}
	} else {
		text = result.Content
	}
	parserRevision := "document-parser-v1"
	if result != nil && result.Metadata["extractor_revision"] != "" {
		parserRevision += "/" + result.Metadata["extractor_revision"]
	}
	if textOnly && parseErr == nil && result.State == rag.ExtractionPartial {
		a, err = scope.PrepareAttachmentTextOnly(name, a.ID, text, parserRevision)
	} else {
		a, err = scope.PrepareAttachment(name, a.ID, text, parserRevision, code)
	}
	if err != nil {
		writeSessionError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if parseErr != nil {
		if code == "document_busy" {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(422)
		}
	}
	response := map[string]any{"attachment": a}
	if parseErr != nil {
		message := "The file could not be prepared. Review its format and extraction requirements."
		var typed *rag.ParseError
		if errors.As(parseErr, &typed) {
			message = typed.Message
		}
		response["error"] = map[string]string{"code": code, "message": message}
	}
	_ = json.NewEncoder(w).Encode(response)
}

func (h *SessionHandlers) validateSessionAttachments(r *http.Request, name string, req *sessionGenerateRequest) error {
	saved, err := h.scoped(r).Load(name)
	if err != nil {
		return nil
	}
	hasFiles := len(req.Attachments) > 0
	for _, m := range saved.Messages {
		hasFiles = hasFiles || len(m.Attachments) > 0
	}
	if !hasFiles {
		return nil
	}
	if req.Agent != nil {
		return newCodedServiceError(422, "agent_attachments_unavailable", "File attachment context is not supported for agent turns yet. Use ordinary Chat; no file access has been granted.", nil)
	}
	if !req.Stream || !req.Durable {
		return newCodedServiceError(400, "attachment_durable_required", "Files require durable conversation streaming; use a compatible client.", nil)
	}
	return nil
}

func collectAttachmentMessages(scope *sessions.ScopedManager, name string, session *sessions.Session, turn *sessions.Turn) ([]api.ChatMessage, []sessions.AttachmentText, error) {
	messages := make([]api.ChatMessage, 0, len(session.Messages)+1)
	var allSources []sessions.AttachmentText
	seen := map[string]string{}
	appendMessage := func(role, text string, images []api.SessionImage, refs []sessions.AttachmentReference) error {
		var sources []sessions.AttachmentText
		var err error
		if len(refs) > 0 {
			sources, err = scope.AttachmentSources(name, refs)
		}
		if err != nil {
			return attachmentServiceError(err)
		}
		for _, source := range sources {
			if revision, ok := seen[source.Attachment.ID]; ok {
				if revision != source.Attachment.Revision {
					return attachmentServiceError(sessions.ErrAttachmentUnavailable)
				}
				continue
			}
			seen[source.Attachment.ID] = source.Attachment.Revision
			allSources = append(allSources, source)
		}
		messages = append(messages, api.ChatMessage{Role: role, Content: api.SessionContent(text, images)})
		return nil
	}
	for _, m := range session.Messages {
		if err := appendMessage(m.Role, m.Content, m.Images, m.Attachments); err != nil {
			return nil, nil, err
		}
	}
	if err := appendMessage("user", turn.Prompt, turn.Images, turn.Attachments); err != nil {
		return nil, nil, err
	}
	return messages, allSources, nil
}
