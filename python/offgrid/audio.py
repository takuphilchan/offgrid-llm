"""
OffGrid Audio - Speech-to-Text (ASR) and Text-to-Speech (TTS)

Transcribe audio files and generate speech, all running locally.

Example:
    >>> from offgrid import Client
    >>> client = Client()
    >>> 
    >>> # Transcribe audio file
    >>> text = client.audio.transcribe("recording.wav")
    >>> print(text)
    >>> 
    >>> # Generate speech
    >>> client.audio.speak("Hello, world!", output_file="hello.wav")
    >>> 
    >>> # Check audio status
    >>> status = client.audio.status()
"""

import os
from typing import Dict, List, Optional, Union, TYPE_CHECKING

if TYPE_CHECKING:
    from .client import Client


class Audio:
    """
    Audio manager for speech-to-text (ASR) and text-to-speech (TTS).
    
    Uses Whisper.cpp for ASR and Piper for TTS, both running locally.
    
    Example:
        >>> audio = client.audio
        >>> text = audio.transcribe("recording.wav")
        >>> audio.speak("Hello!", output_file="hello.wav")
    """
    
    def __init__(self, client: "Client"):
        self._client = client
    
    def transcribe(
        self,
        file_path: str,
        model: str = None,
        language: str = None,
        prompt: str = None,
        response_format: str = "json"
    ) -> Union[str, Dict]:
        """
        Transcribe an audio file to text (Speech-to-Text).
        
        Args:
            file_path: Path to the audio file (wav, mp3, etc.)
            model: Whisper model to use (tiny, base, small, medium, large)
            language: Language code (en, es, fr, etc.) - auto-detected if not specified
            prompt: Optional prompt to guide transcription
            response_format: "json" (default), "text", or "verbose_json"
        
        Returns:
            Transcribed text (str) if response_format="text",
            otherwise dict with "text" key
        
        Example:
            >>> text = audio.transcribe("recording.wav")
            >>> print(text["text"])
            
            >>> text = audio.transcribe("recording.wav", language="es")
        """
        if not os.path.exists(file_path):
            raise FileNotFoundError(f"Audio file not found: {file_path}")
        
        # Read file
        with open(file_path, "rb") as f:
            audio_data = f.read()
        
        # Build multipart form data
        import mimetypes
        content_type = mimetypes.guess_type(file_path)[0] or "audio/wav"
        filename = os.path.basename(file_path)
        
        # Create multipart body manually (no external dependencies)
        boundary = "----OffGridPythonBoundary"
        body = []
        
        # File part
        body.append(f"--{boundary}".encode())
        body.append(f'Content-Disposition: form-data; name="file"; filename="{filename}"'.encode())
        body.append(f"Content-Type: {content_type}".encode())
        body.append(b"")
        body.append(audio_data)
        
        # Optional fields
        if model:
            body.append(f"--{boundary}".encode())
            body.append(b'Content-Disposition: form-data; name="model"')
            body.append(b"")
            body.append(model.encode())
        
        if language:
            body.append(f"--{boundary}".encode())
            body.append(b'Content-Disposition: form-data; name="language"')
            body.append(b"")
            body.append(language.encode())
        
        if prompt:
            body.append(f"--{boundary}".encode())
            body.append(b'Content-Disposition: form-data; name="prompt"')
            body.append(b"")
            body.append(prompt.encode())
        
        body.append(f"--{boundary}".encode())
        body.append(b'Content-Disposition: form-data; name="response_format"')
        body.append(b"")
        body.append(response_format.encode())
        
        body.append(f"--{boundary}--".encode())
        
        # Join with CRLF
        body_bytes = b"\r\n".join(body)
        
        # Make request
        from urllib.request import Request, urlopen
        from urllib.error import HTTPError
        import json
        
        url = f"{self._client.base_url}/v1/audio/transcriptions"
        headers = {
            "Content-Type": f"multipart/form-data; boundary={boundary}"
        }
        if self._client.api_key:
            headers["Authorization"] = f"Bearer {self._client.api_key}"
        
        req = Request(url, data=body_bytes, headers=headers, method="POST")
        
        try:
            response = urlopen(req, timeout=self._client.timeout)
            content = response.read().decode()
            
            if response_format == "text":
                return content
            return json.loads(content)
        except HTTPError as e:
            error_body = e.read().decode()
            from .client import OffGridError
            raise OffGridError(f"Transcription failed: {error_body}")
    
    def speak(
        self,
        text: str,
        output_file: str = None,
        voice: str = None,
        model: str = None,
        speed: float = 1.0,
        response_format: str = "wav"
    ) -> Optional[bytes]:
        """
        Convert text to speech (Text-to-Speech).
        
        Args:
            text: Text to convert to speech
            output_file: Path to save audio file (optional)
            voice: Voice name (e.g., "en_US-amy-medium")
            model: Same as voice (for OpenAI compatibility)
            speed: Speed multiplier (0.25 to 4.0, default 1.0)
            response_format: Audio format: "wav", "mp3", "opus", "flac"
        
        Returns:
            Audio bytes if output_file is None, otherwise None
        
        Example:
            >>> audio.speak("Hello, world!", output_file="hello.wav")
            
            >>> audio_bytes = audio.speak("Hello!")
            >>> with open("output.wav", "wb") as f:
            ...     f.write(audio_bytes)
        """
        payload = {
            "input": text,
            "model": model or voice or "en_US-amy-medium",
            "voice": voice or model or "en_US-amy-medium",
            "speed": speed,
            "response_format": response_format
        }
        
        from urllib.request import Request, urlopen
        from urllib.error import HTTPError
        import json
        
        url = f"{self._client.base_url}/v1/audio/speech"
        headers = {"Content-Type": "application/json"}
        if self._client.api_key:
            headers["Authorization"] = f"Bearer {self._client.api_key}"
        
        body = json.dumps(payload).encode()
        req = Request(url, data=body, headers=headers, method="POST")
        
        try:
            response = urlopen(req, timeout=self._client.timeout)
            audio_data = response.read()
            
            if output_file:
                with open(output_file, "wb") as f:
                    f.write(audio_data)
                return None
            
            return audio_data
        except HTTPError as e:
            error_body = e.read().decode()
            from .client import OffGridError
            raise OffGridError(f"Speech generation failed: {error_body}")
    
    def voices(self) -> List[Dict]:
        """
        List available TTS voices.
        
        Returns:
            List of voice dictionaries with name, language, quality
        
        Example:
            >>> voices = audio.voices()
            >>> for v in voices:
            ...     print(f"{v['name']}: {v['language']}")
        """
        response = self._client._request("GET", "/v1/audio/voices")
        return response.get("voices", [])
    
    def whisper_models(self) -> List[Dict]:
        """
        List available Whisper models for speech-to-text.
        
        Returns:
            List of model dictionaries with name, size, language, installed status
        
        Example:
            >>> models = audio.whisper_models()
            >>> for m in models:
            ...     status = "✓" if m['installed'] else "↓"
            ...     print(f"{status} {m['name']}: {m['size']} ({m['language']})")
        """
        response = self._client._request("GET", "/v1/audio/whisper-models")
        return response.get("models", [])
    
    def models(self) -> Dict:
        """
        List available audio models (Whisper for ASR, Piper for TTS).
        
        Returns:
            Dict with "whisper" and "piper" keys, each containing
            "installed" and "available" lists
        
        Example:
            >>> models = audio.models()
            >>> print("Installed whisper:", models["whisper"]["installed"])
            >>> print("Available voices:", models["piper"]["available"])
        """
        return self._client._request("GET", "/v1/audio/models")
    
    def status(self) -> Dict:
        """
        Get audio system status.
        
        Returns:
            Dict with ASR and TTS availability status
        
        Example:
            >>> status = audio.status()
            >>> print(f"ASR available: {status['asr']['available']}")
            >>> print(f"TTS available: {status['tts']['available']}")
        """
        return self._client._request("GET", "/v1/audio/status")
    
    def download(self, type: str, name: str) -> Dict:
        """
        Download a whisper model or piper voice.
        
        Args:
            type: "whisper" or "piper"
            name: Model/voice name (e.g., "base.en" or "en_US-amy-medium")
        
        Returns:
            Download result
        
        Example:
            >>> audio.download("whisper", "base.en")
            >>> audio.download("piper", "en_US-amy-medium")
        """
        payload = {"type": type, "name": name}
        return self._client._request("POST", "/v1/audio/download", payload)
    
    def setup_whisper(self, model: str = "base.en") -> Dict:
        """
        Download and set up a Whisper model for speech-to-text.
        
        Args:
            model: Model name (tiny, base, small, medium, large)
                   Add .en suffix for English-only (faster)
        
        Returns:
            Setup result
        
        Example:
            >>> audio.setup_whisper("base.en")  # Fast, English only
            >>> audio.setup_whisper("small")    # Multilingual
        """
        return self.download("whisper", model)
    
    def setup_piper(self, voice: str = "en_US-amy-medium") -> Dict:
        """
        Download and set up a Piper voice for text-to-speech.
        
        Args:
            voice: Voice name (e.g., "en_US-amy-medium", "en_GB-alan-medium")
        
        Returns:
            Setup result
        
        Example:
            >>> audio.setup_piper("en_US-amy-medium")
            >>> audio.setup_piper("en_GB-alan-medium")
        """
        return self.download("piper", voice)
