package p2p

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const manifestVersion = 1

type Identity struct {
	private ed25519.PrivateKey
	public  ed25519.PublicKey
}

func LoadOrCreateIdentity(path string) (*Identity, error) {
	if encoded, err := os.ReadFile(path); err == nil {
		key, err := base64.StdEncoding.DecodeString(string(encoded))
		if err != nil || len(key) != ed25519.PrivateKeySize {
			return nil, fmt.Errorf("invalid P2P identity key")
		}
		private := ed25519.PrivateKey(key)
		return &Identity{private: private, public: private.Public().(ed25519.PublicKey)}, nil
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".identity-*")
	if err != nil {
		return nil, err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return nil, err
	}
	if _, err := temporary.WriteString(base64.StdEncoding.EncodeToString(private)); err != nil {
		temporary.Close()
		return nil, err
	}
	if err := temporary.Close(); err != nil {
		return nil, err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return nil, err
	}
	return &Identity{private: private, public: public}, nil
}

func (i *Identity) NodeID() string    { return nodeIDForPublicKey(i.public) }
func (i *Identity) PublicKey() string { return base64.StdEncoding.EncodeToString(i.public) }
func (i *Identity) sign(payload []byte) string {
	return base64.StdEncoding.EncodeToString(ed25519.Sign(i.private, payload))
}

type Manifest struct {
	Version   int       `json:"version"`
	NodeID    string    `json:"node_id"`
	PublicKey string    `json:"public_key"`
	Name      string    `json:"name"`
	Digest    string    `json:"sha256"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
	Signature string    `json:"signature"`
}

func BuildManifest(identity *Identity, path string) (Manifest, error) {
	if identity == nil {
		return Manifest{}, fmt.Errorf("P2P identity is required")
	}
	file, err := os.Open(path)
	if err != nil {
		return Manifest{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return Manifest{}, err
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return Manifest{}, err
	}
	manifest := Manifest{
		Version: manifestVersion, NodeID: identity.NodeID(), PublicKey: identity.PublicKey(),
		Name: filepath.Base(path), Digest: hex.EncodeToString(hash.Sum(nil)), Size: info.Size(), CreatedAt: info.ModTime().UTC(),
	}
	payload, _ := manifest.signingPayload()
	manifest.Signature = identity.sign(payload)
	return manifest, nil
}

func (m Manifest) Verify() error {
	if m.Version != manifestVersion || !validSharedName(m.Name) || m.Size < 0 || len(m.Digest) != sha256.Size*2 {
		return fmt.Errorf("invalid manifest metadata")
	}
	if _, err := hex.DecodeString(m.Digest); err != nil {
		return fmt.Errorf("invalid manifest digest")
	}
	public, err := base64.StdEncoding.DecodeString(m.PublicKey)
	if err != nil || len(public) != ed25519.PublicKeySize || nodeIDForPublicKey(public) != m.NodeID {
		return fmt.Errorf("manifest identity mismatch")
	}
	signature, err := base64.StdEncoding.DecodeString(m.Signature)
	if err != nil {
		return fmt.Errorf("invalid manifest signature")
	}
	payload, err := m.signingPayload()
	if err != nil || !ed25519.Verify(ed25519.PublicKey(public), payload, signature) {
		return fmt.Errorf("manifest signature verification failed")
	}
	return nil
}

func (m Manifest) signingPayload() ([]byte, error) {
	m.Signature = ""
	return json.Marshal(m)
}

func nodeIDForPublicKey(public []byte) string {
	digest := sha256.Sum256(public)
	return "node-" + hex.EncodeToString(digest[:16])
}

type TrustStore struct {
	mu      sync.RWMutex
	path    string
	trusted map[string]string
}

func NewTrustStore(path string) (*TrustStore, error) {
	store := &TrustStore{path: path, trusted: make(map[string]string)}
	if encoded, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(encoded, &store.trusted); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	return store, nil
}

func (s *TrustStore) Trust(nodeID, publicKey string) error {
	public, err := base64.StdEncoding.DecodeString(publicKey)
	if err != nil || len(public) != ed25519.PublicKeySize || nodeIDForPublicKey(public) != nodeID {
		return fmt.Errorf("peer identity does not match public key")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.trusted[nodeID] = publicKey
	return s.saveLocked()
}

func (s *TrustStore) IsTrusted(nodeID, publicKey string) bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.trusted[nodeID] == publicKey && publicKey != ""
}

func (s *TrustStore) Verify(manifest Manifest) error {
	if err := manifest.Verify(); err != nil {
		return err
	}
	if !s.IsTrusted(manifest.NodeID, manifest.PublicKey) {
		return fmt.Errorf("peer %s is not trusted", manifest.NodeID)
	}
	return nil
}

func (s *TrustStore) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(s.trusted, "", "  ")
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(s.path), ".trusted-peers-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(encoded); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, s.path)
}
