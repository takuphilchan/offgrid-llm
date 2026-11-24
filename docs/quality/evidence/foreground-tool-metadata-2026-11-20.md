# Foreground tool metadata discovery

Date: 2026-11-20. Source: the uncommitted shared worktree in `D:\offgrid-llm`,
continuing `agent-conversation-workflows` 10.1 after the
[default-admission increment](default-conversation-admission-2026-11-20.md).
Tasks 10.1–10.3 and broader qualification remain open.

## Implementation and authority boundary

New canonical foreground preparation pins `ConversationDiscoveryScope` version 1
in `AgentConfig` before clearing executable capabilities. The ceiling contains
capability identities with provider descriptions omitted, without host references,
credentials, approvals, executable schemas or resource contents. Historical runs
without this version retain their previous behavior.

`internal/server/conversation_discovery.go` provides a separate local metadata
callback. It checks canonical ownership, current actor permission, conversation
restrictions, optional profile and current provider identity/enabled policy.
Context metadata checks current memory consent or owned immutable skill references
without loading packages. It does not call `Hosts.Resolve`, which contacts hosts.

The existing `task_find_tools` schema is unchanged. Its versioned foreground path
in `internal/agents/conversation_discovery.go` returns at most eight entries,
descriptions bounded to 256 UTF-8 bytes and entries bounded to 1024 encoded bytes.
Oversized identities are omitted rather than truncated. Results are explicitly
untrusted, metadata-only, not execution-authorized and `work_admission_required`.
Search never populates `Checkpoint.DiscoveredTools`, promotes provider schemas or
starts work. Native qualification and `conversation_decide` guards are unchanged.

`browserRunTools.Authorize` independently rejects provider/host authorization for
canonical foreground turns, including internal callers that skip model-facing
qualification. Conversation controls still execute inside the existing runner.

## Requirement-to-test mapping

| Acceptance | Actual regression |
|---|---|
| Useful metadata, no executable schema | `TestConversationMetadataThroughActualRunner`: actual runner/control store with injected inference; calculator metadata followed by an answer; no provider schema, work or access request |
| Bounded progressive catalog | `TestConversationMetadataDiscoveryBoundsAndCeiling`: 20 entries, eight-entry pages/final page, oversized offset rejection, UTF-8 description/output bounds |
| Live scope and identity | `TestConversationMetadataLiveScopeAndIdentity` / `TestConversationMetadataCurrentActorAndProfile`: disable/replacement, empty restrictions, actor demotion/deletion, narrower profile and foreign actor |
| No provider contact or approval transfer | `TestConversationMetadataDoesNotContactMCPOrTransferApproval`: actual isolated MCP HTTP enrollment, then zero HTTP requests/provider effects during discovery/forged dispatch; live broker denial removes entries |
| No host contact | `TestConversationMetadataDoesNotContactSelectedHost`: service-owned fake execution endpoint records zero Check/Call/Admit calls during discovery/forged reads despite a selected folder |
| Durable metadata without execution | `TestConversationMetadataDiscoveryDurabilityAndForgedDispatch`: actual admission/runner, zero provider calls, retained receipt/restart and empty executable capability/discovery maps |
| Historical/native/text-only boundaries | `TestConversationMetadataDiscoveryDoesNotExpandProtocols` and unchanged `TestConversationDecisionDoesNotExpandQualifiedNativeCatalog` |
| Owned immutable skill metadata | `TestSkillMetadataReferencePreservesOwnedOlderRevision`: older/newer owned references remain available; foreign owner/workspace and changed digest fail; package contents are not loaded |

## Checks and limitations

Windows source checkout with the existing Go 1.26.6 toolchain. Tests use temporary
stores, injected inference, local MCP HTTP and a fake execution endpoint. No
personal hosts, installed services or model weights are accessed.

| Command | Actual outcome |
|---|---|
| `go test ./internal/agents ./internal/server -run '^Test(ConversationMetadata\|SkillMetadataReference)' -count=1` | Passed: agents 1.210s, server 1.370s |
| Earlier focused metadata run including MCP transport | Passed: agents 1.657s, server 2.478s |
| `go test ./internal/agents ./internal/server -run 'Test(Conversation\|Canonical\|Discovery\|Skill\|Profile\|DefaultConversation\|Composed)' -count=1` | Passed: agents 15.763s, server 26.255s |
| `go test ./internal/agents ./internal/server -count=1` | Passed with vet enabled: agents 64.844s, server 115.365s; production source held stable during the run |
| Ubuntu WSL `CGO_ENABLED=1 /usr/local/go/bin/go test -race ./internal/agents ./internal/server -run TestConversationMetadata -count=1` | Passed: agents 1.492s, server 3.536s |
| Ubuntu WSL `CGO_ENABLED=1 GOPROXY=off /usr/local/go/bin/go test -race ./internal/agents -run TestSkillMetadataReference -count=1` | Passed: 1.197s |
| `node scripts/maintenance/check-source-size.mjs` | Passed after final code/test additions |
| `node scripts/maintenance/check-docs.mjs` | Passed: 446 maintained Markdown files |
| `openspec.cmd validate agent-conversation-workflows --strict --no-interactive` | Passed; document structure only |
| `git -c core.safecrlf=false diff --check` | Passed |

One initial new test executor omitted its required context parameter; the fixture
was corrected to the existing `SimpleExecutor` contract. Another attempt was
blocked by a transient import cycle introduced by concurrent provider work:
`inference -> execution -> serviceclient -> rag -> inference`. Its owner corrected
that dependency before the successful runs. Neither failure is counted as a pass.

Independent read-only review by the scheduling agent found no blocking defect in
the sampled pinning, policy/identity intersection, dispatch guards, metadata-only
skill query, bounds and regression coverage. The reviewer did not rerun tests.
The shared tool description still mentions preferred schemas; foreground system
instructions and results explicitly override that behavior. It was preserved to
avoid silently changing native qualification catalogs; model clarity remains a
real-model qualification question.

The full affected normal suites and focused Linux race checks passed. The first
two Linux invocations did not reach Go tests: nested shell regex quoting was
incorrect, then non-login PATH lacked Go. The successful commands used direct
WSL arguments and the already installed `/usr/local/go/bin/go` (Go 1.26.6).
Root's separate Linux race snapshot `20261120093941576` predates
the active server discovery pin and cannot qualify this increment.

Metadata discovery does not load skills, access memory or execute browser/provider
operations in foreground conversation. Those require existing work admission and
qualification. These tests do not establish model judgement, real task success,
hardware performance, long-running stability, installed-platform behavior or human
usability. Complete client cutover and all 10.1–10.3 gates remain tracked separately.
