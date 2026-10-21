package models

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// NativeTextLease is a verified, pinned package revision for a trusted local
// worker. The package manager cannot remove its files until Release is called.
// It is not, by itself, runtime readiness or workflow qualification.
type NativeTextLease struct {
	ID              string
	Revision        string
	ModelDir        string
	ModelDigest     string
	ModelFamily     string
	Engine          string
	Format          string
	Backend         string
	RunnerRevision  string
	AdapterRevision string
	ContextLength   int
	TemplateDigest  string
	Adapter         ModelAdapterSpec
	release         func()
}

func (l *NativeTextLease) Release() {
	if l != nil && l.release != nil {
		l.release()
	}
}

// AcquireNativeText is the model-data admission boundary for native workers.
// Import remains possible without a matching adapter, but execution requires
// a trusted pinned adapter and a fresh artifact digest check. Worker launch,
// hardware qualification, and OS process restriction remain separate gates.
func (s *PackageStore) AcquireNativeText(ctx context.Context, id, revision, consumer string, adapters *AdapterRegistry) (*NativeTextLease, error) {
	if adapters == nil {
		return nil, errors.New("trusted adapter registry is required")
	}
	state, release, err := s.Acquire(ctx, id, revision, consumer)
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*NativeTextLease, error) {
		release()
		return nil, err
	}
	if state.Manifest == nil {
		return fail(errors.New("verified package has no manifest"))
	}
	m := *state.Manifest
	if m.Engine != EngineMLX && m.Engine != EngineTransformers {
		return fail(errors.New("package is not a native text model"))
	}
	adapter, err := adapters.MatchPackage(m)
	if err != nil {
		return fail(err)
	}
	if m.TemplateDigest == "" {
		return fail(errors.New("native package requires a pinned tokenizer template"))
	}
	template := sha256.Sum256([]byte(adapter.Template))
	if m.TemplateDigest != hex.EncodeToString(template[:]) {
		return fail(errors.New("native package template does not match its reviewed adapter"))
	}
	if len(m.Preprocessing) != 0 {
		return fail(errors.New("native preprocessing requires separate qualification"))
	}
	encoded, err := json.Marshal(m)
	if err != nil {
		return fail(err)
	}
	digest := sha256.Sum256(encoded)
	base, err := filepath.Abs(s.modelsDir)
	if err != nil {
		return fail(err)
	}
	modelDir := filepath.Join(base, PackageDirectory, id, revision)
	for _, location := range []string{filepath.Join(base, PackageDirectory, id), modelDir} {
		info, err := os.Lstat(location)
		if err != nil {
			return fail(err)
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fail(fmt.Errorf("native package directory is not a regular directory"))
		}
	}
	return &NativeTextLease{
		ID: id, Revision: revision,
		ModelDir: modelDir, ModelDigest: "sha256:" + hex.EncodeToString(digest[:]),
		ModelFamily: m.ModelFamily, Engine: m.Engine, Format: m.Format, Backend: m.Backend,
		RunnerRevision: m.RunnerRevision, AdapterRevision: m.AdapterRevision,
		ContextLength: m.ContextLength, TemplateDigest: m.TemplateDigest,
		Adapter: adapter, release: release,
	}, nil
}
