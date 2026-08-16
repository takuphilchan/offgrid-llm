// Package p2p provides secure P2P model transfer with encryption and resume support.
// This file adds AES-GCM encryption and resumable transfer capabilities.
package p2p

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
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

// SecureTransferState represents the state of a secure transfer
type SecureTransferState string

const (
	SecureTransferPending   SecureTransferState = "pending"
	SecureTransferActive    SecureTransferState = "active"
	SecureTransferPaused    SecureTransferState = "paused"
	SecureTransferCompleted SecureTransferState = "completed"
	SecureTransferFailed    SecureTransferState = "failed"
)

// SecureTransferInfo contains metadata about a secure file transfer
type SecureTransferInfo struct {
	ID              string              `json:"id"`
	Filename        string              `json:"filename"`
	TotalSize       int64               `json:"total_size"`
	TransferredSize int64               `json:"transferred_size"`
	SHA256          string              `json:"sha256"`
	State           SecureTransferState `json:"state"`
	PeerID          string              `json:"peer_id"`
	StartedAt       time.Time           `json:"started_at"`
	UpdatedAt       time.Time           `json:"updated_at"`
	Error           string              `json:"error,omitempty"`
	ChunkSize       int                 `json:"chunk_size"`
	Encrypted       bool                `json:"encrypted"`
}

// SecureTransferManager handles encrypted, resumable file transfers
type SecureTransferManager struct {
	mu            sync.RWMutex
	transfers     map[string]*SecureTransferInfo
	dataDir       string
	tempDir       string
	port          int
	encryptionKey []byte
	chunkSize     int
}

// SecureTransferConfig configures the secure transfer manager
type SecureTransferConfig struct {
	DataDir       string `json:"data_dir"`
	Port          int    `json:"port"`
	EncryptionKey string `json:"encryption_key"` // hex-encoded 32-byte key
	ChunkSize     int    `json:"chunk_size"`
}

// DefaultSecureTransferConfig returns sensible defaults
func DefaultSecureTransferConfig(dataDir string) SecureTransferConfig {
	return SecureTransferConfig{
		DataDir:   dataDir,
		Port:      9091,
		ChunkSize: 1024 * 1024, // 1MB chunks
	}
}

// NewSecureTransferManager creates a new secure transfer manager
func NewSecureTransferManager(config SecureTransferConfig) (*SecureTransferManager, error) {
	transferDir := filepath.Join(config.DataDir, "transfers")
	tempDir := filepath.Join(transferDir, "temp")
	keyPath := filepath.Join(transferDir, ".secure_key")
	if err := os.MkdirAll(tempDir, 0700); err != nil {
		return nil, err
	}

	// Parse or generate encryption key
	var key []byte
	if config.EncryptionKey != "" {
		var err error
		key, err = hex.DecodeString(config.EncryptionKey)
		if err != nil || len(key) != 32 {
			return nil, fmt.Errorf("invalid encryption key: must be 32 bytes hex-encoded")
		}
		if err := writeFileAtomic(keyPath, []byte(hex.EncodeToString(key)+"\n"), 0600); err != nil {
			return nil, fmt.Errorf("persist encryption key: %w", err)
		}
	} else {
		stored, err := os.ReadFile(keyPath)
		if err == nil {
			key, err = hex.DecodeString(strings.TrimSpace(string(stored)))
			if err != nil || len(key) != 32 {
				return nil, fmt.Errorf("invalid persisted encryption key")
			}
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("read encryption key: %w", err)
		} else {
			key = make([]byte, 32)
			if _, err := rand.Read(key); err != nil {
				return nil, err
			}
			if err := writeFileAtomic(keyPath, []byte(hex.EncodeToString(key)+"\n"), 0600); err != nil {
				return nil, fmt.Errorf("persist encryption key: %w", err)
			}
		}
	}

	chunkSize := config.ChunkSize
	if chunkSize <= 0 {
		chunkSize = 1024 * 1024
	}

	tm := &SecureTransferManager{
		transfers:     make(map[string]*SecureTransferInfo),
		dataDir:       config.DataDir,
		tempDir:       tempDir,
		port:          config.Port,
		encryptionKey: key,
		chunkSize:     chunkSize,
	}

	// Load pending transfers
	tm.loadState()

	return tm, nil
}

// GetEncryptionKeyHex returns the encryption key as hex string
// Share this with peers for secure transfer
func (stm *SecureTransferManager) GetEncryptionKeyHex() string {
	stm.mu.RLock()
	defer stm.mu.RUnlock()
	return hex.EncodeToString(stm.encryptionKey)
}

// SetEncryptionKey sets the encryption key from a peer
func (stm *SecureTransferManager) SetEncryptionKey(hexKey string) error {
	key, err := hex.DecodeString(hexKey)
	if err != nil || len(key) != 32 {
		return fmt.Errorf("invalid key: must be 32 bytes hex-encoded")
	}
	keyPath := filepath.Join(stm.dataDir, "transfers", ".secure_key")
	if err := writeFileAtomic(keyPath, []byte(hex.EncodeToString(key)+"\n"), 0600); err != nil {
		return fmt.Errorf("persist encryption key: %w", err)
	}
	stm.mu.Lock()
	stm.encryptionKey = append([]byte(nil), key...)
	stm.mu.Unlock()
	return nil
}

func (stm *SecureTransferManager) encryptionKeyCopy() []byte {
	stm.mu.RLock()
	defer stm.mu.RUnlock()
	return append([]byte(nil), stm.encryptionKey...)
}

// EncryptFile encrypts a file for secure transfer
func (stm *SecureTransferManager) EncryptFile(inputPath, outputPath string) error {
	block, err := aes.NewCipher(stm.encryptionKeyCopy())
	if err != nil {
		return err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	inFile, err := os.Open(inputPath)
	if err != nil {
		return err
	}
	defer inFile.Close()

	outFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	// Write header with chunk size
	if err := binary.Write(outFile, binary.BigEndian, int32(stm.chunkSize)); err != nil {
		return err
	}

	buf := make([]byte, stm.chunkSize)
	nonce := make([]byte, gcm.NonceSize())

	for {
		n, err := inFile.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Generate random nonce
		if _, err := rand.Read(nonce); err != nil {
			return err
		}

		// Encrypt chunk
		encrypted := gcm.Seal(nonce, nonce, buf[:n], nil)

		// Write chunk length and data
		if err := binary.Write(outFile, binary.BigEndian, int32(len(encrypted))); err != nil {
			return err
		}
		if _, err := outFile.Write(encrypted); err != nil {
			return err
		}
	}

	// Write end marker
	return binary.Write(outFile, binary.BigEndian, int32(0))
}

// DecryptFile decrypts a file received via secure transfer
func (stm *SecureTransferManager) DecryptFile(inputPath, outputPath string) error {
	block, err := aes.NewCipher(stm.encryptionKeyCopy())
	if err != nil {
		return err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	inFile, err := os.Open(inputPath)
	if err != nil {
		return err
	}
	defer inFile.Close()

	outFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	// Read chunk size from header
	var chunkSize int32
	if err := binary.Read(inFile, binary.BigEndian, &chunkSize); err != nil {
		return fmt.Errorf("read encrypted file header: %w", err)
	}
	if chunkSize <= 0 || chunkSize > 64*1024*1024 {
		return fmt.Errorf("invalid encrypted chunk size: %d", chunkSize)
	}

	nonceSize := gcm.NonceSize()

	for {
		// Read chunk length
		var length int32
		if err := binary.Read(inFile, binary.BigEndian, &length); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		if length == 0 {
			break // End marker
		}
		maxEncryptedLength := int64(chunkSize) + int64(nonceSize) + int64(gcm.Overhead())
		if length < int32(nonceSize+gcm.Overhead()) || int64(length) > maxEncryptedLength {
			return fmt.Errorf("invalid encrypted chunk length: %d", length)
		}

		// Read encrypted chunk
		encrypted := make([]byte, length)
		if _, err := io.ReadFull(inFile, encrypted); err != nil {
			return err
		}

		// Decrypt
		if len(encrypted) < nonceSize {
			return fmt.Errorf("invalid chunk size")
		}

		nonce, ciphertext := encrypted[:nonceSize], encrypted[nonceSize:]
		plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
		if err != nil {
			return fmt.Errorf("decryption failed: %w", err)
		}

		if _, err := outFile.Write(plaintext); err != nil {
			return err
		}
	}

	return nil
}

// CreateResumableTransfer creates a transfer that can be resumed
func (stm *SecureTransferManager) CreateResumableTransfer(filePath string) (*SecureTransferInfo, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}

	transfer := &SecureTransferInfo{
		ID:        generateSecureTransferID(),
		Filename:  filepath.Base(filePath),
		TotalSize: info.Size(),
		State:     SecureTransferPending,
		StartedAt: time.Now(),
		UpdatedAt: time.Now(),
		ChunkSize: stm.chunkSize,
		Encrypted: true,
	}

	stm.mu.Lock()
	stm.transfers[transfer.ID] = transfer
	stm.mu.Unlock()

	if err := stm.saveState(); err != nil {
		stm.mu.Lock()
		delete(stm.transfers, transfer.ID)
		stm.mu.Unlock()
		return nil, fmt.Errorf("persist transfer state: %w", err)
	}
	result := *transfer
	return &result, nil
}

// ResumeTransfer gets the offset for resuming a transfer
func (stm *SecureTransferManager) ResumeTransfer(transferID string) (int64, error) {
	stm.mu.RLock()
	stored, ok := stm.transfers[transferID]
	var transfer SecureTransferInfo
	if ok {
		transfer = *stored
	}
	stm.mu.RUnlock()

	if !ok {
		return 0, fmt.Errorf("transfer not found: %s", transferID)
	}

	if transfer.State != SecureTransferPaused {
		return 0, fmt.Errorf("transfer is not paused")
	}

	// Check partial file
	partialPath := filepath.Join(stm.tempDir, transferID+".partial")
	info, err := os.Stat(partialPath)
	if err != nil {
		return 0, nil // Start from beginning
	}

	return info.Size(), nil
}

// WriteChunk writes a chunk to a resumable transfer
func (stm *SecureTransferManager) WriteChunk(transferID string, offset int64, data []byte, isLast bool) error {
	if offset < 0 {
		return fmt.Errorf("invalid negative transfer offset")
	}

	stm.mu.RLock()
	stored, ok := stm.transfers[transferID]
	var transfer SecureTransferInfo
	if ok {
		transfer = *stored
	}
	stm.mu.RUnlock()

	if !ok {
		return fmt.Errorf("transfer not found")
	}
	if len(data) > transfer.ChunkSize {
		return fmt.Errorf("chunk exceeds configured size")
	}
	endOffset := offset + int64(len(data))
	if endOffset < offset || endOffset > transfer.TotalSize {
		return fmt.Errorf("chunk exceeds declared transfer size")
	}
	if isLast && endOffset != transfer.TotalSize {
		return fmt.Errorf("final chunk does not complete transfer")
	}

	partialPath := filepath.Join(stm.tempDir, transferID+".partial")

	// Open file for writing at offset
	file, err := os.OpenFile(partialPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	if info, err := file.Stat(); err != nil {
		return err
	} else if info.Size() != offset {
		return fmt.Errorf("unexpected transfer offset: got %d, want %d", offset, info.Size())
	}

	// Seek to offset
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return err
	}

	// Write data
	if n, err := file.Write(data); err != nil {
		return err
	} else if n != len(data) {
		return io.ErrShortWrite
	}

	if isLast {
		// Move to final location
		finalPath := filepath.Join(stm.dataDir, "models", transfer.Filename)
		if err := os.MkdirAll(filepath.Dir(finalPath), 0755); err != nil {
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
		if err := os.Rename(partialPath, finalPath); err != nil {
			return fmt.Errorf("finalize transfer: %w", err)
		}
	}

	stm.mu.Lock()
	if current, exists := stm.transfers[transferID]; exists {
		current.TransferredSize = endOffset
		current.UpdatedAt = time.Now()
		if isLast {
			current.State = SecureTransferCompleted
		} else {
			current.State = SecureTransferActive
		}
	}
	stm.mu.Unlock()

	return stm.saveState()
}

// PauseTransfer pauses an active transfer
func (stm *SecureTransferManager) PauseTransfer(transferID string) error {
	stm.mu.Lock()
	transfer, ok := stm.transfers[transferID]
	if !ok {
		stm.mu.Unlock()
		return fmt.Errorf("transfer not found")
	}

	transfer.State = SecureTransferPaused
	transfer.UpdatedAt = time.Now()
	stm.mu.Unlock()
	return stm.saveState()
}

// GetTransfer returns transfer info
func (stm *SecureTransferManager) GetTransfer(transferID string) (*SecureTransferInfo, bool) {
	stm.mu.RLock()
	defer stm.mu.RUnlock()
	t, ok := stm.transfers[transferID]
	if !ok {
		return nil, false
	}
	copy := *t
	return &copy, true
}

// ListTransfers returns all transfers
func (stm *SecureTransferManager) ListTransfers() []*SecureTransferInfo {
	stm.mu.RLock()
	defer stm.mu.RUnlock()

	result := make([]*SecureTransferInfo, 0, len(stm.transfers))
	for _, t := range stm.transfers {
		copy := *t
		result = append(result, &copy)
	}
	return result
}

// CleanupCompletedTransfers removes completed transfers older than duration
func (stm *SecureTransferManager) CleanupCompletedTransfers(maxAge time.Duration) int {
	stm.mu.Lock()

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for id, t := range stm.transfers {
		if t.State == SecureTransferCompleted && t.UpdatedAt.Before(cutoff) {
			delete(stm.transfers, id)
			removed++
		}
	}
	stm.mu.Unlock()

	if removed > 0 {
		if err := stm.saveState(); err != nil {
			log.Printf("failed to persist secure transfer cleanup: %v", err)
		}
	}

	return removed
}

// saveState persists transfer state
func (stm *SecureTransferManager) saveState() error {
	stm.mu.RLock()
	snapshot := make(map[string]*SecureTransferInfo, len(stm.transfers))
	for id, transfer := range stm.transfers {
		copy := *transfer
		snapshot[id] = &copy
	}
	stm.mu.RUnlock()

	path := filepath.Join(stm.dataDir, "transfers", "secure_state.json")
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, append(data, '\n'), 0600)
}

// loadState loads transfer state
func (stm *SecureTransferManager) loadState() {
	path := filepath.Join(stm.dataDir, "transfers", "secure_state.json")
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	loaded := make(map[string]*SecureTransferInfo)
	if err := json.NewDecoder(file).Decode(&loaded); err != nil {
		return
	}
	stm.mu.Lock()
	stm.transfers = loaded
	stm.mu.Unlock()
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".offgrid-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if n, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	} else if n != len(data) {
		tmp.Close()
		return io.ErrShortWrite
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

// ServeSecureTransfer starts a server to send a file securely
func (stm *SecureTransferManager) ServeSecureTransfer(filePath string, listener net.Listener) error {
	conn, err := listener.Accept()
	if err != nil {
		return err
	}
	defer conn.Close()

	// Read resume offset request
	var offset int64
	if err := binary.Read(conn, binary.BigEndian, &offset); err != nil {
		return fmt.Errorf("read resume offset: %w", err)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if offset < 0 || offset > info.Size() {
		return fmt.Errorf("invalid resume offset: %d", offset)
	}

	// Seek to offset
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return err
	}

	// Send encrypted chunks
	block, err := aes.NewCipher(stm.encryptionKeyCopy())
	if err != nil {
		return err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	buf := make([]byte, stm.chunkSize)
	nonce := make([]byte, gcm.NonceSize())

	for {
		n, err := file.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Generate nonce
		if _, err := rand.Read(nonce); err != nil {
			return err
		}

		// Encrypt
		encrypted := gcm.Seal(nonce, nonce, buf[:n], nil)

		// Send length and data
		if err := binary.Write(conn, binary.BigEndian, int32(len(encrypted))); err != nil {
			return err
		}
		if _, err := conn.Write(encrypted); err != nil {
			return err
		}
	}

	// End marker
	return binary.Write(conn, binary.BigEndian, int32(0))
}

// Helper function
func generateSecureTransferID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
