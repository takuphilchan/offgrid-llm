# OffGrid provider for OpenClaw

This plugin registers `offgrid` as a native OpenClaw model provider using
OffGrid's OpenAI-compatible `/v1` API.

Set `OFFGRID_API_KEY=offgrid-local` for a local no-auth OffGrid service and
use the setup wizard or generated OffGrid configuration to choose the service
URL. Then use model references such as
`offgrid/phi-3.5-mini-instruct.Q4_K_M`.
