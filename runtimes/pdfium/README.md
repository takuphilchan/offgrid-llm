# Local PDF worker (implementation under qualification)

This separate Go module builds `offgrid-document-worker`. It embeds PDFium's
WASM engine through a small, pinned Emscripten binding and wazero. The main service
does not acquire these dependencies or change its minimum Go version.

Prepare the pinned assets, then build from the repository root with Go 1.26.6:

```text
node scripts/build/prepare-document-runtime.mjs
go -C runtimes/pdfium build -mod=readonly -trimpath -o ../../build/offgrid-document-worker.exe .
```

The preparation command explicitly downloads the locked community PDFium build.
For offline builds, use `--archive PATH` with the exact archive identified in
`engine.lock.json`. It verifies the archive, module, version, disabled scripting
features and complete redistribution-notice digest before writing ignored assets.
Document uploads never download code. `--licenses` prints the embedded notices.

The `.exe` output name is used by the cross-platform fixture command; production
non-Windows packaging uses `offgrid-document-worker` without that suffix. The
service looks only beside its own executable, never on PATH. It does not download
or install this worker on demand. Packaging and redistribution qualification are
still open; the worker has not been installed in the user's workspace.
If it is absent, PDF import returns `document_parser_unavailable`; the old PATH
and regex fallback has been removed. A discovered executable is reported as
`installed_unchecked`, never automatically qualified.

## Protocol and boundaries

One invocation accepts PDF bytes on stdin (50 MiB maximum), and returns one JSON
receipt with protocol 1, source SHA-256, pinned extractor revision and ordered
page text/states, or a closed-set error code. There are no filename, command,
URL, password, JavaScript or credential operations. The parent validates the
receipt and never falls back after a worker error.

The parent serializes extraction independently of inference, strips inherited
environment credentials/proxies/PATH, bounds output and kills its owned process
after cancellation or a 30-second deadline. The worker has a 25-second deadline,
1000-page limit, 16 MiB text limit, preallocation character checks and bounded
page-object inspection. Its WASM memory is capped at 512 MiB, with context-based
interruption. The closed import table supplies an empty environment and rejects
filesystem operations; no filesystem mounts or network imports are provided. Unknown
imports fail initialization. `GOMEMLIMIT` is
a Go GC target, not an OS hard-memory guarantee for the entire worker.

The parent supplies an app-owned per-user compilation-cache directory keyed by
the pinned engine revision. Wazero further keys compiled code by its own version
and module identity. This cache contains immutable engine code, not PDF bytes,
text, page images or credentials. WASI still has no filesystem mounts. A fresh
worker and fresh WASM instance process each document. If the cache cannot be
opened, extraction runs without it; cache setup is not a new user requirement.
Do not relocate this executable-code cache into an untrusted/shared folder.

Scanned pages and pages with unqualified image/form content return `ocr_required`.
Native text remains in source units, but legacy complete-only ingestion rejects
partial results. Logos may conservatively trigger this state until mixed-page
qualification is more precise. OCR and its language assets are separate work.

### Internal page rendering for OCR

`--raster-page N` accepts the same bounded PDF input and emits a separate
`-raster2` receipt containing the original digest, one-based page number, dimensions,
native-word text/device rectangles and PNG bytes. Native regions use PDFium's
character boxes and page-to-device transform, with a 20,000-character bound. The
service validates source agreement and geometry before merging OCR; equal text
at separate positions is not deduplicated. It renders at 200 DPI, honors intrinsic PDF rotation and rejects
pages above 8,192 pixels on either axis or 16 million pixels before bitmap
allocation. PNG output is capped at 24 MiB. Interactive forms and annotations
are not rendered. The service checks the receipt identity, dimensions and full
PNG integrity before accepting any pixels; rendering shares the text worker's
admission and cancellation boundary. No page images are written to disk.

This internal operation is a tested input boundary for the planned OCR adapter,
not OCR support. It does not recognize words, install language data, change
readiness, or make scanned documents searchable. Actual recognition, confidence,
mixed-page text reconciliation and language-pack qualification remain open.

## Provenance and remaining release gates

- Community distribution: `bblanchon/pdfium-binaries`, release `chromium/8057`,
  PDFium version `155.0.8057.0`. This is not a Google-published binary.
- Archive SHA-256:
  `4c7cf42cc4ebe6f5eb0a44cc9d0d1dc1cde8dcd094a5003e229a1bac2d56b0aa`.
- Embedded WASM SHA-256:
  `0001cb839e62425ba5b645c17300027708d916b5bfccf93c2cec7a8376dc3989`.
- All archive `LICENSE` and `licenses/` files are retained in `NOTICE.txt`, embedded
  in the worker and shipped as `document-NOTICE.txt`. Their combined digest is
  locked alongside the archive; the wrapper license alone is insufficient.
- wazero: `v1.12.0`; checksums are in this module's `go.sum`.

The previous custom `go-pdfium` binary had unresolved exact-build provenance.
It is no longer used. A direct substitution failed on Emscripten heap and legalized
`fd_seek` imports; the new binding explicitly implements the pinned ABI. Historical
failures remain recorded in the document-ingestion evidence.

Desktop packaging builds an architecture-specific worker and verifies its digest,
engine identity and notices after copying. Complete CLI bundles and container
builds include the same worker beside the service. Source builds must prepare and
build it explicitly. Mere file presence remains `installed_unchecked`.

The Windows real-document corpus passes with this engine. Clean installer/container
tests, macOS/Linux execution, OCR, large-document resource qualification and full
retrieval quality gates remain separate from compilation and fixture success.
See [recorded evidence](../../docs/quality/evidence/document-ingestion-2026-11-01.md).

For the minimal-container corpus check (authored fixtures only):

```sh
docker build --target document-worker-check -f docker/Dockerfile -t offgrid-pdf-check .
python3 scripts/test/verify-pdf-container.py --image offgrid-pdf-check
```

The runner pins the already-built image ID, disables networking and filesystem
writes, drops capabilities, runs as a non-root user, and bounds memory/processes.
It checks the 18 PDF fixtures and embedded notices without mounting any host data.
This target is a test worker, not a replacement for the running OffGrid service.
