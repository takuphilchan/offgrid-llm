# Desktop terminal

On the Windows desktop, open **Terminal** from the toolbar or **View → Terminal**
(`Ctrl+\``). Choose a local folder to open the OffGrid CLI terminal. Chat and Knowledge do not
start a shell automatically. The browser application does not expose a terminal.

- **OffGrid CLI** opens a local PowerShell prompt with the matching bundled
  `offgrid` command ready. Run `offgrid --help`, `offgrid models`, or any other
  supported command; it does not enter Agent automatically.
- **Agent** starts the matching bundled `offgrid agent` in the selected folder
  and opens its existing profile chooser. Existing CLI authentication and
  workspace approval rules still apply. `/exit` saves the conversation and
  returns to a usable PowerShell prompt in that same folder; the tab stays open.
- **+** offers OffGrid CLI, Agent and plain PowerShell, up to four tabs total. All tabs use the
  same configured service address without exposing credentials in command lines.
- Terminal replaces the main Chat area while keeping the sidebar available.
  Switch back with **Chat**; your shell and unfinished command stay in their tab.
  Resize the app window to change the terminal dimensions.
- **Search** searches the current tab. `Ctrl+C` copies selected terminal text;
  with no selection, it interrupts the running command. `Ctrl+Shift+C` always
  uses copy instead of interrupt. `Ctrl+V` pastes into the focused terminal.
- **Terminal display** (`Aa`) adjusts text size, line spacing, cursor appearance
  and scrollback. Changes apply to open and new tabs without restarting commands,
  and are remembered on this device. **Reset defaults** restores the original
  display settings. Lowering scrollback removes old displayed output, not saved
  conversations. Reduced motion disables cursor blinking; system forced colors
  override the terminal palette. Settings and their keystrokes never become shell
  commands. If preferences cannot be saved, they still apply to the current window.
- **Chat** returns to the workspace without stopping tabs.
- Interface language changes update terminal controls, open command previews,
  search status and accessibility labels. Search selections, edited questions,
  redacted output and running sessions are preserved. Terminal interface copy
  has drafts for all 29 selectable languages; this does not imply translated
  CLI commands/help, arbitrary process output or completed native-language review.
- **Commands** inspects the latest 100 shell-reported commands in that tab,
  including folder, duration and status, and jumps to output still in scrollback.
  A failed PowerShell command may have no exact numeric exit code; OffGrid shows
  that limitation rather than reusing a stale native-program code. Missing end
  markers remain unknown. These markers are presentation hints, not verified
  action receipts, and full-screen programs may not provide command boundaries.
- To ask about output, select terminal text or choose a command in **Commands**.
  Edit the preview to remove private information, add a question, then choose
  **Continue in Chat**. Select a conversation and **Add to draft**; existing draft
  text is preserved. Nothing is sent until you press Send. Previews contain at
  most 32,768 output characters and disclose truncation. Command snapshots stay
  in memory until that tab closes; they are not an automatic transcript export.
- Closing a live tab asks to stop its commands. Quitting OffGrid stops its owned
  terminal processes. Reloading the terminal also stops its sessions; commands
  are never replayed automatically.
- In OffGrid agent chat, `/clear` clears the visible terminal and scrollback.
  It does not delete saved chats, reset model context or cancel work. Piped/plain
  output stays free of clearing escape sequences.

Stopping a local process does not undo its changes or automatically cancel work
already submitted to an external service. Saved OffGrid agent work keeps its
normal Stop/Resume controls.

The tab identifies the session and folder; the footer shows whether the session
is open. An open session does not mean a command is busy. Commands run with your normal
computer permissions: this is not a sandbox. Terminal output is not automatically
sent to a model, saved in conversations, or exported in diagnostics. Shell output
cannot automatically write the clipboard or open web links.

If stopping cannot be confirmed, OffGrid reports that uncertainty rather than
claiming the processes stopped. A missing native runtime leaves the rest of the
app available. This preview's native terminal is enabled on Windows x64; macOS,
Linux and Windows ARM64 remain unavailable pending native qualification.

### Asking about command output

**Commands** appears after the shell reports a command. Selecting terminal text
changes it to **Ask about selection**, so unsupported command capture still has
a direct, deliberate path. Before either exists, the toolbar does not offer an
empty context form.

The preview follows a running command through completion. Editing output stops
automatic replacement of that text; shell events never restore information you
removed. A command with no text output cannot be shared as if it had output.
Failed handoffs keep the preview and question for retry, and a pending handoff
cannot be submitted twice. Nothing executes or sends a model request merely by
opening the preview.

Terminal search reports match counts. Clearing or closing search removes match
decorations, and typing into search or a dialog never sends input to the shell.
