# Version management

`VERSION` is the release version source. The Go binary receives it through
linker flags; Electron and React package metadata are updated together with
their npm lockfiles.

## Preparing a version

1. Edit `VERSION` with a valid semantic version, without a leading `v`.
2. Run:

   ```bash
   ./scripts/update-version.sh
   ```

3. Review changes to `VERSION`, `desktop/package.json`,
   `desktop/package-lock.json`, `web/app/package.json`, and
   `web/app/package-lock.json`.
4. Run the checks in the root README before creating a release.

The update script is idempotent. It does not rewrite generated UI assets or Go
source to publish a version.

## Build-time injection

```bash
version="$(cat VERSION)"
go build -trimpath -ldflags="-s -w -X main.Version=$version" -o offgrid ./cmd/offgrid
```

Container and unified release workflows inject the same value and include
source revision/build metadata. `offgrid version`, the root API response, and
the desktop About information should therefore identify the packaged artifact.

## Tags

- `edge` is the manually published development container label.
- Stable releases use a Git tag such as `v1.0.0`.
- Stable container publishing also creates `1.0.0`, `1.0`, `latest`, and an
  immutable `sha-*` tag.

Do not create a release tag merely to update `edge`. Release notes are
historical and keep their original version references.
