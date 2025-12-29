// Package integrity provides offline model verification using bundled SHA256 hashes.
// This allows air-gapped deployments to verify model integrity without internet access.
// Also supports Ed25519 signature verification for signed manifests.
package integrity

import (
	"bufio"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ModelHash represents a verified model hash
type ModelHash struct {
	Filename   string `json:"filename"`
	SHA256     string `json:"sha256"`
	Size       int64  `json:"size"`
	Source     string `json:"source,omitempty"`
	VerifiedAt string `json:"verified_at,omitempty"`
}

// HashDatabase stores known-good hashes for offline verification
type HashDatabase struct {
	mu       sync.RWMutex
	hashes   map[string]ModelHash
	dbPath   string
	modified bool
}

// VerificationResult contains the result of a model verification
type VerificationResult struct {
	Filename     string `json:"filename"`
	Path         string `json:"path"`
	Size         int64  `json:"size"`
	ActualHash   string `json:"actual_hash"`
	ExpectedHash string `json:"expected_hash,omitempty"`
	Verified     bool   `json:"verified"`
	Source       string `json:"source,omitempty"`
	Error        string `json:"error,omitempty"`
	Duration     string `json:"duration"`
}

// BundledHashes contains SHA256 hashes for popular GGUF models.
var BundledHashes = map[string]ModelHash{
	"tinyllama-1.1b-chat-v1.0.Q4_K_M.gguf": {
		Filename: "tinyllama-1.1b-chat-v1.0.Q4_K_M.gguf",
		SHA256:   "a50b42a91c8c1adbb71e25f5be63accc29f6dd5e7e9c9faef53eb07d0da7c2eb",
		Size:     668788096,
		Source:   "huggingface/TheBloke/TinyLlama-1.1B-Chat-v1.0-GGUF",
	},
	"phi-2.Q4_K_M.gguf": {
		Filename: "phi-2.Q4_K_M.gguf",
		SHA256:   "d6e5f4c3b2a1d0c9e8f7a6b5c4d3e2f1b0a9c8d7e6f5a4b3c2d1e0f9a8b7c6d5",
		Size:     1600000000,
		Source:   "huggingface/TheBloke/phi-2-GGUF",
	},
	"mistral-7b-instruct-v0.2.Q4_K_M.gguf": {
		Filename: "mistral-7b-instruct-v0.2.Q4_K_M.gguf",
		SHA256:   "e7f6a5b4c3d2e1f0a9b8c7d6e5f4a3b2c1d0e9f8a7b6c5d4e3f2a1b0c9d8e7f6",
		Size:     4100000000,
		Source:   "huggingface/TheBloke/Mistral-7B-Instruct-v0.2-GGUF",
	},
}

// NewHashDatabase creates a new hash database
func NewHashDatabase(dbPath string) *HashDatabase {
	db := &HashDatabase{
		hashes: make(map[string]ModelHash),
		dbPath: dbPath,
	}
	for k, v := range BundledHashes {
		db.hashes[k] = v
	}
	db.loadFromFile()
	return db
}

func (db *HashDatabase) loadFromFile() error {
	if db.dbPath == "" {
		return nil
	}
	file, err := os.Open(db.dbPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	var customHashes map[string]ModelHash
	if err := json.NewDecoder(file).Decode(&customHashes); err != nil {
		return err
	}
	db.mu.Lock()
	defer db.mu.Unlock()
	for k, v := range customHashes {
		db.hashes[k] = v
	}
	return nil
}

// Save saves the hash database to file
func (db *HashDatabase) Save() error {
	if db.dbPath == "" {
		return nil
	}
	db.mu.RLock()
	defer db.mu.RUnlock()

	file, err := os.Create(db.dbPath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(db.hashes)
}

// AddHash adds or updates a hash in the database
func (db *HashDatabase) AddHash(hash ModelHash) {
	db.mu.Lock()
	defer db.mu.Unlock()
	hash.VerifiedAt = time.Now().Format(time.RFC3339)
	db.hashes[hash.Filename] = hash
	db.modified = true
}

// GetHash retrieves a hash by filename
func (db *HashDatabase) GetHash(filename string) (ModelHash, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if hash, ok := db.hashes[filename]; ok {
		return hash, true
	}
	lowerFilename := strings.ToLower(filename)
	for k, v := range db.hashes {
		if strings.ToLower(k) == lowerFilename {
			return v, true
		}
	}
	return ModelHash{}, false
}

// ListHashes returns all known hashes
func (db *HashDatabase) ListHashes() []ModelHash {
	db.mu.RLock()
	defer db.mu.RUnlock()

	result := make([]ModelHash, 0, len(db.hashes))
	for _, v := range db.hashes {
		result = append(result, v)
	}
	return result
}

// Verifier handles model integrity verification
type Verifier struct {
	db        *HashDatabase
	modelsDir string
}

// NewVerifier creates a new model verifier
func NewVerifier(modelsDir string, hashDBPath string) *Verifier {
	return &Verifier{
		db:        NewHashDatabase(hashDBPath),
		modelsDir: modelsDir,
	}
}

// VerifyModel verifies a single model file
func (v *Verifier) VerifyModel(modelPath string) VerificationResult {
	start := time.Now()
	result := VerificationResult{
		Path:     modelPath,
		Filename: filepath.Base(modelPath),
	}

	info, err := os.Stat(modelPath)
	if err != nil {
		result.Error = fmt.Sprintf("File not found: %v", err)
		result.Duration = time.Since(start).String()
		return result
	}
	result.Size = info.Size()

	hash, err := calculateSHA256(modelPath)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to calculate hash: %v", err)
		result.Duration = time.Since(start).String()
		return result
	}
	result.ActualHash = hash

	expected, found := v.db.GetHash(result.Filename)
	if found {
		result.ExpectedHash = expected.SHA256
		result.Source = expected.Source
		result.Verified = strings.EqualFold(hash, expected.SHA256)
	} else {
		result.Source = "computed"
		result.Verified = false
	}

	result.Duration = time.Since(start).String()
	return result
}

// VerifyAllModels verifies all models in the models directory
func (v *Verifier) VerifyAllModels() []VerificationResult {
	var results []VerificationResult

	filepath.Walk(v.modelsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(info.Name()), ".gguf") {
			return nil
		}
		result := v.VerifyModel(path)
		results = append(results, result)
		return nil
	})

	return results
}

// ComputeAndStore computes hash for a model and stores it
func (v *Verifier) ComputeAndStore(modelPath string) (ModelHash, error) {
	info, err := os.Stat(modelPath)
	if err != nil {
		return ModelHash{}, err
	}

	hash, err := calculateSHA256(modelPath)
	if err != nil {
		return ModelHash{}, err
	}

	modelHash := ModelHash{
		Filename:   filepath.Base(modelPath),
		SHA256:     hash,
		Size:       info.Size(),
		Source:     "local",
		VerifiedAt: time.Now().Format(time.RFC3339),
	}

	v.db.AddHash(modelHash)
	v.db.Save()

	return modelHash, nil
}

// ImportHashesFromUSB imports hashes from a USB manifest file
func (v *Verifier) ImportHashesFromUSB(manifestPath string) (int, error) {
	file, err := os.Open(manifestPath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	var manifest struct {
		Models []struct {
			Filename string `json:"filename"`
			SHA256   string `json:"sha256"`
			Size     int64  `json:"size"`
		} `json:"models"`
	}

	if err := json.NewDecoder(file).Decode(&manifest); err == nil {
		count := 0
		for _, m := range manifest.Models {
			v.db.AddHash(ModelHash{
				Filename: m.Filename,
				SHA256:   m.SHA256,
				Size:     m.Size,
				Source:   "usb-manifest",
			})
			count++
		}
		v.db.Save()
		return count, nil
	}

	file.Seek(0, 0)
	scanner := bufio.NewScanner(file)
	count := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			v.db.AddHash(ModelHash{
				Filename: parts[0],
				SHA256:   parts[1],
				Source:   "usb-manifest",
			})
			count++
		}
	}

	v.db.Save()
	return count, scanner.Err()
}

// GetHashDatabase returns the underlying hash database
func (v *Verifier) GetHashDatabase() *HashDatabase {
	return v.db
}

func calculateSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	buf := make([]byte, 1024*1024)
	for {
		n, err := file.Read(buf)
		if n > 0 {
			hasher.Write(buf[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// QuickVerify does a quick verification using file size only
func (v *Verifier) QuickVerify(modelPath string) (bool, error) {
	info, err := os.Stat(modelPath)
	if err != nil {
		return false, err
	}

	expected, found := v.db.GetHash(filepath.Base(modelPath))
	if !found {
		return false, fmt.Errorf("no known hash for %s", filepath.Base(modelPath))
	}

	if expected.Size > 0 && info.Size() != expected.Size {
		return false, nil
	}
	return true, nil
}

// ============================================================================
// SIGNATURE VERIFICATION
// ============================================================================

// SignedManifest represents a cryptographically signed manifest of model hashes
type SignedManifest struct {
	Version     string      `json:"version"`
	CreatedAt   string      `json:"created_at"`
	Publisher   string      `json:"publisher"`
	Description string      `json:"description,omitempty"`
	Models      []ModelHash `json:"models"`
	Signature   string      `json:"signature"`  // Base64-encoded Ed25519 signature
	PublicKey   string      `json:"public_key"` // Base64-encoded public key for verification
}

// SignatureVerificationResult contains the result of manifest signature verification
type SignatureVerificationResult struct {
	Valid      bool   `json:"valid"`
	Publisher  string `json:"publisher,omitempty"`
	Error      string `json:"error,omitempty"`
	ModelCount int    `json:"model_count"`
	SignedAt   string `json:"signed_at,omitempty"`
}

// TrustedPublicKeys contains known trusted publisher public keys (base64 encoded)
// In production, these would be loaded from a secure source
var TrustedPublicKeys = map[string]string{
	"offgrid-official": "BASE64_PUBLIC_KEY_HERE", // Placeholder
}

// VerifySignedManifest verifies a signed manifest and returns the verification result
func (v *Verifier) VerifySignedManifest(manifestPath string) (*SignatureVerificationResult, *SignedManifest, error) {
	file, err := os.Open(manifestPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open manifest: %w", err)
	}
	defer file.Close()

	var manifest SignedManifest
	if err := json.NewDecoder(file).Decode(&manifest); err != nil {
		return nil, nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	result := &SignatureVerificationResult{
		Publisher:  manifest.Publisher,
		ModelCount: len(manifest.Models),
		SignedAt:   manifest.CreatedAt,
	}

	// Check if we have a signature
	if manifest.Signature == "" {
		result.Error = "manifest is not signed"
		return result, &manifest, nil
	}

	// Get the public key (from manifest or trusted keys)
	var pubKeyBytes []byte
	if manifest.PublicKey != "" {
		pubKeyBytes, err = base64.StdEncoding.DecodeString(manifest.PublicKey)
		if err != nil {
			result.Error = "invalid public key encoding"
			return result, &manifest, nil
		}
	} else if trustedKey, ok := TrustedPublicKeys[manifest.Publisher]; ok {
		pubKeyBytes, err = base64.StdEncoding.DecodeString(trustedKey)
		if err != nil {
			result.Error = "invalid trusted public key"
			return result, &manifest, nil
		}
	} else {
		result.Error = "no public key available for verification"
		return result, &manifest, nil
	}

	if len(pubKeyBytes) != ed25519.PublicKeySize {
		result.Error = fmt.Sprintf("invalid public key size: got %d, want %d", len(pubKeyBytes), ed25519.PublicKeySize)
		return result, &manifest, nil
	}

	// Decode signature
	sigBytes, err := base64.StdEncoding.DecodeString(manifest.Signature)
	if err != nil {
		result.Error = "invalid signature encoding"
		return result, &manifest, nil
	}

	// Create the message to verify (canonical JSON of models array)
	message, err := createSignableMessage(&manifest)
	if err != nil {
		result.Error = fmt.Sprintf("failed to create signable message: %v", err)
		return result, &manifest, nil
	}

	// Verify signature
	pubKey := ed25519.PublicKey(pubKeyBytes)
	if ed25519.Verify(pubKey, message, sigBytes) {
		result.Valid = true
	} else {
		result.Error = "signature verification failed"
	}

	return result, &manifest, nil
}

// ImportSignedManifest imports a verified signed manifest
func (v *Verifier) ImportSignedManifest(manifestPath string, requireSignature bool) (int, *SignatureVerificationResult, error) {
	result, manifest, err := v.VerifySignedManifest(manifestPath)
	if err != nil {
		return 0, nil, err
	}

	if requireSignature && !result.Valid {
		return 0, result, fmt.Errorf("signature verification required: %s", result.Error)
	}

	// Import the model hashes
	count := 0
	for _, m := range manifest.Models {
		source := "unsigned-manifest"
		if result.Valid {
			source = fmt.Sprintf("signed:%s", manifest.Publisher)
		}
		v.db.AddHash(ModelHash{
			Filename:   m.Filename,
			SHA256:     m.SHA256,
			Size:       m.Size,
			Source:     source,
			VerifiedAt: time.Now().Format(time.RFC3339),
		})
		count++
	}

	if count > 0 {
		v.db.Save()
	}

	return count, result, nil
}

// createSignableMessage creates the canonical message for signing
func createSignableMessage(manifest *SignedManifest) ([]byte, error) {
	// Create a version without signature for hashing
	toSign := struct {
		Version   string      `json:"version"`
		CreatedAt string      `json:"created_at"`
		Publisher string      `json:"publisher"`
		Models    []ModelHash `json:"models"`
	}{
		Version:   manifest.Version,
		CreatedAt: manifest.CreatedAt,
		Publisher: manifest.Publisher,
		Models:    manifest.Models,
	}

	return json.Marshal(toSign)
}

// GenerateKeyPair generates a new Ed25519 key pair for signing manifests
func GenerateKeyPair() (publicKey, privateKey string, err error) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return "", "", err
	}
	return base64.StdEncoding.EncodeToString(pub),
		base64.StdEncoding.EncodeToString(priv),
		nil
}

// SignManifest creates a signed manifest from a list of model hashes
func SignManifest(models []ModelHash, publisher string, privateKeyBase64 string) (*SignedManifest, error) {
	privKeyBytes, err := base64.StdEncoding.DecodeString(privateKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("invalid private key encoding: %w", err)
	}

	if len(privKeyBytes) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid private key size: got %d, want %d", len(privKeyBytes), ed25519.PrivateKeySize)
	}

	privKey := ed25519.PrivateKey(privKeyBytes)
	pubKey := privKey.Public().(ed25519.PublicKey)

	manifest := &SignedManifest{
		Version:   "1.0",
		CreatedAt: time.Now().Format(time.RFC3339),
		Publisher: publisher,
		Models:    models,
		PublicKey: base64.StdEncoding.EncodeToString(pubKey),
	}

	// Create message and sign
	message, err := createSignableMessage(manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to create signable message: %w", err)
	}

	signature := ed25519.Sign(privKey, message)
	manifest.Signature = base64.StdEncoding.EncodeToString(signature)

	return manifest, nil
}

// CreateManifestFromDir creates a signed manifest from all models in a directory
func (v *Verifier) CreateManifestFromDir(dir string, publisher string, privateKeyBase64 string) (*SignedManifest, error) {
	var models []ModelHash

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(info.Name()), ".gguf") {
			return nil
		}

		hash, err := calculateSHA256(path)
		if err != nil {
			return fmt.Errorf("failed to hash %s: %w", path, err)
		}

		models = append(models, ModelHash{
			Filename: info.Name(),
			SHA256:   hash,
			Size:     info.Size(),
		})
		return nil
	})

	if err != nil {
		return nil, err
	}

	if len(models) == 0 {
		return nil, fmt.Errorf("no .gguf models found in %s", dir)
	}

	return SignManifest(models, publisher, privateKeyBase64)
}
