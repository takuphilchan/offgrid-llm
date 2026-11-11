package audio

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Desktop resources/bin/offgrid and standalone bin/offgrid share the same
// sibling speech-runtime layout. Never resolve executable code from a model
// package or from the caller's working directory.
func packagedSpeechExecutable(adapter string, names []string) string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		executable = resolved
	}
	return speechExecutableBeside(executable, adapter, names)
}

func speechExecutableBeside(executable, adapter string, names []string) string {
	roots := []string{
		filepath.Join(filepath.Dir(filepath.Dir(executable)), "speech-runtime", adapter),
		filepath.Join(filepath.Dir(executable), "audio", adapter),
	}
	for _, root := range roots {
		for _, name := range names {
			candidate := filepath.Join(root, name)
			info, err := os.Lstat(candidate)
			if err == nil && info.Mode().IsRegular() {
				return candidate
			}
		}
	}
	return ""
}

// findWhisper looks for whisper binary
func (e *Engine) findWhisper() string {
	names := []string{"whisper", "whisper-cli", "whisper-cpp", "main"}
	if runtime.GOOS == "windows" {
		names = []string{"whisper.exe", "whisper-cli.exe", "whisper-cpp.exe", "main.exe"}
	}

	if path := packagedSpeechExecutable("whisper", names); path != "" {
		return path
	}
	// Preserve legacy user-installed runtime locations.
	for _, name := range names {
		path := filepath.Join(e.whisperDir, name)
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			return path
		}
	}

	// Check in PATH
	for _, name := range names {
		// "main" is an old whisper build name, not a safe PATH identity.
		if name == "main" || name == "main.exe" {
			continue
		}
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}

	return ""
}

// findPiper looks for piper binary
func (e *Engine) findPiper() string {
	names := []string{"piper", "piper-tts"}
	if runtime.GOOS == "windows" {
		names = []string{"piper.exe", "piper-tts.exe"}
	}
	if path := packagedSpeechExecutable("piper", names); path != "" {
		return path
	}

	// Check in piper directory and subdirectory (release extracts to piper/)
	searchDirs := []string{
		e.piperDir,
		filepath.Join(e.piperDir, "piper"),
	}

	for _, dir := range searchDirs {
		for _, name := range names {
			path := filepath.Join(dir, name)
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				return path
			}
		}
	}

	// Check in PATH
	for _, name := range names {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}

	return ""
}
