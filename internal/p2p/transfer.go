package p2p

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TransferProgress represents the progress of a file transfer
type TransferProgress struct {
	BytesTransferred int64
	TotalBytes       int64
	Percent          float64
	Speed            int64 // Bytes per second
	Peer             string
	Status           string // "downloading", "verifying", "complete", "failed"
	Error            error
}

// TransferManager handles P2P file transfers
type TransferManager struct {
	localPort       int
	downloadDir     string
	onProgress      func(TransferProgress)
	activeTransfers map[string]context.CancelFunc
}

// NewTransferManager creates a new transfer manager
func NewTransferManager(localPort int, downloadDir string) *TransferManager {
	return &TransferManager{
		localPort:       localPort,
		downloadDir:     downloadDir,
		activeTransfers: make(map[string]context.CancelFunc),
	}
}

// SetProgressCallback sets a callback for transfer progress
func (tm *TransferManager) SetProgressCallback(callback func(TransferProgress)) {
	tm.onProgress = callback
}

// StartServer starts the file server for sharing models
func (tm *TransferManager) StartServer(ctx context.Context) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", tm.localPort))
	if err != nil {
		return fmt.Errorf("failed to start transfer server: %w", err)
	}

	log.Printf("📡 P2P Transfer server started on port %d", tm.localPort)

	go func() {
		defer listener.Close()

		for {
			select {
			case <-ctx.Done():
				return
			default:
				conn, err := listener.Accept()
				if err != nil {
					if ctx.Err() != nil {
						return
					}
					log.Printf("Accept error: %v", err)
					continue
				}

				go tm.handleConnection(conn)
			}
		}
	}()

	return nil
}

// handleConnection handles an incoming file request
func (tm *TransferManager) handleConnection(conn net.Conn) {
	defer conn.Close()

	// Read request (format: "GET <filepath>")
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		log.Printf("Read error: %v", err)
		return
	}

	request := string(buf[:n])
	log.Printf("Received request: %s", request)

	// Parse request
	var filePath string
	if _, err := fmt.Sscanf(request, "GET %s", &filePath); err != nil {
		conn.Write([]byte("ERROR Invalid request format\n"))
		return
	}

	// Validate and serve file: only allow a plain filename (no absolute paths, no traversal)
	clean := strings.TrimSpace(filePath)
	if clean == "" {
		conn.Write([]byte("ERROR Empty path\n"))
		return
	}
	if filepath.IsAbs(clean) || strings.Contains(clean, "..") || strings.ContainsAny(clean, "\\/") {
		conn.Write([]byte("ERROR Invalid path\n"))
		return
	}

	fullPath := filepath.Join(tm.downloadDir, clean)

	file, err := os.Open(fullPath)
	if err != nil {
		conn.Write([]byte(fmt.Sprintf("ERROR File not found: %v\n", err)))
		return
	}
	defer file.Close()

	// Get file info
	stat, err := file.Stat()
	if err != nil {
		conn.Write([]byte(fmt.Sprintf("ERROR Stat failed: %v\n", err)))
		return
	}

	// Send response header
	header := fmt.Sprintf("OK %d\n", stat.Size())
	if _, err := conn.Write([]byte(header)); err != nil {
		log.Printf("Header write error: %v", err)
		return
	}

	// Stream file
	written, err := io.Copy(conn, file)
	if err != nil {
		log.Printf("Transfer error: %v", err)
		return
	}

	log.Printf("✅ Sent %d bytes to %s", written, conn.RemoteAddr())
}

// DownloadFromPeer downloads a file from a peer
func (tm *TransferManager) DownloadFromPeer(ctx context.Context, peer *Peer, modelPath string, expectedHash string) error {
	// Connect to peer (use net.JoinHostPort for IPv6 compatibility)
	addr := net.JoinHostPort(peer.Address, fmt.Sprintf("%d", peer.Port))
	conn, err := net.DialTimeout("tcp", addr, 30*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to peer %s: %w", addr, err)
	}
	defer conn.Close()

	// Send request (only request by filename)
	requestName := filepath.Base(strings.TrimSpace(modelPath))
	if requestName == "." || requestName == "" {
		return fmt.Errorf("invalid model path")
	}
	request := fmt.Sprintf("GET %s\n", requestName)
	if _, err := conn.Write([]byte(request)); err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	// Use bufio.Reader to handle buffering correctly
	reader := bufio.NewReader(conn)

	// Read response header
	header, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read response header: %w", err)
	}

	var totalBytes int64
	if _, err := fmt.Sscanf(header, "OK %d\n", &totalBytes); err != nil {
		if strings.HasPrefix(header, "ERROR") {
			return fmt.Errorf("peer error: %s", strings.TrimSpace(header))
		}
		return fmt.Errorf("invalid response: %s", header)
	}

	// Create destination file
	filename := requestName
	destPath := filepath.Join(tm.downloadDir, filename)
	tmpPath := destPath + ".tmp"

	file, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Download with progress tracking
	progress := TransferProgress{
		TotalBytes: totalBytes,
		Peer:       peer.Address,
		Status:     "downloading",
	}
	tm.notifyProgress(progress)

	startTime := time.Now()
	lastUpdate := time.Now()
	updateInterval := 500 * time.Millisecond

	hash := sha256.New()
	multiWriter := io.MultiWriter(file, hash)

	buffer := make([]byte, 32*1024) // 32KB buffer
	var bytesRead int64

	for bytesRead < totalBytes {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			n, err := reader.Read(buffer)
			if n > 0 {
				if _, writeErr := multiWriter.Write(buffer[:n]); writeErr != nil {
					return fmt.Errorf("write error: %w", writeErr)
				}
				bytesRead += int64(n)

				// Update progress periodically
				if time.Since(lastUpdate) >= updateInterval {
					elapsed := time.Since(startTime).Seconds()
					speed := int64(float64(bytesRead) / elapsed)

					progress.BytesTransferred = bytesRead
					progress.Percent = float64(bytesRead) / float64(totalBytes) * 100
					progress.Speed = speed
					tm.notifyProgress(progress)

					lastUpdate = time.Now()
				}
			}

			if err != nil {
				if err == io.EOF {
					break
				}
				return fmt.Errorf("read error: %w", err)
			}
		}
	}

	// Verify checksum if provided
	if expectedHash != "" {
		progress.Status = "verifying"
		tm.notifyProgress(progress)

		actualHash := hex.EncodeToString(hash.Sum(nil))
		if actualHash != expectedHash {
			os.Remove(tmpPath)
			return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHash, actualHash)
		}
	}

	// Move temp file to final location
	if err := os.Rename(tmpPath, destPath); err != nil {
		return fmt.Errorf("failed to finalize file: %w", err)
	}

	progress.Status = "complete"
	progress.Percent = 100
	progress.BytesTransferred = totalBytes
	tm.notifyProgress(progress)

	log.Printf("✅ Downloaded %s from %s", filename, peer.Address)
	return nil
}

// CancelTransfer cancels an active transfer
func (tm *TransferManager) CancelTransfer(transferID string) {
	if cancel, exists := tm.activeTransfers[transferID]; exists {
		cancel()
		delete(tm.activeTransfers, transferID)
	}
}

// notifyProgress calls the progress callback if set
func (tm *TransferManager) notifyProgress(progress TransferProgress) {
	if tm.onProgress != nil {
		tm.onProgress(progress)
	}
}
