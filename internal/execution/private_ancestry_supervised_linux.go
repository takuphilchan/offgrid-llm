package execution

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// Only the native parent can observe original host-root ownership before the
// worker enters its user namespace. These witnesses are private runtime input,
// never a grant field, model argument or process-global permission exception.
type supervisedRootAncestor struct {
	Path     string
	Identity string
	Mode     os.FileMode
}

type supervisedStorageAncestry struct {
	HostUID  uint32
	Registry string
	Journal  string
	Roots    []supervisedRootAncestor
}

func exactUserMapping(first, outside, count string) bool {
	file, err := os.Open("/proc/self/uid_map")
	if err != nil {
		return false
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 257))
	fields := strings.Fields(string(data))
	return err == nil && len(data) <= 256 && len(fields) == 3 && fields[0] == first && fields[1] == outside && fields[2] == count
}

func captureSupervisedStorageAncestry(journal string) (*supervisedStorageAncestry, error) {
	// UID zero in an already-remapped namespace is not original host root.
	// Nested native supervisors must not mint replacement host-root witnesses.
	if !exactUserMapping("0", "0", "4294967295") {
		return nil, ErrAuthority
	}
	registry, err := AccountHostProtectionRegistry()
	if err != nil {
		return nil, err
	}
	proof := &supervisedStorageAncestry{HostUID: uint32(os.Geteuid()), Registry: registry.directory, Journal: journal}
	seen := map[string]bool{}
	for _, directory := range []string{proof.Registry, proof.Journal} {
		if !protectionPathValid(directory) || filepath.Clean(directory) != directory {
			return nil, ErrPath
		}
		if err := checkPrivateAncestry(directory, func(path string, info os.FileInfo) error {
			if err := checkPrivateAncestor(path, info); err != nil {
				return err
			}
			stat, ok := info.Sys().(*syscall.Stat_t)
			if !ok {
				return ErrAuthority
			}
			if stat.Uid != 0 || seen[path] {
				return nil
			}
			identity, err := folderIdentity(path)
			if err != nil {
				return err
			}
			if len(proof.Roots) >= 128 {
				return ErrLimit
			}
			proof.Roots = append(proof.Roots, supervisedRootAncestor{Path: path, Identity: identity, Mode: info.Mode()})
			seen[path] = true
			return nil
		}); err != nil {
			return nil, err
		}
	}
	return proof, nil
}

// SupervisedHostProtectionRegistry is exclusively for the pinned stdio worker.
// Runtime input arrives from its native parent, separately from public grants.
// The ordinary account constructor and every other registry keep normal checks.
func SupervisedHostProtectionRegistry(privateRuntime, journal string) (*HostProtectionRegistry, error) {
	registry, err := AccountHostProtectionRegistry()
	if err != nil || privateRuntime == "" {
		return registry, err
	}
	var pin supervisedRuntime
	if len(privateRuntime) > 65536 || decodeStrict([]byte(privateRuntime), &pin) != nil || pin.Version != 2 || pin.Storage == nil || pin.Runtime.Verify(context.Background()) != nil {
		return nil, ErrAuthority
	}
	proof := pin.Storage
	if os.Geteuid() != 0 || !exactUserMapping("0", strconv.FormatUint(uint64(proof.HostUID), 10), "1") || proof.Registry != registry.directory || proof.Journal != journal {
		return nil, ErrAuthority
	}
	check, err := proof.checker()
	if err != nil {
		return nil, err
	}
	for _, directory := range []string{registry.directory, journal} {
		if err := check(directory); err != nil {
			return nil, err
		}
	}
	registry.ancestry = check
	return registry, nil
}

func (proof *supervisedStorageAncestry) checker() (func(string) error, error) {
	if proof == nil || len(proof.Roots) == 0 || len(proof.Roots) > 128 {
		return nil, ErrAuthority
	}
	ancestors := map[string]bool{}
	for _, directory := range []string{proof.Registry, proof.Journal} {
		if !protectionPathValid(directory) || filepath.Clean(directory) != directory {
			return nil, ErrAuthority
		}
		for path := directory; ; path = filepath.Dir(path) {
			ancestors[path] = true
			if filepath.Dir(path) == path {
				break
			}
		}
	}
	roots := map[string]supervisedRootAncestor{}
	for _, root := range proof.Roots {
		if !ancestors[root.Path] || !validIdentity(root.Identity) || root.Mode&os.ModeType != os.ModeDir {
			return nil, ErrAuthority
		}
		if _, duplicate := roots[root.Path]; duplicate {
			return nil, ErrAuthority
		}
		roots[root.Path] = root
	}
	return func(directory string) error {
		if directory != proof.Registry && directory != proof.Journal && directory != filepath.Dir(proof.Registry) {
			return ErrAuthority
		}
		return checkPrivateAncestry(directory, func(path string, info os.FileInfo) error {
			root, ok := roots[path]
			if !ok {
				return checkPrivateAncestor(path, info)
			}
			identity, err := folderIdentity(path)
			if err != nil || identity != root.Identity || root.Mode != info.Mode() {
				return ErrAuthority
			}
			return checkPrivateAncestorPermissions(path, info)
		})
	}, nil
}
