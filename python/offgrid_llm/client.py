"""
OffGrid LLM Client

Main client class for interacting with the OffGrid LLM server.
"""

import json
from typing import Iterator, Optional, Union
from urllib.request import urlopen, Request
from urllib.error import URLError, HTTPError
from urllib.parse import urlencode

from .models import ModelManager
from .kb import KnowledgeBase


class OffGridError(Exception):
    """Base exception for OffGrid errors."""
    
    def __init__(self, message: str, code: str = None, details: str = None):
        self.message = message
        self.code = code
        self.details = details
        super().__init__(message)


class Client:
    """
    OffGrid LLM Client.
    
    Connects to a local OffGrid server for AI inference.
    
    Args:
        host: Server host (default: localhost)
        port: Server port (default: 11611)
        timeout: Request timeout in seconds (default: 300)
    
    Example:
        >>> client = Client()
        >>> client.chat("Hello!")
        'Hello! How can I help you today?'
        
        >>> client = Client(port=8080)  # Custom port
    """
    
    def __init__(
        self,
        host: str = "localhost",
        port: int = 11611,
        timeout: int = 300
    ):
        self.host = host
        self.port = port
        self.timeout = timeout
        self.base_url = f"http://{host}:{port}"
        
        # Initialize sub-managers
        self.models = ModelManager(self)
        self.kb = KnowledgeBase(self)
        
        # Cache for default model
        self._default_model = None
    
    def _request(
        self,
        method: str,
        endpoint: str,
        data: dict = None,
        stream: bool = False
    ) -> Union[dict, Iterator[dict]]:
        """Make an HTTP request to the server."""
        url = f"{self.base_url}{endpoint}"
        
        headers = {"Content-Type": "application/json"}
        body = json.dumps(data).encode() if data else None
        
        req = Request(url, data=body, headers=headers, method=method)
        
        try:
            response = urlopen(req, timeout=self.timeout)
            
            if stream:
                return self._stream_response(response)
            
            content = response.read().decode()
            return json.loads(content) if content else {}
            
        except HTTPError as e:
            error_body = e.read().decode()
            try:
                error_data = json.loads(error_body)
                error_msg = error_data.get("error", {})
                if isinstance(error_msg, dict):
                    raise OffGridError(
                        error_msg.get("message", str(e)),
                        code=error_msg.get("code"),
                        details=error_msg.get("details")
                    )
                raise OffGridError(str(error_msg))
            except json.JSONDecodeError:
                raise OffGridError(f"HTTP {e.code}: {error_body}")
        except URLError as e:
            raise OffGridError(
                f"Cannot connect to OffGrid server at {self.base_url}. "
                f"Make sure the server is running with 'offgrid serve'. "
                f"Error: {e.reason}"
            )
    
    def _stream_response(self, response) -> Iterator[dict]:
        """Stream Server-Sent Events response."""
        for line in response:
            line = line.decode().strip()
            if line.startswith("data: "):
                data = line[6:]
                if data == "[DONE]":
                    break
                try:
                    yield json.loads(data)
                except json.JSONDecodeError:
                    continue
    
    def _get_default_model(self) -> str:
        """Get the first available model as default."""
        if self._default_model is None:
            models = self.list_models()
            if not models:
                raise OffGridError(
                    "No models available. Download one with: "
                    "offgrid download-hf bartowski/Llama-3.2-3B-Instruct-GGUF "
                    "--file Llama-3.2-3B-Instruct-Q4_K_M.gguf"
                )
            self._default_model = models[0]["id"]
        return self._default_model
    
    def chat(
        self,
        message: str,
        model: str = None,
        system: str = None,
        messages: list[dict] = None,
        use_kb: bool = False,
        stream: bool = False,
        temperature: float = None,
        max_tokens: int = None,
        **kwargs
    ) -> Union[str, Iterator[str]]:
        """
        Send a chat message and get a response.
        
        Args:
            message: The user message (ignored if messages is provided)
            model: Model name (uses first available if not specified)
            system: Optional system prompt
            messages: Full conversation history (overrides message/system)
            use_kb: Whether to use Knowledge Base for context
            stream: Whether to stream the response
            temperature: Sampling temperature (0.0 to 2.0)
            max_tokens: Maximum tokens to generate
            **kwargs: Additional parameters
        
        Returns:
            The assistant's response text, or an iterator if streaming
        
        Example:
            >>> client.chat("What is Python?")
            'Python is a high-level programming language...'
            
            >>> for chunk in client.chat("Tell me a story", stream=True):
            ...     print(chunk, end="", flush=True)
        """
        if model is None:
            model = self._get_default_model()
        
        # Build messages array
        if messages is None:
            messages = []
            if system:
                messages.append({"role": "system", "content": system})
            messages.append({"role": "user", "content": message})
        
        payload = {
            "model": model,
            "messages": messages,
            "stream": stream,
            **kwargs
        }
        
        if temperature is not None:
            payload["temperature"] = temperature
        if max_tokens is not None:
            payload["max_tokens"] = max_tokens
        if use_kb:
            payload["use_knowledge_base"] = True
        
        if stream:
            return self._stream_chat(payload)
        
        response = self._request("POST", "/v1/chat/completions", payload)
        
        if "choices" in response and len(response["choices"]) > 0:
            return response["choices"][0]["message"]["content"]
        
        raise OffGridError("Invalid response from server", details=str(response))
    
    def _stream_chat(self, payload: dict) -> Iterator[str]:
        """Stream chat response tokens."""
        url = f"{self.base_url}/v1/chat/completions"
        headers = {"Content-Type": "application/json"}
        body = json.dumps(payload).encode()
        
        req = Request(url, data=body, headers=headers, method="POST")
        
        try:
            response = urlopen(req, timeout=self.timeout)
            
            for line in response:
                line = line.decode().strip()
                if line.startswith("data: "):
                    data = line[6:]
                    if data == "[DONE]":
                        break
                    try:
                        chunk = json.loads(data)
                        if "choices" in chunk and len(chunk["choices"]) > 0:
                            delta = chunk["choices"][0].get("delta", {})
                            content = delta.get("content", "")
                            if content:
                                yield content
                    except json.JSONDecodeError:
                        continue
                        
        except HTTPError as e:
            raise OffGridError(f"Streaming error: HTTP {e.code}")
        except URLError as e:
            raise OffGridError(f"Connection error: {e.reason}")
    
    def complete(
        self,
        prompt: str,
        model: str = None,
        max_tokens: int = None,
        temperature: float = None,
        **kwargs
    ) -> str:
        """
        Generate a text completion.
        
        Args:
            prompt: The prompt to complete
            model: Model name (uses first available if not specified)
            max_tokens: Maximum tokens to generate
            temperature: Sampling temperature
            **kwargs: Additional parameters
        
        Returns:
            The completion text
        """
        if model is None:
            model = self._get_default_model()
        
        payload = {
            "model": model,
            "prompt": prompt,
            **kwargs
        }
        
        if max_tokens is not None:
            payload["max_tokens"] = max_tokens
        if temperature is not None:
            payload["temperature"] = temperature
        
        response = self._request("POST", "/v1/completions", payload)
        
        if "choices" in response and len(response["choices"]) > 0:
            return response["choices"][0]["text"]
        
        raise OffGridError("Invalid response from server", details=str(response))
    
    def embed(
        self,
        text: Union[str, list[str]],
        model: str = None
    ) -> Union[list[float], list[list[float]]]:
        """
        Generate embeddings for text.
        
        Args:
            text: Single string or list of strings
            model: Embedding model name
        
        Returns:
            Embedding vector(s)
        """
        if model is None:
            model = self._get_default_model()
        
        # Normalize input to list
        input_list = [text] if isinstance(text, str) else text
        
        payload = {
            "model": model,
            "input": input_list
        }
        
        response = self._request("POST", "/v1/embeddings", payload)
        
        if "data" in response:
            embeddings = [item["embedding"] for item in response["data"]]
            # Return single vector if single input
            return embeddings[0] if isinstance(text, str) else embeddings
        
        raise OffGridError("Invalid response from server", details=str(response))
    
    def list_models(self) -> list[dict]:
        """
        List all available models.
        
        Returns:
            List of model dictionaries with id, size, quantization, etc.
        
        Example:
            >>> client.list_models()
            [{'id': 'Llama-3.2-3B-Instruct-Q4_K_M', 'size': 2147483648, ...}]
        """
        response = self._request("GET", "/v1/models")
        return response.get("data", [])
    
    def info(self) -> dict:
        """
        Get system and server information.
        
        Returns:
            Dictionary with system stats, uptime, loaded models, etc.
        """
        return self._request("GET", "/health")
    
    def health(self) -> bool:
        """
        Check if the server is healthy.
        
        Returns:
            True if server is responding
        """
        try:
            response = self._request("GET", "/health")
            return response.get("status") == "healthy"
        except OffGridError:
            return False
    
    def refresh_models(self) -> list[dict]:
        """
        Refresh the model list (rescan models directory).
        
        Returns:
            Updated list of models
        """
        response = self._request("POST", "/models/refresh")
        return response.get("data", [])
