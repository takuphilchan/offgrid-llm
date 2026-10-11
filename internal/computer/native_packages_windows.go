//go:build windows && amd64

package computer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unsafe"

	ole "github.com/go-ole/go-ole"
	"golang.org/x/sys/windows"
)

var (
	packageKernel       = windows.NewLazySystemDLL("kernel32.dll")
	getPackagesByFamily = packageKernel.NewProc("GetPackagesByPackageFamily")
	getPackagePath      = packageKernel.NewProc("GetPackagePathByFullName")
	getProcessAUMID     = packageKernel.NewProc("GetApplicationUserModelId")
	getProcessPackage   = packageKernel.NewProc("GetPackageFullName")
	getAppsFolder       = windows.NewLazySystemDLL("shell32.dll").NewProc("SHGetKnownFolderItem")
)

type shellPropertyKey struct {
	Format ole.GUID
	ID     uint32
}

var appUserModelKey = shellPropertyKey{Format: *ole.NewGUID("{9F4C2855-9F79-4B39-A8D0-E1D42DE1D5F3}"), ID: 5}

// AppsFolder supplies user-facing names/AUMIDs. Only AppModel-registered package
// entries below are accepted; desktop Shell IDs are not activation commands.
// Caller owns the COM apartment. This only reads the catalog, never activates it.
func installedPackagedApplications() ([]installedApplication, error) {
	var folder *ole.IUnknown
	hr, _, _ := getAppsFolder.Call(uintptr(unsafe.Pointer(windows.FOLDERID_AppsFolder)), 0, 0,
		uintptr(unsafe.Pointer(ole.NewGUID("{43826D1E-E718-42EE-BC55-A1E261C37BFE}"))), uintptr(unsafe.Pointer(&folder)))
	if int32(hr) < 0 || folder == nil {
		return nil, ErrControlScope
	}
	defer folder.Release()
	var enumerator *ole.IUnknown
	if err := comCall(folder, 3, 0,
		uintptr(unsafe.Pointer(ole.NewGUID("{94F60519-2850-4924-AA5A-D15E84868039}"))),
		uintptr(unsafe.Pointer(ole.NewGUID("{70629033-E363-4A28-A567-0DB78006E6D7}"))), uintptr(unsafe.Pointer(&enumerator))); err != nil || enumerator == nil {
		return nil, ErrControlScope
	}
	defer enumerator.Release()
	apps := []installedApplication{}
	seen := map[string]bool{}
	for i := 0; i < 4096; i++ {
		var item *ole.IUnknown
		var fetched uint32
		if err := comCall(enumerator, 3, 1, uintptr(unsafe.Pointer(&item)), uintptr(unsafe.Pointer(&fetched))); err != nil {
			return nil, err
		}
		if fetched != 1 || item == nil {
			break
		}
		name, aumid := packagedShellItem(item)
		item.Release()
		if seen[aumid] || name == "" || len(name) > 256 {
			continue
		}
		app, err := resolvePackagedApplication(aumid, name)
		if err != nil {
			continue
		}
		app.ID = nativeID()
		app.validateRegistration = validatePackagedRegistration
		if checkInstalledApplication(app) != nil {
			continue
		}
		seen[aumid] = true
		apps = append(apps, app)
	}
	return apps, nil
}

func packagedShellItem(item *ole.IUnknown) (string, string) {
	var name *uint16
	if comCall(item, 5, 0, uintptr(unsafe.Pointer(&name))) != nil || name == nil {
		return "", ""
	}
	defer ole.CoTaskMemFree(uintptr(unsafe.Pointer(name)))
	props, err := item.QueryInterface(ole.NewGUID("{7E9FB0D3-919F-4307-AB2E-9B1860310C93}"))
	if err != nil {
		return "", ""
	}
	defer props.Release()
	var id *uint16
	if comCall(&props.IUnknown, 17, uintptr(unsafe.Pointer(&appUserModelKey)), uintptr(unsafe.Pointer(&id))) != nil || id == nil {
		return "", ""
	}
	defer ole.CoTaskMemFree(uintptr(unsafe.Pointer(id)))
	return windows.UTF16PtrToString(name), windows.UTF16PtrToString(id)
}

func packageFullNames(family string) ([]string, error) {
	value, err := windows.UTF16PtrFromString(family)
	if err != nil || family == "" {
		return nil, ErrControlScope
	}
	var count, size uint32
	code, _, _ := getPackagesByFamily.Call(uintptr(unsafe.Pointer(value)), uintptr(unsafe.Pointer(&count)), 0, uintptr(unsafe.Pointer(&size)), 0)
	if code == 0 && count == 0 {
		return nil, nil
	}
	if code != uintptr(windows.ERROR_INSUFFICIENT_BUFFER) || count == 0 || count > 256 || size == 0 || size > 128*1024 {
		return nil, ErrControlScope
	}
	names := make([]*uint16, count)
	buffer := make([]uint16, size)
	code, _, _ = getPackagesByFamily.Call(uintptr(unsafe.Pointer(value)), uintptr(unsafe.Pointer(&count)), uintptr(unsafe.Pointer(&names[0])), uintptr(unsafe.Pointer(&size)), uintptr(unsafe.Pointer(&buffer[0])))
	if code != 0 || count > uint32(len(names)) {
		return nil, ErrControlScope
	}
	result := make([]string, 0, count)
	for _, name := range names[:count] {
		if name == nil {
			return nil, ErrControlScope
		}
		result = append(result, windows.UTF16PtrToString(name))
	}
	runtime.KeepAlive(buffer)
	return result, nil
}

func packageInstallPath(fullName string) (string, error) {
	name, err := windows.UTF16PtrFromString(fullName)
	if err != nil {
		return "", ErrControlScope
	}
	var size uint32
	code, _, _ := getPackagePath.Call(uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(&size)), 0)
	if code != uintptr(windows.ERROR_INSUFFICIENT_BUFFER) || size < 2 || size > 32768 {
		return "", ErrControlScope
	}
	buffer := make([]uint16, size)
	code, _, _ = getPackagePath.Call(uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(&size)), uintptr(unsafe.Pointer(&buffer[0])))
	if code != 0 {
		return "", ErrControlScope
	}
	path := windows.UTF16ToString(buffer)
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil || !strings.EqualFold(filepath.Clean(path), filepath.Clean(resolved)) || len(filepath.VolumeName(path)) != 2 {
		return "", ErrControlScope
	}
	return filepath.Clean(path), nil
}

type packageManifest struct {
	XMLName      xml.Name `xml:"Package"`
	Applications []struct {
		ID         string `xml:"Id,attr"`
		Executable string `xml:"Executable,attr"`
	} `xml:"Applications>Application"`
}

// The OS chooses the package root; metadata never chooses an arbitrary path or
// arguments. Resource/framework packages and unsupported hosted/DLL entries have
// no directly verifiable GUI executable and are omitted, never shell-launched.
func packageEntry(root, entry string) (string, string, error) {
	manifestPath := filepath.Join(root, "AppxManifest.xml")
	resolved, err := filepath.EvalSymlinks(manifestPath)
	if err != nil || !strings.EqualFold(manifestPath, resolved) {
		return "", "", ErrControlScope
	}
	f, err := os.Open(manifestPath)
	if err != nil {
		return "", "", err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 1024*1024+1))
	if err != nil || len(data) > 1024*1024 {
		return "", "", ErrControlScope
	}
	var manifest packageManifest
	if xml.Unmarshal(data, &manifest) != nil {
		return "", "", ErrControlScope
	}
	executable := ""
	for _, app := range manifest.Applications {
		if app.ID != entry {
			continue
		}
		if executable != "" || app.Executable == "" {
			return "", "", ErrControlScope
		}
		executable = app.Executable
	}
	if executable == "" || filepath.IsAbs(executable) || strings.ContainsAny(executable, ":\"\x00\r\n") {
		return "", "", ErrControlScope
	}
	path := filepath.Join(root, executable)
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", "", ErrControlScope
	}
	path, err = registeredApplicationPath(path)
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256(data)
	return path, hex.EncodeToString(sum[:]), nil
}

func resolvePackagedApplication(aumid, name string) (installedApplication, error) {
	family, entry, ok := strings.Cut(aumid, "!")
	if !ok || family == "" || entry == "" || len(aumid) > 256 || strings.ContainsAny(aumid, "/\\\x00\r\n \t") || strings.Contains(entry, "!") {
		return installedApplication{}, ErrControlScope
	}
	fullNames, err := packageFullNames(family)
	if err != nil {
		return installedApplication{}, err
	}
	var result installedApplication
	for _, fullName := range fullNames {
		root, err := packageInstallPath(fullName)
		if err != nil {
			continue
		}
		path, revision, err := packageEntry(root, entry)
		if err != nil || !ordinaryGUIExecutable(path, name) {
			continue
		}
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		if result.packageIdentity != nil {
			return installedApplication{}, ErrControlScope
		} // ambiguous active registration
		result = installedApplication{NativeApplication: NativeApplication{Name: name}, path: path, identity: info, kind: applicationPackaged,
			packageIdentity: &packagedApplication{AUMID: aumid, Family: family, FullName: fullName, Entry: entry, Root: root, ManifestRevision: revision}}
	}
	if result.packageIdentity == nil {
		return installedApplication{}, ErrControlScope
	}
	return result, nil
}

func validatePackagedRegistration(expected packagedApplication) error {
	current, err := resolvePackagedApplication(expected.AUMID, "")
	if err != nil || current.packageIdentity == nil || *current.packageIdentity != expected {
		return ErrControlScope
	}
	return nil
}

func activatePackagedApplication(ctx context.Context, app installedApplication) error {
	if err := checkInstalledApplication(app); err != nil {
		return err
	}
	if app.kind != applicationPackaged || app.packageIdentity == nil {
		return ErrControlScope
	}
	manager, err := ole.CreateInstance(ole.NewGUID("{45BA127D-10A8-46EA-8AB7-56EA9078943C}"), ole.NewGUID("{2E941141-7F97-4756-BA1D-9DECDE894A3D}"))
	if err != nil {
		return err
	}
	defer manager.Release()
	id, err := windows.UTF16PtrFromString(app.packageIdentity.AUMID)
	if err != nil {
		return ErrControlScope
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := checkInstalledApplication(app); err != nil {
		return err
	}
	var pid uint32
	// Zero arguments and AO_NOERRORUI: failure belongs to the task, not a Shell
	// popup. The returned PID is not readiness; separately observe verified state.
	if err := comCall(manager, 3, uintptr(unsafe.Pointer(id)), 0, 2, uintptr(unsafe.Pointer(&pid))); err != nil {
		return ErrUncertain
	}
	return nil
}

func processPackageString(proc *windows.LazyProc, handle windows.Handle) (string, error) {
	var size uint32
	code, _, _ := proc.Call(uintptr(handle), uintptr(unsafe.Pointer(&size)), 0)
	if code != uintptr(windows.ERROR_INSUFFICIENT_BUFFER) || size < 2 || size > 32768 {
		return "", ErrControlScope
	}
	buffer := make([]uint16, size)
	code, _, _ = proc.Call(uintptr(handle), uintptr(unsafe.Pointer(&size)), uintptr(unsafe.Pointer(&buffer[0])))
	if code != 0 {
		return "", ErrControlScope
	}
	return windows.UTF16ToString(buffer), nil
}

func (d *NativeWindows) TargetApplication(target NativeTarget) (processApplicationIdentity, error) {
	// Reuses the existing current-process generation, session and unlocked-desktop
	// checks. Read both package values from the same process handle and recheck
	// target identity afterwards so a reused PID cannot splice identities.
	if _, err := d.TargetExecutable(target); err != nil {
		return processApplicationIdentity{}, err
	}
	surface := d.targets[target.Identity.ID]
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, surface.pid)
	if err != nil {
		return processApplicationIdentity{}, ErrControlScope
	}
	defer windows.CloseHandle(handle)
	aumid, err := processPackageString(getProcessAUMID, handle)
	if err != nil {
		return processApplicationIdentity{}, err
	}
	fullName, err := processPackageString(getProcessPackage, handle)
	if err != nil {
		return processApplicationIdentity{}, err
	}
	if _, err := d.TargetExecutable(target); err != nil {
		return processApplicationIdentity{}, err
	}
	return processApplicationIdentity{AUMID: aumid, FullName: fullName}, nil
}
