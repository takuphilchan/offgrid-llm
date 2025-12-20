package main

import (
	"context"
	"fmt"
	"log"
	"math"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Example demonstrating embedding functionality in OffGrid LLM
func embedMain() {
	fmt.Println("=== OffGrid LLM - Embeddings Demo ===")

	// 1. Create embedding engine
	fmt.Println("1. Creating Embedding Engine")
	fmt.Println("────────────────────────────────────")
	engine := inference.NewEmbeddingEngine()
	if engine == nil {
		log.Fatal("Failed to create embedding engine")
	}
	fmt.Println("[OK] Embedding engine created")
	fmt.Println()

	// 2. Load embedding model
	fmt.Println("2. Loading Embedding Model")
	fmt.Println("────────────────────────────────────")

	// Note: Replace with actual model path
	modelPath := "/var/lib/offgrid/models/all-minilm-l6-v2.gguf"
	fmt.Printf("Model: %s\n", modelPath)

	ctx := context.Background()
	opts := inference.DefaultEmbeddingOptions()

	// For demo purposes with stub, this will work
	err := engine.Load(ctx, modelPath, opts)
	if err != nil {
		log.Printf("Note: Using stub implementation for demo")
	}

	info := engine.GetModelInfo()
	fmt.Printf("[OK] Model loaded\n")
	fmt.Printf("  Dimensions: %d\n", info["dimensions"])
	fmt.Printf("  Batch size: %d\n", info["batch_size"])
	fmt.Println()

	// 3. Generate single embedding
	fmt.Println("3. Single Text Embedding")
	fmt.Println("────────────────────────────────────")

	singleText := "Hello, this is a test sentence for embedding generation."
	req := &api.EmbeddingRequest{
		Model: "all-minilm-l6-v2",
		Input: singleText,
	}

	response, err := engine.GenerateEmbeddings(ctx, req)
	if err != nil {
		log.Fatalf("Failed to generate embedding: %v", err)
	}

	fmt.Printf("Input: \"%s\"\n", singleText)
	fmt.Printf("[OK] Generated embedding\n")
	fmt.Printf("  Dimensions: %d\n", len(response.Data[0].Embedding))
	fmt.Printf("  First 5 values: [%.3f, %.3f, %.3f, %.3f, %.3f]\n",
		response.Data[0].Embedding[0],
		response.Data[0].Embedding[1],
		response.Data[0].Embedding[2],
		response.Data[0].Embedding[3],
		response.Data[0].Embedding[4])
	fmt.Printf("  Tokens used: %d\n", response.Usage.TotalTokens)
	fmt.Println()

	// 4. Batch embedding generation
	fmt.Println("4. Batch Embedding Generation")
	fmt.Println("────────────────────────────────────")

	texts := []interface{}{
		"The ship's engine requires maintenance.",
		"Maritime diesel engines need regular service.",
		"Weather forecast shows clear skies tomorrow.",
		"How to check battery voltage levels.",
	}

	batchReq := &api.EmbeddingRequest{
		Model: "all-minilm-l6-v2",
		Input: texts,
	}

	batchResponse, err := engine.GenerateEmbeddings(ctx, batchReq)
	if err != nil {
		log.Fatalf("Failed to generate batch embeddings: %v", err)
	}

	fmt.Printf("Generated %d embeddings\n", len(batchResponse.Data))
	for i, text := range texts {
		fmt.Printf("  %d. \"%s\"\n", i+1, text)
	}
	fmt.Printf("[OK] Total tokens: %d\n", batchResponse.Usage.TotalTokens)
	fmt.Println()

	// 5. Semantic Similarity
	fmt.Println("5. Semantic Similarity Calculation")
	fmt.Println("────────────────────────────────────")

	// Get embeddings for comparison
	emb1 := batchResponse.Data[0].Embedding
	emb2 := batchResponse.Data[1].Embedding
	emb3 := batchResponse.Data[2].Embedding

	// Calculate cosine similarities
	sim_1_2 := cosineSimilarity(emb1, emb2)
	sim_1_3 := cosineSimilarity(emb1, emb3)
	sim_2_3 := cosineSimilarity(emb2, emb3)

	fmt.Println("Similarity scores (0 = different, 1 = identical):")
	fmt.Printf("  Text 1 vs Text 2 (similar topic): %.3f\n", sim_1_2)
	fmt.Printf("  Text 1 vs Text 3 (different topic): %.3f\n", sim_1_3)
	fmt.Printf("  Text 2 vs Text 3 (different topic): %.3f\n", sim_2_3)
	fmt.Println()

	// 6. Semantic Search Demo
	fmt.Println("6. Semantic Search Demo")
	fmt.Println("────────────────────────────────────")

	// Create a simple document collection
	documents := []string{
		"Engine oil change procedure: Drain old oil, replace filter, add new oil",
		"Fuel system maintenance: Clean fuel filter every 500 hours",
		"Battery maintenance: Check electrolyte levels monthly",
		"Navigation equipment calibration procedures",
		"Emergency shutdown procedures for main engine",
	}

	// Embed all documents
	docReq := &api.EmbeddingRequest{
		Model: "all-minilm-l6-v2",
		Input: convertToInterfaceSlice(documents),
	}

	docResponse, err := engine.GenerateEmbeddings(ctx, docReq)
	if err != nil {
		log.Fatalf("Failed to embed documents: %v", err)
	}

	// Search query
	query := "How do I maintain the engine?"
	queryReq := &api.EmbeddingRequest{
		Model: "all-minilm-l6-v2",
		Input: query,
	}

	queryResponse, err := engine.GenerateEmbeddings(ctx, queryReq)
	if err != nil {
		log.Fatalf("Failed to embed query: %v", err)
	}

	queryEmb := queryResponse.Data[0].Embedding

	// Find most similar documents
	type SearchResult struct {
		Index int
		Score float64
		Text  string
	}

	results := make([]SearchResult, len(documents))
	for i, docEmb := range docResponse.Data {
		score := cosineSimilarity(queryEmb, docEmb.Embedding)
		results[i] = SearchResult{
			Index: i,
			Score: score,
			Text:  documents[i],
		}
	}

	// Sort by score (simple bubble sort for demo)
	for i := 0; i < len(results)-1; i++ {
		for j := 0; j < len(results)-i-1; j++ {
			if results[j].Score < results[j+1].Score {
				results[j], results[j+1] = results[j+1], results[j]
			}
		}
	}

	fmt.Printf("Query: \"%s\"\n\n", query)
	fmt.Println("Top 3 most relevant documents:")
	for i := 0; i < 3 && i < len(results); i++ {
		fmt.Printf("  %d. Score %.3f: %s\n", i+1, results[i].Score, results[i].Text)
	}
	fmt.Println()

	// 7. Summary
	fmt.Println("=== Summary ===")
	fmt.Println("Demonstrated features:")
	fmt.Println("[OK] Single text embedding generation")
	fmt.Println("[OK] Batch embedding generation")
	fmt.Println("[OK] Semantic similarity calculation")
	fmt.Println("[OK] Semantic search over documents")
	fmt.Println()
	fmt.Println("Use cases:")
	fmt.Println("  • Document similarity and clustering")
	fmt.Println("  • Semantic search in knowledge bases")
	fmt.Println("  • RAG (Retrieval Augmented Generation)")
	fmt.Println("  • Duplicate detection")
	fmt.Println("  • Content recommendation")
	fmt.Println()
	fmt.Println("All operations performed 100% offline!")
}

// cosineSimilarity calculates the cosine similarity between two vectors
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := 0; i < len(a); i++ {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// convertToInterfaceSlice converts []string to []interface{}
func convertToInterfaceSlice(strings []string) []interface{} {
	result := make([]interface{}, len(strings))
	for i, s := range strings {
		result[i] = s
	}
	return result
}
