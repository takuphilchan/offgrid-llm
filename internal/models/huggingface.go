package models

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// HuggingFaceClient handles HuggingFace Hub API interactions
type HuggingFaceClient struct {
	client  *http.Client
	baseURL string
}

// HFModel represents a model from HuggingFace Hub
type HFModel struct {
	ID            string    `json:"id"`                      // e.g., "TheBloke/Llama-2-7B-Chat-GGUF"
	ModelID       string    `json:"modelId"`                 // Alternative field name
	Downloads     int64     `json:"downloads"`               // Download count
	Likes         int       `json:"likes"`                   // Like count
	Tags          []string  `json:"tags"`                    // Model tags
	CreatedAt     time.Time `json:"createdAt"`               // Creation date
	LastModified  time.Time `json:"lastModified"`            // Last update
	Private       bool      `json:"private"`                 // Is private
	Gated         bool      `json:"gated,omitempty"`         // Requires approval (optional)
	LibraryName   string    `json:"library_name"`            // e.g., "transformers", "gguf"
	PipelineTag   string    `json:"pipeline_tag"`            // e.g., "text-generation"
	Siblings      []HFFile  `json:"siblings,omitempty"`      // Files in the repo (only in detailed view)
	CardData      HFCard    `json:"cardData,omitempty"`      // Model card metadata (optional)
	Author        string    `json:"author,omitempty"`        // Model author (optional)
	Description   string    `json:"description,omitempty"`   // Short description (optional)
	Disabled      bool      `json:"disabled,omitempty"`      // Is disabled (optional)
	SHA           string    `json:"sha,omitempty"`           // Commit SHA (optional)
	TrendingScore float64   `json:"trendingScore,omitempty"` // Trending score (optional)
}

// HFFile represents a file in a HuggingFace model repo
type HFFile struct {
	Filename string `json:"rfilename"` // Relative filename
	Size     int64  `json:"size"`      // File size in bytes
}

// HFCard represents model card metadata
type HFCard struct {
	Language     []string               `json:"language"`
	License      string                 `json:"license"`
	ModelType    string                 `json:"model_type"`
	Quantization string                 `json:"quantization_config"`
	BaseModel    interface{}            `json:"base_model"` // Can be string or []string
	Tags         []string               `json:"tags"`
	Datasets     []string               `json:"datasets"`
	Metrics      map[string]interface{} `json:"model-index"`
}

// SearchFilter contains search and filter options
type SearchFilter struct {
	Query          string   // Search query
	Tags           []string // Filter by tags (e.g., "gguf", "llama", "q4_k_m")
	Author         string   // Filter by author
	MinDownloads   int64    // Minimum download count
	MinLikes       int      // Minimum like count
	MaxSize        int64    // Maximum file size in bytes
	MinSize        int64    // Minimum file size in bytes
	Quantization   string   // Filter by quantization (e.g., "Q4_K_M")
	SortBy         string   // Sort by: "downloads", "likes", "created", "modified"
	Limit          int      // Max results to return
	OnlyGGUF       bool     // Only return GGUF models
	ExcludeGated   bool     // Exclude gated models
	ExcludePrivate bool     // Exclude private models
}

// SearchResult represents a search result with computed metrics
type SearchResult struct {
	Model         HFModel
	GGUFFiles     []GGUFFileInfo
	TotalSize     int64
	Score         float64 // Computed relevance score
	BestVariant   *GGUFFileInfo
	QualityRating string // "excellent", "good", "fair", "unknown"
	IsPopular     bool   // High download count
	IsTrusted     bool   // From trusted author
	IsRecommended bool   // Recommended for beginners
}

// GGUFFileInfo contains parsed GGUF file information
type GGUFFileInfo struct {
	Filename      string
	Size          int64
	SizeGB        float64
	Quantization  string // Parsed from filename (Q4_K_M, Q5_K_S, etc.)
	ParameterSize string // Parsed parameter count (7B, 13B, etc.)
	IsChat        bool   // Contains "chat" or "instruct"
	DownloadURL   string
}

// NewHuggingFaceClient creates a new HuggingFace API client
func NewHuggingFaceClient() *HuggingFaceClient {
	// Use default transport which respects HTTP_PROXY, HTTPS_PROXY, NO_PROXY env vars
	transport := http.DefaultTransport.(*http.Transport).Clone()

	return &HuggingFaceClient{
		client: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
		baseURL: "https://huggingface.co/api",
	}
}

// SearchModels searches HuggingFace Hub for models matching the filter
func (hf *HuggingFaceClient) SearchModels(filter SearchFilter) ([]SearchResult, error) {
	// Build search URL
	searchURL := fmt.Sprintf("%s/models", hf.baseURL)
	params := url.Values{}

	// Apply search query
	if filter.Query != "" {
		params.Add("search", filter.Query)
	}

	// Apply filters
	if filter.OnlyGGUF {
		params.Add("filter", "gguf")
	}

	// Add tags filter
	if len(filter.Tags) > 0 {
		for _, tag := range filter.Tags {
			params.Add("filter", tag)
		}
	}

	// Add author filter
	if filter.Author != "" {
		params.Add("author", filter.Author)
	}

	// Set sorting
	sortBy := filter.SortBy
	if sortBy == "" {
		sortBy = "downloads" // Default to most popular
	}
	params.Add("sort", sortBy)
	params.Add("direction", "-1") // Descending

	// Set limit
	limit := filter.Limit
	if limit == 0 {
		limit = 50 // Default limit
	}
	params.Add("limit", fmt.Sprintf("%d", limit))

	// Make API request
	fullURL := fmt.Sprintf("%s?%s", searchURL, params.Encode())
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "OffGrid-LLM/0.1.0")

	resp, err := hf.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var models []HFModel
	if err := json.NewDecoder(resp.Body).Decode(&models); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Process and filter results
	results := make([]SearchResult, 0, len(models))
	for _, model := range models {
		// Skip if not a GGUF model (check tags)
		isGGUF := false
		for _, tag := range model.Tags {
			if strings.ToLower(tag) == "gguf" {
				isGGUF = true
				break
			}
		}
		if filter.OnlyGGUF && !isGGUF {
			continue
		}

		// Apply additional filters
		if filter.ExcludeGated && model.Gated {
			continue
		}
		if filter.ExcludePrivate && model.Private {
			continue
		}
		if filter.MinDownloads > 0 && model.Downloads < filter.MinDownloads {
			continue
		}
		if filter.MinLikes > 0 && model.Likes < filter.MinLikes {
			continue
		}

		// For GGUF models, fetch file details with sizes
		var ggufFiles []GGUFFileInfo
		if isGGUF {
			// Use tree API to get actual file sizes
			files, err := hf.GetModelFiles(model.ID)
			if err != nil {
				// Skip models we can't fetch files for
				continue
			}
			ggufFiles = hf.parseGGUFFilesFromTree(model.ID, files)
		}

		if filter.OnlyGGUF && len(ggufFiles) == 0 {
			continue // Skip models without GGUF files
		}

		// Apply size filters
		filteredFiles := make([]GGUFFileInfo, 0)
		for _, file := range ggufFiles {
			if filter.MaxSize > 0 && file.Size > filter.MaxSize {
				continue
			}
			if filter.MinSize > 0 && file.Size < filter.MinSize {
				continue
			}
			if filter.Quantization != "" {
				if !strings.EqualFold(file.Quantization, filter.Quantization) {
					continue
				}
			}
			filteredFiles = append(filteredFiles, file)
		}

		if len(filteredFiles) == 0 {
			continue
		}

		// Calculate total size AFTER filtering
		totalSize := int64(0)
		for _, file := range filteredFiles {
			totalSize += file.Size
		}

		// Calculate relevance score
		score := hf.calculateScore(model, filter)

		// Find best variant (highest quality that fits)
		bestVariant := hf.selectBestVariant(filteredFiles)

		// Calculate quality rating
		qualityRating := calculateQualityRating(model)
		isPopular := model.Downloads > 10000
		isTrusted := isTrustedAuthor(model.ID)
		isRecommended := isPopular && isTrusted && (model.Downloads > 50000 || model.Likes > 100)

		results = append(results, SearchResult{
			Model:         model,
			GGUFFiles:     filteredFiles,
			TotalSize:     totalSize,
			Score:         score,
			BestVariant:   bestVariant,
			QualityRating: qualityRating,
			IsPopular:     isPopular,
			IsTrusted:     isTrusted,
			IsRecommended: isRecommended,
		})
	}

	// Sort by score if not using API sorting
	if filter.SortBy == "" || filter.SortBy == "relevance" {
		sort.Slice(results, func(i, j int) bool {
			return results[i].Score > results[j].Score
		})
	}

	return results, nil
}

// parseGGUFFilesFromTree parses GGUF files from tree API response with actual sizes
func (hf *HuggingFaceClient) parseGGUFFilesFromTree(modelID string, files []HFFile) []GGUFFileInfo {
	ggufFiles := make([]GGUFFileInfo, 0)

	for _, file := range files {
		filename := file.Filename

		// Only include .gguf files
		if !strings.HasSuffix(strings.ToLower(filename), ".gguf") {
			continue
		}

		size := file.Size
		sizeGB := float64(size) / (1024 * 1024 * 1024)

		// If size is 0, estimate (shouldn't happen with tree API but just in case)
		if size == 0 {
			quant := extractQuantization(filename)
			params := extractParameterSize(filename)
			size = estimateModelSize(params, quant)
			sizeGB = float64(size) / (1024 * 1024 * 1024)
		}

		info := GGUFFileInfo{
			Filename:      filename,
			Size:          size,
			SizeGB:        sizeGB,
			Quantization:  extractQuantization(filename),
			ParameterSize: extractParameterSize(filename),
			IsChat: strings.Contains(strings.ToLower(filename), "chat") ||
				strings.Contains(strings.ToLower(filename), "instruct"),
			DownloadURL: fmt.Sprintf("https://huggingface.co/%s/resolve/main/%s",
				modelID, filename),
		}

		ggufFiles = append(ggufFiles, info)
	}

	return ggufFiles
}

// parseGGUFFiles extracts and parses GGUF files from model siblings (legacy method)
func (hf *HuggingFaceClient) parseGGUFFiles(model HFModel) []GGUFFileInfo {
	files := make([]GGUFFileInfo, 0)

	for _, sibling := range model.Siblings {
		filename := sibling.Filename
		if !strings.HasSuffix(strings.ToLower(filename), ".gguf") {
			continue
		}

		size := sibling.Size
		sizeGB := float64(size) / (1024 * 1024 * 1024)

		// If size is 0 or not available, estimate based on quantization and parameters
		if size == 0 {
			quant := extractQuantization(filename)
			params := extractParameterSize(filename)
			size = estimateModelSize(params, quant)
			sizeGB = float64(size) / (1024 * 1024 * 1024)
		}

		info := GGUFFileInfo{
			Filename:      filename,
			Size:          size,
			SizeGB:        sizeGB,
			Quantization:  extractQuantization(filename),
			ParameterSize: extractParameterSize(filename),
			IsChat: strings.Contains(strings.ToLower(filename), "chat") ||
				strings.Contains(strings.ToLower(filename), "instruct"),
			DownloadURL: fmt.Sprintf("https://huggingface.co/%s/resolve/main/%s",
				model.ID, filename),
		}

		files = append(files, info)
	}

	return files
}

// extractQuantization extracts quantization level from filename
func extractQuantization(filename string) string {
	// Common GGUF quantization patterns
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

// extractParameterSize extracts parameter count from filename or model ID
func extractParameterSize(filename string) string {
	upper := strings.ToUpper(filename)

	// More comprehensive patterns - check specific sizes first (most specific to least specific)
	patterns := []struct {
		pattern string
		size    string
	}{
		{"405B", "405B"},
		{"70B", "70B"},
		{"34B", "34B"},
		{"30B", "30B"},
		{"13B", "13B"},
		{"8B", "8B"},
		{"7B", "7B"},
		{"3B", "3B"},
		{"1.1B", "1.1B"},
		{"1B", "1B"},
		// Also check for patterns with hyphen
		{"-405B-", "405B"},
		{"-70B-", "70B"},
		{"-34B-", "34B"},
		{"-30B-", "30B"},
		{"-13B-", "13B"},
		{"-8B-", "8B"},
		{"-7B-", "7B"},
		{"-3.2-3B-", "3B"},
		{"-3.2-1B-", "1B"},
		{"-3B-", "3B"},
		{"-1.1B-", "1.1B"},
		{"-1B-", "1B"},
	}

	for _, p := range patterns {
		if strings.Contains(upper, p.pattern) {
			return p.size
		}
	}

	return "unknown"
}

// estimateModelSize estimates file size based on parameter count and quantization
func estimateModelSize(paramSize, quantization string) int64 {
	// Base parameter counts in billions
	paramMap := map[string]float64{
		"1B":   1.0,
		"1.1B": 1.1,
		"3B":   3.0,
		"7B":   7.0,
		"8B":   8.0,
		"13B":  13.0,
		"30B":  30.0,
		"34B":  34.0,
		"70B":  70.0,
		"405B": 405.0,
	}

	// Bits per parameter for different quantization levels
	// These are approximate effective bits after quantization
	bitsPerParam := map[string]float64{
		"Q2_K":   2.5,
		"Q3_K_S": 3.25,
		"Q3_K_M": 3.5,
		"Q3_K_L": 3.75,
		"Q4_0":   4.0,
		"Q4_1":   4.5,
		"Q4_K_S": 4.25,
		"Q4_K_M": 4.5,
		"Q5_0":   5.0,
		"Q5_1":   5.5,
		"Q5_K_S": 5.25,
		"Q5_K_M": 5.5,
		"Q6_K":   6.5,
		"Q8_0":   8.5,
		"F16":    16.0,
		"F32":    32.0,
	}

	params, ok := paramMap[paramSize]
	if !ok {
		params = 7.0 // Default to 7B if unknown
	}

	bits, ok := bitsPerParam[quantization]
	if !ok {
		bits = 4.5 // Default to Q4_K_M equivalent
	}

	// Calculate size: (parameters in billions * bits per param * 1 billion / 8 bytes per bit) * overhead
	// Overhead is approximately 1.05-1.10 for metadata, embeddings, etc.
	sizeBytes := int64(params * bits * 1e9 / 8 * 1.08)

	return sizeBytes
}

// calculateScore computes a relevance score for ranking
func (hf *HuggingFaceClient) calculateScore(model HFModel, filter SearchFilter) float64 {
	score := 0.0

	// Downloads factor (logarithmic scale)
	if model.Downloads > 0 {
		score += float64(model.Downloads) / 1000.0
	}

	// Likes factor
	score += float64(model.Likes) * 10.0

	// Recency bonus (models updated in last 6 months)
	monthsSinceUpdate := time.Since(model.LastModified).Hours() / 24 / 30
	if monthsSinceUpdate < 6 {
		score += (6 - monthsSinceUpdate) * 50
	}

	// Query relevance bonus
	if filter.Query != "" {
		queryLower := strings.ToLower(filter.Query)
		idLower := strings.ToLower(model.ID)
		if strings.Contains(idLower, queryLower) {
			score += 200.0
		}
	}

	// Penalty for gated models
	if model.Gated {
		score *= 0.5
	}

	return score
}

// selectBestVariant picks the best GGUF file (balanced quality/size)
func (hf *HuggingFaceClient) selectBestVariant(files []GGUFFileInfo) *GGUFFileInfo {
	if len(files) == 0 {
		return nil
	}

	// Prefer Q4_K_M or Q5_K_M (good balance)
	for i := range files {
		if files[i].Quantization == "Q4_K_M" || files[i].Quantization == "Q5_K_M" {
			return &files[i]
		}
	}

	// Fallback to first file
	return &files[0]
}

// GetModelInfo fetches detailed information about a specific model
func (hf *HuggingFaceClient) GetModelInfo(modelID string) (*HFModel, error) {
	// Note: modelID is in format "owner/repo" - don't escape the slash
	apiURL := fmt.Sprintf("%s/models/%s", hf.baseURL, modelID)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "OffGrid-LLM/0.1.0")

	resp, err := hf.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch model: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var model HFModel
	if err := json.NewDecoder(resp.Body).Decode(&model); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &model, nil
}

// GetModelFiles fetches file list with sizes using the tree API

func (hf *HuggingFaceClient) GetModelFiles(modelID string) ([]HFFile, error) {
	// Use the tree API which includes file sizes
	// Note: modelID is in format "owner/repo" - don't escape the slash
	apiURL := fmt.Sprintf("%s/models/%s/tree/main", hf.baseURL, modelID)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "OffGrid-LLM/0.1.0")

	resp, err := hf.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch model files: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	// Parse tree response
	var treeFiles []struct {
		Type string `json:"type"`
		Path string `json:"path"`
		Size int64  `json:"size"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&treeFiles); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to HFFile format (keep all files so we can detect projectors)
	files := make([]HFFile, 0)
	for _, tf := range treeFiles {
		if tf.Type != "file" {
			continue
		}
		files = append(files, HFFile{
			Filename: tf.Path,
			Size:     tf.Size,
		})
	}

	return files, nil
}

// DownloadGGUF downloads a GGUF file from HuggingFace

func (hf *HuggingFaceClient) DownloadGGUF(modelID, filename, destPath string, onProgress func(int64, int64)) error {
	downloadURL := fmt.Sprintf("https://huggingface.co/%s/resolve/main/%s", modelID, filename)

	// Use .tmp file during download
	tmpPath := destPath + ".tmp"

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Check if partially downloaded
	var written int64
	if stat, err := os.Stat(tmpPath); err == nil {
		written = stat.Size()
	}

	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "OffGrid-LLM/0.1.0")

	// Support resume with Range header
	if written > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", written))
	}

	// Use a client with no timeout for large downloads
	client := &http.Client{Timeout: 0}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Get total size
	totalSize := resp.ContentLength
	if resp.StatusCode == http.StatusPartialContent {
		totalSize += written // Add already downloaded bytes
	}

	// Open/create .tmp file
	flag := os.O_CREATE | os.O_WRONLY
	if written > 0 {
		flag |= os.O_APPEND
	}

	out, err := os.OpenFile(tmpPath, flag, 0644)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	// Copy with progress tracking
	buf := make([]byte, 32*1024) // 32KB buffer
	for {
		nr, err := resp.Body.Read(buf)
		if nr > 0 {
			nw, ew := out.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
				if onProgress != nil {
					onProgress(written, totalSize)
				}
			}
			if ew != nil {
				return ew
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	// Verify complete download
	if written != totalSize {
		return fmt.Errorf("incomplete download: got %d bytes, expected %d", written, totalSize)
	}

	// Move .tmp to final destination
	if err := os.Rename(tmpPath, destPath); err != nil {
		return fmt.Errorf("failed to finalize download: %w", err)
	}

	return nil
}

// DetectProjectorFile returns the best matching projector/mmproj companion for a GGUF
func DetectProjectorFile(files []HFFile, modelFilename string) *HFFile {
	if len(files) == 0 || modelFilename == "" {
		return nil
	}

	targetLower := strings.ToLower(modelFilename)
	targetStem := normalizedProjectorStem(modelFilename)
	bestIdx := -1
	bestScore := -1

	for i := range files {
		candidate := files[i]
		if !isProjectorFilename(candidate.Filename) {
			continue
		}

		score := scoreProjectorCandidate(targetLower, strings.ToLower(candidate.Filename), targetStem)
		if bestIdx == -1 || score > bestScore {
			bestIdx = i
			bestScore = score
		}
	}

	if bestIdx == -1 {
		return nil
	}

	file := files[bestIdx]
	return &file
}

func isProjectorFilename(filename string) bool {
	lower := strings.ToLower(filename)
	if !(strings.Contains(lower, "mmproj") || strings.Contains(lower, "projector")) {
		return false
	}

	if !(strings.HasSuffix(lower, ".gguf") || strings.HasSuffix(lower, ".ggml") || strings.HasSuffix(lower, ".bin") || strings.HasSuffix(lower, ".mmproj")) {
		return false
	}

	return true
}

func scoreProjectorCandidate(target, candidate, stem string) int {
	score := longestCommonPrefixLength(target, candidate)
	if stem != "" && strings.Contains(candidate, stem) {
		score += len(stem)
	}
	if strings.Contains(candidate, "mmproj") {
		score += 50
	}
	if strings.Contains(candidate, "projector") {
		score += 25
	}
	return score
}

func longestCommonPrefixLength(a, b string) int {
	limit := len(a)
	if len(b) < limit {
		limit = len(b)
	}
	count := 0
	for i := 0; i < limit; i++ {
		if a[i] != b[i] {
			break
		}
		count++
	}
	return count
}

func normalizedProjectorStem(filename string) string {
	base := strings.TrimSuffix(filename, path.Ext(filename))
	lower := strings.ToLower(base)
	quant := strings.ToLower(extractQuantization(filename))
	if quant == "" || quant == "unknown" {
		return lower
	}

	for _, sep := range []string{"-", "_", "."} {
		token := sep + quant
		if strings.HasSuffix(lower, token) {
			trimmed := strings.TrimSuffix(lower, token)
			trimmed = strings.TrimSuffix(trimmed, sep)
			return trimmed
		}
	}

	return lower
}

// ListGGUFFiles returns GGUF file metadata for a repo using the tree API so sizes are accurate.
func (hf *HuggingFaceClient) ListGGUFFiles(modelID string) ([]GGUFFileInfo, error) {
	files, err := hf.GetModelFiles(modelID)
	if err != nil {
		return nil, err
	}
	return hf.parseGGUFFilesFromTree(modelID, files), nil
}

// ResolveProjectorSource locates the best projector/mmproj companion for a GGUF file.
// It first searches the model's own repository; if none is present, it checks known
// fallback mappings (e.g. koboldcpp/mmproj) so users do not have to fetch adapters manually.
func (hf *HuggingFaceClient) ResolveProjectorSource(modelID string, knownFiles []HFFile, modelFilename string) (*ProjectorSource, error) {
	var files []HFFile
	if len(knownFiles) > 0 {
		files = knownFiles
	} else if hf != nil {
		if fetched, err := hf.GetModelFiles(modelID); err == nil {
			files = fetched
		}
	}

	if len(files) > 0 && modelFilename != "" {
		if candidate := DetectProjectorFile(files, modelFilename); candidate != nil {
			fileCopy := *candidate
			return &ProjectorSource{
				ModelID: modelID,
				File:    fileCopy,
				Source:  "companion",
				Reason:  "Found in model repository",
			}, nil
		}
	}

	fallback := findProjectorFallback(modelID, modelFilename)
	if fallback == nil {
		return nil, nil
	}

	fileMeta := HFFile{Filename: fallback.Filename}
	if hf != nil {
		if fallbackFiles, err := hf.GetModelFiles(fallback.Repository); err == nil {
			for _, f := range fallbackFiles {
				if f.Filename == fallback.Filename {
					fileMeta.Size = f.Size
					break
				}
			}
		}
	}

	return &ProjectorSource{
		ModelID: fallback.Repository,
		File:    fileMeta,
		Source:  "fallback",
		Reason:  fallback.Reason,
	}, nil
}

// calculateQualityRating determines quality based on downloads and likes
func calculateQualityRating(model HFModel) string {
	downloads := model.Downloads
	likes := model.Likes

	// Excellent: High community trust
	if downloads > 100000 && likes > 500 {
		return "excellent"
	}
	if downloads > 50000 && likes > 200 {
		return "excellent"
	}

	// Good: Popular and liked
	if downloads > 10000 && likes > 50 {
		return "good"
	}
	if downloads > 5000 && likes > 20 {
		return "good"
	}

	// Fair: Some usage
	if downloads > 1000 || likes > 5 {
		return "fair"
	}

	return "unknown"
}

// isTrustedAuthor checks if the model is from a well-known author
func isTrustedAuthor(modelID string) bool {
	trustedAuthors := []string{
		"TheBloke",
		"bartowski",
		"meta-llama",
		"mistralai",
		"microsoft",
		"google",
		"HuggingFaceH4",
		"MaziyarPanahi",
		"NousResearch",
		"stabilityai",
		"EleutherAI",
		"bigscience",
		"tiiuae",
		"teknium",
		"Qwen",
		"deepseek-ai",
	}

	idLower := strings.ToLower(modelID)
	for _, author := range trustedAuthors {
		if strings.HasPrefix(idLower, strings.ToLower(author)+"/") {
			return true
		}
	}
	return false
}
