package p2p

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTransferManager(t *testing.T) {
	// Setup directories
	tmpDir, err := os.MkdirTemp("", "p2p_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	serverDir := filepath.Join(tmpDir, "server")
	clientDir := filepath.Join(tmpDir, "client")
	os.Mkdir(serverDir, 0755)
	os.Mkdir(clientDir, 0755)

	// Create a dummy file to serve
	testContent := "Hello P2P World! This is a test file."
	testFileName := "test_model.bin"
	serverFilePath := filepath.Join(serverDir, testFileName)
	err = os.WriteFile(serverFilePath, []byte(testContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Calculate hash
	hasher := sha256.New()
	hasher.Write([]byte(testContent))
	expectedHash := hex.EncodeToString(hasher.Sum(nil))

	// Start Server TransferManager
	serverPort := 12345
	serverTM := NewTransferManager(serverPort, serverDir)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = serverTM.StartServer(ctx)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Start Client TransferManager
	clientPort := 12346
	clientTM := NewTransferManager(clientPort, clientDir)
	// We don't need to start the server on the client for downloading, but we can.

	// Create a peer representing the server
	serverPeer := &Peer{
		ID:      "server-node",
		Address: "127.0.0.1",
		Port:    serverPort,
	}

	// Test Download
	t.Logf("Starting download of %s...", testFileName)
	err = clientTM.DownloadFromPeer(ctx, serverPeer, testFileName, expectedHash)
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	// Verify file exists and content matches
	clientFilePath := filepath.Join(clientDir, testFileName)
	content, err := os.ReadFile(clientFilePath)
	if err != nil {
		t.Fatalf("Failed to read downloaded file: %v", err)
	}

	if string(content) != testContent {
		t.Errorf("Content mismatch. Expected %q, got %q", testContent, string(content))
	}

	t.Log("Download successful and verified!")
}

func TestDiscoveryLogic(t *testing.T) {
	// This test verifies the logic of handling announcements without network binding
	d := NewDiscovery(8080, 9000)

	// Simulate receiving an announcement
	announcementJSON := `{
		"node_id": "peer-1",
		"port": 8081,
		"models": ["llama-2-7b"],
		"version": "0.2.9"
	}`

	d.handleAnnouncement(announcementJSON, "192.168.1.50")

	peers := d.GetPeers()
	if len(peers) != 1 {
		t.Errorf("Expected 1 peer, got %d", len(peers))
	}

	peer := peers[0]
	if peer.ID != "peer-1" {
		t.Errorf("Expected peer ID 'peer-1', got %s", peer.ID)
	}
	if peer.Address != "192.168.1.50" {
		t.Errorf("Expected peer Address '192.168.1.50', got %s", peer.Address)
	}
	if peer.Port != 8081 {
		t.Errorf("Expected peer Port 8081, got %d", peer.Port)
	}
	if len(peer.Models) != 1 || peer.Models[0] != "llama-2-7b" {
		t.Errorf("Models mismatch")
	}

	// Test finding model
	peersWithModel := d.FindModelOnPeers("llama-2-7b")
	if len(peersWithModel) != 1 {
		t.Errorf("Expected to find 1 peer with model, got %d", len(peersWithModel))
	}
}
