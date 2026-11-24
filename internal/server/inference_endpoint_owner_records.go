package server

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/jsonwire"
)

const endpointOwnerJournalFilename = "inference-endpoint-owner-journal.json"

var errEndpointOwnerUncertain = errors.New("endpoint configuration publication is uncertain; reconcile actual stored state")

type endpointOwnerRecord struct {
	RequestID     string `json:"request_id"`
	ReviewID      string `json:"review_id"`
	Actor         string `json:"actor"`
	Epoch         string `json:"epoch"`
	Before        string `json:"before"`
	After         string `json:"after"`
	Endpoint      string `json:"endpoint"`
	State         string `json:"state"` // pending, published, committed, not_applied, conflict
	CreatedAt     string `json:"created_at"`
	ResolvedAt    string `json:"resolved_at,omitempty"`
	ResolvedBy    string `json:"resolved_by,omitempty"`
	ResolvedEpoch string `json:"resolved_epoch,omitempty"`
}

type endpointOwnerJournal struct {
	Version   int                   `json:"version"`
	Records   []endpointOwnerRecord `json:"records"`
	ArchiveID string                `json:"archive_id,omitempty"`
}

type endpointOwnerDraft struct {
	ID            string
	Actor         string
	Epoch         string
	Expires       time.Time
	Expected      string
	HostPin       string
	Configuration inferenceEndpointFile
	Entry         inferenceEndpointEnrollment
}

type endpointOwner struct {
	mu     sync.Mutex
	drafts map[string]endpointOwnerDraft
	fault  error
	write  func(string, any) error // Instance-local fault injection; nil uses storage.WriteDurableJSON.
}

func endpointOwnerNonce() (string, error) {
	var bytes [32]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes[:]), nil
}

func endpointOwnerDigest(bytes []byte) string {
	digest := sha256.Sum256(bytes)
	return hex.EncodeToString(digest[:])
}

// This shared bounded opener is also used for host policy and durable owner
// records. It never echoes input contents or secret-bearing parse errors.
func readEndpointOwnerJSON(path string, limit int, value any) ([]byte, error) {
	if !filepath.IsAbs(path) {
		return nil, errors.New("endpoint metadata requires an absolute path")
	}
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() || before.Size() > int64(limit) {
		return nil, errors.New("endpoint metadata must be a bounded regular file")
	}
	file, err := openObservedInferenceEndpointFile(path, before)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, int64(limit)+1))
	if err != nil || len(raw) > limit || jsonwire.Validate(raw) != nil {
		return nil, errors.New("endpoint metadata is invalid")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(value) != nil {
		return nil, errors.New("endpoint metadata is invalid")
	}
	return raw, nil
}

func readEndpointOwnerJournal(directory string) (endpointOwnerJournal, error) {
	var journal endpointOwnerJournal
	_, err := readEndpointOwnerJSON(filepath.Join(directory, endpointOwnerJournalFilename), 64<<10, &journal)
	if errors.Is(err, os.ErrNotExist) {
		if _, archiveErr := os.Lstat(filepath.Join(directory, endpointOwnerArchiveMarker)); !errors.Is(archiveErr, os.ErrNotExist) {
			return journal, errors.New("endpoint journal is missing from an archived workspace")
		}
		return endpointOwnerJournal{Version: 1, Records: []endpointOwnerRecord{}}, nil
	}
	if err != nil {
		return journal, err
	}
	if err := validateEndpointOwnerJournal(journal); err != nil {
		return journal, err
	}
	if _, err := readEndpointOwnerArchiveID(directory, journal.ArchiveID); err != nil {
		return journal, err
	}
	return journal, nil
}

func validateEndpointOwnerJournal(journal endpointOwnerJournal) error {
	if (journal.Version != 1 && journal.Version != 2) || len(journal.Records) > 64 || journal.Version == 1 && journal.ArchiveID != "" || journal.Version == 2 && !validEndpointOwnerDigest(journal.ArchiveID) {
		return errors.New("endpoint owner journal is invalid")
	}
	seen := map[string]bool{}
	for _, record := range journal.Records {
		identity := strings.Split(record.Endpoint, "@")
		_, createdErr := time.Parse(time.RFC3339Nano, record.CreatedAt)
		if !inferenceEndpointName.MatchString(record.RequestID) || !validEndpointOwnerDigest(record.ReviewID) || record.Actor == "" || len(record.Actor) > 256 || len(identity) != 2 || !inferenceEndpointName.MatchString(identity[0]) || !inferenceEndpointName.MatchString(identity[1]) || createdErr != nil || !validEndpointOwnerDigest(record.Epoch) || !validEndpointOwnerDigest(record.Before) || !validEndpointOwnerDigest(record.After) || seen[record.RequestID] {
			return errors.New("endpoint owner journal identity is invalid")
		}
		switch record.State {
		case "pending", "published":
			if record.ResolvedAt != "" || record.ResolvedBy != "" || record.ResolvedEpoch != "" {
				return errors.New("unresolved endpoint owner journal has resolution fields")
			}
		case "committed", "not_applied", "conflict":
			_, resolvedErr := time.Parse(time.RFC3339Nano, record.ResolvedAt)
			if resolvedErr != nil || record.ResolvedBy == "" || len(record.ResolvedBy) > 256 || !validEndpointOwnerDigest(record.ResolvedEpoch) {
				return errors.New("endpoint owner journal resolution is invalid")
			}
		default:
			return errors.New("endpoint owner journal outcome is invalid")
		}
		seen[record.RequestID] = true
	}
	return nil
}

func validEndpointOwnerDigest(value string) bool {
	decoded, err := hex.DecodeString(value)
	return len(value) == 64 && err == nil && hex.EncodeToString(decoded) == value
}

func endpointOwnerJournalReady(directory string) error {
	journal, err := readEndpointOwnerJournal(directory)
	if err != nil {
		return err
	}
	for _, record := range journal.Records {
		if record.State == "pending" || record.State == "published" || record.State == "conflict" {
			return errEndpointOwnerUncertain
		}
	}
	return nil
}
