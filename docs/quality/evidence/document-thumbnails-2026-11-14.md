# Bounded document page thumbnails — 2026-11-14

Implementation under document-ingestion-reliability 10.1, using the existing
Chat/Knowledge source reader. This does not close the full reader/library scope.

## Behavior

- Multi-page originals offer an optional **Pages** strip. It displays four
  thumbnails at a time, marks the current page and identifies pages needing
  review. Selecting a thumbnail navigates the existing source reader. Group
  controls, keyboard activation, narrow RTL and theme tokens are shared.
- Source previews advertise `thumbnails_available`. Older services retain their
  existing reader behavior. The additive `thumbnail=true` page request produces
  a PNG no larger than 192 by 256 pixels or 256 KiB.
- Thumbnail rendering shares extraction admission, never invokes OCR, preserves
  original bytes and rechecks source access before delivery. SHA-256 and
  derivative-kind response headers bind the preview. Responses are not cached.
- The renderer makes sequential requests only while the strip is open. Closing,
  changing source/group or opening extraction review aborts pending work and
  revokes object URLs. At most four small images remain decoded. Byte and PNG
  dimension bounds are checked before browser image display; oversized streams
  are cancelled. Wrong-source or denied responses display no thumbnail pixels.

## Checks

- `build/document-thumbnails-go.log`: thumbnail geometry/source-preservation,
  malformed query, cancellation and HTTP current-access checks passed. The API
  package had no tests matching that command's filter; it is not counted as a
  tested package in this run.
- `build/document-thumbnails-runtime.log`: the actual packaged PDF worker
  produced 192 by 248 thumbnails for the frozen native `compressed.pdf` and
  scanned `scan.pdf`, without an OCR worker. These are raster tests, not OCR or
  document-answer accuracy evidence.
- `build/document-thumbnails-ui.log`: 32 browser cases passed, including all 29
  interface languages and wrong-source, access-denied and legacy-service cases.
- Screenshot inspection found a doubled disclosure arrow caused by overlapping
  CSS. The extra marker was removed and the heading changed to translated
  **Pages**, avoiding an ambiguous singular-page total. Five final layout/recovery
  cases passed, including light English and narrow dark Arabic. A new response
  bound test initially used the wrong Vite module URL; the corrected test passed
  and confirms both dimension rejection and stream cancellation.
- Production renderer build, type checking, generated API consistency,
  translation completeness, application layout and source-size checks pass.
  The existing large-chunk build advisory remains.

All 29 catalogs contain the new label; native review remains pending. Synthetic
browser page illustrations are explicitly transport/layout fixtures. No installer
or published release is claimed. Version/library workflows and the broader
document quality and hardware qualification gates remain open.
