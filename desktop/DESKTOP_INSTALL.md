# Install OffGrid Desktop

Use the [installation guide](../docs/setup/installation.md) to choose a release package, verify its checksum, and install it on Windows, macOS, or Linux. The published asset list determines which architectures and formats are available.

The desktop includes its matching service, UI, and host components. Models are downloaded separately. A running external service must match the desktop's version and UI build before desktop can attach.

**Installing desktop does not update an existing Docker container.** For that combination, update the service separately while preserving its workspace, then retry the desktop connection. See [startup and recovery](../docs/setup/desktop-startup.md).

For source builds and packaging, use [desktop development](README.md). Publisher signing and native-automation qualification are separate from package availability; consult the release notes rather than assuming either from a successful installation.
