package execution

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"path/filepath"
	"sort"
	"unicode/utf8"
)

const (
	MaxPageBytes        = 16 * 1024
	MaxFileBytes        = 16 * 1024 * 1024
	MaxDirectoryEntries = 10000
	MaxEntriesPerPage   = 128
)

type Location struct {
	Host    string `json:"host"`
	Backend string `json:"backend"`
	Path    string `json:"path"`
}

type Entry struct {
	Name      string `json:"name"`
	Directory bool   `json:"directory"`
	Mode      string `json:"mode"`
}

type DirectoryPage struct {
	Location
	Offset       int     `json:"offset"`
	TotalEntries int     `json:"total_entries"`
	Entries      []Entry `json:"entries"`
	Revision     string  `json:"revision"`
	NextOffset   *int    `json:"next_offset"`
	Complete     bool    `json:"complete"`
}

// Data is raw bytes (base64 in JSON), not a numbered text presentation. Byte
// offsets/revisions allow exact copying even across UTF-8 page boundaries.
type FilePage struct {
	Location
	Offset     int    `json:"offset"`
	Data       []byte `json:"data"`
	Revision   string `json:"revision"`
	TotalBytes int    `json:"total_bytes"`
	NextOffset *int   `json:"next_offset"`
	Complete   bool   `json:"complete"`
}

func digest(data []byte) string {
	d := sha256.Sum256(data)
	return hex.EncodeToString(d[:])
}

func (s *Session) location(path string) Location {
	return Location{Host: s.config.Binding.Host, Backend: s.config.Backend, Path: filepath.Join(s.config.Root, path)}
}

func pageEnd(offset, limit, total, maxLimit int, revision string) (int, error) {
	if offset < 0 || offset > total || limit < 1 || limit > maxLimit || offset > 0 && revision == "" {
		return 0, ErrLimit
	}
	return min(total, offset+limit), nil
}

func nextOffset(end, total int) *int {
	if end == total {
		return nil
	}
	return &end
}

func (s *Session) List(ctx context.Context, binding Binding, path string, offset, limit int, revision string) (DirectoryPage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.authorize(ctx, binding); err != nil {
		return DirectoryPage{}, err
	}
	if !s.config.Permissions.Read {
		return DirectoryPage{}, ErrAuthority
	}
	name, err := s.resolve(path, s.config.Directory)
	if err != nil {
		return DirectoryPage{}, err
	}
	dir, err := s.root.Open(name)
	if err != nil {
		return DirectoryPage{}, err
	}
	defer dir.Close()
	entries, err := dir.ReadDir(MaxDirectoryEntries + 1)
	if err != nil && err != io.EOF {
		return DirectoryPage{}, err
	}
	if len(entries) > MaxDirectoryEntries {
		return DirectoryPage{}, ErrLimit
	}
	// ReadDir(n) returns filesystem order. Normalize before hashing/pagination.
	rows := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return DirectoryPage{}, err
		}
		if !utf8.ValidString(entry.Name()) {
			return DirectoryPage{}, ErrUnsupported
		}
		rows = append(rows, Entry{Name: entry.Name(), Directory: entry.IsDir(), Mode: entry.Type().String()})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	encoded, _ := json.Marshal(rows)
	current := digest(encoded)
	if revision != "" && revision != current {
		return DirectoryPage{}, ErrRevision
	}
	end, err := pageEnd(offset, limit, len(rows), MaxEntriesPerPage, revision)
	if err != nil {
		return DirectoryPage{}, err
	}
	return DirectoryPage{Location: s.location(name), Offset: offset, TotalEntries: len(rows), Entries: rows[offset:end], Revision: current, NextOffset: nextOffset(end, len(rows)), Complete: end == len(rows)}, nil
}

func (s *Session) read(ctx context.Context, name string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Check before open to avoid intentionally opening FIFO/device endpoints;
	// check the descriptor too so a normal replacement cannot masquerade as data.
	info, err := s.root.Stat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, ErrUnsupported
	}
	if info.Size() > MaxFileBytes {
		return nil, ErrLimit
	}
	f, err := s.root.OpenFile(name, readOpenFlags(), 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err = f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, ErrUnsupported
	}
	data, err := io.ReadAll(io.LimitReader(f, MaxFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxFileBytes {
		return nil, ErrLimit
	}
	after, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if after.Size() != info.Size() || !after.ModTime().Equal(info.ModTime()) || int64(len(data)) != after.Size() {
		return nil, ErrRevision
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return data, nil
}

func (s *Session) Read(ctx context.Context, binding Binding, path string, offset, limit int, revision string) (FilePage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.authorize(ctx, binding); err != nil {
		return FilePage{}, err
	}
	if !s.config.Permissions.Read {
		return FilePage{}, ErrAuthority
	}
	name, err := s.resolve(path, s.config.Directory)
	if err != nil {
		return FilePage{}, err
	}
	data, err := s.read(ctx, name)
	if err != nil {
		return FilePage{}, err
	}
	current := digest(data)
	if revision != "" && current != revision {
		return FilePage{}, ErrRevision
	}
	end, err := pageEnd(offset, limit, len(data), MaxPageBytes, revision)
	if err != nil {
		return FilePage{}, err
	}
	return FilePage{Location: s.location(name), Offset: offset, Data: data[offset:end], Revision: current, TotalBytes: len(data), NextOffset: nextOffset(end, len(data)), Complete: end == len(data)}, nil
}
