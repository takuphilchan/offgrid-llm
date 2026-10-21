package cli

import (
	"fmt"
	"strings"
)

func (p Presenter) Help(version string) {
	fmt.Fprintln(p.Stdout)
	fmt.Fprintf(p.Stdout, "  %sOffGrid LLM%s %s%s%s\n", p.Style.Primary+p.Style.Bold, p.Style.Reset, p.Style.Muted, version, p.Style.Reset)
	fmt.Fprintf(p.Stdout, "  %sLocal AI that works offline%s\n", p.Style.Muted, p.Style.Reset)
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
				{"recommend", "Get model recommendations for your system"},
				{"list", "List installed models"},
				{"search <query>", "Search HuggingFace"},
				{"download <id>", "Download from catalog"},
				{"download-hf <id>", "Download from HuggingFace"},
				{"import <path>", "Import from USB/SD card"},
				{"export <id> <dst>", "Export to USB/SD card"},
				{"remove <id>", "Remove installed model"},
			},
		},
		{
			title: "Inference & Chat",
			cmds: []cmdEntry{
				{"serve", "Start API server (default)"},
				{"run <model>", "Interactive chat (--image for VLMs)"},
				{"session <cmd>", "Manage chat sessions"},
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
			},
		},
		{
			title: "Advanced",
			cmds: []cmdEntry{
				{"lora <cmd>", "LoRA adapter management"},
				{"agent <cmd>", "AI agent workflows (6 templates)"},
				{"hermes [cmd]", "Install and run Hermes on OffGrid"},
				{"openclaw [cmd]", "Install and run OpenClaw on OffGrid"},
				{"integrations <cmd>", "Connect Hermes, OpenClaw, and other agents"},
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

	// Quick start examples in a more compact format
	fmt.Fprintf(p.Stdout, "  %sQuick Start%s\n", p.Style.Primary, p.Style.Reset)
	fmt.Fprintf(p.Stdout, "    %s$%s offgrid run llama3               %s# Llama 3.2 3B (auto-downloads)%s\n", p.Style.Muted, p.Style.Reset, p.Style.Muted, p.Style.Reset)
	fmt.Fprintf(p.Stdout, "    %s$%s offgrid run qwen                 %s# Qwen 2.5 3B%s\n", p.Style.Muted, p.Style.Reset, p.Style.Muted, p.Style.Reset)
	fmt.Fprintf(p.Stdout, "    %s$%s offgrid run mistral              %s# Mistral 7B%s\n", p.Style.Muted, p.Style.Reset, p.Style.Muted, p.Style.Reset)
	fmt.Fprintf(p.Stdout, "    %s$%s offgrid alias list               %s# See all model shortcuts%s\n", p.Style.Muted, p.Style.Reset, p.Style.Muted, p.Style.Reset)
	fmt.Fprintf(p.Stdout, "    %s$%s offgrid appliance status         %s# Check hardware fit%s\n", p.Style.Muted, p.Style.Reset, p.Style.Muted, p.Style.Reset)
	fmt.Fprintln(p.Stdout)

	// Footer with helpful info
	fmt.Fprintf(p.Stdout, "  %sRun %soffgrid <command> --help%s %sfor detailed usage%s\n", p.Style.Muted, p.Style.Reset+p.Style.Bold, p.Style.Reset, p.Style.Muted, p.Style.Reset)
	fmt.Fprintf(p.Stdout, "  %sDocs: %shttps://github.com/takuphilchan/offgrid-llm%s\n", p.Style.Muted, p.Style.Primary, p.Style.Reset)
	fmt.Fprintln(p.Stdout)
}
