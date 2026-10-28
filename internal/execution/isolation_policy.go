package execution

import (
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// IsolationPolicy is native-supervisor configuration, never a model tool's
// arguments. Construction does not start anything or grant execution authority.
type IsolationPolicy struct {
	Socket      string `json:"socket"`
	Image       string `json:"image"`
	Network     string `json:"network"`
	MemoryBytes int64  `json:"memory_bytes"`
	CPUMillis   int    `json:"cpu_millis"`
	Pids        int64  `json:"pids"`
	UID         int    `json:"uid"`
	GID         int    `json:"gid"`
}

func (p IsolationPolicy) Validate() error {
	// This is always a Linux-host path, including when a Windows/macOS service
	// validates metadata from that host. Never reinterpret it on the service OS.
	if !strings.HasPrefix(p.Socket, "/") || path.Clean(p.Socket) != p.Socket || strings.ContainsAny(p.Socket, "\x00\\:") || p.Network != "none" && p.Network != "bridge" || p.MemoryBytes < 128<<20 || p.MemoryBytes > 64<<30 || p.CPUMillis < 100 || p.CPUMillis > 64000 || p.Pids < 16 || p.Pids > 512 || p.UID < 1 || p.GID < 1 {
		return ErrAuthority
	}
	if !strings.HasPrefix(p.Image, "sha256:") || len(p.Image) != 71 {
		return ErrProtocol
	}
	id, err := hex.DecodeString(strings.TrimPrefix(p.Image, "sha256:"))
	if err != nil || len(id) != 32 {
		return ErrProtocol
	}
	return nil
}

// Reject device/socket mounts and bound the inspection. A live host directory
// is not protected against a hostile host administrator swapping its contents.
// Never select system/runtime directories or mount the native private journal.
func inspectIsolationRoot(root string) error {
	if !filepath.IsAbs(root) {
		return ErrPath
	}
	count := 0
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		count++
		if count > 100000 {
			return ErrLimit
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if path == root && !info.IsDir() {
			return ErrPath
		}
		mode := info.Mode()
		if mode.IsDir() || mode.IsRegular() || mode&os.ModeSymlink != 0 {
			return nil
		}
		return ErrAuthority
	})
}

type isolatedMount struct {
	Type        string `json:"Type"`
	Source      string `json:"Source"`
	Target      string `json:"Target"`
	ReadOnly    bool   `json:"ReadOnly"`
	BindOptions struct {
		NonRecursive bool `json:"NonRecursive"`
	} `json:"BindOptions"`
}
type isolatedHostConfig struct {
	ReadonlyRootfs bool              `json:"ReadonlyRootfs"`
	CapDrop        []string          `json:"CapDrop"`
	SecurityOpt    []string          `json:"SecurityOpt"`
	NetworkMode    string            `json:"NetworkMode"`
	Memory         int64             `json:"Memory"`
	MemorySwap     int64             `json:"MemorySwap"`
	NanoCPUs       int64             `json:"NanoCpus"`
	PidsLimit      int64             `json:"PidsLimit"`
	Mounts         []isolatedMount   `json:"Mounts"`
	Tmpfs          map[string]string `json:"Tmpfs"`
	LogConfig      struct {
		Type string `json:"Type"`
	} `json:"LogConfig"`
}
type isolatedContainerConfig struct {
	Image           string             `json:"Image"`
	Entrypoint      []string           `json:"Entrypoint"`
	Cmd             []string           `json:"Cmd"`
	User            string             `json:"User"`
	Env             []string           `json:"Env"`
	WorkingDir      string             `json:"WorkingDir"`
	AttachStdout    bool               `json:"AttachStdout"`
	AttachStderr    bool               `json:"AttachStderr"`
	NetworkDisabled bool               `json:"NetworkDisabled"`
	Labels          map[string]string  `json:"Labels"`
	HostConfig      isolatedHostConfig `json:"HostConfig"`
}

// containerConfig has no generic Docker flags, extra mounts, host networking,
// entrypoint inheritance or environment forwarding. The worker remains outside
// the container; its credential, grant, journal and Docker socket are NOT mounts.
func (p IsolationPolicy) containerConfig(root, directory, ownership string, write bool, spec ProcessSpec) (isolatedContainerConfig, error) {
	if p.Validate() != nil || ValidateProcessSpec(spec) != nil || !filepath.IsAbs(root) || !validCredential(ownership) {
		return isolatedContainerConfig{}, ErrAuthority
	}
	clean := filepath.ToSlash(filepath.Clean(directory))
	if directory == "" || strings.ContainsRune(directory, 0) || strings.Contains(directory, `\`) || filepath.IsAbs(directory) || strings.HasPrefix(clean, "/") || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, ":") {
		return isolatedContainerConfig{}, ErrPath
	}
	mount := isolatedMount{Type: "bind", Source: root, Target: "/workspace", ReadOnly: !write}
	mount.BindOptions.NonRecursive = true
	config := isolatedContainerConfig{Image: p.Image, Entrypoint: []string{spec.Program}, Cmd: append([]string{}, spec.Arguments...), User: strconv.Itoa(p.UID) + ":" + strconv.Itoa(p.GID), Env: []string{"HOME=/tmp", "TMPDIR=/tmp", "XDG_CACHE_HOME=/tmp/cache", "LANG=C.UTF-8"}, WorkingDir: "/workspace", AttachStdout: true, AttachStderr: true, NetworkDisabled: p.Network == "none", Labels: map[string]string{"io.offgrid.executor.ownership": ownership}}
	if clean != "." {
		config.WorkingDir += "/" + clean
	}
	config.HostConfig = isolatedHostConfig{ReadonlyRootfs: true, CapDrop: []string{"ALL"}, SecurityOpt: []string{"no-new-privileges:true"}, NetworkMode: p.Network, Memory: p.MemoryBytes, MemorySwap: p.MemoryBytes, NanoCPUs: int64(p.CPUMillis) * 1000000, PidsLimit: p.Pids, Mounts: []isolatedMount{mount}, Tmpfs: map[string]string{"/tmp": fmt.Sprintf("rw,nosuid,nodev,size=268435456,uid=%d,gid=%d,mode=0700", p.UID, p.GID)}}
	// No daemon-managed log files containing private command output. The owned
	// attached stream is bounded and journaled by OffGrid instead.
	config.HostConfig.LogConfig.Type = "none"
	return config, nil
}
