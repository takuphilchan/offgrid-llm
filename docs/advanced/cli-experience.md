# CLI experience architecture

The OffGrid CLI is a first-class product surface for Linux, macOS, Windows,
containers, SSH sessions, and automation. It should feel like the same product
as the web and Electron workspace without assuming a particular terminal
background or color profile.

## Visual contract

Terminal capability detection and the legacy style aliases remain in
`cmd/offgrid/terminal_style.go`. Migrated presentation and command results
live in `internal/cli`; `cmd/offgrid/terminal_output.go` and `progress.go`
adapt host-terminal behavior while older commands move.
The default
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
instructions. Native agent conversation turns link to canonical durable jobs,
with the service retaining conversation history rather than the CLI fabricating
memory by concatenating independent task results.

The reusable-profile path uses `session chat NAME --profile ID
--profile-revision N`: its turns are linked to the same saved conversations and
canonical jobs as React Chat. This is not a second terminal executor. The
default `offgrid agent`/`agent chat` now uses this same source implementation,
with profile selection, explicit model selection and `--resume NAME`. A selected
model must match the profile's pinned model; mismatches are rejected before
creating a conversation, not treated as a silent profile override. Existing
one-shot `agent run` remains separate and compatible. Profile commands require
reviewed revisions, explicit recovery and
service capability negotiation. Structured output retains pending access,
uncertainty, evidence and canonical identities without granting access or
replaying work. Interactive corrections use safe-boundary steering; `/exit`
only detaches observation, whereas `/stop` requests canonical cancellation.
Ctrl+C during a foreground interactive submission inspects its exact turn before
requesting Stop; it cannot cancel an unrelated turn or claim a lost acknowledgment
was confirmed. Interactive budgets come from profiles, not a hardcoded ten steps.

Long operations must show a bounded status or progress indicator and conclude
with an explicit result. Errors should state what failed and the next useful
action. Avoid clearing the terminal on startup because it destroys user
context and command history.

Foreground native runs use one compact OffGrid/model header, concise action
labels and an answer block separated from progress. Request IDs are retained in
submission errors and automation output; the successful terminal path ends with
one inspect command instead of repeated run/request/status headers. Elapsed time
in that footer measures stream observation, not a model throughput benchmark.
Committed file-write receipts show a verified destination on its own line, without
sentence punctuation. Assistant prose remains unchanged; the renderer cannot
infer that a write succeeded from the model's answer. Approval, uncertain-effect
and recovery instructions stay visible. JSON, detached submission and explicit
status output retain their existing contracts.

On an interactive terminal, a single-line working indicator shows elapsed time
while awaiting the service or model. It clears before other output and stops on
every terminal state or disconnected stream. `OFFGRID_ANIMATIONS=0` disables
motion; `OFFGRID_PLAIN=1`, `TERM=dumb`, and redirected streams do not animate.
`NO_COLOR` removes color without implying that progress should stop.

Terminal response previews append incrementally under an explicitly provisional
label. Repeated snapshots do not repeat text. A matching final answer appends only
its remaining suffix; a changed final answer, failed stream or new model turn
explicitly ends the draft. Only the service's completed state earns a completion
footer. Tool arguments and hidden reasoning are never streamed as answer text.
Redirected output and JSON retain final-result behavior without cursor controls.

Root `offgrid --help` and `offgrid help` show the first useful path: inspect
installed models, start a conversation, and check readiness. `offgrid help all`
keeps the complete command catalog. These help screens do not promise a fixed
model alias or an automatic download; local inference and optional network
operations are described separately.

`offgrid agent --help` prioritizes starting, continuing and inspecting work.
`offgrid agent help all` exposes the complete command reference grouped into
conversation, lifecycle, recovery, evidence, configuration and workspace sections.

On a matching service, new foreground `agent run` requests use the same
interactive context allocation as Chat (`OFFGRID_CHAT_CONTEXT`, bounded by the
model limit). Use `--response-profile extended` for larger tasks or
`--response-profile interactive` to select it explicitly. The choice is pinned
for that job. Older services and omitted API fields retain their prior behavior;
JSON/detached submissions are not automatically changed. This does not change
Hermes/OpenClaw allocations or the service maximum. A larger context can increase
memory use, loading time and CPU offload; a smaller allocation is not a guarantee
of faster or correct answers.

## Terminal and automation modes

### Persistent agent sessions

`offgrid agent` offers a searchable agent picker. Type a name, purpose, model or
ID, use Up/Down to move, and Enter to select. Escape returns without starting
work. Plain terminals accept a number, exact name or ID; partial text filters
the list, `/all` resets it, and invalid or ambiguous input can be corrected.
`--model MODEL` narrows selection to matching profiles rather than overriding
their saved models.

Use `/default` inside a session to explicitly remember its exact profile
revision for the connected service, workspace and user. `/default show` inspects
that preference and `/default clear` forgets it. Nothing is remembered merely
by selecting an agent. A changed revision, archived profile, different account
or workspace cannot silently reuse or upgrade a default. The preference stores
no credentials or permissions; local file and computer access remain separate.

`offgrid agent chat --resume NAME` uses the saved turn's profile revision.
Explicit conflicting profile/model flags are rejected. An empty or legacy
conversation without saved agent identity still needs `--profile ID` (and its
revision when known). `/session` retains a complete explicit resume command for
that case. `/agent` selects an agent for a **separate conversation**, closing
this invocation's owned folder lease before starting it. Existing work is saved
and access is not transferred. Active, uncertain or unreconciled work must first
be resolved; cancelling the picker leaves the current conversation open.

Start `offgrid agent` or `offgrid agent chat --profile PROFILE_ID` and enter
successive requests at the prompt. The session shows one welcome with the pinned
profile and model. `/session` exposes revision, budget and the exact resume
command; `/status` inspects the latest canonical job. Normal completed turns do
not repeat internal turn/run IDs. Failures and approvals remain visible.

In supported terminals, `/` opens command suggestions. Up/Down selects, Tab
completes, Enter runs the selected command and Escape dismisses suggestions.
The palette shows a bounded selection and its current position, with the common
conversation actions first. `/help` shows the short command set; `/help all`
shows recovery and configuration controls without crowding the prompt.
Outside that menu, input wraps across up to six visible rows. Enter sends;
Ctrl+J inserts a newline (Alt+Enter also works where the terminal forwards it).
Up/Down moves through wrapped or multiline input; for a single line it recalls
history. Ctrl+P/Ctrl+N always recall this invocation's last 50 inputs and restore
the unfinished draft when moving back down. Ctrl+R searches history; Enter uses
the match for further editing, while Escape restores the draft and caret.
History is not saved to a separate file. Cursor movement and deletion preserve
whole graphemes, including combining accents and emoji. Bracketed paste retains
line breaks and never submits by itself. Input is limited to 32,000 UTF-8 bytes;
an oversized paste is discarded in full and the previous draft is preserved.
The editor shares the existing bounded input reader with on-demand consent;
terminal mode is restored before approval handling and when the editor exits.

`/help` lists commands; `/skills`, `/agents` and `/tools` inspect this pinned
profile's configuration. They do not grant access, attach a skill, change the
profile or start specialist work. Tool readiness is still evaluated per task.
Use the existing profile/skill management commands to make explicit changes.

The terminal picker falls back to ordinary line input for redirected streams,
`TERM=dumb`, `OFFGRID_PLAIN=1`, `OFFGRID_TUI=0`, or windows below 32 columns or
10 rows. Slash commands still work when entered followed by Enter.

During an agent conversation's reply, supported terminals of at least 40 columns
and 16 rows keep an editor below the streaming output. Enter sends the text as a
correction to the current task at a safe boundary. `/queue MESSAGE` retains one
separate follow-up for review after the turn; it never sends it automatically.
Ctrl+C or `/stop` requests cancellation and reports the service's actual state.
An unfinished correction survives Stop. Once the turn ends, retained input is
shown for editing and requires another Enter to submit.

Only one component reads the keyboard. Before steering, the CLI checks the exact
saved task again. Tasks awaiting access, approvals or reconciliation cannot accept
corrections. A missing correction acknowledgment retains the draft and blocks
another attempt in that stream; inspect saved work before retrying. Complete
control receipts remain in scrollback. Plain, piped and small terminals retain
the existing streaming interface. This live editor applies to interactive agent
conversations, not detached jobs or every one-shot `agent run` command.

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

### Human-readable inventories and agent entry

`offgrid info` labels its output **Local configuration**: context and memory
values are configured limits, not measurements of the running service. Zero
threads displays as **Automatic**. Its model inventory comes from the local
folder; `offgrid list` explicitly identifies the service it queried. Neither
screen treats installed files as proof of runtime readiness; use `offgrid doctor`
for readiness diagnostics.

Inventories, root help, agent help and session headers share compact headings
and readable values. Rows use measured terminal columns (80 when unavailable),
with display-cell-aware alignment for Unicode. When identifiers and details
cannot fit together, details move below the identifier. Exact IDs remain
unabbreviated and copyable; very long IDs may soft-wrap in the terminal. Local
inventory summaries omit filename-derived quantization guesses; their existing
JSON projection is unchanged. No model identity or stored metadata is rewritten.

Profile selection separates the profile name, model and revision. Session
headers label the active profile and model; `/session` still provides the full
resume command. Short `agent --help` introduces in-session commands before the
complete reference. This presentation does not introduce a new agent loop,
automatic permissions, downloads or simulated progress.

Recovery guidance avoids assuming Linux or blaming a generic timeout on a
remote provider. It directs users to the configured endpoint, diagnostics and
saved work without recommending blind retries of uncertain effects.

The same human presentation covers discovery, downloads, diagnostics, Knowledge,
speech, integrations, configuration, users, audit, batch work, logs and model
preferences. Family help groups common actions before administration and recovery.
Advanced record/preview commands retain their structured output and exact field
names. This is deliberate: presentation never rewrites tool results or API data.

Interactive chat adapts its header and shortcuts to terminal width. Download
progress updates one line only in a real terminal; redirected output gets a
completion line without cursor-control bytes. Benchmark latency reports complete
response time; it does not fabricate first-token measurements.

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
