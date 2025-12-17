# Release Notes - v0.2.1

**Release Date:** November 30, 2025

## Overview

Version 0.2.1 is a patch release that improves CLI reliability with model readiness checks and fixes model switching issues.

---

## What's New

### CLI Model Readiness Check

**Problem Solved**
- Previously, `offgrid run` would show the chat prompt while the model was still loading
- This led to empty responses if users typed before the model finished loading

**Solution**
- Added robust model readiness verification using a test completion
- The CLI now waits until the model can actually generate responses
- Supports up to 120 seconds timeout for larger models

### Fixed Model Switching

**Problem Solved**
- Switching between models via CLI caused empty responses
- CLI was managing llama-server directly, bypassing the server's model cache
- Newly downloaded models weren't visible until server restart

**Solution**
- Model switching now goes through the OffGrid server's API
- Server's model registry is refreshed before switching
- Proper model cache management ensures correct model is loaded

---

## Technical Details

### Model Switching Flow (Fixed)

1. CLI calls `/models/refresh` to update server's model list
2. CLI sends test completion request with target model name
3. Server's model cache handles loading the new model
4. CLI waits for successful response before showing prompt

### New Function: `waitForModelReady`

```go
// waitForModelReady waits for the model to be fully loaded and ready
// It performs a test completion to verify the model can actually respond
func waitForModelReady(port string, maxWaitSeconds int) error
```

---

## Installation

### Upgrade from v0.2.0

```bash
# Pull latest and rebuild
git pull origin main
go build -o offgrid ./cmd/offgrid
sudo cp offgrid /usr/local/bin/
```

### Fresh Install

```bash
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/desktop.sh | bash
```

---

## Full Changelog

See [GitHub Commits](https://github.com/takuphilchan/offgrid-llm/compare/v0.2.0...v0.2.1) for the complete list of changes.
