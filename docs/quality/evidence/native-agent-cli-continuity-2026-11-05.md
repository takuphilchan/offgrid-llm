# Native agent CLI continuity and terminal presentation — 2026-11-05

## Implemented

- Interactive profile selection searches name, model, description and ID; keyboard selection keeps a visible search caret and highlighted row. Plain/redirected input retains a recoverable numbered and text search path.
- An explicitly remembered default is scoped to actor, service origin, workspace and exact profile revision. Resume uses the saved conversation's agent identity; `/agent` starts a separate conversation after checking active work. Neither path transfers grants or replays tools.
- CLI headings use task names without repeated `OffGrid /` labels. Agent chat opens with its profile and model. The embedded terminal uses a centered reading width on wide windows, larger type, and immediate focus restoration after closing another tab.
- Interactive model progress animates in place, while actual tool actions remain in the transcript. The embedded terminal enables Unicode frames unless the user has disabled Unicode explicitly.

## Verification

- `go test ./internal/cli ./cmd/offgrid -count=1 -timeout 3m` passed after the final CLI changes.
- `go test ./... -count=1 -timeout 10m` passed after the final CLI and terminal changes.
- `node --test apps/desktop/test/terminal*.test.js` passed: 20 tests.
- `node scripts/test/test-desktop-terminal-packaged.mjs 'D:/offgrid-llm/build/desktop-cli-continuity-20261105/win-unpacked/OffGrid LLM Desktop.exe'` passed after final packaging. It exercised picker search, invalid input, cancellation, default selection, separate-session switching, tab focus, 200% zoom, process cleanup and the wide centered layout. Light and dark captures are in the isolated private fixture directory `C:\Users\phil\AppData\Local\Temp\offgrid-desktop-theme-RyW6Nd` for the final candidate.
- The Windows installer exited 0. Five installed package files matched the staged SHA-256 hashes. All 10 desktop-workspace files and 382 root-data files matched their pre-install manifest hashes. The rollback backup remains at `D:\OffGrid-install-backups\v0.4.26-before-continuity-20261105-192824`.

## Limits

This is an installed 0.4.26 preview, not a new release or proof of cross-platform usability. The packaged fixture uses test profiles and does not qualify real-model answer quality, speech, native document workflows, or the broader three-run platform suite. The local installer was not established as Authenticode-signed. Human usability and screen-reader evaluation remain unverified.

The installation above predates the November 6 Chat/sidebar/dictation follow-up
and the shortened terminal folder label. It does not establish that those later
source edits are installed. See the current follow-up in
[the design audit](../../advanced/design-audit.md) for their separate status.
