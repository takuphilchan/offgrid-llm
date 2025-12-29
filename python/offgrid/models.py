"""
Model Management for OffGrid LLM.

Handles model downloading, searching, and USB import/export.
"""

import json
import time
from typing import Callable, Dict, List, Optional
from urllib.request import urlopen, Request
from urllib.error import URLError, HTTPError


class ModelManager:
    """
    Manages models for OffGrid LLM.
    
    Accessed via client.models:
        >>> client = Client()
        >>> client.models.download("repo", "file.gguf")
        >>> client.models.search("llama", ram=8)
    """
    
    def __init__(self, client):
        self._client = client
    
    def list(self) -> List[Dict]:
        """
        List all installed models.
        
        Returns:
            List of model dictionaries
        """
        return self._client.list_models()
    
    def refresh(self) -> List[Dict]:
        """
        Refresh the model list (rescan models directory).
        
        Returns:
            Updated list of models
        """
        return self._client.refresh_models()
    
    def download(
        self,
        repo: str,
        filename: str,
        progress_callback: Callable[[float, int, int], None] = None,
        wait: bool = True
    ) -> bool:
        """
        Download a model from HuggingFace.
        
        Args:
            repo: HuggingFace repository (e.g., "bartowski/Llama-3.2-3B-Instruct-GGUF")
            filename: GGUF file to download (e.g., "Llama-3.2-3B-Instruct-Q4_K_M.gguf")
            progress_callback: Optional callback(percent, bytes_done, bytes_total)
            wait: Wait for download to complete (default: True)
        
        Returns:
            True if successful
        
        Example:
            >>> def on_progress(pct, done, total):
            ...     print(f"\\rDownloading: {pct:.1f}%", end="")
            >>> client.models.download(
            ...     "bartowski/Llama-3.2-3B-Instruct-GGUF",
            ...     "Llama-3.2-3B-Instruct-Q4_K_M.gguf",
            ...     progress_callback=on_progress
            ... )
        """
        payload = {
            "repository": repo,
            "file_name": filename
        }
        
        response = self._client._request("POST", "/v1/models/download", payload)
        
        if not wait:
            return response.get("success", False)
        
        # Poll for progress
        while True:
            progress = self.download_progress()
            
            if filename in progress:
                status = progress[filename]
                
                if progress_callback:
                    progress_callback(
                        status.get("percent", 0),
                        status.get("bytes_done", 0),
                        status.get("bytes_total", 0)
                    )
                
                if status.get("status") == "complete":
                    return True
                elif status.get("status") == "failed":
                    from .client import OffGridError
                    raise OffGridError(
                        f"Download failed: {status.get('error', 'Unknown error')}"
                    )
            
            time.sleep(1)
    
    def download_progress(self) -> dict:
        """
        Get download progress for all active downloads.
        
        Returns:
            Dictionary of filename -> progress info
        """
        return self._client._request("GET", "/v1/models/download/progress")
    
    def search(
        self,
        query: str,
        ram: int = None,
        quantization: str = None,
        author: str = None,
        limit: int = 10
    ) -> List[Dict]:
        """
        Search for models on HuggingFace.
        
        Args:
            query: Search query (e.g., "llama", "mistral", "code")
            ram: Filter by RAM requirement in GB
            quantization: Filter by quantization (e.g., "Q4_K_M")
            author: Filter by author/organization
            limit: Maximum results to return
        
        Returns:
            List of matching models with download info
        
        Example:
            >>> results = client.models.search("llama", ram=8)
            >>> for model in results:
            ...     print(f"{model['id']} - {model['size_gb']}GB")
        """
        params = {
            "query": query,
            "limit": limit
        }
        
        if author:
            params["author"] = author
        if quantization:
            params["quantization"] = quantization
        
        # Build query string
        query_string = "&".join(f"{k}={v}" for k, v in params.items())
        
        response = self._client._request("GET", f"/v1/search?{query_string}")
        results = response.get("results", [])
        
        # Filter by RAM if specified
        if ram is not None:
            filtered = []
            for model in results:
                size_gb = float(model.get("size_gb", 0) or 0)
                # Rough estimate: need ~1.2x model size in RAM
                if size_gb * 1.2 <= ram:
                    filtered.append(model)
            results = filtered
        
        return results
    
    def delete(self, model_id: str) -> bool:
        """
        Delete an installed model.
        
        Args:
            model_id: The model ID to delete
        
        Returns:
            True if successful
        """
        payload = {"model_id": model_id}
        response = self._client._request("POST", "/v1/models/delete", payload)
        return response.get("success", False)
    
    def import_usb(
        self,
        path: str,
        progress_callback: Callable[[str, float], None] = None
    ) -> List[str]:
        """
        Import models from USB drive or directory.
        
        Args:
            path: Path to USB drive or directory containing .gguf files
            progress_callback: Optional callback(filename, percent)
        
        Returns:
            List of imported model filenames
        
        Example:
            >>> imported = client.models.import_usb("/media/usb")
            >>> print(f"Imported {len(imported)} models")
        """
        # First scan the path
        scan_response = self._client._request("POST", "/v1/usb/scan", {"usb_path": path})
        models = scan_response.get("models", [])
        
        if not models:
            return []
        
        # Import all found models
        import_response = self._client._request("POST", "/v1/usb/import", {"usb_path": path})
        
        # Refresh model list
        self.refresh()
        
        return [m["file_name"] for m in models]
    
    def export_usb(
        self,
        model_id: str,
        destination: str,
        progress_callback: Callable[[float, int, int], None] = None
    ) -> bool:
        """
        Export a model to USB drive or directory.
        
        Args:
            model_id: The model ID to export
            destination: Path to destination directory
            progress_callback: Optional callback(percent, bytes_done, bytes_total)
        
        Returns:
            True if successful
        
        Example:
            >>> client.models.export_usb("Llama-3.2-3B-Instruct-Q4_K_M", "/media/usb")
        """
        payload = {
            "model_id": model_id,
            "destination": destination
        }
        
        response = self._client._request("POST", "/v1/usb/export", payload)
        
        # Poll for progress if callback provided
        if progress_callback:
            while True:
                progress = self._client._request("GET", "/v1/usb/export/progress")
                
                for filename, status in progress.items():
                    if model_id in filename:
                        progress_callback(
                            status.get("percent", 0),
                            status.get("bytes_done", 0),
                            status.get("bytes_total", 0)
                        )
                        
                        if status.get("status") == "complete":
                            return True
                        elif status.get("status") == "failed":
                            from .client import OffGridError
                            raise OffGridError(
                                f"Export failed: {status.get('error', 'Unknown error')}"
                            )
                
                time.sleep(0.5)
        
        return response.get("success", False)
    
    def benchmark(
        self,
        model_id: str,
        prompt_tokens: int = 512,
        output_tokens: int = 128,
        iterations: int = 3
    ) -> dict:
        """
        Benchmark a model's performance.
        
        Args:
            model_id: The model to benchmark
            prompt_tokens: Number of prompt tokens
            output_tokens: Number of output tokens
            iterations: Number of test iterations
        
        Returns:
            Benchmark results with tokens/sec, latency, etc.
        """
        payload = {
            "model": model_id,
            "prompt_tokens": prompt_tokens,
            "output_tokens": output_tokens,
            "iterations": iterations
        }
        
        return self._client._request("POST", "/v1/benchmark", payload)
    
    def prewarm(self, model_id: str) -> dict:
        """
        Pre-warm a model into the OS page cache.
        
        Uses aggressive concurrent I/O to load model weights into
        memory before they're needed, making model switches nearly
        instant (can achieve 10GB/s+ read speeds).
        
        Args:
            model_id: The model ID to pre-warm
        
        Returns:
            Dict with warming status
        
        Example:
            >>> client.models.prewarm("llama3.2:3b")
            {'status': 'warming', 'model_id': 'llama3.2:3b', 'size_mb': 2048}
            
            >>> # Later, switch happens instantly
            >>> client.use("llama3.2:3b")  # Already in cache!
        """
        return self._client._request("POST", "/v1/models/prewarm", {
            "model_id": model_id
        })
    
    def get_loading_progress(self) -> dict:
        """
        Get current model loading progress.
        
        Returns:
            Dict with loading progress info:
                - model_id: Model being loaded
                - phase: idle, unloading, starting, loading, warmup, ready, failed
                - progress: 0-100 percentage
                - message: Human-readable status
                - elapsed_ms: Time elapsed
                - estimated_ms: Estimated total time
        
        Example:
            >>> progress = client.models.get_loading_progress()
            >>> if progress['phase'] == 'loading':
            ...     print(f"Loading {progress['model_id']}: {progress['progress']}%")
        """
        return self._client._request("GET", "/v1/loading/progress")
