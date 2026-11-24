# Native browser frame provenance — 2026-11-20

Status: bounded helper implementation for the download increment. It is not yet
the production download path, does not complete M2.07 and does not qualify an
attached personal browser. No new runtime capability is advertised.

`apps/computer/browser-frame-proof.mjs` uses a fixed isolated custom selector on
the exact retained ElementHandle. A temporary native CDP binding identifies its
live execution context and frame without matching URLs, writing DOM markers or
using private Playwright object IDs. Inline frames share their nearest native
target session; separate-process frames retain their own public Frame/CDPSession
binding. Native context unique identity and frame loader identity prevent a
same-frame navigation from preserving the previous document proof.

Bindings are installed only in native-observed isolated context names, never
based on a hard-coded private Playwright world name. Main-world events cannot
establish proof, even when a test deliberately exposes the nonce there. The helper
removes both binding subscriptions and isolated globals; context/listener bounds,
navigation/closure tracking and exact-handle validation reject changed state.
The caller must still validate this proof at download admission and retain its
owner until stream/session cleanup is confirmed.

Actual Chromium 151.0.7922.34 checks:

- Same-URL inline siblings receive different native frame identities; each agrees
  with the request produced by clicking its actual retained button.
- No DOM mutation or binding global remains after successful proof; listeners
  retire after closure, and navigation invalidates the old document.
- Detached/replaced controls and pre-aborted proof cannot rebind a target.
- Separate Chromium frame processes remain identifiable across the supported
  public CDP API; a real cross-process navigation invalidates their proof.
- A deliberately forged default-world event cannot replace child-frame evidence.
- Document replacement during binding setup cannot certify the new target.

Initial three-case run: **3/3 passed, 1.176 seconds**. The first expanded run was
**four passed / one failed, 1.803 seconds** because `Frame.goto` rejected a
cross-process transition with “Target page, context or browser has been closed.”
A bare Playwright probe with no OffGrid helper reproduced that rejection while
the browser/page remained alive and the child URL had changed. The acceptance
fixture now initiates the same transition through the page's iframe and waits
for Chromium's actual navigation event before checking proof rejection. This
does not resolve or hide the separate pinned-library `Frame.goto` behavior.

Final command passed **6/6, 2.177 seconds**:

```powershell
node --test --test-concurrency=1 apps/computer/test/browser-frame-proof.test.mjs
```

Logs: `build/browser-frame-proof-20261120.log`,
`build/browser-frame-proof-expanded-20261120.log`,
`build/browser-frame-goto-baseline-20261120.log` and
`build/browser-frame-proof-final-20261120.log`.

Primary references: [Playwright custom selector isolation](https://playwright.dev/docs/api/class-selectors#selectors-register),
[public CDP sessions](https://playwright.dev/docs/api/class-browsercontext#browser-context-new-cdp-session)
and the [Runtime protocol](https://raw.githubusercontent.com/ChromeDevTools/devtools-protocol/master/pdl/js_protocol.pdl).
The Runtime contract explicitly separates removing a binding subscription from
removing its JavaScript global, which is why both cleanup steps are exercised.
These fixtures use disposable pages/processes and synthetic loopback or locally
fulfilled URLs. They establish no real-model/site, installed-platform, full
download or human usability qualification.
