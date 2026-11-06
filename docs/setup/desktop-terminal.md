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
- **Search** searches the current tab. `Ctrl+Shift+C` copies selected text;
  `Ctrl+C` interrupts the running command.
- **Chat** returns to the workspace without stopping tabs.
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
