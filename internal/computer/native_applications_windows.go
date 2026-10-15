//go:build windows && amd64

package computer

import (
	"context"
	"debug/pe"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	ole "github.com/go-ole/go-ole"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// Resolve ordinary OS registrations to a zero-argument GUI executable. Parsing
// a registration never executes it. Packaged apps use a distinct registered
// activation adapter; command/wrapper shortcuts are not shell fallbacks.
func installedNativeApplications() ([]installedApplication, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	initialized := ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED)
	if initialized == nil {
		defer ole.CoUninitialize()
	} else if e, ok := initialized.(*ole.OleError); ok && e.Code() == 1 {
		defer ole.CoUninitialize()
	}
	apps, _ := installedPackagedApplications()
	seen := map[string]bool{}
	add := func(name, path string, validate func() error) {
		var err error
		path, err = registeredApplicationPath(path)
		if err != nil || !ordinaryGUIExecutable(path, name) {
			return
		}
		key := strings.ToLower(path)
		if seen[key] {
			for i := range apps {
				if strings.EqualFold(apps[i].path, path) {
					addApplicationAlias(&apps[i].NativeApplication, name)
				}
			}
			return
		}
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			return
		}
		app := installedApplication{NativeApplication: NativeApplication{ID: nativeID(), Name: name}, path: path, identity: info, kind: applicationExecutable, validateExecutableRegistration: validate}
		if checkInstalledApplication(app) == nil {
			for i, existing := range apps {
				if os.SameFile(existing.identity, info) {
					addApplicationAlias(&apps[i].NativeApplication, name)
					return
				}
			}
			seen[key] = true
			apps = append(apps, app)
		}
	}
	// Prefer user-facing Start Menu names. Never Resolve a shortcut (which can
	// search a network or move its target), follow links, or run its arguments.
	for _, folder := range []*windows.KNOWNFOLDERID{windows.FOLDERID_Programs, windows.FOLDERID_CommonPrograms} {
		root, err := windows.KnownFolderPath(folder, 0)
		if err != nil || strings.HasPrefix(root, `\\`) {
			continue
		}
		visited := 0
		_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			visited++
			if visited > 4096 {
				return filepath.SkipAll
			}
			if err != nil {
				return nil
			}
			if entry.Type()&os.ModeSymlink != 0 {
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if entry.IsDir() {
				if info, e := entry.Info(); e != nil || info.Sys().(*syscall.Win32FileAttributeData).FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.EqualFold(filepath.Ext(path), ".lnk") {
				if target, e := shortcutExecutable(path); e == nil {
					registration, e := os.Lstat(path)
					if e != nil || !registration.Mode().IsRegular() {
						return nil
					}
					add(strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())), target, func() error {
						current, e := os.Lstat(path)
						if e != nil || !os.SameFile(current, registration) || current.Size() != registration.Size() || !current.ModTime().Equal(registration.ModTime()) {
							return ErrControlScope
						}
						resolved, e := shortcutExecutable(path)
						if e != nil || !strings.EqualFold(resolved, target) {
							return ErrControlScope
						}
						return nil
					})
				}
			}
			return nil
		})
	}
	for _, hive := range []registry.Key{registry.CURRENT_USER, registry.LOCAL_MACHINE} {
		for _, view := range []uint32{registry.WOW64_64KEY, registry.WOW64_32KEY} {
			key, err := registry.OpenKey(hive, `Software\Microsoft\Windows\CurrentVersion\App Paths`, registry.ENUMERATE_SUB_KEYS|view)
			if err != nil {
				continue
			}
			names, _ := key.ReadSubKeyNames(4096)
			for _, name := range names {
				child, e := registry.OpenKey(key, name, registry.QUERY_VALUE|view)
				if e != nil {
					continue
				}
				path, _, e := child.GetStringValue("")
				child.Close()
				if e == nil {
					add(strings.TrimSuffix(name, filepath.Ext(name)), path, func() error {
						current, e := registry.OpenKey(hive, `Software\Microsoft\Windows\CurrentVersion\App Paths\`+name, registry.QUERY_VALUE|view)
						if e != nil {
							return ErrControlScope
						}
						defer current.Close()
						value, _, e := current.GetStringValue("")
						if e != nil || value != path {
							return ErrControlScope
						}
						return nil
					})
				}
			}
			key.Close()
		}
	}
	return apps, nil
}

func registeredApplicationPath(value string) (string, error) {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		value = value[1 : len(value)-1]
	}
	if strings.ContainsAny(value, "\"\x00\r\n") {
		return "", ErrControlScope
	}
	expanded, err := registry.ExpandString(value)
	if err != nil {
		return "", ErrControlScope
	}
	value = filepath.Clean(expanded)
	if !filepath.IsAbs(value) || len(filepath.VolumeName(value)) != 2 || strings.Contains(value[2:], ":") || !strings.EqualFold(filepath.Ext(value), ".exe") {
		return "", ErrControlScope
	}
	return value, nil
}

func ordinaryGUIExecutable(path, name string) bool {
	// This is only launch admission, not a claim that GUI automation can classify
	// every dangerous action. Native control policy remains independently enforced.
	lower := strings.ToLower(filepath.Base(path) + " " + name)
	for _, word := range []string{"uninstall", "installer", "setup", "powershell", "pwsh", "cmd.exe", "rundll32", "regsvr32", "mshta", "wscript", "cscript", "pythonw", "regedit", "mmc.exe"} {
		if strings.Contains(lower, word) {
			return false
		}
	}
	f, err := pe.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	if f.Characteristics&pe.IMAGE_FILE_DLL != 0 {
		return false
	}
	switch h := f.OptionalHeader.(type) {
	case *pe.OptionalHeader32:
		return h.Subsystem == pe.IMAGE_SUBSYSTEM_WINDOWS_GUI
	case *pe.OptionalHeader64:
		return h.Subsystem == pe.IMAGE_SUBSYSTEM_WINDOWS_GUI
	}
	return false
}

func shortcutExecutable(path string) (string, error) {
	link, err := ole.CreateInstance(ole.NewGUID("{00021401-0000-0000-C000-000000000046}"), ole.NewGUID("{000214F9-0000-0000-C000-000000000046}"))
	if err != nil {
		return "", err
	}
	defer link.Release()
	persist, err := link.QueryInterface(ole.NewGUID("{0000010b-0000-0000-C000-000000000046}"))
	if err != nil {
		return "", err
	}
	defer persist.Release()
	file, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", err
	}
	if err = comCall(&persist.IUnknown, 5, uintptr(unsafe.Pointer(file)), 0); err != nil {
		return "", err
	}
	args := make([]uint16, 32768)
	if err = comCall(link, 10, uintptr(unsafe.Pointer(&args[0])), uintptr(len(args))); err != nil || strings.TrimSpace(windows.UTF16ToString(args)) != "" {
		return "", ErrControlScope
	}
	target := make([]uint16, 32768)
	if err = comCall(link, 3, uintptr(unsafe.Pointer(&target[0])), uintptr(len(target)), 0, 4); err != nil {
		return "", err
	}
	return registeredApplicationPath(windows.UTF16ToString(target))
}

// Keep a non-write/non-delete shared handle open across CreateProcess. Both the
// catalog identity and the opened file are rechecked before spawning. The child
// inherits neither IPC nor credentials and is not killed when the task ends.
func launchNativeApplication(ctx context.Context, app installedApplication) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := checkInstalledApplication(app); err != nil {
		return err
	}
	if app.kind == applicationPackaged {
		return activatePackagedApplication(ctx, app)
	}
	path, err := windows.UTF16PtrFromString(app.path)
	if err != nil {
		return ErrControlScope
	}
	handle, err := windows.CreateFile(path, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return ErrControlScope
	}
	file := os.NewFile(uintptr(handle), app.path)
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !os.SameFile(info, app.identity) {
		return ErrControlScope
	}
	if err := checkInstalledApplication(app); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	cmd := exec.Command(app.path) // fixed catalog executable, zero arguments, no shell
	cmd.Dir = filepath.Dir(app.path)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}
	cmd.Env = cleanApplicationEnvironment(os.Environ())
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("computer_application_unavailable")
	}
	go cmd.Wait()
	return nil
}

func (d *NativeWindows) DesktopIdentity() (TargetIdentity, error) {
	if !unlockedDesktop() {
		return TargetIdentity{}, ErrControlScope
	}
	_, session, err := processGeneration(windows.GetCurrentProcessId())
	if err != nil {
		return TargetIdentity{}, err
	}
	if d.desktop.ID == "" {
		id := nativeID()
		d.desktop = TargetIdentity{ID: id, OSSession: session, ProcessGeneration: id, Surface: "desktop", Driver: "windows-uia"}
	}
	if d.desktop.OSSession != session {
		return TargetIdentity{}, ErrControlScope
	}
	return d.desktop, nil
}

func (d *NativeWindows) TargetExecutable(target NativeTarget) (string, error) {
	if !unlockedDesktop() {
		return "", ErrControlScope
	}
	surface, ok := d.targets[target.Identity.ID]
	if !ok || surface.target.Identity != target.Identity {
		return "", ErrControlScope
	}
	var owner uint32
	windowPID.Call(surface.hwnd, uintptr(unsafe.Pointer(&owner)))
	visible, _, _ := isWindowVisible.Call(surface.hwnd)
	if owner != surface.pid || visible == 0 {
		return "", ErrControlScope
	}
	generation, session, err := processGeneration(surface.pid)
	if err != nil || generation != target.Identity.ProcessGeneration || session != target.Identity.OSSession {
		return "", ErrControlScope
	}
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, surface.pid)
	if err != nil {
		return "", ErrControlScope
	}
	defer windows.CloseHandle(handle)
	name := make([]uint16, 32768)
	size := uint32(len(name))
	if windows.QueryFullProcessImageName(handle, 0, &name[0], &size) != nil {
		return "", ErrControlScope
	}
	return windows.UTF16ToString(name[:size]), nil
}
