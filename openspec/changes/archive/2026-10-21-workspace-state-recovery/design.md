# Design

## Context

See proposal.md for motivation and the approved redesign sequence. The current
renderer shares code across web and Electron; docs/advanced/workspace-ui.md owns
its architecture. TaskAccess already separates discovery, launch, and consent
in its handler but not its primary label. PackageModels combines reads, mutations,
polling, and errors through one lock. VoiceInputButton uses a working boolean for
both pre-capture setup and transcription. Chat readiness is boolean/null. The
shell claims 100% local processing independently of its connection.

## Goals / Non-Goals

**Goals:** Correct these externally visible contracts with cancellable,
operation-specific state and regression tests. Preserve accepted work and drafts.

**Non-Goals:** No shell/layout redesign yet, no new lifecycle database or API,
no changes to consent or inference admission, and no speech runtime installation.

## Decisions

- Derive the access CTA from existing session, discovery, and launch state.
  Keep independent explicit consent and the stop-generation guard. Empty launch
  selections cannot accidentally fall through to discovery or start.
- Separate model metadata inspection errors from inventory/poll failures and
  mutation failures. Store the exact failed read callback and a localized recovery
  label. Retry callbacks run reads only. A generation/abort guard prevents a
  superseded read from changing busy/error/result state. Inventory polling cannot
  replace search errors. Mutation errors only offer checking saved state.
- Replace recording/working booleans with one capture-phase union. Keep the
  existing owned media/request lifetime, early readiness check, bounded capture,
  draft callback guard, and cancellation cleanup. Use separate cancel labels for
  checking/permission/transcribing and Stop recording only during capture.
- Keep Knowledge readiness as an explicit phase with stale-response protection.
  Show checking next to the control; unavailable states offer Knowledge setup and
  failed checks offer retry. Do not add a page-wide warning to ordinary chat.
- Use truthful service/workspace wording instead of inferring physical location.
  The existing Settings service panel already identifies its endpoint. The shell
  links to it rather than adding a new public identity claim or host detector.
- Add typed locale copy for these feedback states; do not expand English-only
  speech fallback with additional untranslated states. Existing broader voice
  translation work stays visible as later work.

## Risks / Trade-offs

- [Late media consent after cancellation] -> preserve owned-work identity guards
  and test track release with synthetic media.
- [Search retry accidentally repeats an install] -> keep retryable metadata
  callbacks separate from mutation handlers; test request methods/counts.
- [Polling overwrites actionable errors] -> render operation-scoped failures;
  clear only the corresponding successful operation's failure.
- [Safer wording is less specific] -> show the actual service in Settings and
  never substitute an unsupported locality claim.
- [Fixtures hide runtime problems] -> test renderer contracts with isolated
  fixtures, run a real-service UI suite, and explicitly exclude model/hardware
  quality claims.

## Migration Plan

No data migration or new dependencies. Ship shared assets in matching builds.
Reverting this slice changes only renderer behavior/copy; installed-user state
is untouched. Deployment remains a separately approved operation.
