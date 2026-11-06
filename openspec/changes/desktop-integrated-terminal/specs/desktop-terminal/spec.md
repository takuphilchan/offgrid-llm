## Purpose

Provide an optional native interactive terminal within the desktop workspace
without complicating ordinary workflows or expanding agent authority.

## ADDED Requirements

### Requirement: Optional local terminal
The desktop SHALL expose an initially closed terminal with clear local host,
shell and working-folder identity. Opening ordinary Chat or Knowledge SHALL NOT
start a shell. Windows operation SHALL NOT require Linux or WSL.

#### Scenario: Open native terminal
- **WHEN** a desktop user explicitly opens Terminal
- **THEN** a native interactive shell opens in the selected local folder with visible identity
- **AND** a remote service path is not silently used as a local folder

### Requirement: Interactive terminal behavior
The terminal SHALL support streamed output, resize, Unicode, keyboard input,
Ctrl+C, selection/copy, bounded scrollback and accessible focus navigation.

#### Scenario: Switch Chat and terminal views
- **WHEN** the user selects Terminal or returns to Chat
- **THEN** the selected view occupies the main workspace while the sidebar remains available
- **AND** terminal tabs retain their buffers and unfinished input without replay or spurious text at the cursor

#### Scenario: Clear an agent CLI display
- **WHEN** a user enters `/clear` at an idle agent CLI prompt
- **THEN** the terminal display and scrollback are cleared without deleting saved conversation context or cancelling work
- **AND** plain/piped output receives no terminal control sequences

#### Scenario: Run the installed OffGrid CLI
- **WHEN** the user explicitly launches the matching OffGrid CLI from the terminal
- **THEN** a native shell opens with the matching `offgrid` command ready, including `offgrid --help`
- **AND** interactive prompts and streaming work against the configured service without a second executor or fabricated command results

#### Scenario: Open an agent terminal
- **WHEN** the user explicitly selects Agent in the terminal
- **THEN** the matching `offgrid agent` CLI starts its existing profile-selection flow
- **AND** ordinary OffGrid CLI terminals remain at the shell prompt rather than entering Agent automatically

#### Scenario: Leave Agent and continue in the shell
- **WHEN** the user enters `/exit` in the Agent terminal
- **THEN** the conversation is saved according to the CLI contract and a usable shell prompt remains in the same chosen folder
- **AND** `offgrid --help` and ordinary shell commands can run without opening a replacement tab or repeating agent input
- **AND** closing that tab still terminates the owned shell and remaining descendants

### Requirement: Desktop-only authority boundary
Terminal input/output SHALL use a trusted desktop-owned surface. Remote pages,
model output and tool results SHALL NOT gain shell input or private terminal
contents simply by sharing a conversation or opening the panel.

#### Scenario: Untrusted renderer requests input
- **WHEN** an untrusted origin, stale renderer or different desktop session requests shell access
- **THEN** the request is rejected and no command is dispatched

### Requirement: Owned terminal lifecycle
The desktop SHALL track terminal processes through hiding, tab closure, reload
and quit. It SHALL distinguish a hidden live session from a terminated session
and SHALL NOT acknowledge termination before it is confirmed.

#### Scenario: Open a workspace menu while a terminal is visible
- **WHEN** a user opens sidebar preferences or another web overlay with a terminal active
- **THEN** the controls remain fully visible and keyboard reachable instead of being clipped by the native view
- **AND** opening and dismissing the overlay preserves the shell, unfinished input and cursor without replay or focus theft

#### Scenario: Close a terminal with running descendants
- **WHEN** a user confirms closure of a terminal running a child process
- **THEN** the owned process tree is stopped or the unresolved stop is visibly reported
- **AND** reopening does not replay commands or leave an untracked replacement shell

### Requirement: Packaged terminal qualification
Terminal availability SHALL be gated by its packaged native dependency and
platform tests, with safe fallback to the ordinary app if unavailable.

#### Scenario: Missing PTY dependency
- **WHEN** a desktop cannot load its terminal runtime
- **THEN** Terminal shows a repairable error while Chat and Knowledge remain usable
