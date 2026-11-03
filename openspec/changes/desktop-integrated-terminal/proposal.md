## Why

Advanced desktop users need a real terminal without switching applications.
Current task process receipts are not an interactive terminal. Adding another
required setup surface would worsen the ordinary-user experience.

## What Changes

- Add an optional, initially closed main-workspace terminal view, interchangeable
  with Chat while preserving sidebar navigation, with native PTY,
  streaming output, resize, keyboard input, copy/search and explicit session tabs.
- Show local host, shell and current working folder; use native Windows shells
  without requiring WSL, and native shells on macOS/Linux.
- Resolve the matching bundled OffGrid CLI for an explicit launch action, sharing
  the configured service through existing authenticated contracts.
- Offer separate terminal entries: OffGrid CLI starts an interactive local shell
  with the matching `offgrid` command ready, while Agent starts the matching
  interactive agent CLI and its existing profile selection.
- Own terminal child processes through close/reload/quit and surface unconfirmed
  shutdown instead of abandoning processes. Hiding a panel does not kill a job.
- Use a trusted desktop-only IPC boundary and bounded output/backpressure;
  remote browser pages receive no local shell bridge. Terminal content is
  untrusted and not automatically submitted to models or exported in diagnostics.
- Preserve unfinished input/cursor position across tab changes and add agent CLI
  `/clear` for display-only cleanup without deleting conversation history.

## Capabilities

### New Capabilities
- `desktop-terminal`: optional native interactive shell and owned lifecycle.

### Modified Capabilities
None. Existing desktop compatibility and task authorization remain in force.

## Impact

Electron main/preload, shared renderer capability discovery, native PTY packaging,
shutdown supervision and isolated native/renderer tests. A terminal starts with
the user's OS permissions; a chosen working folder is not a sandbox. Agent input
does not inherit terminal authority. No public remote-shell API, WSL requirement,
automatic shell startup, administrator elevation or shared-session migration.
