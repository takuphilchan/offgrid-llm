package sessions

import (
	"errors"
	"unicode/utf8"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

var ErrInvalidSeedHistory = errors.New("history must contain only bounded user/assistant text and valid images; task and source authority cannot be imported")

// ValidateSeedHistory accepts conversation content, never executable state,
// tool results, source references or client-supplied authority.
func ValidateSeedHistory(messages []Message) error {
	if len(messages) > 1024 {
		return ErrInvalidSeedHistory
	}
	textBytes, imageBytes := 0, 0
	for _, m := range messages {
		if (m.Role != "user" && m.Role != "assistant") || !utf8.ValidString(m.Content) ||
			m.JobID != "" || m.TurnID != "" || len(m.Attachments)+len(m.KnowledgeSources)+len(m.KnowledgeDocuments) != 0 ||
			(m.Role != "user" && len(m.Images) != 0) {
			return ErrInvalidSeedHistory
		}
		if err := api.ValidateSessionImages(m.Images); err != nil {
			return ErrInvalidSeedHistory
		}
		textBytes += len(m.Content)
		for _, image := range m.Images {
			imageBytes += len(image.DataURL)
		}
		if textBytes > 8<<20 || imageBytes > api.MaxSessionImageRequestBytes {
			return ErrInvalidSeedHistory
		}
	}
	return nil
}
