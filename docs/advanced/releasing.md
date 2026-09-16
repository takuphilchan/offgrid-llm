# Releasing OffGrid

The source tag is immutable. Release repair must not force-move a version tag
or invent a new version just to rerun packaging.

## Normal release

1. Update the CLI and Electron versions, release notes, and expected asset list
   in `.github/scripts/finalize-release.sh` when the package matrix changes.
2. Run `go test ./...`, the web/desktop checks, and
   `node dev/scripts/test-finalize-release.mjs`. CI also runs `actionlint` on
   every active workflow.
3. Push the source tag `vX.Y.Z`. The tag starts both
   `release-unified.yml` (desktop and CLI assets) and `docker-publish.yml`
   (Docker Hub images). The latter requires the `DOCKERHUB_TOKEN` repository
   secret with push access to `takuphilchan/offgrid-llm`.
4. Verify all 14 expected GitHub assets and `checksums-vX.Y.Z.sha256`, plus
   the Linux AMD64 and ARM64 images under the exact Docker Hub `X.Y.Z` tag
   and the Linux AMD64 image under `X.Y.Z-gpu`.
   A GitHub release is not complete merely because its page is public.

The release finalizer checks every expected asset's uploaded state, nonzero
size, and GitHub SHA-256 digest before attaching the checksum file. Desktop
filenames contain spaces locally, but GitHub stores them with dots; reruns
compare the stored names.

The container workflow builds AMD64 and ARM64 CPU images concurrently on
native GitHub-hosted runners and joins their immutable digests into one
multi-platform manifest. The CUDA image is an independent AMD64 job. Every
large build has a timeout, so a stalled architecture cannot occupy a runner
indefinitely or prevent the completed platform from being diagnosed.

## Repairing an existing version

If one or more GitHub assets are missing, run `release-unified.yml` with
`workflow_dispatch` on `main` and `version=vX.Y.Z`. Existing verified CLI
bundles are reused. The version's source is checked out from the tag; the
current finalizer script comes from `main`. Never rerun an old failed Actions
attempt to pick up a workflow fix: it uses the old workflow revision.

If Docker Hub did not publish, push a branch named `release-container/vX.Y.Z`
from the reviewed workflow commit. `docker-publish.yml` checks out the existing
tagged application source; a reviewed GPU Dockerfile repair can come from the
repair branch. Already-published CPU and GPU tags are reused when their
platforms are complete. This path always publishes the exact `X.Y.Z` and
`X.Y.Z-gpu` tags. It updates `latest`, `latest-gpu`, and the minor-version
aliases only when `vX.Y.Z` is the repository's highest stable tag, so repairing
an older release cannot move those aliases backward.
Confirm both the multi-platform CPU tag and AMD64 GPU tag are pullable before
proceeding.

When every asset already exists and only checksums/notes/final publication
failed, push `release-finalize/vX.Y.Z` from the reviewed workflow commit.
`release-finalize.yml` verifies the source tag, CPU and GPU container platforms,
all GitHub assets, and then publishes checksums and notes without rebuilding
large native bundles. The branch can be updated to retry after a workflow fix;
the original tag remains unchanged.
