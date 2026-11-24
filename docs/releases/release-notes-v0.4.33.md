# OffGrid v0.4.33

Unsigned preview — qualification scope and known limitations are listed below.

This candidate continues the **unsigned preview** channel. Windows publisher
signing and macOS Developer ID signing/notarization are not established by local
builds. Published artifacts must include their trust reports and SHA-256 manifest.

## Changes

- Adds a supervised Kokoro speech adapter with explicit runtime compatibility
  checks, and fixes Qwen recognition language normalization and automatic
  CustomVoice language handling. Actual Kokoro synthesis remains unqualified.
- Adds verified offline-kit preparation and readback. Model imports preserve
  existing files, verify manifest digests before publication and report partial
  failures without restarting inference or selecting a model.
- Extends browser workspaces with multiple managed tabs, explicitly scoped
  navigation and stronger file-transfer ownership, provenance and recovery.
- Keeps spoken corrections and side questions in the existing conversation and
  active work. Talk now supports continuous capture, local playback interruption,
  adaptive audio pause handling and visible completion/discard controls.
- Distinguishes generated speech from queued, started and completed playback,
  and prevents late callbacks from speaking in a superseding conversation.
- Fixes short manual recordings rejected by Whisper, keeps Talk capture alive
  when saving its first conversation, permits interruption during silent speech
  preparation, and removes duplicate/overlapping Talk controls. Recovery offers
  session-only microphone selection; unsupported automatic interruption is visible.
- Corrects applied echo-cancellation detection and quiet/gapped speech onset for
  hands-free interruption. Bounds onset evidence to reject periodic short spikes.
  Automatic Talk removes the recording counter and empty control dock, centers
  its exit action and keeps “Speak again” on a separate line. These corrections
  passed isolated audio-graph, recorded-speech and responsive/localized UI checks;
  they do not establish physical acoustic qualification.
- Strengthens shared storage protection and execution recovery. An original
  surviving supervisor can release its reservation after verified descendant
  cleanup, while uncertain operations remain uncertain and are not replayed.
- Continues provider, resource scheduling, document, agent and product work
  tracked in the transformation roadmap. This candidate does not certify those
  programs as complete.

## Verification and remaining release work

Focused production-browser voice checks and real Whisper/Piper engine checks
pass within the scope recorded in
[voice evidence](../quality/evidence/continuous-voice-2026-11-23.md).
Six frozen Windows and Linux recovery packages pass; Linux used the race
detector. See [recovery evidence](../quality/evidence/host-protection-survivor-2026-11-23.md).
These checks do not replace exact-release-source CI and artifact verification.

The November 24 integrated working-tree check passed the full Windows Go suite,
292 desktop tests (4 opt-in skips) and 238 computer companion tests (6 opt-in
skips). The current local Windows candidate installed successfully and its 73
checked payload files matched. See [voice and UI correction evidence](../quality/evidence/voice-barge-in-and-auto-ui-2026-11-24.md)
for test scope, installed-source receipts and known limits. Release automation
must still qualify and rebuild the immutable tagged source on supported runners.

The 512 MiB browser upload requirement is not qualified: the prepared Chromium
transport patch has not been built or integrated. Stock attached Chrome/Edge
do not acquire that patch. Continuous voice still uses batch recognition;
physical echo/noise rejection, conversational latency, languages and human
naturalness remain unqualified. Durable played-speech context and complete
owner-loss recovery also remain unfinished.

macOS native execution cannot yet prove retirement of detached descendants or
perform the qualified working-copy quarantine/atomic replacement operations.
It retains uncertain receipts, copied artifacts and reservations instead of
claiming cleanup, and restart does not authorize replay. Tests of those refusal
boundaries do not qualify successful macOS cleanup or close the roadmap gates.

Before publication, qualify the exact candidate source and produced packages,
resolve release blockers and record the actual results. No all-platform,
all-model, complete-transformation or signed-release claim is made here.
