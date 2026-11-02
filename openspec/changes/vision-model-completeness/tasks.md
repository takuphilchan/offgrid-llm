# Tasks

- [x] 1. Add immutable GGUF companion resolution with same-family alternative selection, aggregate sizes and hashes; test Nemotron-shaped generic filenames, unrelated/mixed families, missing files, split weights, auxiliary files and changed source revisions.
- [x] 2. Integrate durable complete-model acquisition and explicit repair in the shared service; test cancellation, resume/restart, corrupt/missing companions, existing model preservation, source collisions and completion after verification only.
- [x] 3. Add model-specific companion receipts and registry/readiness projection; test changed files, malformed receipts, traversal, generic-name collisions, no projector-as-chat-model and unchanged legacy text models.
- [ ] 4. Integrate complete-download/repair progress with web, desktop and CLI, keep image drafts, and add actionable readiness guidance; pass API/UI/client regression tests and document runtime limits.
- [ ] 5. Run full relevant CI and an isolated real installed Nemotron image check with the matching runtime; record source/runtime identities, actual result and unavailable platform checks before release or installation claims.
