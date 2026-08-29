# Desktop installation

Desktop packages are produced by the release workflow for Windows, macOS, and
Linux. Until a signed release is published, build on the target operating
system by following [README.md](README.md).

Published packages will include:

- a per-user Windows installer and portable executable;
- macOS x64 and ARM64 archives;
- a Linux x64 AppImage and Debian package.

The application bundles the OffGrid runtime and React UI. Models are downloaded
after installation and remain in the user's `~/.offgrid-llm/models` directory.
