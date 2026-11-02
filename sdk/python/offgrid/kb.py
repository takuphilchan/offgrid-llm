"""
Knowledge Base for OffGrid LLM.

Manages document ingestion and RAG (Retrieval-Augmented Generation).
"""

import os
from urllib.parse import urlencode, urlsplit
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

    def retention(self, doc_id: str) -> dict:
        """Read the current document retention revision without changing it."""
        return self._retention_response(self._client._request("GET", "/v1/documents/retention?" + urlencode({"id": doc_id})))

    @staticmethod
    def _retention_response(value: dict) -> dict:
        if not isinstance(value, dict) or type(value.get("version")) is not int or value["version"] != 1 or type(value.get("revision")) is not int or value["revision"] < 1 or type(value.get("delete_after")) is not int or not 0 <= value["delete_after"] <= 253402300799 or type(value.get("legal_hold")) is not bool:
            raise ValueError("Retention response is incomplete. Inspect the document before retrying.")
        return value

    def set_retention(self, doc_id: str, *, revision: int, delete_after: int, legal_hold: bool) -> dict:
        """Explicitly replace expiry/hold; stale revisions fail without retry."""
        if type(revision) is not int or revision < 1 or type(delete_after) is not int or not 0 <= delete_after <= 253402300799 or type(legal_hold) is not bool:
            raise ValueError("Provide the current revision, UTC Unix expiry (0 retains), and an explicit boolean hold")
        result = self._retention_response(self._client._request("PUT", "/v1/documents/retention?" + urlencode({"id": doc_id}),
                                     {"version": 1, "revision": revision, "delete_after": delete_after, "legal_hold": legal_hold}, retry=False))
        if result["revision"] != revision + 1 or result["delete_after"] != delete_after or result["legal_hold"] != legal_hold:
            raise ValueError("Retention update was not confirmed. Inspect the document before retrying.")
        return result
    
    def enable(self, embedding_model: Optional[str] = None) -> bool:
        """
        Enable the Knowledge Base.
        
        Returns:
            True if successful
        """
        payload = {"embedding_model": embedding_model} if embedding_model is not None else None
        response = self._client._request("POST", "/v1/rag/enable", payload, retry=False)
        return response.get("success", response.get("enabled", False)) is True
    
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
            {'success': True, 'document': {'id': '...', 'index_status': 'ready'}}
            
            >>> client.kb.add("meeting", content="Meeting notes: ...")
        """
        from .knowledge_files import upload
        if content is None and urlsplit(os.fspath(source)).scheme not in ("http", "https"):
            return upload(self._client, source, name)

        payload = {}
        
        if content is not None:
            # Direct content
            payload["name"] = name or source
            payload["content"] = content
        else:
            # Treat as URL or name
            payload["url"] = source
            if name:
                payload["name"] = name
        
        endpoint = "/v1/documents/ingest" if content is not None else "/v1/documents/ingest-url"
        return self._client._request("POST", endpoint, payload, retry=False)

    def original(self, doc_id: str) -> bytes:
        """Download digest-verified original bytes, when retained by the service."""
        from .knowledge_files import original
        return original(self._client, doc_id)

    def imports(self) -> List[Dict]:
        """Read the most recent 100 imports owned by this authenticated actor."""
        return self._client._request("GET", "/v1/documents/imports").get("imports", [])

    def import_attachment(self, session: str, attachment_id: str, sha256: str) -> Dict:
        """Explicitly copy a Chat source to a saved Knowledge import; no indexing.

        Requires source access and Knowledge management permission. Call
        retry_import with the returned ID to index; mutations are not retried.
        """
        import re
        if not session or not re.fullmatch(r"file-[a-f0-9]{32}", attachment_id) or not re.fullmatch(r"[a-f0-9]{64}", sha256):
            raise ValueError("A session, attachment ID and exact source SHA-256 are required")
        return self._client._request("POST", "/v1/documents/imports", {
            "attachment": {"session": session, "id": attachment_id, "sha256": sha256}
        }, retry=False)

    def ocr_languages(self) -> Dict:
        """Read language installation and runtime readiness separately."""
        return self._client._request("GET", "/v1/documents/ocr-languages")

    def import_ocr_language(self, language: str, path: str) -> Dict:
        """Import an explicitly selected local pack; never download or reindex."""
        from .knowledge_files import import_ocr_language
        return import_ocr_language(self._client, language, path)

    def import_status(self, import_id: str) -> Dict:
        return self._client._request("GET", "/v1/documents/imports?" + urlencode({"id": import_id}))

    def review_import(self, import_id: str) -> Dict:
        """Read owner-scoped partial pages without indexing or replaying work."""
        return self._client._request("GET", "/v1/documents/imports?" + urlencode({"id": import_id, "view": "extraction"}))

    def retry_import(self, import_id: str) -> Dict:
        """Explicitly process the retained source; never upload it again."""
        return self._client._request("POST", "/v1/documents/imports?" + urlencode({"id": import_id, "action": "retry"}), retry=False)

    def cancel_import(self, import_id: str) -> Dict:
        return self._client._request("POST", "/v1/documents/imports?" + urlencode({"id": import_id, "action": "cancel"}), retry=False)

    def remove_import(self, import_id: str) -> Dict:
        """Remove import history/pending bytes; a published document is separate."""
        return self._client._request("DELETE", "/v1/documents/imports?" + urlencode({"id": import_id}), retry=False)
    
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
            "/v1/documents/delete?" + urlencode({"id": doc_id}), retry=False
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
            # Preserve service locators rather than dropping evidence while
            # projecting the older flattened search shape.
            for key in ("source", "page", "section", "content_hash", "truncated"):
                if key in chunk:
                    result[key] = chunk[key]
            
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
