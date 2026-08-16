package agents

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Sandbox defines the interface for executing code safely
type Sandbox interface {
	// Execute runs a command within the sandbox
	Execute(ctx context.Context, cmd string, args []string, env map[string]string) (string, error)
	// WriteFile writes a file to the sandbox filesystem
	WriteFile(path string, content []byte) error
	// ReadFile reads a file from the sandbox filesystem
	ReadFile(path string) ([]byte, error)
	// ListFiles lists files in a directory
	ListFiles(path string) (string, error)
	// Cleanup cleans up sandbox resources
	Cleanup() error
}

// LocalSandbox implements a restricted local execution environment
// This is a "soft" sandbox that relies on path validation and command blocking
type LocalSandbox struct {
	workDir string
}

// NewLocalSandbox creates a new local sandbox in a temporary directory
func NewLocalSandbox() (*LocalSandbox, error) {
	workDir, err := os.MkdirTemp("", "offgrid_sandbox_*")
	if err != nil {
		return nil, fmt.Errorf("failed to create sandbox dir: %w", err)
	}
	return &LocalSandbox{workDir: workDir}, nil
}

func (s *LocalSandbox) Execute(ctx context.Context, cmd string, args []string, env map[string]string) (string, error) {
	// Security check: block dangerous commands
	if isDangerousCommand(cmd) {
		return "", fmt.Errorf("command '%s' is blocked for security", cmd)
	}

	c := exec.CommandContext(ctx, cmd, args...)
	c.Dir = s.workDir

	// Keep credentials and unrelated host configuration out of agent-created
	// processes. Callers can add explicit variables below.
	c.Env = minimalEnvironment()
	for k, v := range env {
		c.Env = append(c.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Capture output
	output, err := c.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("execution failed: %w\nOutput: %s", err, string(output))
	}

	return string(output), nil
}

func (s *LocalSandbox) WriteFile(path string, content []byte) error {
	fullPath, err := s.resolvePath(path)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(fullPath, content, 0644)
}

func (s *LocalSandbox) ReadFile(path string) ([]byte, error) {
	fullPath, err := s.resolvePath(path)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(fullPath)
}

func (s *LocalSandbox) ListFiles(path string) (string, error) {
	if path == "" {
		path = "."
	}
	fullPath, err := s.resolvePath(path)
	if err != nil {
		return "", err
	}

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to list directory: %w", err)
	}

	var result strings.Builder
	for _, entry := range entries {
		info, _ := entry.Info()
		if entry.IsDir() {
			result.WriteString(fmt.Sprintf("[DIR]  %s/\n", entry.Name()))
		} else {
			size := int64(0)
			if info != nil {
				size = info.Size()
			}
			result.WriteString(fmt.Sprintf("[FILE] %s (%d bytes)\n", entry.Name(), size))
		}
	}
	return result.String(), nil
}

func (s *LocalSandbox) resolvePath(path string) (string, error) {
	if filepath.IsAbs(path) {
		return "", fmt.Errorf("access denied: absolute paths are not allowed in sandbox")
	}
	clean := filepath.Clean(path)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("access denied: path escapes sandbox")
	}
	current := s.workDir
	if clean == "." {
		return current, nil
	}
	for _, component := range strings.Split(clean, string(filepath.Separator)) {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("access denied: symlinks are not allowed in sandbox paths")
		}
	}
	return current, nil
}

func (s *LocalSandbox) Cleanup() error {
	return os.RemoveAll(s.workDir)
}

// DockerSandbox implements a containerized sandbox
type DockerSandbox struct {
	containerID string
	image       string
}

// NewDockerSandbox creates a new docker sandbox
// Requires "docker" command to be available
func NewDockerSandbox(image string) (*DockerSandbox, error) {
	if image == "" {
		image = "python:3.10-slim" // Default safe image
	}

	// Check if docker is available
	if _, err := exec.LookPath("docker"); err != nil {
		return nil, fmt.Errorf("docker not found")
	}

	// Start container in detached mode, keeping it alive
	cmd := exec.Command("docker", "run", "-d", "--rm", "--network=none", "--memory=512m", "--cpus=1.0", image, "tail", "-f", "/dev/null")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to start container: %w\n%s", err, string(output))
	}

	containerID := strings.TrimSpace(string(output))
	return &DockerSandbox{containerID: containerID, image: image}, nil
}

func (s *DockerSandbox) Execute(ctx context.Context, cmd string, args []string, env map[string]string) (string, error) {
	// Construct exec command
	execArgs := []string{"exec"}

	// Add env vars
	for k, v := range env {
		execArgs = append(execArgs, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	execArgs = append(execArgs, s.containerID, cmd)
	execArgs = append(execArgs, args...)

	c := exec.CommandContext(ctx, "docker", execArgs...)
	output, err := c.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("execution failed: %w", err)
	}
	return string(output), nil
}

func (s *DockerSandbox) WriteFile(path string, content []byte) error {
	// Use docker cp or pipe to write file
	// Simpler approach: echo content > file inside container
	// But for binary/large files, we need a pipe

	cmd := exec.Command("docker", "exec", "-i", s.containerID, "sh", "-c", fmt.Sprintf("cat > %s", path))
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	go func() {
		defer stdin.Close()
		stdin.Write(content)
	}()

	return cmd.Run()
}

func (s *DockerSandbox) ReadFile(path string) ([]byte, error) {
	cmd := exec.Command("docker", "exec", s.containerID, "cat", path)
	return cmd.Output()
}

func (s *DockerSandbox) ListFiles(path string) (string, error) {
	if path == "" {
		path = "."
	}
	// Use ls -la inside the container
	cmd := exec.Command("docker", "exec", s.containerID, "ls", "-la", path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to list files: %w", err)
	}
	return string(output), nil
}

func (s *DockerSandbox) Cleanup() error {
	if s.containerID != "" {
		return exec.Command("docker", "kill", s.containerID).Run()
	}
	return nil
}

// Helper to check for dangerous commands in LocalSandbox
func isDangerousCommand(cmd string) bool {
	dangerous := []string{
		"rm", "mv", "dd", "mkfs", "shutdown", "reboot", "wget", "curl", "nc", "netcat", "ssh", "scp",
	}

	cmdBase := filepath.Base(cmd)
	for _, d := range dangerous {
		if cmdBase == d {
			return true
		}
	}
	return false
}
