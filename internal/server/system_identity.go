package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
)

// SystemIdentity contains public compatibility metadata, never local paths,
// credentials, user identities, or model/document inventory. Capability names
// describe implemented contracts, not claims of production qualification.
type SystemIdentity struct {
	Product      string   `json:"product"`
	Version      string   `json:"version"`
	Revision     string   `json:"revision"`
	APIVersion   int      `json:"api_version"`
	UIBuildID    string   `json:"ui_build_id"`
	Capabilities []string `json:"capabilities"`
	WorkspaceID  string   `json:"workspace_id,omitempty"`
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
	if s.browserHub != nil {
		identity.Capabilities = append(identity.Capabilities, "native-computer-sessions-v2")
		if s.agentRunner != nil && s.agentManager != nil && s.agentManager.ActivityReplayAvailable() {
			identity.Capabilities = append(identity.Capabilities, "task-first-agents-v2")
		}
	}
	if s.agentManager != nil && s.agentManager.ActivityReplayAvailable() {
		identity.Capabilities = append(identity.Capabilities, "durable-agent-events-v2")
		if s.agentRunner != nil {
			identity.Capabilities = append(identity.Capabilities, "task-lifecycle-v2", "bounded-readonly-delegation-v1", "task-context-archive-v1")
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
	if data, err := os.ReadFile(filepath.Join(resolveUIRoot(), "index.html")); err == nil {
		// Git checkouts can use CRLF on Windows and LF in containers. A
		// newline-only difference is not a different renderer build.
		digest := sha256.Sum256(bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n")))
		identity.UIBuildID = hex.EncodeToString(digest[:])
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(identity)
}
