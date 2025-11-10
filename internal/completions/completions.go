package completions

import (
	"fmt"
	"strings"
)

// Generator generates shell completion scripts
type Generator struct {
	programName string
	commands    []Command
}

// Command represents a CLI command with subcommands and flags
type Command struct {
	Name        string
	Subcommands []string
	Flags       []string
	Description string
}

// NewGenerator creates a new completion generator
func NewGenerator(programName string) *Generator {
	return &Generator{
		programName: programName,
		commands:    []Command{},
	}
}

// AddCommand adds a command to the generator
func (g *Generator) AddCommand(cmd Command) {
	g.commands = append(g.commands, cmd)
}

// GenerateBash generates bash completion script
func (g *Generator) GenerateBash() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# bash completion for %s\n\n", g.programName))
	sb.WriteString(fmt.Sprintf("_%s_completions() {\n", g.programName))
	sb.WriteString("    local cur prev words cword\n")
	sb.WriteString("    _init_completion || return\n\n")

	// Main commands
	sb.WriteString("    local commands=\"")
	cmdNames := []string{}
	for _, cmd := range g.commands {
		cmdNames = append(cmdNames, cmd.Name)
	}
	sb.WriteString(strings.Join(cmdNames, " "))
	sb.WriteString("\"\n\n")

	// Handle subcommands
	sb.WriteString("    if [[ $cword -eq 1 ]]; then\n")
	sb.WriteString("        COMPREPLY=($(compgen -W \"$commands\" -- \"$cur\"))\n")
	sb.WriteString("        return\n")
	sb.WriteString("    fi\n\n")

	// Handle command-specific completions
	sb.WriteString("    case \"${words[1]}\" in\n")
	for _, cmd := range g.commands {
		if len(cmd.Subcommands) > 0 || len(cmd.Flags) > 0 {
			sb.WriteString(fmt.Sprintf("        %s)\n", cmd.Name))

			if len(cmd.Subcommands) > 0 {
				sb.WriteString("            local subcommands=\"")
				sb.WriteString(strings.Join(cmd.Subcommands, " "))
				sb.WriteString("\"\n")
			}

			if len(cmd.Flags) > 0 {
				sb.WriteString("            local flags=\"")
				sb.WriteString(strings.Join(cmd.Flags, " "))
				sb.WriteString("\"\n")
			}

			if len(cmd.Subcommands) > 0 && len(cmd.Flags) > 0 {
				sb.WriteString("            COMPREPLY=($(compgen -W \"$subcommands $flags\" -- \"$cur\"))\n")
			} else if len(cmd.Subcommands) > 0 {
				sb.WriteString("            COMPREPLY=($(compgen -W \"$subcommands\" -- \"$cur\"))\n")
			} else {
				sb.WriteString("            COMPREPLY=($(compgen -W \"$flags\" -- \"$cur\"))\n")
			}

			sb.WriteString("            ;;\n")
		}
	}
	sb.WriteString("    esac\n")
	sb.WriteString("}\n\n")

	sb.WriteString(fmt.Sprintf("complete -F _%s_completions %s\n", g.programName, g.programName))

	return sb.String()
}

// GenerateZsh generates zsh completion script
func (g *Generator) GenerateZsh() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("#compdef %s\n\n", g.programName))
	sb.WriteString(fmt.Sprintf("_%s() {\n", g.programName))
	sb.WriteString("    local line state\n\n")

	sb.WriteString("    _arguments -C \\\n")
	sb.WriteString("        '1: :->command' \\\n")
	sb.WriteString("        '*::arg:->args'\n\n")

	sb.WriteString("    case $state in\n")
	sb.WriteString("        command)\n")
	sb.WriteString("            local -a commands\n")
	sb.WriteString("            commands=(\n")

	for _, cmd := range g.commands {
		desc := cmd.Description
		if desc == "" {
			desc = cmd.Name
		}
		sb.WriteString(fmt.Sprintf("                '%s:%s'\n", cmd.Name, desc))
	}

	sb.WriteString("            )\n")
	sb.WriteString("            _describe 'command' commands\n")
	sb.WriteString("            ;;\n")
	sb.WriteString("        args)\n")
	sb.WriteString("            case $line[1] in\n")

	for _, cmd := range g.commands {
		if len(cmd.Subcommands) > 0 {
			sb.WriteString(fmt.Sprintf("                %s)\n", cmd.Name))
			sb.WriteString("                    local -a subcommands\n")
			sb.WriteString("                    subcommands=(\n")
			for _, sub := range cmd.Subcommands {
				sb.WriteString(fmt.Sprintf("                        '%s'\n", sub))
			}
			sb.WriteString("                    )\n")
			sb.WriteString("                    _describe 'subcommand' subcommands\n")
			sb.WriteString("                    ;;\n")
		}
	}

	sb.WriteString("            esac\n")
	sb.WriteString("            ;;\n")
	sb.WriteString("    esac\n")
	sb.WriteString("}\n\n")

	sb.WriteString(fmt.Sprintf("_%s\n", g.programName))

	return sb.String()
}

// GenerateFish generates fish completion script
func (g *Generator) GenerateFish() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# fish completion for %s\n\n", g.programName))

	// Main commands
	for _, cmd := range g.commands {
		desc := cmd.Description
		if desc == "" {
			desc = cmd.Name
		}
		sb.WriteString(fmt.Sprintf("complete -c %s -n \"__fish_use_subcommand\" -a %s -d '%s'\n",
			g.programName, cmd.Name, desc))
	}

	sb.WriteString("\n")

	// Subcommands
	for _, cmd := range g.commands {
		if len(cmd.Subcommands) > 0 {
			for _, sub := range cmd.Subcommands {
				sb.WriteString(fmt.Sprintf("complete -c %s -n \"__fish_seen_subcommand_from %s\" -a %s\n",
					g.programName, cmd.Name, sub))
			}
		}

		// Flags
		if len(cmd.Flags) > 0 {
			for _, flag := range cmd.Flags {
				sb.WriteString(fmt.Sprintf("complete -c %s -n \"__fish_seen_subcommand_from %s\" -a '%s'\n",
					g.programName, cmd.Name, flag))
			}
		}
	}

	return sb.String()
}
