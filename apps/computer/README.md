# Computer Tasks browser and native-structured preview

This is the implementation/developer reference. For normal setup and useful
task examples, start with [Computer Tasks](../../docs/guides/computer-tasks.md).
Capabilities described here remain preview; the reliability plan owns evidence.

## Managed browser recovery and discovery

Matching services additionally advertise `browser-workspace-v1`. Updated managed
companions negotiate `browser_features: {discovery: 1, workspace: 1}` and an exact
`browser_origins` list. The local user selects the starting site and up to 15
additional public HTTPS origins, including resource domains. Every destination
is validated and pinned at the CONNECT relay before startup; redirects cannot
escape this set. Old clients keep their one-origin contract, and expanded scope
is rejected when the service lacks workspace support.

`browser_tabs` lists up to eight task-owned tabs. `browser_tab_open` opens an
approved URL; `browser_tab_select` and `browser_tab_close` take an issued `tab_id`.
Workspace observations include `tab_id` and `document_id`. Each page keeps its own
references; selection returns a fresh observation. Commands are serialized, tab
overflow is rejected, and unknown/closed identities never target another tab.
Closing a tab is consequential because it can discard unsaved content. Owned
popups count toward the cap; no personal browser profile or tab is attached.
New-tab navigation failures remain uncertain if creation has already started.

CLI consent displays the full site list and tab limit. Network scope cannot be
expanded by model arguments or page text. Stop and obtain fresh local consent
to change it. Signed-in extension attachment remains separate required work in
the transformation tracker.

Matching services advertise `browser-discovery-v1` in `/api/v2/system`. The
companion then pairs with `browser_features: {discovery: 1}`. Older services receive
the legacy pairing shape; older companions are not offered `browser_inspect`.
The existing zero-argument `browser_observe` contract remains valid.

Observations return at most 80 visible controls after scanning at most 400
candidates per response. Follow `next_cursor` through `browser_inspect` with the
same `observation_id` to inspect later controls and text. An empty control page
can still have a continuation. Dropdowns expose `options_cursor` for remaining
options; only observed option IDs can be selected. Responses fit the 48 KiB hub
limit, including Unicode. Field values and target hashes remain local. Text and
control paging alone does not grant additional frame tools or network authority.

Input checks the exact retained control, its semantics and form state, document
identity, URL and observation age, including same-address document replacement. Unrelated DOM text changes do not invalidate an unchanged
target. Captures and continuations additionally require a stable page revision.
Changed input values, form destinations and replacement controls still reject
dispatch. All page-authored text remains untrusted.

Typed `computer_browser_precondition_failed` and
`computer_browser_observation_failed` receipts are journaled before acknowledgement
and keep the companion connected. The service admits only the reviewed browser
descriptor and error stage to bounded repair. It allows at most three consecutive
repair attempts, then pauses. Arbitrary input exceptions remain uncertain.

Successful input reports `effect_status` separately from `verified` and
`observation_status`. A failed follow-up read does not discard the input receipt
or authorize repetition. Use the observation returned by an action when available;
otherwise inspect again. A click receipt is dispatch evidence, not task success.
Authenticated mutation receipts persist through the existing service receipt path;
restart can recover the original result without input, while the old session
expires. Older replies without effect stages remain conservative.

Clicks are consequential even when labelled Save/Apply. Scoped-changes mode asks
for exact approval; existing full-task grants continue within their scope.
Credentials, payments and other prohibited operations remain blocked.

See [dated evidence](../../docs/quality/evidence/browser-execution-2026-11-20.md)
for checks and remaining qualifications.

## Structured search, controls and frames

Matching services advertise `browser-structured-v1`. An owned workspace pairs
with `{discovery: 1, workspace: 1, structured: 1}`; older services receive the
existing shape and older companions are not offered the new tools.

- `browser_search` starts a fresh observation with exact string fields `role`,
  `name` and `text`. At least one must be nonblank; name/text are at most 256 UTF-8
  bytes. Matching is literal, NFC-normalized, case-insensitive and whitespace
  normalized. Supported roles are button, link, textbox, checkbox, radio, switch,
  combobox, option, tab and menuitem; an empty role applies no role filter.
  Filters combine. A full accessible label can match even when its displayed
  label is truncated. Results retain every matching exact control, so duplicate
  labels never cause automatic selection. No CSS, XPath, regex or executable
  expression is accepted.
- Continue the returned `next_cursor` with `browser_inspect` and the original
  `observation_id`. The cursor retains its query; a new search invalidates old
  cursors and targets. Coverage reports scanned/total controls. Text chunks remain
  complete and bounded, including on empty control pages; `text_match` describes
  only the returned chunk, not the whole document. Follow all continuations to
  establish complete text coverage, including matches spanning chunk boundaries.
- Open-shadow controls use labels in their own root and retain host ancestry in
  freshness checks. Silent property changes, replaced hosts and changed semantic
  state invalidate old input. Contenteditable fill checks the resulting exact
  multiline text. Native checkboxes and boolean ARIA checkbox/switch controls
  report verified, mismatch or unavailable read-back. An already-matching state
  sends no input. Custom checked-state input invokes a website click and therefore
  remains consequential. Mixed/absent checked semantics are unsupported.
- Custom combobox expansion and option activation use observed clicks; inspect
  newly rendered options before selecting one. These clicks remain consequential.
  `browser_select` still supports a single enabled native select option only.
- `browser_frames` lists at most 64 owned frames with exact `frame_id`,
  `document_id`, parent identity and availability. `browser_frame_select` accepts
  the issued frame/document pair and returns a fresh observation. Observations
  also carry `frame_id` and `frame_document_id`, separate from the tab document.
  Reload, detach and parent replacement invalidate old references. Loss of a
  selected frame requires explicit reselection; it never falls back to the page.
  `browser_navigate` explicitly navigates the task tab and returns to its main frame.

Frame access requires approved origins for the selected document and all its
ancestors. The companion reads the actual origin in a Chromium isolated world;
page-authored `window.origin`, iframe `src` and the inspector's precursor-origin
metadata are not sufficient authority. Ambiguous same-URL frames are denied if
any matching document has unavailable/opaque origin evidence. Inherited/opaque
documents and frame processes without usable origin evidence remain unavailable.
Scope checking never picks the control; retained browser handles do that.

Visible text omits protected inputs and explicitly private/editable content,
including open shadows. Text/state scans reject more than 50,000 visited nodes;
text also rejects more than 2 Mi UTF-16 code units. Captures inspect at most 1,000
controls across readable frames, mask sensitive fields and private containers,
and conservatively mask whole child-surface groups for unavailable frames. Closed
shadow content or incomplete child-document masking coverage rejects capture.
Capture checks browser DOM counts, bounded markup, document identity and property
state before returning pixels. An isolated-world mutation witness and native
Chromium stylesheet/animation events reject transient changes even if the page
restores its original markup. Local PNG masking includes descendant boxes and
direct text rectangles, including overflowing and `display: contents` content.
Inspection and mask output are bounded to 2,000 elements/rectangles. Raw pixels
stay in companion memory and their buffer is cleared after masking or rejection;
only the masked image can leave the companion. The packaged decoder is pinned
`pngjs` 7.0.0 (MIT), with dimensions checked before decoding.

Capture preserves page styles. Present animations (including paused/fill effects
and SVG animation), protected generated content, shadows/reflections/filters and
transformed child-frame geometry reject capture when private paint cannot be
bounded. Structured reading and controls remain available independently. These
explicit limits do not qualify general animated-page vision, visual model actions
or arbitrary closed-root widgets.

See [structured browser evidence](../../docs/quality/evidence/browser-structured-controls-2026-11-20.md)
for actual fixture results, retained failures and remaining product qualification.

## Manual handoff implementation

The workspace now supports an internal handoff fence: it immediately refuses new
automated reads, captures, preparation and input, then drains admitted work before
reporting manual readiness. A 15-second confirmation timeout leaves automation
fenced and reports an unconfirmed handoff. It does not undo or repeat an input.
The browser context, tabs and cookies remain in the owned process; old observation
handles, cursors, frame selections issued to the model and upload grants are cleared.
An explicitly resumed workspace requires fresh observations and never chooses a
replacement for a manually closed selected tab.

The companion retires polling and its old session token after admitted result
handling settles. The existing utility process retains the browser until explicit
consent and fresh task-bound pairing resume it. Correlated host replies distinguish
confirmed manual control from a timeout; duplicate resume requests observe the
same operation. Failed close retains the cleanup resource and reports unconfirmed
Stop instead of claiming termination. These methods are not model tools or service
authorization bypasses. The [manual takeover evidence](../../docs/quality/evidence/browser-manual-takeover-2026-11-20.md)
records the scoped checks and unfinished integration verification; installed,
model and human qualification remain open. Personal browser attachment remains
separate required work.

## File transfer integrity in progress

Uploads now capture the locally approved bytes into a private, exclusive snapshot
before browser assignment. Subsequent actions revalidate the original selection,
while input uses the snapshot. All task tabs share its owner; manual takeover
revokes the old grant, and actual browser closure precedes snapshot cleanup.
This lifetime matters because Chromium can read a selected File after the tool
returns. Failed cleanup retains ownership for explicit retry.

The 512 MiB admission limit remains unchanged; a real 51 MiB delayed read and
independent HTTP digest check pass. [Transfer evidence](../../docs/quality/evidence/browser-file-transfers-2026-11-20.md)
records the original hash-to-reopen defect, repeated-action regression and actual
checks. Isolated native File read-back now verifies the complete selected content
and reports selection separately from remote receipt. The durable owner is now
connected to task binding, shared tabs and lost-reply recovery. Matching managed
packs use a storage-only native permission check before activation and snapshot
use; existing insecure storage is rejected without permission repair. Older packs
can browse without enabling the protected transfer path. Cross-client custom-profile
exclusion, unrestricted command containment, download limits/publication/provenance,
explicit artifact export and platform/model
qualification remain unfinished. Random private paths/read-only mode do not
defend against another process controlling the user's full OS identity. No
stronger negotiated transfer capability is advertised from these partial changes.

## Workflow evidence and bounded discovery

New task-first native runs retain at most 12 immutable outcome checks per user
revision. The action contract derives mechanical checks from validated typed
launch/create/edit arguments and persists them before authorization and dispatch;
no extra model declaration is needed for these actions. Saved jobs keep their
explicit-check contract. Read-only discovery is allowed first. The service
alone records provenance, bindings and assessments; a document or model cannot
grant access, remove an unfinished check or turn its own output into proof.
`application_open` proves a matching usable application surface;
`control_text_equals` and `control_checked_equals` prove independent read-back
of the specified control state. They do not verify research, saved files or
external submissions. Unsupported kinds remain explicitly unverified.

Native element `identity` is optional, evidence-only metadata. Windows uses UIA
runtime identities, macOS retained AX object equality, and Linux retained AT-SPI
object identities across consecutive captures. Missing/ambiguous identities,
target replacement and reset cannot match by name, value or row. Input still
requires the current observation and element ID. The service pins the full
target digest and session at declaration. Old hosts remain usable with limited
verification; no v2 window permission is upgraded.

`computer_find_controls` searches the latest saved native observation by a
case-insensitive literal Unicode name fragment, exact role and optional boolean
properties. It preserves canonical order; it never chooses a control for input.
`computer_inspect_control` pages stored text for one exact element. Query strings
are limited to 256 UTF-8 bytes. Pages contain at most 16 controls and 4096 encoded
bytes; text pages preserve UTF-8 boundaries. Continuations bind the entire saved
view and query. `model_omitted` counts remaining matching controls in search and
remaining text bytes in inspection. `source_limited`, `text_limited` and name/role
limits describe what was omitted; paging cannot recover absent host content.

These tools make no host calls, focus changes or freshness extensions. They
recheck the owner, active session and latest target. Expired input observations
may be read with `input_fresh: false` and `age_ms`; replaced targets, revoked
sessions and unknown structures are rejected. Ordinary owner-only history stays
available separately. Saved application text is untrusted input.

## Task desktop authority (protocol 3, Windows x64 preview)

The matching Windows desktop and service separate a **task grant** from
individual execution targets. The default composer saves desktop scope and
Full task access and holds each saved task before inference. Explicitly enabling
Full access confirms consent once for the desktop session; Start offers it if
needed. This in-memory permission is reused only for the same service, workspace
and actor, never loaded from renderer preferences. Enrollment is bound to actor,
workspace, run and pending input; it is not authority until the host binds a
separate task grant. Navigation never
opens the dialog or submits work. A failed/lost reply can attach that same grant
without creating another job. Older window grants never become desktop grants.

The private worker runs with `--protocol 3`. `desktop_identity` precedes consent;
`grant_desktop` authorizes inventory and typed operations for at most 60 minutes
or 500 mutations. The desktop is a synthetic authority target. Prepared actions
bind a real process generation/surface separately. `applications`, `surfaces`,
`prepare_lifecycle`, and `verify_application` extend the existing framed IPC.
Launch/focus use the existing intent/result journal and local supervisor.
There is no host-control listener, executable argument, or model path API.

`enable_desktop` is private protocol-3 IPC with a `{service, workspace, actor}`
permission request. Only the worker's affirmative local consent creates that
permission. There is no `approved` flag. `grant_desktop` additionally binds the
exact service when reusing consent. `release_desktop` revokes the finished task's
supervisor and clears prepared actions/targets, retaining only session consent
on the same signed-in desktop. The transport erases cached observation replies
but keeps duplicate-action tombstones. The next task enrolls again with its own
binding. Permission never survives worker restart, OS session loss or explicit Stop.

Service tools are `computer_applications`, `computer_open_application`,
`computer_windows`, `computer_focus_window`, and `computer_verify_application`,
alongside structured control tools and independently authorized MCP tools.
Opening Chrome needs a catalog ID, not a URL. A spawn is only `dispatched`;
completion requires a separately observed surface with matching process image.
Verification can reuse an existing process and never repeats a launch.

Protocol-v3 Windows workers handle slow inference through `RefreshObservation`
before a new control preparation or read-only verification. The proposed
observation must exactly match the host's retained view. Once expired, the driver
actually recaptures the full UIA view, requiring matching target, ordered runtime
identities/state digest, all exposed values/capabilities and completeness. It
retains model-visible element IDs only for this exact match and uses the new
observation identity/time for preparation. Recapture clears old preparations;
dispatch still revalidates process, foreground, control identity and precondition.
Changed, incomplete, forged or foreign state fails closed. There is no general
age extension, coordinate remapping, approval renewal or uncertain-action retry.
V2 and adapters without this optional method keep strict freshness behavior.

Windows Chromium observations allow up to two seconds for a cold UIA document
tree to appear, with fresh snapshots and unchanged target validation. A timeout
marks the snapshot limited. Empty, non-interactive layout containers are omitted
from the flat model view. After text/checkbox changes, bounded read-back waits
for asynchronous provider updates; the mutation itself is dispatched only once.
Cancellation or missing confirmation remains uncertain, never an automatic retry.
`OFFGRID_NATIVE_BROWSER_FIXTURE` opts into `TestNativeOwnedBrowserSearch` using an
explicit installed Chrome executable, a disposable profile and a loopback search
form. Its independent HTTP oracle checks the submitted query. No personal browser
profile, public website, sandbox bypass or debugging port is used.

Windows discovery reads per-user/machine App Paths (both registry views) and
Start Menu shortcuts, with system Notepad as a built-in fallback. Only validated
local GUI executables with zero launch arguments enter the catalog. Shell links
are read through IShellLink without Resolve or script execution; command/installer
registrations, console binaries and redirected paths are excluded. Packaged
activation and shortcuts needing arguments require further typed adapters.
Launcher stubs that redirect to a different executable (including some Store
Notepad and LibreOffice builds), and unaddressable controls may fail readiness;
do not claim those workflows completed. This is an extensible catalog, not
arbitrary shortcut execution. Executable identity is checked at discovery and
dispatch; source/publisher authenticity is not established by a file identity.
Owned Win32 fixtures exercise UIA launch/readiness and independent native
read-back of structured-control changes without touching user applications.
Native visual fallback and direct host filesystem tools remain unavailable.
macOS/Linux retain explicit v2 selected-window access until they have v3 host
implementations and evidence; service capability alone does not promise one.

Task termination revokes task authority, not separately enabled session consent.
Desktop-owned Windows workers use `--stop-control app`: the existing app banner,
File menu and tray stop through private IPC even offline, with no duplicate Stop
popup. A hidden native message thread independently handles the emergency key
and lock/logoff/disconnect/suspend notifications. Renderer loss or unresponsiveness
also revokes locally. Standalone workers retain their visible stop surface.
OS-session revocation persists even if the desktop later unlocks. Returning online never restores
the grant. No stop claims to undo an already dispatched action. Matching packs
are necessary; a container-only update cannot implement this host behavior.

The selected-window and managed-browser sections below describe compatibility
modes, not the normal protocol-v3 Windows flow.

## Native-control foundation (not native availability)

The service no longer instantiates the legacy blanket-approved computer
controller. Browser protocol 1 continues through the durable agent authorization
path. `internal/computer/protocol_v2.go` defines the next typed driver contract,
opaque target identities, bounded-step digests, local-consent binding and private
length-prefixed worker framing. Its tests reject changed values/targets, expired
grants, mixed consequential actions, oversized frames and executable strings.
The native worker implementations below use this contract and a host-local
supervisor. They are supervised preview components, not a qualification claim.
The desktop Agents client now exposes application discovery, catalog-based
launch, window selection and the same durable native session path. Managed
browser sessions can provide a transient, observation-bound viewport capture to
an installed model/projector pair; native-window capture and vision remain
unimplemented.

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

This is a **managed-browser and native-structured preview**, not the completed
cross-platform Computer Tasks program. The desktop app can launch a catalogued
installed application or select an existing window, then perform only the
currently implemented structured operations (observe, bounded text replacement,
explicit checkbox state, activation, fixed application shortcuts and read-back
verification). Native app control does not use vision, attach to personal browser
profiles, enter credentials, or perform file transfers. Managed browser transfer
integrity and bounded download staging remain under qualification above. All drivers
remain **unqualified** until the full task/model/platform evaluation gates pass.

## Installed desktop: no terminal setup

Updated desktop packages include the matching Playwright/Chromium runtime and
native worker. On Windows x64 with protocol 3, choose the computer policy before
**Start task**, approve the task-wide local consent once, and let the agent
discover and open supported applications. The application picker is not part
of this default flow. A tool-capable model is still required.

For explicit selected-window compatibility sessions, when the saved task
requests **Access needed**, select an existing application window
or launch a catalogued Windows shortcut, macOS application, or Linux desktop
entry. Local consent connects that actual target to the same saved task; no
Node installation or copied pairing code is needed. Browser requests use a
separate managed Chromium profile. Before connecting, choose one
session policy: **Ask every time**, **Approve scoped changes**, or **Full task
access**. The default automatically permits only locally classified reversible
work in the selected window. Full task access also permits consequential typed
actions, such as a form submission or verified file transfer, inside that scope.
The selected policy is fixed for the session and shown in activity history.

Managed-browser text fields, native single-choice dropdowns, and native checkboxes
have typed actions. Dropdown options come from the current observation; checkbox actions
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

The web edition provides **Continue in desktop** for the saved task. The
`offgrid://computer` link can carry its validated task ID, not a service URL,
credentials, or an executable action.
The desktop must be connected to the same matching workspace (the ordinary local
container address is `127.0.0.1:11611`). A browser page alone cannot control the
host. Remote service attachment and silent workspace switching are not supported.
Manual pairing is a developer/legacy CLI fallback, not a normal UI prerequisite.

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
configuration and IPv6-only sites are not yet supported. Resource domains must
be explicitly included in the approved origin list; omitted resources remain
blocked and some sites will remain incomplete or unavailable.

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
cd apps/computer
npm ci --ignore-scripts
npm run browser:install
npm start
```

The companion asks for the local service URL (press Enter for the default), then
a browser target. Enter a public HTTPS page URL, or `demo` for an isolated practice
page. For a public site choose `direct` (default) or explicitly `trusted-vpn`.
After local consent,
service/network/browser preflight completes **before** asking for a pairing code.
Only then run `offgrid computer pair` in a second terminal configured for the
same service and actor, and paste the fresh single-use code into the companion.
Keep credentials/codes private. `offgrid computer targets` lists connected
sessions. This legacy developer path submits with
`offgrid computer run SESSION_ID MODEL_ID "Describe the selected page"`; normal
task-first setup uses `offgrid computer setup RUN_ID` and desktop consent instead.
Start with a bounded inspection such as
finding a heading and verifying its text. Review exact arguments before approving
navigation, clicking or typing. A browser action succeeding is not proof of the
entire task's correctness.

Computer access checks the selected model before dispatch. For troubleshooting,
`offgrid computer check MODEL_ID` runs the same two synthetic tool
turns through the selected runtime without reading or controlling your browser:
an observation call followed by a verification call using a generated heading.
A prose simulation, malformed call, wrong tool, truncation or incorrect result
does not pass. The check may load the model and takes up to 90 seconds; you can
cancel the optional diagnostic. The service enforces the check; clients cannot
bypass it. In the task-first flow, a failed check leaves the saved task pending
access. The legacy explicit-session submission can reject before task creation.

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
- Network requests are restricted to the selected origin set (one site for
  legacy clients). A loopback HTTPS CONNECT relay checks each exact hostname
  and port and pins its destination;
  this covers redirect escapes that browser route callbacks alone miss. Direct
  mode rejects private/reserved DNS results. Trusted-VPN mode additionally permits
  the consented fake-DNS range, with the disclosed routing trust boundary above.
  Plain HTTP, unapproved origins, file URLs, service workers and WebSockets remain
  blocked. Blocked resource origins are reported without request paths, cookies
  or credentials. Closing the browser tears down owned relay sockets.
- Structured observations are untrusted page data. Password/credential controls
  are excluded and field values are not collected. With an installed projector
  that passes the current runtime's synthetic image/tool-call check, the model
  may request a current browser-viewport capture after observation. Projector
  presence alone never enables the capture tool; passes expire on restart or
  model/projector replacement.
  Password, payment-autocomplete and explicitly private elements are masked;
  captures stay in memory, are consumed by the next model turn, and are not
  written to task history. Masking cannot identify every secret, so approve only
  pages intended for OffGrid. Native-window screenshots are not implemented.
- Approval authority cannot be supplied by the model. The local driver prepares
  and classifies the exact action from a fresh observation; the service applies
  the immutable session policy, and the companion checks it again immediately
  before dispatch. Ask every time requires exact review for every mutation.
  Approve scoped changes automatically permits reversible work. Full task access
  additionally permits consequential actions exposed by the bounded driver.
  Credentials and payment controls, financial transactions, privilege/security
  changes, software installation, arbitrary scripts, permanent deletion, stale
  actions, and uncertain retries remain blocked in every mode. GUI effects cannot
  be sandboxed perfectly; do not use financial, administrative, security-sensitive
  or account-management pages in this preview.
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

Browser transfer support remains partial. Download size/digest metadata alone
does not establish an in-flight staging bound; durable publication and artifact
export remain required work. Uploads use one file chosen in trusted desktop main.
The renderer and model receive an opaque, digest-bound grant and filename, never
its host path. The companion captures and verifies approved bytes before native
selection, which does not establish remote receipt. The unchanged 512 MiB maximum
still has a failing intercepted raw-body transport case in the evidence tracker.

Tests: `npm test` exercises a real isolated Chromium fixture, including bounded
capture and protected-control masking. The service separately checks typed image
transport and an exact image-grounded tool call before exposing capture. Neither
check is evidence that a particular local vision model can ground or plan a
complete task. Native Windows/macOS/
Linux workflow qualification, native vision, signed/offline automation packs, persistent OS-keystore
pairing, full pause/takeover UX, and the 30-case qualification suite remain pending.

### Native Windows worker development

`cmd/offgrid-computer` now implements a Windows x64 host process with UI
Automation target discovery, selected-window observation, exact approved text
replacement, activation and explicit checkbox state. It uses direct Go COM bindings to Windows UIA, not
shell commands or simulated keystrokes. This replaces the proposed C++ worker
implementation for this slice while retaining process isolation. It is connected
to the Agents task/session APIs through the desktop bridge. The catalog launcher
remains a trusted main-process operation; renderer and model text cannot provide
executable paths.

The worker uses private framed pipes, local Yes/No consent, a visible Stop window,
and Ctrl+Alt+Shift+F12. An independent watchdog terminates a hung owned worker
after revocation; it never terminates the target application. Structured password
controls are excluded, but ordinary control labels can still contain sensitive
data. This is not a guarantee that all secrets are detected. There is no native
screenshot/vision path or arbitrary keyboard/shell interface. A fixed application
shortcut/navigation set (`copy`, `paste`, `undo`, `redo`, `select_all`, `save`,
`enter`, `escape`, `tab`, `reverse_tab`, arrows, paging, `home`, `end`) is available
only after fresh observation, exact service approval, local approval and
foreground-window validation. Dispatch is not proof that a document was saved.

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
fresh state before the supported text replacement, checkbox and activation operations. macOS
also supports the fixed application shortcut set. Linux permits it only on X11;
Wayland refuses it until an XDG RemoteDesktop/EIS grant is implemented.

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
OFFGRID_NATIVE_ISOLATED_DESKTOP=1 xvfb-run -a dbus-run-session -- bash scripts/test/test-native-linux.sh
```

`docker/test/native-contracts.Dockerfile` provides an isolated Ubuntu build
environment. Run it with Docker `--init` and no host display, socket or service
data mounts. Supply the repository and matching Go toolchain as build inputs.
No native worker implements screenshots, vision, whole-desktop input or general
native file operations yet; pack manifests explicitly retain `qualified: false`.

### Building and validating the desktop integration

The `document-session.mjs` transport is shared by selected-window and
task-desktop sessions. It uses the same native worker, grants, journal and Stop
path; it does not create another input authority. Document calls require the
negotiated `native_documents:1` feature and surface-specific capabilities. Read
pages are pinned to snapshot/revision; creation/edit results must carry matching
host action evidence, not just a successful receipt. Old hosts continue their
existing operations without receiving the new fields.

The Windows backend now negotiates the feature and implements bounded UIA
relationship discovery, exact tab selection, TextPattern/Unicode edits and an
exact-version packaged Notepad creation profile. Service tools use the existing
runner and journal. AX and AT-SPI document adapters retain native object
identities and implement capability-checked range/text operations. AX input needs
explicit composition evidence; generic AT-SPI composition remains unknown, so
document edits are unavailable there. Unix and browser creation profiles remain
unfinished. See the [operation/version table](../../docs/guides/computer-tasks.md#document-support-and-qualification).

No installed application is qualified by this implementation. Isolated Xvfb
tests cover owned GTK document reads and legacy structured input, not the new
document-edit sequence, real Wayland, macOS, or installed Notepad. The macOS
provider has not yet been compiled against an Apple SDK. Missing support never
falls back to Windows behavior, guessed shortcuts or another input method.

Install locked build dependencies with `npm ci --prefix apps/computer --ignore-scripts`
before building the desktop. `apps/desktop/prepare-computer.cjs` assembles pinned
Playwright 1.62.1 and its Chromium build for the target platform. The package hook
checks every copied file against the runtime manifest. Preserve vendor browser
binaries/signatures instead of re-signing them as OffGrid; application signing
and macOS notarization require separate qualification. Manifest hashes detect
corruption, not authenticity independently of a trusted desktop package.

`node scripts/test/test-packaged-browser.mjs PATH_TO_DESKTOP_EXECUTABLE` exercises
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
