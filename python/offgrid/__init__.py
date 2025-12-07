"""
OffGrid - Python Client for OffGrid LLM

Run AI models completely offline on your own computer.

Usage:
    import offgrid
    
    # Default (localhost:11611)
    client = offgrid.Client()
    
    # Custom server
    client = offgrid.Client(host="http://192.168.1.100:11611")
    
    # With API key authentication
    client = offgrid.Client(api_key="your-secret-key")
    
    # Chat
    response = client.chat("Hello!")
    
    # Streaming
    for chunk in client.chat("Tell me a story", stream=True):
        print(chunk, end="", flush=True)
    
    # Sessions for conversation persistence
    sessions = client.sessions
    session = sessions.create("my-chat")
    sessions.chat_with_session("my-chat", "Hello!")
    
    # AI Agents with tool use
    result = client.agent.run("Calculate 127 * 48", model="llama3.2:3b")
    print(result["result"])
    
    # MCP server integration
    client.agent.mcp.add("filesystem", "npx -y @modelcontextprotocol/server-filesystem /tmp")
"""

from typing import Dict, List, Union

__version__ = "0.1.3"
__author__ = "OffGrid LLM Team"

from .client import Client, OffGridError, Sessions
from .models import ModelManager
from .kb import KnowledgeBase
from .agent import Agent, MCP
from .lora import LoRA

__all__ = [
    "Client",
    "Sessions",
    "OffGridError",
    "ModelManager",
    "KnowledgeBase",
    "Agent",
    "MCP",
    "LoRA",
    "__version__",
]
