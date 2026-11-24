# Preparing the offline starter kits

The initial media profile is Windows x64, CPU inference and Whisper/Piper speech.
The ten 64 GB drives use the same master. This is distribution media, not a
portable workspace or proof of African-language quality. Do not copy a personal
workspace, tokens, recordings or conversation history into the master.

## Prepared inputs

Prepare one explicit source directory containing:

```text
prepared/
  application/                     complete unpacked Windows desktop build
    OffGrid LLM Desktop.exe
    resources/
      bin/                         service, document workers and CPU inference
        llama-server.exe           plus every required app-local DLL
        runtime-dependencies.json  from Windows runtime preparation
      speech-runtime/              complete verified Whisper/Piper runtime pack
      ...                          retain the rest of the desktop payload
  models/
    manifest.json                  existing PackageManifest models format
    selected-model.gguf
  speech/
    recognition/manifest.json      complete data-only Whisper package
    recognition/...
    voice/manifest.json            complete data-only Piper package
    voice/...
  licenses/                        selected model redistribution notices
  documents/                       optional reviewed demo material
```

The existing desktop packaging verifiers check UI, service identity, document,
speech, terminal, execution and computer payloads. The CPU inference directory
must already have passed `scripts/release/prepare-windows-runtime.ps1`; retain
its dependency manifest and actual DLLs. The builder verifies those digests and
the declared dependency closure. It does not compile/download missing assets or
execute native inference. OCR engine packaging does not include every OCR language.

For `models/manifest.json`, use the existing `models` array of `file_name`, `size`
and `sha256` entries. Every GGUF must be listed. Speech folders retain their
original model-package manifest and every declared artifact, including licenses.
Import validates the full model schema again. Digests identify selected bytes;
they do not independently establish publisher trust or redistribution permission.

Use a desktop build from the matching checkout. Maintainer Node/Go tools and
existing desktop build dependencies are required for its verification; recipients
do not need these tools to run the copied application. Review licenses before
selecting weights. The baseline kit does not bundle Qwen, Kokoro, Omnilingual
or other optional Python runtimes; those need their own complete runtime profile.

## Create and verify a master

```powershell
node scripts/build/create-usb-package.mjs --source D:/kit-prepared --output D:/kit-master-v1 --version 0.4.33
node scripts/build/create-usb-package.mjs --verify D:/kit-master-v1
```

Use the actual prepared app version. The Bash compatibility entry point accepts
the same flags. The old positional bare-binary builder is intentionally rejected:
it omitted essential runtimes and generated overwrite installers.

Output must be a new directory outside the source, with an existing parent.
Default maximum is 40,000,000,000 bytes, including a manifest reserve. Use
`--max-bytes` for a different explicit budget. Required files and available disk
space are checked first; each copied file is rehashed. `kit.json` is written last.
A failed copy can leave a partial directory without a valid completion manifest;
inspect it and choose a fresh output on retry. Existing masters are never merged
or replaced. Re-run `--verify` after every physical copy and retain the master
`kit.json` digest independently to detect manifest replacement as well as damage.

The included instructions copy the app into a new SSD folder and use the existing
Models import controls for weights. App data retains its normal location. Updates
use a fresh application folder and explicit model imports; conflicting installed
filenames are preserved. This is not a one-click installer or a transactional
whole-kit update. CLI `offgrid import` no longer restarts inference or selects a
model. A directory import reports individual failures even if other files succeeded.

## Qualification before distribution

1. On a disposable clean Windows machine with networking disabled, copy the
   exact master and complete a real model response, transcription and playback.
2. Test each advertised language separately for recognition, useful answers and
   intelligible speech with native speakers; include names, numbers, accents,
   code switching, silence and noisy recordings. Model language lists are not results.
3. Measure cold/warm response delays and memory with the complete pipeline running;
   test interruptions, device disconnect and recovery on physical speakers/headsets.
4. After the drives arrive, test full capacity and verify each kit's complete
   readback against the retained master. Do not treat a synthetic fixture as a
   certified language, clean-machine installation or physical drive test.

No current kit manifest claims any of these gates passed. Track evidence in the
[quality index](../quality/README.md), not by marking model downloads as ready.
