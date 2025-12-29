// Package rag provides distributed RAG index functionality
package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"sync"
	"time"
)

// DistributedNode represents a node in the distributed RAG cluster
type DistributedNode struct {
	ID          string    `json:"id"`
	URL         string    `json:"url"`
	Name        string    `json:"name"`
	Healthy     bool      `json:"healthy"`
	LastCheck   time.Time `json:"last_check"`
	DocCount    int       `json:"doc_count"`
	ChunkCount  int       `json:"chunk_count"`
	IndexSizeMB float64   `json:"index_size_mb"`
}

// DistributedSearchResult represents a search result from a distributed search
type DistributedSearchResult struct {
	Chunks       []ChunkResult      `json:"chunks"`
	TotalChunks  int                `json:"total_chunks"`
	SearchTimeMS int64              `json:"search_time_ms"`
	NodesQueried int                `json:"nodes_queried"`
	NodeResults  []NodeSearchResult `json:"node_results"`
}

// NodeSearchResult represents search results from a single node
type NodeSearchResult struct {
	NodeID       string        `json:"node_id"`
	Chunks       []ChunkResult `json:"chunks"`
	SearchTimeMS int64         `json:"search_time_ms"`
	Error        string        `json:"error,omitempty"`
}

// ChunkResult represents a single chunk search result
type ChunkResult struct {
	DocumentID string            `json:"document_id"`
	ChunkID    string            `json:"chunk_id"`
	Content    string            `json:"content"`
	Score      float64           `json:"score"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	SourceNode string            `json:"source_node,omitempty"`
}

// DistributedRAG manages distributed RAG search across multiple nodes
type DistributedRAG struct {
	nodes          map[string]*DistributedNode
	localNodeID    string
	searchPath     string
	healthPath     string
	timeout        time.Duration
	healthCheckInt time.Duration
	mu             sync.RWMutex
	stopCh         chan struct{}
	httpClient     *http.Client
}

// DistributedRAGConfig contains configuration for distributed RAG
type DistributedRAGConfig struct {
	LocalNodeID        string `json:"local_node_id"`
	SearchPath         string `json:"search_path"`
	HealthPath         string `json:"health_path"`
	TimeoutSeconds     int    `json:"timeout_seconds"`
	HealthCheckSeconds int    `json:"health_check_seconds"`
}

// NewDistributedRAG creates a new distributed RAG manager
func NewDistributedRAG(config DistributedRAGConfig) *DistributedRAG {
	dr := &DistributedRAG{
		nodes:          make(map[string]*DistributedNode),
		localNodeID:    config.LocalNodeID,
		searchPath:     config.SearchPath,
		healthPath:     config.HealthPath,
		timeout:        time.Duration(config.TimeoutSeconds) * time.Second,
		healthCheckInt: time.Duration(config.HealthCheckSeconds) * time.Second,
		stopCh:         make(chan struct{}),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	if dr.localNodeID == "" {
		dr.localNodeID = "local"
	}
	if dr.searchPath == "" {
		dr.searchPath = "/v1/rag/search"
	}
	if dr.healthPath == "" {
		dr.healthPath = "/v1/rag/stats"
	}
	if dr.timeout <= 0 {
		dr.timeout = 10 * time.Second
	}
	if dr.healthCheckInt <= 0 {
		dr.healthCheckInt = 30 * time.Second
	}

	// Start health check loop
	go dr.healthCheckLoop()

	return dr
}

// AddNode adds a node to the distributed cluster
func (dr *DistributedRAG) AddNode(node DistributedNode) {
	dr.mu.Lock()
	defer dr.mu.Unlock()

	node.Healthy = true
	dr.nodes[node.ID] = &node
	log.Printf("Added distributed RAG node: %s (%s)", node.ID, node.URL)
}

// RemoveNode removes a node from the cluster
func (dr *DistributedRAG) RemoveNode(nodeID string) error {
	dr.mu.Lock()
	defer dr.mu.Unlock()

	if _, exists := dr.nodes[nodeID]; !exists {
		return fmt.Errorf("node %s not found", nodeID)
	}
	delete(dr.nodes, nodeID)
	log.Printf("Removed distributed RAG node: %s", nodeID)
	return nil
}

// Search performs a distributed search across all healthy nodes
func (dr *DistributedRAG) Search(ctx context.Context, query string, topK int, localSearch func(string, int) ([]ChunkResult, error)) (*DistributedSearchResult, error) {
	startTime := time.Now()

	dr.mu.RLock()
	healthyNodes := make([]*DistributedNode, 0)
	for _, node := range dr.nodes {
		if node.Healthy {
			healthyNodes = append(healthyNodes, node)
		}
	}
	dr.mu.RUnlock()

	result := &DistributedSearchResult{
		NodesQueried: len(healthyNodes) + 1, // +1 for local
	}

	// Channel to collect results
	resultsCh := make(chan NodeSearchResult, len(healthyNodes)+1)
	var wg sync.WaitGroup

	// Local search
	wg.Add(1)
	go func() {
		defer wg.Done()
		localStart := time.Now()
		chunks, err := localSearch(query, topK)
		nr := NodeSearchResult{
			NodeID:       dr.localNodeID,
			SearchTimeMS: time.Since(localStart).Milliseconds(),
		}
		if err != nil {
			nr.Error = err.Error()
		} else {
			for i := range chunks {
				chunks[i].SourceNode = dr.localNodeID
			}
			nr.Chunks = chunks
		}
		resultsCh <- nr
	}()

	// Remote searches
	for _, node := range healthyNodes {
		wg.Add(1)
		go func(n *DistributedNode) {
			defer wg.Done()
			nr := dr.searchNode(ctx, n, query, topK)
			resultsCh <- nr
		}(node)
	}

	// Wait for all searches to complete
	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	// Collect results
	allChunks := make([]ChunkResult, 0)
	for nr := range resultsCh {
		result.NodeResults = append(result.NodeResults, nr)
		if nr.Error == "" {
			allChunks = append(allChunks, nr.Chunks...)
		}
	}

	// Sort by score (descending) and take top K
	sort.Slice(allChunks, func(i, j int) bool {
		return allChunks[i].Score > allChunks[j].Score
	})

	if len(allChunks) > topK {
		allChunks = allChunks[:topK]
	}

	result.Chunks = allChunks
	result.TotalChunks = len(allChunks)
	result.SearchTimeMS = time.Since(startTime).Milliseconds()

	return result, nil
}

// searchNode searches a remote node
func (dr *DistributedRAG) searchNode(ctx context.Context, node *DistributedNode, query string, topK int) NodeSearchResult {
	startTime := time.Now()
	nr := NodeSearchResult{
		NodeID: node.ID,
	}

	// Build request
	searchURL := node.URL + dr.searchPath
	reqBody := fmt.Sprintf(`{"query":"%s","top_k":%d}`, query, topK)

	req, err := http.NewRequestWithContext(ctx, "POST", searchURL,
		http.NoBody)
	if err != nil {
		nr.Error = err.Error()
		nr.SearchTimeMS = time.Since(startTime).Milliseconds()
		return nr
	}

	// Use a custom body since http.NewRequestWithContext doesn't support body
	req, err = http.NewRequestWithContext(ctx, "POST", searchURL,
		&stringReader{s: reqBody})
	if err != nil {
		nr.Error = err.Error()
		nr.SearchTimeMS = time.Since(startTime).Milliseconds()
		return nr
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := dr.httpClient.Do(req)
	if err != nil {
		nr.Error = err.Error()
		nr.SearchTimeMS = time.Since(startTime).Milliseconds()
		return nr
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		nr.Error = fmt.Sprintf("node returned status %d", resp.StatusCode)
		nr.SearchTimeMS = time.Since(startTime).Milliseconds()
		return nr
	}

	// Parse response
	var searchResp struct {
		Chunks []ChunkResult `json:"chunks"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		nr.Error = err.Error()
		nr.SearchTimeMS = time.Since(startTime).Milliseconds()
		return nr
	}

	// Mark source node
	for i := range searchResp.Chunks {
		searchResp.Chunks[i].SourceNode = node.ID
	}

	nr.Chunks = searchResp.Chunks
	nr.SearchTimeMS = time.Since(startTime).Milliseconds()
	return nr
}

// stringReader implements io.Reader for a string
type stringReader struct {
	s string
	i int
}

func (r *stringReader) Read(p []byte) (n int, err error) {
	if r.i >= len(r.s) {
		return 0, io.EOF
	}
	n = copy(p, r.s[r.i:])
	r.i += n
	return n, nil
}

// healthCheckLoop periodically checks node health
func (dr *DistributedRAG) healthCheckLoop() {
	ticker := time.NewTicker(dr.healthCheckInt)
	defer ticker.Stop()

	for {
		select {
		case <-dr.stopCh:
			return
		case <-ticker.C:
			dr.checkAllNodes()
		}
	}
}

// checkAllNodes checks health of all nodes
func (dr *DistributedRAG) checkAllNodes() {
	dr.mu.RLock()
	nodes := make([]*DistributedNode, 0, len(dr.nodes))
	for _, n := range dr.nodes {
		nodes = append(nodes, n)
	}
	dr.mu.RUnlock()

	var wg sync.WaitGroup
	for _, n := range nodes {
		wg.Add(1)
		go func(node *DistributedNode) {
			defer wg.Done()
			dr.checkNodeHealth(node)
		}(n)
	}
	wg.Wait()
}

// checkNodeHealth checks if a node is healthy and updates its stats
func (dr *DistributedRAG) checkNodeHealth(node *DistributedNode) {
	healthURL := node.URL + dr.healthPath

	resp, err := dr.httpClient.Get(healthURL)
	if err != nil {
		dr.mu.Lock()
		node.Healthy = false
		node.LastCheck = time.Now()
		dr.mu.Unlock()
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		dr.mu.Lock()
		node.Healthy = false
		node.LastCheck = time.Now()
		dr.mu.Unlock()
		return
	}

	// Parse stats
	var stats struct {
		Documents   int     `json:"documents"`
		Chunks      int     `json:"chunks"`
		IndexSizeMB float64 `json:"index_size_mb"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&stats); err == nil {
		dr.mu.Lock()
		node.DocCount = stats.Documents
		node.ChunkCount = stats.Chunks
		node.IndexSizeMB = stats.IndexSizeMB
		dr.mu.Unlock()
	}

	dr.mu.Lock()
	node.Healthy = true
	node.LastCheck = time.Now()
	dr.mu.Unlock()
}

// GetStats returns distributed RAG statistics
func (dr *DistributedRAG) GetStats() map[string]interface{} {
	dr.mu.RLock()
	defer dr.mu.RUnlock()

	nodes := make([]map[string]interface{}, 0, len(dr.nodes))
	healthyCount := 0
	totalDocs := 0
	totalChunks := 0

	for _, n := range dr.nodes {
		nodes = append(nodes, map[string]interface{}{
			"id":            n.ID,
			"url":           n.URL,
			"name":          n.Name,
			"healthy":       n.Healthy,
			"doc_count":     n.DocCount,
			"chunk_count":   n.ChunkCount,
			"index_size_mb": n.IndexSizeMB,
			"last_check":    n.LastCheck,
		})
		if n.Healthy {
			healthyCount++
			totalDocs += n.DocCount
			totalChunks += n.ChunkCount
		}
	}

	return map[string]interface{}{
		"local_node_id":   dr.localNodeID,
		"total_nodes":     len(dr.nodes),
		"healthy_nodes":   healthyCount,
		"total_documents": totalDocs,
		"total_chunks":    totalChunks,
		"nodes":           nodes,
	}
}

// ListNodes returns all configured nodes
func (dr *DistributedRAG) ListNodes() []DistributedNode {
	dr.mu.RLock()
	defer dr.mu.RUnlock()

	nodes := make([]DistributedNode, 0, len(dr.nodes))
	for _, n := range dr.nodes {
		nodes = append(nodes, *n)
	}
	return nodes
}

// Stop stops the distributed RAG manager
func (dr *DistributedRAG) Stop() {
	close(dr.stopCh)
}

// SyncIndex triggers index synchronization across nodes
// This is a placeholder for future implementation of index replication
func (dr *DistributedRAG) SyncIndex(ctx context.Context) error {
	// Future: Implement index synchronization
	// - Push new documents to replica nodes
	// - Pull updates from primary nodes
	// - Handle conflict resolution
	log.Printf("Index sync requested (not yet implemented)")
	return nil
}

// RebalanceShards redistributes documents across nodes
// This is a placeholder for future implementation of sharding
func (dr *DistributedRAG) RebalanceShards(ctx context.Context) error {
	// Future: Implement shard rebalancing
	// - Analyze current distribution
	// - Move documents between nodes
	// - Update routing table
	log.Printf("Shard rebalancing requested (not yet implemented)")
	return nil
}
