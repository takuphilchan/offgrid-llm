"""
Loading Progress Tracker for OffGrid LLM.

Track model loading progress with real-time updates.
"""

import json
import time
from typing import Callable, Dict, Iterator, Optional
from urllib.request import urlopen, Request
from urllib.error import URLError


class LoadingProgress:
    """
    Track model loading progress.
    
    Accessed via client.loading:
        >>> client = Client()
        >>> progress = client.loading.progress()
        >>> print(f"Loading {progress['model_id']}: {progress['progress']}%")
        
        >>> # Stream progress updates
        >>> for update in client.loading.stream():
        ...     print(f"{update['phase']}: {update['progress']}%")
        ...     if update['phase'] == 'ready':
        ...         break
    """
    
    def __init__(self, client):
        self._client = client
    
    def progress(self) -> Dict:
        """
        Get current loading progress snapshot.
        
        Returns:
            Dict with keys:
                - model_id: Model being loaded
                - phase: Current phase (idle, unloading, starting, loading, warmup, ready, failed)
                - progress: 0-100 percentage
                - message: Human-readable status
                - elapsed_ms: Time elapsed in milliseconds
                - estimated_ms: Estimated total time
                - is_warm: Whether model was in page cache
                - size_mb: Model size in MB
        
        Example:
            >>> progress = client.loading.progress()
            >>> if progress['phase'] == 'loading':
            ...     print(f"Loading: {progress['progress']}%")
        """
        return self._client._request("GET", "/v1/loading/progress")
    
    def stream(self, timeout: float = 120.0) -> Iterator[Dict]:
        """
        Stream loading progress updates via Server-Sent Events.
        
        Args:
            timeout: Maximum time to wait for loading to complete (default: 120s)
        
        Yields:
            Dict with progress updates (same format as progress())
        
        Example:
            >>> for update in client.loading.stream():
            ...     print(f"{update['phase']}: {update['progress']}%")
            ...     if update['phase'] in ('ready', 'failed'):
            ...         break
        """
        url = f"{self._client._base_url}/v1/loading/progress/stream"
        
        headers = {"Accept": "text/event-stream"}
        if self._client._api_key:
            headers["Authorization"] = f"Bearer {self._client._api_key}"
        
        request = Request(url, headers=headers)
        
        try:
            start_time = time.time()
            with urlopen(request, timeout=timeout) as response:
                buffer = ""
                while True:
                    # Check timeout
                    if time.time() - start_time > timeout:
                        break
                    
                    chunk = response.read(1024).decode('utf-8')
                    if not chunk:
                        break
                    
                    buffer += chunk
                    
                    # Parse SSE events
                    while "\n\n" in buffer:
                        event, buffer = buffer.split("\n\n", 1)
                        for line in event.split("\n"):
                            if line.startswith("data: "):
                                data = line[6:]
                                try:
                                    update = json.loads(data)
                                    yield update
                                    
                                    # Stop if loading complete
                                    if update.get("phase") in ("ready", "failed"):
                                        return
                                except json.JSONDecodeError:
                                    pass
        except URLError:
            pass
    
    def prewarm(self, model_path: str) -> Dict:
        """
        Pre-warm a model into the OS page cache.
        
        This uses aggressive concurrent I/O to load model weights
        into memory before they're needed, making model switches
        nearly instant.
        
        Args:
            model_path: Full path to the model file
        
        Returns:
            Dict with status
        
        Example:
            >>> client.loading.prewarm("/var/lib/offgrid/models/llama3.gguf")
            {'status': 'warming', 'message': 'Pre-warming initiated'}
        """
        return self._client._request("POST", "/v1/loading/prewarm", {
            "model_path": model_path
        })
    
    def wait_for_ready(
        self,
        timeout: float = 120.0,
        progress_callback: Callable[[Dict], None] = None
    ) -> bool:
        """
        Wait for the current model load to complete.
        
        Args:
            timeout: Maximum time to wait (default: 120s)
            progress_callback: Optional callback for progress updates
        
        Returns:
            True if model loaded successfully, False if failed or timeout
        
        Example:
            >>> def on_progress(p):
            ...     print(f"\\r{p['message']} ({p['progress']}%)", end="")
            >>> success = client.loading.wait_for_ready(progress_callback=on_progress)
        """
        start_time = time.time()
        
        while time.time() - start_time < timeout:
            progress = self.progress()
            
            if progress_callback:
                progress_callback(progress)
            
            phase = progress.get("phase", "idle")
            
            if phase == "ready":
                return True
            elif phase == "failed":
                return False
            elif phase == "idle":
                # No loading in progress
                return True
            
            time.sleep(0.5)
        
        return False  # Timeout
