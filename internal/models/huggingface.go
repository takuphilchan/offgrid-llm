package models

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
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
	BaseModel    string                 `json:"base_model"`
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
	return &HuggingFaceClient{
		client: &http.Client{
			Timeout: 30 * time.Second,
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

		// For GGUF models, fetch file details
		var ggufFiles []GGUFFileInfo
		if isGGUF {
			// Fetch detailed model info to get files
			detailedModel, err := hf.GetModelInfo(model.ID)
			if err != nil {
				// Skip models we can't fetch details for
				continue
			}
			ggufFiles = hf.parseGGUFFiles(*detailedModel)
		}

		if filter.OnlyGGUF && len(ggufFiles) == 0 {
			continue // Skip models without GGUF files
		}

		// Apply size filters
		totalSize := int64(0)
		filteredFiles := make([]GGUFFileInfo, 0)
		for _, file := range ggufFiles {
			totalSize += file.Size
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

// parseGGUFFiles extracts and parses GGUF files from model siblings
func (hf *HuggingFaceClient) parseGGUFFiles(model HFModel) []GGUFFileInfo {
	files := make([]GGUFFileInfo, 0)

	for _, sibling := range model.Siblings {
		filename := sibling.Filename
		if !strings.HasSuffix(strings.ToLower(filename), ".gguf") {
			continue
		}

		info := GGUFFileInfo{
			Filename:      filename,
			Size:          sibling.Size,
			SizeGB:        float64(sibling.Size) / (1024 * 1024 * 1024),
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
	// Common parameter sizes
	patterns := []string{"1.1B", "3B", "7B", "8B", "13B", "30B", "34B", "70B", "405B"}

	upper := strings.ToUpper(filename)
	for _, pattern := range patterns {
		if strings.Contains(upper, pattern) {
			return pattern
		}
	}

	return "unknown"
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
	apiURL := fmt.Sprintf("%s/models/%s", hf.baseURL, url.PathEscape(modelID))

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

// DownloadGGUF downloads a GGUF file from HuggingFace
func (hf *HuggingFaceClient) DownloadGGUF(modelID, filename, destPath string, onProgress func(int64, int64)) error {
	downloadURL := fmt.Sprintf("https://huggingface.co/%s/resolve/main/%s", modelID, filename)

	// Use .tmp file during download
	tmpPath := destPath + ".tmp"

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
