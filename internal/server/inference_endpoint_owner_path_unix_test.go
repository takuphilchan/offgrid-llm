//go:build !windows

package server

import "testing"

func endpointOwnerFixtureDirectory(t *testing.T) string { t.Helper(); return t.TempDir() }
