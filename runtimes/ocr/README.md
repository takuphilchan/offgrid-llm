# Local OCR worker

This runtime is implemented for isolated evaluation. It is **not yet a packaged,
qualified Knowledge capability**. Broader layout/language qualification, runtime
packaging and clean-install checks remain open in
`document-ingestion-reliability`.

## Runtime boundary

The pinned Tesseract 5.5.4/Leptonica 1.87.0 engine uses a narrow memory-only C ABI.
The Go supervisor runs it with wazero, a 512 MiB WASM ceiling and a 40-second
request deadline. The engine receives verified language bytes and decoded RGBA,
not paths, URLs, credentials, shell commands or arbitrary runtime options.
Image codecs, networking, training, legacy recognition and diagnostic fonts are
disabled. WASI has no mounted filesystem. Imported directory/mutation syscalls
are explicitly denied; unknown imports reject activation.

The parent launches only the app-local worker and validates protocol/revision,
image/language digests, dimensions, right-angle transforms, bounded hOCR and
confidence before accepting text. OCR source coordinates refer to the original
page. Four orientation attempts share one deadline; insufficient remaining time
returns the validated partial result rather than starting an unbounded retry.
Engine confidence is a quality signal, not proof that every word is correct.

Only the pinned English pack is currently accepted. Explicit offline import is
available through `offgrid kb import-ocr-language eng PATH`, Python's
`client.kb.import_ocr_language("eng", path)` and Knowledge's optional **Read scanned
documents** setup. `offgrid kb ocr-languages` reports its state. The authenticated
service accepts only verified data and uses atomic replacement under its data
root. Each new import takes an immutable pack snapshot. There is no automatic
download, reindex, model change or runtime installation.

Native-text and scanned pages in one PDF retain their page order and provenance.
Mixed content on one page uses verified native-word rectangles to retain native
Unicode and incorporate distinct OCR words. Ambiguous intersections or layouts
that would change native reading order remain partial. Repeated words at different
positions remain present; source text is never deduplicated by spelling alone.
Installed pack state and runtime qualification are reported separately.

## Offline build

`source.lock.json` pins source and Emscripten archives. Obtain those archives
explicitly, then run in an isolated Linux build environment:

```sh
python3 scripts/build/build-ocr-runtime.py \
  --archives /path/to/source-archives \
  --toolchain /path/to/wasm-binaries.tar.xz \
  --work /path/to/dedicated-build-cache \
  --output /path/to/staging/ocr.wasm
```

The script verifies input digests, uses an owned build directory and installs no
system dependencies. CMake, Ninja, Node and Python must already be available.
It checks both the output engine and redistribution notices against
`engine.lock.json` before copying staged assets. No compiler or model download
occurs. Two independent build directories on the same WSL host produced matching
engine and notice digests. Cross-host reproducibility still needs CI evidence.

Stage both verified files into the ignored `worker/assets` directory, then build
the worker from its separate Go module:

```sh
python scripts/build/prepare-ocr-runtime.py --assets /path/to/staging
go -C runtimes/ocr/worker build -mod=readonly -trimpath -o ../../../build/offgrid-ocr-worker .
```

Maintainer CI uses `prepare-ocr-runtime.py --build-from-source CACHE` in an
isolated Linux builder. This explicitly acquires pinned source/compiler archives
before running the same offline build and digest checks. Release jobs share the
verified engine/notices artifact. Desktop packaging requires both PDF and OCR
workers and verifies the copied binaries and notices. Container builds compile
the same sources in a separate build stage. End-user imports never invoke this
tooling. English language data is imported separately by the user.

Use the platform executable suffix on Windows. The runtime verifies the embedded
engine digest, and `--licenses` exposes its embedded redistribution notices.

## Evidence and remaining gates

The real worker passes seven readable authored PDF cases, including all four
visible rotations and the recognition-only mixed-page case. The clipped negative
fixture stays partial. Parent subprocess and full typed PDF extraction tests
also pass. See `docs/quality/evidence/document-ingestion-2026-11-01.md` for commands,
timings and failures retained from earlier attempts.

The initial Tesseract 5.3.0/WASM experiment required an obsolete runner ABI and
was rejected. The initial native 5.5.4 bridge scored 5/7: its 180/270-degree
failures remain recorded under `internal/rag/testdata/documents/ocr/native`.
Those historical receipts have not been rewritten to imply success. The current
worker corrects those fixtures with bounded orientation attempts; this does not
qualify arbitrary documents or languages.

Packaging on supported platforms, broader mixed-layout and language coverage,
durable import UX, retrieval/citation accuracy and organization access/retention
must pass their own gates before release. Compilation and fixture recognition
alone are insufficient.
