package execution

import (
	"context"
	"encoding/json"
	"io/fs"
	"math"
	"os"
)

// Measure accounts observed logical file growth without pretending a trusted
// native process is constrained by a filesystem quota. An incomplete/unsupported
// inventory conservatively occupies the host's entire admission allowance.
func (s *WorkingCopies) Measure(ctx context.Context, id string, owner Binding) (WorkingCopyRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.measureLocked(ctx, id, owner)
}

func (s *WorkingCopies) measureLocked(ctx context.Context, id string, owner Binding) (WorkingCopyRecord, error) {
	r, err := s.inspect(ctx, id)
	if err != nil {
		return r, err
	}
	if r.Source != owner || !validCopyID(id) {
		return WorkingCopyRecord{}, ErrAuthority
	}
	if r.State != "ready" {
		return r, ErrConflict
	}
	root, err := s.root.OpenRoot(id + "/content")
	if err != nil {
		return s.unknownMeasure(r, err)
	}
	defer root.Close()
	measured, err := measureCopyTree(ctx, root, s.policy.MaxEntries)
	if err != nil {
		return s.unknownMeasure(r, err)
	}
	var artifacts int64
	for _, directory := range []string{"artifacts", "journal/replacements"} {
		info, statErr := s.root.Lstat(id + "/" + directory)
		if statErr == nil {
			if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return s.unknownMeasure(r, ErrUnsupported)
			}
			artifactRoot, openErr := s.root.OpenRoot(id + "/" + directory)
			if openErr != nil {
				return s.unknownMeasure(r, openErr)
			}
			var bytes int64
			bytes, err = measureCopyTree(ctx, artifactRoot, 17*s.policy.MaxEntries+32)
			artifactRoot.Close()
			if err != nil {
				return s.unknownMeasure(r, err)
			}
			if bytes > math.MaxInt64-artifacts {
				return s.unknownMeasure(r, ErrLimit)
			}
			artifacts += bytes
		} else if !os.IsNotExist(statErr) {
			return s.unknownMeasure(r, statErr)
		}
	}
	if r.ArtifactReservedBytes < 0 || r.ArtifactReservedBytes > r.ReservedBytes || artifacts > math.MaxInt64-measured {
		return s.unknownMeasure(r, ErrLimit)
	}
	contentReserved := max(r.ReservedBytes-r.ArtifactReservedBytes, measured)
	r.ArtifactReservedBytes = max(r.ArtifactReservedBytes, artifacts)
	if r.ArtifactReservedBytes > math.MaxInt64-contentReserved {
		return s.unknownMeasure(r, ErrLimit)
	}
	r.ReservedBytes = contentReserved + r.ArtifactReservedBytes
	r.MeasuredBytes = measured + artifacts
	r.ContentBytes = measured
	r.UsageMeasured = true
	data, _ := json.Marshal(r)
	if _, err = s.db.ExecContext(ctx, `UPDATE worker_copies SET reserved_bytes=?,record=? WHERE id=?`, r.ReservedBytes, data, id); err != nil {
		return r, err
	}
	return r, nil
}

func measureCopyTree(ctx context.Context, root *os.Root, limit int) (int64, error) {
	var measured int64
	entries := 0
	err := fs.WalkDir(root.FS(), ".", func(name string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if name == "." {
			return nil
		}
		entries++
		if entries > limit {
			return ErrLimit
		}
		info, err := root.Lstat(name)
		if err != nil {
			return err
		}
		if info.Mode().Type() != 0 && info.Mode().Type() != os.ModeDir {
			return ErrUnsupported
		}
		if info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > math.MaxInt64-measured {
			return ErrUnsupported
		}
		measured += info.Size()
		return nil
	})
	return measured, err
}

// Refresh prior ready artifacts before admitting another copy. In particular,
// restarting the store or losing a measurement write cannot reopen capacity
// that an earlier trusted command already consumed.
func (s *WorkingCopies) refreshCopiesLocked(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT record FROM worker_copies WHERE state='ready'`)
	if err != nil {
		return err
	}
	var records []WorkingCopyRecord
	for rows.Next() {
		var data []byte
		var record WorkingCopyRecord
		if err = rows.Scan(&data); err != nil {
			break
		}
		if err = json.Unmarshal(data, &record); err != nil {
			break
		}
		records = append(records, record)
		if len(records) > s.policy.MaxCopies {
			err = ErrLimit
			break
		}
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return err
	}
	for _, record := range records {
		if _, err = s.measureLocked(ctx, record.ID, record.Source); err != nil {
			return err
		}
	}
	return nil
}

func (s *WorkingCopies) unknownMeasure(r WorkingCopyRecord, cause error) (WorkingCopyRecord, error) {
	r.State = "uncertain"
	r.UsageMeasured = false
	r.ReservedBytes = max(r.ReservedBytes, s.policy.MaxTotalBytes)
	data, _ := json.Marshal(r)
	if _, err := s.db.Exec(`UPDATE worker_copies SET state='uncertain',reserved_bytes=?,record=? WHERE id=?`, r.ReservedBytes, data, r.ID); err != nil {
		return r, err
	}
	return r, cause
}
