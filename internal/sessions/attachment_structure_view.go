package sessions

import (
	"context"
	"github.com/takuphilchan/offgrid-llm/internal/document"
)

// A private source is pinned by its attachment revision. Inspection never
// reparses an original into a newer graph, or enables Knowledge corrections.
func (s *ScopedManager) AttachmentStructure(ctx context.Context, name string, ref AttachmentReference, unit string, offset, limit, relationshipOffset int, navigationOffset ...int) (*document.StructurePage, error) {
	return s.attachmentStructure(ctx, name, ref, unit, offset, limit, relationshipOffset, false, navigationOffset...)
}

func (s *ScopedManager) AttachmentCorrectionStructure(ctx context.Context, name string, ref AttachmentReference, unit string, offset, limit, relationshipOffset int, navigationOffset ...int) (*document.StructurePage, error) {
	return s.attachmentStructure(ctx, name, ref, unit, offset, limit, relationshipOffset, true, navigationOffset...)
}

func (s *ScopedManager) attachmentStructure(ctx context.Context, name string, ref AttachmentReference, unit string, offset, limit, relationshipOffset int, editable bool, navigationOffset ...int) (*document.StructurePage, error) {
	if offset < 0 || offset > 200000 || limit < 1 || limit > 200 || relationshipOffset < 0 || relationshipOffset > 400000 || len(unit) > 128 {
		return nil, document.ErrStructureQuery
	}
	s.manager.mutationMu.Lock()
	defer s.manager.mutationMu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	session, err := s.Load(name)
	if err != nil {
		return nil, err
	}
	root, dir, err := s.attachmentRoot(name, false)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	current, err := readAttachmentRecord(root, dir, ref.ID)
	if err != nil {
		return nil, err
	}
	canCorrect := editable && s.canCorrectAttachment(session) && ref.Revision == current.Revision
	sources, err := s.attachmentTexts(name, []AttachmentReference{ref}, 32<<20)
	if err != nil {
		return nil, err
	}
	if sources[0].Structure == nil {
		return nil, ErrAttachmentUnavailable
	}
	page, err := document.ProjectStructure(sources[0].Structure, ref.ID, unit, offset, limit, relationshipOffset, canCorrect, navigationOffset...)
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	page.AttachmentRevision = ref.Revision
	return page, nil
}
