# Windows terminal workspace and input verification

The previous bottom-dock preview is superseded in source by a terminal view that
occupies the main workspace and preserves the sidebar. Chat returns to the same
workspace without killing shells. Shell tabs keep their terminal buffers. Native
authority remains confined to the separate bundled renderer; the web workspace
supplies only a validated display rectangle, clamped to the native window.

Two problems were found during implementation/testing:

- Every activation sent a PTY resize, even when dimensions had not changed.
  Resize is now deduplicated in both renderer and session owner. Hidden terminals
  are not fitted. Inactive inputs blur; the selected input receives focus after
  layout, with pending focus retained across ResizeObserver callbacks.
- A real packaged CLI `/clear` check initially failed: the PTY inherited
  `TERM=dumb` from the launcher and disabled its interactive behavior. The owned
  xterm renderer now supplies `TERM=xterm-256color`. Explicit plain-mode options
  remain respected. The passing rerun used the real bundled CLI in ConPTY.

`/clear` is a local display command in agent chat. The OS adapter clears terminal
screen/scrollback only for a real interactive terminal; piped/plain output has no
escape sequences. The shared command handler does not call service mutations.
Conversation context, profile revision, grants and saved work remain intact.

## Evidence

- `node --test apps/desktop/test/*.test.js`: 120 passed, one existing skip.
- `CGO_ENABLED=0 go test ./internal/cli ./cmd/offgrid -count=1`: passed in full.
- `CGO_ENABLED=0 go test ./internal/cli ./internal/rag ./internal/server ./cmd/offgrid
  -run 'Retention|SessionCommands|SessionInput|SessionOutput|AgentInput|AgentClear'
  -count=1`: passed. Windows race mode was not run.
- `npm run build --prefix apps/web`: passed; existing large-chunk warning remains.
- `node scripts/test/test-desktop-terminal-packaged.mjs
  "build/terminal-polish-20261103/win-unpacked/OffGrid LLM Desktop.exe"`: passed
  against disposable local state and an isolated service fixture. Shell command
  drafts/cursor positions survived repeated tab switches and Chat/Terminal
  changes. Actual agent `/clear` removed prior display, retained the session,
  and submitted no model turn. A partial `/session` command survived tab changes.
  Unicode, Ctrl+C-related existing native coverage, copy/search, blocked OSC52,
  200% zoom, and close/reload/quit descendant cleanup remain separately covered.
- The fixture does not establish model answer quality. No personal workspace was
  used. Raw screenshots remain in private temporary evidence directories.

macOS/Linux/ARM64, manual localization/accessibility and full release CI are not
established by this Windows candidate check.

## Local installation

After the user quit OffGrid, the 0.4.24 preview was installed on November 3.
The installer returned zero; all 786 candidate payload files matched the installed
SHA-256 hashes. The 484 pre-existing workspace files retained their checksums
(model files were compared by size and modification time). The previous app and
non-model workspace backup were retained outside the repository.

Installer SHA-256:
`EF50AC00CF54894BD6BB07FE7136C8E6E770AAC3F279B7A48ABB94AB7D74A98E`.
Local receipt: `build/terminal-polish-20261103/install-result.json`.
The outer PowerShell command inherited robocopy's successful copy exit code 1;
the receipt was written only after the installer exit-zero and all integrity
checks passed. No repeat installation was performed.

The personal app was not automatically launched. Installation integrity is not
a claim that the user's live model workflows were exercised.

Subsequent source-only composer component extraction passed type checking,
the source-size ratchet and 24 composer/Knowledge renderer regressions. That
behavior-preserving extraction is not part of this installed candidate.

## Follow-up: workspace menu occlusion

The user reported a workspace menu clipped at the main-area boundary. The cause
was native WebContentsView compositing above the main renderer's HTML top layer;
CSS z-index cannot resolve it. Sidebar utility panels now fit inside an expanded
sidebar while the terminal is visible, with stacked preference fields at that
width. Larger overlays temporarily suspend native presentation and input, retain
the same terminal buffers/processes, and restore without stealing menu focus.
The underlying inert Chat is concealed while a larger overlay is present. No
terminal transcript is copied into the service renderer.

- Desktop unit suite: 121 passed, one existing skip.
- Renderer menu/layout tests: 10 passed, including English/Arabic and light/dark.
- Packaged Windows test passed with a half-typed shell command: menu bounds,
  native visibility, overlay suspension/restoration and exact cursor/command
  preservation, plus the existing CLI and descendant-cleanup assertions.
- Full `CGO_ENABLED=0 go test ./...` passed. Native race mode is not covered.
- Source-size gate and web build passed. Desktop production dependency audit
  reported zero vulnerabilities at this check.

The follow-up candidate is `build/terminal-overlay-20261103`; it has not replaced
the earlier personal installation. Full web/release qualification remains a
separate gate. Release inventory also found the new native terminal supervisor
was matched by the broad binary ignore rule; explicit source exceptions were
added so a clean checkout contains the supervisor used by packaging.

The subsequent full web run reported 913 passed, 33 skipped and 27 failures in
composer, file-publication, Knowledge setup and voice/streaming journeys. Those
failures are release blockers under investigation; the focused terminal checks
do not override them.
