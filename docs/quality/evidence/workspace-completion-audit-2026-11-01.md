# Workspace experience completion audit — 2026-11-01

The implementation and release candidate are not proof that the entire experience
program is complete. This audit began at **83/93 tasks**; after the order and
presentation follow-up, `workspace-outcome-experience` is **91/101 tasks**, with
the same ten qualification items open. This audit preserves its original
acceptance criteria rather than replacing them with the checks available here.

On 2026-11-01, the user confirmed that non-developer usability participants and
manual screen-reader/localization reviewers are unavailable and instructed us to
keep those checks unverified. Automated release work may proceed independently;
the related tasks remain unchecked and no human-evaluation result is claimed.

## Verified source and distribution evidence

- Release source: `7f72bd76431e7dba86802278789af8b9debaa732`, tag `v0.4.22`.
- [Exact-source CI](https://github.com/takuphilchan/offgrid-llm/actions/runs/41260809684)
  passed all eight jobs: Windows dependency packaging, pinned parser, Python on
  Windows/Linux, native contracts and packaged desktop on Windows/macOS Intel/
  macOS Apple Silicon, and Go/contracts/renderer/Linux desktop.
- The Linux renderer result was **666 passed, 28 opt-in/environment skips**.
  Real-service acquisition and restart fixtures passed another **6 + 1** cases.
  Go tests and the exact CI race package list passed on this revision.
- CI exercised actual packaged Electron startup/recovery and theme/200% zoom
  scripts on all three operating-system families. Windows additionally passed
  isolated installation, reinstall and uninstall. These checks do not constitute
  manual screen-reader, physical-display or real model-workflow qualification.
- The downloadable Windows CPU archive was independently downloaded and matched
  GitHub's asset digest:
  `3233a806532c0b599091aacc4dae3a56723f7a3aa26609f98c3416589ce3584f`.
  Its five runtime-manifest files matched their SHA-256 values. `llama-server
  --version` succeeded with a cleared environment and system-only PATH, reporting
  `b95502b-offgrid-seq2`, MSVC 19.51.36260.0. This is dependency/startup evidence,
  not model inference. Installed runtime and data were unchanged.
- [Release packaging](https://github.com/takuphilchan/offgrid-llm/actions/runs/41262189689)
  is the authoritative publication record. At this audit's creation, all 14
  required downloadable assets had built, but the CUDA container was still
  running and the release remained a draft. Do not infer publication from the tag.

Earlier native model results and retained failures are in the
[native-agent record](native-agent-termination-2026-10-31.md) and
[whole-product record](workspace-experience-2026-10-30.md). The three packaged
Windows CPU artifact passes used the named candidate binaries, not downloaded
v0.4.22 packages. Exact output, evidence scope and restart were checked; those
results do not certify arbitrary models or tasks.

## Outstanding requirement audit

| Task | Evidence available | Missing evidence; completion status |
| --- | --- | --- |
| 17.3 Image journeys | Browser upload/paste/drop, validation, scoped storage/reload and failure tests; packaged Windows unsupported-model/draft-retention journey; CLI transport/history tests | Actual local vision-model interpretation and remaining native model/platform coverage. **Incomplete.** |
| 16.5 Native distribution | Compiled dependency closure tests, downloadable Windows worker startup, cross-platform packaged startup, Windows installer lifecycle, local Windows CPU/GPU candidate replies | Complete fresh-install first-reply, sleep/wake and upgrade/recovery matrix on Windows/macOS/Linux with real models. **Incomplete.** |
| 16.16 Locales and accessibility | Nine locale resources, automated keyboard/RTL/IME/reduced-motion/contrast/reflow, genuine packaged 200% zoom, maintained guides | Manual screen-reader pass and independent locale/assistive-technology assessment. **Incomplete.** |
| 16.17 Complete journeys and CI | Exact-source CI; real Windows first reply, selected-source Knowledge answer, three exact-artifact agent journeys and saved-state restart; fixture cancellation/navigation/failure cases | Real native failure/cancellation/navigation combinations and remaining installed-platform/model journeys. Fixtures cannot substitute for all native outcomes. **Incomplete.** |
| 16.18 Formative study | Frozen journeys and target exist in the design | Five consenting representative non-developer participants, actual observations, 90% unassisted attempt result and any required iteration. **Incomplete.** |
| 14.4 Material qualification | On/off and accessibility fallbacks, sampled/composited contrast, repeated 4x/6x feedback gates and browser captures | Physical-display/native and manual assistive-technology review. **Incomplete.** |
| 6.2 Screen quality | All-page automated coverage, screenshots and cross-platform packaged theme/zoom | Manual screen-reader pass. **Incomplete.** |
| 6.3 Baseline performance | Current history/stream/Stop timings, route costs, bundle sizes and bounded rendering; earlier paging comparison | Full same-fixture comparison against the exact original dirty-source group-1 build, including missing historical cold/warm timings. **Incomplete.** |
| 7.2 Five-person comparison | Twelve frozen journeys and acceptance target | Consented observations, counterbalanced before/after sessions, detours/help/recovery records and target evaluation. **Incomplete.** |
| 8.4 Installed editions | CI runs matching packaged startup/theme scripts on Windows, both Macs and Linux; Windows install lifecycle | Full installed-edition preferences/popup contrast/returning-work/rollback matrix with independent native inspection. Packaged automation alone does not establish every criterion. **Incomplete.** |

## Baseline preservation audit

Group 1 identifies a dirty-source renderer by index SHA-256
`64feef56af8248b92c5e06e5ec33a3629509d8f9d3e19723cf788596fed78d5d`.
The original evidence explicitly says cold/warm route timings were not captured.
The named baseline screenshots remain in the recoverable root-cleanup backup;
the baseline directory contains test results, not a runnable renderer snapshot.
A read-only scan of 103 retained `index.html` files under `D:\offgrid-llm-backups` found
no exact matching index fingerprint. No backup was modified or removed.

The retained workspace outcome, shell and recovery build directories also did
not yield a matching baseline source manifest or patch in the targeted inventory.
Their current evidence cannot reconstruct the original uncommitted source bytes.

This is a scoped negative search, not proof that the build exists nowhere.
A clean checkout of the original HEAD would omit its uncommitted shell changes
and is not an equivalent baseline. A new benchmark against another release must
be labelled separately and cannot retroactively satisfy the missing comparison.

## Downloaded Windows desktop attempt

The actual draft-release portable desktop was downloaded and its SHA-256 matched
GitHub's asset digest:
`8e85d38b3b5d805950509c50eb3f0bf1a0286e54b59c958a9599727fc89b88d8`.
The embedded archive was extracted into ignored build storage, without installing
or replacing the user's desktop. The existing native journey was invoked with
that desktop, the independently verified downloaded Windows CPU worker bundle,
the retained Qwen2.5 3B Q4_K_M model, and `--agent`.

This attempt **failed before the first reply**. The service rejected even the
512-token allocation with `insufficient_runtime_capacity`; the unchanged
120-second first-answer assertion then expired. At inspection, Windows reported
3,632,368 KiB available physical memory on a 16,470,736 KiB system. The CPU
estimator subtracts the approximately 2,008 MiB model and approximately 1,608 MiB
system reserve, leaving no context budget at that observation. This explains the
admission rejection; it does not establish an out-of-memory crash or a broken
GPU runtime. The isolated test explicitly disables GPU use.

The screenshot reported by the user matches this test's retained failure view.
Its temporary home is `offgrid-desktop-native-reply-wvrNK0`; private captures and
the full failure stay there, with the invocation log in ignored
`build/release-0.4.22-download/native-agent-release.log`. The test exited, and a
process check found no remaining OffGrid or llama-server test workers. No user
applications were stopped, no safety reserve was reduced, and no inference was
silently moved to a different model or device.

The raw capacity message and generic "Check request status" action exposed a
recovery-presentation weakness in that release artifact. Zero estimated context
capacity must not be described as zero physical RAM. Download integrity,
extraction, and startup are verified; successful inference from this downloaded
desktop remains unverified. Earlier candidate successes are not substituted for
this failure.

## Typed memory-recovery follow-up

The follow-up preserves optional `error_code` through ordinary session SSE and
durable saved turns. The web/desktop client now preserves typed errors both
before stream startup and within a stream. Known capacity failures receive
nine-locale explanatory copy, a model-selection handoff that retains the draft,
and collapsed original diagnostic details. Unknown codes keep their existing
diagnostic/status-inspection behavior. Reconciled failed turns no longer remain
in the active-work counter. Neither the memory estimator nor its reserve, model
selection, GPU policy, context allocation or retry authority changed.

Validation on the follow-up source:

- `go test ./...` passed on Windows.
- `go test -race ./internal/sessions ./internal/server` passed in WSL; server
  package time was 305.633 seconds. This is focused race coverage, not the entire
  CI race package list.
- Dedicated Go tests cover legacy/durable terminal error codes, saved failed
  status, retained prompt, untouched completed context and read-only replay.
- API generation/check, TypeScript/build and the source-size ratchet passed.
- Focused streaming/continuity checks passed **34/34**, including HTTP rejection,
  SSE failure, reload, no duplicate submission, model navigation, preserved drafts,
  unknown codes and all nine locales at 390px width.
- The full isolated renderer passed **678 tests, 28 existing skips** in 5.3
  minutes. Desktop-width and narrow Arabic recovery screenshots were inspected.
  These are browser fixtures, not a manual screen-reader or native-language review.

Initial test failures remain in `capacity-recovery-ui*.log`: one assertion
incorrectly expected an active-work button after terminal work was removed, one
used the wrong existing button label, and the HTTP case caught a second real
client path that discarded structured errors. The final run fixes the product
path and checks the actual labels and removal behavior. Logs and captures remain
under ignored build/test storage. No prior native inference failure is erased.

This follow-up is newer than release source `7f72bd76`. Do not claim the existing
v0.4.22 tag or already-built artifacts contain it. Publication/rebuild identity
must be verified separately.

## Completion boundary

No checkbox is closed by this audit and the change is not archived. Participant
and screen-reader availability has been requested; no one has been contacted or
represented as a test participant. Real-vision tests require explicit local
compatible artifacts; no model download or fallback is authorized by this audit.
Release publication and local installation are separate actions. The broader
goal remains incomplete until the outstanding criteria have genuine evidence.

## Published v0.4.22 and local replacement

The superseded draft described above was rebuilt with the approved memory-recovery
fix. The published tag resolves to `0c576b84dda2c486b2f445641edbc54c07ab8ef2`.
CI run `41268276153` passed all eight jobs; release run `41269737965` passed
all 21 jobs. GitHub published the release at 2026-11-01 04:27:38 UTC. The
checksum manifest matches all 14 package asset digests. This supersedes the
earlier draft identity, without erasing its native inference failure.

The Windows installer completed successfully. Installed application archive,
renderer index and service binary match the downloaded portable package. All
26 model files and 17 native GPU runtime files match their preservation hashes;
the computer pack, execution pack and UI compatibility manifest verify. The
desktop was reopened during the final data comparison, creating new session
files: a post-launch identical data-directory count is neither claimed nor
required. The pre-install data backup was compared before installation. No data
backup was restored over the reopened workspace.

The published GPU container image was imported with its registry config digest
`sha256:6417ad98dc743c3ec4be89204e9bea8ceda5148b141a07b5eb2402044f994a74`
and all 20 uncompressed layer identities checked. Docker's local manifest
identity differs after archive import; its config descriptor was independently
read back from containerd and matched. The replacement retains the named data
and model volumes, loopback port binding, GPU request and security restrictions.
The old container and a compared data-volume backup remain recoverable. The new
container is intentionally stopped: native Windows remains the active workspace,
and starting the old Docker workspace on the same port would be incorrect.

## Post-release pressure and control audit

The later "Server under heavy load" report was reproduced at the admission
boundary: the pressure manager rejected every request at its emergency level,
including when the resident model had no active or queued generation. Its
advertised emergency concurrency was one. The follow-up enforces its advertised
10/5/2/1 request limits atomically, while retaining loader memory/context checks.
All three chat transports return typed `inference_busy` on exhausted slots.
The renderer explains that rejection, retains the draft and does not offer a
misleading status action or automatically resubmit.

The installed v0.4.22 renderer identity matched the release; the reported
full-width language selector was a CSS precedence defect, not evidence of an
old executable. A shared file-picker control replaces the two remaining exposed
browser file inputs (skills and offline model packages), retaining native file
dialogs, explicit imports, scoped permissions and repeat folder selection.
Knowledge and Chat images already use labelled custom triggers. Settings now
constrains the language field in the controls layer.

Initial browser failures exposed incorrect test selectors and an unmocked v2
read falling through to the installed service. Fixture-mode Vite now disables
all backend proxying; the streaming fixture explicitly supplies its v2 state.
Read-only work from the installed workspace is no longer mixed into test results.
Private traces remain outside tracked source. These new fixes are newer than
published v0.4.22 and must not be described as included in its release assets.

Checks completed before installation of the follow-up: Windows `go test -p 1
./...`; focused WSL race checks for pressure/admission/capacity; API generation,
TypeScript/build; 50 focused renderer tests with one isolated-service test skipped;
92 desktop tests with one existing skip; installed pack verification and the
source-size ratchet. The first race invocation failed shell quoting, the second
could not find Go on the non-login PATH; the explicit Go-path invocation passed.
The first full renderer invocation used the wrong working directory and stopped
before running tests. Full renderer results and follow-up installation identity
are recorded separately after completion. No native inference qualification is
inferred from these fixture checks.

The full disposable-service renderer run subsequently passed **684 tests with
28 existing skips**. A fixture-only run passed 679, skipped 29 and failed four
checks that require service state; it is retained rather than reported as a
passing integration run. The final profile permission-field presentation cleanup
then passed **56 focused tests** after rebuilding the renderer. A separate
67-test presentation/journey capture run passed. Passing these checks is not a
claim that the product's information hierarchy is finished or validated by users.

The post-release Windows follow-up was installed with a new recovery backup.
All 430 current data files compared unchanged across replacement. The desktop
reopened against the original workspace; normalized renderer identity
`02109877d085f294df9f37b513bc86b0ef861c930303df13cfa7877976d6ad04`
matches the packaged compatibility manifest and the service reports a ready
desktop bridge. The original model/runtime preservation check remains separate
from this startup verification. The first launch inherited the automation
environment's `ELECTRON_RUN_AS_NODE=1`; removing that variable only in the launch
process restored normal Electron startup, without changing user settings.

The matching local container follow-up is
`offgrid-llm:local-ui-pressure-20261101`, with image identity
`sha256:88a16a2b128148924345ea80d1b9ded75dc2b55f113060f176596ce49ed69893`.
It remains stopped with the same volumes and security settings. Both the
published-release container and pre-release container remain available for
rollback. These are local follow-up builds based on v0.4.22, not replacements of
the published Git tag or assets. Service changes are in `5c8136f`; control changes
in `099b7ed` and `a38367e`; fixture isolation is in `ce3747b`.


## Whole-app order and presentation follow-up

This follow-up addresses the user's report that the app still lacks coherent
order. It changes the shared renderer and desktop startup presentation, not
inference or task authority. Management pages share a leading edge; Chat groups
occasional voice/response preferences; profiles put identity/model before purpose
and keep wide-screen save actions reachable; Models makes selection explicit and
moves maintenance into a labelled menu; Knowledge groups search/question actions;
Activity separates tasks and schedules while retaining an unsaved schedule when
switching views. Long artifact names are bounded with an exact-name disclosure.
Startup technical metadata stays available behind its existing disclosure.

Validation on the final source:

- `npm --prefix apps/web run check` and `npm --prefix apps/web run api:check` passed.
- Vite production build passed; the existing bundle-over-500-kB warning remains.
- `node scripts/test/test-web-workspace.mjs build/release-0.4.22-rebuild/followup/offgrid.exe --workers=2 --reporter=line`
  passed **689 tests, 28 existing skips** in 3.5 minutes. The wrapper uses isolated
  service state and the newly built renderer, not the installed workspace.
- `npm --prefix apps/desktop test` passed **92 tests, one existing skip**.
- Source-size, application-layout, repository-hygiene and documentation checks
  passed; documentation checked 283 maintained Markdown files with zero problems.
- `openspec validate workspace-outcome-experience --strict --no-interactive` and
  `git diff --check` passed. These are structural checks, not usability evidence.

The final fixture history-feedback p95 was 34.6 ms; measured warm route costs
were 10?37 ms. These are synthetic local-feedback measurements, not inference
latency promises or a completed historical performance comparison.

Failed intermediate runs remain recorded in local build logs. The initial
focused run passed 186 and failed 20; its next run passed 141 and failed two.
The first full run passed 669, failed 19 and skipped 28. It exposed stale tests
for relocated controls as well as real width/spacing and popover-height issues;
those were corrected. The next full run passed 687 and failed two new focus
checks. Diagnostic output showed those checks selected hidden inputs inside the
closed model picker (zero-size bounds), not obscured visible fields. Restricting
the focus check to visible inputs retained its hit-testing assertion; all six
focused layout tests and the subsequent full run passed. No assertion requiring
safe deletion, exact source scope, draft preservation or verified download was
removed. PowerShell stderr redirection reported a nonzero command wrapper for
one six-test run despite Playwright reporting all six passed; the final full
run used a Node process wrapper and returned exit code zero.

Direct screenshot review covered the wide Chat composer, model library and
profile editor; narrow Arabic Settings; long-filename task output; and desktop
startup. The broader light/dark, RTL, short/narrow and localized matrix is
captured/tested, not claimed individually human-reviewed. Startup screenshots
use an inert bridge in Chromium and do not qualify a native installation.

Remaining concerns are explicit in `docs/ux/findings.md`: 320px composer density,
technical profile concepts, advanced metadata consistency and unmeasured novice
comprehension. Native screen-reader speech, native-speaker review, real platform
startup/image journeys and the five-participant usability study remain open.
Group 19 is implemented and automatically tested; the original ten qualification
items remain unchecked. No installed desktop, running service, container, model,
workspace data or published release was replaced during this follow-up.

### Packaged startup regression and v0.4.23 preparation

CI run `41282611012` exposed a test migration omission on Windows and both macOS
runners: `test-desktop-startup.mjs` read `innerText` from version elements while
their new Technical details disclosure was closed. The actual value was empty;
the expected fixture version was still present in state. The test now asserts
that details start closed, opens them, checks both versions and the exact service
address, and closes them before continuing the original recovery scenarios.
The assertion was not replaced with a hidden-DOM check.

A newly packaged Windows v0.4.23 candidate passed the corrected complete startup
and recovery script in isolated state: first window 517 ms, recovery 590 ms.
This is one local timing observation, not a p95 guarantee. macOS must pass the
new exact-source CI run before tagging. The first local packaging invocation used
the repository root and Electron Builder rejected its hook path; running from
`apps/desktop` as CI does succeeded. No installed package was replaced.

Release finalizer contract tests passed under WSL. The initial Windows invocation
hung in its Bash bridge; only that test's identified child processes were stopped.
Targeted MCP/discovery tests passed after updating advertised version metadata.
The v0.4.23 notes retain open qualification limits; preparation is not publication.

### Exact-source CI and downloaded v0.4.23 artifacts

Source `b1c97c797fdbe287329954ea8cf429acc675a5d2` passed all eight jobs in
[CI 41283252472](https://github.com/takuphilchan/offgrid-llm/actions/runs/41283252472).
This includes both macOS architectures, Windows installation lifecycle, Linux
packaged startup/theme, Go/race, native parser, Python and renderer checks.
The Linux renderer run passed 689 tests with 28 skips, followed by six acquisition
and one restart real-service tests. Only after this exact-source gate passed was
the immutable `v0.4.23` tag pushed. Publication is tracked by
[release run 41284247749](https://github.com/takuphilchan/offgrid-llm/actions/runs/41284247749).

Independent downloads matched GitHub's SHA-256 asset digests:

| Asset | SHA-256 |
| --- | --- |
| Windows portable desktop | `0fc2e26709286a9dbff016864a7b5d15dd6e17318e370e48c43b7c842ebd0e19` |
| Windows CPU runtime archive | `5a13b0463acff2081af5955b9c323b9a74943d630ce89424b61fa978a3729e86` |

The portable desktop was extracted, not installed. Using those downloaded
artifacts, `test-desktop-native-reply.mjs` passed with Qwen2.5 3B Q4_K_M on Windows
CPU, a 2,048-token context and isolated local state. It verified one saved user
message and one matching assistant response, first-reply onboarding, restart,
image-draft retention, and unsupported-image rejection without creating a turn.
The response was `hello there` in 3,424 ms for this one run; this is not a cold-load
or latency guarantee. The model SHA-256 was
`626b4a6678b86442240e33df819e00132d3ba7dddfe1cdc4fbb18e0a9615c62d`.
The service reported `0.4.23` and renderer identity
`f74cb8bf0ff3e1f88150c66faa72694e3a714f958342102a845faef603e82117`.
Runtime dependency-manifest hashes were checked before copying/loading.

Separate local candidate checks passed packaged startup/recovery, browser control
through private IPC, saved/live theme preferences and 200% zoom. These candidate
checks are distinct from the downloaded-artifact inference run above.

Retained failure: the extended candidate invocation with BGE-M3 and `--agent`
requested 8,192 tokens and was rejected by memory admission before the initial
reply. Its 120-second response assertion expired; the UI preserved the draft and
displayed the localized memory explanation. Knowledge and agent stages were not
reached and are not claimed passed. A preceding attempt used an incorrect runtime
directory and stopped before application launch; correcting that path allowed
the basic candidate run to pass. No context/model was silently changed to turn
the extended failure into a success.

At this checkpoint, all 14 downloadable assets were uploaded with nonzero sizes
and SHA-256 digests, and the CPU container tag exposed Linux AMD64 and ARM64.
The CUDA container was still building and the release remained a draft. Final
publication/checksum/container evidence must be recorded after it completes.
The installed desktop, production workspace, models and service were not replaced.

### Authorized native installation while publication builds

The user subsequently requested installation while CI/CD continued and confirmed
quitting OffGrid. The running desktop, native service and worker were verified
closed before replacing files. A rollback copy of the previous application,
workspace data/configuration and desktop profile was retained under the local
OffGrid install-backup directory; application and saved-data copies were hashed.

The official setup download was unusually slow. Instead, Electron Builder's
prepackaged NSIS path wrapped the already downloaded, digest-verified v0.4.23
desktop payload using installer sources identical to the release tag. This was
a local installer envelope, not a claim that the official setup asset had been
downloaded or verified. Its SHA-256 was
`5eee84363bd72278dd5a00ad05858000f4de0339cb01c9946c9a34b994d40fcd`.
The superseded partial download was stopped and retained with a `.partial`
suffix; it was never executed.

The normal per-user installer exited successfully. All 611 candidate payload
files matched the installed application. All 430 saved-data files matched their
pre-install hashes before startup. Model inventory paths, sizes and modification
times were unchanged; the configured GPU worker's SHA-256 and workspace
configuration also remained unchanged. Models were not downloaded or replaced.

The installed app reopened with one desktop parent and one bundled service.
Windows registration reported 0.4.23; the service was healthy, exposed the same
workspace identity and all five installed models, and reported renderer identity
`f74cb8bf0ff3e1f88150c66faa72694e3a714f958342102a845faef603e82117`.
Installed UI compatibility and the 0.4.23 executor pack verified successfully.
This checks installation, preservation and startup; it is not a new inference or
native-control qualification run. Docker and the separately installed WSL CLI
were not changed. Public v0.4.23 publication still awaited the CUDA build at this
checkpoint.

## v0.4.23 publication checkpoint

Release workflow 41284247749 completed successfully. The release is now public
(non-draft, non-prerelease), with 15 uploaded assets. The checksum manifest's 14
payload digests match the GitHub asset digests. This is metadata cross-checking,
not a claim that all platform payloads were downloaded and executed locally.
The Windows downloaded payload checks above remain the local artifact evidence.

## Corrective pass following installed feedback

The owner reported that the installed UI still felt disorganized. Group 20 was
added without rewriting previous test evidence or closing the ten unavailable
qualification gates. Corrections cover history/deletion, profile limits and
navigation, Activity selection/alignment and owned desktop shutdown.

Initial checks: 105 focused renderer tests passed; desktop tests passed 99 with
one existing platform skip. The first full renderer run passed 695, skipped 28
and failed one Activity alignment assertion. It exposed diagnostics still using
the old centered width; the layout was corrected without weakening the assertion.

Windows isolated inference ownership tests passed:

`go test ./internal/inference -run '^TestOwnedRuntimeDiesWithWindowsService$|^TestOwnedProcessExitObservation$' -count=1`

The packaged corrective candidate passed saved/live theme, nine option-contrast
checks and 200% zoom across seven routes. Its first startup check showed the first
window in 540ms and recovery in 698ms. These are fixture observations, not timing
guarantees or live model/desktop-control qualification.

Before images are retained under ignored `build/ui-composition-before-v0423`
with source identity and hashes. New captures cover compact menus, confirmation
and profile limits in light/dark English desktop and Arabic narrow layouts;
selected captures were visually inspected. The correction is a local follow-up
to v0.4.23, not the payload of the published immutable tag.

Final corrected renderer run: **696 passed, 28 skipped**, using the downloaded
v0.4.23 Windows service and the newly built UI in disposable state:

`node scripts/test/test-web-workspace.mjs build/release-0.4.23-verify/runtime/offgrid-v0.4.23-windows-amd64-cpu-avx2/offgrid.exe --workers=2 --reporter=line`

API generation, TypeScript/build, source-size, application layout, repository
hygiene, maintained documentation and strict OpenSpec validation passed. The
existing >500 kB bundle advisory remains. Full Go/race and other-OS native suites
were not rerun for this JS/CSS correction. No claim of new model qualification.
After the alignment correction, the packaged native startup fixture passed again
(first window 674ms; recovery 894ms). Final candidate UI identity:
`19a6276aaf16721d24caf33b65a70911e8941fd6808673d34f18d39be2a000ff`.
Group 20's four corrective tasks are implemented and automatically checked;
95/105 tasks are now checked, with the same ten broader qualification gates open.

### Corrective build installation

After the owner quit OffGrid, the per-user installer completed successfully.
All 611 installed payload files matched the candidate; 430 saved-data files
matched their pre-install hashes. Model inventory paths, sizes and modification
times and the configured GPU runtime digest remained unchanged. The rollback
backup is retained outside the repository under the per-user
`OffGrid-install-backups/v0.4.23-ui-correction-20261101-175609` directory.

The first backup attempt stopped before installation because PowerShell could
not copy a long Electron cache path. A path-validated Node copy completed the
profile backup on the next attempt. Both backup directories remain retained.
Installer SHA-256:
`89aff2946ead4624c8c4bda33ecba5c638b43adba66af823572392b319417238`.

Automatic approval review blocked the subsequent installed-app launch command.
The owner opened the app normally. Live verification matched the installed UI
identity and all three entry-point assets, the same workspace and five models.
The process snapshot showed one desktop parent, three Electron helpers and one
owned service. This is the local corrective build, not a replacement of the
published v0.4.23 assets. Docker and the WSL CLI were not changed.

### Blocked conversation and model actions follow-up

The owner confirmed that the installed look improved, then reported a generic
blocked-conversation banner and oversized model actions. The saved workspace
contains an uncertain task cancelled during `skill_draft`; that write's outcome
cannot be declared resolved from cancellation alone. No saved task was mutated.
Chat's Enter path previously generated a generic error whose status action
reloaded history instead of focusing the available recovery controls.

The corrected composer derives guidance from the canonical job state, focuses
its controls on review/Enter, retains the draft and exposes a separate blank
conversation without cancellation or replay. Settled/steerable snapshots remove
the notice. Model maintenance now uses compact action rows, quiet metadata and
an explicit narrow selection indicator; confirmation and authorization remain.
This is correction of the existing continuity and management acceptance scope.

The first focused agent run passed 72 tests and failed two assertions: the new
test used the wrong localized New chat label, and repeated uncertain copy made
an existing global locator ambiguous. The test label and redundant presentation
were corrected. The combined agent/model/layout run then passed **94 tests**.
English light and Arabic narrow dark model-menu captures were visually inspected.
Build/type/API, source-size, layout and hygiene checks are recorded separately
from the full renderer and packaged startup runs below.

The initial full follow-up renderer run passed 711 tests, skipped 28 and failed
one: an old global New chat locator now matched both the header and the new
contextual action. Scoping that test to the header preserves the original
assertion. Subsequent owner screenshots exposed a double-scroll model picker
and a numeric zero rendered from an empty artifact count. The picker now has
one scrolling list with fixed search/footer, and the empty result guard is
explicitly boolean. Terminal task continuation uses a compact New chat link to
`#/chat/new`, rather than reopening the previous conversation.

The targeted picker, result, navigation and task-first suite passed **81 tests**,
including English/Arabic, light/dark, keyboard scrolling and no-mutation checks.

At the owner's explicit request, the live service's 82 saved conversations were
backed up privately outside the repository and removed through its session API
with `work=retain`. Verification returned zero remaining conversations and zero
deletion failures. Task evidence remains in Activity; models, Knowledge and
profiles were not changed. The open window's cached selection is not evidence
that the deleted server-side conversation still exists. No private conversation
content or backup is included in this repository.

The next full renderer run passed 716 tests, skipped 28 and failed one old
history assertion expecting the former Chat link label. That assertion now
expects New chat. Visual review also caught two defects not covered by the
initial geometry assertions: grid rows compressed their text, and the composite
picker could flip below its anchor using its already-clipped height. Rows now
use intrinsic content height; the picker supplies a stable preferred placement
height. Tests check visible row contents and usable list height in addition to
scroll ownership. English light and Arabic dark captures were inspected again.

The owner additionally reported the workspace chooser. It now has one access
label, a compact folder action, Refresh in the heading, and secondary guidance.
Native desktop folder selection does not show the browser's CLI-only setup
instructions. The unsandboxed-command warning appears when command access is
selected, while the actual access enum and native-grant flow remain unchanged.
The revised focused suite passed 98 tests, including grant/stop behavior.
Short-height navigation spacing was also corrected after visual inspection
showed the English Knowledge label splitting mid-word.

### Profile-first composer, Help and typography extension

The owner approved group 21 and then explicitly requested a type-size audit.
Agent settings now consolidates the read-only pinned model, revision and step
budget with existing resource/permission restrictions. It does not update saved
profiles or expand grants. Services supporting on-demand access omit the unused
folder selector until a workspace is attached; old services retain the selector.
Active selections and computer access controls remain visible.

Help is available from the top bar and Quick actions. It contains local examples,
profile/access explanations, returning-work guidance and links to existing pages.
Opening Help does not submit examples or fetch an external help site. Keyboard
dismissal restores the prior focus, background scrolling is locked while open,
and the close action stays visible while the dialog scrolls.

Computed-style checks across the six primary routes found two visible 11px
remnants: the Quick actions shortcut and Knowledge source status. These now use
caption and metadata tokens. Other legacy declarations were not blindly rewritten
where later styles already supplied the correct computed size. The new Help
prose/sections use 16px/18px and localized line-height tokens. This is fixture
coverage, not a native-speaker or screen-reader qualification claim.

The first extension suite passed 94 tests and failed one test that queried the
command search as a textbox rather than its combobox role. A subsequent focused
run passed 21 tests and failed an incorrect focus expectation: Help launched via
Quick actions returns to the original draft, not necessarily the Help button.
Both test expectations were corrected. The expanded full renderer suite passed
**733 tests, with 28 skipped**. Final copy clarification, Help scroll locking and
on-demand selector changes then passed **100 focused tests** covering Help,
typography, composer, agent access/state and navigation overlays.

The final candidate passed packaged Windows startup (1089ms initial window,
1265ms recovery) and saved/live theme checks, nine readable native option checks,
all seven route zoom checks and Help at actual 200% Electron zoom. Captures were
inspected in English light and Arabic dark; no user workspace was used by these
fixtures. Final candidate UI identity:
`f01060e4ef1473f7b33dea68fd1deda0fad319e69d5081cff3d0f696454cd164`.
API/type/build, source-size, layout, hygiene, docs and OpenSpec checks are recorded
alongside this evidence. The existing bundle-size advisory remains. No new full
Go/race, macOS/Linux native, real-model or human-participant qualification is
claimed for this renderer correction. Installation remains a separate operation.

### Installed follow-up verification

After the owner quit OffGrid, the prepared installer completed successfully.
All 611 installed payload files matched the candidate, and 349 saved-data files
matched the preservation baseline. The GPU runtime and model inventory remained
unchanged. A verified external rollback backup was retained; no backup was
deleted. Installer SHA-256:
`18ae2a78e0a3b19140c74cac1689cd8c62617b01d326d012bcd87bdb6a506bfd`.

The owner reopened OffGrid. The live system endpoint reported the final
`f01060e4ef1473f7b33dea68fd1deda0fad319e69d5081cff3d0f696454cd164`
UI identity. The installed and served HTML fingerprints matched, as did all
three referenced entry assets byte-for-byte. The workspace identity matched the
existing local workspace and the service listed five models. Process inspection
found one desktop parent, its three Electron helpers, one owned service and two
runtime children owned by that service; this snapshot does not establish a
general process-leak qualification. No task was dispatched by this verification.

This is the locally installed 0.4.23 corrective build. It does not replace the
already-published release tag or assets. Docker and WSL were not changed by this
UI installation. Human usability, screen-reader, native-speaker and other-platform
qualification remain outstanding as recorded above.

### Activity cleanup and attention-menu follow-up

The owner requested removal of all old activity after observing that conversation
deletion had retained the canonical jobs. Before mutation, all 284 listed jobs
were exported into a verified private external backup. Existing cancel/delete
APIs stopped 16 old waiting/interrupted jobs and removed 277 job records, with no
request failures. Subsequent reads returned zero conversations and seven jobs in
both canonical and diagnostic activity APIs. The seven remaining records have
unknown effects, including unconfirmed command-stop receipts. They were not
falsely reconciled or deleted by direct database editing. A proposed explicit
discard-history extension is awaiting the owner's scope confirmation; current
service deletion restrictions remain intact.

The presentation follow-up replaces oversized attention-menu buttons with
two-line links and secondary status, bounds the panel to 440px with one inner
scroll region and a reachable history link, exposes existing removable-history
cleanup outside the overflow menu, and makes Show more content-sized. Response
settings replaces the ambiguous Context & response label and sun-like icon;
context allocation/token controls are disclosed under Advanced without changing
their values. Knowledge and voice controls remain accessible.

Initial focused coverage passed 80 tests and failed three expectations: a
label-based selector for the native context combobox, the former hidden deletion
control, and a regression removing known completed conversations from the work
menu. The two selectors were corrected and the conversation links restored.
The revised focused suite passed 32 tests. Full-suite follow-up and any later
installation are recorded separately; this source correction is not proof that
the seven protected records have been removed.

The full renderer run passed 724 tests, skipped 28 and failed 12 stale
expectations: three exercised the former overflow-menu cleanup route, and nine
looked up the former context-label translation. After updating only those test
expectations, all 51 tests in their three affected files passed. No application
source changed after that full run. API generation, TypeScript/build, docs,
source-size, repository hygiene and OpenSpec checks passed; the existing bundle
size advisory remains. The compact attention capture was inspected. These later
presentation edits are not part of the installed `f01060e...` build yet.
