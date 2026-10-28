package server

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/models"
)

// Explicit isolated candidate evaluation. This synthetic pin enables the same
// runner scope boundary without installing qualification in the live service.
// Passing tests is evidence for review, not automatic activation or permission.
func applyNativeWorkspaceTestScope(t *testing.T, cfg *agents.AgentConfig, adapter string) {
	t.Helper()
	scope := os.Getenv("OFFGRID_TEST_NATIVE_SCOPE")
	if scope == "" {
		return
	}
	if _, knownScope := models.NativeWorkspaceCasesForScope(scope); !knownScope || adapter != "native" {
		t.Fatal("unknown or incompatible isolated test scope")
	}
	cfg.ToolAdapter = &inference.AgentToolPin{Adapter: inference.NativeAgentToolAdapter, Scope: scope, CapabilitySHA256: agents.NativeWorkspaceCapabilitySHA256(), EvidenceID: strings.Repeat("a", 64), FingerprintID: strings.Repeat("b", 64)}
	cfg.ContextWindow = 16384
	if scope == models.NativeWorkspaceContractScope {
		window, err := strconv.Atoi(os.Getenv("OFFGRID_TEST_RUNTIME_CONTEXT"))
		if err != nil || window < 4096 || window > 1048576 {
			t.Fatal("v2 candidate requires explicit measured OFFGRID_TEST_RUNTIME_CONTEXT")
		}
		cfg.ContextWindow = window
	}
	t.Logf("isolated_candidate_scope=%s suite_sha256=%s capability_sha256=%s context_budget=%d; no qualification installed", scope, models.NativeWorkspaceSuiteSHA256ForScope(scope), cfg.ToolAdapter.CapabilitySHA256, cfg.ContextWindow)
}
