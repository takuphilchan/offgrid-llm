package integrations

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/integrations/verification"
)

// Explicit opt-in; installed runtimes are read only. All plugin/config/file
// writes belong to t.TempDir, not the signed-in user's runtime installation.
func TestInstalledVerificationHooks(t *testing.T) {
	for _, agent := range []string{"hermes", "openclaw"} {
		t.Run(agent, func(t *testing.T) {
			root := os.Getenv("OFFGRID_" + map[string]string{"hermes": "HERMES", "openclaw": "OPENCLAW"}[agent] + "_TEST_ROOT")
			if root == "" {
				t.Skip("installed hook fixture requires an explicit runtime root")
			}
			base := t.TempDir()
			workspace := filepath.Join(base, "workspace")
			if err := os.Mkdir(workspace, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(workspace, "source.txt"), []byte("OffGrid exact — café 日本語\n"), 0600); err != nil {
				t.Fatal(err)
			}
			adapter := verification.OpenClawAdapter
			if agent == "hermes" {
				adapter = verification.HermesAdapter
			}
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()
			s, err := verification.Start(ctx, verification.Contract{Version: 1, Kind: verification.ExactCopy, Workspace: workspace, Source: "source.txt", Destination: "copy.txt"}, adapter)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			c := s.Connection()
			repo, err := filepath.Abs(filepath.Join("..", ".."))
			if err != nil {
				t.Fatal(err)
			}
			env := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + base, "OFFGRID_VERIFY_URL=" + c.URL, "OFFGRID_VERIFY_TOKEN=" + c.Token, "OFFGRID_VERIFY_RUN=" + c.RunID, "OFFGRID_VERIFY_ADAPTER=" + c.Adapter, "PYTHONDONTWRITEBYTECODE=1", "TERMINAL_ENV=local", "TERMINAL_CWD=" + workspace, "OPENCLAW_STATE_DIR=" + filepath.Join(base, "openclaw")}
			var cmd *exec.Cmd
			if agent == "hermes" {
				home := filepath.Join(base, "hermes")
				t.Setenv("HERMES_HOME", home)
				if _, err := InstallPluginBundle("hermes-verifier", base, false); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte("updates:\n  check: false\nplugins:\n  enabled: [offgrid-verifier]\n"), 0600); err != nil {
					t.Fatal(err)
				}
				env = append(env, "HERMES_HOME="+home, "OFFGRID_HERMES_TEST_ROOT="+root)
				cmd = exec.CommandContext(ctx, filepath.Join(root, "venv", "bin", "python"), filepath.Join(repo, "scripts", "test", "installed-verification-hermes.py"))
			} else {
				env = append(env, "OFFGRID_OPENCLAW_TEST_ROOT="+root)
				cmd = exec.CommandContext(ctx, "node", filepath.Join(repo, "scripts", "test", "installed-verification-openclaw.mjs"))
			}
			cmd.Dir, cmd.Env = workspace, env
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("hook driver: %v\n%s", err, out)
			}
			receipt := s.Finish(ctx, "completed")
			if receipt.State != "passed" {
				t.Fatalf("%+v\n%s", receipt, out)
			}
			t.Logf("%+v: %s", receipt, out)
		})
	}
}
