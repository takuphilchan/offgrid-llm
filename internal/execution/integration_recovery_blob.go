package execution

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
)

// A single bounded derived index avoids rehashing an entire retained file for
// every page. Authorization is checked before consulting it. Each page is
// checked against these chunks and clients verify the complete requested hash.
type recoveryBlobIndex struct {
	integration, artifact, hash string
	bytes                       int64
	chunks                      []string
}

func (s *WorkingCopies) indexRecoveryFile(ctx context.Context, root *os.Root, name string, info os.FileInfo, integration, artifact string) (*recoveryBlobIndex, error) {
	if !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > s.policy.MaxCopyBytes || info.Size() > int64(maxChangeBlobChunks)*MaxPageBytes {
		return nil, ErrLimit
	}
	f, err := openWorkingCopyFile(root, name, info)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	index := &recoveryBlobIndex{integration: integration, artifact: artifact, bytes: info.Size()}
	hash, buffer := sha256.New(), make([]byte, MaxPageBytes)
	reader := copyContextReader{ctx, f}
	for remaining := info.Size(); remaining > 0; {
		length := min(int64(len(buffer)), remaining)
		if _, err = io.ReadFull(reader, buffer[:length]); err != nil {
			return nil, err
		}
		hash.Write(buffer[:length])
		index.chunks = append(index.chunks, digest(buffer[:length]))
		remaining -= length
	}
	var extra [1]byte
	if n, err := reader.Read(extra[:]); n != 0 || err != io.EOF {
		return nil, ErrRevision
	}
	after, err := f.Stat()
	if err != nil || after.Size() != info.Size() || !after.ModTime().Equal(info.ModTime()) {
		return nil, ErrRevision
	}
	index.hash = hex.EncodeToString(hash.Sum(nil))
	s.recoveryCache = index
	return index, nil
}

func (s *WorkingCopies) readRecoveryBlobLocked(ctx context.Context, id string, owner Binding, artifact, hash string, offset int64, limit int) (ChangeBlobPage, error) {
	if !validCopyID(hash) || offset < 0 || limit < 1 || limit > MaxPageBytes {
		return ChangeBlobPage{}, ErrLimit
	}
	ordinal, slot, err := recoveryArtifactID(artifact)
	if err != nil {
		return ChangeBlobPage{}, err
	}
	set, _, step, err := s.recoveryStep(ctx, id, owner, ordinal)
	if err != nil {
		return ChangeBlobPage{}, err
	}
	name := recoveryName(step, slot)
	if name == "" {
		return ChangeBlobPage{}, ErrAuthority
	}
	root, closeRoot, err := s.recoveryDirectory(set, id, ordinal)
	if err != nil {
		return ChangeBlobPage{}, err
	}
	defer closeRoot()
	info, err := root.Lstat(name)
	if err != nil {
		return ChangeBlobPage{}, err
	}
	if !info.Mode().IsRegular() {
		return ChangeBlobPage{}, ErrUnsupported
	}
	index := s.recoveryCache
	if index == nil || index.integration != id || index.artifact != artifact || index.hash != hash || index.bytes != info.Size() {
		index, err = s.indexRecoveryFile(ctx, root, name, info, id, artifact)
		if err != nil {
			return ChangeBlobPage{}, err
		}
	}
	if index.hash != hash {
		return ChangeBlobPage{}, ErrRevision
	}
	if offset > index.bytes {
		return ChangeBlobPage{}, ErrLimit
	}
	f, err := openWorkingCopyFile(root, name, info)
	if err != nil {
		return ChangeBlobPage{}, err
	}
	defer f.Close()
	end := offset + min(int64(limit), index.bytes-offset)
	page := ChangeBlobPage{SHA256: hash, Bytes: index.bytes, Offset: offset, Data: make([]byte, 0, end-offset)}
	for position := offset; position < end; {
		if err = ctx.Err(); err != nil {
			return ChangeBlobPage{}, err
		}
		chunk := position / MaxPageBytes
		start := chunk * MaxPageBytes
		data := make([]byte, min(int64(MaxPageBytes), index.bytes-start))
		if n, err := f.ReadAt(data, start); err != nil || n != len(data) || digest(data) != index.chunks[chunk] {
			return ChangeBlobPage{}, ErrRevision
		}
		stop := min(end, start+int64(len(data)))
		page.Data = append(page.Data, data[position-start:stop-start]...)
		position = stop
	}
	if end < index.bytes {
		page.NextOffset = &end
	}
	return page, nil
}
