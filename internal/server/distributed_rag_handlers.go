package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/takuphilchan/offgrid-llm/internal/rag"
)

// handleDistributedRAG returns distributed RAG status
func (s *Server) handleDistributedRAG(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if s.distributedRAG == nil {
		json.NewEncoder(w).Encode(map[string]any{
			"enabled": false,
			"message": "Distributed RAG not configured",
		})
		return
	}

	stats := s.distributedRAG.GetStats()
	stats["enabled"] = true
	json.NewEncoder(w).Encode(stats)
}

// handleDistributedRAGNodes handles distributed RAG node operations
func (s *Server) handleDistributedRAGNodes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if s.distributedRAG == nil {
		http.Error(w, `{"error": "distributed RAG not configured"}`, http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodGet:
		nodes := s.distributedRAG.ListNodes()
		json.NewEncoder(w).Encode(map[string]any{
			"nodes": nodes,
		})

	case http.MethodPost:
		var node rag.DistributedNode
		if err := json.NewDecoder(r.Body).Decode(&node); err != nil {
			http.Error(w, `{"error": "invalid request"}`, http.StatusBadRequest)
			return
		}
		s.distributedRAG.AddNode(node)
		json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"node_id": node.ID,
		})

	case http.MethodDelete:
		var req struct {
			NodeID string `json:"node_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error": "invalid request"}`, http.StatusBadRequest)
			return
		}
		if err := s.distributedRAG.RemoveNode(req.NodeID); err != nil {
			http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"node_id": req.NodeID,
		})

	default:
		http.Error(w, `{"error": "method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

// handleDistributedRAGSearch performs a distributed search
func (s *Server) handleDistributedRAGSearch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, `{"error": "method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Query string `json:"query"`
		TopK  int    `json:"top_k"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "invalid request"}`, http.StatusBadRequest)
		return
	}

	if req.Query == "" {
		http.Error(w, `{"error": "query required"}`, http.StatusBadRequest)
		return
	}

	if req.TopK <= 0 {
		req.TopK = 10
	}

	// If distributed RAG is not configured, fall back to local search
	if s.distributedRAG == nil {
		// Fall back to local RAG search
		if s.ragEngine == nil || !s.ragEngine.IsEnabled() {
			http.Error(w, `{"error": "RAG not configured or not enabled"}`, http.StatusNotFound)
			return
		}
		ragCtx, err := s.ragEngine.Search(r.Context(), req.Query, rag.SearchOptions{
			TopK:           req.TopK,
			MinScore:       0.0,
			IncludeContent: true,
		})
		if err != nil {
			json.NewEncoder(w).Encode(map[string]any{
				"success": false,
				"error":   err.Error(),
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"success":      true,
			"chunks":       ragCtx.Results,
			"total_chunks": len(ragCtx.Results),
			"distributed":  false,
		})
		return
	}

	// Distributed search
	localSearch := func(query string, topK int) ([]rag.ChunkResult, error) {
		if s.ragEngine == nil || !s.ragEngine.IsEnabled() {
			return nil, nil
		}
		ragCtx, err := s.ragEngine.Search(r.Context(), query, rag.SearchOptions{
			TopK:           topK,
			MinScore:       0.0,
			IncludeContent: true,
		})
		if err != nil {
			return nil, err
		}
		// Convert to ChunkResult
		chunks := make([]rag.ChunkResult, len(ragCtx.Results))
		for i, res := range ragCtx.Results {
			chunkID := ""
			content := ""
			if res.Chunk != nil {
				chunkID = res.Chunk.ID
				content = res.Chunk.Content
			}
			chunks[i] = rag.ChunkResult{
				DocumentID: res.DocumentID,
				ChunkID:    chunkID,
				Content:    content,
				Score:      float64(res.Score),
				Metadata:   res.Metadata,
			}
		}
		return chunks, nil
	}

	result, err := s.distributedRAG.Search(r.Context(), req.Query, req.TopK, localSearch)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]any{
		"success":        true,
		"chunks":         result.Chunks,
		"total_chunks":   result.TotalChunks,
		"search_time_ms": result.SearchTimeMS,
		"nodes_queried":  result.NodesQueried,
		"node_results":   result.NodeResults,
		"distributed":    true,
	})
}
