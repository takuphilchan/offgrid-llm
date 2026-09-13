# OffGrid provider for Hermes Agent

This plugin registers `offgrid` as a Hermes model provider. It talks directly
to OffGrid's OpenAI-compatible API.

Set `OFFGRID_BASE_URL` to the reachable OffGrid `/v1` URL and set
`OFFGRID_API_KEY` to `offgrid-local` when the local service does not require a
token. Hermes Agent requires at least a 64,000-token runtime window and OffGrid
recommends 65,536 tokens for long-running agent sessions.
