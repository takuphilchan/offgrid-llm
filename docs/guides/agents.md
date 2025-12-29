# AI Agents

OffGrid LLM includes autonomous AI agents that can reason, use tools, and complete complex tasks.

---

## Quick Start

### CLI

```bash
offgrid agent chat                     # Interactive agent session
offgrid agent run "What is 25 * 47?"   # Run single task
offgrid agent templates                # List available templates
```

### Web UI

1. Open http://localhost:11611
2. Go to **Agent** tab
3. Select a model
4. Enter a task and click **Run**

---

## Templates

Pre-configured agent personas for common tasks:

| Template | Best For |
|----------|----------|
| `researcher` | Information gathering, summarization, fact-checking |
| `coder` | Writing code, debugging, code review |
| `analyst` | Data analysis, pattern recognition, reporting |
| `writer` | Content creation, editing, documentation |
| `sysadmin` | System administration, DevOps, troubleshooting |
| `planner` | Task breakdown, project planning, scheduling |

### Using Templates

**CLI:**
```bash
offgrid agent chat --template coder
offgrid agent chat --template researcher
offgrid agent run "Write a Python web server" --template coder
```

**API:**
```bash
curl -X POST http://localhost:11611/v1/agents/run \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama3",
    "prompt": "Debug this code: [code here]",
    "template": "coder"
  }'
```

---

## Agent Styles

| Style | Description | Best For |
|-------|-------------|----------|
| `react` | Reasoning + Acting, step-by-step | Multi-step tasks, tool use |
| `cot` | Chain of Thought | Analysis, explanations |

```bash
offgrid agent chat --style react
offgrid agent chat --style cot
```

---

## Built-in Tools

Agents can use these tools automatically:

| Tool | Description |
|------|-------------|
| `calculate` | Math calculations |
| `search_models` | Search HuggingFace |
| `list_models` | List local models |
| `search_documents` | Search RAG knowledge base |
| `read_file` | Read file contents |
| `write_file` | Write to files |
| `list_directory` | List directory contents |
| `http_get` | HTTP GET requests |
| `shell` | Execute shell commands |

### Manage Tools

```bash
offgrid agent tools                    # List all tools
```

**API:**
```bash
# Disable a tool
curl -X PATCH http://localhost:11611/v1/agents/tools \
  -H "Content-Type: application/json" \
  -d '{"name": "write_file", "enabled": false}'
```

---

## MCP Integration

Extend agents with [Model Context Protocol](https://modelcontextprotocol.io/) servers.

### Add MCP Servers

**CLI:**
```bash
offgrid agent mcp list
offgrid agent mcp add filesystem "npx -y @modelcontextprotocol/server-filesystem /tmp"
offgrid agent mcp test "npx -y @modelcontextprotocol/server-memory"
offgrid agent mcp remove filesystem
```

**API:**
```bash
curl -X POST http://localhost:11611/v1/agents/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "name": "filesystem",
    "url": "npx -y @modelcontextprotocol/server-filesystem /tmp"
  }'
```

### Popular MCP Servers

| Server | Command | Purpose |
|--------|---------|---------|
| Filesystem | `npx -y @modelcontextprotocol/server-filesystem /tmp` | File operations |
| Memory | `npx -y @modelcontextprotocol/server-memory` | Key-value store |
| SQLite | `npx -y @modelcontextprotocol/server-sqlite /tmp/db.sqlite` | Database |
| GitHub | `npx -y @modelcontextprotocol/server-github` | GitHub API |
| Brave Search | `npx -y @modelcontextprotocol/server-brave-search` | Web search |

---

## API Reference

### Run Agent

```bash
curl -X POST http://localhost:11611/v1/agents/run \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama3",
    "prompt": "Calculate factorial of 10",
    "style": "react",
    "max_steps": 10,
    "stream": true
  }'
```

| Field | Type | Description |
|-------|------|-------------|
| `model` | string | Model to use |
| `prompt` | string | Task description |
| `style` | string | `react` or `cot` |
| `template` | string | Template name (optional) |
| `max_steps` | int | Max reasoning steps (default: 10) |
| `stream` | bool | Stream step-by-step output |

### Streaming Response

```javascript
const response = await fetch('/v1/agents/run', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    model: 'llama3',
    prompt: 'Search for Python models',
    stream: true
  })
});

const reader = response.body.getReader();
while (true) {
  const { done, value } = await reader.read();
  if (done) break;
  
  const text = new TextDecoder().decode(value);
  for (const line of text.split('\n')) {
    if (line.startsWith('data: ')) {
      const data = JSON.parse(line.slice(6));
      console.log(data.step_type, data.content);
    }
  }
}
```

### Step Types

| Type | Description |
|------|-------------|
| `thought` | Agent reasoning |
| `action` | Tool call |
| `observation` | Tool result |
| `answer` | Final answer |
| `error` | Error message |

---

## Example Tasks

**Research:**
```bash
offgrid agent run "Find the top 3 Python ML libraries and compare them" --template researcher
```

**Coding:**
```bash
offgrid agent run "Write a REST API in Go with CRUD operations" --template coder
```

**Analysis:**
```bash
offgrid agent run "Analyze this CSV data and identify trends" --template analyst
```

**System Admin:**
```bash
offgrid agent run "Check disk usage and find large files" --template sysadmin
```

**Planning:**
```bash
offgrid agent run "Create a project plan for building a mobile app" --template planner
```

---

## Best Practices

1. **Be specific** - Clear prompts get better results
2. **Choose the right model** - 7B+ models reason better
3. **Use templates** - They provide optimized system prompts
4. **Limit tools** - Disable unused tools to reduce confusion
5. **Set max_steps** - Start with 10, increase for complex tasks
6. **Watch streaming** - Monitor step-by-step to understand behavior

---

## Troubleshooting

| Problem | Solution |
|---------|----------|
| Agent loops | Use larger model, be more specific, reduce tools |
| MCP won't connect | Check command, ensure npx installed, use `mcp test` |
| Tool fails | Check permissions, network access, review observation |
| Slow responses | Use smaller model, reduce max_steps |
| Wrong answers | Use template, add context, try different model |

---

## See Also

- [CLI Reference](../reference/cli.md)
- [API Reference](../reference/api.md)
- [RAG Guide](embeddings.md)
