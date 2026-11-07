# OffGrid 0.4.28 release and installed-build verification

The release source is `6154aa8b7d752169b32bd7f8c5fa9043ae757542`.
The focused installer, terminal, CLI diagnostics and evidence commits were pushed
to `main`; the annotated `v0.4.28` tag points to that source. The first tag push
failed with an SSH connection error; retrying succeeded without replacing a tag.

## Automated and packaged evidence

[Exact-source CI](https://github.com/takuphilchan/offgrid-llm/actions/runs/42377062659)
passed. The detailed behavioral coverage and retained failures are recorded in
[terminal verification](terminal-session-followup-2026-11-07.md),
[installer verification](native-installer-refresh-2026-11-07.md) and
[Talk/integration verification](talk-silence-and-integration-status-2026-11-07.md).

The production Windows installer was built with:

```powershell
# From apps/desktop
npx.cmd electron-builder --win nsis --x64 --publish never --config.directories.output=../../build/release-0428-local
# From the repository root
node scripts/test/test-desktop-startup.mjs "build/release-0428-local/win-unpacked/OffGrid LLM Desktop.exe"
```

Both commands passed. The isolated production startup test recorded an 883 ms
first window and 1,151 ms recovery fixture on this machine. Local logs are
`build/production-installer-0428.log` and `build/production-startup-0428.log`.
These measurements describe the fixture, not a universal startup guarantee.

## Installed Windows application

After the owner quit OffGrid, the prepared installation helper verified the
installer digest, backed up the application and workspace outside the repository,
and installed the per-user update. Five payload files matched the candidate;
381 root workspace files and 10 desktop workspace files retained their hashes.
Model inventory, inference runtimes and the Electron profile were preserved.
Public branding is OffGrid; the existing installation identity and data paths
remain compatible. The rollback backup is retained.

The local installer SHA-256 is
`F07897047DD27EA7D468728CFE9FF07606FFF30B1EBA314E7264C2C01030F94E`.
This identifies the locally built installer, not the separately built GitHub
asset. Installation results are in
`build/release-0428-local/install-result.json` and
`build/install-offgrid-0428-local.log`.

The owner reopened OffGrid. The native service was a child of the installed
desktop process, and its executable matched the candidate SHA-256:
`A3F28B6A748FE6ADD49F6628F8D293F389C9632BA3B188EDB2F338F3E13AC318`.
`GET /health` reported `healthy`, version `0.4.28`. The installed terminal host
was also running. These are installed identity/startup checks; they do not claim
a new live model-quality or external-agent workflow evaluation.

## Publication

[Release workflow](https://github.com/takuphilchan/offgrid-llm/actions/runs/42378980803)
was still running at this checkpoint. All 14 expected desktop and CLI assets
were independently checked against the finalizer's expected names: each existed
once, had uploaded status, a positive size and a valid SHA-256 digest.

Both CPU image builds subsequently passed. The multi-platform manifest was
published with digest
`sha256:febe2a5b86ecbfeff4e170615d759ced6db6f9985b23ed51e3a2f80cbfda34c2`,
but job `128432462835` failed during `actions/attest@v4` with
`InternalError: error creating tlog entry - (500) unexpected result from transparency log`.
The completed job log was retrieved through the GitHub API and retained at
`build/release-0428-cpu-manifest-failure.log`. This is an attestation-service
failure, not a failed application build. A targeted rerun request was refused
while the CUDA build remained active. The existing workflow was not cancelled.

Publication, successful attestation and the final checksum manifest remain
unverified until the failed gate is retried successfully and the workflow
finishes. The release remains a draft; no manual bypass was performed.

## Remaining product qualification

This maintenance release does not close the entire product roadmap. The existing
plans retain guided-setup journey comparison, document-answer/model accuracy,
enterprise retention qualification, manual accessibility/localization and
five-participant usability evaluation. Other native hardware combinations remain
unverified where the corresponding evidence is absent. The owner previously
confirmed that usability participants and additional provider hardware were not
available; automated checks are not substitutes for those evaluations.

The new terminal analysis is recorded in the
[design audit](../../advanced/design-audit.md). Multiline/grapheme-aware input,
live steering, shell command blocks and deliberate output-to-agent transfer are
follow-up requirements, not functionality shipped by this release.
