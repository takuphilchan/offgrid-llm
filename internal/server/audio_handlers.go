package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/audio"
)

// audioEngine is the global audio engine instance
var audioEngine *audio.Engine

// initAudioEngine initializes the audio engine
func initAudioEngine(dataDir string) error {
	cfg := audio.Config{
		DataDir: dataDir,
	}
	var err error
	audioEngine, err = audio.NewEngine(cfg)
	return err
}

// getAudioDataDir returns the audio data directory
func (s *Server) getAudioDataDir() string {
	return filepath.Join(s.config.ModelsDir, "..", "audio")
}

// handleAudioTranscriptions handles POST /v1/audio/transcriptions (OpenAI-compatible)
func (s *Server) handleAudioTranscriptions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Initialize audio engine if needed
	if audioEngine == nil {
		if err := initAudioEngine(s.getAudioDataDir()); err != nil {
			writeErrorWithCode(w, "Failed to initialize audio engine: "+err.Error(), http.StatusInternalServerError, "audio_init_error")
			return
		}
	}

	if !audioEngine.IsASRAvailable() {
		writeErrorWithCode(w, "Speech-to-text not available. Run: offgrid audio setup whisper", http.StatusServiceUnavailable, "asr_not_available")
		return
	}

	// Parse multipart form (max 25MB for audio files)
	if err := r.ParseMultipartForm(25 << 20); err != nil {
		writeErrorWithCode(w, "Failed to parse form data: "+err.Error(), http.StatusBadRequest, "invalid_form")
		return
	}

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
	result, err := audioEngine.Transcribe(req)
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

	// Initialize audio engine if needed
	if audioEngine == nil {
		if err := initAudioEngine(s.getAudioDataDir()); err != nil {
			writeErrorWithCode(w, "Failed to initialize audio engine: "+err.Error(), http.StatusInternalServerError, "audio_init_error")
			return
		}
	}

	if !audioEngine.IsTTSAvailable() {
		writeErrorWithCode(w, "Text-to-speech not available. Install piper and download a voice. Run: offgrid audio setup piper", http.StatusServiceUnavailable, "tts_not_available")
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

	// Generate speech
	audioData, err := audioEngine.Speak(req)
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

	// Initialize audio engine if needed
	if audioEngine == nil {
		if err := initAudioEngine(s.getAudioDataDir()); err != nil {
			writeErrorWithCode(w, "Failed to initialize audio engine: "+err.Error(), http.StatusInternalServerError, "audio_init_error")
			return
		}
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

	// Initialize audio engine if needed
	if audioEngine == nil {
		if err := initAudioEngine(s.getAudioDataDir()); err != nil {
			writeErrorWithCode(w, "Failed to initialize audio engine: "+err.Error(), http.StatusInternalServerError, "audio_init_error")
			return
		}
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

	// Initialize audio engine if needed
	if audioEngine == nil {
		if err := initAudioEngine(s.getAudioDataDir()); err != nil {
			writeErrorWithCode(w, "Failed to initialize audio engine: "+err.Error(), http.StatusInternalServerError, "audio_init_error")
			return
		}
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
	})
}

// handleAudioStatus handles GET /v1/audio/status
func (s *Server) handleAudioStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Initialize audio engine if needed
	if audioEngine == nil {
		initAudioEngine(s.getAudioDataDir())
	}

	// Get engine status
	status := audioEngine.Status()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// handleAudioDownload handles POST /v1/audio/download
func (s *Server) handleAudioDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Initialize audio engine if needed
	if audioEngine == nil {
		if err := initAudioEngine(s.getAudioDataDir()); err != nil {
			writeErrorWithCode(w, "Failed to initialize audio engine: "+err.Error(), http.StatusInternalServerError, "audio_init_error")
			return
		}
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

	// Initialize audio engine if needed
	if audioEngine == nil {
		if err := initAudioEngine(s.getAudioDataDir()); err != nil {
			writeErrorWithCode(w, "Failed to initialize audio engine: "+err.Error(), http.StatusInternalServerError, "audio_init_error")
			return
		}
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

	// Initialize audio engine if needed
	if audioEngine == nil {
		if err := initAudioEngine(s.getAudioDataDir()); err != nil {
			writeErrorWithCode(w, "Failed to initialize audio engine: "+err.Error(), http.StatusInternalServerError, "audio_init_error")
			return
		}
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
