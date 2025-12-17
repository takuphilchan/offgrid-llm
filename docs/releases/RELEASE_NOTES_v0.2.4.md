# OffGrid LLM v0.2.4 Release Notes

**Release Date:** December 9, 2025

## 🎙️ Audio: Complete Voice Experience

This release delivers a **complete offline voice assistant** with multi-language support, speech recognition, text-to-speech, and a redesigned Audio UI.

---

## Highlights

- 🌍 **18+ Languages** for speech recognition
- **Voice Assistant** with push-to-talk conversation
- **Downloadable Model Library** for Whisper & Piper
- **Redesigned Audio Tab** with sub-tabs for better organization

---

### New Features

#### Voice Assistant
- **Push-to-Talk Conversation**: Natural voice interaction with LLMs
- **Multi-Language Support**: 18+ languages including Chinese, Spanish, French, German, Japanese, Korean, Arabic, Swahili, Hindi, and more
- **Whisper Model Selection**: Choose recognition model based on quality/speed needs
- **Auto-Speak Responses**: Toggle automatic TTS for assistant replies
- **Language-Aware History**: Automatically clears conversation when switching languages

#### Speech-to-Text (ASR)
- **Whisper.cpp Integration**: Transcribe audio files completely offline
- **Multiple Model Sizes**: Choose from tiny (75MB) to large (2.9GB) based on your quality/speed needs
- **OpenAI-Compatible API**: `POST /v1/audio/transcriptions`
- **CLI Support**: `offgrid audio transcribe <file>`

#### Text-to-Speech (TTS)
- **Piper Integration**: Generate natural-sounding speech offline
- **Multiple Voices**: English, German, French, and more from Hugging Face
- **Adjustable Speed**: Control speech rate from 0.5x to 2.0x
- **OpenAI-Compatible API**: `POST /v1/audio/speech`
- **CLI Support**: `offgrid audio speak "Hello world"`

### CLI Commands

```bash
# Setup
offgrid audio setup whisper base    # Download Whisper model
offgrid audio setup piper en_US-amy-medium  # Download voice

# List available
offgrid audio models    # List Whisper models
offgrid audio voices    # List installed voices

# Transcribe audio
offgrid audio transcribe recording.wav
offgrid audio transcribe meeting.mp3 --model small --output text

# Generate speech
offgrid audio speak "Hello, world!" --voice en_US-amy-medium
offgrid audio speak "Welcome!" --output greeting.wav
```

### API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/audio/transcriptions` | POST | Transcribe audio to text |
| `/v1/audio/speech` | POST | Generate speech from text |
| `/v1/audio/models` | GET | List Whisper models |
| `/v1/audio/whisper-models` | GET | List all available Whisper models (new) |
| `/v1/audio/voices` | GET | List Piper voices |
| `/v1/audio/status` | GET | Audio engine status |
| `/v1/audio/setup/whisper` | POST | Download Whisper model |
| `/v1/audio/setup/piper` | POST | Download Piper voice |

### Python Client

```python
from offgrid import Client

client = Client()

# Setup (first time)
client.audio.setup_whisper("base")
client.audio.setup_piper("en_US-amy-medium")

# Transcribe audio
text = client.audio.transcribe("recording.wav")
print(text)

# Generate speech
audio_data = client.audio.speak("Hello world!")
with open("output.wav", "wb") as f:
    f.write(audio_data)

# List available
print(client.audio.voices())
print(client.audio.models())
```

### Web UI

- **Redesigned Audio Tab**: Split into 3 sub-tabs:
  - **Voice Assistant**: Push-to-talk conversation with LLM
  - **STT/TTS Tools**: Standalone transcription and speech generation
  - **Models & Voices**: Download and manage Whisper models and TTS voices
- **Voice Settings in Chat**: Added voice input/output settings to Chat Settings panel
- **Model Library**: Browse and download Whisper models with size/type info
- **Voice Library**: 47 voices across 37 languages with search and filtering
- **Collapsible Language Groups**: Organized voice browsing
- **Record Audio**: Use browser microphone for transcription
- **Upload Files**: Drag & drop audio files
- **Generate Speech**: Type text and generate audio with playback

### Available Whisper Models

| Model | Size | RAM | Speed | Quality |
|-------|------|-----|-------|---------|
| tiny | 75MB | ~1GB | Fastest | Basic |
| base | 142MB | ~1GB | Fast | Good |
| small | 466MB | ~2GB | Medium | Better |
| medium | 1.5GB | ~5GB | Slower | Great |
| large | 2.9GB | ~10GB | Slowest | Best |

### Popular Piper Voices

- `en_US-amy-medium` - American English, female
- `en_US-ryan-medium` - American English, male
- `en_GB-alba-medium` - British English, female
- `de_DE-thorsten-medium` - German, male
- `fr_FR-siwis-medium` - French, female

## Bug Fixes

- **Fixed voice selection**: Backend now correctly uses `req.Voice` parameter for TTS
- **Fixed English-only STT**: Voice input now supports multiple languages with language dropdown
- **Fixed language caching**: Conversation history clears when switching languages

## Dependencies

Audio features require:
- **Whisper.cpp**: Automatically downloaded with `offgrid audio setup whisper`
- **Piper**: Automatically downloaded with `offgrid audio setup piper`

## Upgrade Notes

- This is a feature addition release; no breaking changes
- Audio features are optional; existing functionality unchanged
- Python client remains at v0.1.4

## Full Changelog

### Audio System
- Added Voice Assistant with push-to-talk conversation
- Added multi-language speech recognition (18+ languages)
- Added Whisper model selection in Voice Assistant
- Added `/v1/audio/whisper-models` API endpoint
- Added downloadable Whisper model library
- Added language-aware conversation clearing
- Fixed voice parameter not being used in TTS
- Fixed STT only expecting English input

### UI/UX
- Redesigned Audio tab with sub-tabs (Voice Assistant, Tools, Library)
- Added Voice Settings to Chat Settings panel
- Added Whisper Model Library with download capability
- Improved Voice Library with collapsible language groups
- Added model size and type indicators
- Compact design reducing scrolling

### Backend
- Added `internal/audio` package for ASR/TTS
- Added audio CLI commands
- Added audio API endpoints
- Updated `Speak()` to use voice parameter correctly
- Added `handleAudioWhisperModels()` handler
