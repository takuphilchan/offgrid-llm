# Document formats and recovery

## Extraction coverage and retrieval

New imports, attachments, previews and saved Knowledge passages carry an
`evidence` receipt separately from processing and indexing status. `complete`
describes the extractor's covered source units, not answer accuracy;
`text_only` records known unprocessed content; `partial` records unread or
uncertain units. Older records without a receipt have unknown coverage.
The model receives these limitations with the source. Excerpt selection remains
independent: readable text does not mean every passage was supplied to an answer.
Repeated page warnings are summarized for model context, with the full warning
count and review-unit identities preserved; authorized source records retain the
complete warnings. Usable native text and a need for visual review can coexist
on the same page.

The DOCX note repair applies to newly prepared extractions. Existing saved
attachments and library documents keep their original extraction identity;
reindexing retained text does not recover text the old parser omitted. Updating
those extractions by explicitly re-extracting the original file remains unfinished.
Structured text corrections can create derived revisions and retain earlier
citations, but cannot recover text the old parser never read. Do not delete
existing records to force this migration.

Keyword search treats questions as literal text, preserving short terms,
numbers and multilingual character sequences. Query punctuation is not an FTS
operator. A failed search channel returns an error instead of an empty result
or an undisclosed semantic-only search. Up to 20 candidates per channel are
fused before source bodies are loaded. Ranking scores are not confidence values.

Knowledge retrieval expands retained explicit heading, table-header and footnote
links after selecting passages. Each linked excerpt has its own exact byte range
and source citation; header text is not pasted into a value's citation. New DOCX
extractions recognize explicit repeating-header rows and keep nested tables
separate. They do not infer that every first row is a header or qualify arbitrary
merged-cell layouts. Existing extractions are not silently rewritten.

Saved Knowledge passages have a separate optional `evidence_receipt` for these
links. A missing header or exception caused by bounded expansion or context
truncation produces an incomplete-link notice in all 29 interface languages.
`links_complete` means only that the retained explicit links for this excerpt
were included; it does not prove full extraction, a complete document review or a
correct answer. Newly prepared conversation-private attachments retain the same
structured links, with read-only inspection and separate private source identity.

Before upgrading an existing Knowledge database to the derived multilingual and
binary-vector indexes, the service creates and checks a consistent SQLite
snapshot named `.before-document-upgrade-*.db` beside `rag.db`. A failed snapshot
stops this migration. The snapshot contains private document data and is a
separate retention domain: deleting a live document does not erase the snapshot.
Manage it with workspace backups and retention policy. Restoring an older build
requires a compatible whole-workspace snapshot; this database snapshot alone is
not a complete application backup. New empty workspaces do not create one.

## Read beside Chat

Use **Open document** on a Chat attachment to keep the conversation on the left
and a source reader on the right. On smaller screens, switch between **Chat** and
**Document** without losing the draft. PDFs show locally rendered original pages,
page controls, zoom and search of extracted text. Search does not search unread
pixels. Other supported formats show labeled structured text, not a promise of
original Office layout. Original files remain downloadable.

Retained Knowledge sources now use the same reader. **Review extracted text**
offers a source-unit coverage list and a paginated outline of retained headings,
tables, figures and review items. Selecting an entry opens its exact block in the
same extraction revision. Counts describe stored extraction blocks, not an
accuracy score or proof that all source content was read. Older text-only records
remain readable even when structured navigation is unavailable. **Side by side**
shows original and extracted text together where an original view is available;
on narrow screens the panes stack. A correction draft survives reader-tab changes,
and navigation is disabled until that edit is saved or cancelled.

For a spreadsheet or presentation-table cell, expand **Cell metadata** to inspect
the retained value, extracted display text, number format, formula and cached
result separately. Shared formulas retain their source cell and range; merged
cells retain their spans, and hidden sheets, rows or columns are identified.
Missing formula caches remain unavailable, including shared formulas without
local formula text. An empty retained string is shown as `""`.

These details describe the extraction, not a recalculated workbook or the original
Office layout. Source format identifiers and state codes are preserved verbatim;
counts use the interface language. Opening the disclosure does not change the
document, evaluate a formula, follow a source link or submit a question.

New DOCX extractions resolve declared paragraph-style outline levels, including
same-type style inheritance and explicit body-text overrides. Localized style
names are not guessed. Historical paragraph-formatting changes do not replace
current headings and are disclosed as tracked changes. These additions use parser
revision `offgrid-docx-structure-v6`; they do not rewrite old extraction revisions
or provide rendered Word page coordinates.

Preparation animation runs only while a request is active and stops on success,
failure or cancellation. Reduced-motion preferences use static status. An attached
source indicator during a reply does not mean the model examined every page.

PDF forms are containers and are inspected recursively; their presence alone no
longer means a scan. Raster images or bounded/uncertain structures still need OCR
or review. **Use available text only** explicitly accepts native text for Chat,
with unread content disclosed to the model. It does not certify figure understanding,
complete extraction or publish the file to Knowledge. A fully scanned document
with no native text still needs OCR. Existing failed files need an explicit retry.

CLI conversations offer `/preview-attachment N [unit]` to inspect a selected
source unit and `/attachment-text-only N` to explicitly accept available native
text. `/attachments` shows coverage and failure codes. These commands use the same
authenticated source and preparation APIs; preview never submits a model request.

For a question about a small file, use **Add files** in Chat. You do not need to
configure an embedding model first. To search a reusable document library, open
**Knowledge**, enable the proposed local search model once, and add your files.
An installed search model is offered first; **Change search model** is optional.
A working library does not depend on catalog or download-status availability.
Use **Add document**, then **Ask using Knowledge** once the document is indexed.
Select sources only when you want to narrow the question. **Document options**
contains format details and scanned-document language setup; neither needs to be
opened for ordinary document work. A scan that needs a language pack offers a
direct setup action. Library maintenance is in the header's **Manage Knowledge**
menu, and source-storage details are in each document's management menu.
Ordinary Chat attachments are private to their conversation; adding them does
not automatically publish them to Knowledge.

To reuse a Chat document in your library, open **Saved files** and choose **Add
to Knowledge**. Review the sharing notice: this creates a separate copy available
to people who can access the workspace's Knowledge. If search is not set up yet,
the dialog takes you to its one-time setup and keeps your Chat draft. Adding a
file to Chat, selecting it for another question, or opening its preview never
shares it with the library. If an import is interrupted, inspect **Unfinished
imports** before retrying; there is no need to upload the same file again.

The table describes the current implementation, not a guarantee for every file.
The service's **Document formats** view reports the readers actually present in
your installation. A filename extension or installed runtime is not proof that
the contents were read successfully.

| Format | What is extracted | Important limits and recovery |
| --- | --- | --- |
| Text PDF | Text in page order, including mapped fonts and compressed/object streams; page citations; native AcroForm field values and checkbox/choice states | Requires the packaged PDF reader. Reading order in complex layouts can still be wrong. Inspect the source before relying on an interpretation. |
| Scanned or mixed PDF | Native text plus local OCR where needed, with page provenance | Requires the packaged OCR worker and the selected pinned language pack. Missing language data, low confidence or ambiguous overlap leaves a partial result for review, not a ready document. Representative language accuracy is not qualified. |
| DOCX | Current body paragraphs, tables, referenced footnotes/endnotes, headers and footers | Notes remain beside their references. Referenced headers/footers appear once as identifiable source sections, not inferred rendered pages. Placement, first/even-page visibility and fields need review. Tracked changes remain separate from current text: the reader exposes retained insertions, deletions, moves and formatting-change records with source author/date metadata. Deleted text stays out of current retrieval; historical formatting is not rendered. Images and embedded objects are not interpreted. |
| XLSX | Named sheets in workbook order; typed native values, supported numeric/date displays, explicit table headers, formulas and cache provenance, hidden cells and merged ranges | Up to 100,000 populated cells per sheet. Raw values remain alongside formatted numbers. Unsupported or locale-dependent formats need review; their format IDs/codes remain available. Formula caches may be stale or absent. Charts, images, conditional formatting and macros are not interpreted; no formulas or external links are executed. |
| PPTX | Native slide text, speaker notes and structured table rows/cells in presentation order; hidden slides and visual-part identities | Notes are labeled and linked to their slide evidence. Native table text and merge attributes are preserved; styling alone does not prove semantic headers. Images, charts and embedded objects are identified but not interpreted. Authored visual descriptions are labeled. Master/layout text, animations and visual reading order remain unresolved. Citations identify slides and source parts. |
| TXT, Markdown, supported source-code text | UTF-8 text | Invalid UTF-8 is rejected. Source code is read as text and never executed. |
| CSV, JSON, XML, HTML, RTF | Extracted textual content | Not a rendered browser/document view. Scripts, external resources and embedded objects are not executed or fetched. Exact formatting is not preserved. |
| DOC, XLS, PPT | Unavailable | A dedicated legacy Office adapter is not qualified. Export a copy to DOCX/XLSX/PPTX or a text PDF in a trusted editor. Renaming the extension does not convert it. |
| ODT, ODS, ODP | Unavailable | A dedicated OpenDocument adapter is not qualified. Export a supported copy; preserve the original. |
| EML, MSG, MBOX | Unavailable | A dedicated email adapter is not qualified. Export the relevant body/attachments separately to supported formats. |
| PNG, JPEG as documents | Local printed-text OCR with retained originals, line geometry and source citations | Preview implementation: needs the packaged OCR worker and selected verified language pack. Maximum 24 MiB, 8192 pixels per side and 16 million pixels. Low-confidence text stays partial. Figures, charts, handwriting, signatures and table relationships are not interpreted. Representative multilingual scan quality remains unqualified. |
| TIFF as documents | Implemented, unqualified | Classic multipage TIFF, 24 MiB / 100 pages, up to 16 million pixels per page and 200 million total. Requires local OCR. BigTIFF, subimages, planar samples and JPEG-in-TIFF need conversion. |
| Encrypted, malformed or empty documents | Rejected with a specific error | If authorized, export an unencrypted readable copy. Corruption and emptiness are not treated as successfully indexed content. |

Use **Add files** to prepare PNG/JPEG or multipage TIFF printed records as conversation-private OCR
evidence, or explicitly import them into Knowledge. This is separate from Chat's
image/vision input and needs no vision model. Missing OCR components produce an
actionable failure with the original retained; nothing is downloaded automatically.
The document reader shows the same upright, white-background, 8-bit raster sent
to OCR. This derivative preserves dimensions and never replaces the original;
full colour-profile/HDR fidelity is not qualified. OCR line coordinates address
the original encoded pixel grid, with EXIF rotation/mirroring recorded separately.
Opening the reader reuses saved text and never launches OCR. Available uncertain
text remains inspectable. **Use available text only** explicitly accepts this
limited OCR for Chat without rerunning recognition; the source remains marked
partial and its uncertainty accompanies answers. It does not certify numbers,
complete the extraction, or publish a complete Knowledge index.

Chat preserves original PNG/JPEG uploads. In services advertising
`chat-image-orientation-v1`, primary Exif rotation and mirroring are applied only
to the model-input copy, without resizing or another lossy JPEG encoding. Animated
PNGs and multi-picture JPEGs require an explicitly exported still image. A correct
preview does not qualify the model to transcribe numbers, interpret diagrams or
read a scanned document accurately. Colour-profile/HDR consistency and visual
answer quality still need separate qualification.

## When an import needs attention

1. Open **Unfinished imports**. The accepted file is saved, so another upload is
   normally unnecessary.
2. Use **View source**, when available, to inspect retained partial PDF pages.
   Page states distinguish readable text from pages that still need OCR or review.
3. For missing OCR language data, use the offered language setup and explicitly
   import the verified pack. OffGrid does not silently install a package, call a
   cloud OCR service or download language data.
4. After resolving the cause, choose **Try again**. Review alone never retries,
   indexes partial content or sends a model request.

Choose the OCR languages before adding scanned files. The choice is independent of
the interface language and remains with that source through retries and explicit
copying to Knowledge. The pinned catalog offers 25 packs, mapped to 26 interface
locales. Shona, Ndebele and Zulu currently have no pack; OffGrid does not substitute
a related language. Select up to three languages present in the document; the
first is primary. Use **Add language** for another pack and the remove control to
change the selection before import. Every selected pack must be installed. A
missing pack never silently reduces the selection. Page-specific hints and
deskew remain in development. Pack installation verifies identity, not
transcription accuracy.

Saved OCR pages survive interruption and restart. The unfinished-file view reports
their count; **Try again** validates and reuses them while processing remaining
pages. Native extraction and raster preparation may repeat. Saved uncertain OCR
stays uncertain, and completion of OCR does not interpret figures or tables.
Removing a source removes its saved page receipts; downloaded copies and backups
remain separate retention domains.

If the packaged reader itself is missing, install a matching OffGrid package that
includes it. Installing random utilities on PATH does not repair this execution
path. Keep the original file; a partial preview is not a substitute for it.

CLI users can inspect `offgrid kb status --json`, `offgrid kb imports`, and
`offgrid kb review-import ID`. The authenticated API exposes the same readiness,
import and source states. Advanced runtime details are diagnostic information,
not settings users must repeatedly configure.

## Bounds and source identity

Default bounds are 50 MiB per input, 16 MiB extracted text, and 64 MiB / 10,000
entries in expanded Office archives. Deployment configuration can impose smaller
limits. Original bytes, pending imports and retained partial extraction share a
2 GiB Knowledge storage limit. Cancelled or interrupted work is never silently
published as ready.

Knowledge imports and Chat attachments share one active document reader per
service process, with at most four waiting reader operations. Knowledge also
serializes index publication. Each queue wait is limited to 30 seconds; a full
queue or expired wait returns `document_busy`, preserving an accepted original
for an explicit retry. Reading does not reserve an inference slot, and the reader
is released before indexing. These limits bound concurrency, not total service
memory or model latency.

During an import, the interface distinguishes waiting for other documents,
reading the document and making it searchable. Status checks only observe saved
progress; they never restart an import. A busy response stops the current batch
and leaves remaining files unsent.

New indexed documents retain the source digest, extraction revision and exact
UTF-8 passage ranges. Saved citations reopen the matching passage or report that
it is unavailable/stale. Existing indexes remain readable and do not acquire
invented page or cell identities; reimport is explicit. Source inspection still
requires current access. Current published-library access is workspace-based;
private collection policy and broader organization retention work are unfinished.

## What has actually been tested

If OCR times out or its worker cannot finish a page, OffGrid retains verified
native text and any completed page results as partial evidence. It does not
automatically send that partial extraction to a model. Open the document to
inspect it, then explicitly choose **Use available text only** for text-based
questions, or retry preparation. Native-text-only answers exclude unread image
content. OCR can recover image text; it does not establish the meaning of charts
or diagrams, even when their labels are readable.

In Chat, PDFs with substantial native text on every nonempty page and bounded
visual regions are usable immediately as **text only**. The attachment says
that images and figures have not been interpreted; the reader links to affected
pages, and that coverage limit accompanies the model's source context. This does
not establish complete visual understanding. Sparse headers, large scanned
surfaces and unreadable pages still use OCR or explicit partial-use recovery.
For a saved attachment that failed in an older build, choose **Retry preparation**
once after updating. Existing ready source revisions remain unchanged.

The [November 2 score report](../quality/evidence/document-format-score-2026-11-02.json)
uses the frozen 37-file public corpus with the actual pinned Windows PDF worker,
no PATH extractor and no OCR language pack:

| Fixture group | Result |
| --- | --- |
| Text-bearing PDFs, including object streams and a large unused stream | 11/11 expected text matches and valid retained source ranges |
| Scanned/mixed PDFs without OCR | 3/3 expected OCR-required results; these are successful failure classifications, not successful OCR |
| Encrypted, blank and corrupt PDFs | 4/4 expected rejection classes |
| UTF-8 text, Markdown, CSV, JSON, XML, HTML and RTF | 11/11 expected text matches and valid retained source ranges |
| DOCX, XLSX and PPTX | 3/3 expected text matches and valid retained source ranges |
| Invalid text/JSON, corrupt or empty Office files, expanding archive | 5/5 expected rejection classes |

This is **25 extracted fixtures and 12 expected refusals**, not 37 successful
document imports. Tests also reject all 14 listed unsupported extensions instead
of interpreting them as arbitrary plain text; there is no positive extraction
qualification for those formats. Dedicated OCR, exact-source readback, restart,
client and packaging results are recorded in the
[document-ingestion evidence](../quality/evidence/document-ingestion-2026-11-01.md).

These results do not establish model-answer accuracy, all-language OCR, clean
installation on every platform, enterprise isolation or compliance. Those remain
separate release gates. The original failed baseline remains in the evidence.

### Large attachments and context

Chat keeps the original file and complete extracted text. When they cannot fit in
the selected context alongside the conversation and response reserve, it supplies
selected exact passages relevant to the question, with introductory/distributed
passages for orientation. The answer records source revisions and extracted-text
byte ranges; these are not PDF page numbers. Web and CLI label this as selected
document passages, **not a complete review**. Uninterpreted figures remain marked.

This local passage selection requires no embedding download or model switch.
For exhaustive reviews, inspect the source and ask focused questions about its
sections; passage selection alone cannot establish complete coverage. If even
bounded passages cannot fit, the draft stays available and Response settings
offers the existing context controls. History and files are not discarded.

### Follow-up questions and context

A file stays attached to its saved conversation; you do not need to upload it
again for each question. OffGrid selects relevant exact excerpts for the current
question. On the llama runtime, it measures the complete prompt with the loaded
model's tokenizer before generation. This includes prior messages and any selected
Knowledge context. It does not count UTF-8 bytes as history tokens.

**Maximum response tokens** limits one answer; it is not the conversation's context
size or a limit on the number of questions. The answer allowance, history and
source excerpts share the context. The default answer allowance is 1,024 tokens;
a manually selected 4,096-token allowance reserves more space. If a conversation
really fills its allocation, lower the answer allowance or select **Extended**
when the model and available memory support it. History is never silently removed,
and the service does not automatically switch models or enlarge the allocation.
Providers without token-counting support retain conservative budgeting. Selected
excerpts are still a limited view of the document, not full visual understanding.

## Native forms and tracked revisions

New PDF extractions preserve native AcroForm names, values, alternate labels,
checkbox/radio states and selected choice options separately from page OCR.
Hidden fields and signatures are marked for review. Field coordinates retain
PDF user-space provenance; no label association or signature authentication is
implied. Forms do not execute scripts, actions, calculations or remote requests.
A searchable value still carries a form-review warning. This does not qualify
complex PDF layouts, flattened-form recognition or XFA.

New DOCX extractions retain tracked alternatives in body text, referenced notes
and referenced headers/footers. The shared reader opens these on demand beside
the current text. Source author names and dates are unverified document metadata.
Deleted and moved-from text is not submitted as current Chat or Knowledge evidence.
Historical formatting is recorded as a change, not reconstructed. Existing
extraction revisions and citations remain unchanged.

Collection-owner audit records now include extraction corrections, retention
updates, hold addition/release and source deletion/expiry. These records commit
with the operation; a failed audit write rolls back that operation. Events retain
source identifiers, actors and revisions, not document bodies. Deleting a source
does not delete its audit history or recall previously downloaded exports.
