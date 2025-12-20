package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/rag"
)

// RAG API request/response types

// IngestTextRequest is the request for ingesting text
type IngestTextRequest struct {
	Name     string            `json:"name"`
	Content  string            `json:"content"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// IngestResponse is the response after ingesting a document
type IngestResponse struct {
	Success  bool          `json:"success"`
	Document *rag.Document `json:"document,omitempty"`
	Message  string        `json:"message,omitempty"`
	Error    string        `json:"error,omitempty"`
}

// SearchRequest is the request for searching documents
type SearchRequest struct {
	Query          string   `json:"query"`
	TopK           int      `json:"top_k,omitempty"`
	MinScore       float32  `json:"min_score,omitempty"`
	DocumentFilter []string `json:"document_filter,omitempty"`
}

// SearchResponse is the response for search
type SearchResponse struct {
	Query   string             `json:"query"`
	Results []rag.SearchResult `json:"results"`
	Context string             `json:"context,omitempty"`
}

// DocumentListResponse is the response for listing documents
type DocumentListResponse struct {
	Documents []*rag.Document `json:"documents"`
	Count     int             `json:"count"`
}

// RAGStatusResponse is the response for RAG status
type RAGStatusResponse struct {
	Enabled        bool                   `json:"enabled"`
	EmbeddingModel string                 `json:"embedding_model,omitempty"`
	Stats          map[string]interface{} `json:"stats"`
}

// EnableRAGRequest is the request to enable RAG
type EnableRAGRequest struct {
	EmbeddingModel string `json:"embedding_model"`
}

// handleRAGStatus returns the status of the RAG engine
func (s *Server) handleRAGStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.ragEngine == nil {
		json.NewEncoder(w).Encode(RAGStatusResponse{
			Enabled: false,
			Stats:   map[string]interface{}{"error": "RAG engine not initialized"},
		})
		return
	}

	stats := s.ragEngine.Stats()
	embeddingModel := ""
	if model, ok := stats["embedding_model"].(string); ok {
		embeddingModel = model
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(RAGStatusResponse{
		Enabled:        s.ragEngine.IsEnabled(),
		EmbeddingModel: embeddingModel,
		Stats:          stats,
	})
}

// handleRAGEnable enables the RAG engine
func (s *Server) handleRAGEnable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req EnableRAGRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.EmbeddingModel == "" {
		http.Error(w, "embedding_model is required", http.StatusBadRequest)
		return
	}

	if s.ragEngine == nil {
		http.Error(w, "RAG engine not initialized", http.StatusInternalServerError)
		return
	}

	if err := s.ragEngine.Enable(r.Context(), req.EmbeddingModel); err != nil {
		log.Printf("Failed to enable RAG: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "RAG enabled with model: " + req.EmbeddingModel,
	})
}

// handleRAGDisable disables the RAG engine
func (s *Server) handleRAGDisable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.ragEngine != nil {
		s.ragEngine.Disable()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "RAG disabled",
	})
}

// handleDocumentsList lists all documents
func (s *Server) handleDocumentsList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.ragEngine == nil {
		http.Error(w, "RAG engine not initialized", http.StatusInternalServerError)
		return
	}

	docs := s.ragEngine.ListDocuments()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(DocumentListResponse{
		Documents: docs,
		Count:     len(docs),
	})
}

// handleDocumentIngest ingests a new document
func (s *Server) handleDocumentIngest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.ragEngine == nil {
		http.Error(w, "RAG engine not initialized", http.StatusInternalServerError)
		return
	}

	if !s.ragEngine.IsEnabled() {
		http.Error(w, "RAG is not enabled. Call POST /v1/rag/enable first", http.StatusBadRequest)
		return
	}

	contentType := r.Header.Get("Content-Type")

	var doc *rag.Document
	var err error

	if strings.HasPrefix(contentType, "multipart/form-data") {
		// Handle file upload
		doc, err = s.handleFileUpload(r)
	} else {
		// Handle JSON text ingestion
		var req IngestTextRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}
		if req.Content == "" {
			http.Error(w, "content is required", http.StatusBadRequest)
			return
		}

		doc, err = s.ragEngine.IngestText(r.Context(), req.Name, req.Content, req.Metadata)
	}

	if err != nil {
		log.Printf("Failed to ingest document: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(IngestResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(IngestResponse{
		Success:  true,
		Document: doc,
		Message:  "Document ingested successfully",
	})
}

// handleFileUpload handles multipart file uploads
func (s *Server) handleFileUpload(r *http.Request) (*rag.Document, error) {
	// Limit upload size to 50MB
	r.ParseMultipartForm(50 << 20)

	file, handler, err := r.FormFile("file")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	metadata := map[string]string{
		"original_filename": handler.Filename,
		"content_type":      handler.Header.Get("Content-Type"),
	}

	// Get file extension and check if it needs parsing
	ext := strings.ToLower(filepath.Ext(handler.Filename))

	// Check if file extension is supported
	if !rag.IsSupportedExtension(ext) {
		return nil, fmt.Errorf("unsupported file type: %s. Supported: PDF, DOCX, XLSX, PPTX, RTF, TXT, MD, JSON, CSV, XML, HTML, and code files", ext)
	}

	// Use the document parser to extract text from binary files
	parser := rag.NewDocumentParser()
	result, err := parser.Parse(content, handler.Filename, ext)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s file: %w", ext, err)
	}

	// Add parser metadata
	metadata["file_ext"] = ext
	metadata["content_type"] = result.ContentType
	for k, v := range result.Metadata {
		metadata[k] = v
	}

	return s.ragEngine.IngestText(r.Context(), handler.Filename, result.Content, metadata)
}

// handleDocumentDelete deletes a document
func (s *Server) handleDocumentDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.ragEngine == nil {
		http.Error(w, "RAG engine not initialized", http.StatusInternalServerError)
		return
	}

	// Get document ID from query or body
	docID := r.URL.Query().Get("id")
	if docID == "" {
		var body struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
			docID = body.ID
		}
	}

	if docID == "" {
		http.Error(w, "Document ID is required", http.StatusBadRequest)
		return
	}

	if s.ragEngine.DeleteDocument(docID) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Document deleted",
		})
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Document not found",
		})
	}
}

// handleDocumentSearch searches documents
func (s *Server) handleDocumentSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.ragEngine == nil {
		http.Error(w, "RAG engine not initialized", http.StatusInternalServerError)
		return
	}

	if !s.ragEngine.IsEnabled() {
		http.Error(w, "RAG is not enabled", http.StatusBadRequest)
		return
	}

	var req SearchRequest

	if r.Method == http.MethodGet {
		req.Query = r.URL.Query().Get("query")
	} else {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
	}

	if req.Query == "" {
		http.Error(w, "query is required", http.StatusBadRequest)
		return
	}

	opts := rag.DefaultSearchOptions()
	if req.TopK > 0 {
		opts.TopK = req.TopK
	}
	if req.MinScore > 0 {
		opts.MinScore = req.MinScore
	}
	if len(req.DocumentFilter) > 0 {
		opts.DocumentFilter = req.DocumentFilter
	}

	ragContext, err := s.ragEngine.Search(r.Context(), req.Query, opts)
	if err != nil {
		log.Printf("Search failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(SearchResponse{
		Query:   req.Query,
		Results: ragContext.Results,
		Context: ragContext.Context,
	})
}
