package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/audio"
	"github.com/takuphilchan/offgrid-llm/internal/models"
)

// getAudioEngine is scoped to this service/workspace, not process-global.
// Status and Models polling reuse it instead of orphaning loaded workers.
func (s *Server) getAudioEngine() (*audio.Engine, error) {
	s.audioMutex.Lock()
	defer s.audioMutex.Unlock()
	dataDir := s.getAudioDataDir()
	if s.audioEngine != nil {
		return s.audioEngine, nil
	}
	cfg := audio.Config{
		DataDir: dataDir,
	}
	engine, err := audio.NewEngine(cfg)
	if err != nil {
		return nil, err
	}
	s.audioEngine = engine
	return engine, nil
}

func (s *Server) getSpeechRuntime() (*audio.PackageRuntime, error) {
	legacy, err := s.getAudioEngine()
	if err != nil {
		return nil, err
	}
	s.audioMutex.Lock()
	defer s.audioMutex.Unlock()
	if s.speechPackages == nil {
		if s.registry == nil {
			return nil, fmt.Errorf("model registry unavailable")
		}
		s.speechPackages = audio.NewPackageRuntime(s.registry.Packages(), s.config.ModelsDir, s.getAudioDataDir(), legacy)
	}
	return s.speechPackages, nil
}

// getAudioDataDir returns the audio data directory
func (s *Server) getAudioDataDir() string {
	return filepath.Join(s.config.DataDir, "audio")
}

// handleAudioTranscriptions handles POST /v1/audio/transcriptions (OpenAI-compatible)
func (s *Server) handleAudioTranscriptions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	audioEngine, initErr := s.getAudioEngine()
	if initErr != nil {
		writeErrorWithCode(w, "Failed to initialize audio engine", http.StatusInternalServerError, "audio_init_error")
		return
	}

	_ = audioEngine
	speech, err := s.getSpeechRuntime()
	if err != nil {
		writeErrorWithCode(w, "Speech runtime is unavailable.", 503, "audio_init_error")
		return
	}

	// Parse multipart form (max 25MB for audio files)
	r.Body = http.MaxBytesReader(w, r.Body, 25<<20)
	if err := r.ParseMultipartForm(25 << 20); err != nil {
		writeErrorWithCode(w, "Failed to parse form data: "+err.Error(), http.StatusBadRequest, "invalid_form")
		return
	}

	defer r.MultipartForm.RemoveAll()

	// Get the audio file
	file, header, err := r.FormFile("file")
	if err != nil {
		writeErrorWithCode(w, "No audio file provided: "+err.Error(), http.StatusBadRequest, "missing_file")
		return
	}
	defer file.Close()

	// Build transcription request
	req := audio.TranscriptionRequest{
		File:           file,
		Filename:       header.Filename,
		Model:          r.FormValue("model"),
		Language:       r.FormValue("language"),
		Prompt:         r.FormValue("prompt"),
		ResponseFormat: r.FormValue("response_format"),
	}

	if req.ResponseFormat == "" {
		req.ResponseFormat = "json"
	}

	// Perform transcription
	result, err := speech.Transcribe(r.Context(), req)
	if err != nil {
		writeErrorWithCode(w, "Failed to transcribe audio: "+err.Error(), http.StatusInternalServerError, "transcription_error")
		return
	}

	// Return response based on format
	switch req.ResponseFormat {
	case "text":
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(result.Text))
	case "verbose_json":
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	default: // json
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"text": result.Text})
	}
}

// handleAudioSpeech handles POST /v1/audio/speech (OpenAI-compatible)
func (s *Server) handleAudioSpeech(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	audioEngine, initErr := s.getAudioEngine()
	if initErr != nil {
		writeErrorWithCode(w, "Failed to initialize audio engine", http.StatusInternalServerError, "audio_init_error")
		return
	}

	_ = audioEngine
	speech, err := s.getSpeechRuntime()
	if err != nil {
		writeErrorWithCode(w, "Speech runtime is unavailable.", 503, "audio_init_error")
		return
	}

	// Parse request body
	var req audio.SpeechRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorWithCode(w, "Failed to parse request: "+err.Error(), http.StatusBadRequest, "invalid_json")
		return
	}

	if req.Input == "" {
		writeErrorWithCode(w, "No input text provided", http.StatusBadRequest, "missing_input")
		return
	}

	if req.Speed == 0 {
		req.Speed = 1.0
	}

	if req.ResponseFormat != "" && req.ResponseFormat != "wav" {
		writeErrorWithCode(w, "This speech endpoint currently supports WAV output only.", 422, "unsupported_audio_format")
		return
	}
	// Generate speech
	audioData, err := speech.Speak(r.Context(), req)
	if err != nil {
		writeErrorWithCode(w, "Failed to generate speech: "+err.Error(), http.StatusInternalServerError, "tts_error")
		return
	}

	// Determine content type
	contentType := "audio/wav"
	switch req.ResponseFormat {
	case "mp3":
		contentType = "audio/mpeg"
	case "opus":
		contentType = "audio/opus"
	case "flac":
		contentType = "audio/flac"
	}

	w.Header().Set("Content-Type", contentType)
	io.Copy(w, audioData)
}

// handleAudioVoices handles GET /v1/audio/voices - lists installed and available voices
func (s *Server) handleAudioVoices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	audioEngine, initErr := s.getAudioEngine()
	if initErr != nil {
		writeErrorWithCode(w, "Failed to initialize audio engine", http.StatusInternalServerError, "audio_init_error")
		return
	}

	// Get installed voices
	installedVoices, _ := audioEngine.ListVoices()
	installedMap := make(map[string]bool)
	for _, v := range installedVoices {
		installedMap[v.Name] = true
	}

	// Get all available voices for download
	availableVoices := audio.ListAvailablePiperVoices()

	// Build response
	type VoiceResponse struct {
		Name      string `json:"name"`
		Language  string `json:"language"`
		Quality   string `json:"quality"`
		Installed bool   `json:"installed"`
	}

	var voices []VoiceResponse
	for _, v := range availableVoices {
		voices = append(voices, VoiceResponse{
			Name:      v.Name,
			Language:  v.Language,
			Quality:   v.Quality,
			Installed: installedMap[v.Name],
		})
	}

	// Also add any installed voices not in the available list (custom voices)
	for _, v := range installedVoices {
		found := false
		for _, av := range availableVoices {
			if av.Name == v.Name {
				found = true
				break
			}
		}
		if !found {
			voices = append(voices, VoiceResponse{
				Name:      v.Name,
				Language:  v.Language,
				Quality:   v.Quality,
				Installed: true,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"voices": voices,
	})
}

// handleAudioWhisperModels handles GET /v1/audio/whisper-models - lists installed and available whisper models
func (s *Server) handleAudioWhisperModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	audioEngine, initErr := s.getAudioEngine()
	if initErr != nil {
		writeErrorWithCode(w, "Failed to initialize audio engine", http.StatusInternalServerError, "audio_init_error")
		return
	}

	// Get installed models
	installedModels, _ := audioEngine.ListWhisperModels()
	installedMap := make(map[string]bool)
	for _, m := range installedModels {
		installedMap[m] = true
	}

	// Get all available models for download
	availableModels := audio.ListAvailableWhisperModels()

	// Build response
	type ModelResponse struct {
		Name      string `json:"name"`
		Size      string `json:"size"`
		Language  string `json:"language"`
		Installed bool   `json:"installed"`
	}

	var models []ModelResponse
	for _, m := range availableModels {
		lang := "Multilingual"
		if strings.HasSuffix(m.Name, ".en") {
			lang = "English only"
		}
		models = append(models, ModelResponse{
			Name:      m.Name,
			Size:      m.Size,
			Language:  lang,
			Installed: installedMap[m.Name],
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"models": models,
	})
}

// handleAudioModels handles GET /v1/audio/models
func (s *Server) handleAudioModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	audioEngine, initErr := s.getAudioEngine()
	if initErr != nil {
		writeErrorWithCode(w, "Failed to initialize audio engine", http.StatusInternalServerError, "audio_init_error")
		return
	}

	models, err := audioEngine.ListWhisperModels()
	if err != nil {
		models = []string{}
	}

	voices, err := audioEngine.ListVoices()
	if err != nil {
		voices = []audio.VoiceInfo{}
	}

	// List available for download
	availableWhisper := audio.ListAvailableWhisperModels()
	availableVoices := audio.ListAvailablePiperVoices()
	speech, err := s.getSpeechRuntime()
	if err != nil {
		writeErrorWithCode(w, "Speech inventory is unavailable.", 503, "speech_inventory_error")
		return
	}
	profiles, err := speech.Profiles(r.Context())
	if err != nil {
		writeErrorWithCode(w, "Speech inventory is unavailable.", 503, "speech_inventory_error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"whisper": map[string]interface{}{
			"installed": models,
			"available": availableWhisper,
		},
		"piper": map[string]interface{}{
			"installed": voices,
			"available": availableVoices,
		},
		"profiles": profiles,
	})
}

// handleAudioStatus handles GET /v1/audio/status
func (s *Server) handleAudioStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	audioEngine, initErr := s.getAudioEngine()
	if initErr != nil {
		writeErrorWithCode(w, "Failed to initialize audio engine", http.StatusInternalServerError, "audio_init_error")
		return
	}

	// Get engine status
	status := audioEngine.Status()
	if speech, err := s.getSpeechRuntime(); err == nil {
		profiles, err := speech.Profiles(r.Context())
		if err != nil {
			writeErrorWithCode(w, "Speech inventory is unavailable.", 503, "speech_inventory_error")
			return
		}
		status["profiles"] = profiles
		for _, profile := range profiles {
			for _, capability := range profile.Capabilities {
				kind := ""
				if capability == models.CapabilityTranscription {
					kind = "asr"
				}
				if capability == models.CapabilitySynthesis {
					kind = "tts"
				}
				if kind == "" {
					continue
				}
				entry := status[kind].(map[string]interface{})
				if profile.Available && entry["available"] != true {
					entry["available"] = true
					entry["model"] = profile.ID
					entry["adapter"] = profile.Adapter
					delete(entry, "issue")
				} else if entry["available"] != true && entry["issue"] == nil {
					entry["issue"] = profile.Issue
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// handleAudioDownload handles POST /v1/audio/download
func (s *Server) handleAudioDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	audioEngine, initErr := s.getAudioEngine()
	if initErr != nil {
		writeErrorWithCode(w, "Failed to initialize audio engine", http.StatusInternalServerError, "audio_init_error")
		return
	}

	var req struct {
		Type string `json:"type"` // "whisper" or "piper"
		Name string `json:"name"` // model/voice name
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorWithCode(w, "Failed to parse request: "+err.Error(), http.StatusBadRequest, "invalid_json")
		return
	}

	var err error
	switch strings.ToLower(req.Type) {
	case "whisper":
		err = audioEngine.DownloadWhisperModel(req.Name, func(downloaded, total int64) {
			// Progress callback - could be used for streaming progress
		})
	case "piper", "voice":
		err = audioEngine.DownloadPiperVoice(req.Name, func(downloaded, total int64) {
			// Progress callback
		})
	default:
		writeErrorWithCode(w, "Type must be 'whisper' or 'piper'", http.StatusBadRequest, "invalid_type")
		return
	}

	if err != nil {
		writeErrorWithCode(w, fmt.Sprintf("Failed to download %s: %s", req.Type, err.Error()), http.StatusInternalServerError, "download_error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Downloaded %s: %s", req.Type, req.Name),
	})
}

// handleAudioSetupWhisper handles POST /v1/audio/setup/whisper
func (s *Server) handleAudioSetupWhisper(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	audioEngine, initErr := s.getAudioEngine()
	if initErr != nil {
		writeErrorWithCode(w, "Failed to initialize audio engine", http.StatusInternalServerError, "audio_init_error")
		return
	}

	var req struct {
		Model         string `json:"model"`
		InstallBinary bool   `json:"install_binary"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorWithCode(w, "Failed to parse request: "+err.Error(), http.StatusBadRequest, "invalid_json")
		return
	}

	// Check if whisper binary is available, if not install it first
	if !audioEngine.HasWhisperBinary() || req.InstallBinary {
		err := audioEngine.DownloadWhisperBinary(nil)
		if err != nil {
			writeErrorWithCode(w, fmt.Sprintf("Failed to install Whisper.cpp: %s", err.Error()), http.StatusInternalServerError, "download_error")
			return
		}

		// If only binary install was requested, return now
		if req.InstallBinary && req.Model == "" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"message": "Whisper.cpp installed successfully",
			})
			return
		}
	}

	// Download model
	if req.Model == "" {
		req.Model = "base"
	}

	err := audioEngine.DownloadWhisperModel(req.Model, nil)
	if err != nil {
		writeErrorWithCode(w, fmt.Sprintf("Failed to download Whisper model: %s", err.Error()), http.StatusInternalServerError, "download_error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Whisper %s model installed successfully", req.Model),
	})
}

// handleAudioSetupPiper handles POST /v1/audio/setup/piper
func (s *Server) handleAudioSetupPiper(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	audioEngine, initErr := s.getAudioEngine()
	if initErr != nil {
		writeErrorWithCode(w, "Failed to initialize audio engine", http.StatusInternalServerError, "audio_init_error")
		return
	}

	var req struct {
		Voice         string `json:"voice"`
		InstallBinary bool   `json:"install_binary"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorWithCode(w, "Failed to parse request: "+err.Error(), http.StatusBadRequest, "invalid_json")
		return
	}

	// Install binary if requested
	if req.InstallBinary {
		err := audioEngine.DownloadPiperBinary(nil)
		if err != nil {
			writeErrorWithCode(w, fmt.Sprintf("Failed to install Piper: %s", err.Error()), http.StatusInternalServerError, "download_error")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Piper installed successfully",
		})
		return
	}

	// Download voice
	if req.Voice == "" {
		req.Voice = "en_US-amy-medium"
	}

	err := audioEngine.DownloadPiperVoice(req.Voice, nil)
	if err != nil {
		writeErrorWithCode(w, fmt.Sprintf("Failed to download Piper voice: %s", err.Error()), http.StatusInternalServerError, "download_error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Piper voice %s installed successfully", req.Voice),
	})
}
