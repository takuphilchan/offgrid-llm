package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/models"
)

func TestVerifyModelUsesDisposableRegistryAndPreservesInvalidResult(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "tiny.gguf"), []byte("not a model"), 0600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	env := testEnvironment(&out, &errOut, &fakeRunner{})
	env.LocalAdmin.LoadConfig = func() *config.Config { return &config.Config{ModelsDir: root} }
	env.ModelVerify = ModelVerifyDependencies{NewRegistry: models.NewRegistry, NewValidator: models.NewValidator}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"verify"}, "Verify Model"},
		{[]string{"verify", "missing"}, "Use 'offgrid list' to see available models"},
		{[]string{"verify", "tiny"}, "Invalid Model"},
	} {
		out.Reset()
		errOut.Reset()
		if code := Execute(context.Background(), tc.args, env); code != 0 {
			t.Fatalf("%v code=%d stderr=%q", tc.args, code, errOut.String())
		}
		if !strings.Contains(out.String(), tc.want) {
			t.Fatalf("%v stdout=%q missing %q", tc.args, out.String(), tc.want)
		}
	}
}
