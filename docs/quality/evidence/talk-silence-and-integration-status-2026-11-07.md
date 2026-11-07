# Talk silence recovery and external-agent status

## Corrections

The reported Talk screenshot shows an empty recognition result, not evidence of
a network disconnect. Talk previously sent that result through its failure path
and required Speak again. Empty recordings and successful empty transcripts now
schedule a fresh capture after 750 ms. They create no chat turn and never replay
the recording. Ordinary dictation retains its no-speech explanation.

Stop and component teardown clear the timer. Session revision and ownership
checks prevent a delayed restart after navigation, typing or stopping. Real
capture, transcription, transport and chat failures still pause for review;
this change does not automatically retry requests with uncertain outcomes.

Managed Hermes/OpenClaw status previously displayed positive requirement prose
as errors when the external runtime was absent. It now reports runtime absence
explicitly and marks dependent provider/configuration checks not checked. It
does not confuse a Windows installation with an installation inside WSL.
The CLI uses the shared presenter, identifies the environment, wraps diagnostics,
and separates blockers from setup guidance. Human-readable preflight errors no
longer repeat the complete report. JSON retains structured checks and detailed
errors. Service/model blockers take precedence over install guidance.

Context requirements are unchanged. Formatting a status report cannot make a
32,768-token allocation meet Hermes's 64,000-token requirement. No runtime,
provider plugin, model selection, context setting or installed user data was
changed by these patches.

## Executed checks

- `go test ./internal/integrations/... ./cmd/offgrid` passed, including absent
  runtimes, dependent checks, blocked launch and actionable presentation.
- `npm.cmd run test:e2e -- tests/voice-playback.spec.ts --workers=1` passed
  **43/43** in 2.6 minutes. Includes repeated silence, Stop during re-arm,
  navigation during re-arm, actual transcription transport rejection, three
  successive spoken replies, draft preservation, echo exclusion and cancellation.
- `npm.cmd run build` passed TypeScript and Vite production build. Existing
  bundle-size and plugin-timing warnings remain.
- `openspec validate voice-model-foundation --strict` passed. This checks
  specification structure, not runtime behavior.
- `git diff --check` passed.

## Limits

Speech browser checks use isolated synthetic capture and ASR/TTS responses. No
physical microphone was recorded and no real Hermes/OpenClaw task was run in this
pass. Full release CI, packaged desktop verification and installation of these
patches have not been performed. These results do not replace the existing
hardware, model-quality or enterprise-readiness qualification requirements.
