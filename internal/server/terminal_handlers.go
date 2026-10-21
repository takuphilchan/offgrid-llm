package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// handleTerminalExec executes offgrid commands from the browser terminal
func (s *Server) handleTerminalExec(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var req struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Security: Only allow offgrid commands
	if req.Command != "offgrid" {
		writeError(w, "Only 'offgrid' commands are allowed", http.StatusForbidden)
		return
	}

	// Find offgrid binary
	offgridPath, err := os.Executable()
	if err != nil {
		// Fallback to PATH
		offgridPath, err = exec.LookPath("offgrid")
		if err != nil {
			writeError(w, "offgrid binary not found", http.StatusInternalServerError)
			return
		}
	}

	// Execute command
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, offgridPath, req.Args...)

	// Capture output
	output, err := cmd.CombinedOutput()

	response := map[string]interface{}{
		"output":   string(output),
		"exitCode": 0,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			response["exitCode"] = exitErr.ExitCode()
		} else {
			response["error"] = err.Error()
			response["exitCode"] = 1
		}
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

// handleTerminalExecStream executes offgrid commands with real-time streaming output
func (s *Server) handleTerminalExecStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Security: Only allow offgrid commands
	if req.Command != "offgrid" {
		http.Error(w, "Only 'offgrid' commands are allowed", http.StatusForbidden)
		return
	}

	// Find offgrid binary
	offgridPath, err := os.Executable()
	if err != nil {
		// Fallback to PATH
		offgridPath, err = exec.LookPath("offgrid")
		if err != nil {
			http.Error(w, "offgrid binary not found", http.StatusInternalServerError)
			return
		}
	}

	// Set up Server-Sent Events
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Execute command
	// Use a very long timeout for downloads (1 hour), shorter for others
	timeout := 120 * time.Second
	if len(req.Args) > 0 && (req.Args[0] == "download" || req.Args[0] == "download-hf") {
		timeout = 1 * time.Hour
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, offgridPath, req.Args...)

	// Create pipes for stdout and stderr
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		sendSSE(w, flusher, "error", fmt.Sprintf("Failed to create stdout pipe: %v", err))
		return
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		sendSSE(w, flusher, "error", fmt.Sprintf("Failed to create stderr pipe: %v", err))
		return
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		sendSSE(w, flusher, "error", fmt.Sprintf("Failed to start command: %v", err))
		return
	}

	// Channel to signal completion
	done := make(chan bool)

	// Mutex and flag to prevent SSE writes after disconnect
	var sseMu sync.Mutex
	sseOpen := true

	// Safe SSE sender that checks if connection is still open
	safeSendSSE := func(eventType, data string) {
		sseMu.Lock()
		defer sseMu.Unlock()
		if sseOpen {
			sendSSE(w, flusher, eventType, data)
		}
	}

	// Stream stdout
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stdoutPipe.Read(buf)
			if n > 0 {
				safeSendSSE("output", string(buf[:n]))
			}
			if err != nil {
				break
			}
		}
	}()

	// Stream stderr
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stderrPipe.Read(buf)
			if n > 0 {
				safeSendSSE("output", string(buf[:n]))
			}
			if err != nil {
				break
			}
		}
	}()

	// Wait for command to complete
	go func() {
		defer close(done)
		err := cmd.Wait()
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = 1
			}
		}
		safeSendSSE("exit", fmt.Sprintf("%d", exitCode))
	}()

	// Wait for completion or client disconnect
	select {
	case <-done:
		// Command finished normally
	case <-r.Context().Done():
		// Client disconnected - mark SSE as closed before killing process
		sseMu.Lock()
		sseOpen = false
		sseMu.Unlock()
		cmd.Process.Kill()
	}
}

// sendSSE sends a Server-Sent Event with the given event type and data
func sendSSE(w http.ResponseWriter, flusher http.Flusher, eventType, data string) {
	// Recover from any panics (e.g., writing to closed connection)
	defer func() {
		if r := recover(); r != nil {
			log.Printf("SSE send recovered from panic: %v", r)
		}
	}()

	// SSE format: multi-line data must have each line prefixed with "data: "
	// Split by newlines and prefix each line
	fmt.Fprintf(w, "event: %s\n", eventType)
	lines := strings.Split(data, "\n")
	for _, line := range lines {
		fmt.Fprintf(w, "data: %s\n", line)
	}
	fmt.Fprintf(w, "\n")
	if flusher != nil {
		flusher.Flush()
	}
}
