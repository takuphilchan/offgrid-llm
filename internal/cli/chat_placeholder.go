package cli

import (
	"context"
	"fmt"
)

// The legacy chat command is intentionally a help placeholder. This internal
// route preserves it until an explicitly approved CLI chat product change.
func chatPlaceholderCommand(_ context.Context, _ []string, env Environment) error {
	_, _ = fmt.Fprintln(env.Stdout, "Interactive Chat Mode")
	_, _ = fmt.Fprintln(env.Stdout)
	_, _ = fmt.Fprintln(env.Stdout, "This feature requires a running server with loaded models.")
	_, _ = fmt.Fprintln(env.Stdout)
	_, _ = fmt.Fprintln(env.Stdout, "Quick start:")
	_, _ = fmt.Fprintln(env.Stdout, "  1. Start server:  offgrid serve")
	_, _ = fmt.Fprintln(env.Stdout, "  2. In new terminal, use the API:")
	_, _ = fmt.Fprintln(env.Stdout)
	_, _ = fmt.Fprintln(env.Stdout, "Example curl command:")
	_, _ = fmt.Fprintln(env.Stdout, `  curl http://localhost:11611/v1/chat/completions \`)
	_, _ = fmt.Fprintln(env.Stdout, `    -H "Content-Type: application/json" \`)
	_, _ = fmt.Fprintln(env.Stdout, `    -d '{"model":"auto","messages":[{"role":"user","content":"Hello!"}]}'`)
	_, _ = fmt.Fprintln(env.Stdout)
	_, _ = fmt.Fprintln(env.Stdout, "Or use the web UI at: http://localhost:11611/ui")
	_, _ = fmt.Fprintln(env.Stdout)
	return nil
}

func RunChatPlaceholder(ctx context.Context, args []string, env Environment) error {
	return chatPlaceholderCommand(ctx, args, env)
}
