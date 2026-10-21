"""
OffGrid LoRA - LoRA Adapter Management

Register and manage LoRA (Low-Rank Adaptation) adapters for fine-tuned models.

Example:
    >>> from offgrid import Client
    >>> client = Client()
    >>> 
    >>> # List registered adapters
    >>> adapters = client.lora.list()
    >>> 
    >>> # Register a new adapter
    >>> client.lora.register("my-adapter", "/path/to/adapter.gguf")
    >>> 
    >>> # Use adapter in chat
    >>> client.chat("Hello!", lora="my-adapter")
"""

from typing import Dict, List, Optional, TYPE_CHECKING

if TYPE_CHECKING:
    from .client import Client


class LoRA:
    """
    LoRA adapter manager.
    
    Manage LoRA adapters for fine-tuned model capabilities.
    Adapters must be in GGUF format.
    
    Example:
        >>> lora = client.lora
        >>> lora.register("coding", "/models/code-adapter.gguf")
        >>> adapters = lora.list()
    """
    
    def __init__(self, client: "Client"):
        self._client = client
    
    def list(self) -> List[Dict]:
        """
        List all registered LoRA adapters.
        
        Returns:
            List of adapter dictionaries with name, path, scale, etc.
        
        Example:
            >>> adapters = lora.list()
            >>> for a in adapters:
            ...     print(f"{a['name']}: {a['path']}")
        """
        response = self._client._request("GET", "/v1/lora")
        return response.get("adapters", [])
    
    def register(
        self,
        name: str,
        path: str,
        scale: float = 1.0,
        **kwargs
    ) -> Dict:
        """
        Register a new LoRA adapter.
        
        Args:
            name: Unique name for the adapter
            path: Path to the GGUF adapter file
            scale: Adapter scale factor (default: 1.0)
            **kwargs: Additional parameters
        
        Returns:
            Registered adapter configuration
        
        Example:
            >>> lora.register("coding-assistant", "/models/code-lora.gguf", scale=0.8)
        """
        payload = {
            "name": name,
            "path": path,
            "scale": scale,
            **kwargs
        }
        return self._client._request("POST", "/v1/lora", payload)
    
    def remove(self, name: str) -> Dict:
        """
        Remove a registered LoRA adapter.
        
        Args:
            name: Adapter name to remove
        
        Returns:
            Deletion confirmation
        """
        return self._client._request("DELETE", f"/v1/lora/{name}")
    
    def get(self, name: str) -> Dict:
        """
        Get details of a specific adapter.
        
        Args:
            name: Adapter name
        
        Returns:
            Adapter configuration and status
        """
        return self._client._request("GET", f"/v1/lora/{name}")
