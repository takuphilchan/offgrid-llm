# Current interface images

Captured **25 November 2026** for the [README](../../README.md) and
[workspace tour](../guides/interface-tour.md). These images show the v0.4.33 application source
with isolated sample data.

## Provenance

| Capture | Source and scope |
| --- | --- |
| Chat, Models, Library, Agents, Settings | Real React renderer in headless Chromium at 1440 × 900, English, default OffGrid light/dark palettes; isolated API fixtures |
| Opening light Chat image | Same renderer with a staged project task, agent profile, completed status and sample file/command receipts; no actual agent execution |
| Quick access input and reply | Real compact renderer at 720 × 164 and 720 × 540 with a simulated desktop bridge and saved sample conversation; no global shortcut registration or native app actions |
| Document reader | Same renderer with an authored sample workshop page served through the reader's raster API; demonstrates layout, not PDF extraction |
| Terminal and command context | Hidden packaged Electron window with disposable profile and local HTTP fixtures; actual PowerShell and bundled `offgrid --help` output |
| Terminal welcome | Same hidden packaged terminal, before starting a session; theme transition settled before capture |
| Voice listening, light and dark | Real full workspace React voice view in headless Chromium at 1440 × 900, isolated speech capability and microphone fixtures; no real microphone, transcription or model inference |
| Workspace GIF | Four screenshots, four seconds each; slideshow, not generation or execution footage |

The application source is tag `v0.4.33`, commit
`19fa067a0ddea2447f9f302b4fddf4a693a3efe6`. Only capture tooling and documentation
were modified for this refresh; application code was unchanged. The terminal
uses the locally verified 0.4.33 release candidate with UI identity
`7b47cdcc3b3429ea7b637b4902e0d80201c47ee6d5bc515b52c213e2fddf8394`.
The automatic Talk fixture uses a synthetic audio stream and simulated device
settings; it does not test a physical microphone or echo cancellation.

Conversations, model inventory, document entries, and agent profiles are synthetic
demonstration data. No model inference, real document ingestion, tool workflow,
or speech qualification is represented by these captures. The reader page says
“Demonstration document.” No installed-user workspace or credentials were used.

The terminal images capture the terminal surface itself, without the surrounding
desktop sidebar. A temporary PowerShell drive, `Demo:`, points to the disposable
capture folder. Its output is real, not substituted text. Other screenshots are
full renderer viewports. No controls, colors, or status labels were retouched.
Renderer screenshot animations were disabled for a stable frame.

[The capture manifest](captures-2026-11-25.json) records asset sizes and SHA-256
hashes. Eighteen superseded assets were copied and hash-verified in
[the historical archive](old-images/README.md) before replacement.

## Refresh the renderer captures

Use the repository's existing web development dependencies and Playwright browser.
From the repository root in PowerShell:

```powershell
$env:OFFGRID_DOC_CAPTURES = '1'
$env:OFFGRID_FIXTURE_PORT = '5196'
npm --prefix apps/web run test:e2e -- documentation-captures.spec.ts documentation-agent-capture.spec.ts documentation-quick-captures.spec.ts --workers=1 --max-failures=1
```

The opt-in [capture fixture](../../apps/web/tests/documentation-captures.spec.ts)
writes images to `build/documentation-captures`. It serves sample API responses;
do not replace its endpoint with a personal or production workspace. The chosen
port must be free. Current captures include light/dark Chat, installed/catalog
models, Library, an agent profile, the reader, Appearance, and response settings.
The [opening Chat fixture](../../apps/web/tests/documentation-agent-capture.spec.ts)
shows the work status and result of a staged project task, with matching saved
conversation and job identities. Its tool receipts are sample data, not a record
of commands executed during capture.
The [Quick access fixture](../../apps/web/tests/documentation-quick-captures.spec.ts)
captures both the input and expanded reply, with a simulated desktop bridge.
Its viewport sizes follow the compact layouts; native shortcut behavior is
covered separately by desktop tests.

## Refresh the native terminal captures

Use a locally packaged Windows preview with `candidate.json` or `preview.json` beside its
`win-unpacked` folder. From the repository root:

```powershell
node scripts/maintenance/capture-desktop-docs.mjs 'build/YOUR_PREVIEW_FOLDER/win-unpacked/OffGrid LLM Desktop.exe'
```

The [capture script](../../scripts/maintenance/capture-desktop-docs.mjs) launches
only its own hidden Electron instance, uses a disposable profile and sample
service, and runs read-only CLI help. It does not automate the visible desktop,
invoke inference, or open the installed-user workspace. It records the packaged
version and UI identity in `build/documentation-captures/desktop-provenance.json`.
That local record contains a temporary path; do not publish it as-is.

## Refresh the voice listening captures

The opt-in full workspace voice capture retains documentation images. It
uses simulated microphone permission and audio, without opening the real device.
From the repository root in PowerShell:

```powershell
$env:OFFGRID_DOC_CAPTURES = '1'
$env:OFFGRID_FIXTURE_PORT = '5197'
npm --prefix apps/web run test:e2e -- talk-continuous-browser.spec.ts -g 'capture documentation automatic Talk' --workers=1
```

The output is `build/documentation-captures/voice-listening-light.png` and its
dark variant. The static frame has animations disabled. The capture checks
listening, replacing the main view, and returning to Chat. Separate voice tests
cover motion and reduced-motion behavior. These images
demonstrate automatic listening with a centered Stop Talk control, not
automatic turn-detection accuracy.

## Build the optional slideshow

The existing optional tools are `gifenc@1.0.3` and `pngjs@7.0.0`, kept under
`build/documentation-tools`, outside application dependencies. If not already
present, install them explicitly before running the encoder:

```sh
npm install --prefix build/documentation-tools --no-save --ignore-scripts gifenc@1.0.3 pngjs@7.0.0
node scripts/maintenance/build-documentation-gif.cjs
```

The encoder requires the four 1440 × 900 source images and keeps the GIF below
2 MiB. Inspect every static capture and GIF frame. Preserve the existing image
set in a new dated archive, copy only reviewed assets into this directory, update
the manifest and provenance, and run the documentation checks:

```sh
node --test scripts/maintenance/check-docs.test.mjs
node scripts/maintenance/check-docs.mjs
```

The capture checks and visual review verify presentation, not release readiness
or the quality of a model's answers.

## Verification of this refresh

On 25 November 2026, all five workspace/reader/quick-window capture scenarios
and both automatic Talk capture scenarios passed. The hidden packaged v0.4.33
terminal capture passed its visibility and read-only service checks. Visual
review caught an outdated endpoint fixture; it was corrected to the documented
empty endpoint response and both workspace captures passed again. All current
assets and all eighteen archived assets have SHA-256 manifests. The four GIF
frames use the reviewed Chat, Models, Library and agent-profile captures.

TypeScript checking, the seven documentation-checker tests, the maintained-doc
scan, application/developer layout checks and source-size checks passed. These
fixture checks do not establish inference or physical microphone quality.
