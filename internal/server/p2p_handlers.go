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

	peers := s.p2pDiscovery.GetPeers()
	json.NewEncoder(w).Encode(peers)
}

// handleP2PDownload initiates a download from a peer
func (s *Server) handleP2PDownload(w http.ResponseWriter, r *http.Request) {
	if !s.config.EnableP2P {
		writeError(w, "P2P is disabled", http.StatusForbidden)
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

		// Start download
		err := s.p2pTransfer.DownloadFromPeer(context.Background(), targetPeer, req.ModelPath, req.Hash)
		if err != nil {
			log.Printf("P2P download failed: %v", err)
			// Error is handled in callback
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
		"status":  "download_started",
		"message": fmt.Sprintf("Downloading %s from %s", req.ModelPath, req.PeerID),
	})
}
