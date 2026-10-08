## 1. Shared utility popovers

- [x] 1.1 Extract viewport-bounded, dismissible utility-popover behavior and migrate VoiceSettings without changing profile loading/cancellation; add keyboard/outside-click/short-viewport regressions, document the component in docs/advanced/workspace-ui.md, and pass `npm.cmd --prefix web/app run check` plus the voice-playback Playwright suite through the isolated workspace runner.

## 2. Compact workspace shell

- [x] 2.1 Extract navigation/header presentation, add remembered desktop collapse and workspace options, consolidate shell geometry, and localize new labels in all nine locales; update workspace docs and desktop theme smoke locators, and pass workspace-shell, workspace-experience, control-consistency, and agent-layout Playwright tests covering drafts, refresh, keyboard, narrow/short layouts, RTL, themes, and Electron presentation synchronization.

## 3. Integrated verification

- [x] 3.1 Run `npm.cmd --prefix web/app run api:check`, `npm.cmd --prefix web/app run check`, `npm.cmd --prefix web/app run build`, a fresh `go build -o build/workspace-shell-foundation/offgrid.exe ./cmd/offgrid`, full `node dev/scripts/test-web-workspace.mjs build/workspace-shell-foundation/offgrid.exe --workers=4 --reporter=line`, `npm.cmd --prefix desktop test`, documentation checks, and strict OpenSpec validation; inspect representative screenshots and record actual results and unrun qualification in docs/advanced/product-reliability-plan.md. Do not deploy, commit, or publish.
