package main

import (
	"strconv"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/models"
)

func parseModelSearchArgs(args []string) (models.SearchFilter, int, bool, error) {
	filter := models.SearchFilter{OnlyGGUF: true, ExcludeGated: true, ExcludePrivate: true, Limit: 20, SortBy: "downloads"}
	ram, files := 0, false
	words := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--files":
			files = true
		case "--all":
			filter.ExcludeGated = false
		case "--help", "-h":
		case "--author", "-a", "--quant", "-q", "--ram", "-r", "--sort", "-s", "--limit", "-l":
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				return filter, 0, false, usage("%s requires a value", arg)
			}
			i++
			value := args[i]
			switch arg {
			case "--author", "-a":
				filter.Author = value
			case "--quant", "-q":
				filter.Quantization = value
			case "--sort", "-s":
				switch value {
				case "downloads", "likes", "created", "modified", "relevance":
					filter.SortBy = value
				default:
					return filter, 0, false, usage("unsupported sort: %s", value)
				}
			default:
				n, err := strconv.Atoi(value)
				if err != nil || n < 1 {
					return filter, 0, false, usage("%s requires a positive integer", arg)
				}
				if arg == "--ram" || arg == "-r" {
					ram = n
				} else {
					if n > 50 {
						return filter, 0, false, usage("search limit cannot exceed 50")
					}
					filter.Limit = n
				}
			}
		default:
			if strings.HasPrefix(arg, "-") {
				return filter, 0, false, usage("unknown search option: %s", arg)
			}
			words = append(words, arg)
		}
	}
	filter.Query = strings.Join(words, " ")
	if len(filter.Query) > 200 {
		return filter, 0, false, usage("search query cannot exceed 200 characters")
	}
	return filter, ram, files, nil
}
