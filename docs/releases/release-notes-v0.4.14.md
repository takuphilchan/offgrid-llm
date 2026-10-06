# OffGrid LLM v0.4.14

This patch improves voice-control placement and dismissal in the shared web and
desktop renderer. It does not add new speech runtimes or complete the Voice program.

## Changes

- Keep Voice settings and Speak responses inside the Chat composer footer,
  alongside the microphone and send actions, instead of an unaligned external row.
- Dismiss voice settings by clicking outside or pressing Escape in both Chat and
  Agents. Escape restores keyboard focus to the trigger. Model selections remain
  saved when the panel closes.
- Keep the popover within narrow and RTL layouts, above surrounding content,
  using the existing monochrome theme and light/dark styles.
- Add browser regression coverage for dismissal, focus restoration, selection
  persistence and composer placement on both surfaces.

## Availability and known limitations

The [v0.4.13 voice-preview limitations](release-notes-v0.4.13.md#voice-availability-and-limitations)
still apply. Installing speech weights alone does not install a matching runtime.
Full Talk mode and cross-platform speech qualification remain incomplete.
Qwen ASR automatic language detection works in the recorded local smoke test;
explicitly passing the API language code `en` currently fails in that adapter.
This UI patch does not change that behavior. Computer Tasks remains a preview.

## Upgrade

Back up workspace data and models and stop active work before upgrading. Update
desktop and service together; a desktop installer does not upgrade an external
Docker container. Preserve the existing data/model volumes and installed speech
runtimes. This patch introduces no storage migration.

CPU image: `takuphilchan/offgrid-llm:0.4.14` (AMD64/ARM64).
NVIDIA image: `takuphilchan/offgrid-llm:0.4.14-gpu` (AMD64).
Verify release assets with `checksums-v0.4.14.sha256`.

[Full changelog](https://github.com/takuphilchan/offgrid-llm/compare/v0.4.13...v0.4.14)
