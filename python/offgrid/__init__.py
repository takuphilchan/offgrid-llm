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
"""

from typing import Dict, List, Union

__version__ = "0.1.2"
__author__ = "OffGrid LLM Team"

from .client import Client, OffGridError, Sessions
from .models import ModelManager
from .kb import KnowledgeBase

__all__ = [
    "Client",
    "Sessions",
    "OffGridError",
    "ModelManager",
    "KnowledgeBase",
    "__version__",
]
