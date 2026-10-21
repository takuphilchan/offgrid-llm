# Use applications and browsers in agent tasks

Computer Tasks connects a saved agent task to a locally consented application or managed browser. It is a preview: the implemented controls and tests do not establish universal desktop automation or reliability on every model and OS.

## Start with an outcome

New task-first native workflows record outcome checks before making changes.
The **Outcome checks** panel separates pending work, unavailable verification,
partial results and passed declared checks from the task's execution status.
Checks keep outcome expectations separate from access:
they are model interpretations (or explicit caller checks), not proof that every
part of the request was understood. Only authenticated user steering may revise
them; documents, pages and tool results are not user corrections.

Checks are declared in stages: discover the application, record its open check,
open and observe it, then add text/checkbox checks using the actual controls
before editing. Users do not need to supply window IDs or tool JSON. Invalid
declarations return field-specific guidance and share the three-repair limit;
repeated invalid metadata cannot keep the task looping indefinitely. A pause
preserves the task and does not replay its actions.

Text and checkbox edits cannot proceed with only an application-open check:
OffGrid enforces a check for the actual control and proposed value before sending
the edit to the host. Missing checks produce corrective feedback, not another
access popup. Full task access does not bypass this check or independent read-back.

The current verifiers establish application readiness, exact control text, or
checkbox state. Text in an editor does not prove a saved file; opening a browser
does not prove completed research. A matched check does not certify that the
model interpreted every part of your request. Older tasks are not retroactively
certified. Older hosts without cross-observation identity report control checks
as unavailable; expired observations still cannot authorize input.

The agent can search and inspect controls omitted from its compact view without
refreshing or controlling the application. Results show source truncation and
age. Windows UIA, macOS Accessibility and Linux AT-SPI share these evidence
contracts; this does not add task-wide launching, capture or Wayland input to
platforms that do not implement them. Installed OS workflow qualification remains
separate from shared-format tests.

1. Open **Agents** in the matching desktop app, describe the result, choose the computer approval policy beside the model, and choose **Start task**.
2. With the matching Windows x64 app, **Whole desktop for this task** and **Full task access** are proposed by default. Open **Computer access**, select **Full task access**, and confirm once for this desktop session. This is one choice, not a policy row plus a second Enable button. The row is checked only after the host confirms permission; loading a saved default is never consent. Starting a full-access task also offers this consent if it is not enabled yet.
3. OffGrid continues the same task with the saved policy and fresh observations. There is no mandatory application picker in this flow. **Selected window only** remains an optional restriction; other platforms currently use this narrower compatibility flow.
4. Inspect activity and any requested approvals. Verify the actual saved result, not just the model's completion message.

Web users can choose **Continue in desktop** for the saved task. The desktop must use the same workspace; a link does not grant permissions or switch services silently. Normal packaged setup does not require Node, terminal commands, manual pairing codes, or an expected-page-text field.

A Linux service in Docker/WSL can coordinate work, but it cannot directly control your Windows desktop. The installed host components supply that boundary. Updating the container does not update those components; see [installation and upgrades](../setup/installation.md#update-an-existing-installation).

Task-first work keeps its permitted connected tools when application or browser
access is attached. For example, a task can consult an MCP documentation service
and then inspect an application in the same saved run. MCP calls retain their
own tool policy and approvals; **Full task access** for a computer session does
not approve those calls. Service file tools still operate on the service's
filesystem, not your desktop. Legacy computer-only runs retain their narrower
tool set. This composition does not yet add tools to ordinary Chat turns.

## Choose the right target

| Target | Implemented boundary | Important limit |
| --- | --- | --- |
| Whole desktop for this task (Windows x64 preview) | One local task grant; OS-backed app discovery, recorded launch, window switching and structured controls | Validated zero-argument GUI registrations, 60 minutes/500 changes; no arbitrary commands, native vision, or direct filesystem grant |
| Existing application window | Structured accessibility observations and supported typed edits, selections, activations, and fixed shortcuts | Only controls the driver can address and verify; not unrestricted pointer/keyboard input |
| Existing browser window selected as an app | The native application's exposed accessibility controls | Not attachment to a personal browser profile through Playwright; support varies by browser and page |
| Managed browser | Separate bundled Chromium session, constrained navigation, structured controls, and governed transfers | No personal profile import; cross-origin/authentication workflows remain limited |
| Native vision | Not implemented as general native control in this checkout | Do not infer availability from a vision model or browser screenshot feature |

Native workers exist for Windows UI Automation, macOS Accessibility, and Linux AT-SPI. Linux X11 shortcuts and Wayland portal input are not interchangeable; general Wayland capture/input remains unfinished. All native profiles still need the recorded workflow qualification gates. See the [driver documentation](../../apps/computer/README.md) and [quality evidence](../quality/README.md).

## Approve changes at the right level

| Policy | Behavior inside the selected scope |
| --- | --- |
| Ask every time | Require exact-action approval for changes |
| Approve scoped changes | Automatically permit only supported, locally classified reversible changes; ask for consequential or ambiguous actions |
| Full task access | Permit available typed actions, including supported consequential operations, within the approved scope |

Full task access is not unrestricted OS access. Credential/payment entry, financial transactions, privilege changes, installation, arbitrary scripts, permanent deletion, and replay of uncertain actions remain blocked. The selected policy is fixed for that session; a model cannot broaden it. Unknown-effect controls are not automatically classified as harmless.

The updated composer proposes **Full task access** before Start. The selection
is saved with the task but grants no computer access by itself. Local consent
still applies. Older tasks without a recorded policy retain the compatibility
selector, initially **Approve scoped changes**. A session with a different
policy cannot attach to a new task. Permissions do not make unsupported controls
work, and changing reasoning style does not repair invalid tool calls.

Task-wide access requires both the v3 Windows host and matching service. A
container update alone cannot add it. Existing narrow grants are never widened;
an older saved app-access request can explicitly obtain a new task grant.
Once explicitly enabled, Full access is reused for subsequent tasks in the same
account and workspace until Stop/revocation, quit, screen lock, suspend, or an
account/workspace change. It lives only in the native host's memory; saved
preferences cannot restore it after restart. Each task still receives a separate
60-minute/500-change grant. Switching to a narrower policy revokes session Full
access. Older desktop packages retain their per-task consent until updated.
The Windows catalog reads per-user/machine App Paths and Start Menu registrations
for ordinary GUI executables, including custom installation locations. It also
reads Windows AppsFolder and current-user package registrations for supported
packaged GUI applications. Package identity includes the exact application entry
and package revision, not just the product name or package family. Activation
uses the OS registration with no supplied command or arguments. A package update
invalidates an earlier prepared launch; refresh discovery before continuing.

Windows validates each registered package path using its opened object's final
path. This avoids requiring directory-list access to the protected WindowsApps
parent while still rejecting path redirection. No folder permissions are changed.

Readiness independently checks process identity, the signed-in desktop session
and an observable application surface. Reusing an existing process is supported;
that does **not** mean its current document is new or blank. Checks wait at most
ten seconds and never launch, focus or type. Unavailable surfaces, unsupported or
changed identities and unavailable observations report distinct readiness reasons.
Do not relaunch an application merely because a readiness check failed.

Argument-bearing shortcuts, command wrappers, hosted/DLL-only package entries,
unverifiable shared surfaces and unproven redirectors remain unavailable. There
is no executable-basename or Notepad-specific mapping to another process. Package
support does not expand a selected-window grant or add desktop launching on macOS
or Linux. Native document/tab creation and range-based editing remain separate
work; catalog tests alone do not qualify them. See the
[profile limitations](../../apps/computer/README.md#task-desktop-authority-protocol-3-windows-x64-preview).

Developers can run catalog-only validation using
`OFFGRID_TEST_APPLICATION_DISCOVERY=1 go test -v ./internal/computer -run TestInstalledApplicationCatalogReadOnly`.
It does not activate apps. To assert that a particular installed package is
included, use the read-only test below in PowerShell (the package must already
exist; this does not install or launch it):

```powershell
$env:OFFGRID_TEST_DISCOVERY_AUMID = 'Microsoft.WindowsNotepad_8wekyb3d8bbwe!App'
try { go test -v ./internal/computer -run '^TestRegisteredPackageDiscoveryReadOnly$' -count=1 }
finally { Remove-Item Env:OFFGRID_TEST_DISCOVERY_AUMID }
```

Real package-activation tests are separate: use a
disposable signed-in `OffGridTest-*` Windows account, set
`OFFGRID_NATIVE_ISOLATED_DESKTOP=1`, `OFFGRID_TEST_NATIVE_WINDOWS=1`, and
`OFFGRID_TEST_NATIVE_PACKAGE_APPLICATION` to the exact OS-registered AUMID of the
test application, then run
`go test -p 1 -v ./internal/computer -run TestNativeWindowsRegisteredPackageActivation -count=1`.
The test refuses other accounts, activates once, reads identity/readiness twice,
and tests journal replay without another activation. It does not type, save,
discard documents or terminate the application. This test is not the full
document-writing acceptance suite and must not run in a personal desktop session.

The owned Windows document fixture is a separate test target, not Notepad and
not an installed-application qualification. In that same disposable account, with
the two isolation/native opt-ins above, run
`go test -p 1 -v ./internal/computer -run TestNativeWindowsDocumentFixture -count=1`.
It builds a temporary executable and addresses only its exact child process,
process generation, executable identity and child controls. Its baseline is two
unsaved synthetic documents: `Synthetic lifecycle content` and
`Untouched — café 日本語 12345`. Both tabs intentionally have the title `Untitled`
and share one editor control. Creating a delayed third blank tab and changing its
Unicode text must leave the first two documents unchanged. Private fixture-only
commands configure selection, focus loss and partial input; they are not exposed
by the production companion. Closing the test terminates only that owned process.
The fixture has no file saving, network access or personal profiles. Its contract
test does not exercise native-driver input; the full native workflow suite must
also verify actual UIA creation, editing, cancellation and independent read-back.

Equivalent owned GTK and AppKit targets use the same two synthetic baselines,
duplicate tab titles and reused editor, with private test-only pipes. Their
reported selection offsets are native scalar offsets on GTK and UTF-16 offsets
on AppKit; the production document contract still uses Unicode scalar offsets.
The test selects only the exact unreaped child process discovered by the native
provider, validates its generation and session on every observation, and refuses
missing native permissions rather than reporting a successful native run.

For Linux, install the development dependencies listed in `apps/computer/README.md`
on a disposable test machine, then run
`OFFGRID_NATIVE_ISOLATED_DESKTOP=1 xvfb-run -a dbus-run-session -- bash scripts/test/test-native-linux.sh`.
The runner builds both GTK fixtures into its private temporary directory and
uses a separate configuration, X11 display and session bus. Xvfb validates neither
real Wayland portal permissions nor installed-application compatibility.

For macOS, sign into a disposable `OffGridTest-*` account with Xcode command-line
tools and Go installed. Grant Accessibility to the test host through System
Settings, then run
`OFFGRID_NATIVE_ISOLATED_DESKTOP=1 bash scripts/test/test-native-macos.sh`.
The runner does not alter TCC or request privilege elevation. Permission or
provider failures are failures, not passing skips. Intel and Apple Silicon must
be tested separately. These fixture checks are not installed-editor, provider
edit or real-model qualification; those remain required before support claims.

Chrome may initially expose only its window frame while accessibility starts.
The Windows host briefly re-observes for page controls; after a typed change it
waits for read-back without repeating the input. A profile chooser is not a
search result. Personal profiles are not selected implicitly, and public browsing
can use an available Guest option; tasks needing a particular account need your
direction. Unsupported or incomplete page controls remain a limitation.

Desktop steps reserve at most 1,024 output tokens per model response, keeping
space for tools and current observations. Earlier completed exchanges are archived
intact and remain retrievable. A context-limit failure retains recorded effects;
it does not mean an earlier launch or edit never happened.

Large native control trees use a bounded model view with marked omissions and
paged detail retrieval; original captured evidence stays saved. Writable controls
are prioritized. Supported runtimes measure the actual prompt before generation
instead of rejecting it solely from a JSON-size estimate. This does not increase
the installed model's context allocation or guarantee that every page fits.

Slow inference is separate from context capacity. The Windows desktop can freshly
recheck an expired structured observation before preparing the next action, but
only unchanged complete views qualify. Changed controls still require a new
observation. This does not bypass access rules or retry uncertain input. CPU model
responses may still take minutes; installed-model latency is not a UI delay.

## Try a bounded task

### Document support and qualification

Document workflows are being implemented behind negotiated `native_documents:1`
and workflow-evidence version 2. Installing a new service does not, by itself,
enable these operations in an older companion. Existing control-text operations
keep their original semantics; they are not silently converted into document
insertion. Missing capabilities must be shown as unavailable.

| Operation | Windows UIA | macOS Accessibility | Linux AT-SPI |
| --- | --- | --- | --- |
| Registered application discovery/launch | Executable and packaged identities; native acceptance required | Not supplied by this document change | Not supplied by this document change |
| Selected-window controls and supported whole-value replacement | Existing preview | Existing preview; Accessibility permission required | Existing preview; native build/session required |
| Distinct document/tab inventory and selection | Negotiated UIA relationship-based preview | AXTabs/AXContents provider implemented; macOS build/native checks unverified | Retained AT-SPI objects; owned GTK inventory/read fixture passes on isolated Xvfb |
| New blank document | Exact packaged Notepad profile below; native acceptance unverified | Reviewed creation profiles not implemented | Reviewed creation profiles not implemented |
| Exact document insertion and selection-only replacement | TextPattern/Unicode input preview; native acceptance unverified | Settable AX range/text/value operations implemented; require explicit composition evidence; unverified on macOS | Text/EditableText operations implemented, but unavailable without a reviewed composition provider |
| Document-aware verification and recovery display | Shared v2 contracts and current clients; requires matching provider | Same shared contracts, not platform qualification | Same shared contracts, not platform qualification |

No installed editor/app-version range is currently qualified for the new
document workflow. In particular, modern packaged Notepad, another installed
editor, and disposable browser tabs still need the full native acceptance run.
The Windows owned fixture, AppKit fixture and GTK fixture are test targets, not
substitutes for those installed applications. macOS Intel and Apple Silicon,
Linux X11 and Linux Wayland each need separate results. Passing compilation,
contract tests or Xvfb checks does not qualify another desktop environment.

The initial Windows creation profile is limited to AUMID
`Microsoft.WindowsNotepad_8wekyb3d8bbwe!App`, package
`Microsoft.WindowsNotepad_11.2607.14.0_x64__8wekyb3d8bbwe`. It requires the
observed WinUI tab group, selected tab, content presenter and enabled AddButton
in the expected relationship. A package update or different layout is not
automatically compatible. Invocation is attempted once and must produce a
distinct, independently observed blank document; it is not a successful result
merely because the button accepted input. This implementation profile has not
passed the installed Notepad test. Browser creation profiles are still missing.

Negotiating the document protocol does not imply support for every operation.
AX input requires explicit marked-text state plus the appropriate settable
attributes. The generic AT-SPI provider reports composition as unknown and does
not offer document edits or creation; a reviewed toolkit profile is still
required. OffGrid does not interpret a missing composition API as safe to type.

Document reads are limited to complete 64 KiB UTF-8 snapshots and revision-bound
4 KiB pages. A single input is limited to 16,000 UTF-8 bytes. A truncated preview,
input echo, successful click or matching text in another tab cannot establish
success. A concurrent edit, unstable composition or unsupported range mapping
requires a fresh observation or manual takeover, not a fallback input method.

### Isolated document-workflow runner

Use a disposable `OffGridTest-*` account on the corresponding OS, with Go and
the documented native development dependencies already installed. This stricter
runner refuses personal accounts even for the owned fixture. It creates fresh
service/task storage on an ephemeral loopback port and never connects to port
11611 or a saved task. It does not download models, select a personal browser
profile, save documents or reconcile an existing uncertain task.

On Windows, set `OFFGRID_NATIVE_ISOLATED_DESKTOP=1` and
`OFFGRID_TEST_NATIVE_WINDOWS=1` **in that isolated account**, then run:

```powershell
node scripts/test/test-native-document-workflow.mjs --platform windows --suite fixture --repeat 3
```

On macOS, use the same isolated-account and Accessibility setup described above:

```sh
OFFGRID_NATIVE_ISOLATED_DESKTOP=1 node scripts/test/test-native-document-workflow.mjs --platform macos --suite fixture --repeat 3
```

On Linux, use the isolated account with a private X11 display/session bus and
the GTK/AT-SPI development dependencies. Start a window manager within that
disposable display, then run:

```sh
OFFGRID_NATIVE_ISOLATED_DESKTOP=1 node scripts/test/test-native-document-workflow.mjs --platform linux --suite fixture --repeat 3
```

`fixture` runs the owned native fixture checks only. The runner also accepts
`--suite installed` and `--suite model --model Qwen2.5-7B-Instruct-Q4_K_M`;
these require their full provider/installed-model test entry points. Until those
are implemented, the runner reports `required_native_test_missing` **before
input**. An empty Go test match or skipped permission check is never a pass.
Even when those entry points exist, the installed/model suite must emit one
closed, redacted workflow record asserting distinct creation, exact Unicode
read-back, unchanged other documents, no save, verified blank browser tab and
single dispatch. Each suite must cover at least two application families.
A Go pass event without that record is insufficient; the record itself is a
test-oracle report, not independent human or platform qualification.
Do not describe this as full workflow verification. Wayland is never routed
through an X11 shortcut fallback.

Standard output is a JSON report; progress/evidence location goes to stderr.
Failure or unavailable prerequisites exits with 2 and cancellation with 130.
Evidence contains pass/fail metadata and timings, not application text, service
logs, prompts, screenshots or credentials. Temporary test state is retained for
inspection; only owned child processes are stopped. No current container or
installed desktop application is replaced by this runner.

Once the full installed suite is available, run this exact unsaved-document
scenario three times per claimed OS/app/model profile:

> Open Notepad. In a new blank document, type exactly: OffGrid test — café 日本語 12345. Read it back and confirm it matches. Do not save or change existing documents.

Use the equivalent explicitly named editor on non-Windows platforms. Prepare
unrelated synthetic unsaved tabs in the disposable account first and verify
their exact baselines afterward. Test selection-only replacement separately:

> In the new test document, select only “café”, replace it with “tea”, then read the entire document back. Confirm the prefix and suffix are unchanged. Do not save, close, or edit any other document.

If a run says **Outcome unknown**, do not rerun its input to obtain a green
status. Inspect the disposable target and preserve the recorded action. Missing
provider support, verification failure and uncertain input are different results.

If manual reconciliation is necessary, describe only what you actually observed.
The saved report is labelled human-reported and retains the original action;
it does not automatically verify other task checks. `Done` alone is not evidence
that the requested text matches. Known pre-execution argument/path rejections
use bounded repair instead of asking you to report an unknown mutation.

Use a disposable document or draft, not valuable unsaved work.

**Application example:**

> Open Chrome and verify that its application window is ready.

This uses installed-application discovery, not a managed-browser URL or manual
launch. For editing, use a disposable supported editor:

> In the selected text editor, replace the draft with a short grocery list containing bread, rice, and tomatoes. Read it back and confirm all three items are present. Do not save over an existing file.

**Public browser example:**

> Open the Writing tests page from the selected Playwright documentation page. Summarize the basic test structure and give the source URL. Do not submit forms or download files.

Start the browser at `https://playwright.dev/docs/intro`. Site and model behavior can change; these are suggested tests, not guarantees. A driver test on the local practice page isolates control problems but does not establish usefulness or planning quality on arbitrary websites.

## Files and vision

Managed-browser uploads require a locally selected file and approved destination. Downloads use verified staging and explicit host saving; they are never automatically executed. Native application control does not supply general folder operations or automatic file transfers.

Managed-browser viewport images can be used only with compatible installed model/projector components and applicable preflight. This does not implement native-window screenshots, general visual grounding, or a qualified vision profile. Structured observations remain the first choice. Do not use this preview on sensitive pages or assume redaction detects every secret.

## Stop and recover

Use **Pause**, **Take over**, or **Stop** in the task as appropriate. Take over revokes that task's computer session; the local emergency stop also works when the service cannot be reached. Stop is not undo: already dispatched effects may finish and must be inspected.

The updated Windows desktop uses the existing persistent in-app **Stop control**,
File menu and tray, not a second floating Stop window. The in-app control remains
available between tasks while session Full access is enabled and across page
navigation. Local Stop works without the service. **Ctrl+Alt+Shift+F12** is an
independent emergency shortcut; a conflicting shortcut prevents control from
starting. A crashed/unresponsive workspace renderer revokes host access. Standalone
workers and older/non-Windows compatibility hosts retain their own stop surfaces.

Reloading a page restores the saved task, not a new execution. Service restart invalidates its task sessions; never replay old grants or uncertain actions. If desktop-session permission was revoked, explicitly enable it again. Task sessions remain bounded; finish or renew access when the allowed budget expires.

If the result is **Outcome unknown**, inspect the affected target before reconciliation. Never retry a possibly submitted form just to clear the status. See [task recovery and deletion](agents.md#restart-and-uncertain-outcomes).

## Diagnose access problems

- **Invalid access proposal:** a missing managed-browser URL is a model proposal error, not an OS permission denial. Opening an installed app such as Chrome uses application access with no starting URL. OffGrid returns corrective guidance without granting access; local consent is still required for a valid request.
- **Desktop/service mismatch:** update both to the same product version and UI build; retry afterward.
- **Runtime unavailable:** repair the matching desktop package; replacing the container alone cannot repair host files.
- **Model check failed:** choose a suitable tool-calling model/runtime. A synthetic preflight pass still does not guarantee task planning.
- **Model check timed out:** model loading and the synthetic two-call check now
  have separate budgets. A slow/cold runtime is not automatically incompatible.
  Retry the same saved task after loading, or choose a faster tool-calling model;
  do not submit a duplicate task. Cancellation leaves the access request pending.
- **Stale observation or changed window:** inspect again; do not bypass target or focus validation.
- **VPN fake DNS blocked:** Direct mode rejects reserved addresses. Explicit **Trusted VPN routing** supports its documented fake-DNS case while retaining selected-origin/TLS restrictions; it is not unrestricted proxy support. See [network limits](../../apps/computer/README.md).

For internal protocol and native-worker development, use [apps/computer/README.md](../../apps/computer/README.md). Do not interpret developer pairing commands as the normal user setup flow.
