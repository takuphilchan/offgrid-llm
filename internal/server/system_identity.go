package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
)

// SystemIdentity contains public compatibility metadata, never local paths,
// credentials, user identities, or model/document inventory. Capability names
// describe implemented contracts, not claims of production qualification.
type SystemIdentity struct {
	Product       string                 `json:"product"`
	Version       string                 `json:"version"`
	Revision      string                 `json:"revision"`
	APIVersion    int                    `json:"api_version"`
	UIBuildID     string                 `json:"ui_build_id"`
	Capabilities  []string               `json:"capabilities"`
	WorkspaceID   string                 `json:"workspace_id,omitempty"`
	DesktopBridge *DesktopBridgeIdentity `json:"desktop_bridge,omitempty"`
}

// DesktopBridgeIdentity is compatibility metadata, never authorization.
type DesktopBridgeIdentity struct {
	Status        string `json:"status"`
	SchemaVersion int    `json:"schema_version,omitempty"`
	Protocol      int    `json:"protocol,omitempty"`
	UIBuildID     string `json:"ui_build_id,omitempty"`
}

func desktopBridgeIdentity(root, uiBuildID string) *DesktopBridgeIdentity {
	file, err := os.Open(filepath.Join(root, "desktop-compatibility.json"))
	if os.IsNotExist(err) {
		return &DesktopBridgeIdentity{Status: "missing"}
	}
	invalid := &DesktopBridgeIdentity{Status: "invalid"}
	if err != nil {
		return invalid
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 4096 {
		return invalid
	}
	data, err := io.ReadAll(io.LimitReader(file, 4097))
	if err != nil || len(data) > 4096 {
		return invalid
	}
	var manifest struct {
		SchemaVersion int    `json:"schema_version"`
		Protocol      int    `json:"protocol"`
		UIBuildID     string `json:"ui_build_id"`
	}
	if json.Unmarshal(data, &manifest) != nil || manifest.SchemaVersion != 1 || manifest.Protocol < 1 ||
		manifest.Protocol > 2147483647 || len(uiBuildID) != 64 || manifest.UIBuildID != uiBuildID {
		return invalid
	}
	return &DesktopBridgeIdentity{Status: "ready", SchemaVersion: 1, Protocol: manifest.Protocol, UIBuildID: uiBuildID}
}

// BuildRevision is injected when the build context excludes .git (containers).
// Development builds retain "unknown" unless Go embeds VCS metadata.
var BuildRevision = "unknown"

func (s *Server) handleSystemIdentity(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	identity := SystemIdentity{Product: "offgrid", Version: s.version, APIVersion: 2, Revision: BuildRevision, Capabilities: []string{"sessions-v1", "chat-streaming-v1", "durable-agent-runs-v1", "exclusive-workspace-v1", "offline-backup-v1"}}
	identity.WorkspaceID = s.workspaceID
	identity.Capabilities = append(identity.Capabilities, "agent-response-profiles-v1")
	identity.Capabilities = append(identity.Capabilities, "scoped-knowledge-chat-v1")
	identity.Capabilities = append(identity.Capabilities, "session-images-v1")
	identity.Capabilities = append(identity.Capabilities, "chat-image-orientation-v1")
	identity.Capabilities = append(identity.Capabilities, "session-files-v1")
	identity.Capabilities = append(identity.Capabilities, "document-imports-v1")
	identity.Capabilities = append(identity.Capabilities, "document-evidence-receipts-v1")
	identity.Capabilities = append(identity.Capabilities, "document-structure-review-v1")
	identity.Capabilities = append(identity.Capabilities, "session-document-corrections-v1")
	identity.Capabilities = append(identity.Capabilities, "document-navigation-v1")
	identity.Capabilities = append(identity.Capabilities, "document-comparison-jobs-v1")
	identity.Capabilities = append(identity.Capabilities, "document-analysis-history-v1")
	identity.Capabilities = append(identity.Capabilities, "document-analysis-export-v1")
	identity.Capabilities = append(identity.Capabilities, "document-synthesis-candidates-v1")
	identity.Capabilities = append(identity.Capabilities, "document-synthesis-export-v1")
	identity.Capabilities = append(identity.Capabilities, "document-synthesis-review-v1")
	identity.Capabilities = append(identity.Capabilities, "document-composition-v1")
	identity.Capabilities = append(identity.Capabilities, "document-linked-evidence-v1")
	identity.Capabilities = append(identity.Capabilities, "document-image-ocr-v1")
	identity.Capabilities = append(identity.Capabilities, "document-tiff-ocr-v1")
	identity.Capabilities = append(identity.Capabilities, "document-ocr-language-selection-v1")
	identity.Capabilities = append(identity.Capabilities, "document-ocr-mixed-languages-v1")
	identity.Capabilities = append(identity.Capabilities, "document-ocr-page-recovery-v1")
	identity.Capabilities = append(identity.Capabilities, "session-attachment-structure-v1")
	identity.Capabilities = append(identity.Capabilities, "knowledge-retention-v1")
	identity.Capabilities = append(identity.Capabilities, "knowledge-collections-v1")
	identity.Capabilities = append(identity.Capabilities, "document-attachment-imports-v1")
	identity.Capabilities = append(identity.Capabilities, "session-history-seed-v1")
	identity.Capabilities = append(identity.Capabilities, "gguf-bundles-v1")
	identity.Capabilities = append(identity.Capabilities, "gguf-offline-import-v1")
	identity.Capabilities = append(identity.Capabilities, "work-search-v1")
	if s.endpointOwner != nil {
		// Protocol support survives an unavailable owner/policy so clients can
		// still offer inspection and explicit recovery. This grants no writes.
		identity.Capabilities = append(identity.Capabilities, "inference-endpoint-owner-v1")
	}
	if s.agentManager != nil {
		identity.Capabilities = append(identity.Capabilities, "work-summary-v1")
		if s.agentRunner != nil && s.agentManager.ActivityReplaySupported() {
			identity.Capabilities = append(identity.Capabilities, "conversation-inputs-v1", "conversation-input-options-v1")
			if s.sessionHandlers != nil && s.sessionHandlers.agentServer == s {
				identity.Capabilities = append(identity.Capabilities, "conversation-source-inputs-v1", "conversation-source-scopes-v1")
			}
			if s.config != nil && s.config.EnableConversationPreview {
				identity.Capabilities = append(identity.Capabilities, "conversation-input-admission-v1")
			}
		}
	}
	if s.executionEnrollment != nil && s.config != nil && s.config.EnableExecutionPreview {
		identity.Capabilities = append(identity.Capabilities, "native-execution-preview-v1", "execution-access-request-v1")
	}
	if s.browserHub != nil {
		identity.Capabilities = append(identity.Capabilities, "native-computer-sessions-v2", "native-document-negotiation-v1", "browser-discovery-v1", "browser-workspace-v1", "browser-structured-v1", "browser-task-pairing-v1")
		if s.agentRunner != nil && s.agentManager != nil && s.agentManager.ActivityReplaySupported() {
			identity.Capabilities = append(identity.Capabilities, "task-first-agents-v2", "task-desktop-sessions-v3")
		}
	}
	if s.agentManager != nil && s.agentManager.ActivityReplaySupported() {
		identity.Capabilities = append(identity.Capabilities, "durable-agent-events-v2")
		if s.agentRunner != nil {
			identity.Capabilities = append(identity.Capabilities, "task-lifecycle-v2", "bounded-readonly-delegation-v1", "task-context-archive-v1", "agent-profiles-v1", "agent-profile-deletion-v1", "agent-context-resources-v1", "agent-schedules-v1")
			if s.sessionHandlers != nil && s.sessionHandlers.agentServer == s {
				identity.Capabilities = append(identity.Capabilities, "agent-conversations-v1")
			}
		}
		if s.agentRunner != nil && s.artifactStore != nil {
			identity.Capabilities = append(identity.Capabilities, "verified-workspace-artifacts-v1")
		}
	}
	if info, ok := debug.ReadBuildInfo(); ok && identity.Revision == "unknown" {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				identity.Revision = setting.Value
			}
		}
	}
	uiRoot := resolveUIRoot()
	if data, err := os.ReadFile(filepath.Join(uiRoot, "index.html")); err == nil {
		// Git checkouts can use CRLF on Windows and LF in containers. A
		// newline-only difference is not a different renderer build.
		digest := sha256.Sum256(bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n")))
		identity.UIBuildID = hex.EncodeToString(digest[:])
	}
	identity.DesktopBridge = desktopBridgeIdentity(uiRoot, identity.UIBuildID)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(identity)
}
