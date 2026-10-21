# Tasks

## 1. Preservation inventory

- [x] 1.1 Add a bounded, read-only source/output inventory command and fixture tests; verify `node --test scripts/maintenance/workspace-cleanup.test.mjs` and record its baseline JSON outside the repository with source SHA-256, Git status, and explicit output targets.
- [x] 1.2 Reconcile staged, unstaged and untracked source against moved/archived paths, record the baseline Git categories and compatibility owners in `inventory-evidence.md`; verify `git status --short`, `git diff --cached --name-status`, and `git ls-files --others --exclude-standard` reveal no unexplained source loss.

## 2. Maintained structural ownership

- [x] 2.1 Correct confirmed stale `.gitattributes`, `.gitignore`, and `.dockerignore` entries while preserving effective ignore behavior; verify representative `git check-ignore -v` fixtures and the application/developer/repository layout checks.
- [x] 2.2 Correct current documentation and backup-location guidance without rewriting historical records or removing compatibility stubs; verify `node scripts/maintenance/check-docs.mjs` and an active-reference search across source, CI, scripts, installers, docs, and active OpenSpec changes.
- [x] 2.3 Record confirmed versus intentional duplication and remaining behavioral overlap in `inventory-evidence.md`; verify public installer URLs, package names/versions, OpenAPI hash, SDK identity, and release artifact names against the baseline.

## 3. Validate and quarantine generated output

- [x] 3.1 Run the applicable Go, web API/type/build, desktop/companion, Python SDK/worker, installer, Docker context, layout, documentation, and source-size checks before quarantine; record exact pass/fail/unavailable results in `verification-evidence.md` without claiming native or model qualification.
- [x] 3.2 Add guarded output-quarantine behavior and fixture tests; verify literal-path containment, rejection of tracked/nonignored files and occupied destinations, and recoverable per-target manifest updates with `node --test scripts/maintenance/workspace-cleanup.test.mjs`.
- [x] 3.3 Freeze source, record its final hash inventory, move only validated ignored output targets to a unique same-volume external backup, and verify every target has a matching destination and restoration entry; no permanent deletion.

## 4. Final integrity

- [x] 4.1 Verify post-move source hashes match the frozen inventory, dependency-free repository checks pass, the Git delta is attributable, and prior external backups remain intact; record results and restoration path in `verification-evidence.md`.
- [x] 4.2 Run `openspec validate repository-workspace-cleanup --strict --no-interactive` and `git diff --check`; review no API, CLI, SDK, desktop permission, or installed-user effects. Record unrun checks, and leave deployment, commits, pushes and backup deletion separate.
