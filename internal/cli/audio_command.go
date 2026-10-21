package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/audio"
)

func audioCommand(_ context.Context, args []string, env Environment) error {
	p := Presenter{Stdout: env.Stdout, Stderr: env.Stderr, JSON: env.Options.JSON, Style: env.Style}
	// Check for help flag
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		args = []string{} // Trigger help display
	}

	if len(args) < 1 {
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %s◈ Audio%s\n", env.Style.Primary+env.Style.Bold, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "  %sSpeech-to-text and text-to-speech%s\n", env.Style.Dim, env.Style.Reset)
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sUsage%s  offgrid audio <command> [options]\n", env.Style.Dim, env.Style.Reset)
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sCommands%s\n", env.Style.Primary, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %-24s %sTranscribe audio to text%s\n", "transcribe <file>", env.Style.Dim, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %-24s %sConvert text to speech%s\n", "speak <text>", env.Style.Dim, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %-24s %sShow audio system status%s\n", "status", env.Style.Dim, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %-24s %sList audio models%s\n", "models", env.Style.Dim, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %-24s %sDownload Whisper model%s\n", "setup whisper", env.Style.Dim, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %-24s %sDownload Piper voice%s\n", "setup piper", env.Style.Dim, env.Style.Reset)
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sOptions%s\n", env.Style.Primary, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %-24s %stiny, base, small, medium, large%s\n", "--model <name>", env.Style.Dim, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %-24s %sPiper voice name%s\n", "--voice <name>", env.Style.Dim, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %-24s %sOutput file for TTS%s\n", "--output <file>", env.Style.Dim, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %-24s %sLanguage code for ASR%s\n", "--language <code>", env.Style.Dim, env.Style.Reset)
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sExamples%s\n", env.Style.Primary, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %s$%s offgrid audio transcribe recording.wav\n", env.Style.Dim, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %s$%s offgrid audio speak \"Hello!\" --output hello.wav\n", env.Style.Dim, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %s$%s offgrid audio setup whisper --model base.en\n", env.Style.Dim, env.Style.Reset)
		fmt.Fprintln(env.Stdout)
		return nil
	}

	cfg, err := localConfig(env)
	if err != nil {
		return err
	}
	if env.Audio.Stat == nil || env.Audio.ReadFile == nil || env.Audio.Create == nil || env.Audio.LongClient == nil || env.Audio.ShortClient == nil || env.Audio.GetClient == nil || env.Audio.NewEngine == nil {
		return fmt.Errorf("audio execution adapters unavailable")
	}
	baseURL := fmt.Sprintf("http://localhost:%d", cfg.ServerPort)

	subCmd := args[0]
	switch subCmd {
	case "transcribe":
		if len(args) < 2 {
			p.Error("Usage: offgrid audio transcribe <audio-file> [--model <name>] [--language <code>]")
			return nil
		}

		audioFile := args[1]
		model := "base.en"
		language := ""

		// Parse options
		for i := 2; i < len(args); i++ {
			switch args[i] {
			case "--model", "-m":
				if i+1 < len(args) {
					model = args[i+1]
					i++
				}
			case "--language", "-l":
				if i+1 < len(args) {
					language = args[i+1]
					i++
				}
			}
		}

		// Check if file exists
		if _, err := env.Audio.Stat(audioFile); os.IsNotExist(err) {
			p.Error(fmt.Sprintf("Audio file not found: %s", audioFile))
			return nil
		}

		fmt.Fprintln(env.Stdout)
		p.Section("Transcribing Audio")
		fmt.Fprintf(env.Stdout, "  %sFile:%s %s\n", env.Style.Dim, env.Style.Reset, audioFile)
		fmt.Fprintf(env.Stdout, "  %sModel:%s %s\n", env.Style.Dim, env.Style.Reset, model)
		if language != "" {
			fmt.Fprintf(env.Stdout, "  %sLanguage:%s %s\n", env.Style.Dim, env.Style.Reset, language)
		}
		fmt.Fprintln(env.Stdout)

		// Read the audio file
		audioData, err := env.Audio.ReadFile(audioFile)
		if err != nil {
			p.Error(fmt.Sprintf("Failed to read audio file: %v", err))
			return nil
		}

		// Create multipart form request
		body := &bytes.Buffer{}
		writer := NewMultipartWriter(body)

		part, err := writer.CreateFormFile("file", filepath.Base(audioFile))
		if err != nil {
			p.Error(fmt.Sprintf("Failed to create form: %v", err))
			return nil
		}
		part.Write(audioData)

		if model != "" {
			writer.WriteField("model", model)
		}
		if language != "" {
			writer.WriteField("language", language)
		}
		writer.Close()

		req, err := http.NewRequest("POST", baseURL+"/v1/audio/transcriptions", body)
		if err != nil {
			p.Error(fmt.Sprintf("Failed to create request: %v", err))
			return nil
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())

		client := env.Audio.LongClient
		resp, err := client.Do(req)
		if err != nil {
			p.Error(fmt.Sprintf("Failed to connect to server: %v", err))
			p.Info("Make sure the server is running: offgrid serve")
			return nil
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			p.Error(fmt.Sprintf("Transcription failed: %s", string(respBody)))
			return nil
		}

		var result struct {
			Text string `json:"text"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			p.Error(fmt.Sprintf("Failed to parse response: %v", err))
			return nil
		}

		p.Success("Transcription Complete")
		fmt.Fprintln(env.Stdout)
		fmt.Fprintln(env.Stdout, result.Text)
		fmt.Fprintln(env.Stdout)

	case "speak":
		if len(args) < 2 {
			p.Error("Usage: offgrid audio speak <text> [--voice <name>] [--output <file>]")
			return nil
		}

		text := args[1]
		voice := "en_US-amy-medium"
		outputFile := "output.wav"
		speed := 1.0

		// Parse options
		for i := 2; i < len(args); i++ {
			switch args[i] {
			case "--voice", "-v":
				if i+1 < len(args) {
					voice = args[i+1]
					i++
				}
			case "--output", "-o":
				if i+1 < len(args) {
					outputFile = args[i+1]
					i++
				}
			case "--speed", "-s":
				if i+1 < len(args) {
					fmt.Sscanf(args[i+1], "%f", &speed)
					i++
				}
			}
		}

		fmt.Fprintln(env.Stdout)
		p.Section("Generating Speech")
		fmt.Fprintf(env.Stdout, "  %sText:%s %s\n", env.Style.Dim, env.Style.Reset, text)
		fmt.Fprintf(env.Stdout, "  %sVoice:%s %s\n", env.Style.Dim, env.Style.Reset, voice)
		fmt.Fprintf(env.Stdout, "  %sOutput:%s %s\n", env.Style.Dim, env.Style.Reset, outputFile)
		fmt.Fprintln(env.Stdout)

		// Create request
		reqBody := map[string]interface{}{
			"input": text,
			"model": voice,
			"voice": voice,
			"speed": speed,
		}
		jsonBody, _ := json.Marshal(reqBody)

		req, err := http.NewRequest("POST", baseURL+"/v1/audio/speech", bytes.NewReader(jsonBody))
		if err != nil {
			p.Error(fmt.Sprintf("Failed to create request: %v", err))
			return nil
		}
		req.Header.Set("Content-Type", "application/json")

		client := env.Audio.ShortClient
		resp, err := client.Do(req)
		if err != nil {
			p.Error(fmt.Sprintf("Failed to connect to server: %v", err))
			p.Info("Make sure the server is running: offgrid serve")
			return nil
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			p.Error(fmt.Sprintf("Speech generation failed: %s", string(respBody)))
			return nil
		}

		// Save audio to file
		outFile, err := env.Audio.Create(outputFile)
		if err != nil {
			p.Error(fmt.Sprintf("Failed to create output file: %v", err))
			return nil
		}
		defer outFile.Close()

		written, err := io.Copy(outFile, resp.Body)
		if err != nil {
			p.Error(fmt.Sprintf("Failed to write audio: %v", err))
			return nil
		}

		p.Success(fmt.Sprintf("Speech generated: %s (%d bytes)", outputFile, written))
		fmt.Fprintln(env.Stdout)

	case "status":
		fmt.Fprintln(env.Stdout)
		p.Section("Audio System Status")

		resp, err := env.Audio.GetClient.Get(baseURL + "/v1/audio/status")
		if err != nil {
			p.Error(fmt.Sprintf("Failed to connect to server: %v", err))
			p.Info("Make sure the server is running: offgrid serve")
			return nil
		}
		defer resp.Body.Close()

		var status map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
			p.Error(fmt.Sprintf("Failed to parse response: %v", err))
			return nil
		}

		// ASR Status
		asr, _ := status["asr"].(map[string]interface{})
		asrAvailable, _ := asr["available"].(bool)
		asrPath, _ := asr["whisper_path"].(string)
		asrModels, _ := asr["models"].([]interface{})

		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sSpeech-to-Text (ASR)%s\n", env.Style.Primary+env.Style.Bold, env.Style.Reset)
		if asrAvailable {
			fmt.Fprintf(env.Stdout, "    %s%s Available%s\n", env.Style.Success, env.Style.Check, env.Style.Reset)
			fmt.Fprintf(env.Stdout, "    %sWhisper:%s %s\n", env.Style.Dim, env.Style.Reset, asrPath)
			fmt.Fprintf(env.Stdout, "    %sModels:%s %d installed\n", env.Style.Dim, env.Style.Reset, len(asrModels))
		} else {
			fmt.Fprintf(env.Stdout, "    %s%s Not Available%s\n", env.Style.Error, env.Style.Cross, env.Style.Reset)
			fmt.Fprintf(env.Stdout, "    %sRun: offgrid audio setup whisper%s\n", env.Style.Dim, env.Style.Reset)
		}

		// TTS Status
		tts, _ := status["tts"].(map[string]interface{})
		ttsAvailable, _ := tts["available"].(bool)
		ttsPath, _ := tts["piper_path"].(string)
		ttsVoices, _ := tts["voices"].(float64)

		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sText-to-Speech (TTS)%s\n", env.Style.Primary+env.Style.Bold, env.Style.Reset)
		if ttsAvailable {
			fmt.Fprintf(env.Stdout, "    %s%s Available%s\n", env.Style.Success, env.Style.Check, env.Style.Reset)
			fmt.Fprintf(env.Stdout, "    %sPiper:%s %s\n", env.Style.Dim, env.Style.Reset, ttsPath)
			fmt.Fprintf(env.Stdout, "    %sVoices:%s %d installed\n", env.Style.Dim, env.Style.Reset, int(ttsVoices))
		} else {
			fmt.Fprintf(env.Stdout, "    %s%s Not Available%s\n", env.Style.Error, env.Style.Cross, env.Style.Reset)
			fmt.Fprintf(env.Stdout, "    %sRun: offgrid audio setup piper%s\n", env.Style.Dim, env.Style.Reset)
		}

		dataDir, _ := status["data_dir"].(string)
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sData Directory:%s %s\n", env.Style.Dim, env.Style.Reset, dataDir)
		fmt.Fprintln(env.Stdout)

	case "models":
		fmt.Fprintln(env.Stdout)
		p.Section("Audio Models")

		resp, err := env.Audio.GetClient.Get(baseURL + "/v1/audio/models")
		if err != nil {
			p.Error(fmt.Sprintf("Failed to connect to server: %v", err))
			p.Info("Make sure the server is running: offgrid serve")
			return nil
		}
		defer resp.Body.Close()

		var models map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&models); err != nil {
			p.Error(fmt.Sprintf("Failed to parse response: %v", err))
			return nil
		}

		// Whisper models
		whisper, _ := models["whisper"].(map[string]interface{})
		installed, _ := whisper["installed"].([]interface{})
		available, _ := whisper["available"].([]interface{})

		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sWhisper Models (ASR)%s\n", env.Style.Primary+env.Style.Bold, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "  %sInstalled:%s\n", env.Style.Dim, env.Style.Reset)
		if len(installed) == 0 {
			fmt.Fprintf(env.Stdout, "    %sNone%s\n", env.Style.Dim, env.Style.Reset)
		} else {
			for _, m := range installed {
				fmt.Fprintf(env.Stdout, "    %s%s%s %s\n", env.Style.Success, env.Style.Check, env.Style.Reset, m)
			}
		}
		fmt.Fprintf(env.Stdout, "  %sAvailable for download:%s\n", env.Style.Dim, env.Style.Reset)
		for _, m := range available {
			if model, ok := m.(map[string]interface{}); ok {
				name, _ := model["Name"].(string)
				size, _ := model["Size"].(string)
				fmt.Fprintf(env.Stdout, "    %s%s%s (%s)\n", env.Style.Secondary, name, env.Style.Reset, size)
			}
		}

		// Piper voices
		piper, _ := models["piper"].(map[string]interface{})
		piperInstalled, _ := piper["installed"].([]interface{})
		piperAvailable, _ := piper["available"].([]interface{})

		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sPiper Voices (TTS)%s\n", env.Style.Primary+env.Style.Bold, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "  %sInstalled:%s\n", env.Style.Dim, env.Style.Reset)
		if len(piperInstalled) == 0 {
			fmt.Fprintf(env.Stdout, "    %sNone%s\n", env.Style.Dim, env.Style.Reset)
		} else {
			for _, v := range piperInstalled {
				if voice, ok := v.(map[string]interface{}); ok {
					name, _ := voice["name"].(string)
					fmt.Fprintf(env.Stdout, "    %s%s%s %s\n", env.Style.Success, env.Style.Check, env.Style.Reset, name)
				}
			}
		}
		fmt.Fprintf(env.Stdout, "  %sAvailable for download:%s\n", env.Style.Dim, env.Style.Reset)
		for _, v := range piperAvailable {
			if voice, ok := v.(map[string]interface{}); ok {
				name, _ := voice["Name"].(string)
				lang, _ := voice["Language"].(string)
				fmt.Fprintf(env.Stdout, "    %s%s%s (%s)\n", env.Style.Secondary, name, env.Style.Reset, lang)
			}
		}
		fmt.Fprintln(env.Stdout)

	case "setup":
		if len(args) < 2 {
			p.Error("Usage: offgrid audio setup <whisper|piper> [--model/--voice <name>]")
			return nil
		}

		setupType := args[1]
		name := ""

		// Parse options
		for i := 2; i < len(args); i++ {
			switch args[i] {
			case "--model", "--voice", "-m", "-v":
				if i+1 < len(args) {
					name = args[i+1]
					i++
				}
			}
		}

		switch setupType {
		case "whisper":
			if name == "" {
				name = "base.en"
			}

			// Check if model exists
			modelInfo, ok := audio.WhisperModels[name]
			if !ok {
				p.Error(fmt.Sprintf("Unknown whisper model: %s", name))
				fmt.Fprintln(env.Stdout)
				fmt.Fprintln(env.Stdout, "Available models:")
				for n, m := range audio.WhisperModels {
					fmt.Fprintf(env.Stdout, "  %s%s%s (%s)\n", env.Style.Primary, n, env.Style.Reset, m.Size)
				}
				return nil
			}

			fmt.Fprintln(env.Stdout)
			fmt.Fprintf(env.Stdout, "%sDownloading Whisper Model%s\n", env.Style.Primary+env.Style.Bold, env.Style.Reset)
			fmt.Fprintf(env.Stdout, "%s%s%s (%s)\n", env.Style.Muted, name, env.Style.Reset, modelInfo.Size)
			fmt.Fprintln(env.Stdout)

			// Create audio engine with empty config (uses defaults)
			engine, err := env.Audio.NewEngine(audio.Config{})
			if err != nil {
				p.Error(fmt.Sprintf("Failed to initialize audio engine: %v", err))
				return nil
			}

			// Download with progress (nil uses default progress printer)
			if err := engine.DownloadWhisperModel(name, nil); err != nil {
				p.Error(fmt.Sprintf("Download failed: %v", err))
				return nil
			}

			p.Success(fmt.Sprintf("Whisper model '%s' installed", name))
			fmt.Fprintln(env.Stdout)

		case "piper":
			if name == "" {
				name = "en_US-amy-medium"
			}

			// Check if voice exists
			voiceInfo, ok := audio.PiperVoices[name]
			if !ok {
				p.Error(fmt.Sprintf("Unknown piper voice: %s", name))
				fmt.Fprintln(env.Stdout)
				fmt.Fprintln(env.Stdout, "Available voices:")
				for n, v := range audio.PiperVoices {
					fmt.Fprintf(env.Stdout, "  %s%s%s (%s, %s)\n", env.Style.Primary, n, env.Style.Reset, v.Language, v.Quality)
				}
				return nil
			}

			fmt.Fprintln(env.Stdout)
			fmt.Fprintf(env.Stdout, "%sDownloading Piper Voice%s\n", env.Style.Primary+env.Style.Bold, env.Style.Reset)
			fmt.Fprintf(env.Stdout, "%s%s%s (%s, %s)\n", env.Style.Muted, name, env.Style.Reset, voiceInfo.Language, voiceInfo.Quality)
			fmt.Fprintln(env.Stdout)

			// Create audio engine with empty config (uses defaults)
			engine, err := env.Audio.NewEngine(audio.Config{})
			if err != nil {
				p.Error(fmt.Sprintf("Failed to initialize audio engine: %v", err))
				return nil
			}

			// Download piper binary if not present
			if !engine.HasPiperBinary() {
				fmt.Fprintln(env.Stdout)
				fmt.Fprintf(env.Stdout, "%sDownloading Piper binary...%s\n", env.Style.Dim, env.Style.Reset)
				if err := engine.DownloadPiperBinary(nil); err != nil {
					p.Error(fmt.Sprintf("Failed to download Piper: %v", err))
					return nil
				}
				p.Success("Piper binary installed")
			}

			// Download with progress (nil uses default progress printer)
			if err := engine.DownloadPiperVoice(name, nil); err != nil {
				p.Error(fmt.Sprintf("Download failed: %v", err))
				return nil
			}

			p.Success(fmt.Sprintf("Piper voice '%s' installed", name))
			fmt.Fprintln(env.Stdout)

		default:
			p.Error(fmt.Sprintf("Unknown setup type: %s (use 'whisper' or 'piper')", setupType))
		}

	default:
		p.Error(fmt.Sprintf("Unknown audio command: %s", subCmd))
		fmt.Fprintln(env.Stdout)
		fmt.Fprintln(env.Stdout, "Commands: transcribe, speak, status, models, setup")
	}
	return nil
}

// MultipartWriter wraps the multipart writer
type MultipartWriter struct {
	*multipartWriterImpl
}

type multipartWriterImpl struct {
	w        io.Writer
	boundary string
	parts    []byte
}

func NewMultipartWriter(w io.Writer) *MultipartWriter {
	boundary := fmt.Sprintf("----OffGridBoundary%d", time.Now().UnixNano())
	return &MultipartWriter{
		&multipartWriterImpl{
			w:        w,
			boundary: boundary,
		},
	}
}

func (m *MultipartWriter) FormDataContentType() string {
	return "multipart/form-data; boundary=" + m.boundary
}

func (m *MultipartWriter) CreateFormFile(fieldname, filename string) (io.Writer, error) {
	header := fmt.Sprintf("--%s\r\nContent-Disposition: form-data; name=\"%s\"; filename=\"%s\"\r\nContent-Type: application/octet-stream\r\n\r\n",
		m.boundary, fieldname, filename)
	m.w.Write([]byte(header))
	return m.w, nil
}

func (m *MultipartWriter) WriteField(fieldname, value string) error {
	header := fmt.Sprintf("\r\n--%s\r\nContent-Disposition: form-data; name=\"%s\"\r\n\r\n%s",
		m.boundary, fieldname, value)
	_, err := m.w.Write([]byte(header))
	return err
}

func (m *MultipartWriter) Close() error {
	_, err := m.w.Write([]byte(fmt.Sprintf("\r\n--%s--\r\n", m.boundary)))
	return err
}
func RunAudioCommand(ctx context.Context, args []string, env Environment) error {
	return audioCommand(ctx, args, env)
}
