# OffGrid 0.4.33 local candidate — 2026-11-23

Status: local installation and subsequent publication requested by the user;
candidate qualification is in progress. The full M0–M8 transformation remains
incomplete. No publication or successful local upgrade is claimed by this record.

## Source and release identity

The latest published version was checked through GitHub CLI: v0.4.32, published
2026-11-18. Version metadata was advanced to 0.4.33 using the maintained version
update script. The candidate continues the documented unsigned preview channel.
Signing-tool progress messages are not signing evidence: native artifact trust
inspection reports `NotSigned`, not a trusted publisher.

The initial 4,323-file source snapshot is
`build/release-0.4.33-checkpoint-20261123.tar`, SHA-256
`8352f04f91b868b1073048f0411910d8cbc7c6909da792fab1ca3359f36dcd38`.
The packaging-corrected snapshot is
`build/release-0.4.33-packaging-fix-20261123.tar`, SHA-256
`00ad43ccb698e1fd1a7296e7fdffdf524abbf62fc27163fbf0cd199201ee1654`.
Every snapshot's extracted files were verified before its frozen checks.
Later source corrections require their own evidence; earlier passes do not
automatically qualify changed source.

## Passing checks and retained failures

- Production renderer build/type checks passed for 0.4.33.
- Full desktop tests: 292 passed, 4 opt-in skips, zero failures (3.262 seconds),
  `build/release-0.4.33-desktop-tests-20261123.log`.
- Full computer companion tests: 238 passed, 6 opt-in skips, zero failures
  (55.125 seconds), `build/release-0.4.33-computer-tests-20261123.log`.
- The exact release-script sequence from CI passed in the frozen Linux tree,
  including finalization, draft access/rebuild, desktop environment, immutable
  source/CI gates and container identity. All 12 additional preview/trust tests
  passed. Log: `build/release-0.4.33-linux-checks-20261123.log`.
- The earlier Windows wildcard release-script invocation incorrectly included
  CLI modules and used the Windows Bash bridge for Unix shell fixtures. It
  failed and was stopped; its log remains
  `build/release-0.4.33-scripts-20261123.log`. The Linux results above are a
  separate proper CI invocation, not a rewrite of that failure.
- The first package command misparsed a PowerShell configuration argument.
  The next package failed service identity verification because `-trimpath`
  removed linker-flag metadata needed by the existing gate. Rebuilding without
  that flag restored verifiable metadata. No verification gate was weakened.
- Attempt 3 built installers, but real packaged startup failed with missing
  `workspace-recovery.json`, matching the user's screenshot. The packaging
  mapping now selects the file from its parent directory; `presentation-pack`
  verifies its actual archived bytes. The new gate rejects the failed package.
  All 13 focused presentation/pack checks passed. Attempt 3 must not be installed.
- Attempt 4 passed actual packaged startup/recovery: first window 401 ms,
  recovery 492 ms, log `build/release-0.4.33-packaged-startup-fixed-20261123.log`.
  Isolated fixture evidence: `C:/Users/phil/AppData/Local/Temp/offgrid-desktop-startup-QPJ8ya`.
- The initial frozen full Windows Go run failed the real descendant-cancellation
  generation case. A second inference run reproduced it. Closing the worker's
  stream can expose a process footer; the protocol error replaced the expired
  caller context. The correction retains protocol, context and cleanup errors
  together. The original logs remain `release-0.4.33-windows-full-20261123.jsonl`
  and `release-0.4.33-windows-continued-20261123.jsonl`. Corrected checks are
  recorded separately when terminal.

## Local upgrade protection

Installed registration before upgrade:
`0.4.32-preview.20261116110208432.g9adc304f`, at
`C:/Users/phil/AppData/Local/Programs/OffGrid LLM Desktop`.
A private backup copied and hash-verified 3,590 files / 1,584,872,684 bytes:
the previous installation, main and alternate workspace data, desktop profile
and workspace configuration. It is retained at
`C:/Users/phil/AppData/Local/offgrid-owner-qualification-9532d19ef9954dd68c8594bcd5127580`;
`backup.json` records each source/destination/hash. Existing models remain in
place. The backup was made with no installed OffGrid process running. This is
a private local rollback copy, not an encrypted backup qualification claim.

Attempt 5 installed successfully with exact binary/archive verification. The
subsequent user-reported speech and UI corrections produced attempt 6, installed
successfully with a fresh backup and 73-file byte verification. Actual installed
Home startup was visually verified. See [voice correction and installation
evidence](voice-stop-interruption-ui-2026-11-23.md) for source identity, receipts,
real short-clip negative control, browser checks and limitations.

Exact final source CI and publication remain pending. The separate browser
512 MiB, voice acoustic/latency, model/hardware and full-program gates remain open.
