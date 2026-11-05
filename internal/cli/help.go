package cli

import (
	"fmt"
	"strings"
)

// Help puts the first useful local workflow ahead of the full command catalog.
// Keep HelpAll available for scripts and experienced users looking for a family.
func (p Presenter) Help(version string) {
	fmt.Fprintln(p.Stdout)
	fmt.Fprintf(p.Stdout, "  %sOffGrid%s %s%s%s\n", p.Style.Primary+p.Style.Bold, p.Style.Reset, p.Style.Muted, version, p.Style.Reset)
	fmt.Fprintf(p.Stdout, "  %sRun local models. Downloads and connected tools may use the network.%s\n", p.Style.Muted, p.Style.Reset)
	fmt.Fprintln(p.Stdout)
	fmt.Fprintf(p.Stdout, "  %sStart here%s\n", p.Style.Primary, p.Style.Reset)
	fmt.Fprintln(p.Stdout, "    offgrid model list          See models already installed")
	fmt.Fprintln(p.Stdout, "    offgrid agent               Start a conversation")
	fmt.Fprintln(p.Stdout, "    offgrid doctor              Check service and runtime readiness")
	fmt.Fprintln(p.Stdout)
	fmt.Fprintf(p.Stdout, "  %sWhen you need more%s\n", p.Style.Primary, p.Style.Reset)
	fmt.Fprintln(p.Stdout, "    offgrid model discover      Review models available to install")
	fmt.Fprintln(p.Stdout, "    offgrid kb help             Work with local documents")
	fmt.Fprintln(p.Stdout, "    offgrid agent help          Agent tools and saved work")
	fmt.Fprintln(p.Stdout, "    offgrid help all            Every command family")
	fmt.Fprintln(p.Stdout)
	fmt.Fprintf(p.Stdout, "  %sUsage%s  offgrid %s<command>%s %s[options]%s\n", p.Style.Bold, p.Style.Reset, p.Style.Primary, p.Style.Reset, p.Style.Muted, p.Style.Reset)
	fmt.Fprintf(p.Stdout, "  %sUse --json for automation or --no-color for plain output.%s\n", p.Style.Muted, p.Style.Reset)
	fmt.Fprintln(p.Stdout)
}

func (p Presenter) HelpAll(version string) {
	fmt.Fprintln(p.Stdout)
	fmt.Fprintf(p.Stdout, "  %sOffGrid LLM%s %s%s%s\n", p.Style.Primary+p.Style.Bold, p.Style.Reset, p.Style.Muted, version, p.Style.Reset)
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
				{"info", "System information"},
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

	// Use fixed column width of 22 chars for command column
	const columnWidth = 22

	for _, s := range sections {
		fmt.Fprintf(p.Stdout, "  %s%s%s\n", p.Style.Primary, s.title, p.Style.Reset)
		for _, c := range s.cmds {
			// Calculate padding needed to reach fixed column width
			paddingNeeded := columnWidth - len(c.name)
			if paddingNeeded < 2 {
				paddingNeeded = 2
			}
			padding := strings.Repeat(" ", paddingNeeded)

			fmt.Fprintf(p.Stdout, "    %s%s%s%s%s%s\n",
				p.Style.Bold, c.name, p.Style.Reset,
				padding,
				p.Style.Muted, c.desc)
		}
		fmt.Fprintln(p.Stdout)
	}

	// Use installed-model commands instead of advertising fixed aliases or an
	// implicit download that may not be available for the selected runtime.
	fmt.Fprintf(p.Stdout, "  %sQuick Start%s\n", p.Style.Primary, p.Style.Reset)
	fmt.Fprintf(p.Stdout, "    %s$%s offgrid model list               %s# See installed models%s\n", p.Style.Muted, p.Style.Reset, p.Style.Muted, p.Style.Reset)
	fmt.Fprintf(p.Stdout, "    %s$%s offgrid agent                    %s# Start a conversation%s\n", p.Style.Muted, p.Style.Reset, p.Style.Muted, p.Style.Reset)
	fmt.Fprintf(p.Stdout, "    %s$%s offgrid model discover           %s# Review install choices%s\n", p.Style.Muted, p.Style.Reset, p.Style.Muted, p.Style.Reset)
	fmt.Fprintf(p.Stdout, "    %s$%s offgrid doctor                   %s# Check readiness%s\n", p.Style.Muted, p.Style.Reset, p.Style.Muted, p.Style.Reset)
	fmt.Fprintln(p.Stdout)

	// Footer with helpful info
	fmt.Fprintf(p.Stdout, "  %sRun %soffgrid <command> --help%s %sfor detailed usage%s\n", p.Style.Muted, p.Style.Reset+p.Style.Bold, p.Style.Reset, p.Style.Muted, p.Style.Reset)
	fmt.Fprintf(p.Stdout, "  %sDocs: %shttps://github.com/takuphilchan/offgrid-llm%s\n", p.Style.Muted, p.Style.Primary, p.Style.Reset)
	fmt.Fprintln(p.Stdout)
}
