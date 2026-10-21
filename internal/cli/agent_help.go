package cli

import "fmt"

func legacyAgentHelp(env Environment) {
	p := Presenter{Stdout: env.Stdout, Stderr: env.Stderr, Style: env.Style}
	p.SectionHeader("Agent commands")
	for _, line := range []string{
		"  offgrid agent status RUN_ID                 Inspect a saved run",
		"  offgrid agent approve RUN_ID APPROVAL_ID     Approve one pending call",
		"  offgrid agent deny RUN_ID APPROVAL_ID        Deny a pending call",
		"  offgrid agent cancel RUN_ID                 Stop work (does not undo tools)",
		"  offgrid agent resume RUN_ID                 Resume a safe checkpoint",
		"  offgrid agent pause RUN_ID                  Pause new scheduling",
		"  offgrid agent takeover RUN_ID               Revoke this task's computer session",
		"  offgrid agent reconnect RUN_ID              Request fresh local consent",
		"  offgrid agent steer RUN_ID REQUEST_ID TEXT  Queue a correction at a safe boundary",
		"  offgrid agent export RUN_ID                 Export evidence as JSON",
		"  offgrid agent artifact RUN_ID SHA256 --output NEW_FILE",
		"  offgrid agent reconcile RUN_ID CALL_ID RESULT  Record a human-observed outcome",
		"  offgrid agent profiles help                 Manage reusable profile revisions",
		"  offgrid session chat NAME --profile ID --profile-revision N",
	} {
		fmt.Fprintln(env.Stdout, line)
	}
	s := env.Style
	fmt.Fprintf(env.Stdout, "  %soffgrid agent [chat]%s           Start interactive agent session (default)\n", s.Primary, s.Reset)
	fmt.Fprintf(env.Stdout, "  %soffgrid agent run <prompt>%s     Run a single agent task\n", s.Primary, s.Reset)
	fmt.Fprintf(env.Stdout, "  %soffgrid agent templates%s        List pre-built agent personas\n", s.Primary, s.Reset)
	fmt.Fprintf(env.Stdout, "  %soffgrid agent tools%s            List available tools\n", s.Primary, s.Reset)
	fmt.Fprintf(env.Stdout, "  %soffgrid agent mcp%s              Manage MCP servers (add/list/remove)\n", s.Primary, s.Reset)
	fmt.Fprintf(env.Stdout, "  %soffgrid agent list%s             List active agents\n", s.Primary, s.Reset)
	fmt.Fprintln(env.Stdout)
	fmt.Fprintf(env.Stdout, "%sOptions:%s\n", s.Bold, s.Reset)
	fmt.Fprintf(env.Stdout, "  %s--model <name>%s                 Specify model to use\n", s.Primary, s.Reset)
	fmt.Fprintln(env.Stdout, "  --profile ID --profile-revision N           Use a reviewed profile for agent run")
	fmt.Fprintf(env.Stdout, "  %s--style react|cot|plan%s         Instruction style for agent run\n", s.Primary, s.Reset)
	fmt.Fprintf(env.Stdout, "  %s--max-steps <1-50>%s             Bound agent run model iterations\n", s.Primary, s.Reset)
	fmt.Fprintln(env.Stdout)
	fmt.Fprintln(env.Stdout, "  Review available personas with 'offgrid agent templates'; task commands do not apply --template.")
	fmt.Fprintln(env.Stdout)
}
