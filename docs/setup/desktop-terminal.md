# Desktop terminal

On the Windows desktop, open **Terminal** from the toolbar or **View → Terminal**
(`Ctrl+\``). Choose a local folder to open PowerShell. Chat and Knowledge do not
start a shell automatically. The browser application does not expose a terminal.

- **+** opens another terminal, up to four tabs.
- **OffGrid CLI** starts the matching bundled `offgrid agent` in a selected folder,
  using the same service address as the desktop. Existing CLI authentication and
  workspace approval rules still apply.
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

The footer identifies the shell and local folder. Commands run with your normal
computer permissions: this is not a sandbox. Terminal output is not automatically
sent to a model, saved in conversations, or exported in diagnostics. Shell output
cannot automatically write the clipboard or open web links.

If stopping cannot be confirmed, OffGrid reports that uncertainty rather than
claiming the processes stopped. A missing native runtime leaves the rest of the
app available. This preview's native terminal is enabled on Windows x64; macOS,
Linux and Windows ARM64 remain unavailable pending native qualification.
