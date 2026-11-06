package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/takuphilchan/offgrid-llm/internal/cli"
)

// This process-owned preference contains identity only, never authorization.
type agentDefaultFiles struct{ directory func() (string, error) }

func (s agentDefaultFiles) path(scope cli.AgentDefaultScope) (string, error) {
	if scope.Origin == "" || scope.Workspace == "" || scope.Actor == "" {
		return "", fmt.Errorf("incomplete default scope")
	}
	root, err := s.directory()
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(root) {
		return "", fmt.Errorf("default directory must be absolute")
	}
	data, err := json.Marshal(scope)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return filepath.Join(root, hex.EncodeToString(digest[:])+".json"), nil
}

func (s agentDefaultFiles) Load(scope cli.AgentDefaultScope) (*cli.AgentDefault, error) {
	path, err := s.path(scope)
	if err != nil {
		return nil, err
	}
	stat, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !stat.Mode().IsRegular() || stat.Size() > 4096 {
		return nil, fmt.Errorf("invalid default preference file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(stat, opened) {
		return nil, fmt.Errorf("default preference changed while opening")
	}
	decoder := json.NewDecoder(io.LimitReader(file, 4097))
	decoder.DisallowUnknownFields()
	var value cli.AgentDefault
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	if !value.Valid(scope) {
		return nil, fmt.Errorf("invalid default preference identity")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("invalid default preference trailing data")
	}
	return &value, nil
}

func (s agentDefaultFiles) Save(scope cli.AgentDefaultScope, value *cli.AgentDefault) error {
	path, err := s.path(scope)
	if err != nil {
		return err
	}
	if value == nil {
		err = os.Remove(path)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if !value.Valid(scope) {
		return fmt.Errorf("invalid default preference identity")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".agent-default-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err := json.NewEncoder(file).Encode(value); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
