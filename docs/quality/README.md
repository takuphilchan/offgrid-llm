# Quality and qualification

The [production-readiness gates](production-readiness.md) define release expectations. The dated capability and release records below preserve the prior reliability evidence; the [split manifest](evidence/reliability-split-manifest.json) reconciles their source bytes and hashes. [Benchmarking](../advanced/benchmarking.md) and [performance](../advanced/PERFORMANCE.md) cover measurement methods.

Start with [native document evidence](evidence/native-documents-2026-10.md), [model-adaptive evidence](evidence/model-adaptive-2026-10-16.md), or [release and quality evidence](evidence/quality-stages-and-release-evidence.md). These are dated records, not current certification.

Keep three states distinct: implemented in source, automatically tested, and qualified on a real platform/model. A mock, compilation, Xvfb run, or checked OpenSpec task does not by itself establish installed-user readiness. Unrun checks remain unverified.

The [workspace completion audit](evidence/workspace-completion-audit-2026-11-01.md)
records the latest presentation follow-up, v0.4.23 candidate and downloaded-package
checks, retained failures, and the remaining human/native qualification gates.

The [Talk and terminal entry record](evidence/talk-and-terminal-2026-11-04.md)
separates source and packaged Windows checks from physical microphone and
other-platform qualification.

For opt-in, disposable-folder real-model checks, see the [native-agent workflow fixtures](native-agent-workflows.md) and their [dated results](evidence/native-agent-execution-2026-10-25.md).
