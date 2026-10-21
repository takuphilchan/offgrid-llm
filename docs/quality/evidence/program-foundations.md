# Product reliability and usefulness plan

This is the implementation plan following the September 2026 system review.
It is a delivery checklist, not a claim that every capability is production-ready.

The [approved production-readiness delivery contract](../production-readiness.md)
defines the next five milestones and release gates. The historical stages below
record earlier work; their checked boxes do **not** imply that the newer milestones
or cross-edition qualification have passed.

## Product outcome

Make OffGrid a dependable private workspace for conversations, document-grounded
answers, and governed tasks. Browser, desktop, CLI, and external agents use the
same Go runtime and behavioral contracts. Keep Go, React/TypeScript, Electron,
and the native OffGrid provider adapters. Do not introduce microservices or a
framework migration to solve local state and lifecycle problems.

## Delivery rules

- Ship small vertical slices with service, API, UI/CLI, documentation, and tests.
- Preserve existing models and data. Never silently claim legacy data for the
  first signed-in user, or replace failed persistent storage with volatile data.
- Authorization belongs at the service boundary as well as the HTTP boundary.
- An unavailable dependency must produce an explicit capability state.
- A successful HTTP response or CI build is not proof of a useful model task.
- Never replay side effects automatically after a crash or approval pause.
- Keep experimental capabilities visibly separate from supported workflows.
- No automatic publishing: validate locally before a separately approved release.
