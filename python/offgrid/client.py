"""
OffGrid LLM Client

Main client class for interacting with the OffGrid LLM server.
"""

import json
import time
from typing import Dict, Iterator, List, Optional, Union
from urllib.request import urlopen, Request
from urllib.error import URLError, HTTPError
from urllib.parse import urlencode, urlparse

from .models import ModelManager
from .kb import KnowledgeBase
from .agent import Agent
from .lora import LoRA
from .audio import Audio
from .loading import LoadingProgress
from .p2p import P2P


class OffGridError(Exception):
    """Base exception for OffGrid errors."""
    
    def __init__(self, message: str, code: str = None, details: str = None):
        self.message = message
        self.code = code
        self.details = details
        super().__init__(message)


class Sessions:
    """
    Manage conversation sessions.
    
    Sessions allow you to persist conversation context on the server,
    enabling multi-turn conversations across API calls.
    
    Example:
        >>> sessions = client.sessions
        >>> sessions.create("my-chat")
        >>> response = sessions.chat_with_session("my-chat", "Hello!")
        >>> response = sessions.chat_with_session("my-chat", "What did I say?")
    """
    
    def __init__(self, client: "Client"):
        self._client = client
    
    def list(self) -> List[Dict]:
        """
        List all saved sessions.
        
        Returns:
            List of session objects with name, messages, model_id, etc.
        """
        response = self._client._request("GET", "/v1/sessions")
        return response.get("sessions", [])
    
    def get(self, name: str) -> Dict:
        """
        Get a specific session by name.
        
        Args:
            name: Session name
            
        Returns:
            Session object with messages, model_id, timestamps, etc.
        """
        return self._client._request("GET", f"/v1/sessions/{name}")
    
    def create(self, name: str, model_id: str = "") -> Dict:
        """
        Create a new session.
        
        Args:
            name: Session name (must be unique)
            model_id: Optional model ID to associate with the session
            
        Returns:
            The created session object
        """
        payload = {"name": name}
        if model_id:
            payload["model_id"] = model_id
        return self._client._request("POST", "/v1/sessions", payload)
    
    def delete(self, name: str) -> bool:
        """
        Delete a session.
        
        Args:
            name: Session name to delete
            
        Returns:
            True if deleted successfully
        """
        response = self._client._request("DELETE", f"/v1/sessions/{name}")
        return response.get("success", True)
    
    def add_message(self, name: str, role: str, content: str) -> bool:
        """
        Add a message to an existing session.
        
        Args:
            name: Session name
            role: Message role ("user", "assistant", or "system")
            content: Message content
            
        Returns:
            True if added successfully
        """
        payload = {"role": role, "content": content}
        response = self._client._request("POST", f"/v1/sessions/{name}/messages", payload)
        return response.get("success", True)
    
    def chat_with_session(
        self,
        name: str,
        message: str,
        model: str = None,
        temperature: float = None,
        max_tokens: int = None,
        **kwargs
    ) -> str:
        """
        Chat within a session context.
        
        The message is added to the session, the full conversation history
        is sent to the model, and the response is also saved to the session.
        
        Args:
            name: Session name
            message: User message
            model: Model to use (optional)
            temperature: Sampling temperature (optional)
            max_tokens: Maximum tokens to generate (optional)
            
        Returns:
            The assistant's response
        """
        # Get existing session
        session = self.get(name)
        messages = session.get("messages", [])
        
        # Add user message
        messages.append({"role": "user", "content": message})
        
        # Send to chat
        if model is None:
            model = session.get("model_id") or self._client._get_default_model()
        
        payload = {
            "model": model,
            "messages": messages,
            "stream": False,
            **kwargs
        }
        
        if temperature is not None:
            payload["temperature"] = temperature
        if max_tokens is not None:
            payload["max_tokens"] = max_tokens
        
        response = self._client._request("POST", "/v1/chat/completions", payload)
        
        if "choices" in response and len(response["choices"]) > 0:
            assistant_message = response["choices"][0]["message"]["content"]
            
            # Save messages to session
            self.add_message(name, "user", message)
            self.add_message(name, "assistant", assistant_message)
            
            return assistant_message
        
        raise OffGridError("Invalid response from server", details=str(response))


class Client:
    """
    OffGrid LLM Client.
    
    Connects to an OffGrid server for AI inference.
    
    Args:
        host: Server URL (default: http://localhost:11611)
        timeout: Request timeout in seconds (default: 300)
        api_key: Optional API key for authentication
        max_retries: Maximum number of retries for failed requests (default: 3)
        retry_delay: Delay between retries in seconds (default: 1.0)
        keep_alive: Use HTTP connection pooling for better performance (default: True)
    
    Example:
        >>> client = Client()  # localhost:11611
        >>> client.chat("Hello!")
        
        >>> client = Client(host="http://192.168.1.100:11611")
        >>> client.chat("Hello!")
        
        >>> client = Client(api_key="your-api-key")
        >>> client.chat("Hello!")
    """
    
    def __init__(
        self,
        host: str = "http://localhost:11611",
        timeout: int = 300,
        api_key: str = None,
        max_retries: int = 3,
        retry_delay: float = 1.0,
        keep_alive: bool = True
    ):
        # Normalize the host URL
        if not host.startswith("http://") and not host.startswith("https://"):
            host = f"http://{host}"
        
        self.host = host.rstrip("/")
        self.timeout = timeout
        self.base_url = self.host
        self.api_key = api_key
        self.max_retries = max_retries
        self.retry_delay = retry_delay
        self.keep_alive = keep_alive
        
        # Connection pooling for better performance
        self._http_handler = None
        if keep_alive:
            try:
                import urllib.request
                self._http_handler = urllib.request.HTTPHandler()
                self._opener = urllib.request.build_opener(self._http_handler)
            except Exception:
                self._opener = None
        else:
            self._opener = None
        
        # Initialize sub-managers
        self.models = ModelManager(self)
        self.kb = KnowledgeBase(self)
        self.sessions = Sessions(self)
        self.agent = Agent(self)
        self.lora = LoRA(self)
        self.audio = Audio(self)
        self.loading = LoadingProgress(self)
        self.p2p = P2P(self)
        
        # Cache for default model
        self._default_model = None
    
    def _request(
        self,
        method: str,
        endpoint: str,
        data: dict = None,
        stream: bool = False,
        retry: bool = True
    ) -> Union[dict, Iterator[dict]]:
        """Make an HTTP request to the server with retry logic and connection pooling."""
        url = f"{self.base_url}{endpoint}"
        
        headers = {
            "Content-Type": "application/json",
            "Connection": "keep-alive" if self.keep_alive else "close"
        }
        if self.api_key:
            headers["Authorization"] = f"Bearer {self.api_key}"
        
        body = json.dumps(data).encode() if data else None
        
        last_error = None
        retries = self.max_retries if retry else 1
        
        for attempt in range(retries):
            try:
                req = Request(url, data=body, headers=headers, method=method)
                
                # Use connection pooling opener if available
                if self._opener:
                    response = self._opener.open(req, timeout=self.timeout)
                else:
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
                        last_error = OffGridError(
                            error_msg.get("message", str(e)),
                            code=error_msg.get("code"),
                            details=error_msg.get("details")
                        )
                    else:
                        last_error = OffGridError(str(error_msg))
                except json.JSONDecodeError:
                    last_error = OffGridError(f"HTTP {e.code}: {error_body}")
                
                # Don't retry client errors (4xx)
                if 400 <= e.code < 500:
                    raise last_error
                    
            except URLError as e:
                last_error = OffGridError(
                    f"Cannot connect to OffGrid server at {self.base_url}. "
                    f"Make sure the server is running with 'offgrid serve'. "
                    f"Error: {e.reason}"
                )
            
            # Wait before retry
            if attempt < retries - 1:
                time.sleep(self.retry_delay * (attempt + 1))
        
        raise last_error
    
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
        messages: List[Dict] = None,
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
        text: Union[str, List[str]],
        model: str = None
    ) -> Union[List[float], List[List[float]]]:
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
    
    def list_models(self) -> List[Dict]:
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
            response = self._request("GET", "/health", retry=False)
            return response.get("status") == "healthy"
        except OffGridError:
            return False
    
    def stats(self) -> Dict:
        """
        Get comprehensive server statistics.
        
        Returns:
            Dictionary with server stats, inference metrics, system info,
            cache stats, and RAG status.
        
        Example:
            >>> stats = client.stats()
            >>> print(f"Uptime: {stats['server']['uptime']}")
            >>> print(f"Total requests: {stats['inference']['aggregate']['total_requests']}")
        """
        return self._request("GET", "/v1/stats")
    
    def config(self) -> Dict:
        """
        Get system configuration and feature flags.
        
        Returns:
            Dictionary with:
                - multi_user_mode: Whether multi-user mode is enabled
                - require_auth: Whether authentication is required
                - guest_access: Whether guest access is allowed
                - version: Server version
                - features: Dict of enabled features (users, metrics, agent, lora)
        
        Example:
            >>> config = client.config()
            >>> print(f"Version: {config['version']}")
            >>> print(f"Agent enabled: {config['features']['agent']}")
        """
        return self._request("GET", "/v1/system/config")
    
    def system_stats(self) -> Dict:
        """
        Get real-time system statistics.
        
        Returns:
            Dictionary with CPU, memory, disk usage, and uptime
        
        Example:
            >>> stats = client.system_stats()
            >>> print(f"CPU: {stats['cpu_percent']}%")
            >>> print(f"Memory: {stats['memory_percent']}%")
        """
        return self._request("GET", "/v1/system/stats")
    
    def refresh_models(self) -> List[Dict]:
        """
        Refresh the model list (rescan models directory).
        
        Returns:
            Updated list of models
        """
        response = self._request("POST", "/models/refresh")
        return response.get("data", [])
    
    def cache_stats(self) -> Dict:
        """
        Get model cache statistics.
        
        Returns statistics about loaded models, memory usage, and pre-warming status.
        Useful for understanding model switching performance.
        
        Returns:
            Dictionary with:
                - loaded_models: List of currently loaded models
                - cache_size: Number of models in cache
                - max_size: Maximum cache capacity
                - mmap_warmer: Pre-warming statistics
                - system_ram_mb: Available system RAM
                - mlock_enabled: Whether mlock is enabled
        
        Example:
            >>> stats = client.cache_stats()
            >>> print(f"Models in cache: {stats['cache_size']}/{stats['max_size']}")
            >>> for model in stats['loaded_models']:
            ...     print(f"  - {model['id']}: {model['size_mb']}MB")
        """
        return self._request("GET", "/v1/cache/stats")
    
    def warm_model(self, model: str, wait: bool = True, timeout: int = 60) -> bool:
        """
        Pre-warm a model into cache for faster first response.
        
        Sends a minimal request to trigger model loading, ensuring the model
        is ready for instant responses. This is useful before starting a
        conversation or when anticipating model use.
        
        Args:
            model: Model name to warm
            wait: If True, wait for warming to complete (default: True)
            timeout: Maximum seconds to wait for warming (default: 60)
        
        Returns:
            True if model is warmed and ready
        
        Example:
            >>> # Pre-warm before user interaction
            >>> client.warm_model("Llama-3.2-3B-Instruct-Q4_K_M")
            >>> # Now chat will have instant first response
            >>> response = client.chat("Hello!")
        """
        import time
        
        start = time.time()
        try:
            # Send minimal request to trigger model loading
            response = self._request("POST", "/v1/chat/completions", {
                "model": model,
                "messages": [{"role": "user", "content": "hi"}],
                "stream": False,
                "max_tokens": 1,
                "temperature": 0
            }, retry=False)
            
            if wait:
                # Check if model is actually loaded
                while time.time() - start < timeout:
                    stats = self.cache_stats()
                    loaded = [m.get("id", "") for m in stats.get("loaded_models", [])]
                    if model in loaded or any(model in m for m in loaded):
                        return True
                    time.sleep(0.5)
            
            return "choices" in response
        except Exception as e:
            # Model may still be loading
            if wait and time.time() - start < timeout:
                time.sleep(2)
                return self.warm_model(model, wait=True, timeout=timeout - int(time.time() - start))
            return False
    
    def is_model_cached(self, model: str) -> bool:
        """
        Check if a model is currently loaded in cache.
        
        Cached models respond instantly without loading delay.
        
        Args:
            model: Model name to check
        
        Returns:
            True if model is in cache and ready
        
        Example:
            >>> if client.is_model_cached("Llama-3.2-3B-Instruct"):
            ...     print("Model ready - instant response!")
            ... else:
            ...     print("Model will need to load first")
            ...     client.warm_model("Llama-3.2-3B-Instruct")
        """
        try:
            stats = self.cache_stats()
            loaded = [m.get("id", "") for m in stats.get("loaded_models", [])]
            return model in loaded or any(model in m for m in loaded)
        except Exception:
            return False
