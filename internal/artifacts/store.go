// Package artifacts implements a local content-addressed store shared by model
// downloads, agent outputs, computer-use captures, and P2P transfer manifests.
package artifacts

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

var digestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type Metadata struct {
	Digest    string            `json:"digest"`
	Name      string            `json:"name,omitempty"`
	MediaType string            `json:"media_type,omitempty"`
	Size      int64             `json:"size"`
	CreatedAt time.Time         `json:"created_at"`
	Labels    map[string]string `json:"labels,omitempty"`
}

type Store struct {
	root string
}

func NewStore(root string) (*Store, error) {
	if root == "" {
		return nil, fmt.Errorf("artifact root is required")
	}
	if err := os.MkdirAll(filepath.Join(root, "sha256"), 0o755); err != nil {
		return nil, err
	}
	return &Store{root: root}, nil
}

func (s *Store) Put(ctx context.Context, reader io.Reader, metadata Metadata) (Metadata, error) {
	temporary, err := os.CreateTemp(filepath.Join(s.root, "sha256"), ".upload-*")
	if err != nil {
		return Metadata{}, err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	hash := sha256.New()
	written, err := copyContext(ctx, io.MultiWriter(temporary, hash), reader)
	if err == nil {
		err = temporary.Sync()
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return Metadata{}, err
	}
	digest := hex.EncodeToString(hash.Sum(nil))
	objectPath, err := s.objectPath(digest)
	if err != nil {
		return Metadata{}, err
	}
	if _, err := os.Stat(objectPath); os.IsNotExist(err) {
		if err := os.Rename(temporaryPath, objectPath); err != nil {
			// Another writer may have committed the same digest first.
			if _, statErr := os.Stat(objectPath); statErr != nil {
				return Metadata{}, err
			}
		}
	}
	metadataPath := objectPath + ".json"
	if encoded, err := os.ReadFile(metadataPath); err == nil {
		var stored Metadata
		if err := json.Unmarshal(encoded, &stored); err != nil {
			return Metadata{}, err
		}
		return stored, nil
	}
	metadata.Digest = digest
	metadata.Size = written
	if metadata.CreatedAt.IsZero() {
		metadata.CreatedAt = time.Now().UTC()
	}
	encoded, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return Metadata{}, err
	}
	if err := writeAtomic(metadataPath, encoded, 0o600); err != nil {
		// Content-addressed metadata is immutable. A concurrent writer that
		// won the race is a successful put of the same object.
		storedBytes, readErr := os.ReadFile(metadataPath)
		if readErr != nil {
			return Metadata{}, err
		}
		var stored Metadata
		if decodeErr := json.Unmarshal(storedBytes, &stored); decodeErr != nil {
			return Metadata{}, decodeErr
		}
		return stored, nil
	}
	return metadata, nil
}

func (s *Store) Open(digest string) (io.ReadCloser, Metadata, error) {
	objectPath, err := s.objectPath(digest)
	if err != nil {
		return nil, Metadata{}, err
	}
	encoded, err := os.ReadFile(objectPath + ".json")
	if err != nil {
		return nil, Metadata{}, err
	}
	var metadata Metadata
	if err := json.Unmarshal(encoded, &metadata); err != nil {
		return nil, Metadata{}, err
	}
	file, err := os.Open(objectPath)
	if err != nil {
		return nil, Metadata{}, err
	}
	return file, metadata, nil
}

func (s *Store) objectPath(digest string) (string, error) {
	if !digestPattern.MatchString(digest) {
		return "", fmt.Errorf("invalid sha256 digest")
	}
	return filepath.Join(s.root, "sha256", digest), nil
}

func copyContext(ctx context.Context, writer io.Writer, reader io.Reader) (int64, error) {
	buffer := make([]byte, 128*1024)
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		read, readErr := reader.Read(buffer)
		if read > 0 {
			written, writeErr := writer.Write(buffer[:read])
			total += int64(written)
			if writeErr != nil {
				return total, writeErr
			}
			if written != read {
				return total, io.ErrShortWrite
			}
		}
		if readErr == io.EOF {
			return total, nil
		}
		if readErr != nil {
			return total, readErr
		}
	}
}

func writeAtomic(path string, content []byte, mode os.FileMode) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".metadata-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(content); err != nil {
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
	return os.Rename(temporaryPath, path)
}
