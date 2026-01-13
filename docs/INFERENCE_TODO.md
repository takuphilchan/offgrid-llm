````markdown
# Inference Integration Notes (Roadmap)

This short document collects the actionable steps required to move OffGrid from mock inference to a production-grade, real inference default.

See also: [advanced inference roadmap](advanced/inference-roadmap.md)

Status: partial — mock engine active, `llama-server` integration in progress.

Key tasks

- Provide clear, tested instructions for installing and running `llama-server` per OS/arch.
- Add prebuilt `llama-server` binaries (or packaging instructions) for Linux, macOS and Windows.
- Implement robust lifecycle management: start/stop, crash detection, health checks, port handling.
- Ensure model load/unload correctness and expose accurate status via server endpoints.
- Validate streaming parity with OpenAI-compatible SSE semantics and provide examples for clients.
- Document CPU feature requirements (AVX/AVX2) and how to detect/advise fallbacks.
- Add test harnesses and CI checks that exercise real inference on representative CI machines (or emulators).

Quick references

- Implementation entry points: `internal/inference/llama_http.go`, `internal/models/registry.go`, `internal/server/server.go`.
- Developer note: if you see `MOCK MODE` messages in the UI or logs, the mock engine is active and this document is the integration checklist.

How to contribute

1. Follow `docs/advanced/LLAMA_CPP_SETUP.md` to reproduce a working `llama-server` locally.
2. Implement small feature patches (lifecycle, health endpoints) and submit PRs.
3. Add tests that exercise model load/unload and streaming.

````
