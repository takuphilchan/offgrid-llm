# Maximum upload transport investigation — 2026-11-21

Scope: the existing `browser-file-transfers` change, roadmap M2.07. This is a
diagnostic boundary investigation, not a second upload executor or a network-policy
exception. Every file, receiver and browser profile in the controls is disposable.
No browser/runtime download, personal profile or live installation is involved.

## Trigger and required outcome

The parent's strict `apps/computer/test/browser-file-max.test.mjs` selected and
independently verified all 536,870,912 bytes through the isolated native File
readback. Sending that same native File with page `fetch` then closed Chromium
before the receiver saw any bytes. The diagnostic log recorded browser process
exit 3758096392 and Node RSS 177,999,872 bytes at the successful selection stage:
`build/browser-file-max-diagnostic-20261120.log`.

Selection verification therefore passed in that run; remote receipt did not.
The original maximum-size workflow stays failing. A smaller file or a different
page submission mechanism cannot stand in for the missing accepted workflow.

## Pinned source findings

Installed versions: Playwright 1.62.1, Chromium 151.0.7922.34. Primary sources read
2026-11-20 UTC / 2026-11-21 local:

- [Playwright's pinned Chromium network manager](https://github.com/microsoft/playwright/blob/v1.62.1/packages/playwright-core/src/server/chromium/crNetworkManager.ts#L149-L161)
  enables Fetch interception with an all-request pattern when routing is active.
  The installed `coreBundle.js` has the same behavior; a narrower JavaScript route
  matcher does not avoid this protocol interception step. Its request constructor
  also decodes and joins supplied post-data entries.
- [Chromium's pinned interception job](https://github.com/chromium/chromium/blob/151.0.7922.34/content/browser/devtools/devtools_url_loader_interceptor.cc#L1596-L1657)
  invokes request-body collection before publishing the intercepted request.
- [Chromium's pinned request serialization](https://github.com/chromium/chromium/blob/151.0.7922.34/content/browser/devtools/protocol/network_handler.cc#L3950-L3968)
  constructs a post-data string and binary entries from collected bodies. This
  path supplies no `Network.enable.maxPostDataSize` argument to that construction.
  That is source evidence, not a measured promise about every DevTools domain.
- [Chromium's pinned Windows OOM constant](https://github.com/chromium/chromium/blob/151.0.7922.34/base/allocator/partition_allocator/src/partition_alloc/oom.h#L29-L39)
  identifies 0xe0000008, decimal 3758096392, as its out-of-memory exception. The
  [termination implementation](https://github.com/chromium/chromium/blob/151.0.7922.34/base/allocator/partition_allocator/src/partition_alloc/oom.cc#L50-L65)
  raises/exits with that value. The recorded exit establishes the exception
  category; no crash dump or allocating stack has been obtained.

These findings motivate the interception hypothesis. They do not justify removing
the existing origin controls, temporarily permitting unrestricted traffic, or
rewriting a page's upload request.

## Runnable independent controls

`apps/computer/test/browser-file-transport-probe.test.mjs` contains four strict
controls: native raw File and one-file multipart FormData, each without interception
and with ordinary `context.route('**/*', route => route.continue())`. It bypasses
OffGrid's driver solely as an isolated diagnostic control. Each case launches its
own installed headless Chromium instance and disposable profile. The fixture
creates one 512 MiB source with bounded writes, and the loopback receiver hashes
the incoming payload incrementally. Multipart header/trailer parsing is bounded;
it neither buffers the complete body nor treats HTTP success as byte evidence.
The original source is unchanged during this control, isolating network transport
from snapshot integrity.

PowerShell:

```powershell
$env:OFFGRID_BROWSER_TRANSPORT_PROBE='1'
$env:DEBUG='pw:browser'
node --test --test-concurrency=1 apps/computer/test/browser-file-transport-probe.test.mjs
Remove-Item Env:OFFGRID_BROWSER_TRANSPORT_PROBE
Remove-Item Env:DEBUG
```

The opt-in guard keeps this expensive diagnostic visible as skipped in ordinary
suites. Every enabled case requires the exact independent payload digest and byte
count. A known failing case remains a failure, rather than being inverted into a
passing assertion that the browser crashed. Passive browser tracing records the
owned process exit and profile cleanup. `node --check` passed before execution.

## Executed results

The opt-in command above was run with passive browser tracing after other heavy
checks released the window. It **failed**, retaining the required-success
assertions, in 12.608 s. Node reports three passing leaf cases, one failing leaf
case and its failed parent test (five reported tests, three pass/two fail):

| Native body | Interception | Result | Independent receiver | Case duration |
| --- | --- | --- | --- | --- |
| Raw File | None | PASS | 536,870,912 payload bytes, exact SHA-256 | 1.887 s |
| Raw File | Route/continue | FAIL: browser OOM | No HTTP request; zero bytes | 3.641 s |
| Multipart File | None | PASS | 536,870,912 payload bytes, exact SHA-256 | 3.948 s |
| Multipart File | Route/continue | PASS | 536,870,912 payload bytes, exact SHA-256 | 2.267 s |

All passing payload digests were
`be9d2197bf1bb7c157f15d6dc9d7ed0e6ce841e20fdee639b89aee247c29a865`, matching the
independently hashed source. Multipart wire bodies were 536,871,113 bytes,
including framing. The failing raw/intercepted case reached **zero POST route
callbacks** as well as zero receiver requests: failure occurred before JavaScript
could decide whether to continue that request. Node RSS in its diagnostic was
172,568,576 bytes; this is not browser-process memory measurement.

The passive trace records owned failing Chromium PID 45988 exiting with
3758096392, followed by temporary-profile cleanup. All four owned browsers have
process-exit and cleanup records by command completion. After the crashed browser
disconnected, its final process/cleanup events arrived roughly 0.7 s after the
next case launched; durations are diagnostic observations, not clean process-
isolation or performance qualification. Log:
`build/browser-file-transport-controls-20261121.log`.

This reproduces the raw File failure independently of OffGrid's snapshot,
isolated content readback, source replacement and egress proxy. Combined with
the pinned source path, it supports request-body collection/serialization as the
cause. The exact failed allocation still lacks a native stack trace. Multipart
success does not repair raw File uploads, and no fixture or production request
was rewritten to substitute multipart for the failing case.

## Open requirements

The full 512 MiB production workflow, bounded transport memory and compatible
origin enforcement remain open. No alternative Network/Fetch/proxy design has
been integrated here. Any subsequent replacement must prove frame/worker/popup,
redirect, WebSocket and private-destination behavior with the existing approved
scope, including failure and teardown. This probe provides no model, real-site,
installed-platform, soak or performance qualification.

Documentation maintenance checked 482 files with zero problems, the source-size
ratchet passed, and the scoped whitespace diff check passed before recording the
result table. No full product suite was rerun for this diagnostic-only addition.

## Subsequent native-policy probes — incomplete, investigation stopped

The parent authorized a bounded candidate investigation, then explicitly stopped
it before production changes. Preserve the following failures and runnable source
as diagnostic evidence, not replacement-policy qualification:

- `apps/computer/test/browser-network-policy-probe.test.mjs`, opt-in
  `OFFGRID_BROWSER_NETWORK_PROBE=1`, configures one page's native ordered
  `Network.setBlockedURLs` allow pattern with explicit empty username/password
  and a deny-all fallback. A pre-policy HTTP control proves the foreign fixture
  hostname is reachable. This fixture deliberately has no egress relay, isolating
  target policy; it does not test the production pinned HTTPS CONNECT boundary.
- Initial run had a fixture dependency: the failed credential assertion skipped
  navigation back to the ordinary page URL, invalidating a subsequent redirect
  control. Moving cleanup before the assertion corrected that fixture only;
  the original log remains `build/browser-network-policy-page-20261121.log`.
- Final comparison command **failed in 1.470 s**: five passing/eight total leaf
  cases, plus the failed parent (Node reports nine tests, five pass/four fail).
  Allowed POST, denied foreign fetch and 307 redirect controls passed.
  Credential-bearing navigation reached the receiver despite the native pattern;
  an ungated dedicated worker and a popup each sent a first forbidden request.
  A selective native Fetch pattern `*://*@*` paused exactly one credential request
  and prevented its HTTP effect. The **actual current BrowserDriver** also blocked
  the same credential navigation; a native page-only replacement would regress
  that boundary. Log:
  `build/browser-network-policy-production-comparison-20261121.log`.
- Installed CDP types provide positive wildcard/resource/stage Fetch matching,
  with no native allow/exclusion field. Ordered allow/block is available in the
  separate Network URLPattern interface. Pure Node URLPattern behavior was not
  treated as Chromium enforcement proof; the actual credential failure above
  demonstrates why that distinction matters.
- An actual public browser-root `Target.setAutoAttach` call rejected
  `flatten:false` with the native error that browser-level auto-attach supports
  only flattened mode. A broad flattened filter also rejected simultaneous tab
  and page inclusion. Excluding tab allowed the attachment, but a second manual
  nonflattened attachment could not complete `Network.enable` through
  `Target.sendMessageToTarget` within three seconds; navigation remained paused
  and the receiver saw zero effects. This is an unusable attempted bridge, not
  successful target coverage. Scratch source and logs:
  `build/browser-native-target-nested-probe.mjs`,
  `build/browser-native-target-protocol-20261121.log`,
  `build/browser-native-target-nested-trace-20261121.log`.

No debugging TCP endpoint, Playwright private transport API, production network
change or policy exception was introduced. No browser probe process remained at
the stopping checkpoint. All-target/worker/popup policy, exact literal escaping,
data/blob/protocol behavior, composed private-destination denial, 512 MiB delivery
under a qualified replacement and teardown remain incomplete. The investigation
must not be resumed without a subsequent parent task instruction.
