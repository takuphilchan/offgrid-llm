# Design

## Context

See proposal.md for the observed failure. Access proposals are intercepted during authorization, before the runner persists an executing call. The current repair branch only accepts native invalid/stale errors with an attached computer session. Task checkpoints are private JSON snapshots in the existing persistence adapters.

## Goals / Non-Goals

Repair service-classified validation failures at their proven pre-dispatch boundary. Do not repair arbitrary provider exceptions, hard policy denials, or execution errors. This change does not claim application launch, whole-desktop authority, or real-model qualification.

## Decisions

- Introduce the explicit `computer_invalid_access_request` code with fixed guidance for app versus managed-browser access. Strict decoding also rejects trailing JSON. Keep app/browser wire compatibility; do not guess a replacement URL or grant mode.
- Extract bounded proposal repair into a runner helper. Allow only explicitly named validation codes during authorization; do not let remote tools opt into retries by returning similarly named strings.
- Persist `proposal_repairs` in the private checkpoint. Three repair opportunities are allowed; the next rejected proposal pauses as `tool_proposal_repair_limit`. Successful dispatched work or resolved local access resets the consecutive budget. Ordinary restart/resume preserves an unfinished budget; explicit resume/steering of the limit pause resets it.
- Consume and record rejected and abandoned calls with `effects: none`, clear approvals, and persist before asking the model again. No rejected call enters the executing intent journal.
- Use existing interrupted-task controls for the repair-limit pause. Avoid adding a duplicate UI recovery mechanism or changing the current renderer. Keep old terminal tasks terminal unless a later migration has independent no-effect evidence.

## Risks / Trade-offs

- A poor model may still fail repeatedly -> stop at a durable limit with model/access guidance rather than an unbounded loop.
- Broad error catching could mask a real denial -> explicit code allowlist only at the authorization boundary, plus hard-denial and uncertain-effect regressions.
- Resuming after restart could reset the budget -> persist the count; reset only after proven progress or explicit recovery of the exhausted state.
- A fixed test caller is not model qualification -> label fixture evidence and leave native/model acceptance open for the complete program.

## Migration Plan

No schema rewrite. Missing private checkpoint counters read as zero. Existing snapshots and API projections remain compatible. Do not rewrite installed task history. Deployment is separate from implementation approval.
