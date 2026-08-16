package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"

	"github.com/takuphilchan/offgrid-llm/internal/p2p"
)

// handleP2PPeers returns a list of discovered peers
func (s *Server) handleP2PPeers(w http.ResponseWriter, r *http.Request) {
	if !s.config.EnableP2P {
		writeError(w, "P2P is disabled", http.StatusForbidden)
		return
	}
	if s.p2pDiscovery == nil {
		writeError(w, "Secure P2P is unavailable", http.StatusServiceUnavailable)
		return
	}

	peers := s.p2pDiscovery.GetPeers()
	json.NewEncoder(w).Encode(peers)
}

// handleP2PStatus returns the P2P network status including shared models
func (s *Server) handleP2PStatus(w http.ResponseWriter, r *http.Request) {
	type P2PStatus struct {
		Enabled           bool     `json:"enabled"`
		NodeID            string   `json:"node_id"`
		PeerCount         int      `json:"peer_count"`
		SharedModels      []string `json:"shared_models"`
		RemoteModels      int      `json:"remote_models"`
		Maturity          string   `json:"maturity"`
		TransferAvailable bool     `json:"transfer_available"`
		Message           string   `json:"message,omitempty"`
		TrustedPeers      int      `json:"trusted_peers"`
	}

	status := P2PStatus{
		Enabled:           s.config.EnableP2P,
		SharedModels:      []string{},
		Maturity:          "signed-beta",
		TransferAvailable: s.config.EnableP2P && s.p2pTransfer != nil,
		Message:           "P2P requires an explicitly trusted Ed25519 peer identity and verifies signed SHA-256 manifests.",
	}

	if s.config.EnableP2P && s.p2pDiscovery != nil {
		peers := s.p2pDiscovery.GetPeers()
		status.PeerCount = len(peers)
		status.NodeID = s.p2pDiscovery.GetNodeID()

		// Get shared models from registry
		for _, m := range s.registry.ListModels() {
			status.SharedModels = append(status.SharedModels, m.ID)
		}

		// Count unique remote models
		remoteModels := make(map[string]bool)
		for _, peer := range peers {
			if peer.Trusted {
				status.TrustedPeers++
			}
			for _, model := range peer.Models {
				remoteModels[model] = true
			}
		}
		status.RemoteModels = len(remoteModels)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// handleP2PTrust explicitly pins a discovered peer's public key. Discovery
// alone never grants permission to install content.
func (s *Server) handleP2PTrust(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.config.EnableP2P || s.p2pDiscovery == nil || s.p2pTrust == nil {
		writeError(w, "Secure P2P is unavailable", http.StatusServiceUnavailable)
		return
	}
	var request struct {
		PeerID string `json:"peer_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.PeerID == "" {
		writeError(w, "peer_id is required", http.StatusBadRequest)
		return
	}
	var selected *p2p.Peer
	for _, peer := range s.p2pDiscovery.GetPeers() {
		if peer.ID == request.PeerID {
			selected = peer
			break
		}
	}
	if selected == nil || selected.PublicKey == "" {
		writeError(w, "Signed peer not found", http.StatusNotFound)
		return
	}
	if err := s.p2pTrust.Trust(selected.ID, selected.PublicKey); err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}
	// Refresh the peer's trusted projection using its latest signed announcement.
	s.p2pDiscovery.SetIdentity(s.p2pIdentity, s.p2pTrust)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "trusted", "peer_id": selected.ID, "public_key": selected.PublicKey})
}

// handleP2PDownload initiates a download from a peer
func (s *Server) handleP2PDownload(w http.ResponseWriter, r *http.Request) {
	if !s.config.EnableP2P {
		writeError(w, "P2P is disabled", http.StatusForbidden)
		return
	}
	if s.p2pDiscovery == nil || s.p2pTransfer == nil || s.p2pTrust == nil {
		writeError(w, "Secure P2P is unavailable", http.StatusServiceUnavailable)
		return
	}

	if r.Method != http.MethodPost {
		writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		PeerID    string `json:"peer_id"`
		ModelPath string `json:"model_path"`
		Hash      string `json:"hash"` // Optional checksum
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate path first (fail fast on bad input)
	// Only allow requesting a plain filename from peers (no absolute paths / traversal)
	requestedName := filepath.Base(req.ModelPath)
	if requestedName == "." || requestedName == string(filepath.Separator) || requestedName == "" {
		writeError(w, "Invalid model_path", http.StatusBadRequest)
		return
	}
	if requestedName != req.ModelPath {
		// Reject rather than silently changing semantics
		writeError(w, "model_path must be a filename (no directories)", http.StatusBadRequest)
		return
	}

	// Find peer
	var targetPeer *p2p.Peer
	for _, p := range s.p2pDiscovery.GetPeers() {
		if p.ID == req.PeerID {
			targetPeer = p
			break
		}
	}

	if targetPeer == nil {
		writeError(w, "Peer not found", http.StatusNotFound)
		return
	}
	if !s.p2pTrust.IsTrusted(targetPeer.ID, targetPeer.PublicKey) {
		writeError(w, "Peer is not trusted; trust its pinned identity before downloading", http.StatusForbidden)
		return
	}

	// Start download in background
	go func() {
		log.Printf("Starting P2P download of %s from %s", req.ModelPath, req.PeerID)

		// Track progress
		progressID := fmt.Sprintf("p2p-%s-%s", req.PeerID, filepath.Base(req.ModelPath))

		// Set up progress callback
		s.p2pTransfer.SetProgressCallback(func(p p2p.TransferProgress) {
			s.downloadMutex.Lock()
			s.downloadProgress[progressID] = &DownloadProgress{
				FileName:   filepath.Base(req.ModelPath),
				BytesTotal: p.TotalBytes,
				BytesDone:  p.BytesTransferred,
				Percent:    p.Percent,
				Status:     p.Status,
				Error:      "",
			}
			if p.Error != nil {
				s.downloadProgress[progressID].Error = p.Error.Error()
				s.downloadProgress[progressID].Status = "failed"
			}
			s.downloadMutex.Unlock()
		})

		manifest, err := s.p2pTransfer.FetchManifest(context.Background(), targetPeer, req.ModelPath)
		if err == nil {
			_, err = s.p2pTransfer.DownloadVerified(context.Background(), targetPeer, manifest)
		}
		if err != nil {
			log.Printf("P2P download failed: %v", err)
			s.downloadMutex.Lock()
			s.downloadProgress[progressID] = &DownloadProgress{
				FileName:   filepath.Base(req.ModelPath),
				BytesTotal: 0,
				BytesDone:  0,
				Percent:    0,
				Status:     "failed",
				Error:      err.Error(),
			}
			s.downloadMutex.Unlock()
		} else {
			log.Printf("P2P download complete: %s", req.ModelPath)
			// Refresh local models
			s.registry.ScanModels()
			// Update discovery with new model
			modelIDs := make([]string, 0)
			for _, m := range s.registry.ListModels() {
				modelIDs = append(modelIDs, m.ID)
			}
			s.p2pDiscovery.SetLocalModels(modelIDs)
		}
	}()

	// Return success immediately
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "verified_download_started",
		"message": fmt.Sprintf("Downloading and verifying %s from trusted peer %s", req.ModelPath, req.PeerID),
	})
}
