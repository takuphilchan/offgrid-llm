# OffGrid LLM Architecture

> Technical overview of the OffGrid LLM system design.

---

## Table of Contents

- [System Overview](#system-overview)
- [Component Architecture](#component-architecture)
- [Request Flow](#request-flow)
- [Key Subsystems](#key-subsystems)
- [Data Flow](#data-flow)
- [Configuration](#configuration)
- [Extension Points](#extension-points)

---

## System Overview

OffGrid LLM is a **locally-running AI inference server** that provides:

- HTTP API for chat completions (OpenAI-compatible)
- Web-based user interface
- Voice input/output (Whisper STT, Piper TTS)
- RAG/knowledge base capabilities
- Multi-model management

### Design Principles

1. **Offline-First** - All core functionality works without internet
2. **Privacy-Preserving** - Data never leaves the local machine
3. **Resource-Aware** - Graceful degradation under load
4. **Modular** - Components can be enabled/disabled
5. **API-Compatible** - OpenAI API compatibility for easy migration

---

## Component Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              Clients                                     │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐       │
│  │ Web UI  │  │ Python  │  │  cURL   │  │ Desktop │  │  Other  │       │
│  │(Browser)│  │   SDK   │  │   CLI   │  │   App   │  │ Clients │       │
│  └────┬────┘  └────┬────┘  └────┬────┘  └────┬────┘  └────┬────┘       │
└───────┼────────────┼────────────┼────────────┼────────────┼─────────────┘
        │            │            │            │            │
        └────────────┴────────────┴─────┬──────┴────────────┘
                                        │
                                        ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                           HTTP Server (:11611)                           │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │                        Middleware Stack                           │   │
│  │   ┌─────────┐   ┌─────────┐   ┌─────────┐   ┌─────────┐         │   │
│  │   │ Logging │ → │  Auth   │ → │  CORS   │ → │  Rate   │         │   │
│  │   │         │   │         │   │         │   │ Limit   │         │   │
│  │   └─────────┘   └─────────┘   └─────────┘   └─────────┘         │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                                                          │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐         │
│  │   REST Router   │  │ WebSocket Hub   │  │  Static Files   │         │
│  │ /api/v1/*       │  │ /api/v1/ws      │  │ /ui/*           │         │
│  └────────┬────────┘  └────────┬────────┘  └─────────────────┘         │
└───────────┼────────────────────┼────────────────────────────────────────┘
            │                    │
            ▼                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                          Service Layer                                   │
│                                                                          │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐         │
│  │    Inference    │  │      RAG        │  │     Agents      │         │
│  │    Engine       │  │    Engine       │  │    Executor     │         │
│  │  ┌───────────┐  │  │  ┌───────────┐  │  │  ┌───────────┐  │         │
│  │  │ llama.cpp │  │  │  │ Vector DB │  │  │  │MCP Server │  │         │
│  │  │ bindings  │  │  │  │ Embeddings│  │  │  │Tool Calls │  │         │
│  │  └───────────┘  │  │  └───────────┘  │  │  └───────────┘  │         │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘         │
│                                                                          │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐         │
│  │     Audio       │  │    Sessions     │  │     Models      │         │
│  │    Service      │  │    Manager      │  │    Manager      │         │
│  │  ┌───────────┐  │  │  ┌───────────┐  │  │  ┌───────────┐  │         │
│  │  │  Whisper  │  │  │  │ Chat State│  │  │  │HuggingFace│  │         │
│  │  │  Piper    │  │  │  │ Persistence│ │  │  │ Registry  │  │         │
│  │  └───────────┘  │  │  └───────────┘  │  │  └───────────┘  │         │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘         │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
            │
            ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                        Support Services                                  │
│  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐    │
│  │Watchdog│ │Metrics │ │ Audit  │ │ Cache  │ │ Config │ │Degrade │    │
│  │        │ │        │ │        │ │        │ │        │ │        │    │
│  │Health  │ │Stats   │ │Security│ │Response│ │Settings│ │Fallback│    │
│  │Monitor │ │Tracking│ │Logging │ │Cache   │ │Export  │ │Handler │    │
│  └────────┘ └────────┘ └────────┘ └────────┘ └────────┘ └────────┘    │
└─────────────────────────────────────────────────────────────────────────┘
            │
            ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                          Storage Layer                                   │
│  ┌────────────────┐  ┌────────────────┐  ┌────────────────┐            │
│  │   Model Files  │  │   Vector Store │  │   Config Files │            │
│  │ ~/.offgrid-llm │  │   (embedded)   │  │    settings    │            │
│  │    /models/    │  │                │  │                │            │
│  └────────────────┘  └────────────────┘  └────────────────┘            │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Request Flow

### Chat Completion Request

```
Client                 Server                    Inference
  │                      │                          │
  │ POST /chat/completions                          │
  │ ─────────────────────►                          │
  │                      │                          │
  │                      │ Validate request         │
  │                      │ Check auth               │
  │                      │                          │
  │                      │ Load model (if needed)   │
  │                      │ ─────────────────────────►
  │                      │                          │
  │                      │ RAG lookup (if enabled)  │
  │                      │ ◄─────────────────────── │
  │                      │                          │
  │                      │ Generate tokens          │
  │                      │ ─────────────────────────►
  │                      │                          │
  │ SSE: token chunks    │ ◄───────────────────────│
  │ ◄─────────────────── │         (streaming)     │
  │ ...                  │                          │
  │ SSE: [DONE]          │                          │
  │ ◄─────────────────── │                          │
  │                      │                          │
```

### WebSocket Streaming

```
Client                 Server                    Inference
  │                      │                          │
  │ WS Connect           │                          │
  │ ═════════════════════►                          │
  │                      │                          │
  │ {"type":"chat",...}  │                          │
  │ ─────────────────────►                          │
  │                      │                          │
  │                      │ Process & generate       │
  │                      │ ─────────────────────────►
  │                      │                          │
  │ {"type":"token",...} │                          │
  │ ◄═════════════════════                          │
  │ ...                  │                          │
  │ {"type":"done",...}  │                          │
  │ ◄═════════════════════                          │
  │                      │                          │
```

---

## Key Subsystems

### 1. Inference Engine (`internal/inference`)

The core LLM execution engine built on llama.cpp.

```go
type Engine struct {
    model       *llama.Model      // Loaded model
    contextSize int               // Token context window
    gpuLayers   int               // Layers offloaded to GPU
    threads     int               // CPU threads
}

// Key methods
func (e *Engine) Load(modelPath string) error
func (e *Engine) Generate(ctx context.Context, prompt string, opts Options) (chan string, error)
func (e *Engine) Unload() error
```

**Responsibilities:**
- Model loading/unloading
- Token generation with streaming
- GPU/CPU resource management
- Context window handling

### 2. RAG Engine (`internal/rag`)

Retrieval-Augmented Generation for document context.

```go
type Engine struct {
    vectorStore  VectorStore       // Embedding storage
    embedder     Embedder          // Embedding model
    chunker      Chunker           // Document splitter
}

// Key methods
func (e *Engine) AddDocument(path string) error
func (e *Engine) Search(query string, k int) []Document
func (e *Engine) BuildPrompt(query string, docs []Document) string
```

**Flow:**
1. Documents chunked into segments
2. Chunks embedded via embedding model
3. Embeddings stored in vector DB
4. Query embedded and matched
5. Relevant chunks injected into prompt

### 3. Agent System (`internal/agents`)

Autonomous task execution with tool use.

```go
type Agent struct {
    model       string            // LLM for reasoning
    tools       []Tool            // Available tools
    maxSteps    int               // Iteration limit
}

// Key methods
func (a *Agent) Execute(ctx context.Context, task string) (Result, error)
func (a *Agent) RegisterTool(tool Tool) error
```

**Tool Types:**
- File operations (read/write/list)
- Web search
- Code execution
- MCP server tools
- Custom tools

### 4. Audio Service (`internal/audio`)

Voice input/output capabilities.

```go
type Service struct {
    whisper     *WhisperModel     // Speech-to-text
    piper       *PiperEngine      // Text-to-speech
}

// Key methods
func (s *Service) Transcribe(audio []byte, opts TranscribeOpts) (string, error)
func (s *Service) Synthesize(text string, voice string) ([]byte, error)
```

**Supported:**
- 18+ languages for STT
- Multiple TTS voices
- Streaming audio

### 5. Session Manager (`internal/sessions`)

Chat history and state persistence.

```go
type Manager struct {
    sessions    map[string]*Session
    storage     Storage
}

type Session struct {
    ID          string
    Messages    []Message
    Model       string
    CreatedAt   time.Time
}
```

---

## Data Flow

### Model Loading

```
1. User requests model load
         │
         ▼
2. Check if model exists in ~/.offgrid-llm/models/
         │
         ├─── No: Return error (or trigger download)
         │
         ▼ Yes
3. Initialize llama.cpp context
         │
         ▼
4. Load model weights into memory
         │
         ├─── CPU: Load to RAM
         │
         └─── GPU: Offload layers to VRAM
         │
         ▼
5. Model ready for inference
```

### RAG Pipeline

```
Document Input                    Query Processing
     │                                  │
     ▼                                  ▼
┌─────────────┐                  ┌─────────────┐
│   Chunker   │                  │   Embedder  │
│  (split)    │                  │   (query)   │
└──────┬──────┘                  └──────┬──────┘
       │                                │
       ▼                                ▼
┌─────────────┐                  ┌─────────────┐
│   Embedder  │                  │  VectorDB   │
│  (chunks)   │                  │  (search)   │
└──────┬──────┘                  └──────┬──────┘
       │                                │
       ▼                                ▼
┌─────────────┐                  ┌─────────────┐
│  VectorDB   │                  │   Results   │
│  (store)    │                  │ (top-k docs)│
└─────────────┘                  └──────┬──────┘
                                        │
                                        ▼
                                 ┌─────────────┐
                                 │   Prompt    │
                                 │  Builder    │
                                 └──────┬──────┘
                                        │
                                        ▼
                                 ┌─────────────┐
                                 │  Inference  │
                                 └─────────────┘
```

---

## Configuration

### Environment Variables

```bash
# Server
OFFGRID_PORT=11611              # HTTP port
OFFGRID_HOST=0.0.0.0            # Bind address

# Paths
OFFGRID_MODELS_DIR=~/.offgrid-llm/models
OFFGRID_DATA_DIR=~/.offgrid-llm/data

# Inference
OFFGRID_GPU_LAYERS=0            # GPU layer count
OFFGRID_THREADS=4               # CPU threads
OFFGRID_CONTEXT_SIZE=4096       # Context window

# Features
OFFGRID_MULTI_USER=false        # Enable user management
OFFGRID_REQUIRE_AUTH=false      # Require authentication
```

### Config Files

```
~/.offgrid-llm/
├── config.yaml                 # Main configuration
├── models/                     # Model storage
├── data/                       # Application data
│   ├── vectorstore/            # RAG embeddings
│   └── sessions/               # Chat history
└── audio/                      # Voice models
    ├── whisper/                # STT models
    └── piper/                  # TTS voices
```

---

## Extension Points

### Adding New Tools (Agents)

```go
// internal/tools/mytool.go
type MyTool struct{}

func (t *MyTool) Name() string {
    return "my_tool"
}

func (t *MyTool) Description() string {
    return "Does something useful"
}

func (t *MyTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
    // Implementation
    return "result", nil
}

// Register in agent
agent.RegisterTool(&MyTool{})
```

### Adding New API Endpoints

```go
// internal/server/handlers.go
func (s *Server) handleMyEndpoint(w http.ResponseWriter, r *http.Request) {
    // Handle request
}

// internal/server/server.go (in route setup)
mux.HandleFunc("/api/v1/myendpoint", s.handleMyEndpoint)
```

### Adding New Middleware

```go
// internal/server/middleware.go
func (s *Server) myMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Pre-processing
        next.ServeHTTP(w, r)
        // Post-processing
    })
}
```

---

## Related Documentation

- [Distribution Strategy](distribution.md) - Offline model distribution
- [Performance Tuning](performance.md) - Optimization guide
- [Building from Source](building.md) - Compilation
- [Contributing](../../dev/CONTRIBUTING.md) - Development guide
