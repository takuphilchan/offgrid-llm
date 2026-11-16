package sessions

import (
	"context"
	"io"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

// AttachmentPublication snapshots the explicitly selected revision and its
// ancestry while holding the mutation lock. It never moves the draft reference.
// Only server-owned, verified graphs leave this boundary; clients cannot supply
// extraction receipts as JSON and assert that they belong to an original.
func (s *ScopedManager) AttachmentPublication(ctx context.Context, name, id, revision string) (Attachment, []byte, []*document.Extraction, error) {
	s.manager.mutationMu.Lock()
	defer s.manager.mutationMu.Unlock()
	root, dir, err := s.attachmentRoot(name, false)
	if err != nil {
		return Attachment{}, nil, nil, err
	}
	defer root.Close()
	record, err := readAttachmentRecordRevision(root, dir, id, revision)
	if err != nil {
		return Attachment{}, nil, nil, err
	}
	// Legacy clients may publish uncorrected originals. A corrected extraction
	// requires an explicit revision; otherwise the intended evidence is unknown.
	if record.ParentRevision != "" && revision == "" {
		return Attachment{}, nil, nil, ErrTurnConflict
	}
	if err := verifyAttachmentOriginal(root, dir, record); err != nil {
		return Attachment{}, nil, nil, err
	}
	file, err := root.Open(dir + "/" + id + ".bin")
	if err != nil {
		return Attachment{}, nil, nil, err
	}
	data, err := io.ReadAll(io.LimitReader(file, MaxAttachmentBytes+1))
	file.Close()
	if err != nil || int64(len(data)) != record.Bytes || document.TextSHA256(string(data)) != record.SHA256 {
		return Attachment{}, nil, nil, ErrAttachmentUnavailable
	}
	var graphs []*document.Extraction
	if record.Status == "ready" && revision != "" && record.ExtractionRevision != "" {
		// Inspect sizes before loading the chain, including interrupted artifacts.
		revisions := append(append([]string{}, record.PreviousRevisions...), record.Revision)
		var total int64
		for _, rev := range revisions {
			path := revisionArtifactPath(dir, id, rev)
			if len(record.PreviousRevisions) == 0 {
				path = dir + "/" + id + ".structure"
			}
			info, err := root.Lstat(path)
			if err != nil || !info.Mode().IsRegular() {
				return Attachment{}, nil, nil, ErrAttachmentUnavailable
			}
			total += info.Size()
			if total > 64<<20 {
				return Attachment{}, nil, nil, ErrAttachmentLimit
			}
		}
		for _, rev := range revisions {
			if err := ctx.Err(); err != nil {
				return Attachment{}, nil, nil, err
			}
			old, err := readPublishedAttachmentRevision(root, dir, record, rev)
			if err != nil {
				return Attachment{}, nil, nil, err
			}
			graph, err := readAttachmentStructure(root, dir, old)
			if err != nil || graph == nil {
				return Attachment{}, nil, nil, ErrAttachmentUnavailable
			}
			graphs = append(graphs, graph)
		}
	}
	if err := ctx.Err(); err != nil {
		return Attachment{}, nil, nil, err
	}
	return record.Attachment, data, graphs, nil
}
