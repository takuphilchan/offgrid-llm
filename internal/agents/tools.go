package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// getRestrictedWritePaths returns OS-specific restricted paths
func getRestrictedWritePaths() []string {
	switch runtime.GOOS {
	case "windows":
		return []string{
			`C:\Windows`,
			`C:\Program Files`,
			`C:\Program Files (x86)`,
			`C:\ProgramData`,
			`C:\Users\Default`,
			`C:\Users\Public`,
			`C:\System Volume Information`,
		}
	case "darwin": // macOS
		return []string{
			"/System",
			"/Library",
			"/usr",
			"/bin",
			"/sbin",
			"/etc",
			"/var",
			"/private/etc",
			"/private/var",
		}
	default: // Linux and others
		return []string{
			"/etc",
			"/bin",
			"/sbin",
			"/usr",
			"/boot",
			"/dev",
			"/proc",
			"/sys",
			"/var/lib",
			"/var/log",
			"/root",
			"/lib",
			"/lib64",
			"/opt",
		}
	}
}

// getRestrictedReadPaths returns OS-specific restricted read paths
func getRestrictedReadPaths() []string {
	switch runtime.GOOS {
	case "windows":
		return []string{
			`C:\Windows\System32\config`,
			`C:\Users\Default\NTUSER.DAT`,
		}
	case "darwin":
		return []string{
			"/etc/shadow",
			"/etc/master.passwd",
			"/private/etc/shadow",
			"/private/etc/master.passwd",
		}
	default:
		return []string{
			"/etc/shadow",
			"/etc/passwd",
			"/etc/sudoers",
			"/etc/ssh",
			"/root/.ssh",
		}
	}
}

// normalizePath normalizes a path for consistent comparison across platforms
func normalizePath(path string) string {
	// Clean the path
	path = filepath.Clean(path)

	// On Windows, normalize drive letters to uppercase
	if runtime.GOOS == "windows" && len(path) >= 2 && path[1] == ':' {
		path = strings.ToUpper(path[:1]) + path[1:]
	}

	return path
}

// isSubPath checks if child is under parent directory (cross-platform)
func isSubPath(parent, child string) bool {
	parent = normalizePath(parent)
	child = normalizePath(child)

	// Ensure parent ends with separator for accurate prefix matching
	if !strings.HasSuffix(parent, string(filepath.Separator)) {
		parent = parent + string(filepath.Separator)
	}

	return strings.HasPrefix(child, parent) || child == strings.TrimSuffix(parent, string(filepath.Separator))
}

// validateSafePath checks if a path is safe to write to
func validateSafePath(path string) error {
	cleanPath := normalizePath(path)

	// Check against restricted paths
	for _, restricted := range getRestrictedWritePaths() {
		if isSubPath(restricted, cleanPath) {
			return fmt.Errorf("access denied: cannot write to system path '%s' - agents are restricted from modifying system directories", restricted)
		}
	}

	// Block hidden files in root directories
	if runtime.GOOS != "windows" {
		// Unix: block /.<anything>
		if strings.HasPrefix(cleanPath, "/.") {
			return fmt.Errorf("access denied: cannot write to hidden root files")
		}
	}

	return nil
}

// validateSafeReadPath checks if a path is safe to read from
func validateSafeReadPath(path string) error {
	cleanPath := normalizePath(path)

	// Check against restricted read paths
	for _, restricted := range getRestrictedReadPaths() {
		if isSubPath(restricted, cleanPath) {
			return fmt.Errorf("access denied: cannot read sensitive system file '%s'", restricted)
		}
	}

	return nil
}

// BuiltInTools returns a set of built-in tools the agent can use
func BuiltInTools() []api.Tool {
	return []api.Tool{
		{
			Type: "function",
			Function: api.FunctionDef{
				Name:        "calculator",
				Description: "Evaluate a mathematical expression. Use this for any calculations.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"expression": map[string]interface{}{
							"type":        "string",
							"description": "The mathematical expression to evaluate, e.g., '2 + 2', '15 * 0.15', 'sqrt(16)'",
						},
					},
					"required": []string{"expression"},
				},
			},
		},
		{
			Type: "function",
			Function: api.FunctionDef{
				Name:        "read_file",
				Description: "Read the contents of a file from the filesystem.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "The path to the file to read",
						},
					},
					"required": []string{"path"},
				},
			},
		},
		{
			Type: "function",
			Function: api.FunctionDef{
				Name:        "write_file",
				Description: "Write content to a file. Creates the file if it doesn't exist.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "The path to the file to write",
						},
						"content": map[string]interface{}{
							"type":        "string",
							"description": "The content to write to the file",
						},
					},
					"required": []string{"path", "content"},
				},
			},
		},
		{
			Type: "function",
			Function: api.FunctionDef{
				Name:        "list_files",
				Description: "List files and directories in a given path.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "The directory path to list",
						},
					},
					"required": []string{"path"},
				},
			},
		},
		{
			Type: "function",
			Function: api.FunctionDef{
				Name:        "shell",
				Description: "Execute a shell command and return the output. Use for system operations.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"command": map[string]interface{}{
							"type":        "string",
							"description": "The shell command to execute",
						},
					},
					"required": []string{"command"},
				},
			},
		},
		{
			Type: "function",
			Function: api.FunctionDef{
				Name:        "http_get",
				Description: "Make an HTTP GET request to a URL and return the response.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"url": map[string]interface{}{
							"type":        "string",
							"description": "The URL to fetch",
						},
					},
					"required": []string{"url"},
				},
			},
		},
		{
			Type: "function",
			Function: api.FunctionDef{
				Name:        "current_time",
				Description: "Get the current date and time.",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
	}
}

// ExecuteTool executes a built-in tool by name with JSON arguments
// This is a convenience wrapper for use in registries
func ExecuteTool(ctx context.Context, name string, args json.RawMessage) (string, error) {
	executor := BuiltInExecutor()
	return executor(ctx, name, args)
}

// BuiltInExecutor creates a tool executor for the built-in tools
func BuiltInExecutor() ToolExecutor {
	return func(ctx context.Context, name string, args json.RawMessage) (string, error) {
		var params map[string]interface{}
		if len(args) > 0 {
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("invalid arguments: %w", err)
			}
		}

		switch name {
		case "calculator":
			expr, _ := params["expression"].(string)
			return executeCalculator(expr)

		case "read_file":
			path, _ := params["path"].(string)
			return executeReadFile(path)

		case "write_file":
			path, _ := params["path"].(string)
			content, _ := params["content"].(string)
			return executeWriteFile(path, content)

		case "list_files":
			path, _ := params["path"].(string)
			return executeListFiles(path)

		case "shell":
			command, _ := params["command"].(string)
			return executeShell(ctx, command)

		case "http_get":
			url, _ := params["url"].(string)
			return executeHTTPGet(ctx, url)

		case "current_time":
			return time.Now().Format("2006-01-02 15:04:05 MST"), nil

		default:
			return "", fmt.Errorf("unknown tool: %s", name)
		}
	}
}

// NewSandboxedExecutor creates a tool executor that runs dangerous tools in a sandbox
func NewSandboxedExecutor(sandbox Sandbox) ToolExecutor {
	return func(ctx context.Context, name string, args json.RawMessage) (string, error) {
		var params map[string]interface{}
		if len(args) > 0 {
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("invalid arguments: %w", err)
			}
		}

		switch name {
		case "shell":
			command, _ := params["command"].(string)
			// Use sandbox for shell
			return sandbox.Execute(ctx, command, nil, nil)

		case "read_file":
			path, _ := params["path"].(string)
			content, err := sandbox.ReadFile(path)
			if err != nil {
				return "", err
			}
			// Limit output size
			if len(content) > 3000 {
				return string(content[:3000]) + "\n\n... (file truncated)", nil
			}
			return string(content), nil

		case "write_file":
			path, _ := params["path"].(string)
			content, _ := params["content"].(string)
			err := sandbox.WriteFile(path, []byte(content))
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), path), nil

		case "list_files":
			path, _ := params["path"].(string)
			return sandbox.ListFiles(path)

		default:
			// Fallback to built-in executor for safe tools
			return BuiltInExecutor()(ctx, name, args)
		}
	}
}

func executeCalculator(expr string) (string, error) {
	if expr == "" {
		return "", fmt.Errorf("expression is required")
	}

	// Use bc or python for calculation
	cmd := exec.Command("python3", "-c", fmt.Sprintf("print(eval('%s'))", expr))
	output, err := cmd.Output()
	if err != nil {
		// Fallback to bc
		cmd = exec.Command("bc", "-l")
		cmd.Stdin = strings.NewReader(expr + "\n")
		output, err = cmd.Output()
		if err != nil {
			return "", fmt.Errorf("calculation failed: %w", err)
		}
	}
	return strings.TrimSpace(string(output)), nil
}

func executeReadFile(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	// Get absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	// Security: validate path is safe to read
	if err := validateSafeReadPath(absPath); err != nil {
		return "", err
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// Limit output size to fit in context
	if len(content) > 3000 {
		return string(content[:3000]) + "\n\n... (file truncated, showing first 3000 chars)", nil
	}
	return string(content), nil
}

func executeWriteFile(path, content string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	// Security: validate path is safe
	if err := validateSafePath(path); err != nil {
		return "", err
	}

	// Get absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	// Re-validate absolute path
	if err := validateSafePath(absPath); err != nil {
		return "", err
	}

	// Create directory if needed
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), absPath), nil
}

func executeListFiles(path string) (string, error) {
	if path == "" {
		path = "."
	}

	entries, err := os.ReadDir(path)
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

func executeShell(ctx context.Context, command string) (string, error) {
	if command == "" {
		return "", fmt.Errorf("command is required")
	}

	// Security: block dangerous commands (cross-platform)
	dangerousPatterns := getDangerousCommands()
	commandLower := strings.ToLower(command)
	for _, d := range dangerousPatterns {
		if strings.Contains(commandLower, strings.ToLower(d)) {
			return "", fmt.Errorf("command blocked for safety: contains '%s'", d)
		}
	}

	// Security: block writes to system paths
	for _, restricted := range getRestrictedWritePaths() {
		restrictedLower := strings.ToLower(restricted)
		if strings.Contains(commandLower, restrictedLower) {
			// Check for write-like operations
			writeOps := []string{">", ">>", "tee ", "copy ", "move ", "del ", "rm ", "cp ", "mv "}
			for _, op := range writeOps {
				if strings.Contains(commandLower, op) {
					return "", fmt.Errorf("command blocked: cannot write to system path '%s'", restricted)
				}
			}
		}
	}

	// Execute command based on OS
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, "bash", "-c", command)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("Error: %s\nOutput: %s", err.Error(), string(output)), nil
	}

	// Limit output
	result := string(output)
	if len(result) > 2000 {
		result = result[:2000] + "\n... (output truncated)"
	}
	return result, nil
}

// getDangerousCommands returns OS-specific dangerous command patterns
func getDangerousCommands() []string {
	common := []string{
		"sudo ",
		"runas ",
	}

	switch runtime.GOOS {
	case "windows":
		return append(common, []string{
			"format ",
			"del /s /q c:\\",
			"rd /s /q c:\\",
			"rmdir /s /q",
			"reg delete",
			"bcdedit",
			"diskpart",
		}...)
	default: // Unix-like
		return append(common, []string{
			"rm ",
			"rm -rf",
			"mkfs",
			"dd if=",
			"> /dev/",
			"chmod 777 /",
			"chown root",
			":(){:|:&};:", // fork bomb
		}...)
	}
}

func executeHTTPGet(ctx context.Context, url string) (string, error) {
	if url == "" {
		return "", fmt.Errorf("url is required")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10000))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Status: %d\nBody:\n%s", resp.StatusCode, string(body)), nil
}

// ReActSystemPrompt returns the system prompt for ReAct-style reasoning
func ReActSystemPrompt(tools []api.Tool) string {
	var toolDescs strings.Builder
	for _, tool := range tools {
		toolDescs.WriteString(fmt.Sprintf("### %s\n", tool.Function.Name))
		toolDescs.WriteString(fmt.Sprintf("Description: %s\n", tool.Function.Description))

		// Include parameter schema so LLM knows exact parameter names
		params := tool.Function.Parameters
		if params != nil {
			if props, ok := params["properties"].(map[string]interface{}); ok {
				toolDescs.WriteString("Parameters:\n")
				for name, schema := range props {
					if schemaMap, ok := schema.(map[string]interface{}); ok {
						desc := ""
						paramType := "any"
						if d, ok := schemaMap["description"].(string); ok {
							desc = d
						}
						if t, ok := schemaMap["type"].(string); ok {
							paramType = t
						}
						toolDescs.WriteString(fmt.Sprintf("  - %s (%s): %s\n", name, paramType, desc))
					}
				}
			}
			if required, ok := params["required"].([]interface{}); ok && len(required) > 0 {
				reqs := make([]string, 0, len(required))
				for _, r := range required {
					if s, ok := r.(string); ok {
						reqs = append(reqs, s)
					}
				}
				toolDescs.WriteString(fmt.Sprintf("Required: %s\n", strings.Join(reqs, ", ")))
			}
		}
		toolDescs.WriteString("\n")
	}

	return fmt.Sprintf(`You are a helpful AI assistant that can use tools to accomplish tasks.

Available Tools:
%s
You MUST follow this exact format for EVERY response:

Thought: [Your reasoning about what to do next]
Action: [tool_name]
Action Input: {"param": "value"}

After receiving an observation, continue with another Thought/Action or provide the final answer:

Thought: [Your reasoning about the observation]
Answer: [Your final answer to the user]

IMPORTANT RULES:
1. Always start with "Thought:" to explain your reasoning
2. For simple questions (math, facts, general knowledge), answer directly without tools
3. Use "Action:" and "Action Input:" ONLY when you genuinely need a tool
4. Use "Answer:" when you have the final answer - don't repeat tool calls
5. Action Input must be valid JSON
6. Only use the tools listed above - do not invent new tools
7. If a tool fails or doesn't exist, provide the best answer you can or explain why you cannot help

Example 1 - Simple question (no tools needed):
User: What is 2+2?

Thought: This is simple arithmetic I can answer directly.
Answer: 2+2 equals 4.

Example 2 - Tool needed:
User: What is the current time?

Thought: I need to check the current time using the current_time tool.
Action: current_time
Action Input: {}

Observation: 2024-01-15 10:30:00 UTC

Thought: I now have the current time.
Answer: The current time is 2024-01-15 10:30:00 UTC.`, toolDescs.String())
}
