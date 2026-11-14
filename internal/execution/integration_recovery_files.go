package execution

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Recovery files are current observations of retained private artifacts. They
// never settle an uncertain operation, restore a target or change its receipt.
type IntegrationRecoveryFile struct {
	ID         string `json:"id"`
	Path       string `json:"path"`
	Role       string `json:"role"`
	SHA256     string `json:"sha256,omitempty"`
	Bytes      int64  `json:"bytes"`
	Limitation string `json:"limitation,omitempty"`
}

type IntegrationRecoveryPage struct {
	IntegrationID string                    `json:"integration_id"`
	ChangeSetID   string                    `json:"change_set_id"`
	Files         []IntegrationRecoveryFile `json:"files"`
	NextOffset    *int                      `json:"next_offset,omitempty"`
}

// An uncertain integration does not invalidate previously sealed proposal
// bytes. A draft/interrupted seal has no admitted integration record and must
// not be promoted to a sealed result by a header flag alone.
func (s *WorkingCopies) sealedBeforeIntegration(ctx context.Context, set ChangeSet) bool {
	if !set.IntegrationStarted {
		return false
	}
	var data []byte
	var expected string
	if err := s.db.QueryRowContext(ctx, `SELECT request,request_digest FROM worker_integrations WHERE set_id=?`, set.ID).Scan(&data, &expected); err != nil || digest(data) != expected {
		return false
	}
	var req IntegrateChangeSetRequest
	return json.Unmarshal(data, &req) == nil && req.Source == set.Source && req.ChangeSetID == set.ID && req.OperationsDigest == set.OperationsDigest
}

func recoveryArtifactID(id string) (int, string, error) {
	parts := strings.Split(id, ":")
	if len(parts) != 2 || (parts[1] != "displaced" && parts[1] != "staged") {
		return 0, "", ErrProtocol
	}
	ordinal, err := strconv.Atoi(parts[0])
	if err != nil || ordinal < 0 || ordinal > 200000 || strconv.Itoa(ordinal) != parts[0] {
		return 0, "", ErrProtocol
	}
	return ordinal, parts[1], nil
}

func (s *WorkingCopies) recoveryStep(ctx context.Context, id string, owner Binding, ordinal int) (ChangeSet, IntegrationFileReceipt, integrationStep, error) {
	page, err := s.inspectIntegrationLocked(ctx, id, owner, ordinal, 1)
	if err != nil {
		return ChangeSet{}, IntegrationFileReceipt{}, integrationStep{}, err
	}
	if len(page.Files) != 1 || page.Files[0].Ordinal != ordinal || !page.Files[0].Dispatched {
		return ChangeSet{}, IntegrationFileReceipt{}, integrationStep{}, ErrAuthority
	}
	set, _, err := s.changeSet(ctx, page.ChangeSetID, owner)
	if err != nil {
		return set, IntegrationFileReceipt{}, integrationStep{}, err
	}
	var data []byte
	err = s.db.QueryRowContext(ctx, `SELECT step FROM worker_integration_files WHERE integration_id=? AND ordinal=?`, id, ordinal).Scan(&data)
	var step integrationStep
	if err == nil && (json.Unmarshal(data, &step) != nil || step.Ordinal != ordinal || step.Change.Path != page.Files[0].Path) {
		err = ErrRevision
	}
	return set, page.Files[0], step, err
}

func recoveryName(step integrationStep, slot string) string {
	if slot == "displaced" {
		return "displaced"
	}
	// Linux exchange retains the displaced inode at the staged proposal name.
	if step.Change.Kind == "update" && step.Change.Result != nil && step.Change.Result.Kind == "file" && validCopyID(step.Change.Result.SHA256) {
		return step.Change.Result.SHA256
	}
	return ""
}

func (s *WorkingCopies) recoveryDirectory(set ChangeSet, id string, ordinal int) (*os.Root, func(), error) {
	name := fmt.Sprintf("%s/artifacts/integration-%s/%06d", set.CopyID, id, ordinal)
	pin, err := pinReplacementDirectory(s.root, name)
	if err != nil {
		return nil, nil, err
	}
	root, err := replacementView(s.root, name, pin)
	if err != nil {
		pin.close()
		return nil, nil, err
	}
	return root, func() { root.Close(); pin.close() }, nil
}

func (s *WorkingCopies) inspectRecoveryLocked(ctx context.Context, id string, owner Binding, offset, limit int) (IntegrationRecoveryPage, error) {
	page, err := s.inspectIntegrationLocked(ctx, id, owner, offset, limit)
	result := IntegrationRecoveryPage{IntegrationID: id, ChangeSetID: page.ChangeSetID, Files: []IntegrationRecoveryFile{}, NextOffset: page.NextOffset}
	if err != nil {
		return result, err
	}
	for _, file := range page.Files {
		if !file.Dispatched {
			continue
		}
		set, receipt, step, err := s.recoveryStep(ctx, id, owner, file.Ordinal)
		if err != nil {
			return result, err
		}
		root, closeRoot, err := s.recoveryDirectory(set, id, file.Ordinal)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return result, err
		}
		for _, slot := range []string{"displaced", "staged"} {
			name := recoveryName(step, slot)
			if name == "" {
				continue
			}
			info, err := root.Lstat(name)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				closeRoot()
				return result, err
			}
			artifact := IntegrationRecoveryFile{ID: fmt.Sprintf("%d:%s", file.Ordinal, slot), Path: file.Path, Role: "unconfirmed", Bytes: info.Size()}
			if !info.Mode().IsRegular() {
				artifact.Limitation = "unsupported_file_type"
			} else {
				index, err := s.indexRecoveryFile(ctx, root, name, info, id, artifact.ID)
				if err != nil {
					closeRoot()
					return result, err
				}
				artifact.SHA256 = index.hash
				if receipt.Displaced == name && receipt.DisplacedDigest == index.hash {
					artifact.Role = "displaced"
				}
			}
			result.Files = append(result.Files, artifact)
		}
		closeRoot()
	}
	return result, nil
}
