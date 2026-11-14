package execution

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
)

const maxChangeBlobChunks = 65536

type changeBlobChunks struct {
	Version    int      `json:"version"`
	ChunkBytes int      `json:"chunk_bytes"`
	Hashes     []string `json:"hashes"`
}

// A single bounded chunk manifest is cached, after the owning set is checked.
type changeBlobCache struct {
	setID, hash, digest string
	chunks              changeBlobChunks
}

func copyChangeBlob(ctx context.Context, source, target *os.Root, entry WorkingCopyEntry) (changeBlobChunks, error) {
	chunks := changeBlobChunks{Version: 1, ChunkBytes: MaxPageBytes, Hashes: []string{}}
	if !validCopyID(entry.SHA256) || entry.Bytes < 0 || entry.Bytes > int64(maxChangeBlobChunks)*MaxPageBytes {
		return chunks, ErrLimit
	}
	info, err := source.Lstat(entry.Path)
	if err != nil {
		return chunks, err
	}
	if !info.Mode().IsRegular() || info.Size() != entry.Bytes {
		return chunks, ErrRevision
	}
	input, err := openWorkingCopyFile(source, entry.Path, info)
	if err != nil {
		return chunks, err
	}
	defer input.Close()
	output, err := target.OpenFile(entry.SHA256, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return chunks, err
	}
	defer output.Close()
	hash := sha256.New()
	buffer := make([]byte, MaxPageBytes)
	reader := copyContextReader{ctx, input}
	for remaining := entry.Bytes; remaining > 0; {
		length := min(int64(len(buffer)), remaining)
		if _, err = io.ReadFull(reader, buffer[:length]); err != nil {
			return chunks, err
		}
		if _, err = output.Write(buffer[:length]); err != nil {
			return chunks, err
		}
		hash.Write(buffer[:length])
		chunks.Hashes = append(chunks.Hashes, digest(buffer[:length]))
		remaining -= length
	}
	var extra [1]byte
	if n, err := reader.Read(extra[:]); n != 0 || err != io.EOF {
		return chunks, ErrRevision
	}
	if hex.EncodeToString(hash.Sum(nil)) != entry.SHA256 {
		return chunks, ErrRevision
	}
	if err = errors.Join(output.Sync(), output.Close()); err != nil {
		return chunks, err
	}
	return chunks, nil
}

type ChangeBlobPage struct {
	SHA256     string `json:"sha256"`
	Bytes      int64  `json:"bytes"`
	Offset     int64  `json:"offset"`
	Data       []byte `json:"data"`
	NextOffset *int64 `json:"next_offset,omitempty"`
}

// ReadChangeBlob verifies requested chunks against the sealed manifest. The
// whole-file SHA identifies the result; every returned byte is checked without
// repeatedly hashing the whole file for each page.
func (s *WorkingCopies) ReadChangeBlob(ctx context.Context, id string, owner Binding, hash string, offset int64, limit int) (ChangeBlobPage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readChangeBlobLocked(ctx, id, owner, hash, offset, limit)
}

func (s *WorkingCopies) readChangeBlobLocked(ctx context.Context, id string, owner Binding, hash string, offset int64, limit int) (ChangeBlobPage, error) {
	if !validCopyID(hash) || offset < 0 || limit < 1 || limit > MaxPageBytes {
		return ChangeBlobPage{}, ErrLimit
	}
	set, changes, err := s.changeSet(ctx, id, owner)
	if err != nil {
		return ChangeBlobPage{}, err
	}
	if set.State == "draft" || (set.State == "uncertain" || set.State == "abandoned") && !s.sealedBeforeIntegration(ctx, set) {
		return ChangeBlobPage{}, ErrConflict
	}
	var entry *WorkingCopyEntry
	var chunksDigest string
	for _, change := range changes {
		if change.Result != nil && change.Result.Kind == "file" && change.Result.SHA256 == hash {
			entry, chunksDigest = change.Result, change.ChunksDigest
			break
		}
	}
	if entry == nil {
		return ChangeBlobPage{}, ErrAuthority
	}
	if offset > entry.Bytes || entry.Bytes > int64(maxChangeBlobChunks)*MaxPageBytes {
		return ChangeBlobPage{}, ErrLimit
	}
	chunks, err := s.changeChunks(ctx, id, hash, chunksDigest, entry.Bytes)
	if err != nil {
		return ChangeBlobPage{}, err
	}
	name := set.CopyID + "/artifacts/" + id + "/" + hash
	info, err := s.root.Lstat(name)
	if err != nil {
		return ChangeBlobPage{}, err
	}
	if !info.Mode().IsRegular() || info.Size() != entry.Bytes {
		return ChangeBlobPage{}, ErrRevision
	}
	f, err := openWorkingCopyFile(s.root, name, info)
	if err != nil {
		return ChangeBlobPage{}, err
	}
	defer f.Close()
	end := offset + min(int64(limit), entry.Bytes-offset)
	page := ChangeBlobPage{SHA256: hash, Bytes: entry.Bytes, Offset: offset, Data: make([]byte, 0, end-offset)}
	for position := offset; position < end; {
		if err = ctx.Err(); err != nil {
			return ChangeBlobPage{}, err
		}
		index := position / int64(MaxPageBytes)
		start := index * int64(MaxPageBytes)
		data := make([]byte, min(int64(MaxPageBytes), entry.Bytes-start))
		if n, err := f.ReadAt(data, start); err != nil || n != len(data) || digest(data) != chunks.Hashes[index] {
			return ChangeBlobPage{}, ErrRevision
		}
		stop := min(end, start+int64(len(data)))
		page.Data = append(page.Data, data[position-start:stop-start]...)
		position = stop
	}
	if end < entry.Bytes {
		page.NextOffset = &end
	}
	return page, nil
}

func (s *WorkingCopies) changeChunks(ctx context.Context, setID, hash, expected string, size int64) (changeBlobChunks, error) {
	if c := s.blobCache; c != nil && c.setID == setID && c.hash == hash && c.digest == expected {
		return c.chunks, nil
	}
	var data []byte
	if err := s.db.QueryRowContext(ctx, `SELECT chunks FROM worker_change_blobs WHERE set_id=? AND hash=?`, setID, hash).Scan(&data); err != nil {
		return changeBlobChunks{}, err
	}
	if len(data) > 8<<20 || digest(data) != expected {
		return changeBlobChunks{}, ErrRevision
	}
	var chunks changeBlobChunks
	if json.Unmarshal(data, &chunks) != nil || chunks.Version != 1 || chunks.ChunkBytes != MaxPageBytes || int64(len(chunks.Hashes)) != (size+MaxPageBytes-1)/MaxPageBytes {
		return changeBlobChunks{}, ErrRevision
	}
	s.blobCache = &changeBlobCache{setID: setID, hash: hash, digest: expected, chunks: chunks}
	return chunks, nil
}
