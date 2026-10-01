# Development instructions

Read README.md, the affected API/desktop documentation, and .github/workflows/ci.yml.
Respect installed-user data and local IPC/authentication boundaries. Use isolated
test state; native control tests need explicit side-effect scope.

## OpenSpec

- For substantive features, behavior changes, and cross-cutting refactors, use
  this repository's OpenSpec workflow. Start with `openspec list --json` and read
  `openspec/config.yaml`; continue an existing relevant change.
- Agree on scope and acceptance scenarios before implementation. Do not silently
  expand an approved change or treat a question as permission to implement.
- Tiny fixes and documentation corrections can use a focused patch and checks
  without manufacturing a full proposal.
- Keep existing roadmaps and user docs authoritative for their purposes. OpenSpec
  records change intent and acceptance requirements, not duplicate user guides.
- Generated specs, checked tasks, and archives are not proof of tests, independent
  review, model accuracy, or production readiness.
- Read [the workflow guide](openspec/README.md) for skills, environments, checks,
  and setup on another machine. Preserve unrelated user work.

## Verification

Select checks for the affected layer, then run the applicable existing CI suite
before claiming merge readiness. Commands are in the workflow guide. Report actual
results and unrun checks. OpenSpec validation checks document structure; it does
not execute application tests.
