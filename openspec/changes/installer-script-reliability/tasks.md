# Tasks

## 1. Safe alternative installers and guidance

- [x] 1.1 Add checksum gates, complete scoped Windows payload copying, Unix running-process refusal and truthful failures; add disposable fixtures and run `node --test scripts/maintenance/desktop-installers.test.mjs`.
- [x] 1.2 Correct installers README and setup installation, quickstart and desktop-startup guidance; verify `node --test scripts/maintenance/check-docs.test.mjs` and `node scripts/maintenance/check-docs.mjs` pass.

## 2. Integration verification

- [x] 2.1 Run `bash scripts/maintenance/test-installer-entrypoints.sh`, script syntax checks, and `openspec validate installer-script-reliability --strict --no-interactive`; record actual results and native-platform limitations in this change.
