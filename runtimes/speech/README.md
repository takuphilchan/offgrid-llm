# Native speech packaging

Desktop `beforePack` prepares the platform runtime through
`apps/desktop/speech-pack.cjs`; `afterPack` verifies the copied payload. Build
machines need CMake, a native C++ compiler, curl and tar. Installed users do not.
Whisper is built from the pinned source archive with a portable CPU configuration,
static libraries and no GPU requirement. Piper uses the complete pinned upstream
archive, including its phonemizer, ONNX runtime and language data. No model weights
are bundled or downloaded by this build step.

`engine.lock.json` records the upstream Whisper commit and the SHA-256 values of
the official Piper release archives retrieved on 2026-11-03. The old upstream
release provides no GitHub asset digest; these are reviewed download pins, not
claims of an upstream signature. Preparation checks the archive bytes before
extraction, then records every payload file. The copied payload must match that
manifest, including notices and language data. No executable is taken from a
downloaded model package. Prior staging directories remain recoverable in `build/`.

The service discovers `speech-runtime/{whisper,piper}` beside its `bin` directory.
Explicit configured executables and legacy user-installed paths remain supported.
Only this native Whisper/Piper payload is included: Qwen requires its own reviewed
Python runtime; acquiring Kokoro/Zipformer weights does not install an adapter.

Runtimes are shared by compatible model families, not copied per model. The
service's `PackageRuntime` selects a validated model package, holds its lease,
and passes its weight/configuration paths to the same installed family engine.
Switching Piper voices or Whisper models changes those data paths, not the
runtime installation. Packaging contains one Whisper engine and one Piper engine
regardless of how many compatible models are installed. The UI uses the same
service path as CLI/API clients. Model-specific smoke evidence stays separate:
one working voice does not qualify every voice or a different architecture.

Run `node --test apps/desktop/test/speech-pack.test.js` and
`go test ./internal/audio ./internal/server`. Real browser qualification is opt-in
through `apps/web/tests/voice-native.spec.ts`, using a disposable service and
`OFFGRID_E2E_SPEECH_FIXTURE` pointing to a known recording. A successful `--help`
probe checks loading only; it is not speech quality qualification.

See `NOTICE.txt` and `licenses/` for upstream license texts and source locations.
Platform release qualification, distribution notices and corresponding-source
availability must be reviewed before publishing changed third-party binaries.
