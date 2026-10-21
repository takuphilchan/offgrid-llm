# Proposal

## Why

`cmd/offgrid/main.go` currently carries thousands of lines of unrelated command dispatch, terminal presentation, server startup, and model operations. This makes subsequent CLI architecture work hard to review and increases the chance that a behavior change is hidden inside a structural edit.

## What Changes

- Split the existing `package main` code into responsibility-focused files under `cmd/offgrid`, keeping the current command routing, signatures, control flow, output, and process exits intact.
- Keep `main.go` to version linkage and the process entry call. Extract root dispatch, version, server command, terminal output, progress, model local/remote/transfer/search, run/chat, benchmark, diagnostics, config, users, audit, metrics, audio, legacy agent/integration, and logs responsibilities where they currently reside in that file.
- Scope imports to each resulting file, preserve existing separate command files, and avoid editing neighboring agent/provider feature work.
- Capture and compare CLI help, version, representative JSON errors, aliases, default no-command service startup behavior, and `run` terminal fallback with pre-extraction fixtures.

This is a mechanical source reorganization. It does not fix the existing placeholder `offgrid chat` command or change CLI architecture into `internal/cli`; those are separate behavioral and architectural changes.

## Capabilities

### New Capabilities

None. No new user-visible behavior is introduced.

### Modified Capabilities

None. Existing CLI contracts remain unchanged. This pure refactor sets `skip_specs: true` rather than inventing a requirement delta.

## Impact

Primary impact is `cmd/offgrid/main.go`, new sibling Go files in the same `package main`, CLI tests and golden fixtures, and limited developer documentation. HTTP/API, storage, desktop, Python SDK, model execution, user data, and release artifacts must not change. No deployment, desktop replacement, commit, push, or publication is part of this change.
