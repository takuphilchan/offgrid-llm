package sessions

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/documentocr"
)

type attachmentOCRPage struct {
	Version int             `json:"version"`
	Source  string          `json:"source_sha256"`
	Key     string          `json:"key"`
	SHA256  string          `json:"receipt_sha256"`
	Receipt json.RawMessage `json:"receipt"`
}

func attachmentOCRPageCount(root *os.Root, dir, id string) (int, error) {
	f, err := root.Open(dir)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	entries, err := f.ReadDir(-1)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), id+".ocr-") || strings.HasSuffix(entry.Name(), ".partial") {
			continue
		}
		page, err := strconv.Atoi(strings.TrimPrefix(entry.Name(), id+".ocr-"))
		info, statErr := entry.Info()
		if err != nil || page < 1 || page > documentocr.MaxCheckpointPages || statErr != nil || !info.Mode().IsRegular() {
			return 0, ErrAttachmentUnavailable
		}
		count++
	}
	if count > documentocr.MaxCheckpointPages {
		return 0, ErrAttachmentLimit
	}
	return count, nil
}

// AttachmentOCRCheckpoints retains page receipts under the source's current
// private authority. Closures do not confer authority after deletion/revocation.
func (s *ScopedManager) AttachmentOCRCheckpoints(name, id, source string) documentocr.Checkpoints {
	return documentocr.Checkpoints{
		Read: func(page int, key string) ([]byte, error) {
			return s.attachmentOCRCheckpoint(name, id, source, page, key, nil)
		},
		Write: func(page int, key string, raw []byte) error {
			_, err := s.attachmentOCRCheckpoint(name, id, source, page, key, raw)
			return err
		},
	}
}

func (s *ScopedManager) attachmentOCRCheckpoint(name, id, source string, page int, key string, raw []byte) ([]byte, error) {
	if !documentocr.ValidCheckpointKey(page, key) || raw != nil && (len(raw) == 0 || len(raw) > documentocr.MaxCheckpointBytes || !json.Valid(raw)) {
		return nil, documentocr.ErrCheckpointInvalid
	}
	s.manager.mutationMu.Lock()
	defer s.manager.mutationMu.Unlock()
	root, dir, err := s.attachmentRoot(name, false)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	record, err := readAttachmentRecord(root, dir, id)
	if err != nil || record.SHA256 != source || record.Status == "ready" {
		return nil, ErrAttachmentUnavailable
	}
	destination := fmt.Sprintf("%s/%s.ocr-%04d", dir, id, page)
	if raw == nil {
		info, err := root.Lstat(destination)
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		if err != nil || !info.Mode().IsRegular() || info.Size() > documentocr.MaxCheckpointBytes+1024 {
			return nil, documentocr.ErrCheckpointInvalid
		}
		f, err := root.Open(destination)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		data, err := io.ReadAll(io.LimitReader(f, documentocr.MaxCheckpointBytes+1025))
		var saved attachmentOCRPage
		if err != nil || len(data) > documentocr.MaxCheckpointBytes+1024 || json.Unmarshal(data, &saved) != nil || saved.Version != 1 || saved.Source != source || len(saved.Receipt) == 0 || len(saved.Receipt) > documentocr.MaxCheckpointBytes {
			return nil, documentocr.ErrCheckpointInvalid
		}
		sum := sha256.Sum256(saved.Receipt)
		if saved.SHA256 != hex.EncodeToString(sum[:]) {
			return nil, documentocr.ErrCheckpointInvalid
		}
		if saved.Key != key {
			return nil, nil
		}
		return saved.Receipt, nil
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		return nil, documentocr.ErrCheckpointInvalid
	}
	raw = compact.Bytes()
	sum := sha256.Sum256(raw)
	var envelope bytes.Buffer
	encoder := json.NewEncoder(&envelope)
	encoder.SetEscapeHTML(false) // Preserve the hashed worker receipt verbatim.
	if err := encoder.Encode(attachmentOCRPage{1, source, key, hex.EncodeToString(sum[:]), raw}); err != nil {
		return nil, err
	}
	data := envelope.Bytes()
	// Include interrupted writes; bounds cover every page for this source and
	// every retained source for the owner. No shared cache crosses user scopes.
	f, err := root.Open(dir)
	if err != nil {
		return nil, err
	}
	entries, readErr := f.ReadDir(-1)
	f.Close()
	if readErr != nil {
		return nil, readErr
	}
	var total, previous int64
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), id+".ocr-") {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			return nil, ErrAttachmentUnavailable
		}
		total += info.Size()
		if dir+"/"+entry.Name() == destination {
			previous = info.Size()
		}
	}
	if total-previous+int64(len(data)) > documentocr.MaxCheckpointSourceBytes {
		return nil, ErrAttachmentLimit
	}
	if err := s.checkAttachmentActorQuota(name, int64(len(data))-previous, maxActorAttachmentStructureBytes, ".ocr-"); err != nil {
		return nil, err
	}
	temp := destination + "." + rand.Text() + ".partial"
	out, err := root.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, err
	}
	defer root.Remove(temp)
	_, writeErr := out.Write(data)
	if err := errors.Join(writeErr, out.Sync(), out.Close()); err != nil {
		return nil, err
	}
	return nil, root.Rename(temp, destination)
}
