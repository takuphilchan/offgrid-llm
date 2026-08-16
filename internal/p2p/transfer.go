package p2p

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	mu              sync.RWMutex
	localPort       int
	downloadDir     string
	onProgress      func(TransferProgress)
	activeTransfers map[string]context.CancelFunc
	identity        *Identity
	trustStore      *TrustStore
}

// NewTransferManager creates a new transfer manager
func NewTransferManager(localPort int, downloadDir string) *TransferManager {
	return &TransferManager{
		localPort:       localPort,
		downloadDir:     downloadDir,
		activeTransfers: make(map[string]context.CancelFunc),
	}
}

func NewTrustedTransferManager(localPort int, downloadDir string, identity *Identity, trustStore *TrustStore) *TransferManager {
	manager := NewTransferManager(localPort, downloadDir)
	manager.identity = identity
	manager.trustStore = trustStore
	return manager
}

// SetProgressCallback sets a callback for transfer progress
func (tm *TransferManager) SetProgressCallback(callback func(TransferProgress)) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.onProgress = callback
}

// StartServer starts the file server for sharing models
func (tm *TransferManager) StartServer(ctx context.Context) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", tm.localPort))
	if err != nil {
		return fmt.Errorf("failed to start transfer server: %w", err)
	}

	log.Printf("📡 P2P Transfer server started on port %d", tm.localPort)
	tm.serveListener(ctx, listener)
	return nil
}

func (tm *TransferManager) serveListener(ctx context.Context, listener net.Listener) {
	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()
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
}

// handleConnection handles an incoming file request
func (tm *TransferManager) handleConnection(conn net.Conn) {
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(5 * time.Minute))
	reader := bufio.NewReader(io.LimitReader(conn, 4096))
	request, err := reader.ReadString('\n')
	if err != nil {
		log.Printf("Read error: %v", err)
		return
	}

	request = strings.TrimSpace(request)
	parts := strings.Fields(request)
	if len(parts) < 2 {
		conn.Write([]byte("ERROR Invalid request format\n"))
		return
	}
	command, clean := parts[0], strings.TrimSpace(parts[1])
	if !validSharedName(clean) {
		conn.Write([]byte("ERROR Invalid path\n"))
		return
	}
	file, err := tm.openShared(clean)
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

	if command == "MANIFEST" {
		if tm.identity == nil {
			conn.Write([]byte("ERROR Signed manifests unavailable\n"))
			return
		}
		manifest, err := BuildManifest(tm.identity, file.Name())
		if err != nil {
			conn.Write([]byte("ERROR Manifest failed\n"))
			return
		}
		encoded, _ := json.Marshal(manifest)
		_, _ = conn.Write(append(encoded, '\n'))
		return
	}

	offset := int64(0)
	headerPrefix := "OK"
	if command == "GET2" {
		if len(parts) != 3 {
			conn.Write([]byte("ERROR Resume offset required\n"))
			return
		}
		if _, err := fmt.Sscanf(parts[2], "%d", &offset); err != nil || offset < 0 || offset > stat.Size() {
			conn.Write([]byte("ERROR Invalid offset\n"))
			return
		}
		if _, err := file.Seek(offset, io.SeekStart); err != nil {
			conn.Write([]byte("ERROR Seek failed\n"))
			return
		}
		headerPrefix = "OK2"
	} else if command != "GET" {
		conn.Write([]byte("ERROR Unknown command\n"))
		return
	}

	// Send response header
	header := fmt.Sprintf("OK %d\n", stat.Size())
	if headerPrefix == "OK2" {
		header = fmt.Sprintf("OK2 %d %d\n", stat.Size(), offset)
	}
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

	log.Printf("Sent %d bytes to %s", written, conn.RemoteAddr())
}

func validSharedName(name string) bool {
	return name != "" && name != "." && !filepath.IsAbs(name) && filepath.Base(name) == name &&
		!strings.Contains(name, "..") && !strings.ContainsAny(name, "\\/")
}

func (tm *TransferManager) openShared(name string) (*os.File, error) {
	if !validSharedName(name) {
		return nil, fmt.Errorf("invalid shared filename")
	}
	path := filepath.Join(tm.downloadDir, name)
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("shared object must be a regular file")
	}
	return os.Open(path)
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
	fileClosed := false
	defer func() {
		if !fileClosed {
			file.Close()
		}
	}()

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
					os.Remove(tmpPath)
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
				os.Remove(tmpPath)
				return fmt.Errorf("read error: %w", err)
			}
		}
	}

	if err := file.Sync(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to sync file: %w", err)
	}
	if err := file.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to close file: %w", err)
	}
	fileClosed = true

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
		os.Remove(tmpPath)
		return fmt.Errorf("failed to finalize file: %w", err)
	}

	progress.Status = "complete"
	progress.Percent = 100
	progress.BytesTransferred = totalBytes
	tm.notifyProgress(progress)

	log.Printf("Downloaded %s from %s", filename, peer.Address)
	return nil
}

// FetchManifest retrieves and verifies a peer's signed content manifest. A
// peer must be explicitly trusted before its files can be installed.
func (tm *TransferManager) FetchManifest(ctx context.Context, peer *Peer, name string) (Manifest, error) {
	if peer == nil || !validSharedName(name) {
		return Manifest{}, fmt.Errorf("invalid peer or model name")
	}
	dialer := net.Dialer{Timeout: 30 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(peer.Address, fmt.Sprintf("%d", peer.Port)))
	if err != nil {
		return Manifest{}, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	if _, err := fmt.Fprintf(conn, "MANIFEST %s\n", name); err != nil {
		return Manifest{}, err
	}
	line, err := bufio.NewReader(io.LimitReader(conn, 64<<10)).ReadString('\n')
	if err != nil {
		return Manifest{}, err
	}
	if strings.HasPrefix(line, "ERROR") {
		return Manifest{}, fmt.Errorf("peer error: %s", strings.TrimSpace(line))
	}
	var manifest Manifest
	if err := json.Unmarshal([]byte(line), &manifest); err != nil {
		return Manifest{}, fmt.Errorf("invalid peer manifest: %w", err)
	}
	if manifest.Name != name || manifest.NodeID != peer.ID || manifest.PublicKey != peer.PublicKey {
		return Manifest{}, fmt.Errorf("peer manifest identity or name mismatch")
	}
	if tm.trustStore == nil {
		return Manifest{}, fmt.Errorf("P2P trust store is unavailable")
	}
	if err := tm.trustStore.Verify(manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

// DownloadVerified resumes a signed transfer, verifies its complete digest,
// and atomically installs it under the manifest's safe filename.
func (tm *TransferManager) DownloadVerified(ctx context.Context, peer *Peer, manifest Manifest) (string, error) {
	if tm.trustStore == nil {
		return "", fmt.Errorf("P2P trust store is unavailable")
	}
	if err := tm.trustStore.Verify(manifest); err != nil {
		return "", err
	}
	if peer == nil || peer.ID != manifest.NodeID || peer.PublicKey != manifest.PublicKey {
		return "", fmt.Errorf("manifest does not belong to selected peer")
	}
	if err := os.MkdirAll(tm.downloadDir, 0o755); err != nil {
		return "", err
	}
	destination := filepath.Join(tm.downloadDir, manifest.Name)
	partial := filepath.Join(tm.downloadDir, "."+manifest.Digest+".part")
	offset := int64(0)
	if info, err := os.Stat(partial); err == nil {
		if info.Size() <= manifest.Size {
			offset = info.Size()
		} else if err := os.Remove(partial); err != nil {
			return "", err
		}
	}

	hash := sha256.New()
	if offset > 0 {
		existing, err := os.Open(partial)
		if err != nil {
			return "", err
		}
		_, err = io.Copy(hash, existing)
		existing.Close()
		if err != nil {
			return "", err
		}
	}

	dialer := net.Dialer{Timeout: 30 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(peer.Address, fmt.Sprintf("%d", peer.Port)))
	if err != nil {
		return "", err
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	if _, err := fmt.Fprintf(conn, "GET2 %s %d\n", manifest.Name, offset); err != nil {
		return "", err
	}
	reader := bufio.NewReader(conn)
	header, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	var size, acceptedOffset int64
	if _, err := fmt.Sscanf(header, "OK2 %d %d", &size, &acceptedOffset); err != nil || size != manifest.Size || acceptedOffset != offset {
		return "", fmt.Errorf("invalid resume response: %s", strings.TrimSpace(header))
	}
	file, err := os.OpenFile(partial, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return "", err
	}
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		file.Close()
		return "", err
	}
	progress := TransferProgress{BytesTransferred: offset, TotalBytes: manifest.Size, Peer: peer.Address, Status: "downloading"}
	tm.notifyProgress(progress)
	written, copyErr := copyTransferContext(ctx, io.MultiWriter(file, hash), reader, manifest.Size-offset, func(bytes int64) {
		progress.BytesTransferred = offset + bytes
		if manifest.Size > 0 {
			progress.Percent = float64(progress.BytesTransferred) / float64(manifest.Size) * 100
		}
		tm.notifyProgress(progress)
	})
	if syncErr := file.Sync(); copyErr == nil {
		copyErr = syncErr
	}
	if closeErr := file.Close(); copyErr == nil {
		copyErr = closeErr
	}
	if copyErr != nil {
		return "", copyErr
	}
	if offset+written != manifest.Size || hex.EncodeToString(hash.Sum(nil)) != manifest.Digest {
		_ = os.Remove(partial)
		return "", fmt.Errorf("downloaded content failed size or SHA-256 verification")
	}
	if existing, err := os.Open(destination); err == nil {
		existingHash := sha256.New()
		_, hashErr := io.Copy(existingHash, existing)
		existing.Close()
		if hashErr != nil || hex.EncodeToString(existingHash.Sum(nil)) != manifest.Digest {
			return "", fmt.Errorf("destination already exists with different content")
		}
		_ = os.Remove(partial)
		return destination, nil
	}
	if err := os.Rename(partial, destination); err != nil {
		return "", err
	}
	progress.BytesTransferred, progress.Percent, progress.Status = manifest.Size, 100, "complete"
	tm.notifyProgress(progress)
	return destination, nil
}

func copyTransferContext(ctx context.Context, writer io.Writer, reader io.Reader, remaining int64, progress func(int64)) (int64, error) {
	buffer := make([]byte, 256*1024)
	var total int64
	for total < remaining {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		want := int64(len(buffer))
		if left := remaining - total; left < want {
			want = left
		}
		n, readErr := reader.Read(buffer[:want])
		if n > 0 {
			written, writeErr := writer.Write(buffer[:n])
			total += int64(written)
			if progress != nil {
				progress(total)
			}
			if writeErr != nil {
				return total, writeErr
			}
			if written != n {
				return total, io.ErrShortWrite
			}
		}
		if readErr != nil {
			if readErr == io.EOF && total == remaining {
				return total, nil
			}
			return total, readErr
		}
	}
	return total, nil
}

// CancelTransfer cancels an active transfer
func (tm *TransferManager) CancelTransfer(transferID string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if cancel, exists := tm.activeTransfers[transferID]; exists {
		cancel()
		delete(tm.activeTransfers, transferID)
	}
}

// notifyProgress calls the progress callback if set
func (tm *TransferManager) notifyProgress(progress TransferProgress) {
	tm.mu.RLock()
	callback := tm.onProgress
	tm.mu.RUnlock()
	if callback != nil {
		callback(progress)
	}
}
