# Proposal

## Why

The public Python client, optional Python runtime workers, and maintained developer scripts currently share `python/` and `dev/` trees with mixed ownership. Contributors must infer which files are SDK artifacts, trusted service workers, tests, build helpers, or historical experiments. The application source move makes this boundary the next source of path drift.

## What Changes

- Move the independently versioned `offgrid` Python distribution, tests, examples, and README to `sdk/python` without changing the distribution, import path, client behavior, or version policy.
- Move `audio_service.py` and the pinned provider worker to `runtimes/python`; preserve worker protocol, pinning, supervised-process trust, and offline behavior. The user approved moving the provider worker now while its provider change remains active, with protocol regression checks.
- Rehome maintained `dev/` scripts, native-test Dockerfile, metadata tool, and useful client examples by responsibility. Remove obsolete developer-only material only after confirming no maintained consumer and preserving unique installer behavior.
- Add a small root `Makefile` that delegates to direct commands, and document equivalent direct commands for Windows contributors.
- Update CI, packaging, tests, runtime discovery, docs, and repository-path checks for the new paths. Add isolated Linux and Windows Python SDK test jobs.

## Capabilities

### New Capabilities

- `development-workflows`: Maintained build/test entry points, separated Python SDK/runtime packaging, and source-path ownership checks for contributors and CI.

### Modified Capabilities

None. The existing repository-hygiene requirements remain intact.

## Impact

This is a repository-layout and contributor-workflow change. It touches `python/`, `dev/`, `scripts/`, `docker/`, `tools/`, SDK/runtime tests, CI, package metadata, and maintained docs. Public HTTP/CLI/Python interfaces, desktop identity, model/provider execution semantics, installer URLs, and installed-user data are unchanged. It does not finish MLX or Transformers qualification, change the inference scheduler, install Python dependencies or models, deploy, commit, or publish.
