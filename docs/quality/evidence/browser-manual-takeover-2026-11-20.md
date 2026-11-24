# Browser manual takeover — 2026-11-20

Status: implementation in progress; installed handoff and complete M2 are not
qualified. Existing Take over revokes the task grant and closes the managed
browser. The active change preserves only owned browser resources while requiring
fresh consent and pairing for continued automation.

## Workspace core

`BrowserWorkspace` now fences new/queued preparation, reads, captures and input
before draining its admitted serial work. Manual readiness follows actual drain
and evidence invalidation. Confirmation times out after 15 seconds without
reopening automation; subsequent confirmation can observe a finished drain.
Old element/document handles, cursors, issued frame references and upload grants
are cleared. Tabs/cookies remain in the same isolated browser. Explicit close
still owns browser and egress cleanup.

Actual Windows, Node 24.18, Playwright 1.62.1 Chromium fixtures:

- Initial three core cases **3/3 passed, 4.176 s**: a real independently counted
  HTTP effect drains once; manual sign-in sets a synthetic HttpOnly cookie and
  preserves tabs; old observation input rejects; manually closing the selected
  tab does not select another; Stop closes the retained browser.
- Added an explicit HTTP-response oracle, already-queued read denial and timeout
  recovery. `node --test apps/computer/test/browser-takeover.test.mjs
  apps/computer/test/browser-workspace.test.mjs`: **10/10 passed, 15.477 s**,
  zero skips, including four handoff cases and six existing workspace cases.

## Companion ownership and review

Polling now has a separate abort controller from admitted result delivery. A
handoff fences the existing workspace immediately, lets an admitted journal/reply
settle, retires the old token and retains only the owned browser/demo resource.
Old result payloads are erased while dispatch tombstones remain. Renewed pairing
occurs before that resource resumes automation. Old owner callbacks are removed;
Stop on a released owner cannot close its successor's browser.

The private managed controller sends correlated manual/ready acknowledgements
with exact target identities. Repeated resume commands observe their original
operation; they do not pair, navigate or replay effects again. Stop supersedes
pending handoff. Current runtime packs include the controller module.

Actual checks, in addition to the workspace cases above:

- Three real HTTP/Chromium/session/journal cases: **3/3 passed, 4.820 s**.
  The admitted reply is deliberately held after its effect and durable result;
  manual readiness waits for settlement. Fresh pairing retains the synthetic
  cookie/context and closes old callbacks. Closing the last manual tab cleans up.
- Adding production managed-controller correlation and Stop cases: **5/5
  passed, 13.686 s**. Utility transport itself is injected in these cases.
- Independent review reproduced a real cleanup defect: an injected failed close
  left actual Chromium alive while the old implementation discarded ownership
  and acknowledged Stop. The original review test failed. Cleanup now retains
  failed resources and reports `computer_stop_unconfirmed`; managed control
  neither confirms Stop nor exits successfully on that failure. A follow-up
  implementation error treated two absent optional resources as equal and caused
  five cleanup failures; the process was interrupted after diagnosis. Explicit
  retained-object checks fixed that error without relaxing assertions.
- Combined session/legacy/controller and original review reproducer: **17/17
  passed, 15.202 s**. Independent rerun of the unmodified review reproducer also
  passed (**6.689 s** process). Tracked retry coverage and full integration checks
  are still being completed.

## Host and renderer integration in progress

The additive service capability is `browser-task-pairing-v1`. Browser protocol 1
enrollment may now bind the current saved task/input, stopped previous browser
session and immutable policy/scope before session publication. Generic enrollment
and native protocol 3 remain compatible. The host checks the principal, workspace,
saved request and renewed local consent, and caches correlated operations through
lost replies. Independent review also found dropped target identity on errors and
definitive login revocation being treated as outage; both have host regressions.
Dedicated host evidence records its own checks.

Renderer Take over attempts local quiescence before service revocation, including
when local acknowledgement fails. Reconnect offers the retained browser with
fresh consent and exact task IDs, preserves Stop and reports pending/unconfirmed
states. Seven authored messages have all 29 locale entries. `npm run i18n:check`
passes **171 contextual catalogs / 1620 messages** with zero missing catalog
copy. Native linguistic review remains open. The initial catalog check failed
because the new source-only JSON catalog was not registered; registering it in
the existing inventory fixed validation, with no catalog exclusion added.

The initial renderer run exposed a fixture route mismatch (`/action` versus the
actual `/{action}` API), plus a separate workspace-opening timeout under concurrent
test load. Failures remain recorded; the corrected renderer suite and composed
actual service journey have not yet passed at this checkpoint.

No personal browser/session, real credential, installed application, external
website or model was accessed. Complete companion/host/UI integration and actual
service handoff verification remain unfinished. Fixtures do not qualify real
model planning, signed-in personal-browser attachment, native dialogs or human
usability.

## Subsequent integrated review checkpoint

The corrected initial renderer and existing workspace suites passed **40/40,
6.7 minutes**, with screenshots retained under
`build/browser-manual-renderer-reviewed-20261120`. English and Arabic screenshots
were visually inspected. That checkpoint predates the following review fixes;
it is not a pass claim for the final source.

Independent renderer review then found two concrete recovery defects. Confirmed
local Stop during service outage retained a misleading connected heading; and a
lost resume acknowledgement followed by navigation and a failed service-session
read discarded the exact resume-check intent. Both new regressions failed before
the corrections and **2/2 passed, 27.3 seconds** afterward. The renderer now shows
confirmed local browser closure separately from remote uncertainty and retains a
non-secret exact task/input/previous-session resume intent through navigation.
Only an explicit action checks the trusted host's original operation; the draft
is not an access grant. Consent rejection, definitive missing ownership, confirmed
attachment and Stop clear it appropriately. Baseline/fixed traces and logs are
under `build/browser-manual-renderer-review-{baseline,fixed}-20261120`.

The complete companion run `npm test` produced **144 passed / 2 failed / 0
skipped, 138.511 seconds**. The failures were an initial `network_unavailable`
navigation and a protected capture rejecting a changed document in the static
media-query fixture. A separate command,
`node --test --test-concurrency=1 test/browser-capture-witness.test.mjs
test/browser-workspace.test.mjs`, passed **10/10, 29.887 seconds**. The machine
exposes 16 logical CPUs; free physical memory was approximately 1.54 GiB during
the aggregate and 4.33 GiB afterward. These observations motivate bounded
concurrency checks; they do not establish the cause or resolve the failing
aggregate. Assertions, privacy guards and deadlines remain unchanged. Logs:
`build/browser-manual-companion-final-20261120.log` and
`build/browser-manual-companion-failure-review-20261120.log`.

The [host evidence](browser-manual-host-2026-11-20.md) now records three isolated
actual service/host/Chromium journeys passing in **16.466 seconds** with passive
process logging and unwrapped production methods. Each independently counted
one effect and exactly two pairings, retained the selected page and two tabs,
used a synthetic HttpOnly sign-in cookie, resolved the saved task input and
confirmed real browser/process cleanup. Prior composed cleanup timeouts remain
in that evidence; these passes do not establish their cause or performance
qualification. Full final-source renderer/companion checks remain in progress.

### Final manual increment source checks

- `npm --prefix apps/web run api:check`, `check`, `build`, and `i18n:check`
  **passed** after both renderer review fixes. Vite built in **2.66 seconds**;
  existing large-chunk/startup-asset/config-loader warnings remain visible.
- `playwright test tests/browser-manual-takeover.spec.ts --workers=1`:
  **39/39 passed, 3.8 minutes**, including all 29 locales and the two reproduced
  review regressions. `browser-workspace-access.spec.ts` separately passed
  **5/5, 16.8 seconds**. The initial final command included the nonexistent
  `browser-workspace.spec.ts` selector and therefore ran only the 39 manual cases;
  the separately reported five-case command uses the actual filename.
  Final traces/screenshots and logs are under
  `build/browser-manual-renderer-final-20261120` and
  `build/browser-workspace-final-20261120`. English/Arabic final screenshots were
  visually inspected; narrow RTL controls are visible without duplicate task
  manual-state paragraphs. This is rendered QA, not linguistic/human approval.
- `node --test --test-concurrency=2 test/*.test.mjs` in `apps/computer`:
  **146/146 passed, 73.896 seconds, zero skips**. This includes the independently
  reviewed failed-close/retry case and a lost result acknowledgement at takeover:
  the original action remains uncertain and cannot be replayed under a new
  pairing. Log: `build/browser-manual-companion-bounded-20261120.log`.

Tasks 1.2, 2.2 and 3.1 now have their scoped implementation evidence. Task 3.2
remains open: the earlier default-concurrency failures and composed cleanup
timeouts remain unresolved, and final combined applicable CI/source checks are
not yet complete. The bounded-concurrency pass does not erase those failures.
