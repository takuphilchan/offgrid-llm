// Package docs provides offline documentation and help system.
// All documentation is embedded in the binary for air-gapped access.
package docs

import (
	"fmt"
	"sort"
	"strings"
)

// Topic represents a documentation topic
type Topic struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Category   string   `json:"category"`
	Keywords   []string `json:"keywords"`
	Content    string   `json:"content"`
	RelatedIDs []string `json:"related_ids,omitempty"`
}

// Category represents a documentation category
type Category struct {
	ID     string   `json:"id"`
	Title  string   `json:"title"`
	Order  int      `json:"order"`
	Topics []string `json:"topics"` // Topic IDs
}

// EmbeddedDocs contains all embedded documentation
var EmbeddedDocs = map[string]Topic{
	// Getting Started
	"quickstart": {
		ID:       "quickstart",
		Title:    "Quick Start Guide",
		Category: "getting-started",
		Keywords: []string{"start", "begin", "install", "setup", "first"},
		Content: `# OffGrid LLM Quick Start

## Starting the Server

    offgrid serve

This starts the HTTP API server on port 8080.

## Basic Chat

    offgrid chat "Hello, how are you?"

## Interactive Mode

    offgrid agent

Starts an interactive chat session.

## List Models

    offgrid models list

Shows all available local models.

## Download a Model

    offgrid models download tinyllama-1.1b

Downloads a model for offline use.
`,
		RelatedIDs: []string{"cli-reference", "models"},
	},

	"offline-setup": {
		ID:       "offline-setup",
		Title:    "Air-Gapped Setup",
		Category: "getting-started",
		Keywords: []string{"offline", "airgap", "disconnected", "isolated", "no internet"},
		Content: `# Air-Gapped Installation

## USB Transfer Method

1. On connected machine:
       offgrid models download tinyllama-1.1b --export /media/usb/models/

2. Copy the USB to air-gapped machine

3. On air-gapped machine:
       offgrid models import /media/usb/models/

## Verify Model Integrity

    offgrid models verify tinyllama-1.1b

Uses bundled SHA256 hashes to verify without internet.

## Export Configuration

    offgrid config export --output /media/usb/config.tar.gz

## Import Configuration

    offgrid config import /media/usb/config.tar.gz
`,
		RelatedIDs: []string{"models", "config-management"},
	},

	// CLI Reference
	"cli-reference": {
		ID:       "cli-reference",
		Title:    "CLI Command Reference",
		Category: "reference",
		Keywords: []string{"command", "cli", "terminal", "flags", "options"},
		Content: `# CLI Command Reference

## Global Flags

    --config PATH      Config file path
    --data-dir PATH    Data directory
    --model NAME       Default model
    --verbose          Enable verbose output
    --json             Output as JSON

## Commands

### serve
Start the HTTP API server.
    offgrid serve [--port 8080] [--host 0.0.0.0]

### chat
Send a single prompt.
    offgrid chat "Your prompt here"

### agent
Interactive chat mode.
    offgrid agent [--model NAME]

### models
Model management.
    offgrid models list
    offgrid models download MODEL_NAME
    offgrid models delete MODEL_NAME
    offgrid models verify MODEL_NAME

### config
Configuration management.
    offgrid config show
    offgrid config export --output FILE
    offgrid config import FILE

### status
Show system status.
    offgrid status

### metrics
Show performance metrics.
    offgrid metrics
`,
		RelatedIDs: []string{"quickstart", "config-management"},
	},

	// Models
	"models": {
		ID:       "models",
		Title:    "Model Management",
		Category: "guides",
		Keywords: []string{"model", "gguf", "download", "manage", "quantization"},
		Content: `# Model Management

## Supported Formats

OffGrid supports GGUF format models (llama.cpp compatible).

## Quantization Levels

- Q2_K: Smallest, lowest quality (good for <4GB RAM)
- Q4_K_M: Good balance of size/quality
- Q5_K_M: Better quality, larger size
- Q8_0: Best quality, largest size

## Storage Location

Models are stored in:
    $DATA_DIR/models/

## Recommended Models for Low-End Hardware

1. TinyLlama 1.1B (Q4_K_M) - 650MB, 1GB RAM
2. Phi-2 (Q4_K_M) - 1.6GB, 2GB RAM
3. Llama 3.2 1B (Q4_K_M) - 750MB, 1GB RAM

## Model Verification

OffGrid includes bundled SHA256 hashes for popular models:
    offgrid models verify --all

This works completely offline.
`,
		RelatedIDs: []string{"performance", "offline-setup"},
	},

	// Configuration
	"config-management": {
		ID:       "config-management",
		Title:    "Configuration Management",
		Category: "guides",
		Keywords: []string{"config", "settings", "yaml", "configure"},
		Content: `# Configuration

## Config File Location

    $HOME/.offgrid/config.yaml

## Key Settings

    server:
      port: 8080
      host: "0.0.0.0"
    
    inference:
      default_model: "tinyllama-1.1b"
      context_size: 2048
      batch_size: 512
      threads: 4
      gpu_layers: 0
    
    performance:
      flash_attention: true
      quantized_kv: true
      mmap: true
    
    p2p:
      enabled: false
      port: 9090

## Environment Variables

    OFFGRID_DATA_DIR      Data directory
    OFFGRID_MODEL         Default model
    OFFGRID_PORT          Server port
    OFFGRID_THREADS       CPU threads

## Fleet Deployment

Export config from master:
    offgrid config export -o config.tar.gz

Import on fleet devices:
    offgrid config import config.tar.gz
`,
		RelatedIDs: []string{"cli-reference", "performance"},
	},

	// Performance
	"performance": {
		ID:       "performance",
		Title:    "Performance Tuning",
		Category: "guides",
		Keywords: []string{"performance", "speed", "memory", "ram", "cpu", "optimization"},
		Content: `# Performance Tuning

## Memory Optimization

For systems with limited RAM:

    inference:
      context_size: 512      # Reduce for low RAM
      batch_size: 256        # Lower = less memory
      mmap: true             # Essential for low RAM
    
    performance:
      quantized_kv: true     # Reduces KV cache memory
      flash_attention: true  # More efficient attention

## CPU Optimization

    inference:
      threads: 4             # Match physical cores
      
OffGrid auto-detects AVX/AVX2/AVX-512 support.

## Adaptive Mode

OffGrid automatically adjusts settings based on:
- Available RAM
- CPU capabilities
- Current system load

## Graceful Degradation

Under memory pressure, OffGrid will:
1. Reduce context size
2. Limit concurrent requests
3. Disable embeddings/RAG
4. Enter emergency mode if critical
`,
		RelatedIDs: []string{"low-memory", "models"},
	},

	"low-memory": {
		ID:       "low-memory",
		Title:    "Running on 4GB RAM",
		Category: "guides",
		Keywords: []string{"4gb", "low", "memory", "ram", "minimal", "embedded"},
		Content: `# Running OffGrid on 4GB RAM

## Recommended Configuration

    inference:
      default_model: "tinyllama-1.1b-q2_k"
      context_size: 512
      batch_size: 128
      mmap: true
    
    performance:
      quantized_kv: true
      flash_attention: true

## Best Models for 4GB

1. TinyLlama 1.1B Q2_K - ~400MB
2. Phi-2 Q2_K - ~1GB
3. Llama 3.2 1B Q2_K - ~500MB

## Tips

1. Use mmap for memory-mapped model loading
2. Enable quantized KV cache
3. Limit context size to 512-1024
4. Use one model at a time
5. Close unnecessary processes

## Monitoring Memory

    offgrid status --memory
    offgrid metrics | grep memory
`,
		RelatedIDs: []string{"performance", "models"},
	},

	// Troubleshooting
	"troubleshooting": {
		ID:       "troubleshooting",
		Title:    "Troubleshooting Guide",
		Category: "troubleshooting",
		Keywords: []string{"error", "problem", "issue", "fix", "help", "debug"},
		Content: `# Troubleshooting

## Model Won't Load

1. Check available memory:
       free -m
       
2. Verify model integrity:
       offgrid models verify MODEL_NAME
       
3. Try smaller quantization:
       offgrid models download MODEL_NAME-q2_k

## Server Won't Start

1. Check port availability:
       netstat -tuln | grep 8080
       
2. Check logs:
       offgrid serve --verbose
       
3. Verify llama-server:
       which llama-server

## Slow Responses

1. Check system load:
       offgrid status
       
2. Reduce context size:
       offgrid chat --context-size 512 "Hello"
       
3. Use smaller model

## Out of Memory

1. OffGrid will auto-degrade
2. Use smaller model
3. Reduce context size
4. Enable quantized_kv

## P2P Connection Issues

1. Check firewall:
       ufw status
       
2. Verify port:
       offgrid p2p status
`,
		RelatedIDs: []string{"performance", "low-memory"},
	},

	// API Reference
	"api-reference": {
		ID:       "api-reference",
		Title:    "HTTP API Reference",
		Category: "reference",
		Keywords: []string{"api", "http", "rest", "endpoint", "curl"},
		Content: `# HTTP API Reference

## Base URL

    http://localhost:8080

## Endpoints

### POST /v1/chat/completions
OpenAI-compatible chat endpoint.

    curl -X POST http://localhost:8080/v1/chat/completions \
      -H "Content-Type: application/json" \
      -d '{
        "model": "tinyllama-1.1b",
        "messages": [{"role": "user", "content": "Hello"}]
      }'

### GET /v1/models
List available models.

    curl http://localhost:8080/v1/models

### GET /health
Health check endpoint.

    curl http://localhost:8080/health

### GET /metrics
Prometheus metrics.

    curl http://localhost:8080/metrics

### POST /v1/embeddings
Generate embeddings.

    curl -X POST http://localhost:8080/v1/embeddings \
      -H "Content-Type: application/json" \
      -d '{"input": "Hello world", "model": "bge-small"}'
`,
		RelatedIDs: []string{"cli-reference"},
	},

	// Security
	"security": {
		ID:       "security",
		Title:    "Security & Audit Logging",
		Category: "guides",
		Keywords: []string{"security", "audit", "log", "compliance", "airgap"},
		Content: `# Security & Audit Logging

## Audit Logging

All operations are logged for compliance:

    $DATA_DIR/audit/audit_*.jsonl

## Audit Log Format

Each entry includes:
- Timestamp
- Event type (AUTH, QUERY, MODEL, CONFIG, etc.)
- User/source
- Success/failure
- HMAC signature for tamper detection

## Viewing Audit Logs

    offgrid audit query --last 100
    offgrid audit query --user admin --type AUTH
    offgrid audit export --output audit_report.json

## Verify Log Integrity

    offgrid audit verify

Checks HMAC chain for tampering.

## Security Best Practices

1. Use API keys for authentication
2. Run behind firewall
3. Enable audit logging
4. Regular log exports
5. Verify model hashes
`,
		RelatedIDs: []string{"offline-setup"},
	},
}

// Categories defines the documentation structure
var Categories = []Category{
	{
		ID:     "getting-started",
		Title:  "Getting Started",
		Order:  1,
		Topics: []string{"quickstart", "offline-setup"},
	},
	{
		ID:     "guides",
		Title:  "User Guides",
		Order:  2,
		Topics: []string{"models", "config-management", "performance", "low-memory", "security"},
	},
	{
		ID:     "reference",
		Title:  "Reference",
		Order:  3,
		Topics: []string{"cli-reference", "api-reference"},
	},
	{
		ID:     "troubleshooting",
		Title:  "Troubleshooting",
		Order:  4,
		Topics: []string{"troubleshooting"},
	},
}

// DocSystem provides the offline documentation system
type DocSystem struct {
	topics     map[string]Topic
	categories []Category
	index      map[string][]string // keyword -> topic IDs
}

// NewDocSystem creates a new documentation system
func NewDocSystem() *DocSystem {
	ds := &DocSystem{
		topics:     EmbeddedDocs,
		categories: Categories,
		index:      make(map[string][]string),
	}
	ds.buildIndex()
	return ds
}

// buildIndex builds a keyword index for search
func (ds *DocSystem) buildIndex() {
	for id, topic := range ds.topics {
		// Index by keywords
		for _, kw := range topic.Keywords {
			kw = strings.ToLower(kw)
			ds.index[kw] = append(ds.index[kw], id)
		}
		// Index by title words
		words := strings.Fields(strings.ToLower(topic.Title))
		for _, w := range words {
			ds.index[w] = append(ds.index[w], id)
		}
	}
}

// GetTopic retrieves a topic by ID
func (ds *DocSystem) GetTopic(id string) (Topic, bool) {
	topic, ok := ds.topics[id]
	return topic, ok
}

// ListCategories returns all categories
func (ds *DocSystem) ListCategories() []Category {
	sorted := make([]Category, len(ds.categories))
	copy(sorted, ds.categories)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Order < sorted[j].Order
	})
	return sorted
}

// ListTopics returns all topics in a category
func (ds *DocSystem) ListTopics(categoryID string) []Topic {
	var topics []Topic
	for _, cat := range ds.categories {
		if cat.ID == categoryID {
			for _, topicID := range cat.Topics {
				if topic, ok := ds.topics[topicID]; ok {
					topics = append(topics, topic)
				}
			}
			break
		}
	}
	return topics
}

// Search searches documentation by query
func (ds *DocSystem) Search(query string) []Topic {
	query = strings.ToLower(query)
	words := strings.Fields(query)

	// Score topics by relevance
	scores := make(map[string]int)

	for _, word := range words {
		// Exact keyword match
		if topicIDs, ok := ds.index[word]; ok {
			for _, id := range topicIDs {
				scores[id] += 10
			}
		}

		// Partial match in content
		for id, topic := range ds.topics {
			if strings.Contains(strings.ToLower(topic.Content), word) {
				scores[id] += 1
			}
			if strings.Contains(strings.ToLower(topic.Title), word) {
				scores[id] += 5
			}
		}
	}

	// Sort by score
	type scored struct {
		id    string
		score int
	}
	var results []scored
	for id, score := range scores {
		results = append(results, scored{id, score})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	// Return topics
	var topics []Topic
	for _, r := range results {
		if r.score > 0 {
			topics = append(topics, ds.topics[r.id])
		}
	}

	return topics
}

// GetRelated returns related topics for a given topic
func (ds *DocSystem) GetRelated(topicID string) []Topic {
	topic, ok := ds.topics[topicID]
	if !ok {
		return nil
	}

	var related []Topic
	for _, relatedID := range topic.RelatedIDs {
		if t, ok := ds.topics[relatedID]; ok {
			related = append(related, t)
		}
	}
	return related
}

// Help returns quick help for a command
func (ds *DocSystem) Help(command string) string {
	switch command {
	case "serve":
		return "Start HTTP API server: offgrid serve [--port 8080]"
	case "chat":
		return "Send prompt: offgrid chat \"Your message\""
	case "agent":
		return "Interactive mode: offgrid agent"
	case "models":
		return "Model management: offgrid models [list|download|verify|delete]"
	case "config":
		return "Config management: offgrid config [show|export|import]"
	case "status":
		return "System status: offgrid status"
	default:
		return fmt.Sprintf("No quick help for '%s'. Try: offgrid help docs", command)
	}
}

// TableOfContents returns formatted table of contents
func (ds *DocSystem) TableOfContents() string {
	var sb strings.Builder
	sb.WriteString("# OffGrid LLM Documentation\n\n")

	for _, cat := range ds.ListCategories() {
		sb.WriteString(fmt.Sprintf("## %s\n\n", cat.Title))
		for _, topic := range ds.ListTopics(cat.ID) {
			sb.WriteString(fmt.Sprintf("- %s (offgrid help %s)\n", topic.Title, topic.ID))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
