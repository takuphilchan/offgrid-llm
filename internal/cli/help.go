package cli

import "fmt"

// Help puts the first useful local workflow ahead of the full command catalog.
// Keep HelpAll available for scripts and experienced users looking for a family.
func (p Presenter) Help(version string) {
	commands := p
	commands.Style.Bold = ""
	fmt.Fprintln(p.Stdout)
	fmt.Fprintf(p.Stdout, "  %sOffGrid%s %s%s%s\n", p.Style.Primary+p.Style.Bold, p.Style.Reset, p.Style.Muted, version, p.Style.Reset)
	fmt.Fprintf(p.Stdout, "  %sRun local models. Downloads and connected tools may use the network.%s\n", p.Style.Muted, p.Style.Reset)
	fmt.Fprintln(p.Stdout)
	fmt.Fprintf(p.Stdout, "  %sStart here%s\n", p.Style.Bold, p.Style.Reset)
	commands.rowsWidth([]presentationRow{{"offgrid agent", "Start a conversation"}, {"offgrid model list", "See models already installed"}, {"offgrid doctor", "Check service and runtime readiness"}}, len("offgrid model catalog"))
	fmt.Fprintln(p.Stdout)
	fmt.Fprintf(p.Stdout, "  %sWhen you need more%s\n", p.Style.Bold, p.Style.Reset)
	commands.rowsWidth([]presentationRow{{"offgrid model catalog", "Review models available to install"}, {"offgrid kb help", "Work with local documents"}, {"offgrid agent help", "Agent tools and saved work"}, {"offgrid help all", "Every command family"}}, len("offgrid model catalog"))
	fmt.Fprintln(p.Stdout)
	fmt.Fprintf(p.Stdout, "  %sUsage%s  offgrid %s<command>%s %s[options]%s\n", p.Style.Bold, p.Style.Reset, p.Style.Primary, p.Style.Reset, p.Style.Muted, p.Style.Reset)
	fmt.Fprintf(p.Stdout, "  %sUse --json for automation or --no-color for plain output.%s\n", p.Style.Muted, p.Style.Reset)
	fmt.Fprintln(p.Stdout)
}

func (p Presenter) HelpAll(version string) {
	fmt.Fprintln(p.Stdout)
	fmt.Fprintf(p.Stdout, "  %sOffGrid%s %s%s%s\n", p.Style.Primary+p.Style.Bold, p.Style.Reset, p.Style.Muted, version, p.Style.Reset)
	fmt.Fprintf(p.Stdout, "  %sLocal inference; downloads and connected tools may use the network%s\n", p.Style.Muted, p.Style.Reset)
	fmt.Fprintln(p.Stdout)
	fmt.Fprintf(p.Stdout, "  %sUsage%s  offgrid %s<command>%s %s[options]%s\n", p.Style.Bold, p.Style.Reset, p.Style.Primary, p.Style.Reset, p.Style.Muted, p.Style.Reset)
	fmt.Fprintf(p.Stdout, "  %sGlobal%s --json for automation, --no-color for plain output\n", p.Style.Muted, p.Style.Reset)
	fmt.Fprintln(p.Stdout)

	// Define structure for commands to ensure global alignment
	type cmdEntry struct {
		name string
		desc string
	}

	type section struct {
		title string
		cmds  []cmdEntry
	}

	sections := []section{
		{
			title: "Model Management",
			cmds: []cmdEntry{
				{"model <cmd>", "Discover, review, install, and verify model packages"},
				{"recommend", "Get model recommendations for your system"},
				{"list", "List installed models"},
				{"search <query>", "Search HuggingFace"},
				{"download <id>", "Download from catalog"},
				{"download-hf <id>", "Download from HuggingFace"},
				{"import <path>", "Import from USB/SD card"},
				{"export <id> <dst>", "Export to USB/SD card"},
				{"remove <id>", "Remove installed model"},
				{"verify <id>", "Verify model integrity"},
				{"alias <cmd>", "Manage model aliases"},
				{"favorite <cmd>", "Manage favorite models"},
				{"quantization", "Explain model quantization choices"},
				{"quantize <args>", "Convert model quantization"},
			},
		},
		{
			title: "Inference & Chat",
			cmds: []cmdEntry{
				{"serve", "Start API server (default)"},
				{"run <model>", "Interactive chat (--image for VLMs)"},
				{"chat <args>", "Chat with a selected model"},
				{"session <cmd>", "Manage chat sessions"},
				{"export-session <args>", "Export a saved conversation"},
				{"kb <cmd>", "Manage knowledge base (RAG)"},
				{"workspace <cmd>", "Offline backup, verification, and safe restore"},
				{"template <cmd>", "Manage prompt templates"},
				{"batch <file>", "Batch process prompts"},
			},
		},
		{
			title: "System",
			cmds: []cmdEntry{
				{"init", "First-time setup wizard"},
				{"doctor", "Run system diagnostics"},
				{"info", "Local configuration and model files"},
				{"logs", "View server logs (-f to follow)"},
				{"config <action>", "Manage configuration"},
				{"peers", "P2P network status & model sharing"},
				{"appliance <cmd>", "Plan offline community devices"},
				{"benchmark <id>", "Performance testing"},
				{"compare <args>", "Compare model performance"},
				{"test <args>", "Test inference"},
				{"metrics", "Inspect service metrics"},
				{"version", "Show the installed version"},
				{"help [all]", "First steps or the complete command catalog"},
				{"completions <shell>", "Generate shell completions"},
				{"shell-completion", "Manage shell completion setup"},
			},
		},
		{
			title: "Advanced",
			cmds: []cmdEntry{
				{"lora <cmd>", "LoRA adapter management"},
				{"agent [cmd]", "Conversation, tools, and saved agent work"},
				{"hermes [cmd]", "Install and run Hermes on OffGrid"},
				{"openclaw [cmd]", "Install and run OpenClaw on OffGrid"},
				{"integration <cmd>", "Connect Hermes, OpenClaw, and other agents"},
				{"computer <cmd>", "Inspect and manage local computer access"},
				{"audio <cmd>", "Speech-to-text & TTS"},
				{"users <cmd>", "Multi-user management"},
				{"audit <cmd>", "Security audit logs (export CSV/JSON)"},
			},
		},
	}

	for _, s := range sections {
		fmt.Fprintf(p.Stdout, "  %s%s%s\n", p.Style.Primary, s.title, p.Style.Reset)
		rows := make([]presentationRow, 0, len(s.cmds))
		for _, c := range s.cmds {
			rows = append(rows, presentationRow{c.name, c.desc})
		}
		p.rows(rows)
		fmt.Fprintln(p.Stdout)
	}

	// Use installed-model commands instead of advertising fixed aliases or an
	// implicit download that may not be available for the selected runtime.
	fmt.Fprintf(p.Stdout, "  %sQuick Start%s\n", p.Style.Primary, p.Style.Reset)
	fmt.Fprintf(p.Stdout, "    %s$%s offgrid model list               %s# See installed models%s\n", p.Style.Muted, p.Style.Reset, p.Style.Muted, p.Style.Reset)
	fmt.Fprintf(p.Stdout, "    %s$%s offgrid agent                    %s# Start a conversation%s\n", p.Style.Muted, p.Style.Reset, p.Style.Muted, p.Style.Reset)
	fmt.Fprintf(p.Stdout, "    %s$%s offgrid model catalog           %s# Review install choices%s\n", p.Style.Muted, p.Style.Reset, p.Style.Muted, p.Style.Reset)
	fmt.Fprintf(p.Stdout, "    %s$%s offgrid doctor                   %s# Check readiness%s\n", p.Style.Muted, p.Style.Reset, p.Style.Muted, p.Style.Reset)
	fmt.Fprintln(p.Stdout)

	// Footer with helpful info
	fmt.Fprintf(p.Stdout, "  %sRun %soffgrid <command> --help%s %sfor detailed usage%s\n", p.Style.Muted, p.Style.Reset+p.Style.Bold, p.Style.Reset, p.Style.Muted, p.Style.Reset)
	fmt.Fprintf(p.Stdout, "  %sDocs: %shttps://github.com/takuphilchan/offgrid-llm%s\n", p.Style.Muted, p.Style.Primary, p.Style.Reset)
	fmt.Fprintln(p.Stdout)
}
