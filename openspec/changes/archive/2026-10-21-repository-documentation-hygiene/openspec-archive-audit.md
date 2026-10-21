# Completed-change archive audit — 2026-10-21

This is a source and recorded-evidence audit, not a claim of installed-platform or model qualification. All nine candidates have all tracked tasks checked, complete OpenSpec artifacts, and passed strict validation on 2026-10-21. Their delta specs contain only ADDED requirements for previously absent main capabilities, so syncing creates new main specs without overwriting existing requirements. Current `go test ./... -count=1`, web API/type/build, desktop (71 tests), and computer companion (73 tests) checks pass. The dated evidence below records the original narrower checks and unrun gates.

| Change | Recorded evidence | Decision |
| --- | --- | --- |
| `desktop-workflow-foundation` | `docs/quality/evidence/desktop-access-2026-10.md`, workflow-evidence foundation section | Sync and archive source implementation; real native/model workflow qualification remains unverified. |
| `task-wide-desktop-access` | `docs/quality/evidence/desktop-access-2026-10.md`, task-wide sections | Sync and archive bounded authority implementation; platform-specific native qualification remains separate. |
| `task-access-intent` | `docs/quality/evidence/agents-and-launch-2026-10.md` and desktop-access evidence | Sync and archive; completed policy fixtures are not a real-app claim. |
| `agent-capability-composition` | `docs/quality/evidence/desktop-access-2026-10.md`, composed task capabilities | Sync and archive; external accounts and installed applications were not exercised. |
| `retire-legacy-agent-workspace` | `docs/quality/evidence/desktop-access-2026-10.md`, retired Agents presentation | Sync and archive; installed package and other OS checks remain unverified. |
| `agent-access-recovery` | `docs/quality/evidence/desktop-access-2026-10.md`, proposal recovery | Sync and archive bounded repair; Chrome launch and real-model behavior were not proven by its fixture. |
| `desktop-service-compatibility` | `docs/quality/evidence/desktop-access-2026-10.md` and desktop-artifacts evidence | Sync and archive; release-package qualification remains separate. |
| `workspace-shell-foundation` | `docs/quality/evidence/workspace-2026-10.md`, redesign stage 2 | Sync and archive; real installed desktop/OS permission checks remain unverified. |
| `workspace-state-recovery` | `docs/quality/evidence/workspace-2026-10.md`, redesign stage 1 | Sync and archive; real microphone/model/hardware checks remain unverified. |

The unfinished `inference-provider-runtimes`, `native-document-workflows`, `model-adaptive-execution`, `workspace-outcome-experience`, `agent-conversation-workflows`, `voice-model-foundation`, and this repository change stay active. No archive here authorizes deployment or publication.

All nine decisions above were applied on 2026-10-21: each new capability spec was synced into `openspec/specs/`, strict main-spec validation passed after each sync, and each change was moved to `openspec/changes/archive/2026-10-21-<name>/`. The post-move documentation checker passed (235 maintained Markdown files, zero missing links). The seven unfinished changes remain in `openspec/changes/`.
