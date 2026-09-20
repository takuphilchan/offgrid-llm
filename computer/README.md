# Computer Tasks browser preview

This is a **managed-browser preview**, not the completed cross-platform Computer
Tasks program. It controls a separate Chromium browser on the local host. It does
not control native applications, use vision, attach to personal browser profiles,
enter credentials, or download files. All drivers remain **unqualified** until
the full task/model/platform evaluation gates pass.

## Installed desktop: no terminal setup

Updated desktop packages include the matching Playwright/Chromium runtime. In
**Agents**, describe your task, enable **Use a browser**, and enter a public HTTPS
site or choose **Try a practice page**. Approve the native consent prompt. OffGrid
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
Public sites whose DNS resolves into VPN fake-IP/private ranges remain blocked;
use the practice page without disabling a required VPN.

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
a browser target. Choose `demo` (the default) for the companion's isolated local
research-notes page, or enter one public HTTPS origin. After local consent,
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
- Network requests are restricted to the selected origin. Public IPv4 resolution
  is checked and pinned for the browser session; private/reserved addresses, file
  URLs, cross-origin resources, service workers and WebSockets are blocked. Some
  real sites will not function under this deliberately narrow policy. VPN fake-DNS
  destinations and IPv6-only sites are unsupported for public browsing; use the
  owned demo instead. Explicit proxy support is not implemented. Do not disable
  these checks or add a blanket exception for reserved addresses.
- Structured observations are untrusted page data. Password/credential controls
  are excluded, field values are not collected, and screenshots are not captured.
  Page text may still be sensitive: approve only pages intended for OffGrid.
- Exact action approvals cannot be supplied by the model. Stale observations are
  rejected. GUI effects cannot be sandboxed perfectly; don't use financial,
  administrative, security-sensitive or account-management pages in this preview.
- Dispatch IDs and hashes are journaled in the local user's
  `.offgrid-llm/computer/dispatch.sqlite`. No arguments, credentials or screenshots
  are stored there. Duplicate actions are refused, not automatically replayed.
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
Linux drivers, vision, signed/offline automation packs, persistent OS-keystore
pairing, full pause/takeover UX, and the 30-case qualification suite remain pending.

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
