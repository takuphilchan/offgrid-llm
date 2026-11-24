//go:build !windows

package inference

import "os/exec"

// Preserve the existing native-worker lifecycle on other platforms. This is
// not descendant ownership evidence for local Ollama/vLLM admission; that
// requires an independently verified containment implementation.
func startProviderProcess(cmd *exec.Cmd) (func(*exec.Cmd) error, error) {
	if err := startOwnedProcess(cmd); err != nil {
		return nil, err
	}
	return terminateProviderProcess, nil
}
