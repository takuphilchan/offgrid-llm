package p2p

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSignedManifestRequiresPinnedPeer(t *testing.T) {
	directory := t.TempDir()
	identity, err := LoadOrCreateIdentity(filepath.Join(directory, "identity.key"))
	if err != nil {
		t.Fatal(err)
	}
	modelPath := filepath.Join(directory, "model.gguf")
	if err := os.WriteFile(modelPath, []byte("verified model content"), 0o600); err != nil {
		t.Fatal(err)
	}
	manifest, err := BuildManifest(identity, modelPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := manifest.Verify(); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
	trust, err := NewTrustStore(filepath.Join(directory, "trusted.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := trust.Verify(manifest); err == nil {
		t.Fatal("untrusted manifest was accepted")
	}
	if err := trust.Trust(identity.NodeID(), identity.PublicKey()); err != nil {
		t.Fatal(err)
	}
	if err := trust.Verify(manifest); err != nil {
		t.Fatalf("trusted signed manifest rejected: %v", err)
	}

	tampered := manifest
	tampered.Size++
	if err := trust.Verify(tampered); err == nil {
		t.Fatal("tampered manifest was accepted")
	}
}

func TestIdentityPersistsAndSignsFreshAnnouncements(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity.key")
	first, err := LoadOrCreateIdentity(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := LoadOrCreateIdentity(path)
	if err != nil {
		t.Fatal(err)
	}
	if first.NodeID() != second.NodeID() {
		t.Fatal("P2P identity changed after reload")
	}
	announcement := Announcement{
		NodeID: first.NodeID(), Port: 9090, Models: []string{"model-a"}, Version: "test",
		Timestamp: time.Now().UTC().Unix(), PublicKey: first.PublicKey(),
	}
	payload, _ := announcementPayload(announcement)
	announcement.Signature = first.sign(payload)
	if err := verifyAnnouncement(announcement, time.Now().UTC()); err != nil {
		t.Fatalf("signed announcement rejected: %v", err)
	}
	announcement.Models[0] = "tampered"
	if err := verifyAnnouncement(announcement, time.Now().UTC()); err == nil {
		t.Fatal("tampered announcement was accepted")
	}
}

func TestVerifiedTransferResumesAndInstallsAtomically(t *testing.T) {
	root := t.TempDir()
	serverDirectory := filepath.Join(root, "server")
	clientDirectory := filepath.Join(root, "client")
	if err := os.MkdirAll(serverDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	content := []byte("signed model content that resumes from a partial transfer")
	name := "model.gguf"
	if err := os.WriteFile(filepath.Join(serverDirectory, name), content, 0o600); err != nil {
		t.Fatal(err)
	}
	identity, err := LoadOrCreateIdentity(filepath.Join(root, "server-identity.key"))
	if err != nil {
		t.Fatal(err)
	}
	trust, err := NewTrustStore(filepath.Join(root, "client-trust.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := trust.Trust(identity.NodeID(), identity.PublicKey()); err != nil {
		t.Fatal(err)
	}
	server := NewTrustedTransferManager(0, serverDirectory, identity, nil)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server.serveListener(ctx, listener)
	address := listener.Addr().(*net.TCPAddr)
	peer := &Peer{ID: identity.NodeID(), PublicKey: identity.PublicKey(), Address: "127.0.0.1", Port: address.Port, Trusted: true}
	client := NewTrustedTransferManager(0, clientDirectory, nil, trust)
	manifest, err := client.FetchManifest(context.Background(), peer, name)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(clientDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	partial := filepath.Join(clientDirectory, "."+manifest.Digest+".part")
	if err := os.WriteFile(partial, content[:9], 0o600); err != nil {
		t.Fatal(err)
	}
	installed, err := client.DownloadVerified(context.Background(), peer, manifest)
	if err != nil {
		t.Fatal(err)
	}
	downloaded, err := os.ReadFile(installed)
	if err != nil {
		t.Fatal(err)
	}
	if string(downloaded) != string(content) {
		t.Fatalf("downloaded content mismatch: %q", downloaded)
	}
	if _, err := os.Stat(partial); !os.IsNotExist(err) {
		t.Fatal("partial file remained after atomic installation")
	}
}
