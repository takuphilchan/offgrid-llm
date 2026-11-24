# Offline starter-kit corrections — 2026-11-24

## Release speech review follow-up

The final speech review found that module presence alone could advertise an
espeak-only Kokoro package as runnable with sherpa-onnx 2.0.0, whose frontend
ignores `data_dir`. Readiness now checks distribution metadata against the
source-reviewed versions 1.12.20 and 2.0.0 without importing inference modules;
2.0.0 additionally requires package lexicons. Worker startup independently
checks the loaded version and required generation API before synthesis.
These checks establish compatibility requirements, not acoustic qualification.
Primary references: [frontend change](https://github.com/k2-fsa/sherpa-onnx/pull/4055),
[1.12.20 Python binding](https://github.com/k2-fsa/sherpa-onnx/blob/v1.12.20/sherpa-onnx/python/csrc/offline-tts.cc),
[2.0.0 Python binding](https://github.com/k2-fsa/sherpa-onnx/blob/v2.0.0/sherpa-onnx/python/csrc/offline-tts.cc).

Final Linux WSL `go test ./internal/audio -count=1 -timeout 3m -v` passed in
2.371 seconds, including the actual embedded Python worker against fake model
modules: v1 espeak-only success, v2 espeak-only rejection, unreviewed-version
rejection, both generation APIs, cancellation, leases and Qwen language routing.
No tests skipped in that run. Windows audio checks passed in 8.887 seconds;
that nonverbose result is not used as proof that Python fixtures ran. Logs are
`build/release-audio-final-review-linux.log` and
`build/release-audio-final-review-windows.log`. Source-size and strict voice
OpenSpec checks passed. No model weights, live microphone or installed-runtime
changes were used. Real Kokoro synthesis remains unqualified.

This record covers source corrections for the ten planned 64 GB USB kits. It is
not certification of a prepared master, physical drives or African-language
quality. The working tree contains unrelated concurrent work; these results are
not evidence for a published release or an immutable commit.

## Implemented scope

- Qwen ASR converts supported language codes to upstream language names, accepts
  automatic detection and returns normalized detected language codes. Unsupported
  explicit languages fail clearly. CustomVoice uses automatic language selection
  instead of forcing English.
- Kokoro has a supervised local Sherpa worker, manifest-bound paths and voices,
  dependency readiness, package leases, cancellation and private output handling.
  Zipformer remains unavailable. Kokoro requires separately prepared Python
  dependencies; the current curated voice is English and has not been acoustically
  qualified by this work.
- USB model import stages and validates bytes before publication, preserves
  conflicting installed files and unrelated temporary downloads, rejects invalid
  manifests and reports bulk failures. Legacy registry import uses the same safe
  streaming path. CLI imports no longer restart inference or select a model.
- The maintained USB builder accepts a complete prepared Windows desktop payload,
  checks runtime dependencies and model artifacts, writes only to a fresh output,
  hashes copied bytes and writes the inventory last. Its default budget is 40 GB.
  The baseline speech profile requires Whisper and Piper packages. Optional Python
  engines are not silently included. See the
  [starter-kit guide](../../development/offline-starter-kits.md).

## Automated results

| Check | Observed result |
| --- | --- |
| Windows `go test ./...` | Passed on rerun; retained log `build/offgrid-kit-go-final.log` |
| Windows audio worker regressions | Passed with a real Python interpreter, including Qwen language normalization and Kokoro old/new API, cancellation and lease fixtures |
| `go test ./tools/offline-kit-check ./internal/models ./internal/cli` | Passed after the final schema helper and kit-layout import changes |
| Linux WSL race checks: audio, models, CLI | Passed: 7.523 s, 4.342 s and 8.551 s respectively |
| Linux WSL full server race suite | Failed its default ten-minute timeout; no passing full-server race result is claimed |
| Linux WSL focused server race checks | Passed in 4.768 s for audio ownership, speech/model package import, authorization, inventory and search fixtures |
| `npm.cmd --prefix apps/desktop test` | 292 passed, 4 skipped, 0 failed |
| `node --test scripts/build/create-usb-package.test.mjs` | All 6 passed, including corruption/readback, overwrite refusal, missing runtime, budget and unsafe-path fixtures |
| Web API contract check | Passed |
| Documentation, source-size and application/developer layout checks | Passed |
| Repository hygiene and `git diff --check` | Passed; line-ending warnings only |
| Strict OpenSpec validation for both affected changes | Passed |

The first full Windows Go run failed the existing executor shutdown fixture with
`host_protection_uncertain` and locked-journal cleanup; the subsequent full run
passed. Initial worker runs skipped because Windows Python aliases were unusable;
those skips were not counted as evidence. The worker regressions were rerun with
the existing real interpreter on the command-local PATH. Language/import failure
cases were reproduced before their corrections.

Worker fixtures exercise the actual embedded Python protocol with fake model
modules, not downloaded model weights. Builder fixtures use a disposable fake
application for orchestration; the default CLI retains the real desktop packaging
checks and strict Go speech-manifest validation. No actual kit was certified by
those fixtures.

## Remaining qualification

Release-pass review found and corrected a manifest-selection admission error:
free-space preflight counted GGUF files excluded by the manifest. A deterministic
negative control reproduced the false rejection; the corrected tests verify both
admission of the selected payload and refusal when that payload really exceeds
available space. Full Windows models/CLI/schema-checker suites passed in
7.675/3.257/3.015 seconds, and all six Node builder tests passed. Logs are retained
as `build/release-usb-space-negative.log`, `build/release-usb-full-review.log` and
`build/release-usb-builder-review.log`. The offline-kit implementation and review
correction are committed as `64d3e81b`; this is not publication evidence.

No model weights were downloaded, microphone recordings taken, installed app
replaced, master distributed or release published by this work. A clean offline
machine must still demonstrate chat, transcription and playback using the exact
selected payload. Every advertised language needs native-speaker evaluation,
including accents, names, numbers, code switching, silence and noise. Actual
latency, memory, interruption and audio-device recovery remain hardware checks.

Omnilingual Windows execution and complete optional Python runtime packaging are
not supplied by these corrections. The new kit builder is a preparation and
readback tool, not a transactional updater. Other legacy export paths are not
qualified by this record. Each of the ten physical drives still needs capacity
testing and complete verified readback when it arrives. Broader voice and
distribution OpenSpec qualification tasks remain open.
