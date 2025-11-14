# Model Distribution & Offline Strategy

## The Challenge

OffGrid LLM is designed for **truly offline environments** where internet may be:
- Completely unavailable (ships, remote clinics, air-gapped networks)
- Intermittent (rural areas, disaster zones)
- Expensive/metered (satellite connections)

Unlike Ollama (assumes internet for initial download), we need **multiple distribution paths**.

## Distribution Strategy

### Tier 1: Online Bootstrap (Internet Available)

For users with internet, provide easy downloads from existing sources:

```bash
# Download from Hugging Face (TheBloke's quantized models)
offgrid download llama-2-7b-chat --quantization Q4_K_M

# Under the hood: wget/curl from HuggingFace CDN
# https://huggingface.co/TheBloke/Llama-2-7B-Chat-GGUF/
```

**Advantages:**
- No hosting costs for us
- Leverage existing infrastructure
- Always up-to-date models

**Implementation:**
- Simple wrapper around HuggingFace URLs
- No need to host models ourselves
- Built-in integrity checks (SHA256)

### Tier 2: Peer-to-Peer Distribution (Local Network)

**The core innovation:** Share models across local network

```
Ship Network Example:
┌─────────────┐         ┌─────────────┐
│  Bridge PC  │◄───────►│ Engine Room │
│ (has models)│  LAN    │ (downloads) │
└─────────────┘         └─────────────┘
       ▲                       ▲
       │      P2P Sync         │
       └───────────────────────┘
```

**How it works:**
1. One computer downloads models (while at port/internet)
2. Other computers on ship discover peers via UDP broadcast
3. Models transfer over local network (no internet needed)

**Implementation Details:**

```go
// Enhanced P2P discovery
type ModelAnnouncement struct {
    PeerID    string
    Address   string
    Models    []ModelInfo
    Bandwidth int // Available transfer speed
}

// Peer requests model
func (p *P2P) RequestModel(modelID, peerID string) error {
    // 1. Find peer with model
    peer := p.FindPeerWithModel(modelID)
    
    // 2. Negotiate transfer (HTTP or raw TCP)
    // 3. Stream model file with resume support
    // 4. Verify integrity
    // 5. Install to local registry
}
```

### Tier 3: Physical Media Distribution (USB/SD Card)

For **completely offline** deployment:

```bash
# Prepare distribution package on internet-connected computer
offgrid pack-usb /media/usb --models llama-2-7b,mistral-7b

# Result:
# /media/usb/
#   ├── offgrid-linux-amd64     (binary)
#   ├── offgrid-windows.exe
#   ├── models/
#   │   ├── llama-2-7b.Q4_K_M.gguf
#   │   └── mistral-7b.Q4_K_M.gguf
#   └── install.sh

# On offline machine:
./install.sh
# Automatically copies binary and models to correct locations
```

**Real-world scenario:**
- Medical NGO downloads models at HQ
- Copies to USB drives
- Distributes to 50 rural clinics
- Clinics never need internet

### Tier 4: Sneakernet Updates

**Monthly update workflow:**

```bash
# At HQ (with internet)
offgrid create-update-pack --since 2024-10 --output october-update.tar.gz

# Contains:
# - New model versions
# - Security patches
# - Updated binary
# - Changelog

# Ship to field offices
# Apply update offline
offgrid apply-update october-update.tar.gz
```

## Deep Dive: Why This Solves Real Problems

### Problem 1: Maritime Operations
**Challenge:** Ships at sea for months, need AI for:
- Navigation assistance
- Documentation translation
- Technical manuals Q&A

**Solution:**
- Download models before departure (Tier 1)
- Share across ship network (Tier 2)
- USB backup from home office (Tier 3)

### Problem 2: Rural Healthcare
**Challenge:** Clinic with satellite internet ($50/GB)

**Solution:**
- Monthly model delivery via courier (Tier 3)
- P2P sharing between clinic departments (Tier 2)
- No expensive satellite downloads needed

### Problem 3: Air-Gapped Security Networks
**Challenge:** Government/military networks with no external connectivity

**Solution:**
- Only Tier 3 (USB) distribution allowed
- Internal P2P for efficiency
- Formal approval process for new models

## Implementation Roadmap

### Phase 1: Foundation (Completed ✅)
- Model registry
- Basic file management
- P2P discovery skeleton

### Phase 2: Core Distribution (Next)
```go
// Add to internal/models/downloader.go
type Downloader struct {
    sources []ModelSource // HuggingFace, local peers, USB
}

func (d *Downloader) Download(modelID string) error {
    // 1. Try local peers first (free, fast)
    if model, err := d.tryPeers(modelID); err == nil {
        return d.installFromPeer(model)
    }
    
    // 2. Try internet sources
    if model, err := d.tryInternet(modelID); err == nil {
        return d.installFromURL(model)
    }
    
    // 3. Prompt for USB
    return fmt.Errorf("model not found, please insert USB with model")
}
```

### Phase 3: P2P Transfer
```go
// internal/p2p/transfer.go
type Transfer struct {
    ModelID     string
    SourcePeer  string
    Progress    float64
    BytesTotal  int64
    BytesDone   int64
    Resumable   bool
}

// HTTP-based transfer with resume support
func (t *Transfer) Start() error {
    // Support Range requests for resumable downloads
    // Chunk-based transfer with verification
    // Bandwidth throttling (don't saturate network)
}
```

## Cost Analysis

**Traditional Approach (Ollama-style):**
- Need servers to host models
- 7B model = ~4GB × 1000 downloads = 4TB transfer/month
- AWS S3: ~$40/TB = $160/month minimum
- Scaling costs with users

**OffGrid Approach:**
- Zero hosting costs (use HuggingFace CDN)
- P2P reduces internet dependency
- Physical media for extreme cases
- Cost scales with USB drives, not bandwidth

## Model Storage Locations

### Default Paths
```
Linux/Mac:   ~/.offgrid-llm/models/
Windows:     %USERPROFILE%\.offgrid-llm\models\
Custom:      Set OFFGRID_MODELS_DIR env variable
```

### Shared Network Storage
```bash
# Multiple machines sharing NFS/SMB
export OFFGRID_MODELS_DIR=/mnt/shared/models

# All machines read from shared storage
# No duplication needed
```

### USB/External Storage
```bash
# Run directly from USB (portable mode)
offgrid --models-dir /media/usb/models --portable

# No installation needed
# Useful for demo/testing
```

## Security Considerations

1. **Model Verification:**
   ```go
   // Verify SHA256 before loading
   func VerifyModel(path string, expectedHash string) error {
       hash := sha256.Sum256(fileContent)
       if hex.EncodeToString(hash[:]) != expectedHash {
           return errors.New("model integrity check failed")
       }
   }
   ```

2. **P2P Trust:**
   - Optional peer authentication
   - Verify models match known hashes
   - Reject unknown/modified files

3. **Sandboxing:**
   - Models run in isolated processes
   - No network access from inference engine
   - Limited filesystem access

## Next Implementation Steps

1. **Model Catalog** - JSON file with trusted model sources
2. **Download Manager** - Resume support, parallel chunks
3. **P2P File Transfer** - HTTP-based with encryption option
4. **USB Pack/Unpack** - Automated distribution package creation
5. **Update System** - Diff-based updates for efficiency

This approach makes OffGrid LLM viable for **truly offline** scenarios while remaining easy to use when internet is available.
