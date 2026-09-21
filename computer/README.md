# Computer Tasks browser preview

## Native-control foundation (not native availability)

The service no longer instantiates the legacy blanket-approved computer
controller. Browser protocol 1 continues through the durable agent authorization
path. `internal/computer/protocol_v2.go` defines the next typed driver contract,
opaque target identities, bounded-step digests, local-consent binding and private
length-prefixed worker framing. Its tests reject changed values/targets, expired
grants, mixed consequential actions, oversized frames and executable strings.
The native worker implementations below use this contract and a host-local
supervisor. They are development components, not a qualification claim. Native
control is not exposed through Agents until the shared client/service cutover and
required platform checks are complete; vision remains unimplemented.

The host dispatch journal now saves a result before acknowledgment. An identical
redelivery in the same session can return that recorded result; changed arguments
or session/workspace binding are rejected. A journal entry without a recoverable
result remains uncertain and cannot execute again. Closing a session clears its
result payloads but retains duplicate-action tombstones. After an unclean exit,
starting a new session clears previous-session result payloads. The journal stays
on the host and must not be restored alongside a service backup.

The journal change is included by desktop runtime packaging; rebuilding only the
service/container does not update an installed companion. No personal browser
profiles are imported. Direct mode blocks VPN fake DNS; explicit trusted-VPN
routing is described below. Configurable HTTP/SOCKS proxies and native/vision
setup are still pending.

This is a **managed-browser preview**, not the completed cross-platform Computer
Tasks program. It controls a separate Chromium browser on the local host. It does
not control native applications, use vision, attach to personal browser profiles,
enter credentials, or download files. All drivers remain **unqualified** until
the full task/model/platform evaluation gates pass.

## Installed desktop: no terminal setup

Updated desktop packages include the matching Playwright/Chromium runtime. In
**Agents**, describe your task, enable **Use a browser**, and enter a public HTTPS
page URL or choose **Try a practice page**. Paths, query strings and fragments
are supported; embedded credentials are not. Approve the native consent prompt. OffGrid
starts and pairs its own companion; no Node installation or copied code is needed.
Choose a tool-capable model and run the task. Each proposed change still requires
your approval; removing technical setup does not remove consent.

Text fields, native single-choice dropdowns, and native checkboxes have typed
actions. Dropdown options come from the current observation; checkbox actions
specify checked or unchecked rather than toggling blindly. Approval cards show
the observed field/option names and the proposed state. Disabled controls,
unsupported custom/multiple-choice widgets, stale observations, malformed
arguments, and credential fields are refused. Field/selection verification is
not task completion: inspect the saved page result as well.

For a longer practice task: "Set Report title to Household budget, choose Table
as the Report format, include sources, and save the draft. Confirm the saved
title and options." This changes only the practice page, not a saved file.

The app's **Stop browser** control and tray/application menu stop the owned
companion locally, even if the service cannot be reached. Screen lock, suspend,
and quitting also stop it. A closed browser is not a rollback of effects already
dispatched. Each session still lasts at most ten minutes and one task. Stop a
finished session, then open another with fresh local consent.

If the companion fails to confirm shutdown, OffGrid reports that uncertainty,
retains ownership to prevent a duplicate session, and still attempts service
revocation. Close the separate browser manually and inspect the task before
retrying. A service outage does not prevent the local stop path from running.

The web edition provides **Open desktop app** (`offgrid://computer`). This only
opens Agents: the link cannot carry a service URL, credentials, or an action.
The desktop must be connected to the same matching workspace (the ordinary local
container address is `127.0.0.1:11611`). A browser page alone cannot control the
host. Remote service attachment and silent workspace switching are not supported.
Developer/manual pairing remains collapsed for CLI users and source checkouts.

The browser is bundled rather than downloaded on first use. This adds its size
to the desktop package but permits offline practice-page startup. A damaged or
missing runtime fails closed: reinstall the matching package, not an arbitrary
download. Separate signed optional-pack installation/repair is still pending.
For a trusted VPN using fake DNS, expand **Network settings**, select **Trusted
VPN routing**, then confirm the trust warning locally. Direct mode remains the
default; a network failure never switches modes automatically. Keep required VPNs
enabled. This permits only the selected hostname's `198.18.0.0/15` fake-DNS
mapping through the existing VPN, not arbitrary private-network destinations.
OffGrid cannot independently verify the routing hidden by that VPN. TLS hostname
and certificate checks remain enabled. Explicit authenticated HTTP/SOCKS proxy
configuration, IPv6-only sites, and consented cross-origin resources are not yet
supported; some sites will remain incomplete or unavailable.

One useful read-only test with the updated host runtime is to open
`https://playwright.dev/docs/intro` and ask: "Find the Writing tests documentation,
open it, and explain the basic structure of a test. Include the source URL."
The companion has been tested opening that real page, discovering its link,
navigating and checking the resulting text through the local VPN. This is driver
evidence, not proof that every local model will plan that task successfully.

## Developers and CLI: try it from this checkout

Prerequisites: Node.js 22.13 or newer, a graphical desktop, and the updated OffGrid
service reachable at a loopback address. Use the Windows terminal for a Windows
browser, not an Alpine container. Linux needs Playwright's documented Chromium
system dependencies. Never expose the Docker socket or mount a host display into
the service container.

```sh
cd computer
npm ci --ignore-scripts
npm run browser:install
npm start
```

In OffGrid **Agents**, enable **Use a browser**, open **Developer connection**,
then **Pair browser**.
The companion asks for the local service URL (press Enter for the default), then
a browser target. Enter a public HTTPS page URL, or `demo` for an isolated practice
page. For a public site choose `direct` (default) or explicitly `trusted-vpn`.
After local consent,
service/network/browser preflight completes **before** asking for a pairing code.
Only then click **Pair browser** to generate a fresh code. Select the paired browser in OffGrid, choose a
tool-capable model, and submit a task. Start with a bounded inspection such as
finding a heading and verifying its text. Review exact arguments before approving
navigation, clicking or typing. A browser action succeeding is not proof of the
entire task's correctness.

Starting a task automatically checks the selected model. For troubleshooting,
**Advanced settings → Check selected model** runs the same two synthetic tool
turns through the selected runtime without reading or controlling your browser:
an observation call followed by a verification call using a generated heading.
A prose simulation, malformed call, wrong tool, truncation or incorrect result
does not pass. The check may load the model and takes up to 90 seconds; you can
cancel the optional diagnostic. Changing models resets its result. The service checks on
submission, so clients cannot bypass it; a rejected check creates no task.

The CLI equivalent is `offgrid computer check <model>` (JSON result, nonzero exit
on failure). Runtime build, template digest and context accompany the UI/API
result when available; a passed smoke check is **not** task qualification. No
success is cached across model/runtime changes. A failed check is not fixed by
changing reasoning style or simply enlarging context. Choose a model/template
that actually supports tool calling, then retest; OffGrid never silently replaces
the model's chat template or downloads a different model.

Computer tasks use temperature zero and a sequential observe/action protocol;
normal agent sampling is unchanged. The probe is intentionally narrower than a
real task. In local testing, Qwen2.5 3B passed the probe with `q8_0` K/V cache but
still invented observation IDs or chose the wrong action order in the demo.
It is **not qualified** for browser tasks. Do not approve invented target IDs or
unexpected changes just because the model check passed. See the reliability plan
for the exact profile and failed attempts. Low-precision inference caches can
also affect output quality; test the actual runtime configuration rather than
assuming a tool-support declaration guarantees usable output.

For the demo, enter the task: "Set Report title to OffGrid test and save the
draft." Approve each proposed change. No verification configuration is required:
the model selects task-relevant page evidence and the companion checks it.
There is no expected-text field in the task UI. Retired expected-text drafts are
cleared for the current account/workspace and are never submitted, even if an old
tab recreates them. Task drafts and historical runs are preserved. Developer
API/CLI tests may still supply an exact final criterion; intermediate read-only
checks may use different text without satisfying that strict final requirement.
This updates only the temporary page, not a file or external service. The demo
owns a new loopback server on a random port and closes it with the browser; it
does not enable arbitrary local URLs or bypass public-site network checks.
It works without public DNS and keeps your VPN enabled.

For the CLI, `offgrid computer pair`, `targets`, `status`, and `stop` use the same
authenticated service. `offgrid computer run <session> <model> "Set Report title to OffGrid test and save the draft"` submits a
durable task. Optionally put `--expect "Draft saved: OffGrid test"` before the task
for a strict final check; existing `offgrid agent` controls handle inspection/approval.

The connected-browser strip remains available across workspace pages. Consumed
sessions are shown as assigned/finished rather than offered for new work. Stop
the old session and restart the companion for fresh local consent before another
task. Session time/action limits have not been removed. The source-installed
companion requires a terminal; the updated desktop owns this lifecycle instead.

Keep the companion terminal open. **Ctrl+C closes its browser locally.** OffGrid's
Stop browser button revokes the session and returns `stopping` while the host
receives revocation; it does not claim rollback of an already dispatched effect.
The host polls to renew liveness and closes on lost service access. A browser
failure or uncertain result stops the session. Reconcile the saved task before
starting new work; never blindly repeat a click.

## Boundaries

- A pairing permits one task, 100 actions and ten minutes. Pair again for another
  task. Codes expire in two minutes; tokens stay only in host/service memory and
  are invalid after restart. There is no persistent credential enrollment yet.
- Network requests are restricted to the selected origin. A loopback HTTPS
  CONNECT relay checks the exact hostname and port and pins its destination;
  this covers redirect escapes that browser route callbacks alone miss. Direct
  mode rejects private/reserved DNS results. Trusted-VPN mode additionally permits
  the consented fake-DNS range, with the disclosed routing trust boundary above.
  Plain HTTP, unapproved origins, file URLs, service workers and WebSockets remain
  blocked. Blocked resource origins are reported without request paths, cookies
  or credentials. Closing the browser tears down owned relay sockets.
- Structured observations are untrusted page data. Password/credential controls
  are excluded, field values are not collected, and screenshots are not captured.
  Page text may still be sensitive: approve only pages intended for OffGrid.
- Exact action approvals cannot be supplied by the model. Stale observations are
  rejected. GUI effects cannot be sandboxed perfectly; don't use financial,
  administrative, security-sensitive or account-management pages in this preview.
- Dispatch IDs and argument hashes are journaled in the local user's
  `.offgrid-llm/computer/dispatch.sqlite`. Results can contain observed page text
  and are retained during the session for safe reply replay, then cleared as
  described above. Identical completed action IDs return saved replies, never
  repeated input; uncertain actions remain blocked.
- OffGrid requires an affirmative final `browser_verify` result matching the
  actual check arguments, after any changes. If an optional strict criterion is
  supplied, the final check must match it. In automatic mode, an unchanged initial
  heading cannot establish a mutation's success. This conservative guard can also
  leave idempotent changes unverified; inspect the outcome instead of blindly
  retrying. The model still interprets the task: page evidence is not independent
  semantic proof of every requested step, a saved file, or an external transaction.
  Legacy tasks lacking both a criterion and the new verification policy remain
  incomplete; their outcomes are not silently reinterpreted.

Tests: `npm test` exercises a real isolated Chromium fixture. It is not evidence
that any particular local model can plan a complete task. Native Windows/macOS/
Linux workflow qualification, vision, signed/offline automation packs, persistent OS-keystore
pairing, full pause/takeover UX, and the 30-case qualification suite remain pending.

### Native Windows worker development

`cmd/offgrid-computer` now implements a Windows x64 host process with UI
Automation target discovery, selected-window observation, exact approved text
replacement and activation. It uses direct Go COM bindings to Windows UIA, not
shell commands or simulated keystrokes. This replaces the proposed C++ worker
implementation for this slice while retaining process isolation. It is **not yet
connected to the Agents task/session APIs or offered as a native UI option**.
The existing browser experience remains unchanged.

The worker uses private framed pipes, local Yes/No consent, a visible Stop window,
and Ctrl+Alt+Shift+F12. An independent watchdog terminates a hung owned worker
after revocation; it never terminates the target application. Structured password
controls are excluded, but ordinary control labels can still contain sensitive
data. This is not a guarantee that all secrets are detected. There is no native
screenshot/vision path or arbitrary keyboard/shell interface.

Dispatch intent/results are recorded in a separate **host-local execution
journal**, using the existing SQLite connection policy. It is not another task
history. Never restore that journal from an older workspace backup: unresolved
dispatches must remain uncertain, and action IDs must never repeat input.

Windows packs include the worker and its file digest. This establishes bundle
integrity relative to the existing trusted package, not publisher signing or
native workflow qualification. General target scope qualification, enrollment,
service integration and end-user native setup remain
unfinished. Do not expose this internal worker as general-purpose computer use.

Developer validation (opens and edits only disposable test applications):

```powershell
$env:OFFGRID_TEST_NATIVE_WINDOWS = '1'
go test -p 1 -v -timeout 90s ./internal/computer ./cmd/offgrid-computer -run TestNative -count=1
Remove-Item Env:OFFGRID_TEST_NATIVE_WINDOWS
```

The tests exercise real UIA and actual worker consent dialogs, independently read
the resulting Win32 edit text, and test the Stop button, registered hotkey message,
stale controls and replay rejection. Automatic confirmation exists only in the
test executable and targets its own child/fixture—not the production worker.
This is deterministic driver evidence, not a model-planning or full app test.

### macOS and Linux native worker development

The same Go host/supervisor now builds macOS Accessibility and Linux AT-SPI
adapters. The macOS adapter uses an Objective-C framework shim rather than the
planned Swift worker; the Linux adapter uses libatspi through cgo. Neither invokes
AppleScript, shell commands or arbitrary keyboard input. Both retain provider
objects and check process identity, same-user ownership, window ancestry and
fresh state before the supported text replacement/activation operations.

macOS builds include an AppKit consent/Stop helper with an emergency shortcut.
Accessibility must be granted to the companion's execution identity. Linux builds
include a GTK consent/Stop helper; it refuses startup when it cannot monitor the
desktop screen-lock service. Escape stops only while that Linux window has focus;
a desktop-wide Linux shortcut and Wayland portal capture/input remain unfinished.
Permission setup, native UI integration and qualification are still required.

An isolated Ubuntu 24.04 Xvfb/Openbox fixture has passed real AT-SPI discovery,
approved Unicode editing, an independent GTK text oracle and stale-action
rejection. This does not qualify GNOME/KDE or Wayland desktop sessions. macOS
compilation and packaged checks run on the review branch's Intel/Apple Silicon
CI matrix; results must be recorded before making a platform claim.

Linux build dependencies are `libatspi2.0-dev`, `libgtk-3-dev` and
`libjson-glib-dev`; the developer fixture also needs `at-spi2-core`, `dbus-x11`,
`xvfb`, `xauth` and `openbox`. These are build/test requirements, not terminal
setup instructions for ordinary users. In an unprivileged isolated desktop:

```bash
OFFGRID_NATIVE_ISOLATED_DESKTOP=1 xvfb-run -a dbus-run-session -- bash dev/scripts/test-native-linux.sh
```

`dev/containers/native-contracts.Dockerfile` provides an isolated Ubuntu build
environment. Run it with Docker `--init` and no host display, socket or service
data mounts. Supply the repository and matching Go toolchain as build inputs.
No native worker implements screenshots, vision, whole-desktop input or general
file operations yet; pack manifests explicitly retain `qualified: false`.

### Building and validating the desktop integration

Install locked build dependencies with `npm ci --prefix computer --ignore-scripts`
before building the desktop. `desktop/prepare-computer.cjs` assembles pinned
Playwright 1.62.1 and its Chromium build for the target platform. The package hook
checks every copied file against the runtime manifest. Preserve vendor browser
binaries/signatures instead of re-signing them as OffGrid; application signing
and macOS notarization require separate qualification. Manifest hashes detect
corruption, not authenticity independently of a trusted desktop package.

`node dev/scripts/test-packaged-browser.mjs PATH_TO_DESKTOP_EXECUTABLE` exercises
the real renderer/preload/main/utility process and bundled Chromium against an
owned local protocol fixture. Consent is intercepted **only in that test process**
and accepted only for its demo; it is not a production bypass. The test does not
qualify model planning, public websites, OS permission dialogs, or native control.
CI runs this alongside installed startup/recovery checks on its desktop matrix.

For an explicitly approved developer smoke test against a running local service:

```powershell
node test/live-demo.mjs MODEL_ID --approve-owned-fixture-only
node test/live-demo.mjs MODEL_ID --approve-owned-fixture-only --revisions
```

This creates its own temporary headless demo, runs the real model and durable
agent service without an expected-text field, and approves only fixed field
edits and the fixture's Save draft button. The revisions case saves preliminary
and final Unicode titles with intermediate checks and exactly four approvals.
It refuses an already active browser session,
navigation, unexpected mutations, duplicate dispatches and additional approvals.
It independently checks the final DOM, cancels unfinished work and closes its
session. The diagnostic task remains in history. This is an opt-in test harness,
not an unattended companion: it does not qualify the interactive companion's
journal/recovery behavior, arbitrary tasks, native control or vision. The service
must be locally accessible on port 11611 without authentication for this harness;
never disable authentication on a shared service to run it.
