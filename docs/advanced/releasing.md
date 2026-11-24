# Releasing OffGrid

The source tag is immutable. Release repair must not force-move a version tag
or invent a new version just to rerun packaging.

## Normal release

1. Update the CLI and Electron versions, release notes, and expected asset list
   in `.github/scripts/finalize-release.sh` when the package matrix changes.
2. Run `go test ./...`, the web/desktop checks, and
   `node scripts/release/test-finalize-release.mjs`. CI also runs `actionlint` on
   every active workflow.
3. Push the source tag `vX.Y.Z`. The tag starts `release-unified.yml`, which
   builds desktop and CLI assets and calls the reusable `docker-publish.yml`
   workflow for Docker Hub images. Container publication requires the
   `DOCKERHUB_TOKEN` repository secret with push access to
   `takuphilchan/offgrid-llm`.
4. Verify all 16 expected GitHub assets (14 packages and two desktop trust
   reports) and `checksums-vX.Y.Z.sha256`, plus
   the Linux AMD64 and ARM64 images under the exact Docker Hub `X.Y.Z` tag
   and the Linux AMD64 image under `X.Y.Z-gpu`.
   A GitHub release is not complete merely because its page is public.

The source commit must pass `ci.yml` before release builders proceed. An unsigned
preview requires an explicit workflow dispatch with `unsigned_preview=true` and
the immutable version tag; a tag-triggered run does not imply that disclosure.
Preview publication remains a prerelease and must not promote stable aliases.

The release finalizer checks every expected asset's uploaded state, nonzero
size, and GitHub SHA-256 digest before attaching the checksum file. Desktop
filenames contain spaces locally, but GitHub stores them with dots; reruns
compare the stored names.

Before starting platform builders, the workflow resolves the draft with
`gh release view --json databaseId` and checks asset access by numeric release
ID. Drafts are not available through the REST `releases/tags/{tag}` endpoint;
a 404 there does **not** prove the release or its assets are missing. Asset
queries use the paginated `releases/{id}/assets` endpoint, including raw SHA-256
digests. CI exercises the actual workflow filters against empty drafts,
complete assets, pending digests, and access errors.

The container workflow builds AMD64 and ARM64 CPU images concurrently on
native GitHub-hosted runners, publishes resumable architecture tags, and joins
them into one multi-platform manifest with bounded registry-indexing retries.
The CUDA image is an independent AMD64 job. Every large build has a timeout,
so a stalled architecture cannot occupy a runner indefinitely. The GitHub
release finalizer depends on the reusable container workflow and verifies the
exact CPU and GPU tags again before publishing the release.

## Repairing an existing version

If one or more GitHub assets are missing, run `release-unified.yml` with
`workflow_dispatch` on `main` and `version=vX.Y.Z`, or push a
`release-build/vX.Y.Z` branch from the reviewed workflow commit. Existing
verified CLI bundles, desktop installers, and containers are reused. The
version's source is checked out from the tag; the current workflow and finalizer
come from the dispatch or repair-branch revision. Never rerun an old failed Actions
attempt to pick up a workflow fix: it uses the old workflow revision.

If Docker Hub did not publish, push a branch named `release-container/vX.Y.Z`
from the reviewed workflow commit. `docker-publish.yml` checks out the existing
tagged application source; reviewed CPU or GPU Dockerfile repairs can come from
the repair branch. Already-published CPU and GPU tags are reused when their
platforms are complete. This path always publishes the exact `X.Y.Z` and
`X.Y.Z-gpu` tags. It updates `latest`, `latest-gpu`, and the minor-version
aliases only when `vX.Y.Z` is the repository's highest stable tag, so repairing
an older release cannot move those aliases backward.
Confirm both the multi-platform CPU tag and AMD64 GPU tag are pullable before
proceeding.

To deliberately rebuild already-published containers, dispatch
`docker-publish.yml` from the reviewed workflow revision on `main` with
`version=X.Y.Z`, `source_ref=vX.Y.Z`, `force_rebuild=true`, and
`publish_gpu=true`. The source tag must match the requested version. The
workflow applies reviewed Dockerfiles from the dispatch revision while keeping
the application source pinned to the immutable tag, then rebuilds both CPU
architectures, republishes their manifest, and rebuilds the NVIDIA AMD64 image.
This replaces the versioned Docker tags and, for the highest stable tag, their
latest/minor aliases. It does not move the Git tag or modify GitHub release
assets. Verify the CPU AMD64/ARM64 manifest, GPU AMD64 image, and source identity
after the run.

When every asset already exists and only checksums/notes/final publication
failed, push `release-finalize/vX.Y.Z` from the reviewed workflow commit.
`release-finalize.yml` verifies the source tag, CPU and GPU container platforms,
all GitHub assets, and then publishes checksums and notes without rebuilding
large native bundles. The branch can be updated to retry after a workflow fix;
the original tag remains unchanged.
