# Design

## Context

See [proposal.md](proposal.md), the current [documentation index](../../../../docs/README.md), [repository map](../../../../docs/repository-structure.md), and [production-readiness owner](../../../../docs/quality/production-readiness.md). The source-layout and repository-hygiene checkers already exist. `docs/advanced/product-reliability-plan.md` is a long, mixed chronology; it is evidence, not a current capability guarantee. The main OpenSpec spec tree is no longer empty, but multiple active changes remain incomplete and must not be archived for neatness alone.

## Goals / Non-Goals

**Goals:** current documentation has stable, discoverable owners; evidence can be found by capability or release; path and size checks catch new regressions without punishing unchanged legacy files; completed changes can be archived with honest evidence.

**Non-Goals:** revise product behavior or claims, publish a release, erase or rewrite historical evidence, convert every archived plan into a current spec, or use a line count as a substitute for responsibility review.

## Decisions

1. **Four maintained destinations, compatibility at old paths.** Add `docs/architecture/`, `docs/operations/`, `docs/contributing/`, and `docs/quality/` indexes. Move current material only after link inventory; an old maintained path becomes a short pointer with a clear canonical destination. Existing `docs/advanced/` files with no safe move remain maintained in place and are listed from the new index. Alternative: bulk rename every page; rejected because external links and active plans would break at once.
2. **Evidence is extracted, not rewritten.** Split the reliability record at major capability/release headings into verbatim or mechanically adjusted source sections, with a table of contents and provenance links. Keep release notes untouched. The original path becomes the quality index/compatibility entry. Any text that cannot be classified stays in a dated legacy evidence part rather than being dropped or paraphrased. Alternative: summarize and delete the log; rejected because results and limitations would be lost.
3. **One current roadmap.** `docs/ROADMAP.md` remains the roadmap; production-readiness and quality pages link to it and identify qualification gates. Remove stale target dates only from maintained guidance, not from historical evidence. Alternative: make each OpenSpec change a roadmap; rejected because active changes are narrower and provisional.
4. **Size ratchet with reviewed exceptions.** A script checks existing tracked and untracked handwritten production files. `cmd/*/main.go` has a hard 150-line limit. Files over 600 lines require an allowlist entry with path, owner, responsibility, review date, and baseline line count. The checker rejects unreviewed new oversized files and growth above an exception baseline. Exclude generated schemas, lockfiles, catalogs, release notes and evidence. Alternative: auto-split every large file; rejected because cohesive logic can become harder to review.
5. **Path checkers use readable current sources.** `git ls-files --cached --others --exclude-standard` remains the inventory, but deleted working-tree entries are skipped during content reads; staged new paths remain checked. A test supplies a staged-move fixture. Alternative: inspect index blobs instead; rejected because this phase validates the working copy contributors will build.
6. **Archive individually and evidence-first.** Review each complete active change against its tasks, validation, main-spec sync state and local evidence. Archive only when no blocking finding remains; incomplete or unqualified changes stay active. No archive operation implies deployment or release qualification.

## Risks / Trade-offs

- [Existing links break after a move] → keep old-path stubs until the link checker and repository references pass.
- [Evidence loses context when split] → preserve section text and original headings, record section mapping, and compare source/output text counts or hashes before replacing the long file.
- [Size checker produces noisy legacy failures] → snapshot only justified existing exceptions with dated review responsibility; fail new growth, not unchanged baselines.
- [Git index and working tree disagree] → run fixtures for staged add/delete/rename, and report paths that exist rather than crashing on removed files.
- [An active change looks complete but lacks qualification] → leave it active and document the missing gate; archive status is not product readiness.

## Migration Plan

Inventory maintained links and headings; create destination indexes; split evidence mechanically with a reversible source snapshot under the old path until verified; move selected current pages and leave stubs; add and test checkers; sync and archive completed OpenSpec changes one at a time; run docs, path, size, API, Go, UI/desktop, companion and Python checks. All moves are repository-local, with no installed workspace migration or deployment.
