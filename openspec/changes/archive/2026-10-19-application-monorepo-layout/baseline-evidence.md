# Phase 2 baseline evidence (2026-10-19)

Source worktree: `D:\offgrid-llm`, with existing uncommitted feature work preserved. No installed app or service was touched. `apps/web`, `apps/desktop`, and `apps/computer` were absent before the move.

## Public identity and contracts

| Item | Before move |
| --- | --- |
| Web npm package | `@offgrid/ui`, `0.4.18` |
| Desktop npm package | `offgrid-llm-desktop`, `0.4.18` |
| Desktop ID / app ID | `com.offgrid.llm.desktop` |
| Product name / protocol | `OffGrid LLM Desktop` / `offgrid` |
| Companion npm package | `@offgrid/computer-preview`, `0.1.0` |
| Desktop packaged destinations | `computer`, `computer/node_modules`, `ui`, and platform-specific `bin/offgrid*` |
| OpenAPI SHA-256 | `7251A1E6CCF0B6CED676499F2533BDB82100FED3FA1830D65862F97CFBB43E67` |
| Web package manifest SHA-256 | `9B13AE8FA56C2860E7F7DCDF2B62809182911D35D4B3253460AE30FC18424CCF` |
| Desktop package manifest SHA-256 | `1A49ACA44DE780FAEFA9CF1A973D8B35CC52AFB9E930198E9CEBC103D8E1B5E8` |
| Companion package manifest SHA-256 | `84C3ED765B4F75435080B23863F3E4850725F0C2DE06117EABDB99BAD366A9D3` |
| `go run ./cmd/offgrid --help` | exit `0`, normalized PowerShell output SHA-256 `32F057BACB778E17BCC1ACA617CFC87F4329B467A0CB6AE0D7BE1114D53C789F` |
| `go run ./cmd/offgrid version` | exit `0`, `0.4.18`, `windows/amd64`, CPU only on this host |

The web, desktop, and companion package manifests are independent; no npm workspace is configured. `pkg/api/openapi.yaml` owns the API contract. The baseline reflects current dirty content, not merely committed HEAD.

## Source-tree preservation inventory

This inventory uses `git ls-files` for tracked paths and `git ls-files --others --exclude-standard` for untracked source paths; ignored build outputs and dependencies are not contract source. Each manifest digest is SHA-256 over sorted `path<TAB>file-SHA256` lines joined with LF. The post-move comparison maps only the source prefix to the destination prefix.

| Source | Tracked | Untracked | Total | Manifest SHA-256 |
| --- | ---: | ---: | ---: | --- |
| `web/app` | 193 | 1 | 194 | `4B58C2858ED5B12CB83B50D1054F2CC70B1DD63CAFD315D4B073CE9ADEC20D67` |
| `desktop` | 40 | 2 | 42 | `279B96C7842D34C992020CC6FFE8ACEACB46DB000E322631E35DD24B86B81EE2` |
| `computer` | 38 | 0 | 38 | `91D8774098937BF72CB2907CA0C0FC36C85B42CEE6B684D20E168C7F89C04773` |

The untracked source files are `web/app/src/features/models/LanguagePackageModels.tsx`, `desktop/attachment.js`, and `desktop/test/attachment.test.js`. All are user work to retain. The source trees also contain ignored local build/dependency files; they may move with their directory, but are excluded from the source hash comparison and must not be committed as part of this refactor.

`git mv desktop apps/desktop` was attempted after preflight but Windows returned `Permission denied`. Read-only process inspection identified a running unpacked OffGrid executable at `desktop/dist/win-unpacked/OffGrid LLM Desktop.exe`. This ignored build output is excluded from the source inventory. The tracked desktop source will be moved per-file with `git mv`; the running build will not be stopped or moved.

After the moves, all three content-manifest digests match the before values byte-for-byte: `apps/web` 193 tracked + 1 untracked, `apps/desktop` 40 tracked + 2 untracked, and `apps/computer` 38 tracked. No tracked source remains at the old paths. The old `desktop` tree retains only ignored `dist` runtime output and empty directories; its ignored `node_modules` was moved to `apps/desktop/node_modules` without deletion. The running unpacked desktop build remains at its original path.
