# Document formats and recovery

## Read beside Chat

Use **Open document** on a Chat attachment to keep the conversation on the left
and a source reader on the right. On smaller screens, switch between **Chat** and
**Document** without losing the draft. PDFs show locally rendered original pages,
page controls, zoom and search of extracted text. Search does not search unread
pixels. Other supported formats show labeled structured text, not a promise of
original Office layout. Original files remain downloadable.

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
| Text PDF | Text in page order, including mapped fonts and compressed/object streams; page citations | Requires the packaged PDF reader. Reading order in complex layouts can still be wrong. Inspect the source before relying on an interpretation. |
| Scanned or mixed PDF | Native text plus local OCR where needed, with page provenance | Requires the packaged OCR worker and the pinned English language pack. Missing language data, low confidence or ambiguous overlap leaves a partial result for review, not a ready document. Other OCR languages are not qualified. |
| DOCX | Body paragraphs and tables | No rendered page numbers, headers, footnotes, images or embedded objects. Source locators identify extracted sections. |
| XLSX | Named sheets in workbook order; stored cell values, inline/shared text, formula text and cached results | Up to 100,000 populated cells per sheet. No recalculation, number/date rendering, chart interpretation or macros. Cached values may be stale. New citations identify the cells actually present in each excerpt. |
| PPTX | Slide text in presentation order | No speaker notes, chart/image interpretation or embedded documents. Citations identify slides. |
| TXT, Markdown, supported source-code text | UTF-8 text | Invalid UTF-8 is rejected. Source code is read as text and never executed. |
| CSV, JSON, XML, HTML, RTF | Extracted textual content | Not a rendered browser/document view. Scripts, external resources and embedded objects are not executed or fetched. Exact formatting is not preserved. |
| DOC, XLS, PPT | Unavailable | A dedicated legacy Office adapter is not qualified. Export a copy to DOCX/XLSX/PPTX or a text PDF in a trusted editor. Renaming the extension does not convert it. |
| ODT, ODS, ODP | Unavailable | A dedicated OpenDocument adapter is not qualified. Export a supported copy; preserve the original. |
| EML, MSG, MBOX | Unavailable | A dedicated email adapter is not qualified. Export the relevant body/attachments separately to supported formats. |
| PNG, JPEG, TIFF as Knowledge documents | Unavailable | Standalone image-document OCR is not qualified. Chat image input uses a separate vision model and does not provide Knowledge indexing. |
| Encrypted, malformed or empty documents | Rejected with a specific error | If authorized, export an unencrypted readable copy. Corruption and emptiness are not treated as successfully indexed content. |

## When an import needs attention

1. Open **Unfinished imports**. The accepted file is saved, so another upload is
   normally unnecessary.
2. Use **View source**, when available, to inspect retained partial PDF pages.
   Page states distinguish readable text from pages that still need OCR or review.
3. For missing English OCR data, use the offered language setup and explicitly
   import the verified pack. OffGrid does not silently install a package, call a
   cloud OCR service or download language data.
4. After resolving the cause, choose **Try again**. Review alone never retries,
   indexes partial content or sends a model request.

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
