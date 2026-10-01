# Connect external MCP tools

MCP lets an agent call tools exposed by another server. It is optional: ordinary local chat does not need an MCP connection. Adding a server is an administrator action and does not prove the selected model can use its tools reliably.

## Connect in OffGrid

1. Open **Agents**, then **Connections**.
2. Give the connection a recognizable name.
3. Enter the server's MCP HTTP endpoint, not its homepage and not an `npx` command.
4. Choose **Test connection**, inspect the result, then **Connect**.
5. Review **Available tools** and run a small, non-sensitive task.

Connection testing checks protocol connectivity; task success also depends on the tool schema, model, permissions, and remote service. OffGrid's model can run locally while tool arguments are sent to an external destination. Never include private documents or credentials in a test prompt.

## Try a public documentation server

Microsoft documents a free Streamable HTTP server that does not require authentication. It searches documentation and code samples; see [Microsoft's overview](https://learn.microsoft.com/en-us/training/support/mcp) and [endpoint reference](https://learn.microsoft.com/en-us/training/support/mcp-developer-reference).

Connection name: `Microsoft Learn`

```text
https://learn.microsoft.com/api/mcp
```

Try:

> Use the Microsoft Learn tools to find the official Windows WSL installation instructions. Summarize the prerequisites and include source links. Do not change my computer.

Confirm that the activity shows a remote tool call and that the cited pages support the answer. A response from model memory alone does not test MCP. This is a third-party service; its availability and tool set can change independently of OffGrid.

## Understand local addresses

The **OffGrid service** connects to the MCP server, not your browser. `127.0.0.1` means the environment running that service:

| OffGrid service runs in | `http://127.0.0.1:3000/mcp` refers to |
| --- | --- |
| Native Windows/macOS/Linux process | Port 3000 on that host |
| Docker container | Port 3000 inside that container |
| WSL distribution | Port 3000 reachable in that WSL environment |

An endpoint in Windows is not automatically reachable at the container's loopback address. Use an explicitly configured host route or a private Docker network appropriate to your deployment. Do not expose a local tool server to the LAN just to clear a connection error, mount the Docker socket, or disable TLS verification.

## Troubleshoot

| Error | Meaning and next step |
| --- | --- |
| Connection refused | No listener accepted the connection at that address from the service environment. Start the intended MCP server or correct the address. |
| 404 or protocol initialization failure | Verify the exact MCP endpoint and supported transport. A website homepage is not an MCP endpoint. |
| 401 or 403 | The server needs authentication/authorization. Use only an auth mechanism actually supported by your OffGrid client; the basic URL form is not a general OAuth setup wizard. |
| Browser shows 405 on an MCP URL | A normal browser GET is not an MCP handshake. Use **Test connection**. |
| Connected but no useful result | Inspect available tools, model tool calling, policy denials, and remote errors. Connection success is not agent qualification. |

Keep required VPNs enabled. Do not silently turn off address validation or substitute an untrusted endpoint to get a test to pass.

## Remove a connection

In **Connections**, choose **Remove connection** beside the saved server and confirm. Offline saved connections can also be removed. OffGrid forgets the saved connection, closes its transport, and removes its tools from discovery and execution. A settings write failure leaves the connection intact and reports an error.

Removal does not erase task history, delete the remote server's data, or undo actions already sent. Stop related work first where possible. Reconnect explicitly if you need the server again.

The HTTP removal endpoint is `DELETE /v1/agents/mcp?name=URL_ENCODED_NAME`. See the [API reference](../reference/api.md) for authentication and the [agent guide](agents.md) for approvals and recovery.
