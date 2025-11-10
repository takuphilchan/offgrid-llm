package batch

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Request represents a single batch request
type Request struct {
	ID      string                 `json:"id"`
	Model   string                 `json:"model"`
	Prompt  string                 `json:"prompt"`
	Options map[string]interface{} `json:"options,omitempty"`
}

// Result represents the result of a batch request
type Result struct {
	ID           string        `json:"id"`
	Model        string        `json:"model"`
	Prompt       string        `json:"prompt"`
	Response     string        `json:"response"`
	Error        string        `json:"error,omitempty"`
	Duration     time.Duration `json:"duration_ms"`
	TokensPerSec float64       `json:"tokens_per_sec,omitempty"`
}

// Processor handles batch processing of prompts
type Processor struct {
	engine      inference.Engine
	concurrency int
}

// NewProcessor creates a new batch processor
func NewProcessor(engine inference.Engine, concurrency int) *Processor {
	if concurrency < 1 {
		concurrency = 1
	}
	return &Processor{
		engine:      engine,
		concurrency: concurrency,
	}
}

// ProcessFile reads requests from a JSONL file and processes them
func (p *Processor) ProcessFile(ctx context.Context, inputPath, outputPath string) error {
	// Open input file
	inputFile, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open input file: %w", err)
	}
	defer inputFile.Close()

	// Create output file
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outputFile.Close()

	// Read requests
	requests := make([]*Request, 0)
	scanner := bufio.NewScanner(inputFile)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		var req Request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			return fmt.Errorf("invalid JSON at line %d: %w", lineNum, err)
		}

		// Set default ID if not provided
		if req.ID == "" {
			req.ID = fmt.Sprintf("req-%d", lineNum)
		}

		requests = append(requests, &req)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading input file: %w", err)
	}

	fmt.Printf("Loaded %d requests from %s\n", len(requests), inputPath)
	fmt.Printf("Processing with concurrency=%d\n\n", p.concurrency)

	// Process requests
	results := p.Process(ctx, requests)

	// Write results
	encoder := json.NewEncoder(outputFile)
	for _, result := range results {
		if err := encoder.Encode(result); err != nil {
			return fmt.Errorf("failed to write result: %w", err)
		}
	}

	return nil
}

// Process executes a batch of requests with concurrency control
func (p *Processor) Process(ctx context.Context, requests []*Request) []*Result {
	results := make([]*Result, len(requests))
	resultsMu := sync.Mutex{}

	// Create worker pool
	requestChan := make(chan struct {
		req   *Request
		index int
	}, len(requests))

	// Feed requests
	for i, req := range requests {
		requestChan <- struct {
			req   *Request
			index int
		}{req, i}
	}
	close(requestChan)

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < p.concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for item := range requestChan {
				select {
				case <-ctx.Done():
					return
				default:
					result := p.processRequest(ctx, item.req)
					resultsMu.Lock()
					results[item.index] = result
					resultsMu.Unlock()

					// Print progress
					fmt.Printf("[Worker %d] Processed %s: %v\n", workerID, result.ID, result.Error == "")
				}
			}
		}(i)
	}

	wg.Wait()
	return results
}

// processRequest handles a single request
func (p *Processor) processRequest(ctx context.Context, req *Request) *Result {
	start := time.Now()

	result := &Result{
		ID:     req.ID,
		Model:  req.Model,
		Prompt: req.Prompt,
	}

	// Create inference request
	apiReq := &api.CompletionRequest{
		Model:  req.Model,
		Prompt: req.Prompt,
	}

	// Apply options if provided
	if req.Options != nil {
		if temp, ok := req.Options["temperature"].(float64); ok {
			temp32 := float32(temp)
			apiReq.Temperature = &temp32
		}
		if maxTokens, ok := req.Options["max_tokens"].(float64); ok {
			maxTokensInt := int(maxTokens)
			apiReq.MaxTokens = &maxTokensInt
		}
		if topP, ok := req.Options["top_p"].(float64); ok {
			topP32 := float32(topP)
			apiReq.TopP = &topP32
		}
	}

	// Execute inference
	response, err := p.engine.Completion(ctx, apiReq)
	if err != nil {
		result.Error = err.Error()
		result.Duration = time.Since(start)
		return result
	}

	// Extract text from response
	if len(response.Choices) > 0 {
		result.Response = response.Choices[0].Text
	}
	result.Duration = time.Since(start)

	// Calculate tokens per second if available
	if response.Usage.TotalTokens > 0 {
		result.TokensPerSec = float64(response.Usage.TotalTokens) / result.Duration.Seconds()
	}

	return result
}

// ProcessStream processes requests and writes results as they complete
func (p *Processor) ProcessStream(ctx context.Context, input io.Reader, output io.Writer) error {
	scanner := bufio.NewScanner(input)
	encoder := json.NewEncoder(output)
	lineNum := 0

	// Create a channel for requests
	requestChan := make(chan *Request, p.concurrency*2)
	resultChan := make(chan *Result, p.concurrency*2)

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < p.concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for req := range requestChan {
				result := p.processRequest(ctx, req)
				resultChan <- result
			}
		}(i)
	}

	// Writer goroutine
	done := make(chan struct{})
	go func() {
		for result := range resultChan {
			encoder.Encode(result)
		}
		close(done)
	}()

	// Read and dispatch
	for scanner.Scan() {
		lineNum++
		var req Request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			return fmt.Errorf("invalid JSON at line %d: %w", lineNum, err)
		}

		if req.ID == "" {
			req.ID = fmt.Sprintf("req-%d", lineNum)
		}

		select {
		case <-ctx.Done():
			close(requestChan)
			wg.Wait()
			close(resultChan)
			<-done
			return ctx.Err()
		case requestChan <- &req:
		}
	}

	close(requestChan)
	wg.Wait()
	close(resultChan)
	<-done

	return scanner.Err()
}
