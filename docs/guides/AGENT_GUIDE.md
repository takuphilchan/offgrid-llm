# AI Agent Guide

OffGrid LLM includes an AI Agent system that can autonomously perform tasks using tools. The agent uses a ReAct (Reasoning + Acting) approach to break down problems and execute solutions step by step.

## Quick Start

### Using the UI

1. Navigate to the **Agent** tab in the UI
2. Select a model (any GGUF model works, but larger models perform better)
3. Enter a task description
4. Click **Run Agent**

### Using the API

```bash
curl -X POST http://localhost:11611/v1/agents/run \
  -H "Content-Type: application/json" \
  -d '{
    "model": "qwen2.5-7b-instruct",
    "prompt": "Calculate the factorial of 10",
    "style": "react",
    "max_steps": 10,
    "stream": true
  }'
```

## Agent Styles

| Style | Description |
|-------|-------------|
| `react` | ReAct (Reasoning + Acting) - Best for multi-step tasks |
| `cot` | Chain of Thought - Good for analytical tasks |

## Built-in Tools

OffGrid comes with several built-in tools:

| Tool | Description |
|------|-------------|
| `calculate` | Perform mathematical calculations |
| `search_models` | Search for models on HuggingFace |
| `list_models` | List locally available models |
| `search_documents` | Search RAG documents |
| `read_file` | Read file contents |
| `write_file` | Write content to files |
| `list_directory` | List directory contents |
| `http_get` | Make HTTP GET requests |

### Enable/Disable Tools

You can enable or disable tools in the UI or via API:

```bash
# Disable a tool
curl -X PATCH http://localhost:11611/v1/agents/tools \
  -H "Content-Type: application/json" \
  -d '{"name": "write_file", "enabled": false}'

# List all tools with status
curl "http://localhost:11611/v1/agents/tools?all=true"
```

## MCP Server Integration

OffGrid supports [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) servers, allowing you to extend agent capabilities with external tools.

### Adding MCP Servers via UI

1. Go to **Agent** tab
2. Click **Add** in the MCP Servers panel
3. Enter server name and command/URL
4. Click **Test Connection** to verify
5. Click **Add Server**

### Popular MCP Servers

| Server | Command | Description |
|--------|---------|-------------|
| Filesystem | `npx -y @modelcontextprotocol/server-filesystem /tmp` | Read/write files |
| Memory | `npx -y @modelcontextprotocol/server-memory` | Key-value store |
| SQLite | `npx -y @modelcontextprotocol/server-sqlite /tmp/test.db` | Database operations |
| GitHub | `npx -y @modelcontextprotocol/server-github` | GitHub API |

### Adding MCP Servers via API

```bash
# Test connection first
curl -X POST http://localhost:11611/v1/agents/mcp/test \
  -H "Content-Type: application/json" \
  -d '{"url": "npx -y @modelcontextprotocol/server-filesystem /tmp"}'

# Add the server
curl -X POST http://localhost:11611/v1/agents/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "name": "filesystem",
    "url": "npx -y @modelcontextprotocol/server-filesystem /tmp"
  }'
```

## Streaming Output

The agent supports streaming for real-time step updates:

```javascript
const response = await fetch('/v1/agents/run', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    model: 'qwen2.5-7b-instruct',
    prompt: 'Search for Python models',
    stream: true
  })
});

const reader = response.body.getReader();
while (true) {
  const { done, value } = await reader.read();
  if (done) break;
  
  const text = new TextDecoder().decode(value);
  const lines = text.split('\n');
  
  for (const line of lines) {
    if (line.startsWith('data: ')) {
      const data = JSON.parse(line.substring(6));
      console.log('Step:', data.step_type, data.content);
    }
  }
}
```

## Step Types

| Type | Description |
|------|-------------|
| `thought` | Agent reasoning about the problem |
| `action` | Agent calling a tool |
| `observation` | Result from tool execution |
| `answer` | Final answer to the task |
| `error` | Error occurred during execution |

## Configuration

### Max Steps

Limit the number of steps an agent can take:

```json
{
  "max_steps": 10
}
```

### Context Size

The agent uses an 8192 token context window by default, suitable for complex tasks with many tools.

## Example Tasks

### Research Task
```
"Search for the top 3 Python machine learning models and compare their sizes"
```

### File Operations
```
"Create a file called notes.txt with today's date and a reminder to check emails"
```

### Calculations
```
"Calculate the monthly payment for a $300,000 mortgage at 6.5% APR over 30 years"
```

## Best Practices

1. **Use specific prompts**: Clear, detailed prompts produce better results
2. **Choose appropriate models**: Larger models (7B+) handle complex reasoning better
3. **Limit tools**: Disable unused tools to reduce confusion
4. **Monitor steps**: Watch the streaming output to understand agent behavior
5. **Set reasonable max_steps**: Start with 10, increase for complex tasks

## Troubleshooting

### Agent loops or repeats actions
- Try a larger model
- Add more specific instructions to the prompt
- Reduce the number of available tools

### MCP server won't connect
- Ensure the command or URL is correct
- Check that required dependencies are installed (e.g., `npx`)
- Use the Test Connection button to diagnose issues

### Tool execution fails
- Check file permissions for filesystem operations
- Verify network access for HTTP tools
- Review the observation output for error messages
