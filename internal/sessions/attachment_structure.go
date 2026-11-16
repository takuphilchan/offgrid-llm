package sessions

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

const maxAttachmentStructureBytes = 32 << 20
const maxActorAttachmentStructureBytes = 256 << 20

type attachmentPreparation struct {
	evidence  *document.Coverage
	structure *document.Extraction
}

func (s *ScopedManager) PrepareAttachmentStructure(name, id, text, parser, errorCode, coverage string, evidence *document.Coverage, structure *document.Extraction) (Attachment, error) {
	return s.prepareAttachment(name, id, text, parser, errorCode, coverage, attachmentPreparation{evidence, structure})
}

// The graph is written before its pointer is atomically published in the source
// record. Interrupted files count against a separate derived-data quota. Retry
// replaces only an unready attachment's graph; ready sources are immutable.
func (s *ScopedManager) writeAttachmentStructure(root *os.Root, dir, name, id string, graph *document.Extraction) error {
	stored := *graph
	stored.Text = "" // Text is retained once, in the revision-bound record.
	data, err := json.Marshal(stored)
	if err != nil || len(data) > maxAttachmentStructureBytes {
		return ErrAttachmentLimit
	}
	destination := dir + "/" + id + ".structure"
	previous := int64(0)
	if info, err := root.Lstat(destination); err == nil {
		if !info.Mode().IsRegular() {
			return ErrAttachmentUnavailable
		}
		previous = info.Size()
	} else if !errors.Is(err, os.ErrNotExist) {
		return ErrAttachmentUnavailable
	}
	if err := s.checkAttachmentActorQuota(name, int64(len(data))-previous, maxActorAttachmentStructureBytes, ".structure"); err != nil {
		return err
	}
	temp := destination + "." + rand.Text() + ".partial"
	f, err := root.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer root.Remove(temp)
	_, writeErr := f.Write(data)
	err = errors.Join(writeErr, f.Sync(), f.Close())
	if err != nil {
		return err
	}
	return root.Rename(temp, destination)
}

func readAttachmentStructure(root *os.Root, dir string, record attachmentRecord) (*document.Extraction, error) {
	if record.ExtractionRevision == "" {
		return nil, nil
	}
	if record.ParentRevision != "" {
		artifact, err := readAttachmentRevisionArtifact(root, dir, record.ID, record.Revision)
		if err != nil || artifact.Record.Parser != record.Parser || artifact.Record.Text != record.Text || artifact.Record.ExtractionRevision != record.ExtractionRevision || artifact.Record.CorrectionAuthor != record.CorrectionAuthor || record.CorrectedAt == nil || artifact.Record.CorrectedAt == nil || !artifact.Record.CorrectedAt.Equal(*record.CorrectedAt) {
			return nil, ErrAttachmentUnavailable
		}
		return &artifact.Graph, nil
	}
	if !strings.Contains(record.Parser, "/"+record.ExtractionRevision+"/") && !strings.HasSuffix(record.Parser, "/"+record.ExtractionRevision) {
		return nil, ErrAttachmentUnavailable
	}
	info, err := root.Lstat(dir + "/" + record.ID + ".structure")
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxAttachmentStructureBytes {
		return nil, ErrAttachmentUnavailable
	}
	f, err := root.Open(dir + "/" + record.ID + ".structure")
	if err != nil {
		return nil, ErrAttachmentUnavailable
	}
	defer f.Close()
	info, err = f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxAttachmentStructureBytes {
		return nil, ErrAttachmentUnavailable
	}
	data, err := io.ReadAll(io.LimitReader(f, maxAttachmentStructureBytes+1))
	if err != nil || len(data) > maxAttachmentStructureBytes {
		return nil, ErrAttachmentUnavailable
	}
	var graph document.Extraction
	if json.Unmarshal(data, &graph) != nil || graph.Text != "" || graph.ID != record.ExtractionRevision || graph.OriginalSHA256 != record.SHA256 {
		return nil, ErrAttachmentUnavailable
	}
	graph.Text = record.Text
	if graph.Verify() != nil {
		return nil, ErrAttachmentUnavailable
	}
	return &graph, nil
}

func removeAttachmentStructure(root *os.Root, dir, id string) error {
	f, err := root.Open(dir)
	if err != nil {
		return err
	}
	entries, err := f.ReadDir(-1)
	f.Close()
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if name != id+".structure" && !strings.HasPrefix(name, id+".revision-") && !strings.HasPrefix(name, id+".ocr-") && !(strings.HasPrefix(name, id+".structure.") && strings.HasSuffix(name, ".partial")) {
			continue
		}
		if err := root.Remove(dir + "/" + name); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}
