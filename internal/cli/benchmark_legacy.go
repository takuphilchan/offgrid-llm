package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/models"
)

func benchmarkServerHealthy(ctx context.Context, client *http.Client, baseURL string) bool {
	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(checkCtx, http.MethodGet, baseURL+"/health", nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func benchmarkPost(ctx context.Context, client *http.Client, endpoint, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	return client.Do(req)
}

func benchmarkFormatDuration(d time.Duration) string {
	if d < time.Microsecond {
		return fmt.Sprintf("%dns", d.Nanoseconds())
	}
	if d < time.Millisecond {
		return fmt.Sprintf("%.2fµs", float64(d.Microseconds()))
	}
	if d < time.Second {
		return fmt.Sprintf("%.2fms", float64(d.Milliseconds()))
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}

func runBenchmarkLegacy(ctx context.Context, args []string, env Environment) error {
	s := env.Style
	brandPrimary, colorBold, colorReset := s.Primary, s.Bold, s.Reset
	colorDim, brandMuted, brandError := s.Dim, s.Muted, s.Error
	colorRed, colorGreen := s.Error, s.Success
	presenter := Presenter{Stdout: env.Stdout, Stderr: env.Stderr, Style: s}
	printError := presenter.Error

	// Check for help flag
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		args = []string{} // Trigger help display
	}

	if len(args) < 1 {
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %s◈ Benchmark Model%s\n", brandPrimary+colorBold, colorReset)
		fmt.Fprintf(env.Stdout, "  %sMeasure model performance and resource usage%s\n", colorDim, colorReset)
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sUsage%s  offgrid benchmark <model-id> [options]\n", colorDim, colorReset)
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sOptions%s\n", brandPrimary, colorReset)
		fmt.Fprintf(env.Stdout, "    %-22s %sNumber of iterations (default: 3)%s\n", "--iterations N", colorDim, colorReset)
		fmt.Fprintf(env.Stdout, "    %-22s %sCustom prompt to benchmark%s\n", "--prompt \"text\"", colorDim, colorReset)
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sExamples%s\n", brandPrimary, colorReset)
		fmt.Fprintf(env.Stdout, "    %s$%s offgrid benchmark tinyllama-1.1b-chat.Q4_K_M\n", colorDim, colorReset)
		fmt.Fprintf(env.Stdout, "    %s$%s offgrid benchmark llama-2-7b-chat.Q5_K_M --iterations 5\n", colorDim, colorReset)
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sNote:%s Server must be running first: %soffgrid serve%s\n", brandMuted, colorReset, colorBold, colorReset)
		fmt.Fprintln(env.Stdout)
		return ReportedError{}
	}

	cfg, cfgErr := localConfig(env)
	if cfgErr != nil {
		return cfgErr
	}
	registry := models.NewRegistry(cfg.ModelsDir)

	if err := registry.ScanModels(); err != nil {
		fmt.Fprintf(env.Stderr, "✗ Error scanning models: %v\n\n", err)
		return ReportedError{}
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
		fmt.Fprintf(env.Stderr, "✗ Model not found: %s\n\n", modelID)

		// Show available models
		modelList := registry.ListModels()
		if len(modelList) > 0 {
			fmt.Fprintln(env.Stderr, "Available models:")
			for _, m := range modelList {
				fmt.Fprintf(env.Stderr, "  • %s\n", m.ID)
			}
			fmt.Fprintln(env.Stderr, "")
		} else {
			fmt.Fprintln(env.Stderr, "No models installed. Use 'offgrid download' to add models.")
			fmt.Fprintln(env.Stderr, "")
		}
		return ReportedError{}
	}

	// Print benchmark header
	fmt.Fprintln(env.Stdout)
	fmt.Fprintf(env.Stdout, "%sBenchmark · %s%s\n", brandPrimary+colorBold, modelID, colorReset)
	fmt.Fprintln(env.Stdout, "")
	fmt.Fprintf(env.Stdout, "%sPath:%s %s\n", colorDim, colorReset, meta.Path)
	fmt.Fprintf(env.Stdout, "%sSize:%s %s", colorDim, colorReset, FormatBytes(meta.Size))
	if meta.Quantization != "" {
		fmt.Fprintf(env.Stdout, " · %s", meta.Quantization)
	}
	fmt.Fprintln(env.Stdout)
	fmt.Fprintln(env.Stdout)

	// Check if server is running
	serverURL := fmt.Sprintf("http://127.0.0.1:%d", cfg.ServerPort)
	if !benchmarkServerHealthy(ctx, env.HTTP, serverURL) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		fmt.Fprintln(env.Stdout)
		printError("Server not running")
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sStart the server first:%s\n", colorBold, colorReset)
		fmt.Fprintf(env.Stdout, "    %s$%s offgrid serve\n", brandMuted, colorReset)
		fmt.Fprintf(env.Stdout, "    %s$%s sudo systemctl start offgrid-llm\n", brandMuted, colorReset)
		fmt.Fprintln(env.Stdout)
		return ReportedError{}
	}

	// Default benchmark prompt
	benchPrompt := "Write a short story about a robot learning to paint."
	if customPrompt != "" {
		benchPrompt = customPrompt
	}

	fmt.Fprintf(env.Stdout, "Running %d iterations...\n\n", iterations)

	// Run benchmark iterations
	var (
		totalLatency      time.Duration
		totalTokens       int
		totalPromptTokens int
		firstTokenTimes   []time.Duration
		tokensPerSec      []float64
	)

	for i := 0; i < iterations; i++ {
		fmt.Fprintf(env.Stdout, "%s  [%d/%d]%s Testing... ", colorDim, i+1, iterations, colorReset)

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
		resp, err := benchmarkPost(ctx, env.HTTP,
			serverURL+"/v1/completions",
			"application/json",
			bytes.NewBuffer(jsonData),
		)

		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			fmt.Fprintf(env.Stdout, "%s✗%s\n", colorRed, colorReset)
			continue
		}

		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)

		var result map[string]interface{}
		if err := json.Unmarshal(body, &result); err != nil {
			fmt.Fprintf(env.Stdout, "%s✗%s Failed to parse response\n", brandError, colorReset)
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

		fmt.Fprintf(env.Stdout, "%s✓%s %s (%.1f tok/s)\n", colorGreen, colorReset, benchmarkFormatDuration(latency), tps)
	}

	if len(tokensPerSec) == 0 {
		printError("All benchmark iterations failed")
		return ReportedError{}
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
	fmt.Fprintln(env.Stdout)
	fmt.Fprintf(env.Stdout, "  %sResults%s\n", brandPrimary, colorReset)
	fmt.Fprintln(env.Stdout)
	fmt.Fprintf(env.Stdout, "    Speed       %s%.1f tok/s%s %s(%.1f - %.1f)%s\n", colorBold, avgTPS, colorReset, colorDim, minTPS, maxTPS, colorReset)
	fmt.Fprintf(env.Stdout, "    Latency     %s%s%s %s(first token: ~%s)%s\n", colorBold, benchmarkFormatDuration(avgLatency), colorReset, colorDim, benchmarkFormatDuration(avgFirstToken), colorReset)
	fmt.Fprintf(env.Stdout, "    Tokens      %s%.0f prompt, %.0f generated%s %s(avg)%s\n", colorBold, avgPromptTokens, avgTokens, colorReset, colorDim, colorReset)
	fmt.Fprintf(env.Stdout, "    Throughput  %s~%d queries/hour%s\n", colorBold, int(3600.0/avgLatency.Seconds()), colorReset)

	memEst := float64(meta.Size) * 1.2 // Rough estimate: model + context
	fmt.Fprintf(env.Stdout, "    Memory      %s%.1f GB estimated%s\n", colorDim, memEst/1e9, colorReset)
	fmt.Fprintln(env.Stdout)
	return nil
}

func runBenchmarkCompareLegacy(ctx context.Context, args []string, env Environment) error {
	s := env.Style
	brandPrimary, colorBold, colorReset := s.Primary, s.Bold, s.Reset
	colorDim, brandMuted := s.Dim, s.Muted
	colorRed, colorGreen, colorYellow := s.Error, s.Success, s.Accent
	presenter := Presenter{Stdout: env.Stdout, Stderr: env.Stderr, Style: s}
	printError := presenter.Error

	if len(args) < 2 {
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "%sBenchmark Compare%s\n", brandPrimary+colorBold, colorReset)
		fmt.Fprintln(env.Stdout, "")
		fmt.Fprintf(env.Stdout, "%sUsage:%s offgrid benchmark-compare <model1> <model2> [model3...] [--iterations N]\n", colorDim, colorReset)
		fmt.Fprintln(env.Stdout, "")
		fmt.Fprintln(env.Stdout, "Compare performance across multiple models")
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "%sOptions%s\n", brandPrimary+colorBold, colorReset)
		presenter.Option("--iterations N", "Number of test iterations (default: 3)")
		presenter.Option("--prompt \"text\"", "Custom prompt for comparison")
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "%sExamples%s\n", brandPrimary+colorBold, colorReset)
		presenter.Example("offgrid compare tinyllama-1.1b phi-2")
		presenter.Example("offgrid compare llama-2-7b mistral-7b phi-2 --iterations 5")
		fmt.Fprintln(env.Stdout)
		return ReportedError{}
	}

	cfg, cfgErr := localConfig(env)
	if cfgErr != nil {
		return cfgErr
	}
	registry := models.NewRegistry(cfg.ModelsDir)

	if err := registry.ScanModels(); err != nil {
		fmt.Fprintf(env.Stderr, "✗ Error scanning models: %v\n\n", err)
		return ReportedError{}
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
		return ReportedError{}
	}

	// Check if server is running
	serverURL := fmt.Sprintf("http://127.0.0.1:%d", cfg.ServerPort)
	if !benchmarkServerHealthy(ctx, env.HTTP, serverURL) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		fmt.Fprintln(env.Stdout)
		printError("Server not running")
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sStart the server first:%s\n", colorBold, colorReset)
		fmt.Fprintf(env.Stdout, "    %s$%s offgrid serve\n", brandMuted, colorReset)
		fmt.Fprintf(env.Stdout, "    %s$%s sudo systemctl start offgrid-llm\n", brandMuted, colorReset)
		fmt.Fprintln(env.Stdout)
		return ReportedError{}
	}

	// Default benchmark prompt
	benchPrompt := "Write a short story about a robot learning to paint."
	if customPrompt != "" {
		benchPrompt = customPrompt
	}

	fmt.Fprintln(env.Stdout)
	fmt.Fprintf(env.Stdout, "%sBenchmark Comparison%s\n", brandPrimary+colorBold, colorReset)
	fmt.Fprintf(env.Stdout, "Comparing %d models with %d iterations each\n", len(modelIDs), iterations)

	// Test if inference backend is working with first model
	testPayload := fmt.Sprintf(`{"model":"%s","prompt":"test","max_tokens":1}`, modelIDs[0])
	testResp, testErr := benchmarkPost(ctx, env.HTTP,
		serverURL+"/v1/completions",
		"application/json",
		bytes.NewBuffer([]byte(testPayload)),
	)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if testErr == nil {
		defer testResp.Body.Close()
		testBody, _ := io.ReadAll(testResp.Body)
		var testResult map[string]interface{}
		if json.Unmarshal(testBody, &testResult) == nil {
			if errObj, ok := testResult["error"].(map[string]interface{}); ok {
				if msg, ok := errObj["message"].(string); ok {
					if strings.Contains(msg, "llama-server") || strings.Contains(msg, "connection refused") {
						fmt.Fprintln(env.Stdout, "")
						fmt.Fprintf(env.Stdout, "%s⚠ Warning:%s llama-server backend not running\n", colorYellow, colorReset)
						fmt.Fprintf(env.Stdout, "%sBenchmark will show API latency only (no actual inference)%s\n", colorDim, colorReset)
					}
				}
			}
		}
	}
	fmt.Fprintln(env.Stdout)

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
		fmt.Fprintf(env.Stdout, "%sTesting: %s%s\n", brandPrimary, modelID, colorReset)

		meta, err := registry.GetModel(modelID)
		if err != nil {
			fmt.Fprintf(env.Stdout, "%s✗ Model not found%s\n", colorRed, colorReset)
			results = append(results, BenchResult{ModelID: modelID, Failed: true})
			continue
		}

		var (
			totalLatency time.Duration
			tokensPerSec []float64
		)

		for i := 0; i < iterations; i++ {
			fmt.Fprintf(env.Stdout, "[%d/%d] ", i+1, iterations)

			startTime := time.Now()

			reqBody := map[string]interface{}{
				"model":       modelID,
				"prompt":      benchPrompt,
				"max_tokens":  100,
				"temperature": 0.7,
				"stream":      false,
			}

			jsonData, _ := json.Marshal(reqBody)
			resp, err := benchmarkPost(ctx, env.HTTP,
				serverURL+"/v1/completions",
				"application/json",
				bytes.NewBuffer(jsonData),
			)

			if err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				fmt.Fprintf(env.Stdout, "%s✗%s\n", colorRed, colorReset)
				continue
			}

			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			var result map[string]interface{}
			if err := json.Unmarshal(body, &result); err != nil {
				fmt.Fprintf(env.Stdout, "%s✗%s Parse error\n", colorRed, colorReset)
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
				fmt.Fprintf(env.Stdout, "%s✗%s %s\n", colorRed, colorReset, errMsg)
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
				fmt.Fprintf(env.Stdout, "%s✗%s No tokens generated\n", colorRed, colorReset)
				continue
			}

			totalLatency += latency
			tps := float64(tokenCount) / latency.Seconds()
			tokensPerSec = append(tokensPerSec, tps)

			fmt.Fprintf(env.Stdout, "%s✓%s %.1f tok/s\n", colorGreen, colorReset, tps)
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

		fmt.Fprintf(env.Stdout, "%sAverage: %.1f tok/s%s\n", colorDim, avgTPS, colorReset)
		fmt.Fprintln(env.Stdout, "")
	}

	// Display comparison table
	fmt.Fprintf(env.Stdout, "%sComparison Results%s\n", brandPrimary+colorBold, colorReset)
	fmt.Fprintln(env.Stdout)

	// Sort by speed (descending)
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].AvgSpeed > results[i].AvgSpeed {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	// Print header
	fmt.Fprintf(env.Stdout, "   %s%-30s  %10s  %12s  %10s  %8s%s\n",
		colorDim, "Model", "Speed", "Latency", "Range", "Size", colorReset)

	fastest := results[0].AvgSpeed
	for i, r := range results {
		if r.Failed {
			fmt.Fprintf(env.Stdout, "   %-30s  %s%10s%s\n", r.ModelID, colorRed, "FAILED", colorReset)
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

		sizeStr := FormatBytes(r.Size)
		if r.Quant != "" {
			sizeStr += " " + r.Quant
		}

		fmt.Fprintf(env.Stdout, "   %s%-30s  %s%8.1f%s t/s  %10s  %4.0f-%4.0f  %10s\n",
			rank, r.ModelID,
			speedColor, r.AvgSpeed, colorReset,
			benchmarkFormatDuration(r.AvgLatency),
			r.MinSpeed, r.MaxSpeed,
			sizeStr)
	}

	fmt.Fprintln(env.Stdout)
	fmt.Fprintf(env.Stdout, "   %s★ = Fastest model%s\n", colorDim, colorReset)
	fmt.Fprintln(env.Stdout)
	return nil
}

// handleAudit handles audit log commands
