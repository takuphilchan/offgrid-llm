package cli

import "fmt"

func agentWorkspaceHelp(env Environment) {
	p := Presenter{Stdout: env.Stdout, Stderr: env.Stderr, Style: env.Style}
	p.SectionHeader("Workspace access")
	fmt.Fprintln(env.Stdout, `  List approved hosts
    offgrid agent workspace list

  Start a local host (keep this terminal open)
    offgrid agent workspace start FOLDER --access MODE
    MODE: read, edit, or trusted (commands are not sandboxed)

  Stop an owned host
    offgrid agent workspace stop SESSION

  Inspect or forget remembered folder trust
    offgrid agent workspace trust list
    offgrid agent workspace trust revoke ID

  Optional isolated Linux host: add these to workspace start
    --backend isolated_linux --image sha256:DIGEST
    --access trusted --network none
    Network options: none or bridge. Local consent still applies.

  Forgetting trust affects future approval, not an active host.
  Stop the host to revoke current access. Earlier changes are not undone.`)
}

func legacyAgentHelp(env Environment) {
	p := Presenter{Stdout: env.Stdout, Stderr: env.Stderr, Style: env.Style}
	p.SectionHeader("Agent workspace")
	fmt.Fprintln(env.Stdout, `  Start a conversation
    offgrid agent --model MODEL

  Run one request
    offgrid agent run "your task" --model MODEL

  Continue a conversation
    offgrid agent chat --resume NAME --profile ID

  Inspect or stop saved work
    offgrid agent status RUN_ID
    offgrid agent cancel RUN_ID

  Configure your agent
    offgrid agent profiles help
    offgrid agent tools
    offgrid agent workspace help

  Common run options
    --model MODEL              Select an installed model
    --workspace-folder FOLDER  Offer a local folder when needed
    --wait                     Follow a piped/nonterminal run
    --detach                   Submit in the background

  Conversation needs no folder approval. Local work asks when needed.
  Ctrl+C stops the current run. Stopping does not undo earlier actions.

  Full reference: offgrid agent help all`)
}

func legacyAgentHelpAll(env Environment) {
	p := Presenter{Stdout: env.Stdout, Stderr: env.Stderr, Style: env.Style}
	groups := []struct {
		title string
		lines []string
	}{
		{"Conversation and tasks", []string{"agent [chat] --model MODEL", "agent chat --profile ID [--profile-revision N]", "agent chat --resume NAME --profile ID", "agent run PROMPT --model MODEL", "agent run PROMPT --profile ID --profile-revision N", "agent list", "session chat NAME --profile ID --profile-revision N"}},
		{"Inspect, pause and stop", []string{"agent status RUN_ID", "agent pause RUN_ID", "agent cancel RUN_ID", "agent resume RUN_ID", "agent steer RUN_ID REQUEST_ID TEXT"}},
		{"Approval and recovery", []string{"agent approve RUN_ID APPROVAL_ID", "agent deny RUN_ID APPROVAL_ID", "agent takeover RUN_ID", "agent reconnect RUN_ID", "agent recover-execution RUN_ID", "agent reconcile RUN_ID CALL_ID RESULT"}},
		{"Evidence and artifacts", []string{"agent export RUN_ID", "agent artifact RUN_ID SHA256 --output NEW_FILE"}},
		{"Profiles, tools and automation", []string{"agent profiles help", "agent skills help", "agent memory help", "agent schedules help", "agent tools", "agent mcp", "agent templates"}},
		{"Local workspace", []string{"agent workspace list", "agent workspace start FOLDER --access MODE", "agent workspace stop SESSION", "agent workspace trust list", "agent workspace trust revoke ID", "agent workspace help"}},
	}
	for _, group := range groups {
		p.SectionHeader(group.title)
		for _, line := range group.lines {
			fmt.Fprintln(env.Stdout, "  offgrid "+line)
		}
	}
	p.SectionHeader("Run options")
	for _, entry := range [][2]string{
		{"--model MODEL", "Select an installed model."},
		{"--response-profile interactive|extended", "Interactive uses the Chat allocation; extended requests the model limit."},
		{"--profile ID --profile-revision N", "Use a reviewed profile revision."},
		{"--workspace SESSION", "Use an already approved execution session."},
		{"--workspace-folder FOLDER", "Offer this local folder when work requires access."},
		{"--workspace-access read|edit|trusted", "Limit folder authority. Trusted commands are not sandboxed."},
		{"--wait / --detach", "Follow a run / submit in the background."},
		{"--style react|cot|plan", "Select instructions for a one-shot run."},
		{"--max-steps 1-50", "Limit model decisions for a one-shot run."},
		{"--request-id ID", "Recover a submission with the same request identity."},
	} {
		fmt.Fprintf(env.Stdout, "  %s\n    %s\n\n", entry[0], entry[1])
	}
	fmt.Fprintln(env.Stdout, `  JSON/piped callers submit asynchronously; use --wait to follow.
  Detached local work needs an independently approved workspace session.
  A folder grant never grants desktop control. y is session-only;
  remember saves only the exact folder/account/permission scope.
  Recovery inspects saved evidence; it must not replay uncertain actions.
  Templates are listed for review; task commands do not apply --template.`)
}
