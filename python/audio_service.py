#!/usr/bin/env python3
"""
Audio Service for offgrid-llm
Provides ASR (faster-whisper) and TTS (piper-tts) via HTTP API.
Run alongside the main Go server on a different port.

Usage:
    python audio_service.py [--port 11612] [--model base.en]
"""

import argparse
import io
import json
import os
import sys
import tempfile
import threading
import time
from http.server import HTTPServer, BaseHTTPRequestHandler
from urllib.parse import parse_qs, urlparse

# Model cache
_whisper_model = None
_whisper_lock = threading.Lock()
_model_name = "base.en"

def get_whisper_model():
    """Lazy-load whisper model"""
    global _whisper_model
    if _whisper_model is None:
        with _whisper_lock:
            if _whisper_model is None:
                from faster_whisper import WhisperModel
                print(f"Loading Whisper model: {_model_name}...")
                # Use CPU by default, auto-downloads model
                _whisper_model = WhisperModel(_model_name, device="cpu", compute_type="int8")
                print("Model loaded!")
    return _whisper_model

def transcribe_audio(audio_data: bytes, language: str = None) -> dict:
    """Transcribe audio bytes to text"""
    model = get_whisper_model()
    
    # Save to temp file (faster-whisper needs file path)
    with tempfile.NamedTemporaryFile(suffix='.wav', delete=False) as f:
        f.write(audio_data)
        temp_path = f.name
    
    try:
        segments, info = model.transcribe(
            temp_path,
            language=language if language else None,
            beam_size=5,
            vad_filter=True
        )
        
        # Collect all text
        text_parts = []
        segment_list = []
        for segment in segments:
            text_parts.append(segment.text.strip())
            segment_list.append({
                "id": segment.id,
                "start": segment.start,
                "end": segment.end,
                "text": segment.text.strip()
            })
        
        return {
            "text": " ".join(text_parts),
            "language": info.language,
            "duration": info.duration,
            "segments": segment_list
        }
    finally:
        os.unlink(temp_path)

class AudioServiceHandler(BaseHTTPRequestHandler):
    """HTTP handler for audio service"""
    
    def log_message(self, format, *args):
        """Custom logging"""
        print(f"[{self.log_date_time_string()}] {format % args}")
    
    def send_json(self, data: dict, status: int = 200):
        """Send JSON response"""
        body = json.dumps(data).encode('utf-8')
        self.send_response(status)
        self.send_header('Content-Type', 'application/json')
        self.send_header('Content-Length', len(body))
        self.send_header('Access-Control-Allow-Origin', '*')
        self.end_headers()
        self.wfile.write(body)
    
    def send_error_json(self, message: str, status: int = 500):
        """Send error response"""
        self.send_json({
            "error": {
                "message": message,
                "type": "api_error"
            }
        }, status)
    
    def do_OPTIONS(self):
        """Handle CORS preflight"""
        self.send_response(204)
        self.send_header('Access-Control-Allow-Origin', '*')
        self.send_header('Access-Control-Allow-Methods', 'GET, POST, OPTIONS')
        self.send_header('Access-Control-Allow-Headers', 'Content-Type')
        self.end_headers()
    
    def do_GET(self):
        """Handle GET requests"""
        path = urlparse(self.path).path
        
        if path == '/health' or path == '/':
            self.send_json({
                "status": "ok",
                "service": "offgrid-audio",
                "model": _model_name,
                "loaded": _whisper_model is not None
            })
        elif path == '/v1/audio/status':
            self.send_json({
                "asr": {
                    "available": True,
                    "model": _model_name,
                    "loaded": _whisper_model is not None
                },
                "tts": {
                    "available": False,
                    "note": "TTS coming soon"
                }
            })
        else:
            self.send_error_json("Not found", 404)
    
    def do_POST(self):
        """Handle POST requests"""
        path = urlparse(self.path).path
        
        if path == '/v1/audio/transcriptions':
            self.handle_transcription()
        else:
            self.send_error_json("Not found", 404)
    
    def handle_transcription(self):
        """Handle transcription request"""
        content_type = self.headers.get('Content-Type', '')
        
        try:
            if 'multipart/form-data' in content_type:
                # Parse multipart form
                import cgi
                form = cgi.FieldStorage(
                    fp=self.rfile,
                    headers=self.headers,
                    environ={'REQUEST_METHOD': 'POST', 'CONTENT_TYPE': content_type}
                )
                
                if 'file' not in form:
                    self.send_error_json("No audio file provided", 400)
                    return
                
                file_item = form['file']
                audio_data = file_item.file.read()
                language = form.getvalue('language', None)
                
            else:
                # Raw audio data
                content_length = int(self.headers.get('Content-Length', 0))
                audio_data = self.rfile.read(content_length)
                language = None
            
            if not audio_data:
                self.send_error_json("Empty audio data", 400)
                return
            
            # Transcribe
            result = transcribe_audio(audio_data, language)
            self.send_json(result)
            
        except Exception as e:
            self.send_error_json(f"Transcription failed: {str(e)}", 500)

def main():
    parser = argparse.ArgumentParser(description='Audio Service for offgrid-llm')
    parser.add_argument('--port', type=int, default=11612, help='Port to listen on')
    parser.add_argument('--host', default='0.0.0.0', help='Host to bind to')
    parser.add_argument('--model', default='base.en', help='Whisper model: tiny, base, small, medium, large-v3')
    parser.add_argument('--preload', action='store_true', help='Preload model on startup')
    args = parser.parse_args()
    
    global _model_name
    _model_name = args.model
    
    print(f"""
╔══════════════════════════════════════════════════════════╗
║           offgrid-llm Audio Service                      ║
╠══════════════════════════════════════════════════════════╣
║  ASR: faster-whisper ({_model_name})                            
║  TTS: coming soon                                        ║
║  Port: {args.port}                                              
╚══════════════════════════════════════════════════════════╝
""")
    
    if args.preload:
        print("Preloading model...")
        get_whisper_model()
    
    server = HTTPServer((args.host, args.port), AudioServiceHandler)
    print(f"Audio service running at http://{args.host}:{args.port}")
    print("Endpoints:")
    print(f"  GET  /health              - Health check")
    print(f"  GET  /v1/audio/status     - Service status")
    print(f"  POST /v1/audio/transcriptions - Transcribe audio")
    print()
    
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\nShutting down...")
        server.shutdown()

if __name__ == '__main__':
    main()
