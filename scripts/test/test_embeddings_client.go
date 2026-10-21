//go:build ignore
// +build ignore

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type EmbeddingRequest struct {
	Model string      `json:"model"`
	Input interface{} `json:"input"`
}

type EmbeddingResponse struct {
	Object string          `json:"object"`
	Data   []EmbeddingData `json:"data"`
	Model  string          `json:"model"`
	Usage  EmbeddingUsage  `json:"usage"`
}

type EmbeddingData struct {
	Object    string    `json:"object"`
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
}

type EmbeddingUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

func main() {
	// Test embeddings endpoint
	fmt.Println("🧪 Testing Embeddings Endpoint")
	fmt.Println("===============================\n")

	baseURL := "http://localhost:11611"

	// Test 1: Single string
	fmt.Println("Test 1: Single text embedding")
	req := EmbeddingRequest{
		Model: "test-model",
		Input: "Hello, this is a test!",
	}
	testEmbedding(baseURL, req)

	time.Sleep(1 * time.Second)

	// Test 2: Array of strings
	fmt.Println("\nTest 2: Batch embeddings")
	req2 := EmbeddingRequest{
		Model: "test-model",
		Input: []string{"First text", "Second text", "Third text"},
	}
	testEmbedding(baseURL, req2)

	fmt.Println("\n[OK] Tests complete!")
}

func testEmbedding(baseURL string, req EmbeddingRequest) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		fmt.Printf("[ERROR] Error marshaling request: %v\n", err)
		return
	}

	resp, err := http.Post(
		baseURL+"/v1/embeddings",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		fmt.Printf("[ERROR] Error making request: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("[ERROR] Error reading response: %v\n", err)
		return
	}

	fmt.Printf("Status: %d\n", resp.StatusCode)

	// Try to parse as embedding response
	var embResp EmbeddingResponse
	if err := json.Unmarshal(body, &embResp); err != nil {
		fmt.Printf("Response body: %s\n", string(body))
		return
	}

	fmt.Printf("[OK] Object: %s\n", embResp.Object)
	fmt.Printf("[OK] Model: %s\n", embResp.Model)
	fmt.Printf("[OK] Embeddings count: %d\n", len(embResp.Data))
	if len(embResp.Data) > 0 {
		fmt.Printf("[OK] Dimensions: %d\n", len(embResp.Data[0].Embedding))
		fmt.Printf("[OK] First 5 values: %v\n", embResp.Data[0].Embedding[:5])
	}
	fmt.Printf("[OK] Total tokens: %d\n", embResp.Usage.TotalTokens)
}
