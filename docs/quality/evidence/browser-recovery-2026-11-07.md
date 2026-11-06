# Browser recovery review — 2026-11-07

## Fixed failure

`browser_download` started its download listener before clicking, but did not
attach a rejection handler until the click completed. A failed click could
leave a later timeout unhandled; a slow click could also let the listener reject
first. Both promises are now observed immediately with `Promise.all`.

The click is dispatched once. Failure does not authorize another click, and
the old observation remains invalidated. This change does not interpret a
timeout as proof that no external effect occurred.

`npm --prefix apps/computer test`: 75 passed, zero failed or skipped. The new
`browser-download-failure.test.mjs` exercises both failure orders, confirms no
artifact was produced by the fake provider, and rejects reuse of the stale
observation. These are deterministic fault tests, not live website qualification.

## Boundaries and remaining improvements

The managed browser uses isolated Chromium and an approved origin. It does not
inherit a personal browser profile or permit unrestricted cross-origin research.
Observations are bound to page identity, revision, form state and age. Action
dispatch passes through session authorization and durable receipts.

The session currently treats non-policy provider errors conservatively as an
uncertain outcome. Distinguishing pre-dispatch observation failures from failed
mutations could improve recovery, but requires a typed effect contract and
crash-boundary tests; it must not become a blanket retry policy. Large-page
observations are bounded, and text verification establishes a mechanical text
condition rather than successful research or semantic accuracy.

No permission broadening, personal-browser automation or new browser-workflow
qualification is claimed by this review.
