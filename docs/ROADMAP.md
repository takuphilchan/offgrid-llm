# OffGrid roadmap

This is the single maintained product roadmap. It describes intended work, not shipped capability or a release promise. Current behavior is in [user guides](README.md), requirements in [main specifications](../openspec/specs/), and test/qualification evidence in [quality](quality/README.md). The [January 2026 roadmap snapshot](quality/evidence/roadmap-snapshot-2026-01.md) is retained for history; its checkboxes and target metrics are not current guarantees.

## Delivery order

1. **Correctness and recovery — in progress.** Keep one authoritative service/workspace, verify model and document effects, preserve task receipts, and make failure/unknown states recoverable without replaying uncertain actions. Real embedding and staged recovery slices exist; complete runtime qualification remains open.
2. **Useful governed workflows — in progress.** Finish native document identity, Unicode edit/read-back, browser research, scoped files and cross-application verification. Keep consent, local Stop and account boundaries intact. Windows, macOS and Linux native providers require separate real-hardware evidence.
3. **Model-adaptive local inference — in progress.** Keep llama.cpp compatibility while qualifying shared scheduling, MLX and Transformers text providers on the actual supported hardware. Vision/audio and tool-calling readiness are separate model/runtime checks; unavailable hardware does not become a passing test.
4. **Durable collaboration and evidence — in progress.** Make single-agent work the default and use bounded specialist jobs where context, independent work or model capability justify them. Complete researcher/writer handoffs, source-backed answers, verified artifacts and user steering without giving children unreviewed authority.
5. **Coherent workspace and research — planned.** Finish shared project/conversation/job data, explicit collection membership, versioned citations and datasets, frozen comparisons and human review. Preserve existing saved work through migrations; do not advertise pending v2 APIs as implemented.
6. **Qualification and distribution — ongoing.** Test supported CPU/GPU/platform profiles on isolated installations, measure latency and wrong actions, verify backups and installers, and obtain independent review before general availability claims. Source tests, mocks and a successful build are necessary but insufficient.

## Boundaries

No automatic model downloads, cloud fallback, arbitrary shell execution, privilege changes, credential/payment entry, or silent desktop/profile expansion are implied by this roadmap. Optional external connections require their own scope. Training, unrestricted computer use, distributed inference and expanded P2P remain outside a general production guarantee until explicitly implemented and qualified.

The [production-readiness gates](quality/production-readiness.md) own release decisions. The [active changes](../openspec/changes/) describe narrower implementation work; checked tasks do not certify a platform/model or authorize deployment. Publication, commits and installed-app replacement require separate decisions.
