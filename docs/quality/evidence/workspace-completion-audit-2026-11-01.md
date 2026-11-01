# Workspace experience completion audit — 2026-11-01

The implementation and release candidate are not proof that the entire experience
program is complete. `workspace-outcome-experience` remains **83/93 tasks**. This
audit preserves its original acceptance criteria rather than replacing them with
the checks that happen to be available.

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
