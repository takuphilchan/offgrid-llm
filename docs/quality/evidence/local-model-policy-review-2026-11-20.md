# Local model policy independent review

Recorded 2026-11-20 against the shared uncommitted implementation on base
`b19c658736d5f5f000710c6c608644564cc88cd4`. A separate agent reviewed the new
profile model policy, configuration detachment, admission and evidence selection
code plus their conversation/delegation call sites. This is a bounded code review,
not an audit of the entire agent executor or real-model qualification.

## Findings reproduced and corrected

1. **Response-context readiness could reject a qualified fallback.** With an
   8,192-token default and 4,096-token interactive context, a fallback qualified
   at 4,096 passed direct policy resolution but job admission rejected it with
   HTTP 422 `local_model_unqualified`. Profile readiness had checked the default
   context before response-profile planning. Admission now uses
   `effectiveProfileAtWindow` with the planned response profile. Ordinary profile
   inspection retains its default-context meaning.
2. **Session admission could persist stale context evidence and drop the adapter.**
   When only 8,192-token evidence existed, an interactive session originally
   persisted a 4,096-token intent with an 8,192-token selected fallback candidate
   and a nil tool adapter. `conversation_jobs.go` had changed the context and
   independently replaced the adapter after `resolveJobConfig` pinned policy.
   The session now normalizes the default response profile to interactive before
   resolution and preserves the resolved context/adapter afterward. Missing
   final-context evidence rejects admission before the durable intent.

Both defects were reproduced with independent tests through a Go overlay backed
by an ignored `.go.txt` file, leaving the implementation untouched during review.
The session reproduction stopped at the `intent_saved` fault hook; no model or
effect executed. The two correct-behavior assertions failed before the fix.
The unchanged assertions passed after the fix:

```powershell
go test '-overlay=build/review-model-policy-overlay.json' ./internal/server -run '^TestReview(Qualified|Session)' -count=1 -v
```

Result: two passed, package 3.346 seconds. The overlay is an ephemeral review
artifact, not a maintained prerequisite. The maintained regression is
[`profile_model_policy_conversation_test.go`](../../../internal/server/profile_model_policy_conversation_test.go),
which exercises actual session HTTP admission and saved intents for default
interactive, explicit interactive, extended and missing-context cases. The
implementation owner records its full-suite and race results in
[local model policy admission](local-model-policy-2026-11-20.md).

## Review limits and retained requirements

No further blocking defect was found in the sampled new policy/config snapshot
code after the two corrections. Omitted policy preserves legacy selection;
endpoint routes cannot enter the configured local pool; qualification is
service-owned; perception requires measured projector/preprocessing identity;
and the admitted copy preserves explicit empty resource ceilings. These are
source/fixture conclusions, not a real model pass.

Delegated child policy pins are not yet propagated through child configuration
and dispatch. Automatic switching, parent/child/perception integration, actual
perception dispatch, generalized provider/constrained pool qualification and
hardware evidence remain open under tasks 4.2–4.4 and the broader roadmap. The
current evidence store treats the latest per-model/check record as current;
this review does not establish concurrent qualification at multiple contexts.
No installed state, personal account or model weights were used.
