# Encrypted backup CLI — 2026-11-20

Status: CLI journeys and rebuild integration pass the full affected-package
normal and race suites; the final key-file opening review correction also passes
its focused regression. This is tasks 3.1–3.3
of [encrypted-workspace-backups](../../../openspec/changes/encrypted-workspace-backups/tasks.md),
not completion of the storage, administrator UI or platform qualification gates.
[Storage evidence](encrypted-workspace-backups-2026-11-20.md) records its independent
review findings and their resolution separately.

## Implemented

- Command-specific native age recipient/identity and passphrase options. Existing
  plaintext syntax is retained and labeled; encryption-only verify/restore cannot
  silently accept plaintext. False passphrase flags, conflicting options, empty
  key selections and unsupported key formats are rejected before source mutation.
- Bounded native key files, redirected secrets and no-echo terminal input. Secret
  material never enters an error from flag/key parsing. Interactive backup/rebuild
  confirms input; the fixed-capacity reader checks bounds before appending.
- Context-aware input uses polling without a detached reader goroutine. The caller's
  stdin stays open. Interactive state restoration is attempted on all returns.
- Existing storage options own encrypted backup/verify/restore. Results retain
  the manifest and add actual encryption/authentication metadata. Filenames are
  not treated as format proof.
- Rebuild begins only after verified recovery backup publication. Backup failure
  or prior cancellation prevents the rebuild call; later rebuild failure retains
  and identifies the recovery archive without restoring or replaying work.
- Backup and rebuild hold one workspace owner continuously. The canonical data
  directory is resolved once before ownership; retargeting the caller's symlink
  cannot move the rebuild into a different, unlocked workspace. A real Linux
  symlink regression exercises this boundary; no model is downloaded or loaded.

Files: `cmd/offgrid/workspace_commands.go`, `workspace_encryption.go`,
`workspace_secret.go`, platform input adapters and the corresponding regression
tests. User instructions live in [workspace recovery](../../advanced/workspace-recovery.md).

## Actual checks

Windows, Go 1.26.6, temporary fixture roots only. No installed-user data, models,
personal console or live service was accessed.

- `go test ./cmd/offgrid -run '^TestWorkspace' -count=1` passed (7.642 s).
  Actual subprocesses exercise the root command dispatcher, JSON and exit codes
  for X25519, production-cost passphrase and plaintext backup/verify/restore.
  Restored bytes match source, misleading extensions do not change format,
  secret-valued boolean errors stay redacted, and plaintext cannot satisfy a
  supplied identity. Additional checks cover input bounds, canceled redirected
  pipes, key-file bounds, confirmation policy and the pre-rebuild mutation boundary.
- An earlier same-filter compile run preceded the new tests; it is not counted
  as behavioral evidence for this implementation.
- Linux focused CLI race plus real PTY checks passed, 51.168 s. A later six-case
  terminal run passed, 14.778 s, but stronger checking then exposed unread paste
  suffix bytes remaining for the next terminal consumer (failed, 2.255 s).
  Input is now flushed before echo restoration. The unchanged stronger test and
  closed-descriptor regression passed in the focused secret race run, 12.144 s.
- Independent review reproduced a missing-archive argument being mistaken for
  a filename and echoing a secret-valued flag in an open error. Option-looking
  archive arguments now receive generic usage errors; actual root JSON tests
  pass with exit 2 (1.277 s).
- All six real isolated Windows ConPTY cases passed: Unicode/surrogate input and
  confirmation, mismatch, Ctrl+C, context cancellation, oversized input and
  oversized paste plus suffix; no echo and exact terminal-mode restoration.
  Command: `go test -c ./cmd/offgrid -o build/workspace-secret.test.exe`, then
  `node scripts/test/test-workspace-secret-windows.cjs build/workspace-secret.test.exe
  --isolated-native-test`. Only disposable test processes/consoles were used.
- Concurrent-edit compile/vet failures referenced an agent discovery type before
  its new file was visible; another referenced a not-yet-created Ollama helper,
  and a later run referenced the pending `SkillReferenceAvailable` method.
  They are failed commands even though the CLI test binaries passed. Stable
  affected-package reruns passed as recorded below, with vet still enabled.
- Source-size and whitespace checks passed at the source checkpoint. Independent
  CLI review is in progress. A suggested empty-key parser gap was not established:
  pinned age v1.3.2 already rejects empty key files; explicit defensive checks and
  regressions were added without recording a nonexistent successful plaintext path.

## Stable aggregate gate

After the coordinated source hold, all of these complete package suites passed:

- Windows `go test ./internal/storage ./cmd/offgrid ./internal/rag -count=1`:
  storage **6.704 s**, CLI **5.969 s**, RAG **16.765 s**.
- Ubuntu/WSL `CGO_ENABLED=1 GOPROXY=off /usr/local/go/bin/go test -race
  ./internal/storage ./cmd/offgrid ./internal/rag -count=1`: storage **6.401 s**,
  CLI **82.458 s**, RAG **80.867 s**. Source was on `/mnt/d/offgrid-llm`.
- Local logs: `build/transformation-backup-normal-stable-20261120.log` and
  `build/transformation-backup-race-stable-20261120.log`. These retained all
  assertions, normal vet checks and the existing test deadlines. This checkpoint
  predates the final bounded key-file FIFO-opening correction, whose focused
  regression and rerun passed afterward: Windows key/secret/encrypted tests
  **2.740 s**, Linux FIFO race regression **1.350 s**. Unix key-file opening now
  uses `O_NONBLOCK` before the retained regular-file/identity checks, preventing
  a file replaced by a FIFO from blocking before inspection. Dedicated disposable
  FIFO tests prove the opener returns and rejects the replacement.

No installed terminal/product, macOS, administrator UI, participant or
representative-workspace performance qualification is implied. The storage
review findings were fixed and rechecked in the linked storage evidence.
