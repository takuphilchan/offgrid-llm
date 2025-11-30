# Release Notes - v0.2.1

**Release Date:** November 30, 2025

## Overview

Version 0.2.1 is a patch release that improves CLI reliability by ensuring the model is fully ready before accepting user input.

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

**How It Works**
1. Checks llama.cpp health endpoint
2. Detects "loading model" status during model initialization
3. Performs a minimal test completion to verify the model responds
4. Only then shows the chat prompt to the user

---

## Technical Details

### New Function: `waitForModelReady`

```go
// waitForModelReady waits for the model to be fully loaded and ready
// It performs a test completion to verify the model can actually respond
func waitForModelReady(port string, maxWaitSeconds int) error
```

- Polls health endpoint for status changes
- Sends a minimal chat completion request (`"Hi"` with `max_tokens: 1`)
- Returns only when the model produces valid output
- Timeout configurable (default: 120 seconds)

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
