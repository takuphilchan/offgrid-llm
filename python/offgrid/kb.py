"""
Knowledge Base for OffGrid LLM.

Manages document ingestion and RAG (Retrieval-Augmented Generation).
"""

import os
import base64
from typing import Dict, List, Optional


class KnowledgeBase:
    """
    Knowledge Base manager for RAG.
    
    Accessed via client.kb:
        >>> client = Client()
        >>> client.kb.add("document.txt")
        >>> client.kb.search("What is the main topic?")
    """
    
    def __init__(self, client):
        self._client = client
    
    def status(self) -> dict:
        """
        Get Knowledge Base status.
        
        Returns:
            Dictionary with enabled status, document count, etc.
        """
        return self._client._request("GET", "/v1/rag/status")
    
    def enable(self) -> bool:
        """
        Enable the Knowledge Base.
        
        Returns:
            True if successful
        """
        response = self._client._request("POST", "/v1/rag/enable")
        return response.get("enabled", False)
    
    def disable(self) -> bool:
        """
        Disable the Knowledge Base.
        
        Returns:
            True if successful
        """
        response = self._client._request("POST", "/v1/rag/disable")
        return not response.get("enabled", True)
    
    @property
    def enabled(self) -> bool:
        """Check if Knowledge Base is enabled."""
        return self.status().get("enabled", False)
    
    def add(
        self,
        source: str,
        name: str = None,
        content: str = None
    ) -> dict:
        """
        Add a document to the Knowledge Base.
        
        Args:
            source: File path or URL to ingest
            name: Optional custom name for the document
            content: Direct text content (if source is just a name)
        
        Returns:
            Document info with id, chunks, etc.
        
        Example:
            >>> client.kb.add("notes.md")
            {'id': 'notes.md', 'chunks': 5, 'status': 'indexed'}
            
            >>> client.kb.add("meeting", content="Meeting notes: ...")
        """
        # Auto-enable if disabled
        if not self.enabled:
            self.enable()
        
        payload = {}
        
        if content:
            # Direct content
            payload["name"] = name or source
            payload["content"] = content
        elif os.path.exists(source):
            # Read file
            with open(source, "r", encoding="utf-8") as f:
                file_content = f.read()
            
            payload["name"] = name or os.path.basename(source)
            payload["content"] = file_content
        else:
            # Treat as URL or name
            payload["url"] = source
            if name:
                payload["name"] = name
        
        return self._client._request("POST", "/v1/documents/ingest", payload)
    
    def list(self) -> List[Dict]:
        """
        List all documents in the Knowledge Base.
        
        Returns:
            List of document dictionaries
        
        Example:
            >>> for doc in client.kb.list():
            ...     print(f"{doc['id']}: {doc['chunks']} chunks")
        """
        response = self._client._request("GET", "/v1/documents")
        return response.get("documents", [])
    
    def remove(self, doc_id: str) -> bool:
        """
        Remove a document from the Knowledge Base.
        
        Args:
            doc_id: Document ID to remove
        
        Returns:
            True if successful
        """
        response = self._client._request(
            "DELETE", 
            f"/v1/documents/delete?id={doc_id}"
        )
        return response.get("success", True)
    
    def search(
        self,
        query: str,
        top_k: int = 5,
        threshold: float = None,
        distributed: bool = False
    ) -> List[Dict]:
        """
        Search the Knowledge Base.
        
        Args:
            query: Search query
            top_k: Number of results to return
            threshold: Minimum similarity score (0.0 to 1.0)
            distributed: Search across P2P network peers (default: False)
        
        Returns:
            List of search results with content and scores
        
        Example:
            >>> results = client.kb.search("project deadline")
            >>> for r in results:
            ...     print(f"[{r['score']:.2f}] {r['content'][:100]}...")
            
            >>> # Search across all connected peers
            >>> results = client.kb.search("API documentation", distributed=True)
            >>> for r in results:
            ...     print(f"[{r['peer']}] {r['content'][:100]}...")
        """
        payload = {
            "query": query,
            "top_k": top_k
        }
        
        if threshold is not None:
            payload["threshold"] = threshold
        
        if distributed:
            payload["distributed"] = True
        
        response = self._client._request("POST", "/v1/documents/search", payload)
        results = response.get("results", [])
        
        # Flatten results for easier use
        flattened = []
        for r in results:
            chunk = r.get("chunk", {})
            result = {
                "content": chunk.get("content", ""),
                "document_id": r.get("document_id", ""),
                "document_name": r.get("document_name", ""),
                "score": r.get("score", 0),
                "chunk_index": chunk.get("index", 0)
            }
            
            # Include peer info for distributed search
            if "peer_id" in r:
                result["peer_id"] = r["peer_id"]
                result["peer_hostname"] = r.get("peer_hostname", "")
            
            flattened.append(result)
        
        return flattened
    
    def clear(self) -> int:
        """
        Remove all documents from the Knowledge Base.
        
        Returns:
            Number of documents removed
        """
        docs = self.list()
        count = 0
        
        for doc in docs:
            doc_id = doc.get("id", "")
            if doc_id:
                self.remove(doc_id)
                count += 1
        
        return count
    
    def add_directory(
        self,
        path: str,
        extensions: List[str] = None,
        recursive: bool = True
    ) -> List[Dict]:
        """
        Add all documents from a directory.
        
        Args:
            path: Directory path
            extensions: File extensions to include (default: .txt, .md, .json, .csv)
            recursive: Whether to include subdirectories
        
        Returns:
            List of added document info
        
        Example:
            >>> client.kb.add_directory("./docs", extensions=[".md", ".txt"])
        """
        if extensions is None:
            extensions = [".txt", ".md", ".json", ".csv", ".html"]
        
        results = []
        
        if recursive:
            for root, dirs, files in os.walk(path):
                for file in files:
                    if any(file.endswith(ext) for ext in extensions):
                        filepath = os.path.join(root, file)
                        try:
                            result = self.add(filepath)
                            results.append(result)
                        except Exception as e:
                            results.append({"file": filepath, "error": str(e)})
        else:
            for file in os.listdir(path):
                if any(file.endswith(ext) for ext in extensions):
                    filepath = os.path.join(path, file)
                    if os.path.isfile(filepath):
                        try:
                            result = self.add(filepath)
                            results.append(result)
                        except Exception as e:
                            results.append({"file": filepath, "error": str(e)})
        
        return results
