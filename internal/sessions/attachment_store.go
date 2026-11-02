package sessions

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode"
)

const MaxAttachmentBytes int64 = 50 << 20
const maxSessionAttachmentBytes int64 = 100 << 20
const maxAttachmentTextBytes = 16 << 20

var ErrAttachmentUnavailable = errors.New("attachment unavailable")
var ErrAttachmentLimit = errors.New("attachment storage limit reached")
var attachmentID = regexp.MustCompile(`^file-[a-f0-9]{32}$`)

// Attachment is safe for lists. Original bytes and extracted text are separate
// scoped resources, never copied into an ordinary session-list response.
type Attachment struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	MediaType string    `json:"media_type"`
	Bytes     int64     `json:"bytes"`
	SHA256    string    `json:"sha256"`
	Status    string    `json:"status"`
	Revision  string    `json:"revision,omitempty"`
	ErrorCode string    `json:"error_code,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type attachmentRecord struct {
	Schema int `json:"schema"`
	Attachment
	Text   string `json:"text,omitempty"`
	Parser string `json:"parser,omitempty"`
}

// Including the session creation identity prevents deleted/recreated names from
// inheriting old files. The enclosing caller rechecks ownership under the same
// lock as every mutation; knowledge publication is deliberately not involved.
func (s *ScopedManager) attachmentRoot(name string, create bool) (*os.Root, string, error) {
	session, err := s.Load(name)
	if err != nil {
		return nil, "", err
	}
	key := sha256.Sum256([]byte(session.OwnerID + "\x00" + session.Name + "\x00" + session.CreatedAt.UTC().Format(time.RFC3339Nano)))
	dir := fmt.Sprintf(".attachments/%x", key)
	root, err := os.OpenRoot(s.manager.sessionsDir)
	if err != nil {
		return nil, "", err
	}
	for _, component := range []string{".attachments", dir} {
		if create {
			if err := root.Mkdir(component, 0700); err != nil && !errors.Is(err, os.ErrExist) {
				root.Close()
				return nil, "", err
			}
		}
		info, err := root.Lstat(component)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			root.Close()
			return nil, "", errors.Join(ErrAttachmentUnavailable, err)
		}
	}
	return root, dir, nil
}

func readAttachmentRecord(root *os.Root, dir, id string) (attachmentRecord, error) {
	var record attachmentRecord
	if !attachmentID.MatchString(id) {
		return record, ErrAttachmentUnavailable
	}
	p := dir + "/" + id + ".json"
	info, err := root.Lstat(p)
	if err != nil || !info.Mode().IsRegular() {
		return record, ErrAttachmentUnavailable
	}
	f, err := root.Open(p)
	if err != nil {
		return record, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxAttachmentTextBytes+65537))
	if err != nil || len(b) > maxAttachmentTextBytes+65536 || json.Unmarshal(b, &record) != nil || record.Schema != 1 || record.ID != id {
		return record, ErrAttachmentUnavailable
	}
	return record, nil
}

func writeAttachmentRecord(root *os.Root, dir string, record attachmentRecord) error {
	b, err := json.Marshal(record)
	if err != nil || len(b) > maxAttachmentTextBytes+65536 {
		return ErrAttachmentLimit
	}
	temp := dir + "/.record-" + rand.Text()
	f, err := root.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer root.Remove(temp)
	_, writeErr := f.Write(b)
	err = errors.Join(writeErr, f.Sync(), f.Close())
	if err != nil {
		return err
	}
	return root.Rename(temp, dir+"/"+record.ID+".json")
}

func (s *ScopedManager) CreateAttachment(name, filename, mediaType string, data []byte) (Attachment, error) {
	s.manager.mutationMu.Lock()
	defer s.manager.mutationMu.Unlock()
	if filename == "" || len(filename) > 255 || strings.ContainsAny(filename, `/\`) || strings.IndexFunc(filename, unicode.IsControl) >= 0 || len(mediaType) > 128 || strings.IndexFunc(mediaType, unicode.IsControl) >= 0 || len(data) == 0 || int64(len(data)) > MaxAttachmentBytes {
		return Attachment{}, ErrAttachmentLimit
	}
	root, dir, err := s.attachmentRoot(name, true)
	if err != nil {
		return Attachment{}, err
	}
	defer root.Close()
	directory, err := root.Open(dir)
	if err != nil {
		return Attachment{}, err
	}
	entries, err := directory.ReadDir(-1)
	directory.Close()
	if err != nil {
		return Attachment{}, err
	}
	var used int64
	count := 0
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".bin") {
			info, err := entry.Info()
			if err != nil || !info.Mode().IsRegular() {
				return Attachment{}, ErrAttachmentUnavailable
			}
			used += info.Size()
			count++
		}
	}
	if count >= 8 || used+int64(len(data)) > maxSessionAttachmentBytes {
		return Attachment{}, ErrAttachmentLimit
	}
	if err := s.checkAttachmentActorQuota(name, int64(len(data)), maxActorAttachmentBytes); err != nil {
		return Attachment{}, err
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return Attachment{}, err
	}
	digest := sha256.Sum256(data)
	a := Attachment{ID: "file-" + hex.EncodeToString(nonce[:]), Name: filename, MediaType: mediaType, Bytes: int64(len(data)), SHA256: hex.EncodeToString(digest[:]), Status: "uploaded", CreatedAt: time.Now().UTC()}
	f, err := root.OpenFile(dir+"/"+a.ID+".bin", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return Attachment{}, err
	}
	_, writeErr := f.Write(data)
	err = errors.Join(writeErr, f.Sync(), f.Close())
	if err != nil {
		return Attachment{}, err
	}
	// A crash before this publication retains an unreferenced original which
	// still counts against quota. It cannot become ready or enter a prompt.
	if err = writeAttachmentRecord(root, dir, attachmentRecord{Schema: 1, Attachment: a}); err != nil {
		return Attachment{}, err
	}
	return a, nil
}

func (s *ScopedManager) PrepareAttachment(name, id, text, parser, errorCode string) (Attachment, error) {
	s.manager.mutationMu.Lock()
	defer s.manager.mutationMu.Unlock()
	root, dir, err := s.attachmentRoot(name, false)
	if err != nil {
		return Attachment{}, err
	}
	defer root.Close()
	record, err := readAttachmentRecord(root, dir, id)
	if err != nil {
		return Attachment{}, err
	}
	if record.Status == "ready" {
		return record.Attachment, ErrTurnConflict
	}
	if len(text) > maxAttachmentTextBytes || len(parser) > 128 || len(errorCode) > 128 {
		return Attachment{}, ErrAttachmentLimit
	}
	if errorCode != "" {
		record.Status = "failed"
		record.ErrorCode = errorCode
		record.Text = ""
	} else {
		if strings.TrimSpace(text) == "" || parser == "" {
			return Attachment{}, ErrAttachmentUnavailable
		}
		revision := sha256.Sum256([]byte(record.SHA256 + "\x00" + parser + "\x00" + text))
		record.Status = "ready"
		record.Text = text
		record.Parser = parser
		record.Revision = hex.EncodeToString(revision[:])
		record.ErrorCode = ""
	}
	err = writeAttachmentRecord(root, dir, record)
	return record.Attachment, err
}

func (s *ScopedManager) ReadAttachment(name, id string) (Attachment, string, error) {
	s.manager.mutationMu.Lock()
	defer s.manager.mutationMu.Unlock()
	root, dir, err := s.attachmentRoot(name, false)
	if err != nil {
		return Attachment{}, "", err
	}
	defer root.Close()
	record, err := readAttachmentRecord(root, dir, id)
	if err != nil {
		return Attachment{}, "", err
	}
	if record.Status != "ready" {
		return record.Attachment, "", ErrAttachmentUnavailable
	}
	digest := sha256.Sum256([]byte(record.SHA256 + "\x00" + record.Parser + "\x00" + record.Text))
	if hex.EncodeToString(digest[:]) != record.Revision {
		return Attachment{}, "", ErrAttachmentUnavailable
	}
	return record.Attachment, record.Text, nil
}

func (s *ScopedManager) AttachmentOriginal(name, id string) (Attachment, []byte, error) {
	s.manager.mutationMu.Lock()
	defer s.manager.mutationMu.Unlock()
	root, dir, err := s.attachmentRoot(name, false)
	if err != nil {
		return Attachment{}, nil, err
	}
	defer root.Close()
	record, err := readAttachmentRecord(root, dir, id)
	if err != nil {
		return Attachment{}, nil, err
	}
	p := dir + "/" + id + ".bin"
	info, err := root.Lstat(p)
	if err != nil || !info.Mode().IsRegular() || info.Size() > MaxAttachmentBytes {
		return Attachment{}, nil, ErrAttachmentUnavailable
	}
	f, err := root.Open(p)
	if err != nil {
		return Attachment{}, nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, MaxAttachmentBytes+1))
	digest := sha256.Sum256(b)
	if err != nil || int64(len(b)) != record.Bytes || hex.EncodeToString(digest[:]) != record.SHA256 {
		return Attachment{}, nil, ErrAttachmentUnavailable
	}
	return record.Attachment, b, nil
}

func (s *ScopedManager) ListAttachments(name string) ([]Attachment, error) {
	s.manager.mutationMu.Lock()
	defer s.manager.mutationMu.Unlock()
	if _, err := s.Load(name); err != nil {
		return nil, err
	}
	root, dir, err := s.attachmentRoot(name, false)
	if errors.Is(err, os.ErrNotExist) {
		return []Attachment{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer root.Close()
	f, err := root.Open(dir)
	if err != nil {
		return nil, err
	}
	entries, err := f.ReadDir(-1)
	f.Close()
	if err != nil {
		return nil, err
	}
	items := []Attachment{}
	for _, entry := range entries {
		if path.Ext(entry.Name()) != ".json" {
			continue
		}
		record, err := readAttachmentRecord(root, dir, strings.TrimSuffix(entry.Name(), ".json"))
		if err != nil {
			return nil, err
		}
		items = append(items, record.Attachment)
	}
	return items, nil
}
