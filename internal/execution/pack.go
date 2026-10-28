package execution

import (
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const PackRevision = "offgrid-executor-v1"
const BootstrapJob = "executor-supervisor"

type Pack struct {
	Schema       int    `json:"schema_version"`
	Protocol     int    `json:"protocol"`
	Revision     string `json:"revision"`
	Version      string `json:"version"`
	Platform     string `json:"platform"`
	Architecture string `json:"architecture"`
	File         string `json:"file"`
	SHA256       string `json:"sha256"`
}

func CanonicalVersion(value string) string {
	if value == "" || value == "unknown" {
		return "development"
	}
	return strings.TrimPrefix(value, "v")
}

func (p Pack) Validate(version string) error {
	digest, err := hex.DecodeString(p.SHA256)
	if p.Schema != 1 || p.Protocol != ProtocolVersion || p.Revision != PackRevision || CanonicalVersion(p.Version) != CanonicalVersion(version) || !validIdentity(p.Version) || err != nil || len(digest) != 32 || len(p.SHA256) != 64 {
		return ErrProtocol
	}
	if (p.Platform != "windows" && p.Platform != "linux" && p.Platform != "darwin") || (p.Architecture != "amd64" && p.Architecture != "arm64") {
		return ErrUnsupported
	}
	file := "offgrid-executor"
	if p.Platform == "windows" {
		file += ".exe"
	}
	if p.File != file {
		return ErrPath
	}
	return nil
}

// LoadPack reads trusted application assets, not a model package or project
// executable. No manifest may nominate an arbitrary path or command line.
func LoadPack(directory, version string) (Pack, string, error) {
	if !filepath.IsAbs(directory) {
		return Pack{}, "", ErrPath
	}
	file, err := os.Open(filepath.Join(directory, "execution-pack.json"))
	if err != nil {
		return Pack{}, "", err
	}
	data, readErr := io.ReadAll(io.LimitReader(file, 65537))
	file.Close()
	var pack Pack
	if readErr != nil || len(data) > 65536 || decodeStrict(data, &pack) != nil {
		return Pack{}, "", ErrProtocol
	}
	if err = pack.Validate(version); err != nil {
		return Pack{}, "", err
	}
	if pack.Platform != runtime.GOOS || pack.Architecture != runtime.GOARCH {
		return Pack{}, "", ErrUnsupported
	}
	return pack, filepath.Join(directory, pack.File), nil
}

type WorkerIdentity struct {
	Protocol     int    `json:"protocol"`
	Revision     string `json:"revision"`
	Version      string `json:"version"`
	Platform     string `json:"platform"`
	Architecture string `json:"architecture"`
}

func FindPack(executable, version, override string) (string, error) {
	if override != "" {
		if !filepath.IsAbs(override) {
			return "", ErrPath
		}
		_, _, err := LoadPack(override, version)
		return override, err
	}
	if !filepath.IsAbs(executable) {
		return "", ErrPath
	}
	resolved, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return "", err
	}
	directory := filepath.Dir(resolved)
	for _, candidate := range []string{filepath.Join(directory, "execution-runtime", CanonicalVersion(version)), filepath.Join(directory, "execution-runtime"), filepath.Join(directory, "..", "execution-runtime"), filepath.Join(directory, "..", "lib", "offgrid", "execution-runtime"), filepath.Join(directory, "..", "share", "offgrid", "execution-runtime"), filepath.Join(directory, "..", "Resources", "execution-runtime")} {
		_, _, err = LoadPack(candidate, version)
		if err == nil {
			return filepath.Clean(candidate), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}
	return "", os.ErrNotExist
}

func workerIdentity(version string) WorkerIdentity {
	return WorkerIdentity{ProtocolVersion, PackRevision, CanonicalVersion(version), runtime.GOOS, runtime.GOARCH}
}

func (i WorkerIdentity) matches(p Pack) bool {
	return i.Protocol == p.Protocol && i.Revision == p.Revision && CanonicalVersion(i.Version) == CanonicalVersion(p.Version) && i.Platform == p.Platform && i.Architecture == p.Architecture
}
