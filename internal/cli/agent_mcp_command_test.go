package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/config"
)

func TestLegacyMCPUsesDisposableConfigAndSurvivesMalformedEntries(t *testing.T) {
	var out, errOut bytes.Buffer
	env := testEnvironment(&out, &errOut, &fakeRunner{})
	root := t.TempDir()
	env.LocalAdmin.LoadConfig = func() *config.Config { return &config.Config{DataDir: root} }
	env.LegacyAgent.MCP = MCPDependencies{
		ReadFile: os.ReadFile, MkdirAll: os.MkdirAll, WriteFile: os.WriteFile,
		Now: func() time.Time { return time.Unix(12345, 0) },
	}
	path := filepath.Join(root, "tools.json")
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"agent", "mcp", "add", "http://localhost:3100", "--name", "fixture"}, "Added"},
		{[]string{"agent", "mcp", "list"}, "fixture"},
		{[]string{"agent", "mcp", "disable", "fixture"}, "Disabled"},
		{[]string{"agent", "mcp", "enable", "fixture"}, "Enabled"},
		{[]string{"agent", "mcp", "remove", "fixture"}, "Removed"},
	} {
		out.Reset()
		errOut.Reset()
		if code := Execute(context.Background(), tc.args, env); code != 0 || !strings.Contains(out.String(), tc.want) || errOut.Len() != 0 {
			t.Fatalf("%v: code=%d stdout=%q stderr=%q", tc.args, code, out.String(), errOut.String())
		}
	}
	var saved map[string]any
	data, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(data, &saved) != nil {
		t.Fatalf("saved config: %v", err)
	}
	if servers, ok := saved["mcp_servers"].([]any); !ok || len(servers) != 0 {
		t.Fatalf("servers not removed: %#v", saved)
	}
	// A malformed server name must not panic or write a modified file.
	malformed := []byte(`{"mcp_servers":[{"enabled":true}]}`)
	if err := os.WriteFile(path, malformed, 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errOut.Reset()
	if code := Execute(context.Background(), []string{"agent", "mcp", "remove", "missing"}, env); code != 0 || !strings.Contains(errOut.String(), "MCP server not found") {
		t.Fatalf("malformed entry: code=%d stderr=%q", code, errOut.String())
	}
	data, err = os.ReadFile(path)
	if err != nil || !bytes.Equal(data, malformed) {
		t.Fatalf("malformed file changed: %v", err)
	}
}
