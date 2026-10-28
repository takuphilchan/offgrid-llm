package models

import (
	"context"
	"errors"
	"os"
	"sync"
)

// CapabilityMeasurements is an installation-owner-local cache of byte hashes,
// never of authority, runtime observations or qualification. Reuse requires a
// kernel change stamp on a supported local filesystem; size/mtime alone is not
// enough. Other platforms/filesystems continue to hash on every measurement.
// Callers still hold installation/lifecycle ownership and bind the open handles
// to the selected runtime. The cache is bounded and is never persisted.
type CapabilityMeasurements struct {
	mu     sync.Mutex
	hashes map[capabilityFileStamp]string
}

func (m *CapabilityMeasurements) File(ctx context.Context, file *os.File) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if m == nil || file == nil {
		return MeasureCapabilityFile(ctx, file)
	}
	before, reusable := capabilityChangeStamp(file)
	if !reusable {
		return MeasureCapabilityFile(ctx, file)
	}
	m.mu.Lock()
	digest, found := m.hashes[before]
	m.mu.Unlock()
	if !found {
		var err error
		digest, err = MeasureCapabilityFile(ctx, file)
		if err != nil {
			return "", err
		}
	}
	after, reusable := capabilityChangeStamp(file)
	if !reusable || before != after {
		return "", errors.New("capability artifact changed during measurement")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if !found {
		m.mu.Lock()
		if len(m.hashes) >= 256 {
			m.hashes = nil
		}
		if m.hashes == nil {
			m.hashes = make(map[capabilityFileStamp]string)
		}
		m.hashes[after] = digest
		m.mu.Unlock()
	}
	return digest, nil
}

func (m *CapabilityMeasurements) Bundle(ctx context.Context, files map[string]*os.File) (string, error) {
	return measureCapabilityBundle(ctx, files, m.File)
}
