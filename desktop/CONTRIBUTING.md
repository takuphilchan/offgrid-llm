# Contributing to the desktop application

The product UI lives in `web/app`; do not add a second frontend under
`desktop`. Electron owns only native lifecycle, packaging, tray behavior, and a
small context-isolated preload bridge.

For setup, packaging, security constraints, and validation commands, read
[README.md](README.md). UI changes must pass the checks documented in
[`../README.md`](../README.md) and should include a Playwright test for a
critical user workflow.
