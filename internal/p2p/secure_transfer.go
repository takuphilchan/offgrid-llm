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
	"net"
	"os"
	"path/filepath"
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
	tempDir := filepath.Join(config.DataDir, "transfers", "temp")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
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
	} else {
		// Generate new key
		key = make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, err
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
	return hex.EncodeToString(stm.encryptionKey)
}

// SetEncryptionKey sets the encryption key from a peer
func (stm *SecureTransferManager) SetEncryptionKey(hexKey string) error {
	key, err := hex.DecodeString(hexKey)
	if err != nil || len(key) != 32 {
		return fmt.Errorf("invalid key: must be 32 bytes hex-encoded")
	}
	stm.encryptionKey = key
	return nil
}

// EncryptFile encrypts a file for secure transfer
func (stm *SecureTransferManager) EncryptFile(inputPath, outputPath string) error {
	block, err := aes.NewCipher(stm.encryptionKey)
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
	binary.Write(outFile, binary.BigEndian, int32(stm.chunkSize))

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
		binary.Write(outFile, binary.BigEndian, int32(len(encrypted)))
		outFile.Write(encrypted)
	}

	// Write end marker
	binary.Write(outFile, binary.BigEndian, int32(0))

	return nil
}

// DecryptFile decrypts a file received via secure transfer
func (stm *SecureTransferManager) DecryptFile(inputPath, outputPath string) error {
	block, err := aes.NewCipher(stm.encryptionKey)
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
	binary.Read(inFile, binary.BigEndian, &chunkSize)

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

		outFile.Write(plaintext)
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

	stm.saveState()
	return transfer, nil
}

// ResumeTransfer gets the offset for resuming a transfer
func (stm *SecureTransferManager) ResumeTransfer(transferID string) (int64, error) {
	stm.mu.RLock()
	transfer, ok := stm.transfers[transferID]
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
	stm.mu.Lock()
	transfer, ok := stm.transfers[transferID]
	stm.mu.Unlock()

	if !ok {
		return fmt.Errorf("transfer not found")
	}

	partialPath := filepath.Join(stm.tempDir, transferID+".partial")

	// Open file for writing at offset
	file, err := os.OpenFile(partialPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// Seek to offset
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return err
	}

	// Write data
	if _, err := file.Write(data); err != nil {
		return err
	}

	// Update transfer state
	stm.mu.Lock()
	transfer.TransferredSize = offset + int64(len(data))
	transfer.UpdatedAt = time.Now()
	if isLast {
		transfer.State = SecureTransferCompleted
	} else {
		transfer.State = SecureTransferActive
	}
	stm.mu.Unlock()

	if isLast {
		// Move to final location
		finalPath := filepath.Join(stm.dataDir, "models", transfer.Filename)
		os.Rename(partialPath, finalPath)
	}

	stm.saveState()
	return nil
}

// PauseTransfer pauses an active transfer
func (stm *SecureTransferManager) PauseTransfer(transferID string) error {
	stm.mu.Lock()
	defer stm.mu.Unlock()

	transfer, ok := stm.transfers[transferID]
	if !ok {
		return fmt.Errorf("transfer not found")
	}

	transfer.State = SecureTransferPaused
	transfer.UpdatedAt = time.Now()
	stm.saveState()
	return nil
}

// GetTransfer returns transfer info
func (stm *SecureTransferManager) GetTransfer(transferID string) (*SecureTransferInfo, bool) {
	stm.mu.RLock()
	defer stm.mu.RUnlock()
	t, ok := stm.transfers[transferID]
	return t, ok
}

// ListTransfers returns all transfers
func (stm *SecureTransferManager) ListTransfers() []*SecureTransferInfo {
	stm.mu.RLock()
	defer stm.mu.RUnlock()

	result := make([]*SecureTransferInfo, 0, len(stm.transfers))
	for _, t := range stm.transfers {
		result = append(result, t)
	}
	return result
}

// CleanupCompletedTransfers removes completed transfers older than duration
func (stm *SecureTransferManager) CleanupCompletedTransfers(maxAge time.Duration) int {
	stm.mu.Lock()
	defer stm.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for id, t := range stm.transfers {
		if t.State == SecureTransferCompleted && t.UpdatedAt.Before(cutoff) {
			delete(stm.transfers, id)
			removed++
		}
	}

	if removed > 0 {
		stm.saveState()
	}

	return removed
}

// saveState persists transfer state
func (stm *SecureTransferManager) saveState() {
	stm.mu.RLock()
	defer stm.mu.RUnlock()

	path := filepath.Join(stm.dataDir, "transfers", "secure_state.json")
	os.MkdirAll(filepath.Dir(path), 0755)

	file, err := os.Create(path)
	if err != nil {
		return
	}
	defer file.Close()

	json.NewEncoder(file).Encode(stm.transfers)
}

// loadState loads transfer state
func (stm *SecureTransferManager) loadState() {
	path := filepath.Join(stm.dataDir, "transfers", "secure_state.json")
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	json.NewDecoder(file).Decode(&stm.transfers)
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
	binary.Read(conn, binary.BigEndian, &offset)

	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Seek to offset
	file.Seek(offset, io.SeekStart)

	// Send encrypted chunks
	block, err := aes.NewCipher(stm.encryptionKey)
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
		rand.Read(nonce)

		// Encrypt
		encrypted := gcm.Seal(nonce, nonce, buf[:n], nil)

		// Send length and data
		binary.Write(conn, binary.BigEndian, int32(len(encrypted)))
		conn.Write(encrypted)
	}

	// End marker
	binary.Write(conn, binary.BigEndian, int32(0))

	return nil
}

// Helper function
func generateSecureTransferID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
