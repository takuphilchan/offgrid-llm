# Version Management

OffGrid LLM uses a centralized version management system.

## Single Source of Truth

The `VERSION` file at the repository root contains the current version:

```
0.1.7
```

## How It Works

All version references are managed programmatically:

1. **VERSION file** - Single source of truth (repository root)
2. **update-version.sh** - Updates all files when version changes
3. **Build scripts** - Read VERSION file automatically
4. **Runtime** - Version set via ldflags during compilation

## Updating the Version

To bump the version:

1. Edit the `VERSION` file:
   ```bash
   echo "0.1.8" > VERSION
   ```

2. Run the update script:
   ```bash
   ./scripts/update-version.sh
   ```

3. Commit the changes:
   ```bash
   git add VERSION desktop/package.json desktop/index.html scripts/build-all.sh internal/p2p/discovery.go
   git commit -m "chore: bump version to 0.1.8"
   ```

## Files Updated Automatically

The `update-version.sh` script updates:

- `desktop/package.json` - Electron app version
- `desktop/index.html` - UI version displays
- `scripts/build-all.sh` - Build script version variable
- `internal/p2p/discovery.go` - P2P protocol version

## Build-Time Version Injection

The main binary version is injected at build time via ldflags:

```bash
go build -ldflags="-X main.Version=$(cat VERSION)" ./cmd/offgrid
```

The `scripts/build-all.sh` script does this automatically.

## Version Format

Follow semantic versioning (semver):

- **MAJOR.MINOR.PATCH** (e.g., 0.1.7)
- **MAJOR** - Breaking changes
- **MINOR** - New features (backwards compatible)
- **PATCH** - Bug fixes

## Checking Current Version

```bash
# From VERSION file
cat VERSION

# From built binary
./offgrid version

# From desktop app
# See bottom of sidebar in UI
```

## CI/CD Integration

Build scripts automatically read the VERSION file:

```bash
# Build with correct version
./scripts/build-all.sh

# Docker build with version tag
cd docker
./docker-build.sh  # Uses VERSION file automatically
```

## Migration from Hardcoded Versions

Previously, versions were hardcoded in multiple files. Now:

- **Before**: Manually update 10+ files for each release
- **After**: Edit VERSION file, run `update-version.sh`, done

## Documentation References

Documentation files may reference specific versions as examples (e.g., in installation commands). These are fine to keep as-is and don't need to be updated unless creating new documentation.

Example references that are OK:
- Release notes referencing historical versions
- Installation examples showing specific version downloads
- Git tag examples in documentation

## Scripts

- `scripts/get-version.sh` - Returns current version from VERSION file
- `scripts/update-version.sh` - Updates all files to match VERSION
- `scripts/build-all.sh` - Builds with version from VERSION file
- `docker/docker-build.sh` - Docker build with version from VERSION file
