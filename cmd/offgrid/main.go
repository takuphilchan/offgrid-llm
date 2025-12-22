package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/audio"
	"github.com/takuphilchan/offgrid-llm/internal/batch"
	"github.com/takuphilchan/offgrid-llm/internal/completions"
	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/internal/output"
	"github.com/takuphilchan/offgrid-llm/internal/resource"
	"github.com/takuphilchan/offgrid-llm/internal/server"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/internal/templates"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

// Version is set via ldflags during build
var Version = "dev"

// getVersion returns the current version, reading from VERSION file if needed
func getVersion() string {
	if Version != "dev" {
		return Version
	}
	// Try to read from VERSION file for development builds
	if data, err := os.ReadFile("VERSION"); err == nil {
		return strings.TrimSpace(string(data))
	}
	// Try common locations
	for _, path := range []string{"/var/lib/offgrid/VERSION", "/usr/local/share/offgrid/VERSION"} {
		if data, err := os.ReadFile(path); err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	return Version
}

// Shared HTTP clients with connection pooling for better performance
// Avoids creating new connections for every request
var (
	// httpClient is a shared client with reasonable timeouts for most API calls
	httpClient = &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 5,
			IdleConnTimeout:     90 * time.Second,
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
		},
	}

	// httpClientLong is for long-running requests like inference
	httpClientLong = &http.Client{
		Timeout: 5 * time.Minute,
		Transport: &http.Transport{
			MaxIdleConns:        5,
			MaxIdleConnsPerHost: 2,
			IdleConnTimeout:     120 * time.Second,
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
		},
	}
)

// Check if colors should be disabled
func init() {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		disableColors()
	}
}

// Visual identity constants
var (
	// Colors (ANSI escape codes)
	colorReset   = "\033[0m"
	colorBold    = "\033[1m"
	colorDim     = "\033[2m"
	colorCyan    = "\033[36m"
	colorGreen   = "\033[32m"
	colorYellow  = "\033[33m"
	colorRed     = "\033[31m"
	colorBlue    = "\033[34m"
	colorMagenta = "\033[35m"

	// Brand colors
	brandPrimary   = "\033[36m"   // Cyan
	brandSecondary = "\033[37m"   // White
	brandAccent    = "\033[36m"   // Cyan
	brandSuccess   = "\033[36m"   // Cyan
	brandError     = "\033[1;37m" // White Bold
	brandMuted     = "\033[90m"   // Gray (Bright Black)
)

const (
	// Box drawing characters
	boxTL     = "╭"
	boxTR     = "╮"
	boxBL     = "╰"
	boxBR     = "╯"
	boxH      = "─"
	boxV      = ""
	boxVR     = "├"
	boxVL     = "┤"
	boxHD     = "┬"
	boxHU     = "┴"
	boxCross  = "┼"
	separator = "━"

	// Custom icons
	iconBolt     = "◈"
	iconCheck    = "✓"
	iconCross    = "✗"
	iconArrow    = "→"
	iconDot      = "•"
	iconStar     = "★"
	iconBox      = "▪"
	iconCircle   = "◉"
	iconDiamond  = "◆"
	iconChevron  = "›"
	iconDownload = "⇣"
	iconUpload   = "⇡"
	iconSearch   = "⌕"
	iconModel    = "◭"
	iconCpu      = "⟨⟩"
	iconGpu      = "⟪⟫"
)

func disableColors() {
	colorReset = ""
	colorBold = ""
	colorDim = ""
	colorCyan = ""
	colorGreen = ""
	colorYellow = ""
	colorRed = ""
	colorBlue = ""
	colorMagenta = ""
	brandPrimary = ""
	brandSecondary = ""
	brandAccent = ""
	brandSuccess = ""
	brandError = ""
	brandMuted = ""
}

func printBanner() {
	if output.JSONMode {
		return
	}
	fmt.Println()
	fmt.Printf("  %s%s OffGrid LLM%s %s%s%s\n", brandPrimary+colorBold, iconBolt, colorReset, brandMuted, getVersion(), colorReset)
	fmt.Printf("  %sLocal LLM inference at the edge%s\n", brandMuted, colorReset)
	fmt.Println()
}

func printSection(title string) {
	if output.JSONMode {
		return
	}
	fmt.Printf("\n%s%s%s\n\n", brandPrimary+colorBold, title, colorReset)
}

func printSuccess(message string) {
	fmt.Printf("%s%s%s %s\n", brandSuccess, iconCheck, colorReset, message)
}

func printError(message string) {
	fmt.Printf("%s%s%s %s\n", brandError, iconCross, colorReset, message)
}

func printInfo(message string) {
	fmt.Printf("%sℹ%s %s\n", brandPrimary, colorReset, message)
}

func printWarning(message string) {
	fmt.Printf("%s⚠%s %s\n", brandAccent, colorReset, message)
}

func printHelpfulError(err error, context string) {
	printError(fmt.Sprintf("%s failed: %v", context, err))
	fmt.Println()

	// Provide context-specific help
	errStr := err.Error()

	// Network errors
	if strings.Contains(errStr, "connection refused") {
		printInfo("Possible causes:")
		fmt.Println("  • OffGrid server not running")
		fmt.Println("  • Port 11611 in use by another application")
		fmt.Println()
		printInfo("Solutions:")
		fmt.Println("  • Start server: offgrid serve")
		fmt.Println("  • Check server: systemctl status llama-server")
		fmt.Println("  • Check port: lsof -i :11611")
	} else if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded") {
		printInfo("Connection timed out - possible causes:")
		fmt.Println("  • Slow internet connection")
		fmt.Println("  • HuggingFace servers overloaded")
		fmt.Println("  • Model is very large")
		fmt.Println()
		printInfo("Solutions:")
		fmt.Println("  • Try again (temporary network issue)")
		fmt.Println("  • Check internet: ping huggingface.co")
		fmt.Println("  • Use smaller model")
	} else if strings.Contains(errStr, "no such host") || strings.Contains(errStr, "DNS") {
		printInfo("Cannot reach server - check internet connection:")
		fmt.Println("  • Test connection: ping 8.8.8.8")
		fmt.Println("  • Check DNS: ping huggingface.co")
		fmt.Println("  • Try offline mode: offgrid list (local models)")
	} else if strings.Contains(errStr, "permission denied") {
		printInfo("Permission problem:")
		fmt.Println("  • Check directory permissions")
		fmt.Println("  • Models directory: ~/.offgrid/models")
		fmt.Println("  • Fix: chmod 755 ~/.offgrid && chmod 755 ~/.offgrid/models")
	} else if strings.Contains(errStr, "no space left") {
		printInfo("Disk full:")
		fmt.Println("  • Check space: df -h")
		fmt.Println("  • Free space: delete old models with offgrid remove")
		fmt.Println("  • Models are typically 2-8GB each")
	} else if strings.Contains(errStr, "out of memory") || strings.Contains(errStr, "OOM") {
		printInfo("Not enough RAM:")
		fmt.Println("  • Check available RAM: offgrid info")
		fmt.Println("  • Use smaller model: offgrid search --ram 4")
		fmt.Println("  • Close other applications")
		fmt.Println("  • See: docs/4GB_RAM.md")
	} else if strings.Contains(errStr, "model not found") || strings.Contains(errStr, "404") {
		printInfo("Model not available:")
		fmt.Println("  • Check model name is correct")
		fmt.Println("  • Model may be private or removed")
		fmt.Println("  • Search for alternatives: offgrid search <query>")
	} else if strings.Contains(errStr, "rate limit") || strings.Contains(errStr, "429") {
		printInfo("HuggingFace rate limit reached:")
		fmt.Println("  • Wait a few minutes and try again")
		fmt.Println("  • Consider using local models: offgrid list")
		fmt.Println("  • Use fewer concurrent downloads")
	}

	fmt.Println()
}

// Consistent formatting constants for terminal output
const (
	labelWidth   = 20 // Width for labels in key-value pairs
	optionWidth  = 26 // Width for option/command names
	exampleWidth = 34 // Width for example commands
	tableCol1    = 40 // Width for first table column (model names)
	tableCol2    = 12 // Width for second table column (size)
	tableCol3    = 12 // Width for third table column (quant)
)

func printItem(label, value string) {
	fmt.Printf("  %s%-*s%s %s%s%s\n", brandMuted, labelWidth, label+":", colorReset, colorBold, value, colorReset)
}

func printOption(option, description string) {
	fmt.Printf("  %s%-*s%s %s\n", brandSecondary, optionWidth, option, colorReset, description)
}

func printExample(cmd string) {
	fmt.Printf("  %s%s%s\n", colorDim, cmd, colorReset)
}

func printDivider() {
	fmt.Printf("%s%s%s\n", brandMuted, strings.Repeat(separator, 60), colorReset)
}

func printBox(title, content string) {
	width := 58
	fmt.Printf("%s%s%s%s%s\n", brandPrimary, boxTL, strings.Repeat(boxH, width), boxTR, colorReset)

	// Title
	padding := (width - len(title) - 2) / 2
	fmt.Printf("%s%s%s %s%s%s %s%s%s\n",
		brandPrimary, boxV, colorReset,
		strings.Repeat(" ", padding),
		colorBold+title+colorReset,
		strings.Repeat(" ", width-len(title)-padding-2),
		brandPrimary, boxV, colorReset)

	// Divider
	fmt.Printf("%s%s%s%s%s\n", brandPrimary, boxVR, strings.Repeat(boxH, width), boxVL, colorReset)

	// Content
	for _, line := range strings.Split(content, "\n") {
		contentPadding := width - len(stripAnsi(line))
		fmt.Printf("%s%s%s %s%s %s%s%s\n",
			brandPrimary, boxV, colorReset,
			line,
			strings.Repeat(" ", contentPadding-2),
			brandPrimary, boxV, colorReset)
	}

	// Bottom
	fmt.Printf("%s%s%s%s%s\n", brandPrimary, boxBL, strings.Repeat(boxH, width), boxBR, colorReset)
}

// printModernSection prints a command section with aligned columns
func printModernSection(title string, items [][]string) {
	fmt.Printf("  %s%s%s\n", brandPrimary, title, colorReset)
	for _, item := range items {
		if len(item) >= 2 {
			fmt.Printf("    %s›%s %-22s %s%s%s\n", brandPrimary, colorReset, item[0], colorDim, item[1], colorReset)
		}
	}
	fmt.Println()
}

// ensureOffgridServerRunning checks if OffGrid server is responding
func ensureOffgridServerRunning() error {
	cfg := config.LoadConfig()
	healthURL := fmt.Sprintf("http://localhost:%d/health", cfg.ServerPort)

	client := &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			Proxy: func(req *http.Request) (*url.URL, error) {
				return nil, nil // Bypass proxy for localhost
			},
		},
	}

	resp, err := client.Get(healthURL)
	if err != nil {
		return fmt.Errorf("server not responding: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	return nil
}

// startOffgridServerInBackground starts the OffGrid server in background
func startOffgridServerInBackground() error {
	// Start offgrid serve in background using shell
	cmd := exec.Command("sh", "-c", "offgrid serve > /dev/null 2>&1 &")
	return cmd.Run()
}

// ensureLlamaServerRunning checks if llama-server is responding
func ensureLlamaServerRunning() error {
	healthURL := "http://localhost:8081/health"

	client := &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			Proxy: func(req *http.Request) (*url.URL, error) {
				return nil, nil // Bypass proxy for localhost
			},
		},
	}

	resp, err := client.Get(healthURL)
	if err != nil {
		return fmt.Errorf("llama-server not responding: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("llama-server returned status %d", resp.StatusCode)
	}

	return nil
}

// waitForModelReady waits for the model to be fully loaded and ready to generate responses
// It performs a test completion to verify the model can actually respond
func waitForModelReady(port string, maxWaitSeconds int) error {
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			Proxy: func(req *http.Request) (*url.URL, error) {
				return nil, nil
			},
		},
	}

	startTime := time.Now()
	lastStatus := ""

	for {
		elapsed := time.Since(startTime)
		if elapsed > time.Duration(maxWaitSeconds)*time.Second {
			return fmt.Errorf("timeout waiting for model to be ready (waited %ds)", maxWaitSeconds)
		}

		// First check health endpoint
		healthResp, err := client.Get(fmt.Sprintf("http://localhost:%s/health", port))
		if err != nil {
			if lastStatus != "connecting" {
				lastStatus = "connecting"
			}
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// Parse health response to check model loading status
		var healthData struct {
			Status          string `json:"status"`
			SlotsIdle       int    `json:"slots_idle"`
			SlotsProcessing int    `json:"slots_processing"`
		}
		if err := json.NewDecoder(healthResp.Body).Decode(&healthData); err == nil {
			// llama.cpp returns "loading model" status while loading
			if healthData.Status == "loading model" {
				if lastStatus != "loading" {
					lastStatus = "loading"
				}
				healthResp.Body.Close()
				time.Sleep(1 * time.Second)
				continue
			}
			// Check if status is "ok" and we have idle slots
			if healthData.Status != "ok" {
				healthResp.Body.Close()
				time.Sleep(500 * time.Millisecond)
				continue
			}
		}
		healthResp.Body.Close()

		// Health is OK, now do a quick test completion to verify model can respond
		testPayload := map[string]interface{}{
			"model": "default",
			"messages": []map[string]string{
				{"role": "user", "content": "Hi"},
			},
			"max_tokens": 1,
			"stream":     false,
		}

		payloadBytes, _ := json.Marshal(testPayload)
		testResp, err := client.Post(
			fmt.Sprintf("http://localhost:%s/v1/chat/completions", port),
			"application/json",
			bytes.NewReader(payloadBytes),
		)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// Check if we got a valid response (not an error)
		if testResp.StatusCode == http.StatusOK {
			var result map[string]interface{}
			if err := json.NewDecoder(testResp.Body).Decode(&result); err == nil {
				// Check if we got actual choices back
				if choices, ok := result["choices"].([]interface{}); ok && len(choices) > 0 {
					testResp.Body.Close()
					return nil // Model is ready!
				}
			}
		}
		testResp.Body.Close()

		time.Sleep(500 * time.Millisecond)
	}
}

// startLlamaServerInBackground starts llama-server with the specified model
func startLlamaServerInBackground(modelPath string) error {
	// Check if llama-server exists
	llamaServerPath, err := exec.LookPath("llama-server")
	if err != nil {
		return fmt.Errorf("llama-server not found in PATH: %w", err)
	}

	// Detect system resources for optimal configuration
	res, _ := resource.DetectResources()

	// Get optimal thread count using physical core detection
	var threads int
	if res != nil {
		threads = res.GetOptimalThreads()
	} else {
		// Fallback: use half of logical cores
		cpuCores := runtime.NumCPU()
		threads = cpuCores / 2
		if threads < 1 {
			threads = 1
		}
	}

	// Adaptive context size based on available RAM
	contextSize := 4096
	if res != nil && res.AvailableRAM > 0 {
		if res.AvailableRAM < 4000 {
			contextSize = 1024
		} else if res.AvailableRAM < 6000 {
			contextSize = 2048
		} else if res.AvailableRAM < 12000 {
			contextSize = 4096
		} else {
			contextSize = 8192
		}
	}

	// Adaptive batch size - lower batch = faster time-to-first-token
	batchSize := 512
	if res != nil && res.AvailableRAM < 6000 {
		batchSize = 256
	}

	// Read port from config file, default to 42382
	port := "42382"
	if portBytes, err := os.ReadFile("/etc/offgrid/llama-port"); err == nil {
		port = strings.TrimSpace(string(portBytes))
	}

	// Build optimized command line
	// -fa: Flash attention for 20-40% faster inference
	// --cont-batching: Better throughput
	// --cache-type-k/v q8_0: Quantized KV cache saves memory with minimal quality loss
	// --cache-prompt: Cache prompt prefixes for faster repeated prompts
	var cmdStr string
	if res != nil && res.AvailableRAM < 8000 {
		// Low RAM: use mmap (won't crash, but slower first token)
		cmdStr = fmt.Sprintf("%s -m %s --port %s --host 127.0.0.1 -t %d -c %d --n-gpu-layers 0 -b %d -fa on --cont-batching --cache-type-k q8_0 --cache-type-v q8_0 --cache-reuse 256",
			llamaServerPath, modelPath, port, threads, contextSize, batchSize)
	} else {
		// Sufficient RAM: load fully into memory for speed
		cmdStr = fmt.Sprintf("%s -m %s --port %s --host 127.0.0.1 -t %d -c %d --n-gpu-layers 0 -b %d --no-mmap --mlock -fa on --cont-batching --cache-type-k q8_0 --cache-type-v q8_0 --cache-reuse 256",
			llamaServerPath, modelPath, port, threads, contextSize, batchSize)
	}

	// Start llama-server in background using shell with nohup
	cmd := exec.Command("sh", "-c", fmt.Sprintf("nohup %s > /dev/null 2>&1 &", cmdStr))

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start llama-server: %w", err)
	}

	return nil
}

func stripAnsi(str string) string {
	// Simple ANSI strip for length calculation
	result := str
	for _, code := range []string{colorReset, colorBold, colorDim, colorCyan, colorGreen, colorYellow, colorRed, colorBlue, colorMagenta, brandPrimary, brandSecondary, brandAccent, brandSuccess, brandError, brandMuted} {
		result = strings.ReplaceAll(result, code, "")
	}
	return result
}

// Modern spinner characters for loading animations
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Spinner represents an animated loading indicator
type Spinner struct {
	message string
	frame   int
	running bool
	done    chan bool
}

// NewSpinner creates a new spinner with a message
func NewSpinner(message string) *Spinner {
	return &Spinner{
		message: message,
		frame:   0,
		running: false,
		done:    make(chan bool),
	}
}

// Start begins the spinner animation
func (s *Spinner) Start() {
	if output.JSONMode {
		return
	}
	s.running = true
	go func() {
		for s.running {
			fmt.Printf("\r%s%s%s %s", brandPrimary, spinnerFrames[s.frame], colorReset, s.message)
			s.frame = (s.frame + 1) % len(spinnerFrames)
			time.Sleep(80 * time.Millisecond)
		}
		s.done <- true
	}()
}

// Stop ends the spinner with a final status
func (s *Spinner) Stop(success bool) {
	if output.JSONMode {
		return
	}
	s.running = false
	<-s.done
	if success {
		fmt.Printf("\r%s%s%s %s\n", brandSuccess, iconCheck, colorReset, s.message)
	} else {
		fmt.Printf("\r%s%s%s %s\n", brandError, iconCross, colorReset, s.message)
	}
}

// StopWithMessage ends the spinner with a custom message
func (s *Spinner) StopWithMessage(success bool, message string) {
	if output.JSONMode {
		return
	}
	s.running = false
	<-s.done
	if success {
		fmt.Printf("\r%s%s%s %s\n", brandSuccess, iconCheck, colorReset, message)
	} else {
		fmt.Printf("\r%s%s%s %s\n", brandError, iconCross, colorReset, message)
	}
}

// UpdateMessage changes the spinner message while running
func (s *Spinner) UpdateMessage(message string) {
	s.message = message
}

// ProgressBar renders a visual progress bar
type ProgressBar struct {
	total     int64
	current   int64
	width     int
	label     string
	startTime time.Time
}

// NewProgressBar creates a new progress bar
func NewProgressBar(total int64, label string) *ProgressBar {
	return &ProgressBar{
		total:     total,
		current:   0,
		width:     30,
		label:     label,
		startTime: time.Now(),
	}
}

// Update updates the progress bar with current progress
func (p *ProgressBar) Update(current int64) {
	if output.JSONMode {
		return
	}
	p.current = current
	percent := float64(current) / float64(p.total) * 100
	filled := int(float64(p.width) * percent / 100)
	empty := p.width - filled

	// Calculate speed and ETA
	elapsed := time.Since(p.startTime).Seconds()
	speed := float64(current) / elapsed / (1024 * 1024) // MB/s

	var eta string
	if speed > 0 {
		remaining := float64(p.total-current) / (speed * 1024 * 1024)
		if remaining < 60 {
			eta = fmt.Sprintf("%.0fs", remaining)
		} else {
			eta = fmt.Sprintf("%.1fm", remaining/60)
		}
	}

	// Build progress bar
	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)

	// Render
	fmt.Printf("\r  %s%s%s %s%s%s %s%.1f%%%s %s%.1f MB/s%s %s%s%s",
		brandPrimary, bar, colorReset,
		colorBold, p.label, colorReset,
		brandMuted, percent, colorReset,
		brandMuted, speed, colorReset,
		brandMuted, eta, colorReset)
}

// Complete marks the progress as done
func (p *ProgressBar) Complete() {
	if output.JSONMode {
		return
	}
	bar := strings.Repeat("█", p.width)
	fmt.Printf("\r  %s%s%s %s%s%s %s100.0%%%s\n", brandPrimary, bar, colorReset, colorBold, p.label, colorReset, brandSuccess, colorReset)
}

// Modern section headers with subtle styling
func printSectionHeader(title string) {
	if output.JSONMode {
		return
	}
	fmt.Printf("\n%s%s %s%s\n\n", brandPrimary, iconChevron, title, colorReset)
}

// Print a key-value pair with modern styling
func printKeyValue(key, value string) {
	if output.JSONMode {
		return
	}
	fmt.Printf("  %s%-16s%s %s%s%s\n", brandMuted, key, colorReset, colorBold, value, colorReset)
}

// Print a card-style box with content
func printCard(title string, lines []string) {
	if output.JSONMode {
		return
	}
	width := 56

	// Top border with title
	titleLen := len(stripAnsi(title))
	leftPad := 2
	rightPad := width - titleLen - leftPad - 2
	if rightPad < 0 {
		rightPad = 0
	}

	fmt.Printf("%s%s%s%s %s%s%s %s%s%s\n",
		brandMuted, boxTL, strings.Repeat(boxH, leftPad), colorReset,
		colorBold+brandPrimary, title, colorReset,
		brandMuted, strings.Repeat(boxH, rightPad)+boxTR, colorReset)

	// Content lines
	for _, line := range lines {
		lineLen := len(stripAnsi(line))
		padding := width - lineLen - 2
		if padding < 0 {
			padding = 0
		}
		fmt.Printf("%s│%s %s%s %s│%s\n",
			brandMuted, colorReset,
			line, strings.Repeat(" ", padding),
			brandMuted, colorReset)
	}

	// Bottom border
	fmt.Printf("%s%s%s%s%s\n", brandMuted, boxBL, strings.Repeat(boxH, width), boxBR, colorReset)
}

// Print a subtle divider line
func printSubtleDivider() {
	if output.JSONMode {
		return
	}
	fmt.Printf("%s%s%s\n", brandMuted, strings.Repeat("─", 50), colorReset)
}

func reloadLlamaServer() error {
	return reloadLlamaServerWithModel("")
}

func reloadLlamaServerWithModel(modelPath string) error {
	// Check if systemd service exists
	cmd := exec.Command("systemctl", "is-active", "llama-server")
	systemdAvailable := cmd.Run() == nil || exec.Command("systemctl", "status", "llama-server").Run() == nil

	if systemdAvailable {
		// Use systemd service
		// If modelPath is provided, update the active model configuration
		if modelPath != "" {
			// Store the active model path for the service to use
			activeModelFile := "/etc/offgrid/active-model"
			cmd := exec.Command("sudo", "sh", "-c", fmt.Sprintf("echo '%s' > %s", modelPath, activeModelFile))
			if err := cmd.Run(); err != nil {
				printWarning(fmt.Sprintf("Could not update active model config: %v", err))
			}
		}

		// Restart llama-server service
		fmt.Println()
		printInfo("Reloading inference server with new model...")

		cmd = exec.Command("sudo", "systemctl", "restart", "llama-server")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to restart llama-server: %v\nOutput: %s", err, string(output))
		}

		// Wait for service to be active
		time.Sleep(1 * time.Second)

		// Check if service is active
		cmd = exec.Command("systemctl", "is-active", "llama-server")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("llama-server failed to start - check logs with: sudo journalctl -u llama-server -n 50")
		}

		// Wait for model to actually load (can take several seconds for large models)
		if err := waitForLlamaServerReady(30); err != nil {
			printWarning(fmt.Sprintf("Model may still be loading: %v", err))
			printInfo("Large models can take 10-30 seconds to load")
		}

		printSuccess("Inference server reloaded")
		return nil
	}

	// No systemd service - start llama-server directly in background
	fmt.Println()
	printInfo("Starting llama-server in background...")

	// Check if llama-server is already running
	if isLlamaServerRunning() {
		// Server is running, just wait for it to be ready
		if err := waitForLlamaServerReady(5); err == nil {
			printSuccess("llama-server is already running")
			return nil
		}
	}

	// Start llama-server as background process
	// Read internal port (fallback to default if not found)
	llamaPort := "8081"
	portFile := "/etc/offgrid/llama-port"
	if data, err := os.ReadFile(portFile); err == nil {
		llamaPort = strings.TrimSpace(string(data))
	} else {
		// Try user config directory
		homeDir, _ := os.UserHomeDir()
		userPortFile := filepath.Join(homeDir, ".config", "offgrid", "llama-port")
		if data, err := os.ReadFile(userPortFile); err == nil {
			llamaPort = strings.TrimSpace(string(data))
		}
	}

	// Start llama-server with the model in background using shell with optimized flags
	// Detect optimal thread count using physical core detection
	res, err := resource.DetectResources()
	var threads int
	if err == nil {
		threads = res.GetOptimalThreads()
	} else {
		// Fallback: half of logical cores - 1
		cpuCores := runtime.NumCPU()
		threads = cpuCores / 2
		if threads < 1 {
			threads = 1
		}
		if threads > 1 {
			threads-- // Leave headroom for OS
		}
	}

	// Detect available RAM to adjust context size
	contextSize := 4096
	if err == nil && res.AvailableRAM > 0 {
		// Scale context based on RAM: each 1K context ≈ 0.5MB overhead per layer
		// For 4GB RAM, use 2048; for 8GB use 4096; for 16GB+ use 8192
		if res.AvailableRAM < 4000 {
			contextSize = 1024 // Very low RAM - minimal context
		} else if res.AvailableRAM < 6000 {
			contextSize = 2048 // 4-6GB RAM
		} else if res.AvailableRAM < 12000 {
			contextSize = 4096 // 6-12GB RAM
		} else {
			contextSize = 8192 // 12GB+ RAM
		}
	}

	// Choose batch size based on available RAM
	// Lower batch = faster time-to-first-token but slower throughput
	batchSize := 512
	if res != nil && res.AvailableRAM < 6000 {
		batchSize = 256 // Lower batch for constrained RAM
	}

	// Build optimized command
	// -t: Thread count for CPU inference
	// -c: Context window size
	// -b: Batch size (lower = faster first token)
	// -fa: Flash attention for 20-40% faster inference
	// --cont-batching: Better throughput for concurrent requests
	// --cache-type-k/v q8_0: Use q8 for KV cache (good balance of speed/quality)
	// --cache-prompt: Cache prompt prefixes for faster repeated prompts
	var cmdStr string
	if res != nil && res.AvailableRAM < 8000 {
		// Low RAM mode: use mmap (slower first token, but won't OOM)
		cmdStr = fmt.Sprintf("llama-server -m '%s' --port %s --host 127.0.0.1 -t %d -c %d -b %d -fa on --cont-batching --cache-type-k q8_0 --cache-type-v q8_0 --cache-reuse 256 > /dev/null 2>&1 &",
			modelPath, llamaPort, threads, contextSize, batchSize)
	} else {
		// High RAM mode: load model fully into RAM (faster inference)
		cmdStr = fmt.Sprintf("llama-server -m '%s' --port %s --host 127.0.0.1 -t %d -c %d -b %d --no-mmap --mlock -fa on --cont-batching --cache-type-k q8_0 --cache-type-v q8_0 --cache-reuse 256 > /dev/null 2>&1 &",
			modelPath, llamaPort, threads, contextSize, batchSize)
	}

	cmd = exec.Command("sh", "-c", cmdStr)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start llama-server: %v", err)
	}

	// Wait for server to be ready
	fmt.Println()
	printInfo("Waiting for llama-server to load model...")
	if err := waitForLlamaServerReady(30); err != nil {
		return fmt.Errorf("llama-server failed to start: %v", err)
	}

	printSuccess("llama-server started successfully")
	return nil
}

// isLlamaServerRunning checks if llama-server process is running
func isLlamaServerRunning() bool {
	cmd := exec.Command("pgrep", "-x", "llama-server")
	return cmd.Run() == nil
}

// waitForLlamaServerReady polls llama-server until it's ready or timeout
func waitForLlamaServerReady(timeoutSec int) error {
	// Read llama-server port (fallback to default if not found)
	port := "8081"
	portBytes, err := os.ReadFile("/etc/offgrid/llama-port")
	if err == nil {
		port = strings.TrimSpace(string(portBytes))
	} else {
		// Try user config directory
		homeDir, _ := os.UserHomeDir()
		userPortFile := filepath.Join(homeDir, ".config", "offgrid", "llama-port")
		if portBytes, err := os.ReadFile(userPortFile); err == nil {
			port = strings.TrimSpace(string(portBytes))
		}
	}

	// Create client that bypasses proxy for localhost
	client := &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			Proxy: func(req *http.Request) (*url.URL, error) {
				return nil, nil // Explicitly bypass all proxies for localhost
			},
		},
	}

	healthURL := fmt.Sprintf("http://127.0.0.1:%s/health", port)
	completionURL := fmt.Sprintf("http://127.0.0.1:%s/v1/chat/completions", port)

	startTime := time.Now()
	for time.Since(startTime) < time.Duration(timeoutSec)*time.Second {
		// First check health endpoint
		resp, err := client.Get(healthURL)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// Health OK, now check if model is loaded (try a minimal completion)
		testReq := map[string]interface{}{
			"model": "test",
			"messages": []map[string]string{
				{"role": "user", "content": "hi"},
			},
			"max_tokens": 1,
		}

		jsonData, _ := json.Marshal(testReq)
		resp, err = client.Post(completionURL, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// Check for "loading model" error
		if resp.StatusCode == 503 && strings.Contains(string(body), "Loading model") {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// Any other response means model is loaded (even errors about the request)
		return nil
	}

	return fmt.Errorf("timeout waiting for model to load")
}

func main() {
	// Check for global --json flag
	jsonFlag := false
	filteredArgs := make([]string, 0, len(os.Args))
	for i, arg := range os.Args {
		if arg == "--json" {
			jsonFlag = true
		} else {
			filteredArgs = append(filteredArgs, arg)
		}
		// Also check if it's at position 2 (after command)
		if i == 2 && arg == "--json" {
			jsonFlag = true
		}
	}

	// Set global JSON mode
	output.JSONMode = jsonFlag

	// Use filtered args if --json was found
	if jsonFlag {
		os.Args = filteredArgs
	}

	// Parse command
	if len(os.Args) > 1 {
		command := os.Args[1]

		switch command {
		case "auto-select", "autoselect", "recommend":
			handleAutoSelect(os.Args[2:])
			return
		case "doctor", "check", "diagnose":
			handleDoctor(os.Args[2:])
			return
		case "init", "setup":
			handleInit(os.Args[2:])
			return
		case "download":
			handleDownload(os.Args[2:])
			return
		case "download-hf":
			handleDownloadHF(os.Args[2:])
			return
		case "search":
			handleSearch(os.Args[2:])
			return
		case "run":
			handleRun(os.Args[2:])
			return
		case "import":
			handleImport(os.Args[2:])
			return
		case "remove", "delete", "rm":
			handleRemove(os.Args[2:])
			return
		case "export":
			handleExport(os.Args[2:])
			return
		case "chat":
			handleChat(os.Args[2:])
			return
		case "benchmark", "bench":
			handleBenchmark(os.Args[2:])
			return
		case "benchmark-compare", "compare":
			handleBenchmarkCompare(os.Args[2:])
			return
		case "test":
			handleTest(os.Args[2:])
			return
		case "list":
			handleList(os.Args[2:])
			return
		case "verify":
			handleVerify(os.Args[2:])
			return
		case "shell-completion":
			handleShellCompletion(os.Args[2:])
			return
		case "export-session":
			handleExportSession(os.Args[2:])
			return
		case "quantization", "quant":
			handleQuantization()
			return
		case "quantize":
			handleQuantize(os.Args[2:])
			return
		case "info", "status":
			handleInfo()
			return
		case "config":
			handleConfig(os.Args[2:])
			return
		case "alias":
			handleAlias(os.Args[2:])
			return
		case "favorite", "fav":
			handleFavorite(os.Args[2:])
			return
		case "template", "tpl":
			handleTemplate(os.Args[2:])
			return
		case "batch":
			handleBatch(os.Args[2:])
			return
		case "session", "sessions":
			handleSession(os.Args[2:])
			return
		case "kb", "knowledge", "rag":
			handleKnowledgeBase(os.Args[2:])
			return
		case "completions", "completion":
			handleCompletions(os.Args[2:])
			return
		case "users", "user":
			handleUsers(os.Args[2:])
			return
		case "metrics":
			handleMetrics(os.Args[2:])
			return
		case "lora":
			handleLoRA(os.Args[2:])
			return
		case "agent", "agents":
			handleAgent(os.Args[2:])
			return
		case "audio":
			handleAudio(os.Args[2:])
			return
		case "serve", "server":
			// Fall through to start server
		case "version", "-v", "--version":
			handleVersion()
			return
		case "help", "-h", "--help":
			printHelp()
			return
		default:
			fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
			printHelp()
			os.Exit(1)
		}
	}

	// Load configuration
	configPath := os.Getenv("OFFGRID_CONFIG")
	cfg, err := config.LoadWithPriority(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Start the HTTP server (default command)
	srv := server.NewWithConfig(cfg)
	if err := srv.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting server: %v\n", err)
		os.Exit(1)
	}
}

func handleDownload(args []string) {
	// Check for help flag
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		args = []string{} // Trigger help display
	}

	if len(args) < 1 {
		fmt.Println()
		fmt.Printf("  %s◈ Download Model%s\n", brandPrimary+colorBold, colorReset)
		fmt.Printf("  %sDownload models from catalog or HuggingFace%s\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sUsage%s\n", brandPrimary, colorReset)
		fmt.Printf("    offgrid download %s<model>%s [options]\n", brandPrimary, colorReset)
		fmt.Println()
		fmt.Printf("  %sOptions%s\n", brandPrimary, colorReset)
		fmt.Printf("    %-22s %sFilter by quantization (e.g., Q4_K_M)%s\n", "--quant <type>", colorDim, colorReset)
		fmt.Printf("    %-22s %sSpecific GGUF file to download%s\n", "--file <name>", colorDim, colorReset)
		fmt.Printf("    %-22s %sSkip confirmation prompts%s\n", "--yes, -y", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sCatalog Models%s %s(curated, verified)%s\n", brandPrimary, colorReset, colorDim, colorReset)
		fmt.Printf("    %s$%s offgrid download llama-3.1-8b-instruct\n", colorDim, colorReset)
		fmt.Printf("    %s$%s offgrid download phi-3.5-mini-instruct\n", colorDim, colorReset)
		fmt.Printf("    %s$%s offgrid download mistral-7b-instruct-v0.3\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sHuggingFace Models%s %s(owner/repo format)%s\n", brandPrimary, colorReset, colorDim, colorReset)
		fmt.Printf("    %s$%s offgrid download bartowski/Llama-3.2-3B-Instruct-GGUF\n", colorDim, colorReset)
		fmt.Printf("    %s$%s offgrid download MaziyarPanahi/Mistral-7B-Instruct-v0.3-GGUF --quant Q4_K_M\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sTip:%s Use %soffgrid search <query>%s to find models\n", brandMuted, colorReset, colorBold, colorReset)
		fmt.Println()
		os.Exit(1)
	}

	modelID := args[0]
	var quantFilter string
	var filename string
	var skipConfirm bool

	// Parse options
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--quant":
			if i+1 < len(args) {
				quantFilter = args[i+1]
				i++
			}
		case "--file":
			if i+1 < len(args) {
				filename = args[i+1]
				i++
			}
		case "--yes", "-y":
			skipConfirm = true
		default:
			// If no flag, treat as quantization for backward compatibility
			if !strings.HasPrefix(args[i], "-") && quantFilter == "" {
				quantFilter = args[i]
			}
		}
	}

	// Detect if this is a HuggingFace model (contains /)
	if strings.Contains(modelID, "/") {
		// HuggingFace download path
		handleHuggingFaceDownload(modelID, quantFilter, filename, skipConfirm)
		return
	}

	// Catalog download path
	cfg := config.LoadConfig()
	catalog := models.DefaultCatalog()
	downloader := models.NewDownloader(cfg.ModelsDir, catalog)

	// Find the model in catalog
	var modelEntry *models.CatalogEntry
	for i := range catalog.Models {
		if strings.EqualFold(catalog.Models[i].ID, modelID) {
			modelEntry = &catalog.Models[i]
			break
		}
	}

	if modelEntry == nil {
		fmt.Println()
		printError(fmt.Sprintf("Model '%s' not found in catalog", modelID))
		fmt.Println()
		fmt.Printf("  %sTry one of these:%s\n", brandMuted, colorReset)
		fmt.Printf("    • Use %soffgrid search %s%s to find HuggingFace models\n", colorBold, modelID, colorReset)
		fmt.Printf("    • Use %soffgrid list --catalog%s to see available catalog models\n", colorBold, colorReset)
		fmt.Println()
		os.Exit(1)
	}

	// Determine quantization
	quantization := quantFilter
	if quantization == "" {
		if len(modelEntry.Variants) > 0 {
			quantization = modelEntry.Variants[0].Quantization
		} else {
			quantization = "Q4_K_M"
		}
	}

	// Set progress callback
	downloader.SetProgressCallback(func(p models.DownloadProgress) {
		if p.Status == "complete" {
			fmt.Println()
			fmt.Println("  ✓ Download complete")
		} else if p.Status == "verifying" {
			fmt.Println()
			fmt.Println("  🔍 Verifying checksum...")
		} else {
			fmt.Printf("\r  ⏬ %.1f%% · %s / %.1f MB · %.1f MB/s          ",
				p.Percent,
				formatBytes(p.BytesDone), float64(p.BytesTotal)/(1024*1024),
				float64(p.Speed)/(1024*1024))
		}
		os.Stdout.Sync()
	})

	fmt.Println()
	fmt.Printf("  %s◈ Downloading Model%s\n", brandPrimary+colorBold, colorReset)
	fmt.Printf("  %s%s%s · %s%s%s\n", brandPrimary, modelEntry.Name, colorReset, brandMuted, quantization, colorReset)
	fmt.Println()

	if err := downloader.Download(modelEntry.ID, quantization); err != nil {
		fmt.Fprintf(os.Stderr, "\n  ✗ Download failed: %v\n", err)
		os.Exit(1)
	}

	modelPath := filepath.Join(cfg.ModelsDir, fmt.Sprintf("%s.%s.gguf", modelEntry.ID, quantization))

	if err := reloadLlamaServerWithModel(modelPath); err != nil {
		fmt.Println()
		printWarning(fmt.Sprintf("Could not auto-reload server: %v", err))
		fmt.Println()
		printInfo("Manually restart the server:")
		printItem("Restart service", "sudo systemctl restart llama-server")
		fmt.Println()
	}
}

// handleHuggingFaceDownload handles downloading from HuggingFace Hub
func handleHuggingFaceDownload(modelID, quantFilter, filename string, skipConfirm bool) {
	cfg := config.LoadConfig()
	hf := models.NewHuggingFaceClient()

	fmt.Println()
	fmt.Printf("  %s◈ Download from HuggingFace%s\n", brandPrimary+colorBold, colorReset)
	fmt.Printf("  %sFetching model info: %s%s%s\n", colorDim, colorBold, modelID, colorReset)
	fmt.Println()

	// Get model files with sizes using tree API
	files, err := hf.GetModelFiles(modelID)
	if err != nil {
		printHelpfulError(err, "Model fetch")
		os.Exit(1)
	}

	// Parse GGUF files
	ggufFiles := []models.GGUFFileInfo{}
	appendFiltered := func(candidate models.GGUFFileInfo) {
		if filename != "" && candidate.Filename != filename {
			return
		}
		if quantFilter != "" && !strings.EqualFold(candidate.Quantization, quantFilter) {
			return
		}
		ggufFiles = append(ggufFiles, candidate)
	}

	for _, file := range files {
		if !strings.HasSuffix(strings.ToLower(file.Filename), ".gguf") {
			continue
		}
		info := models.GGUFFileInfo{
			Filename:     file.Filename,
			Size:         file.Size,
			SizeGB:       float64(file.Size) / (1024 * 1024 * 1024),
			Quantization: extractQuantFromFilename(file.Filename),
			DownloadURL:  fmt.Sprintf("https://huggingface.co/%s/resolve/main/%s", modelID, file.Filename),
		}
		appendFiltered(info)
	}

	// Fallback to ListGGUFFiles if no files found
	if len(ggufFiles) == 0 {
		if treeFiles, err := hf.ListGGUFFiles(modelID); err == nil {
			for _, info := range treeFiles {
				appendFiltered(info)
			}
		}
	}

	if len(ggufFiles) == 0 {
		printError("No matching GGUF files found")
		fmt.Println()
		if quantFilter != "" {
			fmt.Printf("  No files with quantization '%s%s%s' found.\n", brandPrimary, quantFilter, colorReset)
			fmt.Println()
			printInfo("Try without --quant filter or use 'offgrid search' to see available quantizations")
		} else {
			printInfo("This model may not have GGUF format files.")
			fmt.Println()
			printInfo("Search for GGUF models:")
			fmt.Printf("    %s$%s offgrid search <query>\n", colorDim, colorReset)
		}
		fmt.Println()
		os.Exit(1)
	}

	// Select file
	var selectedFile models.GGUFFileInfo
	if len(ggufFiles) == 1 {
		selectedFile = ggufFiles[0]
	} else if skipConfirm {
		selectedFile = ggufFiles[0]
		fmt.Printf("  %sAuto-selecting:%s %s (%s%s%s)\n", brandMuted, colorReset,
			selectedFile.Filename, brandPrimary, selectedFile.Quantization, colorReset)
		fmt.Println()
	} else {
		fmt.Printf("  %sAvailable Files%s\n", brandPrimary, colorReset)
		for i, file := range ggufFiles {
			sizeStr := formatSizeGB(file.SizeGB)
			fmt.Printf("    %s%d.%s %s %s(%s · %s)%s\n",
				brandMuted, i+1, colorReset,
				file.Filename,
				colorDim, file.Quantization, sizeStr, colorReset)
		}
		fmt.Println()
		fmt.Printf("  %sSelect file%s (1-%d): ", brandMuted, colorReset, len(ggufFiles))

		var choice int
		fmt.Scanf("%d", &choice)
		if choice < 1 || choice > len(ggufFiles) {
			fmt.Println()
			printError("Invalid choice")
			fmt.Println()
			os.Exit(1)
		}
		selectedFile = ggufFiles[choice-1]
	}

	// Check for vision adapter
	var projectorSource *models.ProjectorSource
	if source, err := hf.ResolveProjectorSource(modelID, files, selectedFile.Filename); err == nil && source != nil {
		projectorSource = source
		fmt.Printf("  %sVision Support%s\n", brandPrimary, colorReset)
		fmt.Printf("    Adapter: %s%s%s", brandPrimary, projectorSource.File.Filename, colorReset)
		if projectorSource.File.Size > 0 {
			fmt.Printf(" · %s", formatBytes(projectorSource.File.Size))
		}
		fmt.Println()
		fmt.Println()
	}

	// Download
	destPath := filepath.Join(cfg.ModelsDir, selectedFile.Filename)
	fmt.Printf("  %sDownloading%s %s\n", brandMuted, colorReset, selectedFile.Filename)
	fmt.Printf("    %sSize:%s %.2f GB\n", colorDim, colorReset, selectedFile.SizeGB)
	fmt.Printf("    %sDestination:%s %s\n", colorDim, colorReset, destPath)
	fmt.Println()

	var lastProgress int64
	lastUpdate := time.Now()
	if err := hf.DownloadGGUF(modelID, selectedFile.Filename, destPath, func(current, total int64) {
		now := time.Now()
		elapsed := now.Sub(lastUpdate)
		if elapsed < 500*time.Millisecond && current < total {
			return
		}
		percent := float64(current) / float64(total) * 100
		var speed float64
		if elapsed.Seconds() > 0 {
			speed = float64(current-lastProgress) / elapsed.Seconds()
		}
		fmt.Printf("\r  ⏬ %.1f%% · %s / %.2f GB · %.1f MB/s          ",
			percent,
			formatBytes(current), selectedFile.SizeGB,
			speed/(1024*1024))
		os.Stdout.Sync()
		lastProgress = current
		lastUpdate = now
	}); err != nil {
		fmt.Fprintf(os.Stderr, "\n  ✗ Download failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println() // New line after progress
	fmt.Println("  ✓ Download complete")
	fmt.Println()

	// Download projector if available
	if projectorSource != nil {
		projPath := filepath.Join(cfg.ModelsDir, projectorSource.File.Filename)
		fmt.Printf("  %sDownloading vision adapter...%s\n", brandMuted, colorReset)
		// Use projectorSource.ModelID - this may be a fallback repo like koboldcpp/mmproj
		if err := hf.DownloadGGUF(projectorSource.ModelID, projectorSource.File.Filename, projPath, nil); err != nil {
			printWarning(fmt.Sprintf("Could not download vision adapter: %v", err))
			fmt.Printf("  %sTry manually:%s offgrid download %s --file %s\n",
				brandMuted, colorReset, projectorSource.ModelID, projectorSource.File.Filename)
		} else {
			fmt.Println("  ✓ Vision adapter downloaded")
		}
		fmt.Println()
	}

	// Reload server
	if err := reloadLlamaServerWithModel(destPath); err != nil {
		printWarning(fmt.Sprintf("Could not auto-reload server: %v", err))
		fmt.Println()
		printInfo("Manually restart the server:")
		printItem("Restart service", "sudo systemctl restart llama-server")
		fmt.Println()
	}
}

func handleImport(args []string) {
	// Check for help flag
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		args = []string{} // Trigger help display
	}

	if len(args) < 1 {
		fmt.Println()
		fmt.Printf("  %s◈ Import Model%s\n", brandPrimary+colorBold, colorReset)
		fmt.Printf("  %sImport GGUF models from USB/SD card or external storage%s\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sUsage%s  offgrid import <path> [model-file]\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sExamples%s\n", brandPrimary, colorReset)
		fmt.Printf("    %s$%s offgrid import /media/usb              %s# Import all .gguf files%s\n", colorDim, colorReset, brandMuted, colorReset)
		fmt.Printf("    %s$%s offgrid import /media/usb/model.gguf  %s# Import specific file%s\n", colorDim, colorReset, brandMuted, colorReset)
		fmt.Printf("    %s$%s offgrid import D:\\models              %s# Windows path%s\n", colorDim, colorReset, brandMuted, colorReset)
		fmt.Println()
		fmt.Printf("  %sTip:%s Use %soffgrid list%s to view imported models\n", brandMuted, colorReset, colorBold, colorReset)
		fmt.Println()
		os.Exit(1)
	}

	cfg := config.LoadConfig()
	registry := models.NewRegistry(cfg.ModelsDir)
	importer := models.NewUSBImporter(cfg.ModelsDir, registry)

	usbPath := args[0]

	// Check if path is a specific file or directory
	info, err := os.Stat(usbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ Path not found: %s\n\n", usbPath)
		fmt.Fprintf(os.Stderr, "Common USB/SD mount points:\n")
		fmt.Fprintf(os.Stderr, "  • Linux:   /media/<username>/<device>\n")
		fmt.Fprintf(os.Stderr, "  • macOS:   /Volumes/<device>\n")
		fmt.Fprintf(os.Stderr, "  • Windows: D:\\ E:\\ F:\\\n")
		fmt.Fprintf(os.Stderr, "\nTip: Use 'ls /media' or 'mount' to find your device\n\n")
		os.Exit(1)
	}

	if info.IsDir() {
		// Import all models from directory
		fmt.Printf("Scanning %s\n\n", usbPath)

		modelFiles, err := importer.ScanUSBDrive(usbPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ✗ Scan error: %v\n", err)
			os.Exit(1)
		}

		if len(modelFiles) == 0 {
			fmt.Println("✗ No GGUF model files found in", usbPath)
			fmt.Println()
			fmt.Println("Looking for files matching: *.gguf")
			fmt.Println()
			fmt.Println("Make sure your models:")
			fmt.Println("  • Have .gguf file extension")
			fmt.Println("  • Are in GGUF format (not safetensors or PyTorch)")
			fmt.Println("  • Are readable (check permissions)")
			fmt.Println()
			os.Exit(0)
		}

		fmt.Printf("Found %d model file(s):\n\n", len(modelFiles))
		for i, file := range modelFiles {
			modelID, quant := importer.GetModelInfo(filepath.Base(file))
			size := getFileSize(file)
			fmt.Printf("  %d. %s (%s) · %s\n", i+1, modelID, quant, formatBytes(size))
		}
		fmt.Println()

		// Import all
		fmt.Println("Importing models...")
		fmt.Println()
		imported, err := importer.ImportAll(usbPath, func(p models.ImportProgress) {
			if p.Status == "copying" {
				fmt.Printf("\r  %s: %.1f%% · %s",
					p.FileName, p.Percent, formatBytes(p.BytesDone))
			} else if p.Status == "verifying" {
				fmt.Printf("\r  Verifying %s...          ", p.FileName)
			} else if p.Status == "complete" {
				fmt.Printf("\r  ✓ %s\n", p.FileName)
			}
		})

		if err != nil {
			fmt.Fprintf(os.Stderr, "\n  ✗ Import failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\n  ✓ Imported %d model(s) to %s\n", imported, cfg.ModelsDir)

		// Reload llama-server with imported models
		if imported > 0 {
			if err := reloadLlamaServer(); err != nil {
				fmt.Println()
				printWarning(fmt.Sprintf("Could not auto-reload server: %v", err))
				fmt.Println()
				printInfo("Manually restart the server:")
				printItem("Restart service", "sudo systemctl restart llama-server")
				fmt.Println()
			}
		}
	} else {
		// Import single file
		fmt.Printf("Importing %s\n\n", filepath.Base(usbPath))

		err := importer.ImportModel(usbPath, "", func(p models.ImportProgress) {
			if p.Status == "copying" {
				fmt.Printf("\r  Progress: %.1f%% · %s / %s",
					p.Percent, formatBytes(p.BytesDone), formatBytes(p.BytesTotal))
			} else if p.Status == "verifying" {
				fmt.Print("\r  Verifying integrity...          ")
			} else if p.Status == "complete" {
				fmt.Print("\r  ✓ Import complete                \n")
			}
		})

		if err != nil {
			fmt.Fprintf(os.Stderr, "\n  ✗ Import failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\n  ✓ Model imported to %s\n", cfg.ModelsDir)

		// Reload llama-server with the imported model
		// Construct the destination path where the model was imported
		importedModelPath := filepath.Join(cfg.ModelsDir, filepath.Base(usbPath))
		if err := reloadLlamaServerWithModel(importedModelPath); err != nil {
			fmt.Println()
			printWarning(fmt.Sprintf("Could not auto-reload server: %v", err))
			fmt.Println()
			printInfo("Manually restart the server:")
			printItem("Restart service", "sudo systemctl restart llama-server")
			fmt.Println()
		}
	}
}

func getFileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func handleRemove(args []string) {
	// Check for help flag first
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		args = []string{} // Trigger help display
	}

	if len(args) < 1 {
		fmt.Println()
		fmt.Printf("  %s◈ Remove Model%s\n", brandPrimary+colorBold, colorReset)
		fmt.Printf("  %sRemove an installed model from your system%s\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sUsage%s  offgrid remove <model-id> [options]\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sOptions%s\n", brandPrimary, colorReset)
		fmt.Printf("    %-20s %sSkip confirmation prompt%s\n", "--yes, -y", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sExamples%s\n", brandPrimary, colorReset)
		fmt.Printf("    %s$%s offgrid remove tinyllama-1.1b-chat.Q4_K_M\n", colorDim, colorReset)
		fmt.Printf("    %s$%s offgrid remove llama-2-7b-chat.Q5_K_M --yes\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sTip:%s Use %soffgrid list%s to see installed models\n", brandMuted, colorReset, colorBold, colorReset)
		fmt.Println()
		os.Exit(1)
	}

	cfg := config.LoadConfig()
	registry := models.NewRegistry(cfg.ModelsDir)

	if err := registry.ScanModels(); err != nil {
		fmt.Fprintf(os.Stderr, "  ✗ Error scanning models: %v\n", err)
		os.Exit(1)
	}

	modelID := args[0]
	var skipConfirm bool

	// Parse options
	for i := 1; i < len(args); i++ {
		if args[i] == "--yes" || args[i] == "-y" {
			skipConfirm = true
		}
	}

	// Check if model exists
	meta, err := registry.GetModel(modelID)
	if err != nil {
		fmt.Println()
		fmt.Printf("%s✗ Model not found: %s%s\n", brandError, modelID, colorReset)
		fmt.Println()

		// Show available models
		modelList := registry.ListModels()
		if len(modelList) > 0 {
			fmt.Printf("%sAvailable Models%s\n", brandPrimary+colorBold, colorReset)
			for _, m := range modelList {
				modelMeta, _ := registry.GetModel(m.ID)
				fmt.Printf("%s", m.ID)
				if modelMeta != nil && modelMeta.Size > 0 {
					fmt.Printf(" · %s", formatBytes(modelMeta.Size))
				}
				if modelMeta != nil && modelMeta.Quantization != "" {
					fmt.Printf(" · %s", modelMeta.Quantization)
				}
				fmt.Println()
			}
			fmt.Println()
		} else {
			fmt.Printf("%sℹ No models installed%s\n", colorDim, colorReset)
			fmt.Println()
			fmt.Println("Download models:")
			fmt.Printf("  %sFrom HuggingFace:%s offgrid download-hf <repo> --file <file>.gguf\n", brandSecondary, colorReset)
			fmt.Println()
		}
		os.Exit(1)
	}

	// Confirm deletion
	fmt.Println()
	fmt.Printf("%sRemove Model%s\n", brandPrimary+colorBold, colorReset)
	fmt.Println("")

	fmt.Printf("%sModel Information%s\n", brandPrimary, colorReset)
	fmt.Printf("Name: %s%s%s\n", colorBold, modelID, colorReset)
	if meta.Path != "" {
		fmt.Printf("Path: %s%s%s\n", brandMuted, meta.Path, colorReset)
	}
	if meta.Size > 0 {
		fmt.Printf("Size: %s%s%s will be freed\n", brandSuccess, formatBytes(meta.Size), colorReset)
	}
	fmt.Println("")

	if !skipConfirm {
		fmt.Printf("%s⚠  This action cannot be undone%s\n", brandError, colorReset)
		fmt.Println("")
		fmt.Printf("%sContinue?%s (y/N): ", brandMuted, colorReset)

		var response string
		fmt.Scanln(&response)

		if response != "y" && response != "Y" {
			fmt.Println()
			printInfo("Cancelled - model preserved")
			fmt.Println()
			return
		}
	} else {
		fmt.Printf("%sRemoving model (--yes flag)...%s\n", brandMuted, colorReset)
		fmt.Println("")
	}

	// Delete the model file
	if meta.Path != "" {
		if err := os.Remove(meta.Path); err != nil {
			fmt.Println()
			printError(fmt.Sprintf("Failed to remove file: %v", err))
			fmt.Println()
			os.Exit(1)
		}
	}

	fmt.Println()
	fmt.Printf("%s✓ Model removed: %s%s%s\n", brandSuccess, brandPrimary, modelID, colorReset)

	// Rescan to update registry after file deletion
	if err := registry.ScanModels(); err != nil {
		fmt.Println()
		printWarning(fmt.Sprintf("Failed to refresh model list: %v", err))
	}

	// Show remaining models
	remaining := registry.ListModels()
	fmt.Printf("%s%d model(s) remaining%s\n\n", brandMuted, len(remaining), colorReset)
}

func handleExport(args []string) {
	// Check for help flag
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		args = []string{} // Trigger help display
	}

	if len(args) < 2 {
		fmt.Println()
		fmt.Printf("  %s◈ Export Model%s\n", brandPrimary+colorBold, colorReset)
		fmt.Printf("  %sExport a model to USB/SD card or external storage%s\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sUsage%s  offgrid export <model-id> <destination>\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sExamples%s\n", brandPrimary, colorReset)
		fmt.Printf("    %s$%s offgrid export tinyllama-1.1b-chat.Q4_K_M /media/usb\n", colorDim, colorReset)
		fmt.Printf("    %s$%s offgrid export llama-2-7b-chat.Q5_K_M D:\\backup\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sTip:%s Use %soffgrid list%s to see available models\n", brandMuted, colorReset, colorBold, colorReset)
		fmt.Println()
		os.Exit(1)
	}

	cfg := config.LoadConfig()
	registry := models.NewRegistry(cfg.ModelsDir)

	if err := registry.ScanModels(); err != nil {
		fmt.Fprintf(os.Stderr, "  ✗ Error scanning models: %v\n", err)
		os.Exit(1)
	}

	modelID := args[0]
	destPath := args[1]

	// Check if model exists
	meta, err := registry.GetModel(modelID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ Model not found: %s\n\n", modelID)

		// Show available models
		modelList := registry.ListModels()
		if len(modelList) > 0 {
			fmt.Fprintln(os.Stderr, "Available models:")
			for _, m := range modelList {
				fmt.Fprintf(os.Stderr, "  • %s\n", m.ID)
			}
			fmt.Fprintln(os.Stderr, "")
		} else {
			fmt.Fprintln(os.Stderr, "No models installed. Use 'offgrid download' to add models.")
			fmt.Fprintln(os.Stderr, "")
		}
		os.Exit(1)
	}

	if meta.Path == "" {
		fmt.Fprintf(os.Stderr, "✗ Model path not found for: %s\n\n", modelID)
		os.Exit(1)
	}

	// Ensure destination directory exists
	if err := os.MkdirAll(destPath, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "✗ Cannot create destination directory: %v\n\n", err)
		fmt.Fprintf(os.Stderr, "Make sure:\n")
		fmt.Fprintf(os.Stderr, "  • The USB/SD card is mounted\n")
		fmt.Fprintf(os.Stderr, "  • You have write permissions\n")
		fmt.Fprintf(os.Stderr, "  • The device has enough space\n\n")
		os.Exit(1)
	}

	// Construct destination file path
	fileName := filepath.Base(meta.Path)
	destFile := filepath.Join(destPath, fileName)

	fmt.Printf("%sExport Model%s\n", brandPrimary+colorBold, colorReset)
	fmt.Println("")
	fmt.Printf("Model: %s%s%s\n", brandPrimary, modelID, colorReset)
	fmt.Printf("From:  %s%s%s\n", brandMuted, meta.Path, colorReset)
	fmt.Printf("To:    %s%s%s\n", brandMuted, destFile, colorReset)
	fmt.Printf("Size:  %s\n", formatBytes(meta.Size))
	fmt.Println("")

	// Copy file
	sourceFile, err := os.Open(meta.Path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  ✗ Failed to open source: %v\n", err)
		os.Exit(1)
	}
	defer sourceFile.Close()

	destFileHandle, err := os.Create(destFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  ✗ Failed to create destination: %v\n", err)
		os.Exit(1)
	}
	defer destFileHandle.Close()

	// Copy with progress
	buffer := make([]byte, 1024*1024) // 1MB buffer
	var totalCopied int64

	for {
		n, err := sourceFile.Read(buffer)
		if n > 0 {
			if _, err := destFileHandle.Write(buffer[:n]); err != nil {
				fmt.Fprintf(os.Stderr, "\n  ✗ Write error: %v\n", err)
				os.Exit(1)
			}
			totalCopied += int64(n)
			percent := float64(totalCopied) / float64(meta.Size) * 100
			fmt.Printf("\r  Progress: %.1f%% · %s / %s",
				percent, formatBytes(totalCopied), formatBytes(meta.Size))
		}
		if err != nil {
			break
		}
	}

	fmt.Printf("\n\n✓ Export complete\n")
	fmt.Printf("  Location: %s\n\n", destFile)
}

func handleChat(args []string) {
	fmt.Println("Interactive Chat Mode")
	fmt.Println()
	fmt.Println("This feature requires a running server with loaded models.")
	fmt.Println()
	fmt.Println("Quick start:")
	fmt.Println("  1. Start server:  offgrid serve")
	fmt.Println("  2. In new terminal, use the API:")
	fmt.Println()
	fmt.Println("Example curl command:")
	fmt.Println(`  curl http://localhost:11611/v1/chat/completions \`)
	fmt.Println(`    -H "Content-Type: application/json" \`)
	fmt.Println(`    -d '{"model":"auto","messages":[{"role":"user","content":"Hello!"}]}'`)
	fmt.Println()
	fmt.Println("Or use the web UI at: http://localhost:11611/ui")
	fmt.Println()

	// TODO: Implement interactive CLI chat
	// This would require:
	// 1. Connect to running server via HTTP client
	// 2. Read user input in loop
	// 3. Send requests and stream responses
	// 4. Handle conversation history
}

func handleDoctor(args []string) {
	fmt.Println()
	fmt.Printf("  %s%s System Diagnostics%s\n", brandPrimary+colorBold, iconSearch, colorReset)
	fmt.Println()

	allPassed := true
	checks := 0
	passed := 0

	// Helper for check output
	printCheck := func(success bool, optional bool, name, detail string) {
		checks++
		icon := iconCheck
		color := brandSuccess
		if !success {
			if optional {
				icon = "○"
				color = brandMuted
			} else {
				icon = iconCross
				color = brandError
				allPassed = false
			}
		} else {
			passed++
		}
		fmt.Printf("    %s%s%s  %-12s %s%s%s\n", color, icon, colorReset, name, brandMuted, detail, colorReset)
	}

	// 1. Check system resources
	res, err := resource.DetectResources()
	if err != nil {
		printCheck(false, false, "Resources", fmt.Sprintf("Failed (%v)", err))
	} else {
		detail := fmt.Sprintf("%s/%s · %d cores · %s RAM", res.OS, res.Arch, res.CPUCores, formatBytes(res.TotalRAM*1024*1024))
		printCheck(true, false, "Resources", detail)
		if res.GPUAvailable {
			fmt.Printf("              %s└─ GPU: %s (%s)%s\n", brandMuted, res.GPUName, formatBytes(res.GPUMemory*1024*1024), colorReset)
		}
	}

	// 2. Check configuration
	cfg := config.LoadConfig()
	configPath := filepath.Join(os.Getenv("HOME"), ".offgrid-llm", "config.json")
	if _, err := os.Stat(configPath); err == nil {
		printCheck(true, false, "Config", configPath)
	} else {
		printCheck(true, true, "Config", "Using defaults")
	}

	// 3. Check models directory
	if _, err := os.Stat(cfg.ModelsDir); err == nil {
		// Check permissions
		testFile := filepath.Join(cfg.ModelsDir, ".test_write")
		if err := os.WriteFile(testFile, []byte("test"), 0644); err == nil {
			os.Remove(testFile)
			printCheck(true, false, "Storage", fmt.Sprintf("%s (writable)", cfg.ModelsDir))
		} else {
			printCheck(false, false, "Storage", fmt.Sprintf("%s (read-only)", cfg.ModelsDir))
		}

		// Count models
		registry := models.NewRegistry(cfg.ModelsDir)
		if err := registry.ScanModels(); err == nil {
			modelList := registry.ListModels()
			fmt.Printf("              %s└─ %d model(s) installed%s\n", brandMuted, len(modelList), colorReset)
		}
	} else {
		printCheck(false, false, "Storage", fmt.Sprintf("Missing %s", cfg.ModelsDir))
	}

	// 4. Check network connectivity
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://huggingface.co")
	if err == nil && resp.StatusCode == 200 {
		printCheck(true, false, "Network", "Online (HuggingFace reachable)")
		resp.Body.Close()
	} else {
		printCheck(true, true, "Network", "Offline mode")
	}

	// 5. Check server status
	healthURL := fmt.Sprintf("http://localhost:%d/health", cfg.ServerPort)
	resp, err = client.Get(healthURL)
	if err == nil && resp.StatusCode == 200 {
		printCheck(true, false, "Server", fmt.Sprintf("Running on port %d", cfg.ServerPort))
		resp.Body.Close()
	} else {
		printCheck(true, true, "Server", "Not running")
	}

	fmt.Println()
	if allPassed {
		fmt.Printf("    %s%s All checks passed%s (%d/%d)\n", brandSuccess, iconCheck, colorReset, passed, checks)
	} else {
		fmt.Printf("    %s%s Some checks failed%s (%d/%d passed)\n", brandError, iconCross, colorReset, passed, checks)
	}
	fmt.Println()
}

func handleInit(args []string) {
	fmt.Println()

	// Check if already initialized
	cfg := config.LoadConfig()
	registry := models.NewRegistry(cfg.ModelsDir)
	registry.ScanModels()
	installedModels := registry.ListModels()

	if len(installedModels) > 0 {
		fmt.Printf("  %s%s Already initialized%s — %d model(s) installed\n\n", brandSuccess, iconCheck, colorReset, len(installedModels))
		fmt.Printf("  %sNext steps:%s\n", colorBold, colorReset)
		fmt.Printf("    %s$%s offgrid list          %s# Show installed models%s\n", brandMuted, colorReset, brandMuted, colorReset)
		fmt.Printf("    %s$%s offgrid run <model>   %s# Start chatting%s\n", brandMuted, colorReset, brandMuted, colorReset)
		fmt.Printf("    %s$%s offgrid doctor        %s# Check system health%s\n", brandMuted, colorReset, brandMuted, colorReset)
		fmt.Println()
		return
	}

	// Step 1: Detect system
	fmt.Printf("  %s[1/4]%s %sDetecting system...%s\n", brandPrimary, colorReset, colorBold, colorReset)
	res, err := resource.DetectResources()
	if err != nil {
		printError(fmt.Sprintf("Failed to detect system: %v", err))
		os.Exit(1)
	}

	fmt.Printf("        %sOS%s       %s/%s\n", brandMuted, colorReset, res.OS, res.Arch)
	fmt.Printf("        %sCPU%s      %d cores\n", brandMuted, colorReset, res.CPUCores)
	fmt.Printf("        %sRAM%s      %s\n", brandMuted, colorReset, formatBytes(res.AvailableRAM*1024*1024))
	if res.GPUAvailable {
		fmt.Printf("        %sGPU%s      %s\n", brandMuted, colorReset, res.GPUName)
	} else {
		fmt.Printf("        %sGPU%s      None (CPU mode)\n", brandMuted, colorReset)
	}
	fmt.Println()

	// Step 2: Get recommendations
	fmt.Printf("  %s[2/4]%s %sFinding compatible models...%s\n", brandPrimary, colorReset, colorBold, colorReset)
	recommendations := res.RecommendedModels()

	primary := []resource.ModelRecommendation{}
	for _, rec := range recommendations {
		if rec.Priority == 1 {
			primary = append(primary, rec)
		}
	}

	if len(primary) == 0 {
		fmt.Println()
		printWarning("Your system has limited resources")
		fmt.Printf("\n  %sMinimum requirements:%s\n", colorBold, colorReset)
		fmt.Printf("    %s•%s 2GB RAM available\n", brandMuted, colorReset)
		fmt.Printf("    %s•%s 2GB free disk space\n", brandMuted, colorReset)
		fmt.Println()
		os.Exit(1)
	}

	fmt.Println()
	fmt.Printf("        %sRecommended for your system:%s\n", colorBold, colorReset)
	maxModels := 3
	if len(primary) < maxModels {
		maxModels = len(primary)
	}
	for i := 0; i < maxModels; i++ {
		rec := primary[i]
		fmt.Printf("        %s%d.%s %s%s%s %s(%s · %.1fGB)%s\n",
			brandPrimary, i+1, colorReset,
			colorBold, rec.ModelID, colorReset,
			brandMuted, rec.Quantization, rec.SizeGB, colorReset)
		fmt.Printf("           %s%s%s\n", brandMuted, rec.Reason, colorReset)
	}
	fmt.Println()

	// Step 3: Choose model
	fmt.Printf("  %s[3/4]%s %sSelect a model%s\n", brandPrimary, colorReset, colorBold, colorReset)
	fmt.Printf("        Enter %s1-%d%s or %ss%s to skip: ", brandPrimary, maxModels, colorReset, brandMuted, colorReset)

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "s" || input == "S" || input == "" {
		fmt.Println()
		fmt.Printf("  %sSkipped.%s Search models anytime:\n", brandMuted, colorReset)
		fmt.Printf("    %s$%s offgrid search llama\n", brandMuted, colorReset)
		fmt.Printf("    %s$%s offgrid recommend\n", brandMuted, colorReset)
		fmt.Println()
		return
	}

	var choice int
	fmt.Sscanf(input, "%d", &choice)
	if choice < 1 || choice > len(primary) {
		printError("Invalid choice")
		os.Exit(1)
	}

	selected := primary[choice-1]
	fmt.Println()

	// Step 4: Download
	fmt.Printf("  %s[4/4]%s %sDownloading %s...%s\n\n", brandPrimary, colorReset, colorBold, selected.ModelID, colorReset)

	// Search for the model on HuggingFace
	hf := models.NewHuggingFaceClient()
	filters := models.SearchFilter{
		Query:        selected.ModelID,
		Quantization: selected.Quantization,
		OnlyGGUF:     true,
		Limit:        1,
	}

	results, err := hf.SearchModels(filters)
	if err != nil || len(results) == 0 {
		printError("Failed to find model on HuggingFace")
		fmt.Println()
		fmt.Println("Search manually:")
		fmt.Printf("  offgrid search %s\n", selected.ModelID)
		fmt.Println()
		os.Exit(1)
	}

	result := results[0]
	if result.BestVariant == nil {
		printError("No suitable variant found")
		os.Exit(1)
	}

	var wizardProjector *models.ProjectorSource
	if projector, err := hf.ResolveProjectorSource(result.Model.ID, nil, result.BestVariant.Filename); err != nil {
		fmt.Println()
		printWarning(fmt.Sprintf("Vision adapter lookup failed: %v", err))
		fmt.Println()
	} else {
		wizardProjector = projector
	}
	if wizardProjector != nil {
		sourceLabel := wizardProjector.ModelID
		if wizardProjector.Source == "fallback" {
			sourceLabel = fmt.Sprintf("%s (fallback)", sourceLabel)
		}
		fmt.Printf("Vision adapter detected: %s%s%s", brandPrimary, wizardProjector.File.Filename, colorReset)
		if wizardProjector.File.Size > 0 {
			fmt.Printf(" · %s", formatBytes(wizardProjector.File.Size))
		}
		fmt.Println()
		fmt.Printf("Source: %s%s%s\n", brandMuted, sourceLabel, colorReset)
		if wizardProjector.Reason != "" {
			fmt.Printf("  %s%s%s\n", brandMuted, wizardProjector.Reason, colorReset)
		}
		fmt.Printf("%sIt will download automatically after the main model.%s\n\n", brandMuted, colorReset)
	}

	// Download the model
	destPath := filepath.Join(cfg.ModelsDir, result.BestVariant.Filename)

	fmt.Printf("Downloading from: %s\n", result.Model.ID)
	fmt.Printf("File: %s (%.1fGB)\n\n", result.BestVariant.Filename, result.BestVariant.SizeGB)

	err = hf.DownloadGGUF(result.Model.ID, result.BestVariant.Filename, destPath, func(current, total int64) {
		percent := float64(current) / float64(total) * 100
		fmt.Printf("\r  Progress: %.1f%% (%s / %s) · %.1f MB/s",
			percent,
			formatBytes(current),
			formatBytes(total),
			float64(current)/1024/1024)
	})

	if err != nil {
		fmt.Println()
		printHelpfulError(err, "Download")
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println()
	fmt.Printf("%s✓ Setup complete!%s\n\n", brandSuccess+colorBold, colorReset)

	modelName := strings.TrimSuffix(result.BestVariant.Filename, ".gguf")
	fmt.Println("Next steps:")
	fmt.Printf("  offgrid run %s         # Start chatting\n", modelName)
	fmt.Printf("  offgrid list                 # View installed models\n")
	fmt.Printf("  offgrid search llama         # Find more models\n")
	fmt.Println()

	if wizardProjector != nil {
		fmt.Printf("%sVision Adapter%s\n", brandPrimary+colorBold, colorReset)
		if err := downloadVisionProjector(hf, cfg.ModelsDir, wizardProjector); err != nil {
			printError(fmt.Sprintf("Vision adapter download failed: %v", err))
			os.Exit(1)
		}
		fmt.Println()
	}
}

func handleAutoSelect(args []string) {
	fmt.Println()
	fmt.Printf("  %s◈ Model Recommendations%s\n", brandPrimary+colorBold, colorReset)
	fmt.Printf("  %sAnalyzing your system for optimal model selection%s\n", colorDim, colorReset)
	fmt.Println()

	// Detect hardware resources
	res, err := resource.DetectResources()
	if err != nil {
		printError(fmt.Sprintf("Failed to detect system resources: %v", err))
		os.Exit(1)
	}

	// Display system info in a compact format
	fmt.Printf("  %sSystem%s\n", brandPrimary, colorReset)
	fmt.Printf("    OS        %s%s/%s%s\n", colorDim, res.OS, res.Arch, colorReset)
	fmt.Printf("    CPU       %s%d cores%s\n", colorDim, res.CPUCores, colorReset)
	fmt.Printf("    RAM       %s%s total · %s available%s\n", colorDim,
		formatBytes(res.TotalRAM*1024*1024),
		formatBytes(res.AvailableRAM*1024*1024), colorReset)

	if res.GPUAvailable {
		fmt.Printf("    GPU       %s%s · %s VRAM%s\n", colorDim, res.GPUName, formatBytes(res.GPUMemory*1024*1024), colorReset)
	} else {
		fmt.Printf("    GPU       %sNot detected (CPU-only)%s\n", brandMuted, colorReset)
	}
	fmt.Println()

	// Get recommendations
	recommendations := res.RecommendedModels()

	if len(recommendations) == 0 {
		fmt.Println()
		printWarning("Insufficient memory for any models")
		fmt.Println()
		printInfo("Minimum requirements:")
		printItem("RAM", "2 GB available")
		fmt.Println()
		os.Exit(1)
	}

	// Group by priority
	primary := []resource.ModelRecommendation{}
	alternatives := []resource.ModelRecommendation{}
	supplementary := []resource.ModelRecommendation{}

	for _, rec := range recommendations {
		switch rec.Priority {
		case 1:
			primary = append(primary, rec)
		case 2:
			alternatives = append(alternatives, rec)
		case 3:
			supplementary = append(supplementary, rec)
		}
	}

	// Display recommendations
	fmt.Printf("  %sRecommended Models%s\n", brandPrimary, colorReset)
	fmt.Println()

	if len(primary) > 0 {
		for i, rec := range primary {
			fmt.Printf("    %s%d.%s %s%s%s %s(%s)%s\n",
				brandMuted, i+1, colorReset,
				colorBold, rec.ModelID, colorReset,
				brandMuted, rec.Quantization, colorReset)
			fmt.Printf("       %s%s · %s%s\n",
				colorDim, formatModelSize(rec.SizeGB), rec.Reason, colorReset)
		}
		fmt.Println()
	}

	if len(alternatives) > 0 {
		fmt.Printf("  %sAlternatives%s\n", brandMuted, colorReset)
		for _, rec := range alternatives {
			fmt.Printf("    %s◦%s %s %s(%s)%s\n",
				brandMuted, colorReset,
				rec.ModelID, brandMuted, rec.Quantization, colorReset)
			fmt.Printf("       %s%s · %s%s\n",
				colorDim, formatModelSize(rec.SizeGB), rec.Reason, colorReset)
		}
		fmt.Println()
	}

	if len(supplementary) > 0 {
		fmt.Printf("  %sSupplementary%s\n", brandMuted, colorReset)
		for _, rec := range supplementary {
			fmt.Printf("    %s◦%s %s %s(%s)%s\n",
				brandMuted, colorReset,
				rec.ModelID, brandMuted, rec.Quantization, colorReset)
			fmt.Printf("       %s%s · %s%s\n",
				colorDim, formatModelSize(rec.SizeGB), rec.Reason, colorReset)
		}
		fmt.Println()
	}

	fmt.Printf("  %sNext Steps%s\n", brandPrimary, colorReset)
	if len(primary) > 0 {
		// Use catalog download with proper model ID
		fmt.Printf("    %s$%s offgrid download %s\n", colorDim, colorReset, primary[0].ModelID)
		fmt.Printf("    %s$%s offgrid list %s# View installed models%s\n", colorDim, colorReset, colorDim, colorReset)
	}
	fmt.Println()
}

func handleBenchmark(args []string) {
	// Check for help flag
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		args = []string{} // Trigger help display
	}

	if len(args) < 1 {
		fmt.Println()
		fmt.Printf("  %s◈ Benchmark Model%s\n", brandPrimary+colorBold, colorReset)
		fmt.Printf("  %sMeasure model performance and resource usage%s\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sUsage%s  offgrid benchmark <model-id> [options]\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sOptions%s\n", brandPrimary, colorReset)
		fmt.Printf("    %-22s %sNumber of iterations (default: 3)%s\n", "--iterations N", colorDim, colorReset)
		fmt.Printf("    %-22s %sCustom prompt to benchmark%s\n", "--prompt \"text\"", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sExamples%s\n", brandPrimary, colorReset)
		fmt.Printf("    %s$%s offgrid benchmark tinyllama-1.1b-chat.Q4_K_M\n", colorDim, colorReset)
		fmt.Printf("    %s$%s offgrid benchmark llama-2-7b-chat.Q5_K_M --iterations 5\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sNote:%s Server must be running first: %soffgrid serve%s\n", brandMuted, colorReset, colorBold, colorReset)
		fmt.Println()
		os.Exit(1)
	}

	cfg := config.LoadConfig()
	registry := models.NewRegistry(cfg.ModelsDir)

	if err := registry.ScanModels(); err != nil {
		fmt.Fprintf(os.Stderr, "✗ Error scanning models: %v\n\n", err)
		os.Exit(1)
	}

	// Parse arguments
	modelID := args[0]
	iterations := 3
	customPrompt := ""

	for i := 1; i < len(args); i++ {
		if args[i] == "--iterations" && i+1 < len(args) {
			fmt.Sscanf(args[i+1], "%d", &iterations)
			i++
		} else if args[i] == "--prompt" && i+1 < len(args) {
			customPrompt = args[i+1]
			i++
		}
	}

	// Check if model exists
	meta, err := registry.GetModel(modelID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ Model not found: %s\n\n", modelID)

		// Show available models
		modelList := registry.ListModels()
		if len(modelList) > 0 {
			fmt.Fprintln(os.Stderr, "Available models:")
			for _, m := range modelList {
				fmt.Fprintf(os.Stderr, "  • %s\n", m.ID)
			}
			fmt.Fprintln(os.Stderr, "")
		} else {
			fmt.Fprintln(os.Stderr, "No models installed. Use 'offgrid download' to add models.")
			fmt.Fprintln(os.Stderr, "")
		}
		os.Exit(1)
	}

	// Print benchmark header
	fmt.Println()
	fmt.Printf("%sBenchmark · %s%s\n", brandPrimary+colorBold, modelID, colorReset)
	fmt.Println("")
	fmt.Printf("%sPath:%s %s\n", colorDim, colorReset, meta.Path)
	fmt.Printf("%sSize:%s %s", colorDim, colorReset, formatBytes(meta.Size))
	if meta.Quantization != "" {
		fmt.Printf(" · %s", meta.Quantization)
	}
	fmt.Println()
	fmt.Println()

	// Check if server is running
	serverURL := fmt.Sprintf("http://127.0.0.1:%d", cfg.ServerPort)
	if !isServerHealthy(serverURL) {
		fmt.Printf("%sError: Server not running%s\n", colorRed, colorReset)
		fmt.Printf("Start server first: %soffgrid serve%s\n\n", brandSecondary, colorReset)
		os.Exit(1)
	}

	// Default benchmark prompt
	benchPrompt := "Write a short story about a robot learning to paint."
	if customPrompt != "" {
		benchPrompt = customPrompt
	}

	fmt.Printf("Running %d iterations...\n\n", iterations)

	// Run benchmark iterations
	var (
		totalLatency      time.Duration
		totalTokens       int
		totalPromptTokens int
		firstTokenTimes   []time.Duration
		tokensPerSec      []float64
	)

	for i := 0; i < iterations; i++ {
		fmt.Printf("%s  [%d/%d]%s Testing... ", colorDim, i+1, iterations, colorReset)

		startTime := time.Now()
		var firstTokenTime time.Duration
		tokenCount := 0
		promptTokens := 0

		// Call completion endpoint
		reqBody := map[string]interface{}{
			"prompt":      benchPrompt,
			"max_tokens":  100,
			"temperature": 0.7,
			"stream":      false,
		}

		jsonData, _ := json.Marshal(reqBody)
		resp, err := http.Post(
			serverURL+"/v1/completions",
			"application/json",
			bytes.NewBuffer(jsonData),
		)

		if err != nil {
			fmt.Printf("%s✗%s\n", colorRed, colorReset)
			continue
		}

		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)

		var result map[string]interface{}
		if err := json.Unmarshal(body, &result); err != nil {
			fmt.Printf("%s✗%s Failed to parse response\n", brandError, colorReset)
			continue
		}

		latency := time.Since(startTime)

		// Extract metrics from response
		if usage, ok := result["usage"].(map[string]interface{}); ok {
			if pt, ok := usage["prompt_tokens"].(float64); ok {
				promptTokens = int(pt)
			}
			if ct, ok := usage["completion_tokens"].(float64); ok {
				tokenCount = int(ct)
			}
		}

		// Estimate first token time (roughly 10% of total for most models)
		firstTokenTime = latency / 10

		totalLatency += latency
		totalTokens += tokenCount
		totalPromptTokens += promptTokens
		firstTokenTimes = append(firstTokenTimes, firstTokenTime)

		tps := float64(tokenCount) / latency.Seconds()
		tokensPerSec = append(tokensPerSec, tps)

		fmt.Printf("%s✓%s %s (%.1f tok/s)\n", colorGreen, colorReset, formatDuration(latency), tps)
	}

	if len(tokensPerSec) == 0 {
		printError("All benchmark iterations failed")
		os.Exit(1)
	}

	// Calculate averages
	avgLatency := totalLatency / time.Duration(len(tokensPerSec))
	avgTokens := float64(totalTokens) / float64(len(tokensPerSec))
	avgPromptTokens := float64(totalPromptTokens) / float64(len(tokensPerSec))
	avgFirstToken := time.Duration(0)
	for _, ft := range firstTokenTimes {
		avgFirstToken += ft
	}
	avgFirstToken /= time.Duration(len(firstTokenTimes))

	avgTPS := 0.0
	minTPS := tokensPerSec[0]
	maxTPS := tokensPerSec[0]
	for _, tps := range tokensPerSec {
		avgTPS += tps
		if tps < minTPS {
			minTPS = tps
		}
		if tps > maxTPS {
			maxTPS = tps
		}
	}
	avgTPS /= float64(len(tokensPerSec))

	// Display results
	fmt.Println()
	fmt.Printf("  %sResults%s\n", brandPrimary, colorReset)
	fmt.Println()
	fmt.Printf("    Speed       %s%.1f tok/s%s %s(%.1f - %.1f)%s\n", colorBold, avgTPS, colorReset, colorDim, minTPS, maxTPS, colorReset)
	fmt.Printf("    Latency     %s%s%s %s(first token: ~%s)%s\n", colorBold, formatDuration(avgLatency), colorReset, colorDim, formatDuration(avgFirstToken), colorReset)
	fmt.Printf("    Tokens      %s%.0f prompt, %.0f generated%s %s(avg)%s\n", colorBold, avgPromptTokens, avgTokens, colorReset, colorDim, colorReset)
	fmt.Printf("    Throughput  %s~%d queries/hour%s\n", colorBold, int(3600.0/avgLatency.Seconds()), colorReset)

	memEst := float64(meta.Size) * 1.2 // Rough estimate: model + context
	fmt.Printf("    Memory      %s%.1f GB estimated%s\n", colorDim, memEst/1e9, colorReset)
	fmt.Println()
}

func handleTest(args []string) {
	fmt.Println()
	fmt.Printf("  %s◈ System Test%s\n", brandPrimary+colorBold, colorReset)
	fmt.Printf("  %sTesting model switching and chat completions%s\n", colorDim, colorReset)
	fmt.Println()

	cfg := config.LoadConfig()
	serverURL := fmt.Sprintf("http://localhost:%d", cfg.ServerPort)

	// Check if server is running
	fmt.Printf("  %sServer%s\n", brandPrimary, colorReset)
	resp, err := http.Get(serverURL + "/v1/health")
	if err != nil {
		fmt.Printf("%s✗%s Server not running on port %d\n", brandError, colorReset, cfg.ServerPort)
		fmt.Printf("%sℹ%s Start server with: offgrid serve\n", colorDim, colorReset)
		fmt.Printf("%s%s\n", brandPrimary, colorReset)
		fmt.Println()
		os.Exit(1)
	}
	resp.Body.Close()
	fmt.Printf("%s✓%s Server running\n", brandSuccess, colorReset)

	// Load models
	fmt.Printf("%sLoading Models%s\n", brandPrimary+colorBold, colorReset)
	resp, err = http.Get(serverURL + "/models")
	if err != nil {
		fmt.Printf("%s✗%s Failed to fetch models: %v\n", brandError, colorReset, err)
		fmt.Printf("%s%s\n", brandPrimary, colorReset)
		fmt.Println()
		os.Exit(1)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	modelsData := result["data"]
	if modelsData == nil {
		fmt.Printf("%s✗%s No models found\n", brandError, colorReset)
		fmt.Printf("%s%s\n", brandPrimary, colorReset)
		fmt.Println()
		os.Exit(1)
	}

	modelsArray, ok := modelsData.([]interface{})
	if !ok || len(modelsArray) == 0 {
		fmt.Printf("%s✗%s No models available\n", brandError, colorReset)
		fmt.Printf("%s%s\n", brandPrimary, colorReset)
		fmt.Println()
		os.Exit(1)
	}

	fmt.Printf("%s✓%s Found %d models\n", brandSuccess, colorReset, len(modelsArray))

	// Test with up to 2 models
	testModels := []string{}
	for i, m := range modelsArray {
		if i >= 2 {
			break
		}
		modelMap := m.(map[string]interface{})
		modelID := modelMap["id"].(string)
		testModels = append(testModels, modelID)
		fmt.Printf("%s• %s%s\n", colorDim, modelID, colorReset)
	}

	// Run tests
	fmt.Printf("%sRunning Tests%s\n", brandPrimary+colorBold, colorReset)

	for i, modelID := range testModels {
		fmt.Printf("%sTest %d/%d:%s %s\n", brandSecondary, i+1, len(testModels), colorReset, modelID)

		startTime := time.Now()

		payload := map[string]interface{}{
			"model": modelID,
			"messages": []map[string]string{
				{"role": "user", "content": "Say just hello"},
			},
			"max_tokens": 10,
			"stream":     false,
		}

		jsonData, _ := json.Marshal(payload)
		resp, err := http.Post(
			serverURL+"/v1/chat/completions",
			"application/json",
			bytes.NewBuffer(jsonData),
		)

		if err != nil {
			fmt.Printf("%s✗%s Request failed: %v\n", brandError, colorReset, err)
			continue
		}

		var chatResult map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&chatResult)
		resp.Body.Close()

		duration := time.Since(startTime)

		if choices, ok := chatResult["choices"].([]interface{}); ok && len(choices) > 0 {
			choice := choices[0].(map[string]interface{})
			message := choice["message"].(map[string]interface{})
			content := message["content"].(string)

			trimmedContent := strings.TrimSpace(content)
			if trimmedContent == "" {
				fmt.Printf("%s⚠%s Response: (empty response)\n", brandAccent, colorReset)
			} else {
				// Limit output to first 60 chars
				displayContent := trimmedContent
				if len(displayContent) > 60 {
					displayContent = displayContent[:60] + "..."
				}
				fmt.Printf("%s✓%s Response: %s\n", brandSuccess, colorReset, displayContent)
			}
			fmt.Printf("%s⏱%s  Time: %s\n", colorDim, colorReset, formatDuration(duration))
		} else {
			fmt.Printf("%s✗%s Invalid response format\n", brandError, colorReset)
		}
	}

	fmt.Printf("%sAll Tests Complete%s\n", brandPrimary+colorBold, colorReset)
	fmt.Println()
}

func handleList(args []string) {
	cfg := config.LoadConfig()
	registry := models.NewRegistry(cfg.ModelsDir)

	if err := registry.ScanModels(); err != nil {
		if output.JSONMode {
			output.Error("Error scanning models", err)
		}
		printError(fmt.Sprintf("Error scanning models: %v", err))
		os.Exit(1)
	}

	modelList := registry.ListModels()

	// JSON output mode
	if output.JSONMode {
		var jsonModels []output.ModelInfo
		for _, model := range modelList {
			meta, err := registry.GetModel(model.ID)
			modelInfo := output.ModelInfo{
				Name: model.ID,
			}
			if err == nil {
				if meta.Size > 0 {
					modelInfo.Size = formatBytes(meta.Size)
				}
				if meta.Quantization != "" && meta.Quantization != "unknown" {
					modelInfo.Quantization = meta.Quantization
				}
				modelInfo.Format = meta.Format
				if meta.Path != "" {
					modelInfo.Path = meta.Path
				}
			}
			jsonModels = append(jsonModels, modelInfo)
		}
		output.PrintModels(jsonModels)
		return
	}

	// Human-readable output - Modern design
	fmt.Println()
	fmt.Printf("  %s%s Installed Models%s\n", brandPrimary+colorBold, iconModel, colorReset)

	if len(modelList) == 0 {
		fmt.Println()
		fmt.Printf("    %sNo models installed%s\n", brandMuted, colorReset)
		fmt.Println()
		fmt.Printf("    %sGet started:%s\n", colorBold, colorReset)
		fmt.Printf("      %s$%s offgrid recommend     %s# Find models for your system%s\n", brandMuted, colorReset, brandMuted, colorReset)
		fmt.Printf("      %s$%s offgrid search llama  %s# Search HuggingFace%s\n", brandMuted, colorReset, brandMuted, colorReset)
		fmt.Println()
		return
	}

	fmt.Printf("    %s%d model(s) · %s total%s\n", brandMuted, len(modelList), func() string {
		var total int64
		for _, m := range modelList {
			if meta, err := registry.GetModel(m.ID); err == nil && meta.Size > 0 {
				total += meta.Size
			}
		}
		return formatBytes(total)
	}(), colorReset)
	fmt.Println()

	// Modern table header with subtle styling
	headerFormat := "    %s%-36s  %-10s  %-10s%s\n"
	fmt.Printf(headerFormat, brandMuted, "MODEL", "SIZE", "QUANT", colorReset)
	fmt.Printf("    %s%s%s\n", brandMuted, strings.Repeat("─", 60), colorReset)

	for _, model := range modelList {
		meta, err := registry.GetModel(model.ID)

		modelName := model.ID
		maxNameLen := 36
		if len(modelName) > maxNameLen {
			modelName = modelName[:maxNameLen-1] + "…"
		}

		sizeStr := "—"
		quantStr := "—"

		if err == nil {
			if meta.Size > 0 {
				sizeStr = formatBytes(meta.Size)
			}
			if meta.Quantization != "" && meta.Quantization != "unknown" {
				quantStr = meta.Quantization
			}
		}

		fmt.Printf("    %s%-36s%s  %s%-10s%s  %s%-10s%s\n",
			colorBold, modelName, colorReset,
			brandMuted, sizeStr, colorReset,
			brandMuted, quantStr, colorReset)
	}

	fmt.Println()
	fmt.Printf("    %sRun:%s offgrid run <model>%s\n", brandMuted, colorReset+colorBold, colorReset)
	fmt.Println()
}

func handleQuantization() {
	fmt.Println()
	fmt.Printf("  %s◈ Quantization Guide%s\n", brandPrimary+colorBold, colorReset)
	fmt.Printf("  %sLower bits = smaller size + faster speed - slight quality loss%s\n", colorDim, colorReset)
	fmt.Println()

	// Group by quality tier
	tiers := []struct {
		name   string
		quants []string
	}{
		{"Compact (2-3 bit)", []string{"Q2_K", "Q3_K_S", "Q3_K_M"}},
		{"Balanced (4 bit) — Recommended", []string{"Q4_K_S", "Q4_K_M"}},
		{"High Quality (5-6 bit)", []string{"Q5_K_S", "Q5_K_M", "Q6_K"}},
		{"Maximum Quality (8 bit)", []string{"Q8_0"}},
	}

	for _, tier := range tiers {
		fmt.Printf("  %s%s%s\n", brandPrimary, tier.name, colorReset)
		for _, quant := range tier.quants {
			info := models.GetQuantizationInfo(quant)
			star := "  "
			starColor := ""
			if quant == "Q4_K_M" || quant == "Q5_K_M" {
				star = "★ "
				starColor = brandSuccess
			}

			fmt.Printf("    %s%s%s%-8s%s %.1f bits · %s%s\n",
				starColor, star, colorReset,
				info.Name, brandMuted,
				info.BitsPerWeight,
				info.Description, colorReset)
		}
		fmt.Println()
	}

	fmt.Printf("  %sRecommendations%s\n", brandPrimary, colorReset)
	fmt.Printf("    %s★%s Q4_K_M   %sBest for most users (4.0 GB for 7B)%s\n", brandSuccess, colorReset, colorDim, colorReset)
	fmt.Printf("    %s★%s Q5_K_M   %sProduction quality (4.8 GB for 7B)%s\n", brandSuccess, colorReset, colorDim, colorReset)
	fmt.Printf("      Q3_K_M   %sLimited RAM (3.0 GB for 7B)%s\n", colorDim, colorReset)
	fmt.Printf("      Q8_0     %sMaximum quality (7.2 GB for 7B)%s\n", colorDim, colorReset)
	fmt.Println()
}

func handleQuantize(args []string) {
	if len(args) < 2 {
		fmt.Println()
		fmt.Printf("  %s◈ Quantize Model%s\n", brandPrimary+colorBold, colorReset)
		fmt.Printf("  %sConvert a model to a different precision level%s\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sUsage%s  offgrid quantize <model-id> <quant> [options]\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sQuantization Types%s\n", brandPrimary, colorReset)
		fmt.Printf("    Q2_K     %s2-bit (smallest, lowest quality)%s\n", colorDim, colorReset)
		fmt.Printf("    Q3_K_M   %s3-bit medium%s\n", colorDim, colorReset)
		fmt.Printf("    %s★%s Q4_K_M %s4-bit medium (recommended)%s\n", brandSuccess, colorReset, colorDim, colorReset)
		fmt.Printf("    %s★%s Q5_K_M %s5-bit medium (high quality)%s\n", brandSuccess, colorReset, colorDim, colorReset)
		fmt.Printf("    Q6_K     %s6-bit%s\n", colorDim, colorReset)
		fmt.Printf("    Q8_0     %s8-bit (largest, highest quality)%s\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sOptions%s\n", brandPrimary, colorReset)
		fmt.Printf("    %-20s %sOutput model name%s\n", "--output <name>", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sExamples%s\n", brandPrimary, colorReset)
		fmt.Printf("    %s$%s offgrid quantize llama-2-7b.F16 Q4_K_M\n", colorDim, colorReset)
		fmt.Printf("    %s$%s offgrid quantize phi-2.F16 Q5_K_M --output phi-2-hq\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sTip:%s Use %soffgrid quantization%s for quality comparisons\n", brandMuted, colorReset, colorBold, colorReset)
		fmt.Println()
		os.Exit(1)
	}

	cfg := config.LoadConfig()
	registry := models.NewRegistry(cfg.ModelsDir)

	if err := registry.ScanModels(); err != nil {
		printError(fmt.Sprintf("Error scanning models: %v", err))
		os.Exit(1)
	}

	modelID := args[0]
	targetQuant := strings.ToUpper(args[1])
	outputName := ""

	// Parse optional --output flag
	for i := 2; i < len(args); i++ {
		if args[i] == "--output" && i+1 < len(args) {
			outputName = args[i+1]
			i++
		}
	}

	// Check if model exists
	meta, err := registry.GetModel(modelID)
	if err != nil {
		printError(fmt.Sprintf("Model not found: %s", modelID))
		fmt.Println()
		modelList := registry.ListModels()
		if len(modelList) > 0 {
			fmt.Println("Available models:")
			for _, m := range modelList {
				fmt.Printf("  • %s\n", m.ID)
			}
			fmt.Println()
		}
		os.Exit(1)
	}

	// Validate quantization type
	validQuants := []string{"Q2_K", "Q3_K_S", "Q3_K_M", "Q3_K_L", "Q4_0", "Q4_1", "Q4_K_S", "Q4_K_M", "Q5_0", "Q5_1", "Q5_K_S", "Q5_K_M", "Q6_K", "Q8_0"}
	isValid := false
	for _, q := range validQuants {
		if targetQuant == q {
			isValid = true
			break
		}
	}
	if !isValid {
		printError(fmt.Sprintf("Invalid quantization type: %s", targetQuant))
		fmt.Printf("\n%sValid types:%s Q2_K, Q3_K_S, Q3_K_M, Q4_K_S, Q4_K_M, Q5_K_S, Q5_K_M, Q6_K, Q8_0\n\n", brandMuted, colorReset)
		os.Exit(1)
	}

	// Generate output filename
	if outputName == "" {
		// Remove extension and current quantization from model ID
		baseName := strings.TrimSuffix(modelID, filepath.Ext(modelID))
		baseName = strings.TrimSuffix(baseName, ".gguf")
		// Remove existing quantization suffix if present
		for _, q := range validQuants {
			if strings.HasSuffix(baseName, "."+q) {
				baseName = strings.TrimSuffix(baseName, "."+q)
				break
			}
			if strings.HasSuffix(baseName, "-"+q) {
				baseName = strings.TrimSuffix(baseName, "-"+q)
				break
			}
		}
		outputName = fmt.Sprintf("%s.%s", baseName, targetQuant)
	}
	outputPath := filepath.Join(cfg.ModelsDir, outputName+".gguf")

	// Check if output file already exists
	if _, err := os.Stat(outputPath); err == nil {
		printError(fmt.Sprintf("Output file already exists: %s", outputName+".gguf"))
		fmt.Printf("\n%sUse --output to specify a different name%s\n\n", brandMuted, colorReset)
		os.Exit(1)
	}

	// Print quantization header
	fmt.Printf("\n%sQuantize Model%s\n", brandPrimary+colorBold, colorReset)
	fmt.Println("")

	fmt.Printf("%sSource Model%s\n", brandPrimary, colorReset)
	fmt.Printf("Name: %s%s%s\n", colorBold, modelID, colorReset)
	fmt.Printf("Path: %s%s%s\n", brandMuted, meta.Path, colorReset)
	fmt.Printf("Size: %s%s%s", brandPrimary, formatBytes(meta.Size), colorReset)
	if meta.Quantization != "" {
		fmt.Printf(" · %s%s%s", brandMuted, meta.Quantization, colorReset)
	}
	fmt.Println()
	fmt.Println("")

	quantInfo := models.GetQuantizationInfo(targetQuant)
	fmt.Printf("%sTarget Quantization%s\n", brandPrimary, colorReset)
	fmt.Printf("Type:    %s%s%s\n", brandPrimary, targetQuant, colorReset)
	fmt.Printf("Bits:    %.1f bits per weight\n", quantInfo.BitsPerWeight)
	fmt.Printf("Quality: %s\n", quantInfo.Description)
	fmt.Printf("Output:  %s%s.gguf%s\n", brandMuted, outputName, colorReset)
	fmt.Println("")

	// Check if llama-quantize is available
	llamaQuantize := "llama-quantize"
	if _, err := exec.LookPath(llamaQuantize); err != nil {
		printError("llama-quantize tool not found")
		fmt.Printf("\n%sInstall llama.cpp first:%s\n", brandMuted, colorReset)
		fmt.Println("  cd /tmp && git clone https://github.com/ggerganov/llama.cpp")
		fmt.Println("  cd llama.cpp && make")
		fmt.Println("  sudo cp llama-quantize /usr/local/bin/")
		fmt.Println()
		os.Exit(1)
	}

	// Run quantization
	fmt.Println("Starting quantization...")
	fmt.Println()

	cmd := exec.Command(llamaQuantize, meta.Path, outputPath, targetQuant)

	// Set LD_LIBRARY_PATH to include /usr/local/lib for llama.cpp shared libraries
	env := os.Environ()
	ldLibPath := "/usr/local/lib"
	if existingPath := os.Getenv("LD_LIBRARY_PATH"); existingPath != "" {
		ldLibPath = ldLibPath + ":" + existingPath
	}
	env = append(env, "LD_LIBRARY_PATH="+ldLibPath)
	cmd.Env = env

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	startTime := time.Now()
	if err := cmd.Run(); err != nil {
		fmt.Println()
		printError(fmt.Sprintf("Quantization failed: %v", err))
		os.Exit(1)
	}
	duration := time.Since(startTime)

	// Get output file size
	outputStat, err := os.Stat(outputPath)
	if err != nil {
		printError(fmt.Sprintf("Failed to stat output file: %v", err))
		os.Exit(1)
	}

	// Calculate compression ratio
	compressionRatio := float64(meta.Size) / float64(outputStat.Size())
	sizeSaved := meta.Size - outputStat.Size()

	// Display results
	fmt.Println()
	fmt.Printf("%sQuantization Complete%s\n", brandSuccess+colorBold, colorReset)
	fmt.Println("")

	fmt.Printf("%sResults%s\n", brandSuccess, colorReset)
	fmt.Printf("Original:  %s\n", formatBytes(meta.Size))
	fmt.Printf("Quantized: %s%s%s\n", brandSuccess, formatBytes(outputStat.Size()), colorReset)
	fmt.Printf("Saved:     %s%s%s (%.1fx smaller)\n", brandPrimary, formatBytes(sizeSaved), colorReset, compressionRatio)
	fmt.Printf("Time:      %s\n", formatDuration(duration))
	fmt.Println("")

	fmt.Printf("%sModel Ready%s\n", brandSuccess, colorReset)
	fmt.Printf("Name:     %s%s%s\n", brandPrimary, outputName, colorReset)
	fmt.Printf("Location: %s%s%s\n", brandMuted, outputPath, colorReset)
	fmt.Println("")
	fmt.Println()

	fmt.Printf("%sNext Steps%s\n", brandMuted, colorReset)
	fmt.Printf("  Test model:  %soffgrid run %s%s\n", brandMuted, outputName, colorReset)
	fmt.Printf("  Benchmark:   %soffgrid benchmark %s%s\n", brandMuted, outputName, colorReset)
	fmt.Println()
}

func printHelp() {
	fmt.Println()
	// Modern branded header
	fmt.Printf("  %s%s OffGrid LLM%s %s%s%s\n", brandPrimary+colorBold, iconBolt, colorReset, brandMuted, getVersion(), colorReset)
	fmt.Printf("  %sEdge inference orchestrator for local LLMs%s\n", brandMuted, colorReset)
	fmt.Println()
	fmt.Printf("  %sUsage%s  offgrid %s<command>%s %s[options]%s\n", colorBold, colorReset, brandPrimary, colorReset, brandMuted, colorReset)
	fmt.Println()

	// Define structure for commands to ensure global alignment
	type cmdEntry struct {
		name string
		desc string
	}

	type section struct {
		title string
		icon  string
		cmds  []cmdEntry
	}

	sections := []section{
		{
			title: "Model Management",
			icon:  iconDownload,
			cmds: []cmdEntry{
				{"recommend", "Get model recommendations for your system"},
				{"list", "List installed models"},
				{"search <query>", "Search HuggingFace"},
				{"download <id>", "Download from catalog"},
				{"download-hf <id>", "Download from HuggingFace"},
				{"import <path>", "Import from USB/SD card"},
				{"export <id> <dst>", "Export to USB/SD card"},
				{"remove <id>", "Remove installed model"},
			},
		},
		{
			title: "Inference & Chat",
			icon:  iconCircle,
			cmds: []cmdEntry{
				{"serve", "Start API server (default)"},
				{"run <model>", "Interactive chat (--image for VLMs)"},
				{"session <cmd>", "Manage chat sessions"},
				{"kb <cmd>", "Manage knowledge base (RAG)"},
				{"template <cmd>", "Manage prompt templates"},
				{"batch <file>", "Batch process prompts"},
			},
		},
		{
			title: "System",
			icon:  iconCpu,
			cmds: []cmdEntry{
				{"init", "First-time setup wizard"},
				{"doctor", "Run system diagnostics"},
				{"info", "System information"},
				{"config <action>", "Manage configuration"},
				{"benchmark <id>", "Performance testing"},
			},
		},
		{
			title: "Advanced",
			icon:  iconStar,
			cmds: []cmdEntry{
				{"lora <cmd>", "LoRA adapter management"},
				{"agent <cmd>", "AI agent workflows"},
				{"audio <cmd>", "Speech-to-text & TTS"},
				{"users <cmd>", "Multi-user management"},
			},
		},
	}

	// Use fixed column width of 22 chars for command column
	const columnWidth = 22

	for _, s := range sections {
		fmt.Printf("  %s%s %s%s\n", brandPrimary, s.icon, s.title, colorReset)
		for _, c := range s.cmds {
			// Calculate padding needed to reach fixed column width
			paddingNeeded := columnWidth - len(c.name)
			if paddingNeeded < 2 {
				paddingNeeded = 2
			}
			padding := strings.Repeat(" ", paddingNeeded)

			fmt.Printf("    %s%s%s%s%s%s\n",
				colorBold, c.name, colorReset,
				padding,
				brandMuted, c.desc)
		}
		fmt.Println()
	}

	// Quick start examples in a more compact format
	fmt.Printf("  %s%s Quick Start%s\n", brandPrimary, iconArrow, colorReset)
	fmt.Printf("    %s$%s offgrid init                     %s# First-time setup%s\n", brandMuted, colorReset, brandMuted, colorReset)
	fmt.Printf("    %s$%s offgrid recommend                %s# Get model suggestions%s\n", brandMuted, colorReset, brandMuted, colorReset)
	fmt.Printf("    %s$%s offgrid download-hf <model>      %s# Download a model%s\n", brandMuted, colorReset, brandMuted, colorReset)
	fmt.Printf("    %s$%s offgrid run <model>              %s# Start chatting%s\n", brandMuted, colorReset, brandMuted, colorReset)
	fmt.Println()

	// Footer with helpful info
	fmt.Printf("  %sRun %soffgrid <command> --help%s %sfor detailed usage%s\n", brandMuted, colorReset+colorBold, colorReset, brandMuted, colorReset)
	fmt.Printf("  %sDocs: %shttps://github.com/takuphilchan/offgrid-llm%s\n", brandMuted, brandPrimary, colorReset)
	fmt.Println()
}

func handleVersion() {
	version := getVersion()
	if output.JSONMode {
		output.PrintJSON(map[string]interface{}{
			"version": version,
			"go":      runtime.Version(),
			"os":      runtime.GOOS,
			"arch":    runtime.GOARCH,
		})
		return
	}

	fmt.Println()
	fmt.Printf("  %s%s OffGrid LLM%s\n", brandPrimary+colorBold, iconBolt, colorReset)
	fmt.Println()
	printKeyValue("Version", version)
	printKeyValue("Go", runtime.Version())
	printKeyValue("Platform", fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH))
	fmt.Println()
}

func handleInfo() {
	cfg := config.LoadConfig()
	registry := models.NewRegistry(cfg.ModelsDir)

	if err := registry.ScanModels(); err != nil {
		if !output.JSONMode {
			fmt.Fprintf(os.Stderr, "  ✗ Model scan error: %v\n", err)
		}
	}

	modelList := registry.ListModels()

	// JSON output mode
	if output.JSONMode {
		var cpuInfo string
		var memInfo string
		var gpuInfo string
		var osInfo string
		var archInfo string

		// Simple system info gathering
		if runtime.GOOS != "" {
			osInfo = runtime.GOOS
			archInfo = runtime.GOARCH
		}

		// Get CPU count
		cpuInfo = fmt.Sprintf("%d cores", runtime.NumCPU())

		// Try to get memory info
		if memStat, err := os.ReadFile("/proc/meminfo"); err == nil {
			lines := strings.Split(string(memStat), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "MemTotal:") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						memInfo = fields[1] + " kB"
						break
					}
				}
			}
		}

		sysInfo := output.SystemInfo{
			CPU:          cpuInfo,
			Memory:       memInfo,
			GPU:          gpuInfo,
			OS:           osInfo,
			Architecture: archInfo,
		}

		var jsonModels []output.ModelInfo
		for _, model := range modelList {
			meta, _ := registry.GetModel(model.ID)
			modelInfo := output.ModelInfo{
				Name: model.ID,
			}
			if meta.Size > 0 {
				modelInfo.Size = formatBytes(meta.Size)
			}
			if meta.Quantization != "" && meta.Quantization != "unknown" {
				modelInfo.Quantization = meta.Quantization
			}
			if meta.Path != "" {
				modelInfo.Path = meta.Path
			}
			jsonModels = append(jsonModels, modelInfo)
		}

		output.PrintJSON(map[string]interface{}{
			"version": getVersion(),
			"config": map[string]interface{}{
				"port":        cfg.ServerPort,
				"models_dir":  cfg.ModelsDir,
				"max_context": cfg.MaxContextSize,
				"threads":     cfg.NumThreads,
				"max_memory":  cfg.MaxMemoryMB,
				"p2p_enabled": cfg.EnableP2P,
			},
			"models": map[string]interface{}{
				"installed": jsonModels,
				"count":     len(jsonModels),
			},
			"system": sysInfo,
		})
		return
	}

	// Human-readable output
	fmt.Println()
	fmt.Printf("  %s◈ OffGrid LLM%s %sv%s%s\n", brandPrimary+colorBold, colorReset, brandMuted, getVersion(), colorReset)
	fmt.Println()

	// Configuration
	fmt.Printf("  %sConfiguration%s\n", brandPrimary, colorReset)
	fmt.Printf("    Port       %s%d%s\n", colorDim, cfg.ServerPort, colorReset)
	fmt.Printf("    Models     %s%s%s\n", colorDim, cfg.ModelsDir, colorReset)
	fmt.Printf("    Threads    %s%d%s\n", colorDim, cfg.NumThreads, colorReset)
	fmt.Printf("    Context    %s%d tokens%s\n", colorDim, cfg.MaxContextSize, colorReset)
	fmt.Printf("    Memory     %s%d MB%s\n", colorDim, cfg.MaxMemoryMB, colorReset)
	if cfg.EnableP2P {
		fmt.Printf("    P2P        %senabled (port %d)%s\n", colorDim, cfg.P2PPort, colorReset)
	}
	fmt.Println()

	// Installed Models
	var totalSize int64
	modelCount := len(modelList)
	if modelCount == 1 {
		fmt.Printf("  %sInstalled Models%s %s· 1 model%s\n", brandPrimary, colorReset, brandMuted, colorReset)
	} else {
		fmt.Printf("  %sInstalled Models%s %s· %d models%s\n", brandPrimary, colorReset, brandMuted, modelCount, colorReset)
	}

	if len(modelList) > 0 {
		for _, model := range modelList {
			meta, err := registry.GetModel(model.ID)
			if err == nil {
				statusIcon := "○"
				statusColor := colorDim
				if meta.IsLoaded {
					statusIcon := "●"
					statusColor = brandSuccess
					fmt.Printf("    %s%s%s %s", statusColor, statusIcon, colorReset, model.ID)
				} else {
					fmt.Printf("    %s%s%s %s", statusColor, statusIcon, colorReset, model.ID)
				}
				if meta.Size > 0 {
					fmt.Printf(" %s· %s%s", colorDim, formatBytes(meta.Size), colorReset)
					totalSize += meta.Size
				}
				if meta.Quantization != "" && meta.Quantization != "unknown" {
					fmt.Printf(" %s· %s%s", colorDim, meta.Quantization, colorReset)
				}
				fmt.Println()
			}
		}
		if totalSize > 0 {
			fmt.Printf("    %sTotal: %s%s\n", brandMuted, formatBytes(totalSize), colorReset)
		}
	} else {
		fmt.Printf("    %sNo models installed%s\n", brandMuted, colorReset)
	}
	fmt.Println()

	// Catalog info
	catalog := models.DefaultCatalog()
	recommended := 0
	for _, entry := range catalog.Models {
		if entry.Recommended {
			recommended++
		}
	}
	fmt.Printf("  %sCatalog%s %s· %d models (%d recommended)%s\n",
		brandPrimary, colorReset, brandMuted, len(catalog.Models), recommended, colorReset)
	fmt.Println()
}

func handleConfig(args []string) {
	// Check for help flag
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		args = []string{} // Trigger help display
	}

	if len(args) < 1 {
		fmt.Println()
		fmt.Printf("  %s◈ Configuration%s\n", brandPrimary+colorBold, colorReset)
		fmt.Printf("  %sManage settings and preferences%s\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sUsage%s  offgrid config <action>\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sActions%s\n", brandPrimary, colorReset)
		fmt.Printf("    %-20s %sCreate config file%s\n", "init [path]", colorDim, colorReset)
		fmt.Printf("    %-20s %sDisplay current config%s\n", "show", colorDim, colorReset)
		fmt.Printf("    %-20s %sValidate config file%s\n", "validate [path]", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sExamples%s\n", brandPrimary, colorReset)
		fmt.Printf("    %s$%s offgrid config init\n", colorDim, colorReset)
		fmt.Printf("    %s$%s offgrid config show\n", colorDim, colorReset)
		fmt.Printf("    %s$%s offgrid config validate config.yaml\n", colorDim, colorReset)
		fmt.Println()
		os.Exit(1)
	}

	action := args[0]

	switch action {
	case "init":
		// Determine output path
		outputPath := ""
		if len(args) > 1 {
			outputPath = args[1]
		} else {
			homeDir, _ := os.UserHomeDir()
			configDir := filepath.Join(homeDir, ".offgrid-llm")
			os.MkdirAll(configDir, 0755)
			outputPath = filepath.Join(configDir, "config.yaml")
		}

		// Create default config
		cfg := config.LoadConfig()

		// Save to file
		if err := cfg.SaveToFile(outputPath); err != nil {
			fmt.Fprintf(os.Stderr, "  ✗ Failed to create config: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("  ✓ Created config: %s\n\n", outputPath)
		fmt.Println("To use:")
		fmt.Printf("  export OFFGRID_CONFIG=%s\n", outputPath)
		fmt.Println("  offgrid")
		fmt.Println()

	case "show":
		configPath := os.Getenv("OFFGRID_CONFIG")
		cfg, err := config.LoadWithPriority(configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ✗ Failed to load config: %v\n", err)
			os.Exit(1)
		}

		fmt.Println()
		fmt.Println("Configuration")
		fmt.Println()
		fmt.Println("Server")
		fmt.Printf("  Host:             %s\n", cfg.ServerHost)
		fmt.Printf("  Port:             %d\n", cfg.ServerPort)
		fmt.Println()
		fmt.Println("Models")
		fmt.Printf("  Directory:        %s\n", cfg.ModelsDir)
		fmt.Printf("  Default:          %s\n", cfg.DefaultModel)
		fmt.Printf("  Max context:      %d\n", cfg.MaxContextSize)
		fmt.Printf("  Threads:          %d\n", cfg.NumThreads)
		fmt.Println()
		fmt.Println("Resources")
		fmt.Printf("  Max memory:       %d MB\n", cfg.MaxMemoryMB)
		fmt.Printf("  Max models:       %d\n", cfg.MaxModels)
		fmt.Printf("  GPU:              %v\n", cfg.EnableGPU)
		if cfg.EnableGPU {
			fmt.Printf("  GPU layers:       %d\n", cfg.NumGPULayers)
		}
		fmt.Println()
		if cfg.EnableP2P {
			fmt.Println("P2P")
			fmt.Printf("  Enabled:          %v\n", cfg.EnableP2P)
			fmt.Printf("  Port:             %d\n", cfg.P2PPort)
			fmt.Printf("  Discovery:        %d\n", cfg.DiscoveryPort)
			fmt.Println()
		}
		fmt.Println("Logging")
		fmt.Printf("  Level:            %s\n", cfg.LogLevel)
		if cfg.LogFile != "" {
			fmt.Printf("  File:             %s\n", cfg.LogFile)
		}
		fmt.Println()

	case "validate":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: offgrid config validate <path>")
			os.Exit(1)
		}

		configPath := args[1]
		cfg, err := config.LoadFromFile(configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ✗ Invalid config: %v\n", err)
			os.Exit(1)
		}

		if err := cfg.Validate(); err != nil {
			fmt.Fprintf(os.Stderr, "  ✗ Validation failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("  ✓ Config valid: %s\n", configPath)
		fmt.Println()

	default:
		fmt.Fprintf(os.Stderr, "Unknown action: %s\n", action)
		os.Exit(1)
	}
}

func handleSearch(args []string) {
	// Parse search query and filters
	var query string
	var filters models.SearchFilter
	var maxRAM int // Maximum RAM in GB (0 = no filter)

	// Default filters
	filters.OnlyGGUF = true
	filters.ExcludeGated = true
	filters.Limit = 20
	filters.SortBy = "downloads"

	// Parse arguments
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--author" || arg == "-a":
			if i+1 < len(args) {
				filters.Author = args[i+1]
				i++
			}
		case arg == "--quant" || arg == "-q":
			if i+1 < len(args) {
				filters.Quantization = args[i+1]
				i++
			}
		case arg == "--ram" || arg == "-r":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &maxRAM)
				i++
			}
		case arg == "--sort" || arg == "-s":
			if i+1 < len(args) {
				filters.SortBy = args[i+1]
				i++
			}
		case arg == "--limit" || arg == "-l":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &filters.Limit)
				i++
			}
		case arg == "--all":
			filters.ExcludeGated = false
		case arg == "--help" || arg == "-h":
			fmt.Println()
			fmt.Printf("  %s%s Search HuggingFace%s\n", brandPrimary+colorBold, iconSearch, colorReset)
			fmt.Println()
			fmt.Printf("  %sUsage%s  offgrid search %s[query]%s %s[options]%s\n", colorBold, colorReset, brandPrimary, colorReset, brandMuted, colorReset)
			fmt.Println()
			fmt.Printf("  %sOptions%s\n", colorBold, colorReset)
			fmt.Printf("    %s-a, --author%s <name>    Filter by author (e.g., 'TheBloke')\n", brandPrimary, colorReset)
			fmt.Printf("    %s-q, --quant%s <type>     Filter by quantization (e.g., 'Q4_K_M')\n", brandPrimary, colorReset)
			fmt.Printf("    %s-r, --ram%s <GB>         Filter by max RAM (e.g., 4, 8, 16)\n", brandPrimary, colorReset)
			fmt.Printf("    %s-s, --sort%s <field>     Sort by: downloads, likes, created\n", brandPrimary, colorReset)
			fmt.Printf("    %s-l, --limit%s <n>        Limit results (default: 20)\n", brandPrimary, colorReset)
			fmt.Printf("    %s--all%s                  Include gated models\n", brandPrimary, colorReset)
			fmt.Println()
			fmt.Printf("  %sExamples%s\n", colorBold, colorReset)
			fmt.Printf("    %s$%s offgrid search llama\n", brandMuted, colorReset)
			fmt.Printf("    %s$%s offgrid search llama --ram 4\n", brandMuted, colorReset)
			fmt.Printf("    %s$%s offgrid search mistral --author TheBloke --quant Q4_K_M\n", brandMuted, colorReset)
			fmt.Println()
			return
		default:
			if query == "" {
				query = arg
			}
		}
	}

	filters.Query = query

	if !output.JSONMode {
		fmt.Printf("\n%s%s%s Searching HuggingFace Hub%s\n", brandPrimary, iconSearch, colorBold, colorReset)
		fmt.Println()
	}

	hf := models.NewHuggingFaceClient()
	results, err := hf.SearchModels(filters)
	if err != nil {
		if output.JSONMode {
			output.Error("Search failed", err)
		}
		printError(fmt.Sprintf("Search failed: %v", err))
		fmt.Println()
		os.Exit(1)
	}

	// Filter by RAM if specified
	if maxRAM > 0 {
		var filtered []models.SearchResult
		for _, result := range results {
			var estimatedRAM float64
			if result.BestVariant != nil && result.BestVariant.SizeGB > 0 {
				// Use actual size if available
				estimatedRAM = result.BestVariant.SizeGB * 1.3
			} else if result.BestVariant != nil {
				// Estimate from model name and quantization
				estimatedRAM = estimateRAMFromModel(result.Model.ID, result.BestVariant.Quantization)
			}

			if estimatedRAM > 0 && estimatedRAM <= float64(maxRAM) {
				filtered = append(filtered, result)
			}
		}
		results = filtered

		if !output.JSONMode && len(filtered) > 0 {
			fmt.Printf("%s[Filtered for ≤%dGB RAM]%s\n", brandMuted, maxRAM, colorReset)
			fmt.Println()
		}
	} // JSON output mode
	if output.JSONMode {
		var jsonResults []output.SearchResult
		for _, result := range results {
			searchResult := output.SearchResult{
				Name:      result.Model.ID,
				ModelID:   result.Model.ID,
				Downloads: int(result.Model.Downloads),
				Likes:     result.Model.Likes,
			}
			if len(result.Model.Tags) > 0 {
				searchResult.Tags = result.Model.Tags
			}
			jsonResults = append(jsonResults, searchResult)
		}
		output.PrintSearchResults(jsonResults)
		return
	}

	// Human-readable output
	if len(results) == 0 {
		fmt.Println()
		printWarning("No models found matching your criteria")
		fmt.Println()
		fmt.Printf("    %sTry:%s offgrid search llama --ram 4\n", brandMuted, colorReset)
		fmt.Println()
		return
	}

	fmt.Printf("    %s%d model(s) found%s\n", brandMuted, len(results), colorReset)
	fmt.Println()

	for i, result := range results {
		model := result.Model

		// Number and model name (full - don't truncate)
		fmt.Printf("  %s%d.%s %s%s%s",
			brandMuted, i+1, colorReset,
			colorBold, model.ID, colorReset)

		// Quality badges inline
		if result.IsRecommended {
			fmt.Printf(" %s★%s", brandSuccess, colorReset)
		}
		if result.IsTrusted {
			fmt.Printf(" %s✓%s", brandPrimary, colorReset)
		}
		fmt.Println()

		// Stats line: downloads · likes · quant · size · RAM
		var infoParts []string
		infoParts = append(infoParts, fmt.Sprintf("↓ %s", formatNumber(model.Downloads)))
		infoParts = append(infoParts, fmt.Sprintf("♥ %s", formatNumber(int64(model.Likes))))

		if result.BestVariant != nil {
			infoParts = append(infoParts, result.BestVariant.Quantization)
			if result.BestVariant.SizeGB > 0 {
				infoParts = append(infoParts, fmt.Sprintf("%.1fGB", result.BestVariant.SizeGB))
				estimatedRAM := result.BestVariant.SizeGB * 1.3
				infoParts = append(infoParts, fmt.Sprintf("~%.0fGB RAM", estimatedRAM))
			} else {
				estimatedRAM := estimateRAMFromModel(result.Model.ID, result.BestVariant.Quantization)
				if estimatedRAM > 0 {
					infoParts = append(infoParts, fmt.Sprintf("~%.0fGB RAM", estimatedRAM))
				}
			}
		}
		fmt.Printf("     %s%s%s\n", colorDim, strings.Join(infoParts, " · "), colorReset)

		// Add spacing between results
		if i < len(results)-1 {
			fmt.Println()
		}
	}

	// Footer hint
	fmt.Println()
	fmt.Printf("  %sTip: offgrid download <model-name> to download%s\n", brandMuted, colorReset)
	fmt.Println()
}

func handleDownloadHF(args []string) {
	// Redirect to unified download command (backward compatibility)
	// download-hf is now just an alias for download with HuggingFace models
	handleDownload(args)
}

func downloadVisionProjector(hf *models.HuggingFaceClient, modelsDir string, projector *models.ProjectorSource) error {
	if projector == nil || projector.File.Filename == "" {
		return nil
	}

	remotePath := projector.File.Filename
	localName := filepath.Base(remotePath)
	destPath := filepath.Join(modelsDir, localName)
	if _, err := os.Stat(destPath); err == nil {
		fmt.Printf("  ✓ Vision adapter already present at %s%s%s\n", brandMuted, destPath, colorReset)
		return nil
	}

	sourceLabel := projector.ModelID
	if projector.Source == "fallback" {
		sourceLabel = fmt.Sprintf("%s (fallback)", sourceLabel)
	}
	fmt.Printf("  Source: %s%s%s\n", brandMuted, sourceLabel, colorReset)
	if projector.Reason != "" {
		fmt.Printf("  %s%s%s\n", brandMuted, projector.Reason, colorReset)
	}
	fmt.Printf("  ⧉ Downloading %s%s%s\n", brandPrimary, remotePath, colorReset)

	err := hf.DownloadGGUF(projector.ModelID, remotePath, destPath, func(current, total int64) {
		percent := 0.0
		if total > 0 {
			percent = float64(current) / float64(total) * 100
		}
		fmt.Printf("\r      %.1f%% (%s / %s)",
			percent,
			formatBytes(current),
			formatBytes(total))
	})

	fmt.Println()
	if err != nil {
		return err
	}

	fmt.Printf("  ✓ Vision adapter saved to %s%s%s\n", brandMuted, destPath, colorReset)
	return nil
}

func extractQuantFromFilename(filename string) string {
	patterns := []string{
		"Q2_K", "Q3_K_S", "Q3_K_M", "Q3_K_L",
		"Q4_0", "Q4_1", "Q4_K_S", "Q4_K_M",
		"Q5_0", "Q5_1", "Q5_K_S", "Q5_K_M",
		"Q6_K", "Q8_0", "F16", "F32",
	}

	upper := strings.ToUpper(filename)
	for _, pattern := range patterns {
		if strings.Contains(upper, pattern) {
			return pattern
		}
	}

	return "unknown"
}

func handleRun(args []string) {
	// Check for help flag first
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		args = []string{} // Trigger help display
	}

	if len(args) < 1 {
		fmt.Println()
		fmt.Printf("  %s%s Interactive Chat%s\n", brandPrimary+colorBold, iconCircle, colorReset)
		fmt.Println()
		fmt.Printf("  %sUsage%s  offgrid run %s<model>%s %s[options]%s\n", colorBold, colorReset, brandPrimary, colorReset, brandMuted, colorReset)
		fmt.Println()
		fmt.Printf("  %sOptions%s\n", colorBold, colorReset)
		fmt.Printf("    %s--save%s <name>      Save conversation to session\n", brandPrimary, colorReset)
		fmt.Printf("    %s--load%s <name>      Load and continue existing session\n", brandPrimary, colorReset)
		fmt.Printf("    %s--image%s <path>     Attach an image (for VLM models)\n", brandPrimary, colorReset)
		fmt.Printf("    %s--rag%s              Enable knowledge base (RAG)\n", brandPrimary, colorReset)
		fmt.Println()
		fmt.Printf("  %sExamples%s\n", colorBold, colorReset)
		fmt.Printf("    %s$%s offgrid run llama-3.1-8b-instruct\n", brandMuted, colorReset)
		fmt.Printf("    %s$%s offgrid run llava --image photo.jpg\n", brandMuted, colorReset)
		fmt.Printf("    %s$%s offgrid run llama --save my-project\n", brandMuted, colorReset)
		fmt.Println()
		fmt.Printf("  %sTip:%s Use %soffgrid list%s to see installed models\n", brandMuted, colorReset, colorBold, colorReset)
		fmt.Println()
		os.Exit(1)
	}

	modelName := args[0]
	var sessionName string
	var loadSession bool
	var saveSession bool
	var imagePath string
	var useKnowledgeBase bool

	// Parse flags
	for i := 1; i < len(args); i++ {
		if args[i] == "--save" && i+1 < len(args) {
			sessionName = args[i+1]
			saveSession = true
			i++
		} else if args[i] == "--load" && i+1 < len(args) {
			sessionName = args[i+1]
			loadSession = true
		} else if args[i] == "--image" && i+1 < len(args) {
			imagePath = args[i+1]
			i++
		} else if args[i] == "--rag" {
			useKnowledgeBase = true
		}
	}

	cfg := config.LoadConfig()
	registry := models.NewRegistry(cfg.ModelsDir)

	// Scan for local models
	if err := registry.ScanModels(); err != nil {
		fmt.Printf("Error scanning models: %v\n", err)
		os.Exit(1)
	}

	// Find the model
	model, err := registry.GetModel(modelName)
	if err != nil || model == nil {
		fmt.Printf("Model not found: %s\n", modelName)
		fmt.Println("Use 'offgrid list' to see available models")
		os.Exit(1)
	}

	// Check for vision model
	isVLM := strings.Contains(strings.ToLower(modelName), "llava") ||
		strings.Contains(strings.ToLower(modelName), "vision") ||
		strings.Contains(strings.ToLower(modelName), "vlm")

	// Find projector file if this is a VLM
	var projectorPath string
	if isVLM {
		// Look for projector file in models directory
		files, _ := os.ReadDir(cfg.ModelsDir)
		for _, f := range files {
			name := strings.ToLower(f.Name())
			if strings.Contains(name, "mmproj") || strings.Contains(name, "projector") {
				projectorPath = filepath.Join(cfg.ModelsDir, f.Name())
				break
			}
		}
	}
	_ = projectorPath // Silence unused warning if not VLM

	// Check available RAM before running the model
	sysResources, err := resource.DetectResources()
	if err == nil && sysResources.AvailableRAM > 0 {
		// Estimate RAM requirement for this model
		requiredRAM := estimateRAMFromModel(modelName, "")

		// Convert available RAM from MB to GB for comparison
		availableGB := float64(sysResources.AvailableRAM) / 1024.0

		// Show warning if model requires more RAM than available
		if requiredRAM > 0 && requiredRAM > availableGB {
			fmt.Println()
			printWarning(fmt.Sprintf("This model requires ~%.1fGB RAM, but you have ~%.1fGB available", requiredRAM, availableGB))
			fmt.Println()
			printInfo("The model may run slowly or fail to load. Consider:")
			fmt.Println("  • Closing other applications to free memory")
			fmt.Println("  • Using a smaller model (1B or 3B parameters)")
			fmt.Println("  • Using a more aggressive quantization (Q2_K or Q3_K)")
			fmt.Println("  • Upgrading your RAM")
			fmt.Println()
			printInfo("See docs/4GB_RAM.md for model recommendations")
			fmt.Println()
			fmt.Printf("%sContinue anyway?%s (y/N): ", brandMuted, colorReset)

			var response string
			fmt.Scanln(&response)
			if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
				fmt.Println()
				fmt.Printf("%sℹ%s Aborted. Use 'offgrid search --ram %.0f' to find suitable models\n", brandSecondary, colorReset, availableGB)
				fmt.Println()
				os.Exit(0)
			}
			fmt.Println()
		}
	}

	// Check if this is an embedding model (not designed for chat)
	isEmbeddingModel := strings.Contains(strings.ToLower(modelName), "minilm") ||
		strings.Contains(strings.ToLower(modelName), "e5-") ||
		strings.Contains(strings.ToLower(modelName), "bge-") ||
		strings.Contains(strings.ToLower(modelName), "gte-") ||
		strings.Contains(strings.ToLower(modelName), "embedding")

	if isEmbeddingModel {
		fmt.Println()
		printWarning("This appears to be an embedding model, not a chat model")
		fmt.Println()
		printInfo("Embedding models are designed for:")
		fmt.Println("  • Converting text to vectors")
		fmt.Println("  • Semantic search")
		fmt.Println("  • Text similarity")
		fmt.Println()
		printInfo("For chat/text generation, use a language model instead:")
		fmt.Println("  • tinyllama-1.1b-chat")
		fmt.Println("  • phi-2")
		fmt.Println("  • llama-2-7b-chat")
		fmt.Println("  • mistral-7b-instruct")
		fmt.Println()
		fmt.Printf("%sContinue anyway?%s (y/N): ", brandMuted, colorReset)

		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
			fmt.Println()
			printInfo("Aborted. Use 'offgrid list' to see available models")
			fmt.Println()
			os.Exit(0)
		}
		fmt.Println()
	}

	// Check if OffGrid API server is running and start it if needed
	if err := ensureOffgridServerRunning(); err != nil {
		fmt.Println()
		printWarning(fmt.Sprintf("OffGrid server not running: %v", err))
		printInfo("Starting OffGrid server in background...")
		if err := startOffgridServerInBackground(); err != nil {
			fmt.Println()
			printError("Failed to start OffGrid server")
			printInfo("Please start the server manually:")
			printItem("Start server", "offgrid serve &")
			fmt.Println()
			os.Exit(1)
		}
		// Wait for server to be ready
		time.Sleep(2 * time.Second)
	}

	// Check if llama-server is running and start it if needed
	if err := ensureLlamaServerRunning(); err != nil {
		fmt.Println()
		fmt.Printf("%sStarting llama-server%s\n", brandPrimary+colorBold, colorReset)
		fmt.Printf("%sModel:%s %s\n", colorDim, colorReset, filepath.Base(model.Path))
		fmt.Printf("%sFlags:%s --no-mmap --mlock --cont-batching (optimized)\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("%sKeep server running for instant responses:%s\n", colorDim, colorReset)
		fmt.Printf("  %ssudo systemctl enable --now llama-server%s\n", brandSecondary, colorReset)
		fmt.Println()
		if err := startLlamaServerInBackground(model.Path); err != nil {
			fmt.Println()
			printError("Failed to start llama-server")
			fmt.Println()
			fmt.Printf("%sℹ Start manually:%s\n", colorDim, colorReset)
			fmt.Printf("  %sllama-server -m %s --port 42382 &%s\n", brandSecondary, model.Path, colorReset)
			fmt.Println()
			os.Exit(1)
		}
		// Wait for llama-server to load the model
		fmt.Printf("%sLoading model...%s ", colorDim, colorReset)

		// Read llama-server port from config
		llamaPort := "42382"
		if portBytes, err := os.ReadFile("/etc/offgrid/llama-port"); err == nil {
			llamaPort = strings.TrimSpace(string(portBytes))
		}

		// Use the new robust readiness check
		if err := waitForModelReady(llamaPort, 120); err != nil {
			fmt.Printf("%s✗%s\n", brandError, colorReset)
			printError(fmt.Sprintf("Model failed to load: %v", err))
			os.Exit(1)
		}
		fmt.Printf("%s✓%s\n", brandSuccess, colorReset)
		fmt.Println()
	}

	// Setup session management
	homeDir, _ := os.UserHomeDir()
	sessionsDir := filepath.Join(homeDir, ".offgrid", "sessions")
	sessionMgr := sessions.NewSessionManager(sessionsDir)

	var currentSession *sessions.Session

	// Load existing session or create new one
	if loadSession {
		sess, err := sessionMgr.Load(sessionName)
		if err != nil {
			printError(fmt.Sprintf("Failed to load session '%s': %v", sessionName, err))
			fmt.Println()
			printInfo("Available sessions:")
			sessionList, _ := sessionMgr.List()
			for _, s := range sessionList {
				fmt.Printf("  • %s\n", s.Name)
			}
			fmt.Println()
			os.Exit(1)
		}
		currentSession = sess
		printSuccess(fmt.Sprintf("Loaded session '%s' (%d messages)", sessionName, sess.MessageCount()))
		fmt.Println()

		// Display previous conversation
		if sess.MessageCount() > 0 {
			printInfo("Previous conversation:")
			fmt.Println()
			for _, msg := range sess.Messages {
				if msg.Role == "user" {
					fmt.Printf("  %sYou:%s %s\n", brandPrimary, colorReset, msg.Content)
				} else {
					fmt.Printf("  %sAssistant:%s %s\n", brandSuccess, colorReset, msg.Content)
				}
			}
			fmt.Println()
		}
	} else if saveSession {
		currentSession = sessions.NewSession(sessionName, modelName)
		printInfo(fmt.Sprintf("Starting new session '%s' (will auto-save)", sessionName))
		fmt.Println()
	}

	fmt.Println()
	fmt.Printf("  %s%s Chat%s %s· %s%s\n", brandPrimary+colorBold, iconCircle, colorReset, brandMuted, modelName, colorReset)
	fmt.Println()
	if currentSession != nil {
		fmt.Printf("    %sSession:%s %s %s(auto-saving)%s\n", brandMuted, colorReset, currentSession.Name, brandMuted, colorReset)
	}
	if useKnowledgeBase {
		fmt.Printf("    %sRAG:%s Enabled\n", brandMuted, colorReset)
	}
	fmt.Printf("    %sCommands:%s %sexit%s %s·%s %sclear%s %s·%s %srag%s\n",
		brandMuted, colorReset,
		colorBold, colorReset, brandMuted, colorReset,
		colorBold, colorReset, brandMuted, colorReset,
		colorBold, colorReset)
	fmt.Println()

	// Start chat session
	spinner := NewSpinner("Connecting to server...")
	spinner.Start()

	// Import required packages for HTTP client
	client := &http.Client{
		Timeout: 300 * time.Second,
	}

	// Check if server is running
	healthURL := fmt.Sprintf("http://localhost:%d/health", cfg.ServerPort)
	resp, err := client.Get(healthURL)
	if err != nil {
		spinner.Stop(false)
		fmt.Println()
		printError("Server not running")
		fmt.Println()
		fmt.Printf("  %sStart server:%s\n", colorBold, colorReset)
		fmt.Printf("    %s$%s offgrid serve\n", brandMuted, colorReset)
		fmt.Printf("    %s$%s sudo systemctl start offgrid-llm\n", brandMuted, colorReset)
		fmt.Println()
		os.Exit(1)
	}
	resp.Body.Close()

	// Give server a moment to be fully ready (especially after model load)
	time.Sleep(1 * time.Second)

	spinner.Stop(true)

	// Refresh server's model list to pick up newly downloaded models
	refreshURL := fmt.Sprintf("http://localhost:%d/models/refresh", cfg.ServerPort)
	if refreshResp, err := client.Post(refreshURL, "application/json", nil); err == nil {
		refreshResp.Body.Close()
	}

	// Check which model is currently loaded in the server
	modelsURL := fmt.Sprintf("http://localhost:%d/v1/models", cfg.ServerPort)
	resp, err = client.Get(modelsURL)
	if err == nil {
		defer resp.Body.Close()
		var modelsResp struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err == nil {
			// Check if requested model is in the active models list
			foundActive := false
			var activeModel string
			for _, m := range modelsResp.Data {
				if m.ID == modelName {
					foundActive = true
					break
				}
				activeModel = m.ID
			}

			if !foundActive && len(modelsResp.Data) > 0 {
				fmt.Println()
				printInfo(fmt.Sprintf("Switching model: %s → %s", activeModel, modelName))
				fmt.Printf("%sLoading model...%s ", colorDim, colorReset)

				// Let the OffGrid server handle model switching by making a test request
				// The server's model cache will automatically load the new model
				testPayload := map[string]interface{}{
					"model": modelName,
					"messages": []map[string]string{
						{"role": "user", "content": "Hi"},
					},
					"max_tokens": 1,
					"stream":     false,
				}
				payloadBytes, _ := json.Marshal(testPayload)

				// Give the server time to switch models (may take a while for large models)
				switchClient := &http.Client{
					Timeout: 180 * time.Second, // 3 minute timeout for model loading
				}

				switchResp, err := switchClient.Post(
					fmt.Sprintf("http://localhost:%d/v1/chat/completions", cfg.ServerPort),
					"application/json",
					bytes.NewReader(payloadBytes),
				)

				if err != nil {
					fmt.Printf("%s✗%s\n", brandError, colorReset)
					printError(fmt.Sprintf("Failed to switch model: %v", err))
					os.Exit(1)
				}
				switchResp.Body.Close()

				if switchResp.StatusCode != http.StatusOK {
					fmt.Printf("%s✗%s\n", brandError, colorReset)
					printError(fmt.Sprintf("Failed to switch model (status %d)", switchResp.StatusCode))
					os.Exit(1)
				}

				fmt.Printf("%s✓%s\n", brandSuccess, colorReset)
				fmt.Println()
			}
		}
	}

	fmt.Println()

	// Interactive chat loop
	messages := []ChatMessage{}

	// Load messages from session if continuing
	if currentSession != nil && currentSession.MessageCount() > 0 {
		for _, msg := range currentSession.Messages {
			messages = append(messages, ChatMessage{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
	}

	// Handle initial image if provided
	if imagePath != "" {
		// Verify image file exists and is readable
		_, err := os.ReadFile(imagePath)
		if err != nil {
			printError(fmt.Sprintf("Failed to read image file: %v", err))
			os.Exit(1)
		}

		fmt.Printf("%sImage attached:%s %s\n", brandSuccess, colorReset, filepath.Base(imagePath))
		fmt.Println()

		// We'll attach this to the first user message
	}

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("\n  %s%s%s ", brandPrimary+colorBold, iconChevron, colorReset)
		input, err := reader.ReadString('\n')
		if err != nil {
			break
		}

		input = strings.TrimSpace(input)

		if input == "" {
			continue
		}

		if input == "exit" || input == "quit" {
			fmt.Println()
			fmt.Printf("  %sSession ended%s\n\n", brandMuted, colorReset)
			break
		}

		if input == "clear" {
			messages = []ChatMessage{}
			// Clear session too if active
			if currentSession != nil {
				currentSession.Messages = []sessions.Message{}
				if err := sessionMgr.Save(currentSession); err != nil {
					fmt.Printf("  %sWarning: Failed to save cleared session: %v%s\n", colorYellow, err, colorReset)
				}
			}
			// Clear image path so it doesn't get re-attached
			imagePath = ""
			fmt.Printf("\n  %s%s Conversation cleared%s\n", brandMuted, iconCheck, colorReset)
			continue
		}

		if input == "rag" || input == "/rag" {
			useKnowledgeBase = !useKnowledgeBase
			if useKnowledgeBase {
				fmt.Printf("\n  %s%s Knowledge Base enabled%s\n", brandSuccess, iconCheck, colorReset)
			} else {
				fmt.Printf("\n  %s○ Knowledge Base disabled%s\n", brandMuted, colorReset)
			}
			continue
		}

		// Add user message
		var userContent interface{} = input

		// If we have an image pending, attach it to this message
		if imagePath != "" {
			// Read image file again (or we could have cached the dataURI)
			imageData, err := os.ReadFile(imagePath)
			if err == nil {
				base64Image := base64.StdEncoding.EncodeToString(imageData)
				mimeType := "image/jpeg"
				if strings.HasSuffix(strings.ToLower(imagePath), ".png") {
					mimeType = "image/png"
				} else if strings.HasSuffix(strings.ToLower(imagePath), ".webp") {
					mimeType = "image/webp"
				} else if strings.HasSuffix(strings.ToLower(imagePath), ".gif") {
					mimeType = "image/gif"
				}

				dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Image)

				// Create multimodal content
				userContent = []map[string]interface{}{
					{
						"type": "text",
						"text": input,
					},
					{
						"type": "image_url",
						"image_url": map[string]string{
							"url": dataURI,
						},
					},
				}

				// Clear image path so it's only sent once
				imagePath = ""
			} else {
				printError(fmt.Sprintf("Failed to read image file: %v", err))
			}
		}

		messages = append(messages, ChatMessage{
			Role:    "user",
			Content: userContent,
		})

		// Save user message to session (store as text for simplicity in session file for now)
		if currentSession != nil {
			currentSession.AddMessage("user", input)
		}

		// Make API request
		reqBody := ChatCompletionRequest{
			Model:            modelName,
			Messages:         messages,
			Stream:           true,
			UseKnowledgeBase: useKnowledgeBase,
		}

		jsonData, _ := json.Marshal(reqBody)
		apiURL := fmt.Sprintf("http://localhost:%d/v1/chat/completions", cfg.ServerPort)

		req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
		if err != nil {
			fmt.Println()
			fmt.Fprintf(os.Stderr, "✗ Error creating request: %v\n", err)
			fmt.Println()
			continue
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			fmt.Println()
			fmt.Fprintf(os.Stderr, "✗ Error: %v\n", err)
			fmt.Println()
			continue
		}

		// Handle streaming response
		fmt.Print("\n")
		scanner := bufio.NewScanner(resp.Body)
		var assistantMsg strings.Builder
		lineLength := 0

		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}

			var chunk ChatCompletionChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			if len(chunk.Choices) > 0 {
				tokenInterface := chunk.Choices[0].Delta.Content
				var token string
				if str, ok := tokenInterface.(string); ok {
					token = str
				}
				fmt.Print(token)
				os.Stdout.Sync() // Flush output immediately for streaming
				assistantMsg.WriteString(token)
				lineLength += len(token)

				// Wrap long lines
				if lineLength > 80 && strings.Contains(token, " ") {
					lineLength = 0
				}
			}
		}

		fmt.Println()
		fmt.Println()

		resp.Body.Close()

		// Add assistant response to history
		messages = append(messages, ChatMessage{
			Role:    "assistant",
			Content: assistantMsg.String(),
		})

		// Save assistant message to session
		if currentSession != nil {
			currentSession.AddMessage("assistant", assistantMsg.String())
			// Auto-save after each exchange
			if err := sessionMgr.Save(currentSession); err != nil {
				// Don't interrupt the conversation, just log the error
				fmt.Printf("%s⚠ Failed to save session: %v%s\n", brandMuted, err, colorReset)
			}
		}
	}

	// Save session one final time on exit
	if currentSession != nil && (saveSession || loadSession) {
		if err := sessionMgr.Save(currentSession); err != nil {
			printWarning(fmt.Sprintf("Failed to save session: %v", err))
		} else {
			printSuccess(fmt.Sprintf("Session '%s' saved (%d messages)", currentSession.Name, currentSession.MessageCount()))
		}
		fmt.Println()
	}
}

// Helper types for handleRun
type ChatMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

func (m ChatMessage) StringContent() string {
	if m.Content == nil {
		return ""
	}
	if str, ok := m.Content.(string); ok {
		return str
	}
	if parts, ok := m.Content.([]interface{}); ok {
		var text string
		for _, part := range parts {
			if p, ok := part.(map[string]interface{}); ok {
				if t, ok := p["type"].(string); ok && t == "text" {
					if val, ok := p["text"].(string); ok {
						text += val
					}
				}
			}
		}
		return text
	}
	return ""
}

type ChatCompletionRequest struct {
	Model            string        `json:"model"`
	Messages         []ChatMessage `json:"messages"`
	Stream           bool          `json:"stream"`
	UseKnowledgeBase bool          `json:"use_knowledge_base,omitempty"`
}

type ChatCompletionChunk struct {
	Choices []struct {
		Delta struct {
			Content interface{} `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

func formatNumber(n int64) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	} else if n >= 1000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

// estimateRAMFromModel estimates RAM needed based on model name and quantization
func estimateRAMFromModel(modelID string, quant string) float64 {
	// Parse parameter size from model name (1B, 3B, 7B, 13B, etc.)
	var paramSize float64
	modelLower := strings.ToLower(modelID)

	// Common parameter sizes
	if strings.Contains(modelLower, "1b") || strings.Contains(modelLower, "1.1b") {
		paramSize = 1
	} else if strings.Contains(modelLower, "3b") || strings.Contains(modelLower, "3.2b") {
		paramSize = 3
	} else if strings.Contains(modelLower, "7b") {
		paramSize = 7
	} else if strings.Contains(modelLower, "8b") {
		paramSize = 8
	} else if strings.Contains(modelLower, "13b") {
		paramSize = 13
	} else if strings.Contains(modelLower, "70b") {
		paramSize = 70
	} else {
		return 0 // Can't estimate
	}

	// Estimate based on quantization (rough approximations)
	var bytesPerParam float64
	quantUpper := strings.ToUpper(quant)
	switch {
	case strings.Contains(quantUpper, "Q2"):
		bytesPerParam = 0.3 // ~2 bits
	case strings.Contains(quantUpper, "Q3"):
		bytesPerParam = 0.4 // ~3 bits
	case strings.Contains(quantUpper, "Q4"):
		bytesPerParam = 0.5 // ~4 bits
	case strings.Contains(quantUpper, "Q5"):
		bytesPerParam = 0.65 // ~5 bits
	case strings.Contains(quantUpper, "Q6"):
		bytesPerParam = 0.75 // ~6 bits
	case strings.Contains(quantUpper, "Q8"):
		bytesPerParam = 1.0 // ~8 bits
	default:
		bytesPerParam = 0.5 // Default to Q4
	}

	// Model size in GB = params (in billions) * bytes per param
	modelSizeGB := paramSize * bytesPerParam

	// RAM needed = model size * 1.3 (overhead for context, KV cache, etc.)
	return modelSizeGB * 1.3
}

type diskSpaceInfo struct {
	Total       int64
	Available   int64
	UsedPercent float64
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func formatSizeGB(gb float64) string {
	if gb < 1.0 {
		return fmt.Sprintf("%.0f MB", gb*1024)
	}
	return fmt.Sprintf("%.1f GB", gb)
}

// wrapText wraps text to a specified width
func wrapText(text string, width int) string {
	if len(text) <= width {
		return text
	}
	var lines []string
	words := strings.Fields(text)
	var line string
	for _, word := range words {
		if len(line)+len(word)+1 > width {
			if line != "" {
				lines = append(lines, line)
			}
			line = word
		} else {
			if line != "" {
				line += " "
			}
			line += word
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// formatDuration formats a duration in human-readable form
func formatDuration(d time.Duration) string {
	if d < time.Microsecond {
		return fmt.Sprintf("%dns", d.Nanoseconds())
	} else if d < time.Millisecond {
		return fmt.Sprintf("%.2fµs", float64(d.Microseconds()))
	} else if d < time.Second {
		return fmt.Sprintf("%.2fms", float64(d.Milliseconds()))
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}

// formatModelSize formats model size in GB with appropriate precision
func formatModelSize(sizeGB float64) string {
	if sizeGB < 0.1 {
		// For sizes less than 0.1 GB, show in MB
		return fmt.Sprintf("%d MB", int(sizeGB*1024))
	} else if sizeGB < 1.0 {
		// For sizes less than 1 GB, use 2 decimal places
		return fmt.Sprintf("%.2f GB", sizeGB)
	}
	// For larger sizes, use 1 decimal place
	return fmt.Sprintf("%.1f GB", sizeGB)
}

// isServerHealthy checks if the server is responding
func isServerHealthy(baseURL string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/health", nil)
	if err != nil {
		return false
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// handleAlias manages model aliases
func handleAlias(args []string) {
	// Check for help flag
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		args = []string{} // Trigger help display
	}

	if len(args) == 0 {
		fmt.Println()
		fmt.Printf("  %s◈ Aliases%s\n", brandPrimary+colorBold, colorReset)
		fmt.Printf("  %sShortcut names for models%s\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sUsage%s  offgrid alias <command>\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sCommands%s\n", brandPrimary, colorReset)
		fmt.Printf("    %-22s %sList all aliases%s\n", "list", colorDim, colorReset)
		fmt.Printf("    %-22s %sCreate an alias%s\n", "set <alias> <model>", colorDim, colorReset)
		fmt.Printf("    %-22s %sRemove an alias%s\n", "remove <alias>", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sExamples%s\n", brandPrimary, colorReset)
		fmt.Printf("    %s$%s offgrid alias set llama tinyllama-1.1b-chat.Q4_K_M\n", colorDim, colorReset)
		fmt.Printf("    %s$%s offgrid run llama  %s# uses the alias%s\n", colorDim, colorReset, colorDim, colorReset)
		fmt.Println()
		return
	}

	configDir := "/var/lib/offgrid"
	if home, err := os.UserHomeDir(); err == nil {
		configDir = filepath.Join(home, ".offgrid")
	}

	am := models.NewAliasManager(configDir)

	switch args[0] {
	case "list", "ls":
		aliases := am.ListAliases()
		if len(aliases) == 0 {
			fmt.Println()
			fmt.Printf("%sℹ No aliases defined%s\n", colorDim, colorReset)
			fmt.Println()
			return
		}

		fmt.Println()
		fmt.Printf("%sAliases%s\n", brandPrimary+colorBold, colorReset)
		for alias, modelID := range aliases {
			fmt.Printf("%s%-20s%s %s·%s %s\n", brandSecondary, alias, colorReset, brandMuted, colorReset, modelID)
		}
		fmt.Println()

	case "set", "create", "add":
		if len(args) < 3 {
			printError("Usage: offgrid alias set <alias> <model>")
			return
		}

		alias := args[1]
		modelID := args[2]

		if err := am.SetAlias(alias, modelID); err != nil {
			printError(fmt.Sprintf("Failed to set alias: %v", err))
			return
		}

		printSuccess(fmt.Sprintf("Alias '%s' created for '%s'", alias, modelID))

	case "remove", "rm", "delete":
		if len(args) < 2 {
			printError("Usage: offgrid alias remove <alias>")
			return
		}

		alias := args[1]
		if err := am.RemoveAlias(alias); err != nil {
			printError(fmt.Sprintf("Failed to remove alias: %v", err))
			return
		}

		printSuccess(fmt.Sprintf("Alias '%s' removed", alias))

	default:
		printError(fmt.Sprintf("Unknown alias command: %s", args[0]))
	}
}

// handleFavorite manages favorite models
func handleFavorite(args []string) {
	// Check for help flag
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		args = []string{} // Trigger help display
	}

	if len(args) == 0 {
		fmt.Println()
		fmt.Printf("  %s◈ Favorites%s\n", brandPrimary+colorBold, colorReset)
		fmt.Printf("  %sBookmark frequently used models%s\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sUsage%s  offgrid favorite <command>\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sCommands%s\n", brandPrimary, colorReset)
		fmt.Printf("    %-20s %sList favorites%s\n", "list", colorDim, colorReset)
		fmt.Printf("    %-20s %sAdd to favorites%s\n", "add <model>", colorDim, colorReset)
		fmt.Printf("    %-20s %sRemove from favorites%s\n", "remove <model>", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sExamples%s\n", brandPrimary, colorReset)
		fmt.Printf("    %s$%s offgrid favorite add llama-3.2-3b\n", colorDim, colorReset)
		fmt.Printf("    %s$%s offgrid favorite list\n", colorDim, colorReset)
		fmt.Println()
		return
	}

	configDir := "/var/lib/offgrid"
	if home, err := os.UserHomeDir(); err == nil {
		configDir = filepath.Join(home, ".offgrid")
	}

	am := models.NewAliasManager(configDir)

	switch args[0] {
	case "list", "ls":
		favorites := am.ListFavorites()
		if len(favorites) == 0 {
			fmt.Println()
			fmt.Printf("%sℹ No favorite models%s\n", colorDim, colorReset)
			fmt.Println()
			return
		}

		fmt.Println()
		fmt.Printf("%sFavorites%s\n", brandPrimary+colorBold, colorReset)
		for _, modelID := range favorites {
			fmt.Printf("%s★%s %s\n", brandSuccess, colorReset, modelID)
		}
		fmt.Println()

	case "add", "set":
		if len(args) < 2 {
			printError("Usage: offgrid favorite add <model>")
			return
		}

		modelID := args[1]
		if err := am.SetFavorite(modelID, true); err != nil {
			printError(fmt.Sprintf("Failed to add favorite: %v", err))
			return
		}

		printSuccess(fmt.Sprintf("'%s' added to favorites", modelID))

	case "remove", "rm", "delete":
		if len(args) < 2 {
			printError("Usage: offgrid favorite remove <model>")
			return
		}

		modelID := args[1]
		if err := am.SetFavorite(modelID, false); err != nil {
			printError(fmt.Sprintf("Failed to remove favorite: %v", err))
			return
		}

		printSuccess(fmt.Sprintf("'%s' removed from favorites", modelID))

	default:
		printError(fmt.Sprintf("Unknown favorite command: %s", args[0]))
	}
}

// handleTemplate manages prompt templates
func handleTemplate(args []string) {
	// Check for help flag
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		args = []string{} // Trigger help display
	}

	if len(args) == 0 {
		fmt.Println()
		fmt.Printf("  %s◈ Templates%s\n", brandPrimary+colorBold, colorReset)
		fmt.Printf("  %sReusable prompt templates%s\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sUsage%s  offgrid template <command>\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sCommands%s\n", brandPrimary, colorReset)
		fmt.Printf("    %-20s %sList all templates%s\n", "list", colorDim, colorReset)
		fmt.Printf("    %-20s %sShow template details%s\n", "show <name>", colorDim, colorReset)
		fmt.Printf("    %-20s %sApply interactively%s\n", "apply <name>", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sExamples%s\n", brandPrimary, colorReset)
		fmt.Printf("    %s$%s offgrid template list\n", colorDim, colorReset)
		fmt.Printf("    %s$%s offgrid template show code-review\n", colorDim, colorReset)
		fmt.Println()
		return
	}

	switch args[0] {
	case "list", "ls":
		fmt.Println()
		fmt.Printf("%sTemplates%s\n", brandPrimary+colorBold, colorReset)
		templateList := templates.ListTemplates()
		for _, name := range templateList {
			tpl, _ := templates.GetTemplate(name)
			fmt.Printf("%s%-20s%s %s\n", brandSecondary, name, colorReset, tpl.Description)
		}
		fmt.Println()

	case "show", "info":
		if len(args) < 2 {
			printError("Usage: offgrid template show <name>")
			return
		}

		tpl, err := templates.GetTemplate(args[1])
		if err != nil {
			printError(fmt.Sprintf("Template not found: %s", args[1]))
			return
		}

		fmt.Println()
		printBox(tpl.Name, fmt.Sprintf("%s\n\n%sVariables:%s %s",
			tpl.Description,
			colorBold, colorReset,
			strings.Join(tpl.Variables, ", ")))
		fmt.Println()
		fmt.Println(colorDim + "Template:" + colorReset)
		fmt.Println(tpl.Template)
		fmt.Println()

		if len(tpl.Examples) > 0 {
			fmt.Println(colorDim + "Examples:" + colorReset)
			for key, value := range tpl.Examples {
				printItem(key, value)
			}
			fmt.Println()
		}

	case "apply", "use":
		if len(args) < 2 {
			printError("Usage: offgrid template apply <name>")
			return
		}

		tpl, err := templates.GetTemplate(args[1])
		if err != nil {
			printError(fmt.Sprintf("Template not found: %s", args[1]))
			return
		}

		fmt.Println()
		fmt.Printf("%sTemplate:%s %s - %s\n\n", colorBold, colorReset, tpl.Name, tpl.Description)

		// Collect variables interactively
		variables := make(map[string]string)
		scanner := bufio.NewScanner(os.Stdin)

		for _, varName := range tpl.Variables {
			fmt.Printf("%s%s:%s ", brandPrimary, varName, colorReset)
			if example, ok := tpl.Examples[varName]; ok {
				fmt.Printf("%s(%s)%s\n> ", brandMuted, example, colorReset)
			} else {
				fmt.Print("\n> ")
			}

			scanner.Scan()
			value := scanner.Text()
			if value != "" {
				variables[varName] = value
			}
		}

		prompt, err := tpl.Apply(variables)
		if err != nil {
			printError(fmt.Sprintf("Failed to apply template: %v", err))
			return
		}

		fmt.Println()
		fmt.Printf("%sGenerated Prompt%s\n", brandPrimary+colorBold, colorReset)
		fmt.Println("")
		fmt.Println(prompt)
		fmt.Println()
		fmt.Printf("%s%s\n", brandPrimary, colorReset)
		fmt.Println()

	default:
		printError(fmt.Sprintf("Unknown template command: %s", args[0]))
	}
}

// handleBatch processes requests in batch mode
func handleBatch(args []string) {
	// Check for help flag
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		args = []string{} // Trigger help display
	}

	if len(args) == 0 {
		fmt.Println()
		fmt.Printf("  %s◈ Batch Processing%s\n", brandPrimary+colorBold, colorReset)
		fmt.Printf("  %sProcess multiple prompts from JSONL files%s\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sUsage%s  offgrid batch process <input.jsonl> [output.jsonl]\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sOptions%s\n", brandPrimary, colorReset)
		fmt.Printf("    %-20s %sParallel requests (default: 4)%s\n", "--concurrency <n>", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sInput Format%s  JSONL with id, model, prompt fields\n", colorDim, colorReset)
		fmt.Printf("    %s{\"id\": \"1\", \"model\": \"llama3\", \"prompt\": \"Hello\"}%s\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sExamples%s\n", brandPrimary, colorReset)
		fmt.Printf("    %s$%s offgrid batch process prompts.jsonl\n", colorDim, colorReset)
		fmt.Printf("    %s$%s offgrid batch process in.jsonl out.jsonl --concurrency 8\n", colorDim, colorReset)
		fmt.Println()
		return
	}

	if args[0] != "process" {
		printError("Usage: offgrid batch process <input.jsonl>")
		fmt.Printf("  %sSee:%s offgrid batch --help\n", brandMuted, colorReset)
		return
	}

	if len(args) < 2 {
		printError("Usage: offgrid batch process <input.jsonl> [output.jsonl]")
		return
	}

	inputPath := args[1]
	outputPath := "batch-results.jsonl"
	concurrency := 4

	// Parse remaining args
	for i := 2; i < len(args); i++ {
		if args[i] == "--concurrency" || args[i] == "-c" {
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &concurrency)
				i++
			}
		} else if !strings.HasPrefix(args[i], "--") {
			outputPath = args[i]
		}
	}

	printInfo(fmt.Sprintf("Processing: %s to %s (concurrency=%d)", inputPath, outputPath, concurrency))
	fmt.Println()

	// Load config
	configPath := os.Getenv("OFFGRID_CONFIG")
	_, err := config.LoadWithPriority(configPath)
	if err != nil {
		printError(fmt.Sprintf("Failed to load config: %v", err))
		return
	}

	// Create inference engine
	engine := inference.NewMockEngine()

	// Create batch processor
	processor := batch.NewProcessor(engine, concurrency)

	// Process file
	ctx := context.Background()
	if err := processor.ProcessFile(ctx, inputPath, outputPath); err != nil {
		printError(fmt.Sprintf("Batch processing failed: %v", err))
		return
	}

	printSuccess(fmt.Sprintf("Results written to: %s", outputPath))
}

// handleSession handles session commands
func handleSession(args []string) {
	homeDir, _ := os.UserHomeDir()
	sessionsDir := filepath.Join(homeDir, ".offgrid", "sessions")
	sessionMgr := sessions.NewSessionManager(sessionsDir)

	// Check for help flag
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		args = []string{} // Trigger help display
	}

	if len(args) == 0 {
		fmt.Println()
		fmt.Printf("  %s◈ Sessions%s\n", brandPrimary+colorBold, colorReset)
		fmt.Printf("  %sPersistent chat history management%s\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sUsage%s  offgrid session <command>\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sCommands%s\n", brandPrimary, colorReset)
		fmt.Printf("    %-22s %sList all sessions%s\n", "list", colorDim, colorReset)
		fmt.Printf("    %-22s %sShow session details%s\n", "show <name>", colorDim, colorReset)
		fmt.Printf("    %-22s %sDelete a session%s\n", "delete <name>", colorDim, colorReset)
		fmt.Printf("    %-22s %sExport to markdown%s\n", "export <name> [file]", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sExamples%s\n", brandPrimary, colorReset)
		fmt.Printf("    %s$%s offgrid session list\n", colorDim, colorReset)
		fmt.Printf("    %s$%s offgrid session show my-project\n", colorDim, colorReset)
		fmt.Printf("    %s$%s offgrid session export my-project notes.md\n", colorDim, colorReset)
		fmt.Println()
		return
	}

	subcommand := args[0]

	switch subcommand {
	case "list", "ls":
		handleSessionList(sessionMgr)
	case "show", "view":
		if len(args) < 2 {
			printError("Usage: offgrid session show <name>")
			return
		}
		handleSessionShow(sessionMgr, args[1])
	case "delete", "del", "rm":
		if len(args) < 2 {
			printError("Usage: offgrid session delete <name>")
			return
		}
		handleSessionDelete(sessionMgr, args[1])
	case "export":
		if len(args) < 2 {
			printError("Usage: offgrid session export <name> [output.md]")
			return
		}
		outputPath := ""
		if len(args) >= 3 {
			outputPath = args[2]
		}
		handleSessionExport(sessionMgr, args[1], outputPath)
	default:
		printError(fmt.Sprintf("Unknown subcommand: %s", subcommand))
		fmt.Println("Available subcommands: list, show, delete, export")
	}
}

func handleSessionList(sessionMgr *sessions.SessionManager) {
	sessionList, err := sessionMgr.List()
	if err != nil {
		if output.JSONMode {
			output.Error("Failed to list sessions", err)
		}
		printError(fmt.Sprintf("Failed to list sessions: %v", err))
		return
	}

	// JSON output mode
	if output.JSONMode {
		var jsonSessions []output.SessionInfo
		for _, sess := range sessionList {
			jsonSessions = append(jsonSessions, output.SessionInfo{
				Name:      sess.Name,
				ModelID:   sess.ModelID,
				Messages:  sess.MessageCount(),
				CreatedAt: sess.CreatedAt.Format(time.RFC3339),
				UpdatedAt: sess.UpdatedAt.Format(time.RFC3339),
			})
		}
		output.PrintSessions(jsonSessions)
		return
	}

	// Human-readable output
	if len(sessionList) == 0 {
		printInfo("No saved sessions found")
		fmt.Println()
		printInfo("Sessions are automatically saved when using the chat command with --save flag")
		return
	}

	fmt.Printf("Found %d session(s):\n\n", len(sessionList))

	for i, sess := range sessionList {
		fmt.Printf("  %d. %s%s%s\n", i+1, brandPrimary, sess.Name, colorReset)
		fmt.Printf("     Model: %s · Messages: %d · Updated: %s\n",
			sess.ModelID, sess.MessageCount(), formatTimeAgo(sess.UpdatedAt))
		if i < len(sessionList)-1 {
			fmt.Println()
		}
	}
	fmt.Println()
}

func handleSessionShow(sessionMgr *sessions.SessionManager, name string) {
	sess, err := sessionMgr.Load(name)
	if err != nil {
		printError(fmt.Sprintf("Failed to load session: %v", err))
		return
	}

	fmt.Printf("%s%s%s\n", colorBold, sess.Name, colorReset)
	fmt.Printf("Model: %s\n", sess.ModelID)
	fmt.Printf("Created: %s\n", sess.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Updated: %s (%s)\n", sess.UpdatedAt.Format("2006-01-02 15:04:05"), formatTimeAgo(sess.UpdatedAt))
	fmt.Printf("Messages: %d\n", sess.MessageCount())
	fmt.Println()

	for i, msg := range sess.Messages {
		if msg.Role == "user" {
			fmt.Printf("%s● User%s (%s)\n", brandPrimary, colorReset, msg.Timestamp.Format("15:04:05"))
		} else {
			fmt.Printf("%s● Assistant%s (%s)\n", brandSuccess, colorReset, msg.Timestamp.Format("15:04:05"))
		}
		fmt.Println(msg.Content)
		if i < len(sess.Messages)-1 {
			fmt.Println()
		}
	}
	fmt.Println()
}

func handleSessionDelete(sessionMgr *sessions.SessionManager, name string) {
	if err := sessionMgr.Delete(name); err != nil {
		printError(fmt.Sprintf("Failed to delete session: %v", err))
		return
	}

	printSuccess(fmt.Sprintf("Deleted session: %s", name))
}

func handleSessionExport(sessionMgr *sessions.SessionManager, name string, outputPath string) {
	sess, err := sessionMgr.Load(name)
	if err != nil {
		printError(fmt.Sprintf("Failed to load session: %v", err))
		return
	}

	markdown := sess.ExportMarkdown()

	if outputPath == "" {
		outputPath = sess.Name + ".md"
	}

	if err := os.WriteFile(outputPath, []byte(markdown), 0644); err != nil {
		printError(fmt.Sprintf("Failed to write file: %v", err))
		return
	}

	printSuccess(fmt.Sprintf("Exported to: %s", outputPath))
}

// handleKnowledgeBase handles the kb/knowledge/rag command for managing the knowledge base
func handleKnowledgeBase(args []string) {
	// Check for help flag
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		args = []string{} // Trigger help display
	}

	if len(args) == 0 {
		fmt.Println()
		fmt.Printf("  %s◈ Knowledge Base%s\n", brandPrimary+colorBold, colorReset)
		fmt.Printf("  %sRAG document management%s\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sUsage%s  offgrid kb <command>\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sCommands%s\n", brandPrimary, colorReset)
		fmt.Printf("    %-20s %sShow KB status%s\n", "status", colorDim, colorReset)
		fmt.Printf("    %-20s %sList all documents%s\n", "list", colorDim, colorReset)
		fmt.Printf("    %-20s %sAdd a document%s\n", "add <file>", colorDim, colorReset)
		fmt.Printf("    %-20s %sRemove by ID%s\n", "remove <id>", colorDim, colorReset)
		fmt.Printf("    %-20s %sSearch documents%s\n", "search <query>", colorDim, colorReset)
		fmt.Printf("    %-20s %sClear all documents%s\n", "clear", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sSupported%s  .txt .md .json .csv .html\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sExamples%s\n", brandPrimary, colorReset)
		fmt.Printf("    %s$%s offgrid kb add ./docs/manual.md\n", colorDim, colorReset)
		fmt.Printf("    %s$%s offgrid kb search \"how to configure\"\n", colorDim, colorReset)
		fmt.Println()
		return
	}

	subcommand := args[0]

	switch subcommand {
	case "status":
		handleKBStatus()
	case "list", "ls":
		handleKBList()
	case "add":
		if len(args) < 2 {
			printError("Usage: offgrid kb add <file>")
			return
		}
		handleKBAdd(args[1])
	case "remove", "rm", "delete", "del":
		if len(args) < 2 {
			printError("Usage: offgrid kb remove <id>")
			return
		}
		handleKBRemove(args[1])
	case "search":
		if len(args) < 2 {
			printError("Usage: offgrid kb search <query>")
			return
		}
		query := strings.Join(args[1:], " ")
		handleKBSearch(query)
	case "clear":
		handleKBClear()
	default:
		printError(fmt.Sprintf("Unknown subcommand: %s", subcommand))
		fmt.Println("Available subcommands: status, list, add, remove, search, clear")
	}
}

// getServerPort returns the configured server port
func getServerPort() int {
	cfg := config.LoadConfig()
	return cfg.ServerPort
}

func handleKBStatus() {
	port := getServerPort()
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/v1/rag/status", port))
	if err != nil {
		printError(fmt.Sprintf("Failed to connect to server: %v", err))
		fmt.Println("Make sure the server is running with: offgrid serve")
		return
	}
	defer resp.Body.Close()

	var status map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		printError(fmt.Sprintf("Failed to parse response: %v", err))
		return
	}

	fmt.Println()
	fmt.Printf("%sKnowledge Base Status%s\n", brandPrimary+colorBold, colorReset)
	fmt.Println()

	enabled, _ := status["enabled"].(bool)
	if enabled {
		fmt.Printf("   %sStatus:%s      %sEnabled%s\n", colorDim, colorReset, colorGreen, colorReset)
	} else {
		fmt.Printf("   %sStatus:%s      %sDisabled%s\n", colorDim, colorReset, colorYellow, colorReset)
	}

	if model, ok := status["model"].(string); ok && model != "" {
		fmt.Printf("   %sModel:%s       %s\n", colorDim, colorReset, model)
	}

	if docCount, ok := status["document_count"].(float64); ok {
		fmt.Printf("   %sDocuments:%s   %.0f\n", colorDim, colorReset, docCount)
	}

	if chunkCount, ok := status["chunk_count"].(float64); ok {
		fmt.Printf("   %sChunks:%s      %.0f\n", colorDim, colorReset, chunkCount)
	}

	if persistPath, ok := status["persist_path"].(string); ok && persistPath != "" {
		fmt.Printf("   %sData Path:%s   %s\n", colorDim, colorReset, persistPath)
	}

	fmt.Println()
}

func handleKBList() {
	port := getServerPort()
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/v1/rag/status", port))
	if err != nil {
		printError(fmt.Sprintf("Failed to connect to server: %v", err))
		fmt.Println("Make sure the server is running with: offgrid serve")
		return
	}
	defer resp.Body.Close()

	var status map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		printError(fmt.Sprintf("Failed to parse response: %v", err))
		return
	}

	enabled, _ := status["enabled"].(bool)
	if !enabled {
		printError("RAG is not enabled. Enable it first via the web UI or API.")
		return
	}

	docs, ok := status["documents"].([]interface{})
	if !ok || len(docs) == 0 {
		fmt.Println()
		fmt.Printf("%sKnowledge Base Documents%s\n", brandPrimary+colorBold, colorReset)
		fmt.Println()
		fmt.Printf("   %sNo documents in knowledge base%s\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("   Add documents with: %soffgrid kb add <file>%s\n", brandSecondary, colorReset)
		fmt.Println()
		return
	}

	fmt.Println()
	fmt.Printf("%sKnowledge Base Documents%s (%d total)\n", brandPrimary+colorBold, colorReset, len(docs))
	fmt.Println()

	for _, d := range docs {
		doc, ok := d.(map[string]interface{})
		if !ok {
			continue
		}

		id, _ := doc["id"].(string)
		name, _ := doc["name"].(string)
		chunkCount, _ := doc["chunk_count"].(float64)
		size, _ := doc["size"].(float64)

		shortID := id
		if len(id) > 8 {
			shortID = id[:8]
		}

		fmt.Printf("   %s%s%s  %-40s  %s%d chunks%s  %s\n",
			brandSecondary, shortID, colorReset,
			name,
			colorDim, int(chunkCount), colorReset,
			formatBytes(int64(size)))
	}
	fmt.Println()
}

func handleKBAdd(filePath string) {
	port := getServerPort()
	// Check if file exists
	info, err := os.Stat(filePath)
	if err != nil {
		printError(fmt.Sprintf("File not found: %s", filePath))
		return
	}

	if info.IsDir() {
		printError("Cannot add directory. Please specify a file.")
		return
	}

	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		printError(fmt.Sprintf("Failed to read file: %v", err))
		return
	}

	// Prepare request
	reqBody := map[string]interface{}{
		"name":    filepath.Base(filePath),
		"content": string(content),
	}

	jsonData, _ := json.Marshal(reqBody)

	resp, err := http.Post(
		fmt.Sprintf("http://localhost:%d/v1/documents/ingest", port),
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		printError(fmt.Sprintf("Failed to connect to server: %v", err))
		fmt.Println("Make sure the server is running with: offgrid serve")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		printError(fmt.Sprintf("Failed to add document: %s", string(body)))
		return
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if doc, ok := result["document"].(map[string]interface{}); ok {
		name, _ := doc["name"].(string)
		chunkCount, _ := doc["chunk_count"].(float64)
		printSuccess(fmt.Sprintf("Added '%s' (%d chunks)", name, int(chunkCount)))
	} else {
		printSuccess(fmt.Sprintf("Added '%s'", filepath.Base(filePath)))
	}
}

func handleKBRemove(id string) {
	port := getServerPort()
	req, _ := http.NewRequest(http.MethodDelete,
		fmt.Sprintf("http://localhost:%d/v1/documents/delete?id=%s", port, id), nil)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		printError(fmt.Sprintf("Failed to connect to server: %v", err))
		fmt.Println("Make sure the server is running with: offgrid serve")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		printError(fmt.Sprintf("Document not found: %s", id))
		return
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		printError(fmt.Sprintf("Failed to remove document: %s", string(body)))
		return
	}

	printSuccess(fmt.Sprintf("Removed document: %s", id))
}

func handleKBSearch(query string) {
	port := getServerPort()
	reqBody := map[string]interface{}{
		"query": query,
		"top_k": 5,
	}

	jsonData, _ := json.Marshal(reqBody)

	resp, err := http.Post(
		fmt.Sprintf("http://localhost:%d/v1/documents/search", port),
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		printError(fmt.Sprintf("Failed to connect to server: %v", err))
		fmt.Println("Make sure the server is running with: offgrid serve")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		printError(fmt.Sprintf("Search failed: %s", string(body)))
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		printError(fmt.Sprintf("Failed to parse response: %v", err))
		return
	}

	results, ok := result["results"].([]interface{})
	if !ok || len(results) == 0 {
		fmt.Println()
		fmt.Printf("%sSearch Results%s\n", brandPrimary+colorBold, colorReset)
		fmt.Println()
		fmt.Printf("   %sNo results found for:%s \"%s\"\n", colorDim, colorReset, query)
		fmt.Println()
		return
	}

	fmt.Println()
	fmt.Printf("%sSearch Results%s for \"%s\"\n", brandPrimary+colorBold, colorReset, query)
	fmt.Println()

	for i, r := range results {
		res, ok := r.(map[string]interface{})
		if !ok {
			continue
		}

		score, _ := res["score"].(float64)
		docName, _ := res["document_name"].(string)

		chunk, _ := res["chunk"].(map[string]interface{})
		content, _ := chunk["content"].(string)

		// Truncate content for display
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		content = strings.ReplaceAll(content, "\n", " ")

		scoreColor := colorGreen
		if score < 0.5 {
			scoreColor = colorYellow
		} else if score < 0.3 {
			scoreColor = colorRed
		}

		fmt.Printf("   %s%d.%s %s[%.2f]%s %s%s%s\n",
			brandSecondary, i+1, colorReset,
			scoreColor, score, colorReset,
			colorDim, docName, colorReset)
		fmt.Printf("      %s%s%s\n", colorDim, content, colorReset)
		fmt.Println()
	}
}

func handleKBClear() {
	port := getServerPort()
	fmt.Printf("%sWarning:%s This will remove all documents from the knowledge base.\n", colorYellow, colorReset)
	fmt.Print("Are you sure? (yes/no): ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	if input != "yes" && input != "y" {
		fmt.Println("Cancelled.")
		return
	}

	// Get list of documents first
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/v1/rag/status", port))
	if err != nil {
		printError(fmt.Sprintf("Failed to connect to server: %v", err))
		return
	}
	defer resp.Body.Close()

	var status map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&status)

	docs, ok := status["documents"].([]interface{})
	if !ok || len(docs) == 0 {
		fmt.Println("Knowledge base is already empty.")
		return
	}

	// Delete each document
	deleted := 0
	for _, d := range docs {
		doc, ok := d.(map[string]interface{})
		if !ok {
			continue
		}
		id, _ := doc["id"].(string)
		if id == "" {
			continue
		}

		req, _ := http.NewRequest(http.MethodDelete,
			fmt.Sprintf("http://localhost:%d/v1/documents/delete?id=%s", port, id), nil)
		client := &http.Client{}
		resp, err := client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			deleted++
		}
		if resp != nil {
			resp.Body.Close()
		}
	}

	printSuccess(fmt.Sprintf("Cleared %d documents from knowledge base", deleted))
}

// formatTimeAgo formats a time as a human-readable "ago" string
func formatTimeAgo(t time.Time) string {
	duration := time.Since(t)

	if duration < time.Minute {
		return "just now"
	} else if duration < time.Hour {
		minutes := int(duration.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	} else if duration < 24*time.Hour {
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	} else if duration < 7*24*time.Hour {
		days := int(duration.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	} else {
		return t.Format("2006-01-02")
	}
}

// handleCompletions generates shell completion scripts
func handleCompletions(args []string) {
	// Only show banner/help when no args provided (help mode)
	// Don't show when generating actual completion scripts
	if len(args) == 0 {
		fmt.Println()
		fmt.Printf("  %s◈ Shell Completions%s\n", brandPrimary+colorBold, colorReset)
		fmt.Printf("  %sTab completion for your shell%s\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sUsage%s  offgrid completions <shell>\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sSupported%s  bash, zsh, fish\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sExamples%s\n", brandPrimary, colorReset)
		fmt.Printf("    %s$%s offgrid completions bash >> ~/.bashrc\n", colorDim, colorReset)
		fmt.Printf("    %s$%s offgrid completions zsh > ~/.zsh/completions/_offgrid\n", colorDim, colorReset)
		fmt.Printf("    %s$%s offgrid completions fish > ~/.config/fish/completions/offgrid.fish\n", colorDim, colorReset)
		fmt.Println()
		return
	}

	shell := args[0]
	gen := completions.NewGenerator("offgrid")

	// Add all commands with their subcommands and flags
	gen.AddCommand(completions.Command{
		Name:        "list",
		Flags:       []string{"--json"},
		Description: "List available models",
	})
	gen.AddCommand(completions.Command{
		Name:        "search",
		Flags:       []string{"--author", "--limit", "--json"},
		Description: "Search for models",
	})
	gen.AddCommand(completions.Command{
		Name:        "download",
		Description: "Download a model",
	})
	gen.AddCommand(completions.Command{
		Name:        "download-hf",
		Flags:       []string{"--quant"},
		Description: "Download from HuggingFace",
	})
	gen.AddCommand(completions.Command{
		Name:        "run",
		Flags:       []string{"--save", "--load"},
		Description: "Start interactive chat",
	})
	gen.AddCommand(completions.Command{
		Name:        "import",
		Description: "Import models from USB",
	})
	gen.AddCommand(completions.Command{
		Name:        "export",
		Description: "Export model to USB",
	})
	gen.AddCommand(completions.Command{
		Name:        "remove",
		Description: "Remove a model",
	})
	gen.AddCommand(completions.Command{
		Name:        "benchmark",
		Description: "Benchmark a model",
	})
	gen.AddCommand(completions.Command{
		Name:        "quantization",
		Description: "Show quantization info",
	})
	gen.AddCommand(completions.Command{
		Name:        "alias",
		Subcommands: []string{"list", "set", "remove"},
		Description: "Manage model aliases",
	})
	gen.AddCommand(completions.Command{
		Name:        "favorite",
		Subcommands: []string{"list", "add", "remove"},
		Description: "Manage favorites",
	})
	gen.AddCommand(completions.Command{
		Name:        "template",
		Subcommands: []string{"list", "show", "apply"},
		Description: "Manage templates",
	})
	gen.AddCommand(completions.Command{
		Name:        "batch",
		Subcommands: []string{"process"},
		Flags:       []string{"--concurrency"},
		Description: "Batch processing",
	})
	gen.AddCommand(completions.Command{
		Name:        "session",
		Subcommands: []string{"list", "show", "export", "delete"},
		Flags:       []string{"--json"},
		Description: "Manage sessions",
	})
	gen.AddCommand(completions.Command{
		Name:        "completions",
		Subcommands: []string{"bash", "zsh", "fish"},
		Description: "Generate completions",
	})
	gen.AddCommand(completions.Command{
		Name:        "config",
		Subcommands: []string{"init", "show", "validate"},
		Description: "Manage configuration",
	})
	gen.AddCommand(completions.Command{
		Name:        "info",
		Flags:       []string{"--json"},
		Description: "System information",
	})
	gen.AddCommand(completions.Command{
		Name:        "serve",
		Description: "Start API server",
	})
	gen.AddCommand(completions.Command{
		Name:        "help",
		Description: "Show help",
	})

	var script string
	switch shell {
	case "bash":
		script = gen.GenerateBash()
	case "zsh":
		script = gen.GenerateZsh()
	case "fish":
		script = gen.GenerateFish()
	default:
		// Check if it's a help flag
		if shell == "--help" || shell == "-h" || shell == "help" {
			fmt.Println()
			fmt.Printf("  %s◈ Shell Completions%s\n", brandPrimary+colorBold, colorReset)
			fmt.Printf("  %sTab completion for your shell%s\n", colorDim, colorReset)
			fmt.Println()
			fmt.Printf("  %sUsage%s  offgrid completions <shell>\n", colorDim, colorReset)
			fmt.Println()
			fmt.Printf("  %sSupported%s  bash, zsh, fish\n", colorDim, colorReset)
			fmt.Println()
			fmt.Printf("  %sExamples%s\n", brandPrimary, colorReset)
			fmt.Printf("    %s$%s offgrid completions bash >> ~/.bashrc\n", colorDim, colorReset)
			fmt.Printf("    %s$%s offgrid completions zsh > ~/.zsh/completions/_offgrid\n", colorDim, colorReset)
			fmt.Printf("    %s$%s offgrid completions fish > ~/.config/fish/completions/offgrid.fish\n", colorDim, colorReset)
			fmt.Println()
			return
		}
		printError(fmt.Sprintf("Unsupported shell: %s", shell))
		fmt.Printf("  %sSupported:%s bash, zsh, fish\n", brandMuted, colorReset)
		fmt.Println()
		return
	}

	fmt.Println(script)
}

func handleVerify(args []string) {
	// Check for help flag
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		args = []string{} // Trigger help display
	}

	if len(args) < 1 {
		fmt.Println()
		fmt.Printf("  %s◈ Verify Model%s\n", brandPrimary+colorBold, colorReset)
		fmt.Printf("  %sVerify model file integrity using SHA256 checksum%s\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sUsage%s  offgrid verify <model-name>\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sExamples%s\n", brandPrimary, colorReset)
		fmt.Printf("    %s$%s offgrid verify llama-2-7b-chat.Q4_K_M\n", colorDim, colorReset)
		fmt.Println()
		return
	}

	modelName := args[0]
	cfg := config.LoadConfig()
	registry := models.NewRegistry(cfg.ModelsDir)

	if err := registry.ScanModels(); err != nil {
		printHelpfulError(err, "Scanning models")
		return
	}

	meta, err := registry.GetModel(modelName)
	if err != nil {
		printError(fmt.Sprintf("Model not found: %s", modelName))
		fmt.Println()
		printInfo("Use 'offgrid list' to see available models")
		fmt.Println()
		return
	}

	fmt.Println()
	fmt.Printf("%sVerifying Model%s\n", brandPrimary+colorBold, colorReset)
	fmt.Printf("Model: %s%s%s\n", colorBold, meta.ID, colorReset)
	fmt.Printf("Path:  %s%s%s\n", colorDim, meta.Path, colorReset)
	fmt.Println("")

	validator := models.NewValidator(cfg.ModelsDir)
	result, err := validator.ValidateModel(meta.Path)
	if err != nil {
		printHelpfulError(err, "Validation")
		return
	}

	if result.Valid {
		fmt.Printf("%s✓ Valid GGUF Model%s\n", brandSuccess, colorReset)
	} else {
		fmt.Printf("%s✗ Invalid Model%s\n", brandError, colorReset)
	}

	fmt.Printf("Size: %s\n", formatBytes(result.FileSize))
	if result.SHA256Hash != "" {
		fmt.Printf("SHA256: %s%s%s\n", colorDim, result.SHA256Hash, colorReset)
	}

	if len(result.Errors) > 0 {
		fmt.Println("")
		fmt.Printf("%sErrors:%s\n", brandError, colorReset)
		for _, err := range result.Errors {
			fmt.Printf("• %s\n", err)
		}
	}

	if len(result.Warnings) > 0 {
		fmt.Println("")
		fmt.Printf("%sWarnings:%s\n", brandAccent, colorReset)
		for _, warn := range result.Warnings {
			fmt.Printf("• %s\n", warn)
		}
	}

	fmt.Println("")
	fmt.Println()
}

func handleShellCompletion(args []string) {
	if len(args) < 1 {
		fmt.Println()
		fmt.Printf("%sShell Completions%s\n", brandPrimary+colorBold, colorReset)
		fmt.Println("")
		fmt.Printf("%sUsage:%s offgrid shell-completion <shell>\n", colorDim, colorReset)
		fmt.Println("")
		fmt.Printf("%sSupported shells:%s bash, zsh, fish\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("%sInstallation%s\n", brandPrimary+colorBold, colorReset)
		fmt.Println()
		fmt.Println("  Bash:")
		fmt.Printf("   %s$ offgrid shell-completion bash > /etc/bash_completion.d/offgrid%s\n", colorDim, colorReset)
		fmt.Println()
		fmt.Println("  Zsh:")
		fmt.Printf("   %s$ offgrid shell-completion zsh > ~/.zsh/completions/_offgrid%s\n", colorDim, colorReset)
		fmt.Println()
		fmt.Println("  Fish:")
		fmt.Printf("   %s$ offgrid shell-completion fish > ~/.config/fish/completions/offgrid.fish%s\n", colorDim, colorReset)
		fmt.Println()
		return
	}

	shell := strings.ToLower(args[0])

	// Call the existing completions handler
	handleCompletions([]string{shell})
}

func handleExportSession(args []string) {
	if len(args) < 1 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Println()
		fmt.Printf("%sExport Session%s\n", brandPrimary+colorBold, colorReset)
		fmt.Println("")
		fmt.Printf("%sUsage:%s offgrid export-session <session-name> [options]\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("%sOptions%s\n", brandPrimary+colorBold, colorReset)
		printOption("--format <type>", "Output format: markdown, json, txt (default: markdown)")
		printOption("--output <file>", "Output file (default: stdout)")
		fmt.Println()
		printInfo("Exports chat session for documentation or research papers")
		fmt.Println()
		return
	}

	sessionName := args[0]
	format := "markdown"
	outputFile := ""

	// Parse options
	for i := 1; i < len(args); i++ {
		if args[i] == "--format" && i+1 < len(args) {
			format = args[i+1]
			i++
		} else if args[i] == "--output" && i+1 < len(args) {
			outputFile = args[i+1]
			i++
		}
	}

	homeDir, _ := os.UserHomeDir()
	sessionsDir := filepath.Join(homeDir, ".offgrid", "sessions")
	sessionMgr := sessions.NewSessionManager(sessionsDir)

	session, err := sessionMgr.Load(sessionName)
	if err != nil {
		printHelpfulError(err, "Loading session")
		return
	}

	var output string
	switch format {
	case "markdown", "md":
		output = exportSessionMarkdown(session)
	case "json":
		data, err := json.MarshalIndent(session, "", "  ")
		if err != nil {
			printHelpfulError(err, "Exporting to JSON")
			return
		}
		output = string(data)
	case "txt", "text":
		output = exportSessionText(session)
	default:
		printError(fmt.Sprintf("Unknown format: %s", format))
		fmt.Println()
		printInfo("Supported formats: markdown, json, txt")
		fmt.Println()
		return
	}

	if outputFile != "" {
		if err := os.WriteFile(outputFile, []byte(output), 0644); err != nil {
			printHelpfulError(err, "Writing output file")
			return
		}
		fmt.Println()
		printSuccess(fmt.Sprintf("Session exported to %s", outputFile))
		fmt.Println()
	} else {
		fmt.Println(output)
	}
}

func exportSessionMarkdown(session *sessions.Session) string {
	var sb strings.Builder

	sb.WriteString("# Chat Session: " + session.Name + "\n\n")
	sb.WriteString("**Model:** " + session.ModelID + "  \n")
	sb.WriteString("**Created:** " + session.CreatedAt.Format("2006-01-02 15:04:05") + "  \n")
	sb.WriteString("**Updated:** " + session.UpdatedAt.Format("2006-01-02 15:04:05") + "  \n")
	sb.WriteString("\n---\n\n")

	for _, msg := range session.Messages {
		if msg.Role == "user" {
			sb.WriteString("### User\n\n")
		} else {
			sb.WriteString("### Assistant\n\n")
		}
		sb.WriteString(msg.Content + "\n\n")
	}

	return sb.String()
}

func exportSessionText(session *sessions.Session) string {
	var sb strings.Builder

	sb.WriteString("Chat Session: " + session.Name + "\n")
	sb.WriteString("Model: " + session.ModelID + "\n")
	sb.WriteString("Created: " + session.CreatedAt.Format("2006-01-02 15:04:05") + "\n")
	sb.WriteString(strings.Repeat("=", 60) + "\n\n")

	for _, msg := range session.Messages {
		if msg.Role == "user" {
			sb.WriteString("[USER]\n")
		} else {
			sb.WriteString("[ASSISTANT]\n")
		}
		sb.WriteString(msg.Content + "\n\n")
	}

	return sb.String()
}

func handleBenchmarkCompare(args []string) {
	if len(args) < 2 {
		fmt.Println()
		fmt.Printf("%sBenchmark Compare%s\n", brandPrimary+colorBold, colorReset)
		fmt.Println("")
		fmt.Printf("%sUsage:%s offgrid benchmark-compare <model1> <model2> [model3...] [--iterations N]\n", colorDim, colorReset)
		fmt.Println("")
		fmt.Println("Compare performance across multiple models")
		fmt.Println()
		fmt.Printf("%sOptions%s\n", brandPrimary+colorBold, colorReset)
		printOption("--iterations N", "Number of test iterations (default: 3)")
		printOption("--prompt \"text\"", "Custom prompt for comparison")
		fmt.Println()
		fmt.Printf("%sExamples%s\n", brandPrimary+colorBold, colorReset)
		printExample("offgrid compare tinyllama-1.1b phi-2")
		printExample("offgrid compare llama-2-7b mistral-7b phi-2 --iterations 5")
		fmt.Println()
		os.Exit(1)
	}

	cfg := config.LoadConfig()
	registry := models.NewRegistry(cfg.ModelsDir)

	if err := registry.ScanModels(); err != nil {
		fmt.Fprintf(os.Stderr, "✗ Error scanning models: %v\n\n", err)
		os.Exit(1)
	}

	// Parse arguments
	var modelIDs []string
	iterations := 3
	customPrompt := ""

	for i := 0; i < len(args); i++ {
		if args[i] == "--iterations" && i+1 < len(args) {
			fmt.Sscanf(args[i+1], "%d", &iterations)
			i++
		} else if args[i] == "--prompt" && i+1 < len(args) {
			customPrompt = args[i+1]
			i++
		} else if !strings.HasPrefix(args[i], "--") {
			modelIDs = append(modelIDs, args[i])
		}
	}

	if len(modelIDs) < 2 {
		printError("Need at least 2 models to compare")
		os.Exit(1)
	}

	// Check if server is running
	serverURL := fmt.Sprintf("http://127.0.0.1:%d", cfg.ServerPort)
	if !isServerHealthy(serverURL) {
		fmt.Printf("%sError: Server not running%s\n", colorRed, colorReset)
		fmt.Printf("Start server first: %soffgrid serve%s\n\n", brandSecondary, colorReset)
		os.Exit(1)
	}

	// Default benchmark prompt
	benchPrompt := "Write a short story about a robot learning to paint."
	if customPrompt != "" {
		benchPrompt = customPrompt
	}

	fmt.Println()
	fmt.Printf("%sBenchmark Comparison%s\n", brandPrimary+colorBold, colorReset)
	fmt.Printf("Comparing %d models with %d iterations each\n", len(modelIDs), iterations)

	// Test if inference backend is working with first model
	testPayload := fmt.Sprintf(`{"model":"%s","prompt":"test","max_tokens":1}`, modelIDs[0])
	testResp, testErr := http.Post(
		serverURL+"/v1/completions",
		"application/json",
		bytes.NewBuffer([]byte(testPayload)),
	)
	if testErr == nil {
		defer testResp.Body.Close()
		testBody, _ := io.ReadAll(testResp.Body)
		var testResult map[string]interface{}
		if json.Unmarshal(testBody, &testResult) == nil {
			if errObj, ok := testResult["error"].(map[string]interface{}); ok {
				if msg, ok := errObj["message"].(string); ok {
					if strings.Contains(msg, "llama-server") || strings.Contains(msg, "connection refused") {
						fmt.Println("")
						fmt.Printf("%s⚠ Warning:%s llama-server backend not running\n", colorYellow, colorReset)
						fmt.Printf("%sBenchmark will show API latency only (no actual inference)%s\n", colorDim, colorReset)
					}
				}
			}
		}
	}
	fmt.Println()

	type BenchResult struct {
		ModelID    string
		AvgSpeed   float64
		AvgLatency time.Duration
		MinSpeed   float64
		MaxSpeed   float64
		Size       int64
		Quant      string
		Failed     bool
	}

	results := make([]BenchResult, 0, len(modelIDs))

	for _, modelID := range modelIDs {
		fmt.Printf("%sTesting: %s%s\n", brandPrimary, modelID, colorReset)

		meta, err := registry.GetModel(modelID)
		if err != nil {
			fmt.Printf("%s✗ Model not found%s\n", colorRed, colorReset)
			results = append(results, BenchResult{ModelID: modelID, Failed: true})
			continue
		}

		var (
			totalLatency time.Duration
			tokensPerSec []float64
		)

		for i := 0; i < iterations; i++ {
			fmt.Printf("[%d/%d] ", i+1, iterations)

			startTime := time.Now()

			reqBody := map[string]interface{}{
				"model":       modelID,
				"prompt":      benchPrompt,
				"max_tokens":  100,
				"temperature": 0.7,
				"stream":      false,
			}

			jsonData, _ := json.Marshal(reqBody)
			resp, err := http.Post(
				serverURL+"/v1/completions",
				"application/json",
				bytes.NewBuffer(jsonData),
			)

			if err != nil {
				fmt.Printf("%s✗%s\n", colorRed, colorReset)
				continue
			}

			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			var result map[string]interface{}
			if err := json.Unmarshal(body, &result); err != nil {
				fmt.Printf("%s✗%s Parse error\n", colorRed, colorReset)
				continue
			}

			// Check for API error response
			if errObj, ok := result["error"].(map[string]interface{}); ok {
				errMsg := "API error"
				if msg, ok := errObj["message"].(string); ok {
					// Shorten error message
					if len(msg) > 50 {
						errMsg = msg[:50] + "..."
					} else {
						errMsg = msg
					}
				}
				fmt.Printf("%s✗%s %s\n", colorRed, colorReset, errMsg)
				continue
			}

			latency := time.Since(startTime)
			tokenCount := 0

			if usage, ok := result["usage"].(map[string]interface{}); ok {
				if ct, ok := usage["completion_tokens"].(float64); ok {
					tokenCount = int(ct)
				}
			}

			// If no tokens were generated, this iteration failed
			if tokenCount == 0 {
				fmt.Printf("%s✗%s No tokens generated\n", colorRed, colorReset)
				continue
			}

			totalLatency += latency
			tps := float64(tokenCount) / latency.Seconds()
			tokensPerSec = append(tokensPerSec, tps)

			fmt.Printf("%s✓%s %.1f tok/s\n", colorGreen, colorReset, tps)
		}

		if len(tokensPerSec) == 0 {
			results = append(results, BenchResult{ModelID: modelID, Failed: true})
			continue
		}

		avgLatency := totalLatency / time.Duration(len(tokensPerSec))
		avgTPS := 0.0
		minTPS := tokensPerSec[0]
		maxTPS := tokensPerSec[0]

		for _, tps := range tokensPerSec {
			avgTPS += tps
			if tps < minTPS {
				minTPS = tps
			}
			if tps > maxTPS {
				maxTPS = tps
			}
		}
		avgTPS /= float64(len(tokensPerSec))

		results = append(results, BenchResult{
			ModelID:    modelID,
			AvgSpeed:   avgTPS,
			AvgLatency: avgLatency,
			MinSpeed:   minTPS,
			MaxSpeed:   maxTPS,
			Size:       meta.Size,
			Quant:      meta.Quantization,
		})

		fmt.Printf("%sAverage: %.1f tok/s%s\n", colorDim, avgTPS, colorReset)
		fmt.Println("")
	}

	// Display comparison table
	fmt.Printf("%sComparison Results%s\n", brandPrimary+colorBold, colorReset)
	fmt.Println()

	// Sort by speed (descending)
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].AvgSpeed > results[i].AvgSpeed {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	// Print header
	fmt.Printf("   %s%-30s  %10s  %12s  %10s  %8s%s\n",
		colorDim, "Model", "Speed", "Latency", "Range", "Size", colorReset)

	fastest := results[0].AvgSpeed
	for i, r := range results {
		if r.Failed {
			fmt.Printf("   %-30s  %s%10s%s\n", r.ModelID, colorRed, "FAILED", colorReset)
			continue
		}

		rank := ""
		if i == 0 {
			rank = colorGreen + "★ " + colorReset
		}

		speedPct := (r.AvgSpeed / fastest) * 100
		speedColor := colorGreen
		if speedPct < 80 {
			speedColor = colorYellow
		}
		if speedPct < 50 {
			speedColor = colorRed
		}

		sizeStr := formatBytes(r.Size)
		if r.Quant != "" {
			sizeStr += " " + r.Quant
		}

		fmt.Printf("   %s%-30s  %s%8.1f%s t/s  %10s  %4.0f-%4.0f  %10s\n",
			rank, r.ModelID,
			speedColor, r.AvgSpeed, colorReset,
			formatDuration(r.AvgLatency),
			r.MinSpeed, r.MaxSpeed,
			sizeStr)
	}

	fmt.Println()
	fmt.Printf("   %s★ = Fastest model%s\n", colorDim, colorReset)
	fmt.Println()
}

// handleUsers handles user management commands
func handleUsers(args []string) {
	cfg := config.LoadConfig()

	// Check if multi-user mode is enabled
	if !cfg.MultiUserMode {
		fmt.Println()
		fmt.Printf("  %s◈ Users%s\n", brandPrimary+colorBold, colorReset)
		fmt.Printf("  %sSingle-user mode (default)%s\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sTo enable multi-user:%s\n", colorDim, colorReset)
		fmt.Printf("    %s$%s export OFFGRID_MULTI_USER=true\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sOr in config:%s\n", colorDim, colorReset)
		fmt.Printf("    multi_user_mode: true\n")
		fmt.Println()
		return
	}

	dataDir := filepath.Join(cfg.ModelsDir, "..", "data")
	store := users.NewUserStore(dataDir)

	if len(args) < 1 {
		fmt.Println()
		fmt.Printf("  %s◈ Users%s\n", brandPrimary+colorBold, colorReset)
		fmt.Printf("  %sMulti-user management%s\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sUsage%s  offgrid users <command>\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sCommands%s\n", brandPrimary, colorReset)
		fmt.Printf("    %-24s %sList all users%s\n", "list", colorDim, colorReset)
		fmt.Printf("    %-24s %sCreate a user%s\n", "create <name> <role>", colorDim, colorReset)
		fmt.Printf("    %-24s %sDelete a user%s\n", "delete <id>", colorDim, colorReset)
		fmt.Printf("    %-24s %sShow user info%s\n", "info <id>", colorDim, colorReset)
		fmt.Printf("    %-24s %sRegenerate API key%s\n", "reset-key <id>", colorDim, colorReset)
		fmt.Printf("    %-24s %sShow quota usage%s\n", "quota <id>", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sRoles%s  admin, user, viewer, guest\n", colorDim, colorReset)
		fmt.Println()
		return
	}

	subCmd := args[0]
	switch subCmd {
	case "list":
		userList := store.ListUsers()
		if output.JSONMode {
			publicUsers := make([]users.UserPublic, 0, len(userList))
			for _, u := range userList {
				publicUsers = append(publicUsers, u.ToPublic())
			}
			output.PrintJSON(publicUsers)
			return
		}
		fmt.Println()
		fmt.Printf("%sUsers%s\n", brandPrimary+colorBold, colorReset)
		fmt.Println()
		if len(userList) == 0 {
			fmt.Println("  No users configured")
			fmt.Println()
			fmt.Printf("  %sCreate a user:%s offgrid users create <name> <role>\n", colorDim, colorReset)
		} else {
			fmt.Printf("  %s%-32s  %-10s  %-10s  %s%s\n", colorDim, "ID", "Username", "Role", "Created", colorReset)
			for _, u := range userList {
				status := ""
				if u.Disabled {
					status = " (disabled)"
				}
				fmt.Printf("  %-32s  %-10s  %-10s  %s%s\n", u.ID[:16]+"...", u.Username, u.Role, u.CreatedAt.Format("2006-01-02"), status)
			}
		}
		fmt.Println()

	case "create":
		if len(args) < 3 {
			printError("Usage: offgrid users create <username> <role>")
			return
		}
		username := args[1]
		role := users.Role(args[2])

		// Validate role
		validRoles := map[users.Role]bool{
			users.RoleAdmin: true, users.RoleUser: true,
			users.RoleViewer: true, users.RoleGuest: true,
		}
		if !validRoles[role] {
			printError("Invalid role. Use: admin, user, viewer, or guest")
			return
		}

		// Generate a random password
		password := fmt.Sprintf("temp-%d", time.Now().UnixNano())

		user, apiKey, err := store.CreateUser(username, password, role)
		if err != nil {
			printError(fmt.Sprintf("Failed to create user: %v", err))
			return
		}

		if output.JSONMode {
			output.PrintJSON(map[string]any{
				"id":       user.ID,
				"username": user.Username,
				"role":     user.Role,
				"api_key":  apiKey,
			})
			return
		}

		fmt.Println()
		printSuccess(fmt.Sprintf("Created user: %s", username))
		fmt.Println()
		fmt.Printf("  %sUser ID:%s     %s\n", colorDim, colorReset, user.ID)
		fmt.Printf("  %sUsername:%s    %s\n", colorDim, colorReset, user.Username)
		fmt.Printf("  %sRole:%s        %s\n", colorDim, colorReset, user.Role)
		fmt.Printf("  %sAPI Key:%s     %s\n", colorDim, colorReset, apiKey)
		fmt.Println()
		fmt.Printf("  %s⚠ Save the API key - it cannot be retrieved later%s\n", colorYellow, colorReset)
		fmt.Println()

	case "delete":
		if len(args) < 2 {
			printError("Usage: offgrid users delete <user-id>")
			return
		}
		userID := args[1]
		if err := store.DeleteUser(userID); err != nil {
			printError(fmt.Sprintf("Failed to delete user: %v", err))
			return
		}
		printSuccess(fmt.Sprintf("Deleted user: %s", userID))

	case "info":
		if len(args) < 2 {
			printError("Usage: offgrid users info <user-id>")
			return
		}
		userID := args[1]
		user, ok := store.GetUser(userID)
		if !ok {
			printError("User not found")
			return
		}
		if output.JSONMode {
			output.PrintJSON(user.ToPublic())
			return
		}
		fmt.Println()
		fmt.Printf("%sUser Info%s\n", brandPrimary+colorBold, colorReset)
		fmt.Println()
		fmt.Printf("  %sID:%s          %s\n", colorDim, colorReset, user.ID)
		fmt.Printf("  %sUsername:%s    %s\n", colorDim, colorReset, user.Username)
		fmt.Printf("  %sRole:%s        %s\n", colorDim, colorReset, user.Role)
		fmt.Printf("  %sCreated:%s     %s\n", colorDim, colorReset, user.CreatedAt.Format(time.RFC3339))
		if user.LastLoginAt != nil {
			fmt.Printf("  %sLast Login:%s  %s\n", colorDim, colorReset, user.LastLoginAt.Format(time.RFC3339))
		}
		fmt.Printf("  %sDisabled:%s    %v\n", colorDim, colorReset, user.Disabled)
		fmt.Println()

	case "reset-key":
		if len(args) < 2 {
			printError("Usage: offgrid users reset-key <user-id>")
			return
		}
		userID := args[1]
		apiKey, err := store.RegenerateAPIKey(userID)
		if err != nil {
			printError(fmt.Sprintf("Failed to regenerate API key: %v", err))
			return
		}
		if output.JSONMode {
			output.PrintJSON(map[string]string{"api_key": apiKey})
			return
		}
		fmt.Println()
		printSuccess("API key regenerated")
		fmt.Printf("  %sNew API Key:%s %s\n", colorDim, colorReset, apiKey)
		fmt.Println()
		fmt.Printf("  %s⚠ Save the API key - it cannot be retrieved later%s\n", colorYellow, colorReset)
		fmt.Println()

	case "quota":
		if len(args) < 2 {
			printError("Usage: offgrid users quota <user-id>")
			return
		}
		userID := args[1]
		quotaManager := users.NewQuotaManager(dataDir)
		summary := quotaManager.GetUsageSummary(userID)
		if output.JSONMode {
			output.PrintJSON(summary)
			return
		}
		fmt.Println()
		fmt.Printf("%sQuota Usage%s\n", brandPrimary+colorBold, colorReset)
		fmt.Println()
		quotas, ok := summary["quotas"].([]map[string]any)
		if !ok || len(quotas) == 0 {
			fmt.Println("  No quotas configured for this user")
		} else {
			for _, q := range quotas {
				exceeded := ""
				if ex, ok := q["exceeded"].(bool); ok && ex {
					exceeded = " " + colorRed + "(EXCEEDED)" + colorReset
				}
				fmt.Printf("  %s (%s): %v / %v%s\n", q["type"], q["period"], q["current"], q["limit"], exceeded)
			}
		}
		fmt.Println()

	default:
		printError(fmt.Sprintf("Unknown users command: %s", subCmd))
	}
}

// handleMetrics handles metrics display
func handleMetrics(args []string) {
	cfg := config.LoadConfig()

	// Try to fetch from running server
	serverURL := fmt.Sprintf("http://localhost:%d/metrics", cfg.ServerPort)

	resp, err := http.Get(serverURL)
	if err != nil {
		// Server not running
		if output.JSONMode {
			output.PrintJSON(map[string]string{"status": "server_not_running", "message": "Start server to view live metrics"})
			return
		}
		fmt.Println()
		fmt.Printf("%sMetrics%s\n", brandPrimary+colorBold, colorReset)
		fmt.Println()
		fmt.Printf("  %sServer not running%s\n", colorYellow, colorReset)
		fmt.Println()
		fmt.Printf("  Start the server to view live metrics:\n")
		fmt.Printf("    %soffgrid serve%s\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  Then access metrics at:\n")
		fmt.Printf("    %s%s%s\n", colorDim, serverURL, colorReset)
		fmt.Println()
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if output.JSONMode {
		// Parse Prometheus format to JSON
		lines := strings.Split(string(body), "\n")
		metricsData := make(map[string]string)
		for _, line := range lines {
			if strings.HasPrefix(line, "#") || line == "" {
				continue
			}
			parts := strings.SplitN(line, " ", 2)
			if len(parts) == 2 {
				metricsData[parts[0]] = parts[1]
			}
		}
		output.PrintJSON(metricsData)
		return
	}

	fmt.Println()
	fmt.Printf("%sPrometheus Metrics%s\n", brandPrimary+colorBold, colorReset)
	fmt.Println()
	fmt.Println(string(body))
}

// handleLoRA handles LoRA adapter management
func handleLoRA(args []string) {
	cfg := config.LoadConfig()
	dataDir := filepath.Join(cfg.ModelsDir, "..", "data")
	manager := inference.NewLoRAManager(dataDir, nil)

	// Check for help flag
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		args = []string{} // Trigger help display
	}

	if len(args) < 1 {
		fmt.Println()
		fmt.Printf("  %s◈ LoRA Adapters%s\n", brandPrimary+colorBold, colorReset)
		fmt.Printf("  %sFine-tuning adapter management%s\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sUsage%s  offgrid lora <command>\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sCommands%s\n", brandPrimary, colorReset)
		fmt.Printf("    %-24s %sList registered adapters%s\n", "list", colorDim, colorReset)
		fmt.Printf("    %-24s %sRegister an adapter%s\n", "register <name> <path>", colorDim, colorReset)
		fmt.Printf("    %-24s %sRemove an adapter%s\n", "remove <id>", colorDim, colorReset)
		fmt.Printf("    %-24s %sShow adapter info%s\n", "info <id>", colorDim, colorReset)
		fmt.Printf("    %-24s %sSet scale (0.0-1.0)%s\n", "scale <id> <value>", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sNote%s  Use /v1/lora/* API for runtime loading\n", colorDim, colorReset)
		fmt.Println()
		return
	}

	subCmd := args[0]
	switch subCmd {
	case "list":
		adapterList := manager.ListAdapters()
		if output.JSONMode {
			output.PrintJSON(manager.GetStatus())
			return
		}
		fmt.Println()
		fmt.Printf("%sLoRA Adapters%s\n", brandPrimary+colorBold, colorReset)
		fmt.Println()
		if len(adapterList) == 0 {
			fmt.Println("  No adapters registered")
			fmt.Println()
			fmt.Printf("  %sRegister an adapter:%s offgrid lora register <name> <path>\n", colorDim, colorReset)
		} else {
			fmt.Printf("  %s%-16s  %-20s  %-8s  %s%s\n", colorDim, "ID", "Name", "Scale", "Path", colorReset)
			for _, a := range adapterList {
				fmt.Printf("  %-16s  %-20s  %-8.2f  %s\n", a.ID[:16], a.Name, a.Scale, a.Path)
			}
		}
		fmt.Println()

	case "register":
		if len(args) < 3 {
			printError("Usage: offgrid lora register <name> <path> [scale]")
			return
		}
		name := args[1]
		path := args[2]
		scale := float32(1.0)
		if len(args) > 3 {
			fmt.Sscanf(args[3], "%f", &scale)
		}

		adapter, err := manager.RegisterAdapter("", name, path, scale, "")
		if err != nil {
			printError(fmt.Sprintf("Failed to register adapter: %v", err))
			return
		}

		if output.JSONMode {
			output.PrintJSON(adapter)
			return
		}
		printSuccess(fmt.Sprintf("Registered adapter: %s (%s)", adapter.Name, adapter.ID))

	case "remove":
		if len(args) < 2 {
			printError("Usage: offgrid lora remove <id>")
			return
		}
		if err := manager.DeleteAdapter(args[1]); err != nil {
			printError(fmt.Sprintf("Failed to remove adapter: %v", err))
			return
		}
		printSuccess("Adapter removed")

	case "info":
		if len(args) < 2 {
			printError("Usage: offgrid lora info <id>")
			return
		}
		adapter, ok := manager.GetAdapter(args[1])
		if !ok {
			printError("Adapter not found")
			return
		}
		if output.JSONMode {
			output.PrintJSON(adapter)
			return
		}
		fmt.Println()
		fmt.Printf("%sAdapter Info%s\n", brandPrimary+colorBold, colorReset)
		fmt.Println()
		fmt.Printf("  %sID:%s          %s\n", colorDim, colorReset, adapter.ID)
		fmt.Printf("  %sName:%s        %s\n", colorDim, colorReset, adapter.Name)
		fmt.Printf("  %sPath:%s        %s\n", colorDim, colorReset, adapter.Path)
		fmt.Printf("  %sScale:%s       %.2f\n", colorDim, colorReset, adapter.Scale)
		fmt.Printf("  %sCreated:%s     %s\n", colorDim, colorReset, adapter.CreatedAt.Format(time.RFC3339))
		fmt.Println()

	case "scale":
		if len(args) < 3 {
			printError("Usage: offgrid lora scale <id> <value>")
			return
		}
		var scale float32
		fmt.Sscanf(args[2], "%f", &scale)
		if err := manager.SetAdapterScale(context.Background(), args[1], scale); err != nil {
			printError(fmt.Sprintf("Failed to set scale: %v", err))
			return
		}
		printSuccess(fmt.Sprintf("Scale updated to %.2f", scale))

	default:
		printError(fmt.Sprintf("Unknown lora command: %s", subCmd))
	}
}

// handleAgent handles AI agent commands
func handleAgent(args []string) {
	// Check for help flag
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		printAgentHelp()
		return
	}

	// Interactive mode if no args or "chat" or flags
	if len(args) < 1 || args[0] == "chat" || strings.HasPrefix(args[0], "--") {
		modelName := ""
		// Simple flag parsing for interactive mode
		for i, arg := range args {
			if arg == "--model" && i+1 < len(args) {
				modelName = args[i+1]
			}
		}

		// If no model specified, try to find a reasonable default
		// if modelName == "" {
		// 	modelName = "llama3.2:3b" // Default to a fast model
		// }

		startInteractiveAgent(modelName)
		return
	}

	subCmd := args[0]
	switch subCmd {
	case "run":
		if len(args) < 2 {
			printError("Usage: offgrid agent run <prompt> --model <name>")
			return
		}

		// Parse options first
		style := "react"
		maxSteps := 10
		modelName := ""
		skipNext := false
		var promptParts []string

		for i, arg := range args[1:] {
			if skipNext {
				skipNext = false
				continue
			}
			if arg == "--style" && i+1 < len(args[1:]) {
				switch args[i+2] {
				case "cot":
					style = "cot"
				case "plan":
					style = "plan-execute"
				}
				skipNext = true
				continue
			}
			if arg == "--max-steps" && i+1 < len(args[1:]) {
				fmt.Sscanf(args[i+2], "%d", &maxSteps)
				skipNext = true
				continue
			}
			if arg == "--model" && i+1 < len(args[1:]) {
				modelName = args[i+2]
				skipNext = true
				continue
			}
			if !strings.HasPrefix(arg, "--") {
				promptParts = append(promptParts, arg)
			}
		}

		prompt := strings.Join(promptParts, " ")
		if prompt == "" {
			printError("Usage: offgrid agent run <prompt> --model <name>")
			return
		}

		if modelName == "" {
			printError("Model required. Use: offgrid agent run <prompt> --model <name>\n\n  List models: offgrid list")
			return
		}

		fmt.Println()
		fmt.Printf("  %s◈ Agent Run%s\n", brandPrimary+colorBold, colorReset)
		fmt.Println()
		fmt.Printf("  %sModel%s       %s%s%s\n", colorDim, colorReset, brandSecondary, modelName, colorReset)
		fmt.Printf("  %sStyle%s       %s  %sSteps%s  %d\n", colorDim, colorReset, style, colorDim, colorReset, maxSteps)
		fmt.Printf("  %sTask%s        %s\n", colorDim, colorReset, prompt)
		fmt.Println()
		fmt.Printf("  %s⏳ Processing...%s\n", colorDim, colorReset)

		// Connect to running server with streaming
		cfg := config.LoadConfig()
		serverURL := fmt.Sprintf("http://localhost:%d/v1/agents/run", cfg.ServerPort)

		runAgentRequest(serverURL, prompt, modelName, style, maxSteps)

	case "tasks", "list":
		// Connect to running server
		cfg := config.LoadConfig()
		serverURL := fmt.Sprintf("http://localhost:%d/v1/agents/tasks", cfg.ServerPort)

		resp, err := http.Get(serverURL)
		if err != nil {
			printError(fmt.Sprintf("Cannot connect to server: %v\n\n  Start the server with: offgrid serve", err))
			return
		}
		defer resp.Body.Close()

		var tasks []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&tasks); err != nil {
			printError(fmt.Sprintf("Failed to parse response: %v", err))
			return
		}

		if output.JSONMode {
			output.PrintJSON(tasks)
			return
		}
		fmt.Println()
		fmt.Printf("  %s◈ Recent Tasks%s\n", brandPrimary+colorBold, colorReset)
		fmt.Println()
		if len(tasks) == 0 {
			fmt.Printf("  %sNo tasks yet%s\n", colorDim, colorReset)
			fmt.Println()
			fmt.Printf("  %sRun your first task:%s\n", colorDim, colorReset)
			fmt.Printf("    %s$%s offgrid agent run \"Your task here\" --model <name>\n", colorDim, colorReset)
		} else {
			for _, task := range tasks {
				id, _ := task["id"].(string)
				status, _ := task["status"].(string)
				prompt, _ := task["prompt"].(string)

				// Status icon and color
				statusIcon := "○"
				statusColor := colorYellow
				if status == "completed" {
					statusIcon = "✓"
					statusColor = colorGreen
				} else if status == "failed" {
					statusIcon = "✗"
					statusColor = colorRed
				} else if status == "running" {
					statusIcon = "›"
					statusColor = brandPrimary
				}

				displayID := id
				if len(id) > 8 {
					displayID = id[:8]
				}
				if len(prompt) > 50 {
					prompt = prompt[:47] + "..."
				}
				fmt.Printf("  %s%s%s %s%-10s%s  %s%s%s  %s\n", statusColor, statusIcon, colorReset, statusColor, status, colorReset, colorDim, displayID, colorReset, prompt)
			}
		}
		fmt.Println()

	case "workflows":
		// Connect to running server
		cfg := config.LoadConfig()
		serverURL := fmt.Sprintf("http://localhost:%d/v1/agents/workflows", cfg.ServerPort)

		resp, err := http.Get(serverURL)
		if err != nil {
			printError(fmt.Sprintf("Cannot connect to server: %v\n\n  Start the server with: offgrid serve", err))
			return
		}
		defer resp.Body.Close()

		var workflows []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&workflows); err != nil {
			printError(fmt.Sprintf("Failed to parse response: %v", err))
			return
		}

		if output.JSONMode {
			output.PrintJSON(workflows)
			return
		}
		fmt.Println()
		fmt.Printf("  %s◈ Workflows%s\n", brandPrimary+colorBold, colorReset)
		fmt.Println()
		if len(workflows) == 0 {
			fmt.Printf("  %sNo workflows defined yet%s\n", colorDim, colorReset)
			fmt.Println()
			fmt.Printf("  %sWorkflows are multi-step agent pipelines.%s\n", colorDim, colorReset)
			fmt.Printf("  %sCreate them via the API:%s\n", colorDim, colorReset)
			fmt.Println()
			fmt.Printf("    POST /v1/agents/workflows\n")
			fmt.Printf("    %s{\"name\": \"my-workflow\", \"steps\": [...]}%s\n", colorDim, colorReset)
		} else {
			for _, wf := range workflows {
				name, _ := wf["name"].(string)
				desc := ""
				if d, ok := wf["description"].(string); ok {
					desc = d
				}
				stepCount := 0
				if steps, ok := wf["steps"].([]interface{}); ok {
					stepCount = len(steps)
				}
				fmt.Printf("  %s%s%s", colorBold, name, colorReset)
				if stepCount > 0 {
					fmt.Printf("  %s%d steps%s", colorDim, stepCount, colorReset)
				}
				fmt.Println()
				if desc != "" {
					fmt.Printf("    %s%s%s\n", colorDim, desc, colorReset)
				}
			}
		}
		fmt.Println()

	case "tools":
		// List or manage agent tools
		cfg := config.LoadConfig()
		serverURL := fmt.Sprintf("http://localhost:%d/v1/agents/tools", cfg.ServerPort)

		resp, err := http.Get(serverURL)
		if err != nil {
			printError(fmt.Sprintf("Cannot connect to server: %v\n\n  Start the server with: offgrid serve", err))
			return
		}
		defer resp.Body.Close()

		var result struct {
			Tools []struct {
				Name        string                 `json:"name"`
				Description string                 `json:"description"`
				Parameters  map[string]interface{} `json:"parameters,omitempty"`
				Type        string                 `json:"type"`
			} `json:"tools"`
			Count int `json:"count"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			printError(fmt.Sprintf("Failed to parse response: %v", err))
			return
		}

		if output.JSONMode {
			output.PrintJSON(result)
			return
		}
		fmt.Println()
		fmt.Printf("  %s◈ Agent Tools%s\n", brandPrimary+colorBold, colorReset)
		fmt.Printf("  %s%d tools available for agent use%s\n", colorDim, result.Count, colorReset)
		fmt.Println()
		if len(result.Tools) == 0 {
			fmt.Printf("  %sNo tools available%s\n", colorDim, colorReset)
			fmt.Println()
			fmt.Printf("  %sAdd MCP tools:%s offgrid agent mcp add <url>\n", colorDim, colorReset)
		} else {
			// Group by type
			builtInTools := []struct{ name, desc, typ string }{}
			mcpTools := []struct{ name, desc, typ string }{}

			for _, t := range result.Tools {
				item := struct{ name, desc, typ string }{t.Name, t.Description, t.Type}
				if t.Type == "mcp" || t.Type == "external" {
					mcpTools = append(mcpTools, item)
				} else {
					builtInTools = append(builtInTools, item)
				}
			}

			if len(builtInTools) > 0 {
				fmt.Printf("  %sBuilt-in Tools%s\n", colorDim, colorReset)
				// Clean short descriptions for built-in tools
				shortDescs := map[string]string{
					"read_file":    "Read file contents",
					"write_file":   "Write to a file",
					"list_files":   "List directory contents",
					"shell":        "Execute shell commands",
					"http_get":     "Make HTTP GET requests",
					"current_time": "Get current date/time",
					"calculator":   "Evaluate math expressions",
					"search":       "Search the web",
					"memory":       "Store/retrieve info",
				}
				for _, t := range builtInTools {
					desc := t.desc
					if short, ok := shortDescs[t.name]; ok {
						desc = short
					} else if len(desc) > 40 {
						// Truncate at word boundary
						desc = desc[:37]
						if idx := strings.LastIndex(desc, " "); idx > 20 {
							desc = desc[:idx]
						}
					}
					fmt.Printf("    %-16s  %s%s%s\n", t.name, colorDim, desc, colorReset)
				}
			}

			if len(mcpTools) > 0 {
				fmt.Println()
				fmt.Printf("  %sMCP Tools%s\n", colorDim, colorReset)
				for _, t := range mcpTools {
					desc := t.desc
					if len(desc) > 40 {
						desc = desc[:37]
						if idx := strings.LastIndex(desc, " "); idx > 20 {
							desc = desc[:idx]
						}
					}
					fmt.Printf("    %-16s  %s%s%s\n", t.name, colorDim, desc, colorReset)
				}
			}
		}
		fmt.Println()
		fmt.Printf("  %sAdd MCP tools:%s offgrid agent mcp add <url>\n", colorDim, colorReset)
		fmt.Println()

	case "mcp":
		// MCP server management
		handleAgentMCP(args[1:])

	default:
		printError(fmt.Sprintf("Unknown agent command: %s", subCmd))
	}
}

// handleAgentMCP manages MCP server configuration
func handleAgentMCP(args []string) {
	// Get config path
	homeDir, _ := os.UserHomeDir()
	configPath := filepath.Join(homeDir, ".offgrid-llm", "data", "tools.json")

	if len(args) < 1 {
		fmt.Println()
		fmt.Printf("  %s◈ MCP Servers%s\n", brandPrimary+colorBold, colorReset)
		fmt.Printf("  %sConnect external tools via Model Context Protocol%s\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sUsage%s  offgrid agent mcp <command>\n", colorDim, colorReset)
		fmt.Println()

		// Commands
		fmt.Printf("  %sCommands%s\n", brandPrimary, colorReset)
		fmt.Printf("    %-20s %sAdd an MCP server%s\n", "add <url>", colorDim, colorReset)
		fmt.Printf("    %-20s %sList configured servers%s\n", "list", colorDim, colorReset)
		fmt.Printf("    %-20s %sRemove a server%s\n", "remove <name>", colorDim, colorReset)
		fmt.Printf("    %-20s %sEnable a server%s\n", "enable <name>", colorDim, colorReset)
		fmt.Printf("    %-20s %sDisable a server%s\n", "disable <name>", colorDim, colorReset)
		fmt.Println()

		// Options
		fmt.Printf("  %sOptions%s\n", brandPrimary, colorReset)
		fmt.Printf("    %-20s %sServer name (auto-derived from URL)%s\n", "--name <name>", colorDim, colorReset)
		fmt.Printf("    %-20s %sAPI key for authentication%s\n", "--api-key <key>", colorDim, colorReset)
		fmt.Println()

		// Examples
		fmt.Printf("  %sExamples%s\n", brandPrimary, colorReset)
		fmt.Printf("    %s$%s offgrid agent mcp add http://localhost:3100 --name my-tools\n", colorDim, colorReset)
		fmt.Printf("    %s$%s offgrid agent mcp list\n", colorDim, colorReset)
		fmt.Printf("    %s$%s offgrid agent mcp disable my-tools\n", colorDim, colorReset)
		fmt.Println()
		return
	}

	// Load existing config
	loadConfig := func() (map[string]interface{}, error) {
		data, err := os.ReadFile(configPath)
		if err != nil {
			if os.IsNotExist(err) {
				// Create default config
				return map[string]interface{}{
					"tools":       []interface{}{},
					"mcp_servers": []interface{}{},
				}, nil
			}
			return nil, err
		}
		var cfg map[string]interface{}
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, err
		}
		return cfg, nil
	}

	// Save config
	saveConfig := func(cfg map[string]interface{}) error {
		dir := filepath.Dir(configPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		data, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return err
		}
		return os.WriteFile(configPath, data, 0644)
	}

	mcpCmd := args[0]
	switch mcpCmd {
	case "add":
		if len(args) < 2 {
			printError("Usage: offgrid agent mcp add <url> [--name <name>] [--api-key <key>]")
			return
		}

		url := args[1]
		name := ""
		apiKey := ""

		// Parse options
		for i := 2; i < len(args); i++ {
			if args[i] == "--name" && i+1 < len(args) {
				name = args[i+1]
				i++
			} else if args[i] == "--api-key" && i+1 < len(args) {
				apiKey = args[i+1]
				i++
			}
		}

		// Generate name from URL if not provided
		if name == "" {
			// Extract host from URL
			name = url
			if idx := strings.Index(url, "://"); idx != -1 {
				name = url[idx+3:]
			}
			if idx := strings.Index(name, "/"); idx != -1 {
				name = name[:idx]
			}
			if idx := strings.Index(name, ":"); idx != -1 {
				name = name[:idx]
			}
			if name == "localhost" || name == "127.0.0.1" {
				name = fmt.Sprintf("mcp-%d", time.Now().Unix()%10000)
			}
		}

		cfg, err := loadConfig()
		if err != nil {
			printError(fmt.Sprintf("Failed to load config: %v", err))
			return
		}

		// Add new MCP server
		servers, _ := cfg["mcp_servers"].([]interface{})
		newServer := map[string]interface{}{
			"name":    name,
			"url":     url,
			"enabled": true,
		}
		if apiKey != "" {
			newServer["api_key"] = apiKey
		}
		servers = append(servers, newServer)
		cfg["mcp_servers"] = servers

		if err := saveConfig(cfg); err != nil {
			printError(fmt.Sprintf("Failed to save config: %v", err))
			return
		}

		fmt.Println()
		fmt.Printf("  %s✓ Added%s %s%s%s\n", colorGreen+colorBold, colorReset, brandSecondary, name, colorReset)
		fmt.Printf("    %s%s%s\n", colorDim, url, colorReset)
		fmt.Println()
		fmt.Printf("  %sRestart server to connect:%s offgrid serve\n", colorDim, colorReset)
		fmt.Println()

	case "list":
		cfg, err := loadConfig()
		if err != nil {
			printError(fmt.Sprintf("Failed to load config: %v", err))
			return
		}

		servers, _ := cfg["mcp_servers"].([]interface{})

		fmt.Println()
		fmt.Printf("  %s◈ MCP Servers%s\n", brandPrimary+colorBold, colorReset)
		fmt.Println()

		if len(servers) == 0 {
			fmt.Printf("  %sNo servers configured%s\n", colorDim, colorReset)
			fmt.Println()
			fmt.Printf("  %sAdd one:%s offgrid agent mcp add <url>\n", colorDim, colorReset)
		} else {
			for _, s := range servers {
				srv, ok := s.(map[string]interface{})
				if !ok {
					continue
				}
				name, _ := srv["name"].(string)
				url, _ := srv["url"].(string)
				enabled, _ := srv["enabled"].(bool)

				statusIcon := "✓"
				statusColor := colorGreen
				if !enabled {
					statusIcon = "○"
					statusColor = colorDim
				}

				fmt.Printf("  %s%s%s %s%s%s\n", statusColor, statusIcon, colorReset, colorBold, name, colorReset)
				fmt.Printf("    %s%s%s\n", colorDim, url, colorReset)
			}
		}
		fmt.Println()

	case "remove":
		if len(args) < 2 {
			printError("Usage: offgrid agent mcp remove <name>")
			return
		}

		name := args[1]

		cfg, err := loadConfig()
		if err != nil {
			printError(fmt.Sprintf("Failed to load config: %v", err))
			return
		}

		servers, _ := cfg["mcp_servers"].([]interface{})
		newServers := []interface{}{}
		found := false
		for _, s := range servers {
			srv, ok := s.(map[string]interface{})
			if !ok {
				continue
			}
			if srv["name"].(string) == name {
				found = true
				continue
			}
			newServers = append(newServers, srv)
		}

		if !found {
			printError(fmt.Sprintf("MCP server not found: %s", name))
			return
		}

		cfg["mcp_servers"] = newServers
		if err := saveConfig(cfg); err != nil {
			printError(fmt.Sprintf("Failed to save config: %v", err))
			return
		}

		fmt.Println()
		fmt.Printf("  %s✓ Removed%s %s%s%s\n", colorGreen+colorBold, colorReset, brandSecondary, name, colorReset)
		fmt.Println()
		fmt.Printf("  %s→ Restart the server to apply:%s offgrid serve\n", colorDim, colorReset)
		fmt.Println()

	case "enable", "disable":
		if len(args) < 2 {
			printError(fmt.Sprintf("Usage: offgrid agent mcp %s <name>", mcpCmd))
			return
		}

		name := args[1]
		enable := mcpCmd == "enable"

		cfg, err := loadConfig()
		if err != nil {
			printError(fmt.Sprintf("Failed to load config: %v", err))
			return
		}

		servers, _ := cfg["mcp_servers"].([]interface{})
		found := false
		for i, s := range servers {
			srv, ok := s.(map[string]interface{})
			if !ok {
				continue
			}
			if srv["name"].(string) == name {
				srv["enabled"] = enable
				servers[i] = srv
				found = true
				break
			}
		}

		if !found {
			printError(fmt.Sprintf("MCP server not found: %s", name))
			return
		}

		cfg["mcp_servers"] = servers
		if err := saveConfig(cfg); err != nil {
			printError(fmt.Sprintf("Failed to save config: %v", err))
			return
		}

		action := "Enabled"
		if !enable {
			action = "Disabled"
		}
		fmt.Println()
		fmt.Printf("%sMCP Server %s:%s %s\n", colorGreen+colorBold, action, colorReset, name)
		fmt.Println()
		fmt.Printf("%sRestart the server to apply:%s offgrid serve\n", colorDim, colorReset)
		fmt.Println()

	default:
		printError(fmt.Sprintf("Unknown mcp command: %s", mcpCmd))
	}
}

// handleAudio handles audio commands for TTS/ASR
func handleAudio(args []string) {
	// Check for help flag
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		args = []string{} // Trigger help display
	}

	if len(args) < 1 {
		fmt.Println()
		fmt.Printf("  %s◈ Audio%s\n", brandPrimary+colorBold, colorReset)
		fmt.Printf("  %sSpeech-to-text and text-to-speech%s\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sUsage%s  offgrid audio <command> [options]\n", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sCommands%s\n", brandPrimary, colorReset)
		fmt.Printf("    %-24s %sTranscribe audio to text%s\n", "transcribe <file>", colorDim, colorReset)
		fmt.Printf("    %-24s %sConvert text to speech%s\n", "speak <text>", colorDim, colorReset)
		fmt.Printf("    %-24s %sShow audio system status%s\n", "status", colorDim, colorReset)
		fmt.Printf("    %-24s %sList audio models%s\n", "models", colorDim, colorReset)
		fmt.Printf("    %-24s %sDownload Whisper model%s\n", "setup whisper", colorDim, colorReset)
		fmt.Printf("    %-24s %sDownload Piper voice%s\n", "setup piper", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sOptions%s\n", brandPrimary, colorReset)
		fmt.Printf("    %-24s %stiny, base, small, medium, large%s\n", "--model <name>", colorDim, colorReset)
		fmt.Printf("    %-24s %sPiper voice name%s\n", "--voice <name>", colorDim, colorReset)
		fmt.Printf("    %-24s %sOutput file for TTS%s\n", "--output <file>", colorDim, colorReset)
		fmt.Printf("    %-24s %sLanguage code for ASR%s\n", "--language <code>", colorDim, colorReset)
		fmt.Println()
		fmt.Printf("  %sExamples%s\n", brandPrimary, colorReset)
		fmt.Printf("    %s$%s offgrid audio transcribe recording.wav\n", colorDim, colorReset)
		fmt.Printf("    %s$%s offgrid audio speak \"Hello!\" --output hello.wav\n", colorDim, colorReset)
		fmt.Printf("    %s$%s offgrid audio setup whisper --model base.en\n", colorDim, colorReset)
		fmt.Println()
		return
	}

	cfg := config.LoadConfig()
	baseURL := fmt.Sprintf("http://localhost:%d", cfg.ServerPort)

	subCmd := args[0]
	switch subCmd {
	case "transcribe":
		if len(args) < 2 {
			printError("Usage: offgrid audio transcribe <audio-file> [--model <name>] [--language <code>]")
			return
		}

		audioFile := args[1]
		model := "base.en"
		language := ""

		// Parse options
		for i := 2; i < len(args); i++ {
			switch args[i] {
			case "--model", "-m":
				if i+1 < len(args) {
					model = args[i+1]
					i++
				}
			case "--language", "-l":
				if i+1 < len(args) {
					language = args[i+1]
					i++
				}
			}
		}

		// Check if file exists
		if _, err := os.Stat(audioFile); os.IsNotExist(err) {
			printError(fmt.Sprintf("Audio file not found: %s", audioFile))
			return
		}

		fmt.Println()
		printSection("Transcribing Audio")
		fmt.Printf("  %sFile:%s %s\n", colorDim, colorReset, audioFile)
		fmt.Printf("  %sModel:%s %s\n", colorDim, colorReset, model)
		if language != "" {
			fmt.Printf("  %sLanguage:%s %s\n", colorDim, colorReset, language)
		}
		fmt.Println()

		// Read the audio file
		audioData, err := os.ReadFile(audioFile)
		if err != nil {
			printError(fmt.Sprintf("Failed to read audio file: %v", err))
			return
		}

		// Create multipart form request
		body := &bytes.Buffer{}
		writer := NewMultipartWriter(body)

		part, err := writer.CreateFormFile("file", filepath.Base(audioFile))
		if err != nil {
			printError(fmt.Sprintf("Failed to create form: %v", err))
			return
		}
		part.Write(audioData)

		if model != "" {
			writer.WriteField("model", model)
		}
		if language != "" {
			writer.WriteField("language", language)
		}
		writer.Close()

		req, err := http.NewRequest("POST", baseURL+"/v1/audio/transcriptions", body)
		if err != nil {
			printError(fmt.Sprintf("Failed to create request: %v", err))
			return
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())

		client := &http.Client{Timeout: 300 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			printError(fmt.Sprintf("Failed to connect to server: %v", err))
			printInfo("Make sure the server is running: offgrid serve")
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			printError(fmt.Sprintf("Transcription failed: %s", string(respBody)))
			return
		}

		var result struct {
			Text string `json:"text"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			printError(fmt.Sprintf("Failed to parse response: %v", err))
			return
		}

		printSuccess("Transcription Complete")
		fmt.Println()
		fmt.Println(result.Text)
		fmt.Println()

	case "speak":
		if len(args) < 2 {
			printError("Usage: offgrid audio speak <text> [--voice <name>] [--output <file>]")
			return
		}

		text := args[1]
		voice := "en_US-amy-medium"
		outputFile := "output.wav"
		speed := 1.0

		// Parse options
		for i := 2; i < len(args); i++ {
			switch args[i] {
			case "--voice", "-v":
				if i+1 < len(args) {
					voice = args[i+1]
					i++
				}
			case "--output", "-o":
				if i+1 < len(args) {
					outputFile = args[i+1]
					i++
				}
			case "--speed", "-s":
				if i+1 < len(args) {
					fmt.Sscanf(args[i+1], "%f", &speed)
					i++
				}
			}
		}

		fmt.Println()
		printSection("Generating Speech")
		fmt.Printf("  %sText:%s %s\n", colorDim, colorReset, text)
		fmt.Printf("  %sVoice:%s %s\n", colorDim, colorReset, voice)
		fmt.Printf("  %sOutput:%s %s\n", colorDim, colorReset, outputFile)
		fmt.Println()

		// Create request
		reqBody := map[string]interface{}{
			"input": text,
			"model": voice,
			"voice": voice,
			"speed": speed,
		}
		jsonBody, _ := json.Marshal(reqBody)

		req, err := http.NewRequest("POST", baseURL+"/v1/audio/speech", bytes.NewReader(jsonBody))
		if err != nil {
			printError(fmt.Sprintf("Failed to create request: %v", err))
			return
		}
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 60 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			printError(fmt.Sprintf("Failed to connect to server: %v", err))
			printInfo("Make sure the server is running: offgrid serve")
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			printError(fmt.Sprintf("Speech generation failed: %s", string(respBody)))
			return
		}

		// Save audio to file
		outFile, err := os.Create(outputFile)
		if err != nil {
			printError(fmt.Sprintf("Failed to create output file: %v", err))
			return
		}
		defer outFile.Close()

		written, err := io.Copy(outFile, resp.Body)
		if err != nil {
			printError(fmt.Sprintf("Failed to write audio: %v", err))
			return
		}

		printSuccess(fmt.Sprintf("Speech generated: %s (%d bytes)", outputFile, written))
		fmt.Println()

	case "status":
		fmt.Println()
		printSection("Audio System Status")

		resp, err := http.Get(baseURL + "/v1/audio/status")
		if err != nil {
			printError(fmt.Sprintf("Failed to connect to server: %v", err))
			printInfo("Make sure the server is running: offgrid serve")
			return
		}
		defer resp.Body.Close()

		var status map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
			printError(fmt.Sprintf("Failed to parse response: %v", err))
			return
		}

		// ASR Status
		asr, _ := status["asr"].(map[string]interface{})
		asrAvailable, _ := asr["available"].(bool)
		asrPath, _ := asr["whisper_path"].(string)
		asrModels, _ := asr["models"].([]interface{})

		fmt.Println()
		fmt.Printf("  %sSpeech-to-Text (ASR)%s\n", brandPrimary+colorBold, colorReset)
		if asrAvailable {
			fmt.Printf("    %s%s Available%s\n", colorGreen, iconCheck, colorReset)
			fmt.Printf("    %sWhisper:%s %s\n", colorDim, colorReset, asrPath)
			fmt.Printf("    %sModels:%s %d installed\n", colorDim, colorReset, len(asrModels))
		} else {
			fmt.Printf("    %s%s Not Available%s\n", colorRed, iconCross, colorReset)
			fmt.Printf("    %sRun: offgrid audio setup whisper%s\n", colorDim, colorReset)
		}

		// TTS Status
		tts, _ := status["tts"].(map[string]interface{})
		ttsAvailable, _ := tts["available"].(bool)
		ttsPath, _ := tts["piper_path"].(string)
		ttsVoices, _ := tts["voices"].(float64)

		fmt.Println()
		fmt.Printf("  %sText-to-Speech (TTS)%s\n", brandPrimary+colorBold, colorReset)
		if ttsAvailable {
			fmt.Printf("    %s%s Available%s\n", colorGreen, iconCheck, colorReset)
			fmt.Printf("    %sPiper:%s %s\n", colorDim, colorReset, ttsPath)
			fmt.Printf("    %sVoices:%s %d installed\n", colorDim, colorReset, int(ttsVoices))
		} else {
			fmt.Printf("    %s%s Not Available%s\n", colorRed, iconCross, colorReset)
			fmt.Printf("    %sRun: offgrid audio setup piper%s\n", colorDim, colorReset)
		}

		dataDir, _ := status["data_dir"].(string)
		fmt.Println()
		fmt.Printf("  %sData Directory:%s %s\n", colorDim, colorReset, dataDir)
		fmt.Println()

	case "models":
		fmt.Println()
		printSection("Audio Models")

		resp, err := http.Get(baseURL + "/v1/audio/models")
		if err != nil {
			printError(fmt.Sprintf("Failed to connect to server: %v", err))
			printInfo("Make sure the server is running: offgrid serve")
			return
		}
		defer resp.Body.Close()

		var models map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&models); err != nil {
			printError(fmt.Sprintf("Failed to parse response: %v", err))
			return
		}

		// Whisper models
		whisper, _ := models["whisper"].(map[string]interface{})
		installed, _ := whisper["installed"].([]interface{})
		available, _ := whisper["available"].([]interface{})

		fmt.Println()
		fmt.Printf("  %sWhisper Models (ASR)%s\n", brandPrimary+colorBold, colorReset)
		fmt.Printf("  %sInstalled:%s\n", colorDim, colorReset)
		if len(installed) == 0 {
			fmt.Printf("    %sNone%s\n", colorDim, colorReset)
		} else {
			for _, m := range installed {
				fmt.Printf("    %s%s%s %s\n", colorGreen, iconCheck, colorReset, m)
			}
		}
		fmt.Printf("  %sAvailable for download:%s\n", colorDim, colorReset)
		for _, m := range available {
			if model, ok := m.(map[string]interface{}); ok {
				name, _ := model["Name"].(string)
				size, _ := model["Size"].(string)
				fmt.Printf("    %s%s%s (%s)\n", brandSecondary, name, colorReset, size)
			}
		}

		// Piper voices
		piper, _ := models["piper"].(map[string]interface{})
		piperInstalled, _ := piper["installed"].([]interface{})
		piperAvailable, _ := piper["available"].([]interface{})

		fmt.Println()
		fmt.Printf("  %sPiper Voices (TTS)%s\n", brandPrimary+colorBold, colorReset)
		fmt.Printf("  %sInstalled:%s\n", colorDim, colorReset)
		if len(piperInstalled) == 0 {
			fmt.Printf("    %sNone%s\n", colorDim, colorReset)
		} else {
			for _, v := range piperInstalled {
				if voice, ok := v.(map[string]interface{}); ok {
					name, _ := voice["name"].(string)
					fmt.Printf("    %s%s%s %s\n", colorGreen, iconCheck, colorReset, name)
				}
			}
		}
		fmt.Printf("  %sAvailable for download:%s\n", colorDim, colorReset)
		for _, v := range piperAvailable {
			if voice, ok := v.(map[string]interface{}); ok {
				name, _ := voice["Name"].(string)
				lang, _ := voice["Language"].(string)
				fmt.Printf("    %s%s%s (%s)\n", brandSecondary, name, colorReset, lang)
			}
		}
		fmt.Println()

	case "setup":
		if len(args) < 2 {
			printError("Usage: offgrid audio setup <whisper|piper> [--model/--voice <name>]")
			return
		}

		setupType := args[1]
		name := ""

		// Parse options
		for i := 2; i < len(args); i++ {
			switch args[i] {
			case "--model", "--voice", "-m", "-v":
				if i+1 < len(args) {
					name = args[i+1]
					i++
				}
			}
		}

		switch setupType {
		case "whisper":
			if name == "" {
				name = "base.en"
			}

			// Check if model exists
			modelInfo, ok := audio.WhisperModels[name]
			if !ok {
				printError(fmt.Sprintf("Unknown whisper model: %s", name))
				fmt.Println()
				fmt.Println("Available models:")
				for n, m := range audio.WhisperModels {
					fmt.Printf("  %s%s%s (%s)\n", brandPrimary, n, colorReset, m.Size)
				}
				return
			}

			fmt.Println()
			fmt.Printf("%sDownloading Whisper Model%s\n", brandPrimary+colorBold, colorReset)
			fmt.Printf("%s%s%s (%s)\n", brandMuted, name, colorReset, modelInfo.Size)
			fmt.Println()

			// Create audio engine with empty config (uses defaults)
			engine, err := audio.NewEngine(audio.Config{})
			if err != nil {
				printError(fmt.Sprintf("Failed to initialize audio engine: %v", err))
				return
			}

			// Download with progress (nil uses default progress printer)
			if err := engine.DownloadWhisperModel(name, nil); err != nil {
				printError(fmt.Sprintf("Download failed: %v", err))
				return
			}

			printSuccess(fmt.Sprintf("Whisper model '%s' installed", name))
			fmt.Println()

		case "piper":
			if name == "" {
				name = "en_US-amy-medium"
			}

			// Check if voice exists
			voiceInfo, ok := audio.PiperVoices[name]
			if !ok {
				printError(fmt.Sprintf("Unknown piper voice: %s", name))
				fmt.Println()
				fmt.Println("Available voices:")
				for n, v := range audio.PiperVoices {
					fmt.Printf("  %s%s%s (%s, %s)\n", brandPrimary, n, colorReset, v.Language, v.Quality)
				}
				return
			}

			fmt.Println()
			fmt.Printf("%sDownloading Piper Voice%s\n", brandPrimary+colorBold, colorReset)
			fmt.Printf("%s%s%s (%s, %s)\n", brandMuted, name, colorReset, voiceInfo.Language, voiceInfo.Quality)
			fmt.Println()

			// Create audio engine with empty config (uses defaults)
			engine, err := audio.NewEngine(audio.Config{})
			if err != nil {
				printError(fmt.Sprintf("Failed to initialize audio engine: %v", err))
				return
			}

			// Download piper binary if not present
			if !engine.HasPiperBinary() {
				fmt.Println()
				fmt.Printf("%sDownloading Piper binary...%s\n", colorDim, colorReset)
				if err := engine.DownloadPiperBinary(nil); err != nil {
					printError(fmt.Sprintf("Failed to download Piper: %v", err))
					return
				}
				printSuccess("Piper binary installed")
			}

			// Download with progress (nil uses default progress printer)
			if err := engine.DownloadPiperVoice(name, nil); err != nil {
				printError(fmt.Sprintf("Download failed: %v", err))
				return
			}

			printSuccess(fmt.Sprintf("Piper voice '%s' installed", name))
			fmt.Println()

		default:
			printError(fmt.Sprintf("Unknown setup type: %s (use 'whisper' or 'piper')", setupType))
		}

	default:
		printError(fmt.Sprintf("Unknown audio command: %s", subCmd))
		fmt.Println()
		fmt.Println("Commands: transcribe, speak, status, models, setup")
	}
}

// MultipartWriter wraps the multipart writer
type MultipartWriter struct {
	*multipartWriterImpl
}

type multipartWriterImpl struct {
	w        io.Writer
	boundary string
	parts    []byte
}

func NewMultipartWriter(w io.Writer) *MultipartWriter {
	boundary := fmt.Sprintf("----OffGridBoundary%d", time.Now().UnixNano())
	return &MultipartWriter{
		&multipartWriterImpl{
			w:        w,
			boundary: boundary,
		},
	}
}

func (m *MultipartWriter) FormDataContentType() string {
	return "multipart/form-data; boundary=" + m.boundary
}

func (m *MultipartWriter) CreateFormFile(fieldname, filename string) (io.Writer, error) {
	header := fmt.Sprintf("--%s\r\nContent-Disposition: form-data; name=\"%s\"; filename=\"%s\"\r\nContent-Type: application/octet-stream\r\n\r\n",
		m.boundary, fieldname, filename)
	m.w.Write([]byte(header))
	return m.w, nil
}

func (m *MultipartWriter) WriteField(fieldname, value string) error {
	header := fmt.Sprintf("\r\n--%s\r\nContent-Disposition: form-data; name=\"%s\"\r\n\r\n%s",
		m.boundary, fieldname, value)
	_, err := m.w.Write([]byte(header))
	return err
}

func (m *MultipartWriter) Close() error {
	_, err := m.w.Write([]byte(fmt.Sprintf("\r\n--%s--\r\n", m.boundary)))
	return err
}
