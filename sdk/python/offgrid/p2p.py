"""
P2P (Peer-to-Peer) Client for OffGrid LLM.

Discover and interact with other OffGrid nodes on the local network.
"""

from typing import Dict, List, Optional


class P2P:
    """
    Peer-to-peer network client.
    
    Accessed via client.p2p:
        >>> client = Client()
        >>> status = client.p2p.status()
        >>> print(f"P2P Enabled: {status['enabled']}")
        
        >>> # List available peers
        >>> peers = client.p2p.peers()
        >>> for peer in peers:
        ...     print(f"{peer['hostname']}: {len(peer['models'])} models")
    """
    
    def __init__(self, client):
        self._client = client
    
    def status(self) -> Dict:
        """
        Get P2P network status.
        
        Returns:
            Dict with keys:
                - enabled: Whether P2P is enabled
                - node_id: This node's unique ID
                - hostname: This node's hostname
                - peer_count: Number of connected peers
                - models_shared: Number of models available to peers
        
        Example:
            >>> status = client.p2p.status()
            >>> if status['enabled']:
            ...     print(f"Connected to {status['peer_count']} peers")
        """
        return self._client._request("GET", "/v1/p2p/status")
    
    def peers(self) -> List[Dict]:
        """
        List all discovered peers on the network.
        
        Returns:
            List of peer dicts with keys:
                - node_id: Peer's unique ID
                - hostname: Peer's hostname
                - address: IP:port address
                - models: List of available model IDs
                - last_seen: Last heartbeat timestamp
                - capabilities: Dict of peer capabilities
        
        Example:
            >>> peers = client.p2p.peers()
            >>> for peer in peers:
            ...     print(f"{peer['hostname']} @ {peer['address']}")
            ...     for model in peer['models']:
            ...         print(f"  - {model}")
        """
        return self._client._request("GET", "/v1/p2p/peers")
    
    def models(self) -> List[Dict]:
        """
        List all models available across the P2P network.
        
        Returns:
            List of model dicts with keys:
                - model_id: Model identifier
                - peer_id: Which peer has this model
                - hostname: Peer hostname
                - size: Model size in bytes
                - hash: Model file hash
        
        Example:
            >>> models = client.p2p.models()
            >>> for m in models:
            ...     print(f"{m['model_id']} on {m['hostname']}")
        """
        return self._client._request("GET", "/v1/p2p/models")
    
    def download(
        self,
        model_id: str,
        peer_id: Optional[str] = None
    ) -> Dict:
        """
        Download a model from a peer.
        
        Args:
            model_id: The model ID to download
            peer_id: Optional specific peer to download from
                     (auto-selects fastest if not specified)
        
        Returns:
            Dict with download status
        
        Example:
            >>> # Download from any available peer
            >>> result = client.p2p.download("llama3")
            >>> print(f"Downloading from {result['peer']}")
            
            >>> # Download from specific peer
            >>> result = client.p2p.download("llama3", peer_id="node-abc123")
        """
        payload = {"model_id": model_id}
        if peer_id:
            payload["peer_id"] = peer_id
        
        return self._client._request("POST", "/v1/p2p/download", payload)
    
    def verify(self, model_id: str) -> Dict:
        """
        Verify a model's integrity with hash from peers.
        
        Compares local model hash against peer consensus to
        detect corruption or tampering.
        
        Args:
            model_id: The model ID to verify
        
        Returns:
            Dict with keys:
                - valid: True if hashes match
                - local_hash: Hash of local file
                - peer_hashes: Dict of peer_id -> hash
                - consensus: Most common hash value
        
        Example:
            >>> result = client.p2p.verify("llama3")
            >>> if result['valid']:
            ...     print("Model integrity verified")
            ... else:
            ...     print("WARNING: Hash mismatch!")
        """
        return self._client._request("POST", f"/v1/models/verify", {
            "model_id": model_id
        })
    
    def broadcast(self, model_id: str) -> Dict:
        """
        Broadcast that a model is available for sharing.
        
        Call this after downloading or adding a new model
        to let other peers know it's available.
        
        Args:
            model_id: The model ID to broadcast
        
        Returns:
            Dict with broadcast status
        
        Example:
            >>> client.p2p.broadcast("my-custom-model")
            {'status': 'broadcasted', 'peers_notified': 3}
        """
        return self._client._request("POST", "/v1/p2p/broadcast", {
            "model_id": model_id
        })
    
    def enable(self) -> Dict:
        """
        Enable P2P networking.
        
        Returns:
            Dict with status
        """
        return self._client._request("POST", "/v1/p2p/enable")
    
    def disable(self) -> Dict:
        """
        Disable P2P networking.
        
        Returns:
            Dict with status
        """
        return self._client._request("POST", "/v1/p2p/disable")
