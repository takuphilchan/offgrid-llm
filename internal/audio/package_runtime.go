package audio

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/models"
)

// SpeechProfile separates package compatibility from successful inference. A
// catalog architecture does not imply that its adapter is installed or tested.
type SpeechProfile struct {
	ID           string                   `json:"id"`
	Revision     string                   `json:"revision"`
	Name         string                   `json:"name"`
	Architecture string                   `json:"architecture"`
	Adapter      string                   `json:"adapter"`
	Capabilities []models.ModelCapability `json:"capabilities"`
	Languages    []string                 `json:"languages"`
	Voices       []models.PackageVoice    `json:"voices,omitempty"`
	Available    bool                     `json:"available"`
	SmokeTested  bool                     `json:"smoke_tested"`
	Issue        string                   `json:"issue,omitempty"`
	manifest     *models.ModelPackageManifest
	dir          string
}

// PackageRuntime is the single architecture router for managed speech packages.
// Installation remains in PackageStore. It owns one loaded package and its
// lease; status polling neither loads models nor changes worker ownership.
type PackageRuntime struct {
	store         *models.PackageStore
	root, dataDir string
	legacy        *Engine
	gate          chan struct{}
	engine        *Engine
	key           string
	release       func()
	idle          *time.Timer
	closed        bool
	generation    uint64
	mu            sync.Mutex
	smoke         map[string]bool
	modules       map[string]bool
	probeAt       time.Time
}

func NewPackageRuntime(store *models.PackageStore, root, dataDir string, legacy *Engine) *PackageRuntime {
	return &PackageRuntime{store: store, root: root, dataDir: dataDir, legacy: legacy, gate: make(chan struct{}, 1), smoke: map[string]bool{}}
}

func speechPython() string {
	if p := os.Getenv("OFFGRID_SPEECH_PYTHON"); p != "" {
		return p
	}
	return "python3"
}

func (p *PackageRuntime) pythonModules(ctx context.Context) map[string]bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if time.Since(p.probeAt) < time.Minute {
		return p.modules
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	// Probe module metadata without importing inference libraries or loading models.
	cmd := exec.CommandContext(ctx, speechPython(), "-c", `import importlib.util,json; print(json.dumps({m:importlib.util.find_spec(m) is not None for m in ('qwen_asr','qwen_tts')}))`)
	var found map[string]bool
	if b, err := cmd.Output(); err == nil {
		_ = json.Unmarshal(b, &found)
	}
	p.modules, p.probeAt = found, time.Now()
	return found
}

func (p *PackageRuntime) Profiles(ctx context.Context) ([]SpeechProfile, error) {
	states, err := p.store.List()
	if err != nil {
		return nil, err
	}
	var modules map[string]bool
	profiles := []SpeechProfile{}
	for _, state := range states {
		m := state.Manifest
		if m == nil {
			continue
		}
		profile := SpeechProfile{ID: m.ID, Revision: m.Revision, Name: m.Name, Architecture: m.Architecture, Adapter: m.Runtime.Adapter, Languages: m.Languages, Voices: m.Voices, manifest: m, dir: filepath.Join(p.root, models.PackageDirectory, m.ID, m.Revision)}
		// This HTTP adapter does file recognition and complete WAV chunks; model
		// claims of native streaming do not enable a streaming service contract.
		for _, capability := range m.Capabilities {
			if capability == models.CapabilityTranscription || capability == models.CapabilitySynthesis {
				profile.Capabilities = append(profile.Capabilities, capability)
			}
		}
		switch {
		case !state.Installed || state.Integrity == "failed":
			profile.Issue = "Model package needs verification or repair."
		case m.Validate() != nil:
			profile.Issue = "Model package manifest is incompatible."
		default:
			switch m.Architecture {
			case "whisper":
				profile.Available = p.legacy.HasWhisperBinary()
				if !profile.Available {
					profile.Issue = "Install the whisper.cpp speech runtime for this model."
				}
			case "piper":
				profile.Available = p.legacy.HasPiperBinary()
				if !profile.Available {
					profile.Issue = "Install the Piper speech runtime for this model."
				}
			case "qwen3-asr", "qwen3-tts":
				if modules == nil {
					modules = p.pythonModules(ctx)
				}
				module := "qwen_asr"
				if m.Architecture == "qwen3-tts" {
					module = "qwen_tts"
				}
				profile.Available = modules[module]
				if !profile.Available {
					profile.Issue = "Install the matching " + m.Runtime.Adapter + " speech runtime for this model."
				}
				if m.Architecture == "qwen3-tts" && !SupportsQwenDirectSpeech(profile.dir) {
					profile.Available = false
					profile.Issue = "Response playback requires Qwen CustomVoice. Base requires reference audio; Base and VoiceDesign are not supported for read-aloud."
				}
			default:
				profile.Issue = "This build has no executable " + m.Runtime.Adapter + " adapter for " + m.Architecture + ". Installed files are preserved; select a supported runtime/model."
			}
		}
		p.mu.Lock()
		profile.SmokeTested = p.smoke[m.ID+"/"+m.Revision]
		p.mu.Unlock()
		profiles = append(profiles, profile)
	}
	return profiles, nil
}

// AdapterAvailable is a lightweight pre-download runtime inspection. A true
// result is not model compatibility or a successful inference claim.
func (p *PackageRuntime) AdapterAvailable(ctx context.Context, adapter string) bool {
	switch adapter {
	case "whisper.cpp":
		return p.legacy.HasWhisperBinary()
	case "piper":
		return p.legacy.HasPiperBinary()
	case "qwen3-asr":
		return p.pythonModules(ctx)["qwen_asr"]
	case "qwen3-tts":
		return p.pythonModules(ctx)["qwen_tts"]
	default:
		return false
	}
}

func matchesSpeechModel(profile SpeechProfile, requested string) bool {
	if requested == profile.ID || requested == profile.ID+"@"+profile.Revision || requested == profile.Name {
		return true
	}
	for _, artifact := range profile.manifest.Artifacts {
		if artifact.Source != nil && strings.TrimPrefix(artifact.Source.Repository, "https://huggingface.co/") == requested {
			return true
		}
	}
	return false
}

func (p *PackageRuntime) selectProfile(ctx context.Context, requested string, capability models.ModelCapability) (*SpeechProfile, error) {
	profiles, err := p.Profiles(ctx)
	if err != nil {
		return nil, err
	}
	var unavailable string
	for _, profile := range profiles {
		matches := requested != "" && matchesSpeechModel(profile, requested)
		if requested != "" && !matches {
			continue
		}
		if !slices.Contains(profile.Capabilities, capability) {
			if matches {
				return nil, fmt.Errorf("selected model does not support %s", capability)
			}
			continue
		}
		if profile.Available {
			return &profile, nil
		}
		if matches {
			return nil, fmt.Errorf("%s", profile.Issue)
		}
		if unavailable == "" {
			unavailable = profile.Issue
		}
	}
	// Named legacy models still use legacy lookup; never silently use another
	// managed model when an explicit model ID was supplied.
	if requested == "" && unavailable != "" {
		legacyReady := p.legacy.IsASRAvailable()
		if capability == models.CapabilitySynthesis {
			legacyReady = p.legacy.IsTTSAvailable()
		}
		if !legacyReady {
			return nil, fmt.Errorf("%s", unavailable)
		}
	}
	return nil, nil
}

func (p *PackageRuntime) unload() {
	if p.idle != nil {
		p.idle.Stop()
		p.idle = nil
	}
	if p.engine != nil {
		p.engine.Close()
		p.engine = nil
	}
	if p.release != nil {
		p.release()
		p.release = nil
	}
	p.key = ""
}

func (p *PackageRuntime) Close() {
	p.gate <- struct{}{}
	defer func() { <-p.gate }()
	p.closed = true
	p.unload()
}

func (p *PackageRuntime) withEngine(ctx context.Context, model string, capability models.ModelCapability, run func(context.Context, *Engine, bool) error) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	select {
	case p.gate <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-p.gate }()
	if p.closed {
		return fmt.Errorf("speech service is shutting down")
	}
	profile, err := p.selectProfile(ctx, model, capability)
	if err != nil {
		return err
	}
	p.generation++
	generation := p.generation
	if p.idle != nil {
		p.idle.Stop()
	}
	if profile == nil {
		p.unload()
		return run(ctx, p.legacy, false)
	}
	key := profile.ID + "/" + profile.Revision
	if p.key != key {
		p.unload()
		_, release, err := p.store.Acquire(ctx, profile.ID, profile.Revision, "speech playback/transcription")
		if err != nil {
			return fmt.Errorf("verify speech package: %w", err)
		}
		cfg := Config{DataDir: p.dataDir, WhisperPath: p.legacy.whisperPath, PiperPath: p.legacy.piperPath}
		switch profile.Architecture {
		case "qwen3-asr":
			cfg.Runtime.ASRModelDir = profile.dir
		case "qwen3-tts":
			cfg.Runtime.TTSModelDir = profile.dir
		case "whisper", "piper":
			for _, artifact := range profile.manifest.Artifacts {
				path := filepath.Join(profile.dir, filepath.FromSlash(artifact.Path))
				if artifact.Role == "weights" {
					if profile.Architecture == "whisper" {
						cfg.Runtime.WhisperModelPath = path
					} else {
						cfg.Runtime.PiperModelPath = path
					}
				}
				if artifact.Role == "config" {
					cfg.Runtime.PiperConfigPath = path
				}
			}
		default:
			release()
			return fmt.Errorf("speech adapter is unavailable")
		}
		engine, err := NewEngine(cfg)
		if err != nil {
			release()
			return err
		}
		p.engine, p.key, p.release = engine, key, release
	}
	err = run(ctx, p.engine, true)
	if err != nil {
		p.unload()
		return err
	}
	p.mu.Lock()
	p.smoke[key] = true
	p.mu.Unlock()
	// Keep short follow-up chunks warm, then release memory and the package lease.
	p.idle = time.AfterFunc(30*time.Second, func() {
		select {
		case p.gate <- struct{}{}:
			defer func() { <-p.gate }()
			if p.key == key && p.generation == generation {
				p.unload()
			}
		default:
		}
	})
	return nil
}

func (p *PackageRuntime) Speak(ctx context.Context, req SpeechRequest) (io.Reader, error) {
	if req.Model == "tts-1" {
		req.Model = ""
	}
	if req.ResponseFormat != "" && req.ResponseFormat != "wav" {
		return nil, fmt.Errorf("speech output supports WAV only")
	}
	if len(strings.TrimSpace(req.Input)) == 0 || len(req.Input) > 12000 {
		return nil, fmt.Errorf("speech input must contain 1 to 12000 bytes of text")
	}
	var out io.Reader
	err := p.withEngine(ctx, req.Model, models.CapabilitySynthesis, func(ctx context.Context, e *Engine, managed bool) (err error) {
		if managed {
			req.Model = ""
		}
		out, err = e.SpeakContext(ctx, req)
		return err
	})
	return out, err
}

func (p *PackageRuntime) Transcribe(ctx context.Context, req TranscriptionRequest) (*TranscriptionResponse, error) {
	if req.Model == "whisper-1" {
		req.Model = ""
	}
	var out *TranscriptionResponse
	err := p.withEngine(ctx, req.Model, models.CapabilityTranscription, func(ctx context.Context, e *Engine, managed bool) (err error) {
		if managed {
			req.Model = ""
		}
		out, err = e.TranscribeContext(ctx, req)
		return err
	})
	return out, err
}
