"""
OffGrid LLM Python Client

Run AI models completely offline on your own computer.

Usage:
    import offgrid_llm as offgrid
    
    # Simple one-liner
    response = offgrid.chat("Hello!")
    
    # Full control
    client = offgrid.Client()
    client.chat("What is AI?", model="Llama-3.2-3B-Instruct-Q4_K_M")
    
    # Knowledge Base
    client.kb.add("documents/notes.md")
    client.chat("Summarize my notes", use_kb=True)
    
    # Model management
    client.download("bartowski/Llama-3.2-3B-Instruct-GGUF", "Llama-3.2-3B-Instruct-Q4_K_M.gguf")
    client.search("llama", ram=8)
"""

__version__ = "0.2.1"
__author__ = "OffGrid LLM Team"

from .client import Client, OffGridError
from .models import ModelManager
from .kb import KnowledgeBase

# Default client instance for convenience functions
_default_client = None


def _get_client():
    """Get or create the default client instance."""
    global _default_client
    if _default_client is None:
        _default_client = Client()
    return _default_client


def chat(
    message: str,
    model: str = None,
    system: str = None,
    use_kb: bool = False,
    stream: bool = False,
    **kwargs
) -> str:
    """
    Send a chat message and get a response.
    
    Args:
        message: The user message
        model: Model name (uses first available if not specified)
        system: Optional system prompt
        use_kb: Whether to use Knowledge Base for context
        stream: Whether to stream the response
        **kwargs: Additional parameters (temperature, max_tokens, etc.)
    
    Returns:
        The assistant's response text
    
    Example:
        >>> import offgrid_llm as offgrid
        >>> offgrid.chat("What is Python?")
        'Python is a high-level programming language...'
    """
    return _get_client().chat(message, model=model, system=system, use_kb=use_kb, stream=stream, **kwargs)


def complete(prompt: str, model: str = None, **kwargs) -> str:
    """
    Generate a text completion.
    
    Args:
        prompt: The prompt to complete
        model: Model name (uses first available if not specified)
        **kwargs: Additional parameters
    
    Returns:
        The completion text
    """
    return _get_client().complete(prompt, model=model, **kwargs)


def embed(text: str | list[str], model: str = None) -> list[float] | list[list[float]]:
    """
    Generate embeddings for text.
    
    Args:
        text: Single string or list of strings
        model: Embedding model name
    
    Returns:
        Embedding vector(s)
    """
    return _get_client().embed(text, model=model)


def list_models() -> list[dict]:
    """
    List all available models.
    
    Returns:
        List of model dictionaries with id, size, etc.
    """
    return _get_client().list_models()


def download(repo: str, filename: str, **kwargs) -> bool:
    """
    Download a model from HuggingFace.
    
    Args:
        repo: HuggingFace repository (e.g., "bartowski/Llama-3.2-3B-Instruct-GGUF")
        filename: GGUF file to download
    
    Returns:
        True if successful
    """
    return _get_client().models.download(repo, filename, **kwargs)


def search(query: str, ram: int = None, limit: int = 10) -> list[dict]:
    """
    Search for models on HuggingFace.
    
    Args:
        query: Search query
        ram: Filter by RAM requirement (GB)
        limit: Maximum results
    
    Returns:
        List of matching models
    """
    return _get_client().models.search(query, ram=ram, limit=limit)


def info() -> dict:
    """
    Get system and server information.
    
    Returns:
        Dictionary with system stats, loaded models, etc.
    """
    return _get_client().info()


# Expose the Knowledge Base via the default client
@property
def kb():
    """Access the Knowledge Base."""
    return _get_client().kb


__all__ = [
    "Client",
    "OffGridError",
    "ModelManager",
    "KnowledgeBase",
    "chat",
    "complete",
    "embed",
    "list_models",
    "download",
    "search",
    "info",
    "__version__",
]
