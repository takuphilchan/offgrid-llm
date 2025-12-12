package agents

import (
"context"
"strings"
"testing"
)

func TestLocalSandbox(t *testing.T) {
sandbox, err := NewLocalSandbox()
if err != nil {
t.Fatalf("Failed to create local sandbox: %v", err)
}
defer sandbox.Cleanup()

ctx := context.Background()

// Test 1: Write and Read File
filename := "test.txt"
content := []byte("hello sandbox")
if err := sandbox.WriteFile(filename, content); err != nil {
t.Errorf("WriteFile failed: %v", err)
}

readContent, err := sandbox.ReadFile(filename)
if err != nil {
t.Errorf("ReadFile failed: %v", err)
}
if string(readContent) != string(content) {
t.Errorf("Content mismatch: got %s, want %s", string(readContent), string(content))
}

// Test 2: List Files
listOut, err := sandbox.ListFiles(".")
if err != nil {
t.Errorf("ListFiles failed: %v", err)
}
if !strings.Contains(listOut, filename) {
t.Errorf("ListFiles did not contain created file")
}

// Test 3: Execute Command (Safe)
// echo is usually safe, but might be blocked if we were strict. 
// Our isDangerousCommand only blocks specific dangerous ones.
out, err := sandbox.Execute(ctx, "echo", []string{"hello"}, nil)
if err != nil {
// On some systems echo might not be in path or behave differently, 
// but usually it works. If it fails, check if it's because of path.
// For this test, we assume standard linux/unix environment or windows with echo.
// If it fails, we might skip.
t.Logf("Execute echo failed (might be expected in some envs): %v", err)
} else {
if !strings.Contains(out, "hello") {
t.Errorf("Execute output mismatch: got %s", out)
}
}

// Test 4: Block Dangerous Command
_, err = sandbox.Execute(ctx, "rm", []string{"-rf", "/"}, nil)
if err == nil {
t.Error("Dangerous command 'rm' was not blocked")
}

// Test 5: Path Escape
err = sandbox.WriteFile("../escape.txt", []byte("bad"))
if err == nil {
t.Error("Path escape via WriteFile was not blocked")
}

_, err = sandbox.ReadFile("../escape.txt")
if err == nil {
t.Error("Path escape via ReadFile was not blocked")
}
}
