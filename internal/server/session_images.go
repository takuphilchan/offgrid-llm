package server

import (
	"net/http"
	"os"

	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func (s *Server) sessionImageReady(model string) error {
	metadata, err := s.registry.GetChatModel(model)
	if err == nil && metadata.ProjectorPath != "" {
		if info, err := os.Stat(metadata.ProjectorPath); err == nil && info.Mode().IsRegular() && info.Size() > 0 {
			return nil
		}
	}
	return newCodedServiceError(422, "vision_projector_unavailable", "Select a local vision model with its installed projector. Images were not sent to inference.", nil)
}

func (h *SessionHandlers) validateSessionImageRequest(r *http.Request, name string, req *sessionGenerateRequest) error {
	if err := api.ValidateSessionImages(req.Images); err != nil {
		return newServiceError(400, err.Error(), nil)
	}
	if len(req.Images) == 0 {
		// Historical image bytes are projected according to the current model.
		// Their presence must not permanently lock the conversation to vision.
		return nil
	}
	if !req.Stream || !req.Durable {
		return newServiceError(400, "Images require durable Chat streaming", nil)
	}
	saved, err := h.scoped(r).Load(name)
	if err != nil {
		return nil
	} // Existing handler owns missing/inaccessible session responses.
	encodedBytes := 0
	for _, item := range req.Images {
		encodedBytes += len(item.DataURL)
	}
	for _, message := range saved.Messages {
		for _, item := range message.Images {
			encodedBytes += len(item.DataURL)
		}
	}
	if req.Agent != nil {
		return newServiceError(422, "Image input is not supported for agent turns. Use ordinary Chat or start a text-only conversation.", nil)
	}
	// Let durable identity validation resolve an existing request even if its
	// model was removed or its already-saved images fill the conversation budget.
	if req.RequestID != "" && saved.Turn != nil && saved.Turn.ID == req.RequestID {
		return nil
	}
	if encodedBytes > api.MaxSessionImageRequestBytes {
		return newServiceError(400, "This conversation's image limit is reached. Start a new conversation; your current draft is retained.", nil)
	}
	model := req.ModelID
	if model == "" {
		model = saved.ModelID
	}
	if h.imageReady == nil {
		return newServiceError(503, "Image input is unavailable on this service", nil)
	}
	return h.imageReady(model)
}

const unavailableHistoricalImages = "\n[Historical image content is retained in this conversation but is unavailable to the current model/turn. Do not infer unseen image details. Select a vision-capable model to inspect it.]"

// Only the inference projection changes. Saved image bytes and prior prose
// remain intact, so switching back to vision restores the original context.
func (h *SessionHandlers) sessionHistoryForModel(saved *sessions.Session, model string) *sessions.Session {
	hasImages := false
	for _, message := range saved.Messages {
		hasImages = hasImages || len(message.Images) > 0
	}
	if !hasImages {
		return saved
	}
	if h.imageReady != nil && h.imageReady(model) == nil {
		return saved
	}
	projected := *saved
	projected.Messages = append([]sessions.Message(nil), saved.Messages...)
	for i := range projected.Messages {
		if len(projected.Messages[i].Images) > 0 {
			projected.Messages[i].Content += unavailableHistoricalImages
			projected.Messages[i].Images = nil
		}
	}
	return &projected
}
