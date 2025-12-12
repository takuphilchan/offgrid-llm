# Release Notes v0.2.6

**Date:** December 12, 2025

## 🚀 Highlights

This release focuses on synchronizing the Web UI with advanced backend capabilities, specifically introducing full management for the Model Context Protocol (MCP) and enhanced Agent History tracking. It also includes a corresponding update to the Python client SDK.

### ✨ New Features

*   **MCP Management UI**: 
    *   Added a dedicated interface for managing Model Context Protocol (MCP) servers.
    *   Users can now add, remove, and view the status of connected MCP servers directly from the UI.
    *   Backend support in `registry.go` and `server.go` to handle dynamic MCP server registration.

*   **Agent History & Sandbox**:
    *   **Execution History**: The system now tracks and persists agent execution history, allowing users to review past actions and results.
    *   **Sandbox Improvements**: Enhanced the agent sandbox environment for safer tool execution.
    *   **UI Integration**: The Agent tab now displays execution history, making it easier to debug and monitor agent performance.

*   **Python Client v0.1.6**:
    *   Updated the `offgrid` Python package to version `0.1.6`.
    *   Added support for the new Agent History APIs.
    *   Ensured feature parity with the v0.2.6 system backend.

### 🛠 Improvements

*   **P2P Networking**: Stability improvements for peer-to-peer node discovery and communication.
*   **UI Synchronization**: Aligned the Desktop and Web UI codebases to ensure a consistent experience across platforms.
*   **Repository Hygiene**: Cleaned up build artifacts and improved `.gitignore` to prevent accidental commits of binary files.

### 🐛 Bug Fixes

*   Fixed an issue where Python build artifacts (`.egg-info`) were incorrectly included in the source tree.
*   Resolved version mismatch issues between the system and the Python client.

## 📦 Installation / Upgrade

**Desktop App:**
Download the latest installer for your platform from the [Releases Page](https://github.com/offgrid-llm/offgrid/releases/tag/v0.2.6).

**Python Client:**
```bash
pip install --upgrade offgrid
```

**Docker:**
```bash
docker pull ghcr.io/offgrid-llm/offgrid:v0.2.6
```
