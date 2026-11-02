# Defaults-first profile setup — 2026-11-02

Scope: `workspace-outcome-experience` task 22.5. These are implementation and
isolated browser results, not human usability or installed-platform qualification.

## Behavior

- New profiles start with name, purpose and an existing chat-model suggestion.
  The eligible current selection wins, then a loaded model, then an installed
  eligible model. Explicit runtime failures and non-chat models are excluded.
  This runs only when creating a draft; it does not load/download a model or
  replace saved revisions or an edited draft when the inventory changes.
- Tools and context are optional and initially collapsed for an empty profile.
  Resource discovery starts when opened. Resource outages are contained there;
  they do not prevent creating a writing-only profile. Existing attachments are
  opened for review, and closing a disclosure retains its unsaved state.
- Description, runtime details, decision budget and execution policy live under
  Advanced. Defaults remain ten decisions and no tool, folder, desktop, Knowledge
  or collaborator authority. Save and Use in Chat retain the existing revision
  and effective-permission checks.
- The form has a bounded reading width, no empty library column and a compact
  model picker. Nine-locale tests assert two initially visible text fields,
  closed optional sections, exact saved settings and no horizontal overflow.

## Verification

`npm --prefix apps/web run test:e2e -- tests/profile-defaults.spec.ts tests/agent-profiles.spec.ts tests/agent-context.spec.ts tests/workspace-order.spec.ts tests/agent-layout.spec.ts --workers=1 --output=../../build/profile-lazy-final`

Result: **66 passed**, 2.8 minutes. Coverage includes revision conflicts,
dirty-draft protection, model removal, unavailable resources, purpose examples,
skills/memory, profile handoff, nine locales and narrow/RTL management layouts.
TypeScript and production build passed; the existing bundle-size warning remains.

Baseline and resulting screenshots are retained in `build/profile-before/` and
`build/profile-lazy-final/`. Visual inspection found that the shared controls layer
overrode the intended model-picker width. Its owning stylesheet was corrected,
and `profile-defaults.spec.ts` now asserts a rendered width at most 449px on the
wide fixture. The follow-up capture in `build/readiness-verified/` passes.

Earlier test failures are retained in `build/profile-defaults-verified.log`:
the effective-profile fixture omitted its `issues` array, and a model-refresh
fixture refreshed the page rather than the shared inventory. Corrected fixtures
preserve the real contract and user interaction. The first resource-disclosure
run also exposed tests assuming optional controls were always mounted; those
tests now open the visible section before interacting.

## Related document verification

With `OFFGRID_E2E_DOCUMENT_WORKERS=1`:

`node scripts/test/test-web-workspace.mjs build/document-runtime/win-x64/offgrid-ux.exe tests/chat-files.spec.ts tests/knowledge-partial.spec.ts tests/knowledge-recovery.spec.ts --grep "real service|real mixed" --workers=1 --output=../../build/document-ux-real-final`

Result: **3 passed**, 6.0 seconds, explicit native runner exit code **0**.
These exercise real PDF/Office attachments and retained originals, mixed-PDF
readable-page recovery, and rejected Office import retention/deletion. They do
not qualify model answers, embedding retrieval quality or other platforms.
The earlier broader runner reported 71 passed but the shell returned nonzero
while formatting a Node color-environment warning. Its log remains in
`build/document-ux-real.log`; the targeted rerun captures `$LASTEXITCODE` explicitly.

The broad redesign, enterprise policy/retention completion, integrated terminal,
hardware qualification and manual usability gates remain open. No installed
application, user data, inference service or model selection was changed by these
isolated checks.

## Commit-boundary verification and readiness follow-up

Profile source was committed as `306a5270` after extracting the exact staged Git
tree into `build/profile-staged-20261102/`. Its production build passed; 54
profile/context tests passed there. Vite development serving rejected font files
through the dependency junction, so visual confidence was not inferred from that
run. The built bundle was then served on isolated loopback port 5174: all 13
defaults-first checks passed, including nine locales and the model-picker width.
Both preview servers used for this verification were stopped afterward.

Readiness improvements were committed separately as `9cc373df`:

- Known runtime failures are excluded from initial selection and first-chat
  readiness. Legacy services without the additive readiness field retain their
  compatible chat path. These checks are not model-quality qualification.
- An existing choice is preserved when its model disappears. Chat keeps the
  draft, blocks button/Enter submission, and offers explicit model selection.
  An inventory request failure retains the previous model inventory and reports
  the failure; it does not restart onboarding or substitute a model.
- Setup guidance includes installed models and offline import, without starting
  downloads. Quick actions now has one contiguous System group rather than
  duplicate React keys and repeated headings.

The exact staged readiness tree built successfully; its production bundle passed
33 checks (`chat-readiness.spec.ts`, `workspace-experience.spec.ts`,
`ux-recovery.spec.ts`) in 44.3 seconds. The broader dirty-tree run before the final
palette correction passed 46 checks. A later run exposed a timing assumption in
the reconnect test: saved partial text can render before the new event stream
attaches. It now waits for the observable second attachment and still checks one
explicit cancellation. A legacy Activity test was also corrected to interact
with the directly presented run list, rather than wait for Diagnostics to own it.
Earlier failed logs remain under `build/readiness-final/` and
`build/readiness-palette-final/`.

Both commits were pushed to main. OpenSpec validation, source-size checks and the
maintained Markdown link check (304 files) passed. No new release or installation
was performed. Broader CI and the outstanding program gates are not established
by these focused results. Task 22.5 is complete; tasks 22.1–22.4 and 22.6–22.7
remain open, including the full guided readiness and end-to-end document journey.

## User-requested local installation

The user subsequently requested installation and quit the existing desktop.
The current worktree was packaged as a **local 0.4.24 preview**, not an official
release payload. UI build passed; desktop tests reported 102 passed, zero failed,
one platform-gated skip. Packaged PDF/OCR, executor, computer and renderer
manifests passed verification. The packaged service passed the three real
PDF/Office browser checks again (6.1 seconds).

The previous application and non-model workspace files were copied to
`%LOCALAPPDATA%/OffGrid-install-backups/defaults-20261102-182549`. Application and
workspace backup digests were checked before replacement. The installer exited
zero; all 617 candidate payload files matched their installed hashes. All 475
pre-existing workspace entries retained their hashes, or size/timestamps for
model weights. The native GPU runtime and configuration hashes were unchanged.
No Docker container, WSL configuration, model or language pack was replaced.

Installer SHA-256:
`dccf3c18fa42b8efdc4b5703cdc8a914a14b59885382c28ae3085bc5b0a4c4ba`.
After the user opened OffGrid, the native service reported 0.4.24, healthy, and a
ready desktop bridge. Both the installed renderer and served `/ui/` matched build
`aa26e1fc432fceeae0f31a054a84f924a6054fde832b37a240a6c22981ae217b`.
Exactly one native `offgrid.exe` service was observed, parented by the installed
desktop process. No inference or personal-document task was submitted during
this installation check. The wider qualification gates remain open.
