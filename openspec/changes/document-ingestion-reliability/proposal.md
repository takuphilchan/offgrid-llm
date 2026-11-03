## Why

PDF ingestion can panic when the optional system extractor is missing, while
format support, language preservation and source fidelity are overstated. Users
need dependable local document answers without installing command-line tools or
learning embedding/index configuration. Organization readiness requires evidence
of access isolation, recovery and faithful citations, not a larger extension list.

## What Changes

- Replace the unsafe PDF fallback with a bounded, packaged local parser path;
  diagnose encrypted, corrupt, scanned and unsupported content distinctly.
- Introduce a tested format/readiness registry and extraction contract covering
  page/sheet/slide structure, original-source identity, warnings and partial text.
- Provide local OCR with explicit installed language readiness and bounded page,
  memory, output and time budgets. No silent downloads, cloud OCR or remote code.
- Add durable import stages, cancellation, retry without duplicate publication,
  source inspection and immutable citation locators. Preserve existing indexes.
- Repair Office/Unicode fidelity and qualify real files instead of mocking all
  parser outcomes. Legacy binary Office, ODF and email require dedicated adapters;
  they must not be mislabeled as equivalent text/XML formats.
- Enforce organization collection/document access at import, retrieval, source
  inspection, cache, export and deletion; define retention and recovery behavior.
- Consume the defaults-first Knowledge workflow owned by
  `workspace-outcome-experience`; retain authenticated CLI/API compatibility.

## Capabilities

### User-requested attachment parity

Add conversation-private file uploads to web/desktop Chat and both ordinary CLI
chat modes. Attachments are not automatically published into the Knowledge
library. Share service-owned validation, extraction, source identities and access
checks. Offer multi-file selection/drop, filename/type/status/removal, retained
failed-send drafts and saved-history references. CLI file flags and interactive
attach/list/remove commands use the same contracts. Unsupported formats, missing
OCR and context limits remain explicit; no silent text-only request or truncation.
Agent use must go through a supported, scoped context-reference path before it is
enabled. Merely attaching a file never grants command execution or broad folder
access. This extends the approved document-ingestion implementation scope.

### New Capabilities
- `document-ingestion`: local format readiness, bounded extraction, durable import,
  faithful sources/citations, access isolation, retention and qualification.

### Modified Capabilities
None. Existing role guards and client contracts remain prerequisites.

## Impact

`internal/rag`, document HTTP handlers, storage migrations, parser workers/runtime
packaging, generated API clients, Knowledge UI and cross-platform release tests.
Dependencies must be pinned with redistribution notices and verified provenance.
Installed user documents are not re-imported, deleted or reindexed automatically.
SSO/directory provisioning and legal compliance certification are separate work;
this change cannot claim enterprise readiness from parser unit tests.
## Approved document workspace extension (2026-11-03)

Expose authorized original-page previews and structured extracted units for Chat and
Knowledge. Correct the reported PDF form/image classification without treating
unread figures as understood. Retain partial native text for inspection and
text-only use; never silently promote partial extraction to complete.
UI and CLI share preparation states, source identity and recovery. No cloud
viewer, automatic OCR download or automatic reindexing is introduced.

### Approved readable-text correction (2026-11-03)

Conversation PDFs with substantial native text on every nonempty page may be
used immediately with persisted native-text-only coverage and visible unexamined
visual-content notices. Large scanned surfaces, sparse native text, and unreadable
pages still require OCR or explicit partial-use recovery. This is text readiness,
not full visual interpretation. Existing failed imports recover through retry;
saved ready sources and historical citations are not rewritten.
