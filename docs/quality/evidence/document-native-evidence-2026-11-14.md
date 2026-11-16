# Native forms, tracked Word changes and lifecycle audit

Implemented in document-ingestion-reliability; no installation or release is claimed.

## Delivered behavior

- PDF worker revision pdfium-8057-0001cb839e62425b-offgrid5 retains native AcroForm values, labels, checkbox/radio states, selected choice options, hidden state and PDF user-space bounds. Fields remain separate from OCR. Signatures and ambiguous field associations require review; no scripts, calculations or remote resources execute.
- DOCX parser revision offgrid-docx-structure-v7 retains tracked insertions, deletions, moves and formatting-change records in body text, referenced notes and referenced margins. Current visible text remains searchable; deleted text is available only as a review alternative. Source author/date metadata is not authenticated.
- Shared reader alternatives are collapsed by default, keyboard operable and translated across all 29 catalogs. Original files, extraction identity and historical citations remain intact. Narrow English and Arabic screenshots were inspected; author/date layout was corrected after inspection.
- Correction, retention/hold and deletion/expiry audit events commit atomically with the operation. Audit failure rolls back mutation. Owners can inspect surviving private-collection audit history after deletion; readers, editors and unrelated administrators cannot. Events contain identifiers and provenance, not document bodies.

## Executed checks

- Actual local bge-m3 Knowledge HTTP import, hybrid retrieval and exact citation readback: private collection plus ten format/evidence cases, including tracked Word and native PDF form. Private PDF and tracked Word HTTP-to-Chat checks pass. Log: build/native-documents-real-final.log.
- Full document, rag, sessions, serviceclient and API suites pass; rag includes opt-in actual packaged PDF cases. Log: build/native-documents-audit-final-go.log.
- Full pinned PDF worker suite passes (33.493 seconds). Log: build/pdf-native-fields-final-worker.log.
- Transaction-failure, held-source, correction rollback, expiry/restart and audit authorization regressions pass. Logs: build/document-audit-final.log and build/document-audit-http.log.
- All 29 tracked-alternative language fixtures pass; final narrow English/Arabic presentation checks pass. Logs: build/docx-alternatives-final.log and build/docx-reader-polished.log. The earlier all-language native-form suite also passes: build/pdf-form-ui.log.
- Generated API drift, TypeScript and source-size/documentation checks passed before the final presentation/test edits. Final renderer checks are recorded separately below.
- Translation packs exported to build/translation-review-documents-native-20261114. There are 1,521 authored contextual messages per supported catalog; native linguistic approval remains pending.

## Failures retained and corrected

The initial broad HTTP filter also selected an unrelated curated-model preview test, which failed on insufficient free disk space. Temporary files were redirected to D:. A new PDF fixture initially entered the Office-only manifest and broke that harness; it now has a separate digest/provenance manifest and regression. The initial new translation module bypassed preview feature packs; translations were moved into the established pack mechanism. A Go compiler and renderer attempt hit Windows memory/commit exhaustion; Go checks passed with bounded compiler concurrency. These failed logs remain under build rather than being counted as passes.

## Remaining scope

This does not finish task 8.2 or the seven-phase program. PDF layout/table extraction, remaining Office fidelity, page OCR hints/deskew, optional offline Docling packaging, complete shared/private retrieval and analyses, departmental metadata and remaining lifecycle coverage are still implementation work. Complex visuals/archive formats remain gated behind printed-document qualification. Native review, the licensed human-labelled corpus, 8/16 GB performance profiles, clean offline installation and a departmental pilot remain open. Actual embedding/citation tests do not qualify generated-answer accuracy. No installation, commit or publication was performed.

## Final verification follow-up

The renderer production build passed with a 1 GB Node heap after the resource
failures (build/native-document-renderer-final.log). Vite retains its bundle-size
warning. The final native-form/tracked-alternative suite passed 58 tests across
all 29 languages using one headless worker, including keyboard activation
(build/native-document-ui-rechecked.log). API generation drift, source-size,
385 maintained Markdown files and strict OpenSpec validation pass. The separate
native-form fixture identity regression passes.

The translation check caught missing source-tracking entries for the two new
labels after their preview translations moved into feature packs. Those source
entries are now recorded; the check passes with 1,521/1,521 messages and zero
missing catalog messages (build/native-document-i18n-rechecked.log). This records
source identity, not translator approval. Native review remains pending.
