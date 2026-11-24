package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConversationAdmissionDefaultAndExplicitDisable(t *testing.T) {
	t.Setenv("OFFGRID_ENABLE_CONVERSATION_PREVIEW", "")
	if !LoadConfig().EnableConversationPreview {
		t.Fatal("ordinary default assistant admission disabled")
	}
	for _, body := range []string{`{}`, `{"enable_conversation_preview":false}`} {
		path := filepath.Join(t.TempDir(), "settings.json")
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		cfg, err := LoadFromFile(path)
		if err != nil || cfg.EnableConversationPreview != (body == `{}`) {
			t.Fatal(cfg, err)
		}
	}
	t.Setenv("OFFGRID_ENABLE_CONVERSATION_PREVIEW", "false")
	if LoadConfig().EnableConversationPreview {
		t.Fatal("explicit disable ignored")
	}
}
