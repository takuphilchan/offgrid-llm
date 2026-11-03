# Native desktop speech setup, November 3

## Failure and correction

The Windows desktop installation lacked Whisper/Piper executables. The existing
Whisper tiny.en weights were present; the Piper package folder contained no voice
revision. Qwen package folders were also empty. Model installation therefore did
not establish runtime readiness, and the former Automatic selector did not make
the missing setup clear.

Desktop packaging now includes one pinned Whisper engine and one pinned Piper
engine, with phonemizer data, dependencies, licenses and a file-digest manifest.
The service discovers them relative to its executable. Model packages remain data;
no runtime code is loaded from them. Multiple compatible models share these
family engines. Explicit executable/profile choices and legacy paths remain
supported. Unknown or unsupported architectures are not silently routed to them.

Voice setup separates absent models from unavailable runtimes and links to the
appropriate installed/discovery category. The user's existing recognition model
was retained. With explicit approval, the Lessac English voice was acquired using
the pinned catalog resolution and verified package publication flow in isolated
state. Its revision is `c10ece1aade47bb51c153c893d14e5bf8e5b7117`.

## Evidence

- `go test ./internal/audio ./internal/server -count=1`: passed.
- `npm --prefix apps/desktop test`: 123 passed, one platform fixture skipped.
- Web API contract check, TypeScript and production build: passed.
- `playwright test tests/voice-playback.spec.ts --workers=1`: 22 passed,
  including missing setup, explicit selection, cancellation and playback cases.
- `playwright test tests/voice-native.spec.ts --workers=1`: passed against the
  real isolated service and again against the packaged service and UI. A known
  Whisper JFK recording replaces microphone capture; browser audio decoding,
  transcription, synthesis and HTML audio playback remain real. Playback Stop
  and capture-track release were checked. No user microphone was recorded.
- The packaged Windows service transcribed the known recording with tiny.en and
  returned valid Piper RIFF/WAV audio. A packaged run took 16.1 seconds for
  recognition and 1.8 seconds for synthesis while packaging/backup were active;
  an earlier isolated run took 4.0/3.9 seconds. These are fixture observations,
  not a performance guarantee or a language-quality benchmark.
- `TestSpeechModelsShareFamilyRuntime` checks different packages use the same
  engine paths, change only model data paths and release the previous lease.
- Desktop preparation and copied-payload verification passed. Native engine help
  checks use a system-only PATH, without developer-installed libraries.
- Source-size, repository hygiene, documentation links, diff checks and strict
  `voice-model-foundation` validation passed during implementation.

Local raw logs and fixtures are in `build/speech-source/` and
`build/native-speech-20261103/`; they are not release artifacts.

## Limits and release gates

This qualifies the tested Windows Whisper tiny.en and Piper Lessac path only.
Other models require their own compatibility and quality checks. Qwen needs its
Python runtime; Kokoro/Zipformer acquisition is not executable adapter support.
There is no promise that arbitrary Hugging Face weights work in Whisper/Piper.
Mac/Linux native runtime execution, physical microphone/device behavior, languages
other than this English fixture, and GPU speech acceleration were not qualified
here. Existing broad voice-plan gates remain open. Third-party distribution
notices and corresponding-source availability need release review; pinned
archive hashes are download pins, not upstream signatures.

## Installed desktop and microphone diagnosis

The preview installer completed and the desktop reopened with the matching
service/UI. The live audio status reports both recognition and synthesis
available. The installed Lessac artifacts match all four pinned catalog sizes
and hashes; no duplicate voice import was needed. The separately prepared import
helper was not executed against the user's workspace.

Installed verification matched all 1,157 candidate payload files. Existing model
file sizes/timestamps and the GPU runtime hash were unchanged; none of the 449
backed-up workspace files was missing. The reopened app changed presentation,
window state, download state and one conversation. Its ownership lock and live
RAG database files could not be rehashed while held open. These were reported,
not overwritten or counted as verified unchanged. Rollback is retained under
`D:\OffGrid-install-backups\native-speech-20261103-215650`; the detailed local
record is `build/native-speech-20261103/install-result.json`.

The user's subsequent microphone-denied screenshot was investigated separately.
Read-only Windows privacy inspection showed machine-wide microphone access
`Deny`, with per-user microphone and `NonPackaged` desktop access both `Allow`.
The device-level denial prevents capture regardless of speech runtime readiness.
No registry/privacy setting was changed, and no user microphone was recorded.
The user must enable **Settings → Privacy & security → Microphone → Microphone
access** before testing physical capture. The isolated known-recording test does
not establish that this Windows permission or an actual microphone works.
