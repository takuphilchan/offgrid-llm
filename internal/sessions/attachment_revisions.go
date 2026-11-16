package sessions

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

const maxAttachmentRevisions = 64
const maxAttachmentRevisionBytes = maxAttachmentRecordBytes + maxAttachmentStructureBytes
const maxActorAttachmentRevisionBytes = 256 << 20

type attachmentRevisionArtifact struct {
	Version int                 `json:"version"`
	Record  attachmentRecord    `json:"record"`
	Graph   document.Extraction `json:"graph"`
}

func validAttachmentRevision(revision string) bool {
	if len(revision) != 64 {
		return false
	}
	for _, c := range revision {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

func attachmentHistoryDigest(history []string) string {
	data, _ := json.Marshal(history)
	return document.TextSHA256(string(data))
}

func validAttachmentHistory(record attachmentRecord) bool {
	if len(record.PreviousRevisions) == 0 {
		return record.ParentRevision == "" && record.CorrectionAuthor == "" && record.CorrectedAt == nil && !strings.Contains(record.Parser, "/history-")
	}
	if record.Status != "ready" || len(record.PreviousRevisions) > maxAttachmentRevisions || record.ParentRevision != record.PreviousRevisions[len(record.PreviousRevisions)-1] || record.CorrectionAuthor == "" || len(record.CorrectionAuthor) > 256 || record.CorrectedAt == nil || record.CorrectedAt.IsZero() {
		return false
	}
	seen := map[string]bool{}
	for _, revision := range record.PreviousRevisions {
		if !validAttachmentRevision(revision) || seen[revision] || revision == record.Revision {
			return false
		}
		seen[revision] = true
	}
	token := "/history-" + attachmentHistoryDigest(record.PreviousRevisions)
	return strings.Contains(record.Parser, token+"/") || strings.HasSuffix(record.Parser, token)
}

func revisionArtifactPath(dir, id, revision string) string {
	return dir + "/" + id + ".revision-" + revision
}

func readAttachmentRevisionArtifact(root *os.Root, dir, id, revision string) (*attachmentRevisionArtifact, error) {
	if !attachmentID.MatchString(id) || !validAttachmentRevision(revision) {
		return nil, ErrAttachmentUnavailable
	}
	path := revisionArtifactPath(dir, id, revision)
	info, err := root.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxAttachmentRevisionBytes {
		return nil, ErrAttachmentUnavailable
	}
	file, err := root.Open(path)
	if err != nil {
		return nil, ErrAttachmentUnavailable
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxAttachmentRevisionBytes+1))
	var result attachmentRevisionArtifact
	if err != nil || len(data) > maxAttachmentRevisionBytes || json.Unmarshal(data, &result) != nil || result.Version != 1 {
		return nil, ErrAttachmentUnavailable
	}
	record := result.Record
	if record.ID != id || record.Revision != revision || record.Status != "ready" || validateAttachmentRecord(record) != nil || document.TextSHA256(record.SHA256+"\x00"+record.Parser+"\x00"+record.Text) != revision {
		return nil, ErrAttachmentUnavailable
	}
	if result.Graph.Text != "" || result.Graph.ID != record.ExtractionRevision || result.Graph.OriginalSHA256 != record.SHA256 {
		return nil, ErrAttachmentUnavailable
	}
	result.Graph.Text = record.Text
	if result.Graph.Verify() != nil {
		return nil, ErrAttachmentUnavailable
	}
	if record.ParentRevision != "" && (result.Graph.Author != record.CorrectionAuthor || result.Graph.CreatedAt == nil || record.CorrectedAt == nil || !result.Graph.CreatedAt.Equal(*record.CorrectedAt)) {
		return nil, ErrAttachmentUnavailable
	}
	return &result, nil
}

// Only the current record's published history authorizes an old read. A fully
// written artifact from an interrupted publication is deliberately unreachable.
func readAttachmentRecordRevision(root *os.Root, dir, id, revision string) (attachmentRecord, error) {
	current, err := readAttachmentRecord(root, dir, id)
	if err != nil {
		return current, err
	}
	return readPublishedAttachmentRevision(root, dir, current, revision)
}

func readPublishedAttachmentRevision(root *os.Root, dir string, current attachmentRecord, revision string) (attachmentRecord, error) {
	if revision == "" || revision == current.Revision {
		return current, nil
	}
	index := slices.Index(current.PreviousRevisions, revision)
	if index < 0 {
		return attachmentRecord{}, ErrAttachmentUnavailable
	}
	saved, err := readAttachmentRevisionArtifact(root, dir, current.ID, revision)
	if err != nil {
		return attachmentRecord{}, err
	}
	old := saved.Record
	if old.SHA256 != current.SHA256 || old.Bytes != current.Bytes || old.Name != current.Name || old.MediaType != current.MediaType || !slices.Equal(old.PreviousRevisions, current.PreviousRevisions[:index]) {
		return attachmentRecord{}, ErrAttachmentUnavailable
	}
	return old, nil
}

func (s *ScopedManager) writeAttachmentRevision(root *os.Root, dir, name string, record attachmentRecord, graph *document.Extraction) error {
	if validateAttachmentRecord(record) != nil || graph == nil || graph.Verify() != nil || graph.ID != record.ExtractionRevision || graph.Text != record.Text {
		return ErrAttachmentUnavailable
	}
	path := revisionArtifactPath(dir, record.ID, record.Revision)
	if _, err := root.Lstat(path); err == nil {
		prior, err := readAttachmentRevisionArtifact(root, dir, record.ID, record.Revision)
		if err != nil || prior.Graph.ID != graph.ID || prior.Record.Parser != record.Parser || prior.Record.Text != record.Text {
			return ErrAttachmentUnavailable
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	stored := *graph
	stored.Text = ""
	data, err := json.Marshal(attachmentRevisionArtifact{Version: 1, Record: record, Graph: stored})
	if err != nil || len(data) > maxAttachmentRevisionBytes {
		return ErrAttachmentLimit
	}
	if err = s.checkAttachmentActorQuota(name, int64(len(data)), maxActorAttachmentRevisionBytes, ".revision-"); err != nil {
		return err
	}
	// Bound interrupted small writes too; a byte quota alone cannot bound counts.
	directory, err := root.Open(dir)
	if err != nil {
		return err
	}
	entries, err := directory.ReadDir(-1)
	directory.Close()
	if err != nil {
		return err
	}
	count := 0
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), record.ID+".revision-") {
			count++
		}
	}
	if count >= 2*maxAttachmentRevisions+2 {
		return ErrAttachmentLimit
	}
	temporary := path + "." + rand.Text() + ".partial"
	file, err := root.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer root.Remove(temporary)
	_, writeErr := file.Write(data)
	err = errors.Join(writeErr, file.Sync(), file.Close())
	if err != nil {
		return err
	}
	return root.Rename(temporary, path)
}
