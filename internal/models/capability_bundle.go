package models

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// MeasureCapabilityBundle measures an explicit, trusted installation's file
// handles, not paths from model output. Roles identify the executable and every
// required runtime dependency; discovery and completeness belong to the runtime
// installer. A bundle measurement alone is not qualification or proof that an
// already-running process loaded these bytes. The caller must retain its
// installation/lifecycle lease, as required by MeasureCapabilityFile.
//
// Only roles and digests enter the identity. Paths, file contents and credentials
// are never included in a persisted bundle manifest by this function.
func MeasureCapabilityBundle(ctx context.Context, files map[string]*os.File) (string, error) {
	return measureCapabilityBundle(ctx, files, MeasureCapabilityFile)
}

func measureCapabilityBundle(ctx context.Context, files map[string]*os.File, measure func(context.Context, *os.File) (string, error)) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if len(files) == 0 || len(files) > 256 {
		return "", fmt.Errorf("capability bundle requires 1-256 artifacts")
	}
	roles := make([]string, 0, len(files))
	for role, file := range files {
		if !evidenceCode.MatchString(role) || file == nil {
			return "", fmt.Errorf("invalid capability artifact role or handle")
		}
		roles = append(roles, role)
	}
	sort.Strings(roles)
	type measuredArtifact struct {
		Role   string `json:"role"`
		SHA256 string `json:"sha256"`
	}
	artifacts := make([]measuredArtifact, 0, len(roles))
	for _, role := range roles {
		digest, err := measure(ctx, files[role])
		if err != nil {
			return "", fmt.Errorf("measure runtime artifact %s: %w", role, err)
		}
		artifacts = append(artifacts, measuredArtifact{role, digest})
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	encoded, err := json.Marshal(struct {
		Version   int                `json:"version"`
		Artifacts []measuredArtifact `json:"artifacts"`
	}{1, artifacts})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}
