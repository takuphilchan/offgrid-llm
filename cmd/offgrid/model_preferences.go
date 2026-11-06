package main

import (
	"bufio"
	"fmt"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/internal/templates"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func handleAlias(args []string) {
	// Check for help flag
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		args = []string{} // Trigger help display
	}

	if len(args) == 0 {
		p := legacyPresenter()
		p.JSON = false
		p.Heading("OffGrid / Model aliases")
		p.ModernSection("Shortcuts", [][]string{
			{"offgrid alias list", "Inspect built-in and custom aliases"},
			{"offgrid alias set NAME MODEL", "Give a model a shortcut name"},
			{"offgrid alias remove NAME", "Remove a custom shortcut"},
		})
		p.Note("Use an exact installed model ID from offgrid list. Run an alias with offgrid run NAME.")
		return
	}

	configDir := "/var/lib/offgrid"
	if home, err := os.UserHomeDir(); err == nil {
		configDir = filepath.Join(home, ".offgrid")
	}

	am := models.NewAliasManager(configDir)

	switch args[0] {
	case "list", "ls":
		// Show built-in aliases first
		builtIn := models.ListBuiltInAliases()
		userAliases := am.ListAliases()

		fmt.Println()
		fmt.Printf("  %s%s Built-in Shortcuts%s\n", brandPrimary+colorBold, iconStar, colorReset)
		fmt.Printf("  %sUse these with 'offgrid run <alias>' - auto-downloads if needed%s\n", colorDim, colorReset)
		fmt.Println()

		// Group by category
		categories := map[string][]string{
			"Chat Models":      {"llama3", "llama3.2", "llama3.1", "qwen", "qwen2.5", "mistral", "phi", "phi3", "gemma", "gemma2"},
			"Small/Fast":       {"tiny", "smol", "llama3.2-1b", "qwen2.5-3b"},
			"Coding":           {"qwen2.5-coder", "codellama", "deepseek-coder", "starcoder"},
			"Vision":           {"llava", "moondream"},
			"Embeddings (RAG)": {"embed", "nomic"},
		}

		p := legacyPresenter()
		p.JSON = false
		for _, category := range []string{"Chat Models", "Small/Fast", "Coding", "Vision", "Embeddings (RAG)"} {
			aliases := categories[category]
			rows := [][]string{}
			for _, alias := range aliases {
				if info, ok := builtIn[alias]; ok {
					rows = append(rows, []string{alias, fmt.Sprintf("%s | estimated %d GB RAM", info.Pattern, info.RAM)})
				}
			}
			p.ModernSection(category, rows)
		}

		// Show user-defined aliases
		if len(userAliases) > 0 {
			fmt.Printf("  %s%s Custom Aliases%s\n", brandSecondary+colorBold, iconDiamond, colorReset)
			fmt.Println()
			aliases := make([]string, 0, len(userAliases))
			for alias := range userAliases {
				aliases = append(aliases, alias)
			}
			sort.Strings(aliases)
			for _, alias := range aliases {
				p.Detail(alias, userAliases[alias])
			}
			fmt.Println()
		}

		fmt.Printf("  %sCreate custom alias:%s offgrid alias set <name> <model-file>\n", colorDim, colorReset)
		fmt.Println()

	case "set", "create", "add":
		if len(args) < 3 {
			printError("Usage: offgrid alias set <alias> <model>")
			return
		}

		alias := args[1]
		modelID := args[2]

		if err := am.SetAlias(alias, modelID); err != nil {
			printError(fmt.Sprintf("Failed to set alias: %v", err))
			return
		}

		printSuccess(fmt.Sprintf("Alias '%s' created for '%s'", alias, modelID))

	case "remove", "rm", "delete":
		if len(args) < 2 {
			printError("Usage: offgrid alias remove <alias>")
			return
		}

		alias := args[1]
		if err := am.RemoveAlias(alias); err != nil {
			printError(fmt.Sprintf("Failed to remove alias: %v", err))
			return
		}

		printSuccess(fmt.Sprintf("Alias '%s' removed", alias))

	default:
		printError(fmt.Sprintf("Unknown alias command: %s", args[0]))
	}
}

// handleFavorite manages favorite models
func handleFavorite(args []string) {
	// Check for help flag
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		args = []string{} // Trigger help display
	}

	if len(args) == 0 {
		p := legacyPresenter()
		p.JSON = false
		p.Heading("OffGrid / Favorites")
		p.ModernSection("Bookmarks", [][]string{
			{"offgrid favorite list", "List saved models"},
			{"offgrid favorite add MODEL", "Bookmark a model ID"},
			{"offgrid favorite remove MODEL", "Remove a bookmark"},
		})
		p.Note("Favorites do not install models or change the active model.")
		return
	}

	configDir := "/var/lib/offgrid"
	if home, err := os.UserHomeDir(); err == nil {
		configDir = filepath.Join(home, ".offgrid")
	}

	am := models.NewAliasManager(configDir)

	switch args[0] {
	case "list", "ls":
		favorites := am.ListFavorites()
		if len(favorites) == 0 {
			fmt.Println()
			fmt.Printf("%sℹ No favorite models%s\n", colorDim, colorReset)
			fmt.Println()
			return
		}

		fmt.Println()
		fmt.Printf("%sFavorites%s\n", brandPrimary+colorBold, colorReset)
		for _, modelID := range favorites {
			fmt.Printf("%s★%s %s\n", brandSuccess, colorReset, modelID)
		}
		fmt.Println()

	case "add", "set":
		if len(args) < 2 {
			printError("Usage: offgrid favorite add <model>")
			return
		}

		modelID := args[1]
		if err := am.SetFavorite(modelID, true); err != nil {
			printError(fmt.Sprintf("Failed to add favorite: %v", err))
			return
		}

		printSuccess(fmt.Sprintf("'%s' added to favorites", modelID))

	case "remove", "rm", "delete":
		if len(args) < 2 {
			printError("Usage: offgrid favorite remove <model>")
			return
		}

		modelID := args[1]
		if err := am.SetFavorite(modelID, false); err != nil {
			printError(fmt.Sprintf("Failed to remove favorite: %v", err))
			return
		}

		printSuccess(fmt.Sprintf("'%s' removed from favorites", modelID))

	default:
		printError(fmt.Sprintf("Unknown favorite command: %s", args[0]))
	}
}

// handleTemplate manages prompt templates
func handleTemplate(args []string) {
	// Check for help flag
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		args = []string{} // Trigger help display
	}

	if len(args) == 0 {
		p := legacyPresenter()
		p.JSON = false
		p.Heading("OffGrid / Prompt templates")
		p.ModernSection("Reusable prompts", [][]string{
			{"offgrid template list", "List templates"},
			{"offgrid template show NAME", "Inspect text and variables"},
			{"offgrid template apply NAME", "Fill in variables interactively"},
		})
		p.Note("Prompt templates are separate from agent profiles and reviewed skills.")
		return
	}

	switch args[0] {
	case "list", "ls":
		fmt.Println()
		fmt.Printf("%sTemplates%s\n", brandPrimary+colorBold, colorReset)
		templateList := templates.ListTemplates()
		rows := [][]string{}
		for _, name := range templateList {
			tpl, _ := templates.GetTemplate(name)
			rows = append(rows, []string{name, tpl.Description})
		}
		legacyPresenter().ModernSection("Available prompts", rows)
		fmt.Println()

	case "show", "info":
		if len(args) < 2 {
			printError("Usage: offgrid template show <name>")
			return
		}

		tpl, err := templates.GetTemplate(args[1])
		if err != nil {
			printError(fmt.Sprintf("Template not found: %s", args[1]))
			return
		}

		fmt.Println()
		printBox(tpl.Name, fmt.Sprintf("%s\n\n%sVariables:%s %s",
			tpl.Description,
			colorBold, colorReset,
			strings.Join(tpl.Variables, ", ")))
		fmt.Println()
		fmt.Println(colorDim + "Template:" + colorReset)
		fmt.Println(tpl.Template)
		fmt.Println()

		if len(tpl.Examples) > 0 {
			fmt.Println(colorDim + "Examples:" + colorReset)
			for key, value := range tpl.Examples {
				printItem(key, value)
			}
			fmt.Println()
		}

	case "apply", "use":
		if len(args) < 2 {
			printError("Usage: offgrid template apply <name>")
			return
		}

		tpl, err := templates.GetTemplate(args[1])
		if err != nil {
			printError(fmt.Sprintf("Template not found: %s", args[1]))
			return
		}

		fmt.Println()
		fmt.Printf("%sTemplate:%s %s - %s\n\n", colorBold, colorReset, tpl.Name, tpl.Description)

		// Collect variables interactively
		variables := make(map[string]string)
		scanner := bufio.NewScanner(os.Stdin)

		for _, varName := range tpl.Variables {
			fmt.Printf("%s%s:%s ", brandPrimary, varName, colorReset)
			if example, ok := tpl.Examples[varName]; ok {
				fmt.Printf("%s(%s)%s\n> ", brandMuted, example, colorReset)
			} else {
				fmt.Print("\n> ")
			}

			scanner.Scan()
			value := scanner.Text()
			if value != "" {
				variables[varName] = value
			}
		}

		prompt, err := tpl.Apply(variables)
		if err != nil {
			printError(fmt.Sprintf("Failed to apply template: %v", err))
			return
		}

		fmt.Println()
		fmt.Printf("%sGenerated Prompt%s\n", brandPrimary+colorBold, colorReset)
		fmt.Println("")
		fmt.Println(prompt)
		fmt.Println()
		fmt.Printf("%s%s\n", brandPrimary, colorReset)
		fmt.Println()

	default:
		printError(fmt.Sprintf("Unknown template command: %s", args[0]))
	}
}

// handleBatch processes requests in batch mode
