# CLI experience architecture

The OffGrid CLI is a first-class product surface for Linux, macOS, Windows,
containers, SSH sessions, and automation. It should feel like the same product
as the web and Electron workspace without assuming a particular terminal
background or color profile.

## Visual contract

Terminal presentation is owned by `cmd/offgrid/terminal_style.go`. The default
language is monochrome:

- terminal foreground and background colors remain authoritative;
- weight, spacing, labels, and symbols communicate hierarchy;
- interactive Bubble Tea views use adaptive light and dark neutral colors;
- success, warning, and error states always include text or a symbol and never
  depend on color alone;
- Unicode symbols are enabled only when an interactive UTF-8 terminal is
  detected, with an ASCII fallback everywhere else.

Do not add feature-specific RGB palettes or raw color escape sequences. Reuse
the semantic styles and output helpers. Escape sequences that control cursor
position or clear an interactive line are acceptable when guarded by an
interactive terminal path.

## Interaction contract

Interactive chat uses Bubble Tea and supports streaming, input history,
resize-aware layout, slash commands, knowledge status, and clear exit
instructions. The agent workspace presents each prompt as a governed, durable
task rather than implying conversational memory that the agent API does not
provide.

Long operations must show a bounded status or progress indicator and conclude
with an explicit result. Errors should state what failed and the next useful
action. Avoid clearing the terminal on startup because it destroys user
context and command history.

## Terminal and automation modes

The CLI supports these stable controls:

| Control | Behavior |
| --- | --- |
| `--json` | Machine-readable output; also disables decorative styling |
| `--no-color` | Removes terminal color and emphasis escape sequences |
| `NO_COLOR=1` | Standard environment equivalent of `--no-color` |
| `FORCE_COLOR=1` | Enables styling when output would otherwise appear non-interactive |
| `OFFGRID_UNICODE=0` | Forces portable ASCII symbols |
| `OFFGRID_UNICODE=1` | Forces Unicode symbols |
| `OFFGRID_TUI=0` or `OFFGRID_PLAIN=1` | Uses the line-oriented chat interface |

Piped and redirected output must never include animated progress, terminal
color escapes, or terminal-dependent symbols by default. JSON output must not
be mixed with human-readable banners.

## Quality gates

Run the CLI and shared-runtime checks after presentation changes:

```bash
go test ./cmd/offgrid ./internal/server
go test ./...
go build -trimpath ./cmd/offgrid
NO_COLOR=1 OFFGRID_UNICODE=0 offgrid --help
```

Tests in `cmd/offgrid/terminal_style_test.go` protect environment precedence
and Unicode-safe text handling. Add focused tests whenever a terminal
capability or automation contract changes.
