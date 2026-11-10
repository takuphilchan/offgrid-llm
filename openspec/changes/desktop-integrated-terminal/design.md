## Context

The Electron preload exposes typed workspace/computer operations, not shell I/O.
The native executor records bounded command receipts and is not an interactive
PTY. Existing shutdown supervision and origin validation must remain authoritative.

## Goals / Non-Goals

Provide a real optional developer terminal while keeping ordinary Chat/Knowledge
usable without it. No terminal in remote web clients, automatic model access to
terminal text, agent-to-shell shortcut, implicit WSL or elevated execution.

## Decisions

- Use a maintained terminal renderer (xterm.js) and pinned native PTY host
  (node-pty/ConPTY on Windows, PTY on Unix), qualified against the Electron ABI.
  A textarea around process stdout is insufficient for interactive CLI programs.
- Put the terminal in a separate bundled trusted renderer surface, controlled by
  desktop main. The remotely served application can request opening the surface
  only through existing authenticated origin checks; it gets no raw shell API.
- Start closed. View/Terminal and a keyboard shortcut open a main-workspace view
  interchangeable with Chat, preserving the sidebar, shell/folder identity and tabs.
  The former bottom dock is replaced, not added as another layout setting.
  Choose the folder explicitly if no local folder
  is established; never infer a Windows folder from a service/container path.
- Launch a native non-admin shell with the user's environment under explicit
  user action. The working directory is not a sandbox. The OffGrid CLI action
  uses the matching installed binary and selected service without putting tokens
  into argv or model messages. Command input remains user-controlled.
- Keep terminal entry modes distinct. OffGrid CLI uses the native shell with the
  bundled CLI directory on PATH so `offgrid --help` and all other commands work
  at the prompt; it must not automatically enter `offgrid agent`. Agent launches
  the bundled CLI with the fixed `agent` argument so its normal profile chooser
  runs. Both modes use the same explicitly chosen folder, service endpoint,
  process supervision and trusted renderer boundary; no model-selected argv.
- Main owns each PTY, generation and renderer attachment. Bound buffered output
  and scrollback, apply backpressure, validate resize/input/frame lengths and
  preserve Ctrl+C semantics. Treat escape sequences/links/clipboard as untrusted;
  no silent clipboard writes or automatic URL execution.
- Switching views preserves terminal buffers and unfinished input. Fit only visible
  surfaces, suppress unchanged PTY resizes and restore focus after layout settles.
  Inactive terminals do not retain keyboard focus. The workspace can supply a
  bounded display rectangle but receives no terminal input/output capability.
- Agent CLI `/clear` clears the visible terminal and scrollback only; saved
  conversation context, work and authority remain unchanged. Plain/piped output
  receives an explanatory message instead of terminal escape sequences.
- Hiding preserves sessions; closing a live tab explains process termination.
  App quit closes owned process trees with bounded acknowledgement and a visible
  failure state if unconfirmed. Reopen/reload reattaches only to the same desktop
  owner; never replay shell input or recreate killed commands automatically.

## Risks / Trade-offs

### Agent exit and terminal presentation follow-up (2026-11-07)

The Agent entry invokes the pinned CLI through a literal, single-quoted command
in the existing Windows PowerShell launcher with `-NoExit`. Single quotes in the
executable path are escaped; user input is never interpolated. The shell and its
agent child remain inside the same owned process tree and selected folder.
`/exit` ends the CLI conversation and returns to the shell; no agent is restarted
and no previous input is replayed. Closing the tab still stops its owned tree.
One plus menu exposes CLI, Agent and PowerShell. Tabs include folder labels;
the footer describes an open session rather than claiming a command is running.
Output uses the available pane width with bounded edge padding.

- Native ABI dependency -> pinned packages, packaged Windows/macOS/Linux smoke
  tests; compilation alone is not interactive PTY qualification.
- Shell privileges -> isolate renderer/IPC and disclose host/folder; no agent
  authority inheritance or claims that the shell is sandboxed.
- Lingering children -> explicit lifecycle tests with spawned descendants,
  backpressure, shell crash, tab close, renderer reload and app shutdown.
- More UI -> advanced opt-in surface, no auto-start or first-use requirement.

## Migration Plan

### Display preferences (approved CLI/terminal review, 2026-11-08)

Store only versioned, validated display preferences in the trusted terminal
renderer's local storage. Invalid, unknown or unavailable storage uses defaults.
Font size (11–24), line height (1–1.8), cursor shape/blink and scrollback
(1,000–20,000 lines) apply to existing and new tabs without restarting shells.
Reducing scrollback may discard older displayed output and must be explained.
Reduced motion disables blinking even when the stored preference enables it.
Define all sixteen ANSI colors with measurable contrast against each background;
forced-color mode uses system foreground/background rather than the palette.
The settings dialog owns focus while open, restores focus when closed and never
passes form keystrokes to the PTY. Palette/preference validation, browser behavior
and isolated PTY evidence remain distinct from manual accessibility qualification.

Add desktop capability negotiation and preserve old clients with no terminal
capability. No saved sessions or permission records are migrated. Rollback closes
owned PTYs before replacing the desktop; installation remains separately governed.

### Command context contract (approved review implementation, 2026-11-08)

The existing task 6.2 uses a versioned PowerShell OSC 633 envelope for command
start/end and current directory. Fixed first-party initialization is confined to
the new NoProfile shell. It never edits profiles or execution policy. These
markers can be forged by terminal programs: label their status shell-reported,
never consume them as agent receipts, approvals or recovery evidence. Incomplete
input and missing end markers do not become successful commands. Other shells
remain explicitly without command integration.

Keep at most 100 command records per tab and 32,768 characters of displayed output
per completed command, captured before the next prompt changes its last line.
Marks allow navigation while in scrollback; trimmed start markers make output
unavailable. Alternate-screen markers cannot define normal-buffer commands.
Records remain in terminal memory only and disappear when the tab closes.

Commands opens an editable bounded preview of one command or a manual selection.
The user supplies a question and removes private content before continuing.
Authenticated terminal IPC checks exact frame, renderer generation, session
ownership and payload bounds. Delivery requires a current workspace acknowledgment;
unavailable/reloaded workspaces and overlapping previews retain the terminal
preview for retry. The actor/workspace-scoped web provider clears pending content
on identity changes and never restores it across reload.

Chat lets the user choose the current, another saved, or a new conversation and
edit the final context. Adding it appends to that conversation's draft; Send is
still required. Provenance and truncation are visible in the serialized context;
quoted output is untrusted user-supplied material, not an executable instruction.
This is explicit text sharing, not a service-owned file attachment or automatic
transcript upload. File-change review remains native-agent task 10.3.
