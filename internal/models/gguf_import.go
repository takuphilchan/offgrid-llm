package models

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

var ErrGGUFImportInvalid = errors.New("invalid single-file GGUF import")
var ErrGGUFImportExists = errors.New("model filename already exists")
var ErrGGUFImportBusy = errors.New("another model import is active")
var splitGGUFName = regexp.MustCompile(`(?i)-\d{5}-of-\d{5}\.gguf$`)

// GGUFImportReceipt records storage only, not runtime or model qualification.
type GGUFImportReceipt struct {
	ModelID            string `json:"model_id"`
	Filename           string `json:"filename"`
	Size               int64  `json:"size"`
	SHA256             string `json:"sha256"`
	InventoryRefreshed bool   `json:"inventory_refreshed"`
}

// ImportGGUF streams user-owned weights into a private staging file, then publishes
// without replacement. It never selects/loads a model or acquires companion files.
func (r *Registry) ImportGGUF(ctx context.Context, name string, size int64, input io.Reader) (GGUFImportReceipt, error) {
	var receipt GGUFImportReceipt
	if input == nil {
		return receipt, ErrGGUFImportInvalid
	}
	if !validArtifactPath(name) || strings.Contains(name, "/") || !strings.HasSuffix(name, ".gguf") ||
		isProjectorFilename(name) || auxiliaryGGUF(name) || size < 1<<20 || size > MaxPackageBytes {
		return receipt, ErrGGUFImportInvalid
	}
	// Split GGUFs are not independently usable. Do not install a partial bundle.
	if splitGGUFName.MatchString(name) {
		return receipt, ErrGGUFImportInvalid
	}
	if err := ctx.Err(); err != nil {
		return receipt, err
	}
	if !r.importMu.TryLock() {
		return receipt, ErrGGUFImportBusy
	}
	defer r.importMu.Unlock()
	if err := r.ScanModels(); err != nil {
		return receipt, err
	}
	for _, existing := range r.ListModels() {
		if strings.EqualFold(existing.ID, strings.TrimSuffix(name, ".gguf")) {
			return receipt, ErrGGUFImportExists
		}
	}
	if err := os.MkdirAll(r.modelsDir, 0755); err != nil {
		return receipt, err
	}
	root, err := os.OpenRoot(r.modelsDir)
	if err != nil {
		return receipt, err
	}
	defer root.Close()
	dir, err := root.Open(".")
	if err != nil {
		return receipt, err
	}
	names, err := dir.Readdirnames(-1)
	dir.Close()
	if err != nil {
		return receipt, err
	}
	for _, existing := range names {
		if strings.EqualFold(existing, name) {
			return receipt, ErrGGUFImportExists
		}
	}
	stage := ".offgrid-import-" + rand.Text() + ".tmp"
	f, err := root.OpenFile(stage, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		return receipt, err
	}
	defer func() { f.Close(); root.Remove(stage) }()
	hash := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, hash), io.LimitReader(&importContextReader{ctx, input}, size+1))
	if err != nil {
		return receipt, err
	}
	if n != size {
		return receipt, ErrGGUFImportInvalid
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return receipt, err
	}
	var header [24]byte
	if _, err = io.ReadFull(f, header[:]); err != nil {
		return receipt, ErrGGUFImportInvalid
	}
	version := binary.LittleEndian.Uint32(header[4:8])
	tensors := binary.LittleEndian.Uint64(header[8:16])
	if string(header[:4]) != "GGUF" || version < 2 || version > 3 || tensors == 0 || tensors > 1_000_000 {
		return receipt, ErrGGUFImportInvalid
	}
	if err = checkImportedGGUFMetadata(f, binary.LittleEndian.Uint64(header[16:24])); err != nil {
		return receipt, ErrGGUFImportInvalid
	}
	if err = f.Sync(); err != nil {
		return receipt, err
	}
	if err = f.Close(); err != nil {
		return receipt, err
	}
	if err = ctx.Err(); err != nil {
		return receipt, err
	}
	// A hard link provides atomic no-replace publication on supported local
	// filesystems. If unavailable, fail safely instead of falling back to overwrite.
	if err = root.Link(stage, name); err != nil {
		if errors.Is(err, os.ErrExist) {
			return receipt, ErrGGUFImportExists
		}
		return receipt, fmt.Errorf("publish imported model: %w", err)
	}
	receipt = GGUFImportReceipt{ModelID: strings.TrimSuffix(name, ".gguf"), Filename: name, Size: n, SHA256: hex.EncodeToString(hash.Sum(nil))}
	// Once published, always return the receipt. A refresh problem must not turn
	// a completed write into an ambiguous failure inviting another upload.
	receipt.InventoryRefreshed = r.ScanModels() == nil
	return receipt, nil
}

// Parse the whole bounded metadata region: context admission's early exit would
// miss a renamed shard's split metadata. This is not tensor/runtime qualification.
func checkImportedGGUFMetadata(f *os.File, count uint64) error {
	if count == 0 || count > 1_000_000 {
		return ErrGGUFImportInvalid
	}
	architecture := ""
	for i := uint64(0); i < count; i++ {
		position, err := f.Seek(0, io.SeekCurrent)
		if err != nil || position > ggufMetadataScanLimit {
			return ErrGGUFImportInvalid
		}
		key, err := readGGUFString(f)
		if err != nil || strings.HasPrefix(key, "split.") {
			return ErrGGUFImportInvalid
		}
		kind, err := readU32(f)
		if err != nil {
			return err
		}
		value, err := readGGUFValue(f, kind, key == "general.architecture", 0)
		if err != nil {
			return err
		}
		if key == "general.architecture" {
			if architecture != "" {
				return ErrGGUFImportInvalid
			}
			architecture, _ = value.(string)
		}
	}
	position, err := f.Seek(0, io.SeekCurrent)
	info, statErr := f.Stat()
	if err != nil || statErr != nil || position >= info.Size() || position > ggufMetadataScanLimit || strings.TrimSpace(architecture) == "" || architecture == "clip" {
		return ErrGGUFImportInvalid
	}
	return nil
}

type importContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *importContextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}
