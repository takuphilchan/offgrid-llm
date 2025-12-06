package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Message represents a conversation message
type Message struct {
	Role      string         `json:"role"`
	Content   string         `json:"content"`
	Timestamp time.Time      `json:"timestamp,omitempty"`
	Tokens    int            `json:"tokens,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// MemoryType represents the type of memory storage
type MemoryType string

const (
	MemoryTypeBuffer    MemoryType = "buffer"       // Simple buffer with truncation
	MemoryTypeSummary   MemoryType = "summary"      // Summarize old messages
	MemoryTypeHierarchy MemoryType = "hierarchical" // Short + long term memory
	MemoryTypeWindow    MemoryType = "window"       // Sliding window
)

// CompressionConfig configures memory compression
type CompressionConfig struct {
	MaxMessages       int        `json:"max_messages"`        // Max messages before compression
	MaxTokens         int        `json:"max_tokens"`          // Max total tokens
	WindowSize        int        `json:"window_size"`         // Sliding window size
	SummaryMaxTokens  int        `json:"summary_max_tokens"`  // Max tokens for summary
	PreserveSystemMsg bool       `json:"preserve_system_msg"` // Always keep system message
	PreserveRecentN   int        `json:"preserve_recent_n"`   // Always keep N most recent messages
	Type              MemoryType `json:"type"`                // Memory type
}

// DefaultCompressionConfig returns sensible defaults
func DefaultCompressionConfig() CompressionConfig {
	return CompressionConfig{
		MaxMessages:       100,
		MaxTokens:         4096,
		WindowSize:        20,
		SummaryMaxTokens:  500,
		PreserveSystemMsg: true,
		PreserveRecentN:   5,
		Type:              MemoryTypeSummary,
	}
}

// Summarizer generates summaries of conversations
type Summarizer func(ctx context.Context, messages []Message) (string, error)

// TokenCounter counts tokens in text
type TokenCounter func(text string) int

// Memory manages conversation memory with compression
type Memory struct {
	mu             sync.RWMutex
	messages       []Message
	summary        string
	summaryTokens  int
	config         CompressionConfig
	summarizer     Summarizer
	tokenCounter   TokenCounter
	lastCompressed time.Time
}

// NewMemory creates a new memory instance
func NewMemory(config CompressionConfig) *Memory {
	if config.MaxMessages == 0 {
		config = DefaultCompressionConfig()
	}

	return &Memory{
		messages:     make([]Message, 0),
		config:       config,
		tokenCounter: defaultTokenCounter,
	}
}

// SetSummarizer sets the function used for generating summaries
func (m *Memory) SetSummarizer(fn Summarizer) {
	m.summarizer = fn
}

// SetTokenCounter sets the function used for counting tokens
func (m *Memory) SetTokenCounter(fn TokenCounter) {
	m.tokenCounter = fn
}

// Add adds a message to memory
func (m *Memory) Add(msg Message) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	if msg.Tokens == 0 && m.tokenCounter != nil {
		msg.Tokens = m.tokenCounter(msg.Content)
	}

	m.messages = append(m.messages, msg)
}

// AddMessages adds multiple messages
func (m *Memory) AddMessages(msgs []Message) {
	for _, msg := range msgs {
		m.Add(msg)
	}
}

// GetMessages returns all messages (possibly compressed)
func (m *Memory) GetMessages(ctx context.Context) []Message {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.needsCompression() {
		m.compress(ctx)
	}

	return m.buildOutput()
}

// GetRawMessages returns all messages without compression
func (m *Memory) GetRawMessages() []Message {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Message, len(m.messages))
	copy(result, m.messages)
	return result
}

// Clear clears all memory
func (m *Memory) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.messages = make([]Message, 0)
	m.summary = ""
	m.summaryTokens = 0
}

// needsCompression checks if compression is needed
func (m *Memory) needsCompression() bool {
	if len(m.messages) <= m.config.PreserveRecentN {
		return false
	}

	switch m.config.Type {
	case MemoryTypeBuffer:
		return len(m.messages) > m.config.MaxMessages
	case MemoryTypeWindow:
		return len(m.messages) > m.config.WindowSize
	case MemoryTypeSummary, MemoryTypeHierarchy:
		totalTokens := m.summaryTokens
		for _, msg := range m.messages {
			totalTokens += msg.Tokens
		}
		return totalTokens > m.config.MaxTokens || len(m.messages) > m.config.MaxMessages
	default:
		return len(m.messages) > m.config.MaxMessages
	}
}

// compress performs memory compression
func (m *Memory) compress(ctx context.Context) {
	switch m.config.Type {
	case MemoryTypeBuffer:
		m.compressBuffer()
	case MemoryTypeWindow:
		m.compressWindow()
	case MemoryTypeSummary:
		m.compressSummary(ctx)
	case MemoryTypeHierarchy:
		m.compressHierarchy(ctx)
	}

	m.lastCompressed = time.Now()
}

// compressBuffer simply truncates old messages
func (m *Memory) compressBuffer() {
	if len(m.messages) <= m.config.MaxMessages {
		return
	}

	// Find system message if we need to preserve it
	var systemMsg *Message
	startIdx := 0
	if m.config.PreserveSystemMsg && len(m.messages) > 0 && m.messages[0].Role == "system" {
		systemMsg = &m.messages[0]
		startIdx = 1
	}

	// Calculate how many to keep
	keepCount := m.config.MaxMessages
	if systemMsg != nil {
		keepCount--
	}

	// Keep the most recent messages
	if len(m.messages)-startIdx > keepCount {
		remaining := m.messages[len(m.messages)-keepCount:]
		if systemMsg != nil {
			m.messages = append([]Message{*systemMsg}, remaining...)
		} else {
			m.messages = remaining
		}
	}
}

// compressWindow keeps only a sliding window of messages
func (m *Memory) compressWindow() {
	if len(m.messages) <= m.config.WindowSize {
		return
	}

	// Find system message
	var systemMsg *Message
	startIdx := 0
	if m.config.PreserveSystemMsg && len(m.messages) > 0 && m.messages[0].Role == "system" {
		systemMsg = &m.messages[0]
		startIdx = 1
	}

	windowSize := m.config.WindowSize
	if systemMsg != nil {
		windowSize--
	}

	// Keep window
	if len(m.messages)-startIdx > windowSize {
		remaining := m.messages[len(m.messages)-windowSize:]
		if systemMsg != nil {
			m.messages = append([]Message{*systemMsg}, remaining...)
		} else {
			m.messages = remaining
		}
	}
}

// compressSummary summarizes old messages
func (m *Memory) compressSummary(ctx context.Context) {
	if m.summarizer == nil || len(m.messages) <= m.config.PreserveRecentN {
		// Fallback to buffer compression
		m.compressBuffer()
		return
	}

	// Find system message
	var systemMsg *Message
	startIdx := 0
	if m.config.PreserveSystemMsg && len(m.messages) > 0 && m.messages[0].Role == "system" {
		systemMsg = &m.messages[0]
		startIdx = 1
	}

	// Messages to summarize (everything except recent N)
	preserveCount := m.config.PreserveRecentN
	if startIdx > 0 {
		preserveCount--
	}

	if len(m.messages)-startIdx <= preserveCount {
		return
	}

	toSummarize := m.messages[startIdx : len(m.messages)-preserveCount]
	toPreserve := m.messages[len(m.messages)-preserveCount:]

	// Generate summary
	summary, err := m.summarizer(ctx, toSummarize)
	if err != nil {
		// Fallback to buffer compression on error
		m.compressBuffer()
		return
	}

	// Combine old summary with new
	if m.summary != "" {
		summary = m.summary + "\n\n" + summary
	}

	// Truncate summary if too long
	if m.tokenCounter != nil {
		tokens := m.tokenCounter(summary)
		if tokens > m.config.SummaryMaxTokens {
			// Simple truncation - in production you'd re-summarize
			words := strings.Fields(summary)
			ratio := float64(m.config.SummaryMaxTokens) / float64(tokens)
			keepWords := int(float64(len(words)) * ratio)
			if keepWords > 0 {
				summary = strings.Join(words[:keepWords], " ") + "..."
			}
		}
	}

	m.summary = summary
	m.summaryTokens = m.tokenCounter(summary)

	// Rebuild messages with preserved ones
	newMessages := make([]Message, 0)
	if systemMsg != nil {
		newMessages = append(newMessages, *systemMsg)
	}
	newMessages = append(newMessages, toPreserve...)
	m.messages = newMessages
}

// compressHierarchy uses hierarchical memory (short + long term)
func (m *Memory) compressHierarchy(ctx context.Context) {
	// Similar to summary but maintains separate short and long term
	m.compressSummary(ctx)
}

// buildOutput builds the output message list
func (m *Memory) buildOutput() []Message {
	result := make([]Message, 0, len(m.messages)+1)

	// Add summary as a system message supplement if exists
	hasSummary := m.summary != ""
	hasSystemMsg := len(m.messages) > 0 && m.messages[0].Role == "system"

	for i, msg := range m.messages {
		if i == 0 && hasSystemMsg && hasSummary {
			// Append summary to system message
			combined := Message{
				Role:      "system",
				Content:   msg.Content + "\n\n[Previous conversation summary]\n" + m.summary,
				Timestamp: msg.Timestamp,
				Metadata:  msg.Metadata,
			}
			result = append(result, combined)
		} else {
			result = append(result, msg)
		}
	}

	// If no system message but has summary, add summary as first message
	if hasSummary && !hasSystemMsg {
		summaryMsg := Message{
			Role:    "system",
			Content: "[Previous conversation summary]\n" + m.summary,
		}
		result = append([]Message{summaryMsg}, result...)
	}

	return result
}

// GetSummary returns the current summary
func (m *Memory) GetSummary() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.summary
}

// Stats returns memory statistics
func (m *Memory) Stats() map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalTokens := m.summaryTokens
	for _, msg := range m.messages {
		totalTokens += msg.Tokens
	}

	return map[string]any{
		"message_count":   len(m.messages),
		"total_tokens":    totalTokens,
		"summary_tokens":  m.summaryTokens,
		"has_summary":     m.summary != "",
		"last_compressed": m.lastCompressed,
		"memory_type":     m.config.Type,
	}
}

// Export exports memory to JSON
func (m *Memory) Export() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	export := struct {
		Messages []Message         `json:"messages"`
		Summary  string            `json:"summary,omitempty"`
		Config   CompressionConfig `json:"config"`
	}{
		Messages: m.messages,
		Summary:  m.summary,
		Config:   m.config,
	}

	return json.Marshal(export)
}

// Import imports memory from JSON
func (m *Memory) Import(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var imported struct {
		Messages []Message         `json:"messages"`
		Summary  string            `json:"summary,omitempty"`
		Config   CompressionConfig `json:"config"`
	}

	if err := json.Unmarshal(data, &imported); err != nil {
		return err
	}

	m.messages = imported.Messages
	m.summary = imported.Summary
	if imported.Config.MaxMessages > 0 {
		m.config = imported.Config
	}

	return nil
}

// defaultTokenCounter is a simple token counter (approximation)
func defaultTokenCounter(text string) int {
	// Rough approximation: ~4 characters per token
	return len(text) / 4
}

// DefaultSummarizerPrompt returns a prompt for summarization
func DefaultSummarizerPrompt(messages []Message) string {
	var sb strings.Builder
	sb.WriteString("Summarize the following conversation concisely, preserving key information and context:\n\n")

	for _, msg := range messages {
		sb.WriteString(fmt.Sprintf("%s: %s\n", strings.Title(msg.Role), msg.Content))
	}

	sb.WriteString("\nProvide a brief summary that captures the main topics, decisions, and any important details.")
	return sb.String()
}

// MemoryManager manages multiple conversation memories
type MemoryManager struct {
	mu         sync.RWMutex
	memories   map[string]*Memory
	config     CompressionConfig
	summarizer Summarizer
}

// NewMemoryManager creates a new memory manager
func NewMemoryManager(config CompressionConfig) *MemoryManager {
	return &MemoryManager{
		memories: make(map[string]*Memory),
		config:   config,
	}
}

// SetSummarizer sets the summarizer for all memories
func (mm *MemoryManager) SetSummarizer(fn Summarizer) {
	mm.summarizer = fn
}

// GetOrCreate gets or creates a memory for a session
func (mm *MemoryManager) GetOrCreate(sessionID string) *Memory {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	if mem, exists := mm.memories[sessionID]; exists {
		return mem
	}

	mem := NewMemory(mm.config)
	if mm.summarizer != nil {
		mem.SetSummarizer(mm.summarizer)
	}
	mm.memories[sessionID] = mem
	return mem
}

// Get gets a memory for a session
func (mm *MemoryManager) Get(sessionID string) (*Memory, bool) {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	mem, exists := mm.memories[sessionID]
	return mem, exists
}

// Delete deletes a memory
func (mm *MemoryManager) Delete(sessionID string) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	delete(mm.memories, sessionID)
}

// List lists all session IDs
func (mm *MemoryManager) List() []string {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	ids := make([]string, 0, len(mm.memories))
	for id := range mm.memories {
		ids = append(ids, id)
	}
	return ids
}

// Stats returns stats for all memories
func (mm *MemoryManager) Stats() map[string]any {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	stats := make(map[string]any)
	for id, mem := range mm.memories {
		stats[id] = mem.Stats()
	}
	return stats
}
