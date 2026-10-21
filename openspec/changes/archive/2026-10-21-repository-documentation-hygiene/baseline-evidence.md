# Phase 8 baseline — 2026-10-21

- `docs/advanced/product-reliability-plan.md`: 434,888 bytes at inventory time. Its top-level headings mix dated 2026-10 desktop, voice, UI, model and agent follow-ups with staged quality rules and September evidence; this is not one current readiness claim. The complete heading list remains in the source until a lossless split is verified.
- Maintained old-path references were found in 15 searched Markdown/script/workflow files, including root and docs indexes, contribution guidance and checkers. Historical `docs/releases` files are excluded from rewrites.
- `node scripts/maintenance/check-docs.mjs`: 203 maintained Markdown files, 0 problems. This validates local paths, not the currency of claims or external URLs.
- `openspec list --json`: 16 active changes, 9 with all tracked tasks checked, 7 archive directories. A checked task count is not evidence of model/platform qualification; each archive candidate needs individual review.
- `openspec list --specs --json`: `development-workflows` (5 requirements), `installer-entrypoint-compatibility` (3), and `repository-hygiene` (4). The main spec tree is already nonempty; Phase 8 should extend it, not recreate it.
- Current handwritten Go files over 600 measured lines number 22 under `cmd/` and `internal/`; the largest include `internal/inference/model_cache.go` (1176), `internal/audio/download.go` (996), `internal/agents/durable.go` (988), `internal/models/huggingface.go` (954), and `internal/agents/agent.go` (933). These are candidates for reviewed exceptions, not automatic split targets. The source-size checker will use its own exact newline count for its baseline.
- `node scripts/maintenance/check-application-layout.mjs` currently exits 1 on `ENOENT` for an indexed but removed `cmd/offgrid/model_package_commands.go`. The new source path must still be scanned; the missing old path must not crash the checker.

All observations are source/fixture evidence. No native app, user workspace, deployment or release was exercised.
