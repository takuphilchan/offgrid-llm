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

- Native ABI dependency -> pinned packages, packaged Windows/macOS/Linux smoke
  tests; compilation alone is not interactive PTY qualification.
- Shell privileges -> isolate renderer/IPC and disclose host/folder; no agent
  authority inheritance or claims that the shell is sandboxed.
- Lingering children -> explicit lifecycle tests with spawned descendants,
  backpressure, shell crash, tab close, renderer reload and app shutdown.
- More UI -> advanced opt-in surface, no auto-start or first-use requirement.

## Migration Plan

Add desktop capability negotiation and preserve old clients with no terminal
capability. No saved sessions or permission records are migrated. Rollback closes
owned PTYs before replacing the desktop; installation remains separately governed.
