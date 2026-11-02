# Local release checks, November 3, 2026

This record describes the current worktree, including changes not yet committed.
It is not a production-readiness or real-model qualification declaration.

## Checks completed

- `CGO_ENABLED=0 go test ./...`: passed on Windows. This does not cover race mode.
- Desktop Node suite: 121 passed, one existing skip.
- `python -m pytest sdk/python/tests -q`: 78 passed using the isolated build
  virtual environment.
- Generated API check, TypeScript/production web build, source-size ratchet,
  repository hygiene and maintained-document checks passed.
- The packaged terminal test passed with real ConPTY input, cursor preservation,
  menu/overlay visibility, CLI `/clear` and owned descendant cleanup. See the
  separate terminal polish record for its scope and personal-install status.
- Desktop production-dependency audit reported zero vulnerabilities. This does
  not replace native-library or development-tool vulnerability review.

## Full renderer run and corrections

The first fixture-server run reported 913 passed, 33 skipped and 27 failures.
Several tests still assumed the previous composer: directly visible model/agent
and saved-file controls, rectangular response buttons and visible Stop text.
Tests now navigate the actual attachment/settings menus and assert the icon
buttons' accessible names, preserving cancellation/streaming/privacy checks.
Three service-dependent tests were incorrectly run against Vite alone; their
404/startup failures were not evidence of application service failures.

The first disposable-service run then reported 936 passed, 28 skipped and nine
failures, all localized versions of the attachment-to-Knowledge dialog. This
revealed a real top-layer/focus defect: the modal was nested in an auto popover,
and Escape could hide its ancestor while leaving the dialog open. The dialog
now renders in a body portal, records its opener before autofocus, and returns
focus to the visible attachment trigger. Utility popovers defer Escape to a
modal above them. Reopening the source list does not repeat publication.

After rebuilding, the full disposable-service run passed **945 tests**, with
**28 skipped and zero failures** in 3.0 minutes. The run used the packaged Windows
service and parser workers, a fresh temporary home/data/model directory, a unique
loopback port and the production web build. The process was stopped after the
run. No installed models, personal documents or saved conversations were used.

Skipped checks require separate model acquisition/restart fixtures, installed
user-state or canonical job seeds, opt-in performance throttling and the optional
visual matrix. Passing this suite does not establish live model answer quality,
macOS/Linux native terminal support, manual accessibility/localization or the
unavailable five-person usability evaluation.

Local logs: `build/release-go-all.log`, `build/terminal-overlay-desktop-tests.log`,
`build/release-python-sdk.log`, `build/release-web-service-final.log` and
`build/terminal-overlay-native.log`. Private test outputs remain outside ordinary
exports. Publication still requires the remaining source commits and CI gates.

## Staged snapshots and packaging follow-up

The extraction/storage commit was checked from an exported staged tree: its RAG
and OCR helper tests passed, and all root-module packages compiled. The service
commit was also tested from an exported staged tree. Its first split exposed
cross-layer CLI test dependencies; those tests and their Knowledge CLI commands
were grouped together before committing. Agent, session, server, service client,
application and CLI tests then passed in that staged snapshot.

Both independent PDF and OCR worker modules passed their Go tests. Preparation
tamper checks passed; the optional original-archive extraction test was skipped
without its archive environment variable. The final packaged Windows terminal
test and packaged theme/200% zoom test passed against the corrected candidate.

CI for the earlier terminal-only commit failed five renderer expectations and
the packaged profile-library selector on Windows/macOS. The complete local UI
run covers the updated interactions; the packaged selector now follows the
current profile-library action. The initial CI failure remains recorded, and
the complete committed release source still requires a fresh green CI run.

Release workflow review also found that desktop CI jobs did not acquire the new
ignored PDF/OCR assets. They now depend on the pinned OCR build, download its
verified output and prepare PDF assets before packaging. The Windows job also
runs the packaged terminal regression. Workflow syntax validation passed.
