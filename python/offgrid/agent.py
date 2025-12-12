"""
OffGrid Agent - AI Agents with Tool Use

Run autonomous AI agents that can use tools to complete tasks.

Example:
    >>> from offgrid import Client
    >>> client = Client()
    >>> 
    >>> # Run an agent task
    >>> result = client.agent.run("Calculate 127 * 48 + 356", model="llama3.2:3b")
    >>> print(result["result"])
    >>> 
    >>> # List available tools
    >>> tools = client.agent.tools()
    >>> for tool in tools:
    ...     print(f"{tool['name']}: {tool['description']}")
    >>> 
    >>> # Disable a tool
    >>> client.agent.toggle_tool("shell", enabled=False)
"""

from typing import Dict, List, Optional, TYPE_CHECKING

if TYPE_CHECKING:
    from .client import Client


class Agent:
    """
    AI Agent manager for running autonomous tasks with tool use.
    
    Agents can use built-in tools like calculator, file operations,
    HTTP requests, and shell commands to complete complex tasks.
    
    Example:
        >>> agent = client.agent
        >>> result = agent.run("What time is it?", model="llama3.2:3b")
        >>> print(result["result"])
    """
    
    def __init__(self, client: "Client"):
        self._client = client
        self.mcp = MCP(client)
    
    def run(
        self,
        prompt: str,
        model: str = None,
        style: str = "react",
        max_steps: int = 10,
        tools: List[str] = None,
        **kwargs
    ) -> Dict:
        """
        Run an agent task.
        
        The agent will autonomously use available tools to complete the task.
        
        Args:
            prompt: The task description or question
            model: Model to use (uses first available if not specified)
            style: Agent style - "react" (default) or "simple"
            max_steps: Maximum number of tool-use steps (default: 10)
            tools: Optional list of specific tools to use
            **kwargs: Additional parameters
        
        Returns:
            Dict with:
                - result: Final answer/result
                - steps: List of steps taken
                - tool_calls: Tools that were used
                - tokens_used: Total tokens consumed
        
        Example:
            >>> result = agent.run("Calculate 2024 - 1990", model="llama3.2:3b")
            >>> print(result["result"])  # "34"
        """
        if model is None:
            model = self._client._get_default_model()
        
        payload = {
            "model": model,
            "prompt": prompt,
            "style": style,
            "max_steps": max_steps,
            **kwargs
        }
        
        if tools is not None:
            payload["tools"] = tools
        
        return self._client._request("POST", "/v1/agents/run", payload)
    
    def tools(self) -> List[Dict]:
        """
        List all available agent tools.
        
        Returns:
            List of tool dictionaries with name, description, enabled status
        
        Example:
            >>> tools = agent.tools()
            >>> for tool in tools:
            ...     status = "✓" if tool["enabled"] else "✗"
            ...     print(f"[{status}] {tool['name']}: {tool['description']}")
        """
        response = self._client._request("GET", "/v1/agents/tools")
        return response.get("tools", [])
    
    def tasks(self) -> List[Dict]:
        """
        List agent task history.
        
        Returns:
            List of past agent tasks with status and steps.
        
        Example:
            >>> tasks = agent.tasks()
            >>> for task in tasks:
            ...     print(f"{task['id']}: {task['prompt']} ({task['status']})")
        """
        response = self._client._request("GET", "/v1/agents/tasks")
        return response.get("tasks", [])
    
    def toggle_tool(self, name: str, enabled: bool) -> Dict:
        """
        Enable or disable a specific tool.
        
        Args:
            name: Tool name (e.g., "shell", "calculator", "http_get")
            enabled: True to enable, False to disable
        
        Returns:
            Updated tool status
        
        Example:
            >>> agent.toggle_tool("shell", enabled=False)
            >>> agent.toggle_tool("calculator", enabled=True)
        """
        payload = {"name": name, "enabled": enabled}
        return self._client._request("PATCH", "/v1/agents/tools", payload)
    
    def enable_tool(self, name: str) -> Dict:
        """Enable a tool by name."""
        return self.toggle_tool(name, enabled=True)
    
    def disable_tool(self, name: str) -> Dict:
        """Disable a tool by name."""
        return self.toggle_tool(name, enabled=False)


class MCP:
    """
    MCP (Model Context Protocol) server manager.
    
    Connect external tools via MCP servers to extend agent capabilities.
    
    Example:
        >>> mcp = client.agent.mcp
        >>> mcp.add("filesystem", "npx -y @modelcontextprotocol/server-filesystem /tmp")
        >>> servers = mcp.list()
    """
    
    def __init__(self, client: "Client"):
        self._client = client
    
    def list(self) -> List[Dict]:
        """
        List all configured MCP servers.
        
        Returns:
            List of MCP server configurations
        
        Example:
            >>> servers = mcp.list()
            >>> for s in servers:
            ...     print(f"{s['name']}: {s['url']}")
        """
        response = self._client._request("GET", "/v1/agents/mcp")
        return response.get("servers", [])
    
    def add(self, name: str, url: str) -> Dict:
        """
        Add a new MCP server.
        
        Args:
            name: Unique name for the server
            url: Server URL or command (e.g., "npx -y @modelcontextprotocol/server-filesystem /tmp")
        
        Returns:
            Server configuration
        
        Example:
            >>> mcp.add("filesystem", "npx -y @modelcontextprotocol/server-filesystem /tmp")
            >>> mcp.add("github", "npx -y @modelcontextprotocol/server-github")
        """
        payload = {"name": name, "url": url}
        return self._client._request("POST", "/v1/agents/mcp", payload)
    
    def test(self, name: str = None, url: str = None) -> Dict:
        """
        Test an MCP server connection.
        
        Args:
            name: Existing server name to test, OR
            url: New server URL to test
        
        Returns:
            Connection test result with tools count
        
        Example:
            >>> result = mcp.test(url="npx -y @modelcontextprotocol/server-filesystem /tmp")
            >>> print(f"Found {result['tools_count']} tools")
        """
        payload = {}
        if name:
            payload["name"] = name
        if url:
            payload["url"] = url
        return self._client._request("POST", "/v1/agents/mcp/test", payload)
