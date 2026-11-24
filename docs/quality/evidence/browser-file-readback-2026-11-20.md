# Native browser File readback evidence — 2026-11-20

This is a bounded implementation/evidence slice of
[browser-file-transfers](../../../openspec/changes/browser-file-transfers/tasks.md),
roadmap M2.07. It does not complete the eight transfer tasks, establish a remote
upload, or qualify real sites, models, installed platforms or maximum-size
performance. The existing upload snapshot and download lifecycle are separate
parts of the same change.

## Boundary and implementation

The previous browser upload readback evaluated `input.files[0].name` in the page
world. A page can replace a selected File with same-name, same-size different
bytes, or override page-world getters. The negative fixture retains the exact
old expression as a positive control: it returns true for wrong selected bytes,
while the new complete readback rejects them.

`apps/computer/browser-file-readback.mjs` registers one fixed engine before page
creation. `browser-file-readback-engine.mjs` runs only as an isolated content-script
selector against the already retained exact ElementHandle. The selector contains
host-generated operation/token/chunk metadata, never a model expression or
selector. Its only successful match is that same scope node. It creates no
carrier elements, DOM mutations or page-visible file-byte buffers.

The isolated closure uses native input/FileList/File/Blob/Node getters and
FileReader plus WebCrypto. It retains the exact native File and document,
checks connectivity and current selection for every chunk and final completion,
and compares each at-most-1 MiB actual File chunk with the SHA-256 of the same
chunk independently read from the private snapshot. Node also hashes the full
snapshot again against its selected grant. Success therefore establishes complete
byte equality and exact name/size at file selection, not remote receipt or future
page behavior. No 512 MiB whole-file browser ArrayBuffer is required.

The isolated map admits at most eight active records per document. Explicit
release aborts FileReader, clears its record and retires callbacks; a bounded
expiry also retires abandoned records. Node clears its reusable buffer and closes
the exact snapshot descriptor in `finally`. Browser context closure remains the
owner's terminal cleanup boundary. Stop must pass the snapshot owner's abort
signal when this module is integrated. A post-assignment mismatch or timeout must
retain dispatched uncertainty and must never trigger another file selection.

## Executed checks

On the already installed Playwright 1.62.1 / Chromium 151.0.7922.34, without any
browser or dependency download:

```text
node --test --test-concurrency=1 apps/computer/test/browser-file-readback.test.mjs
Initial feasibility checkpoint: PASS 6/6, 2.630 s
Expanded checkpoint: FAIL 9/10, 3.660 s (fixture assertion described below)
Corrected expanded suite: PASS 10/10, 5.818 s
```

Those six actual Chromium cases cover a no-DOM-mutation positive control;
page-world own-input/File/Blob/FileReader/crypto/Element/Node spoofing; same-name,
same-size wrong-byte rejection; detached and replaced retained inputs; delayed
51 MiB native File content; and cancellation/timeout followed by renewed explicit
verification. The expanded suite also executes empty-file evidence, ten actual
chunk-start cancellations followed by successful verification (exceeding the
eight-slot bound if records leaked), mid-read replacement, and exact child-frame
selection with a wrong-byte top-frame sentinel.

The initial expanded failure was a fixture assumption: Chromium's `cloneNode`
retained the native File selection. The verifier correctly rejected the detached
original, but the fixture incorrectly expected the clone to be empty. Replacing
it with a newly created empty native input preserved both strict rejection and
the no-reselection assertion; no production code was changed for that correction.
These remain fixture checks, not model qualification.

Light repository gates also passed: maintained docs 474 files / zero problems,
source-size ratchet clean, and strict `browser-file-transfers` validation. The
full companion/desktop/Go/renderer gates belong to the parent integration; these
focused results do not claim they were rerun for this module.

The parent independently reviewed and integrated the helper into the production
driver: registration precedes page creation; the exact retained input and snapshot
abort signal are used; readback has a 120-second bound; receipts explicitly mean
selection; both helper modules enter the packed runtime. An actual page change
handler replacing the native File with same-name/same-size wrong bytes reproduced
the previous driver failure (missing rejection, 0.612 s), then passed after
integration. The parent's integrated snapshot/readback/transfer/legacy/pack
command passed 32/32 in 9.490 s; the pack file in that invocation defines no
individual test cases. Logs are
`build/browser-file-readback-driver-baseline-20261120.log` and
`build/browser-file-readback-integrated-20261120.log`.
These are parent-reported executed integration results, separate from this
reviewer's ten-case helper run above. Post-assignment failures remain uncertain.

The fixture creates only disposable directories, browsers, loopback HTTP servers
and synthetic content. Every browser is awaited through close before its owned
snapshot and root are removed. It never accesses personal Chrome/Edge sessions.

## Primary API evidence and limits

Read on 2026-11-20: Playwright's
[custom selectors](https://playwright.dev/docs/api/class-selectors#selectors-register)
and [extensibility documentation](https://playwright.dev/docs/extensibility)
document `contentScript: true` isolation against page prototype modification.
They also warn that combining other custom engines may select the main world.
This implementation therefore invokes only its fixed engine through the exact
retained handle. Pinned local `playwright-core/lib/coreBundle.js` was inspected:
selector parsing chooses the utility world unless a main-world engine is present;
scope handles are adopted by identity rather than reselected. No private
Playwright object ID, channel or world-name API is consumed by application code.

Public [CDP DOM.resolveNode](https://chromedevtools.github.io/devtools-protocol/tot/DOM/#method-resolveNode)
can resolve a known backend node into an isolated execution context, but pinned
Playwright exposes no public ElementHandle-to-backend-node conversion. A guessed
selector, coordinates, page-visible marker or unsupported private handle field
would weaken exact target identity; this implementation does not use them.

The review found no additional confirmed orderly-close leak in the sampled
snapshot sharing/cleanup paths. An abrupt companion death still needs durable
upload snapshot ownership/reconciliation: the current upload store's random
OS-temp path alone is not a persistent cleanup record. Keep that recovery work and
the full transfer negotiation/artifact integration open; do not sweep unrelated
temporary directories or treat this readback proof as their completion.
