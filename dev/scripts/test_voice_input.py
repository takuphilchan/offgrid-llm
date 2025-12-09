#!/usr/bin/env python3
"""
Voice Input Test Script for offgrid-llm
Records audio from microphone and transcribes it.
"""

import subprocess
import sys
import os
import tempfile
import time

def record_audio(duration=5, output_file=None):
    """Record audio using arecord (ALSA)"""
    if output_file is None:
        output_file = tempfile.mktemp(suffix='.wav')
    
    print(f"\n🎤 Recording for {duration} seconds... Speak now!")
    print("=" * 40)
    
    try:
        # Record using arecord
        cmd = [
            'arecord',
            '-f', 'cd',        # CD quality (16-bit, 44100 Hz)
            '-t', 'wav',       # WAV format
            '-d', str(duration),  # Duration in seconds
            '-q',              # Quiet mode
            output_file
        ]
        subprocess.run(cmd, check=True)
        print("=" * 40)
        print("✅ Recording complete!")
        return output_file
    except subprocess.CalledProcessError as e:
        print(f"❌ Recording failed: {e}")
        return None
    except FileNotFoundError:
        print("❌ arecord not found. Install alsa-utils: sudo apt install alsa-utils")
        return None

def transcribe_with_google(audio_file):
    """Transcribe using Google Speech Recognition (free, online)"""
    try:
        import speech_recognition as sr
        
        recognizer = sr.Recognizer()
        with sr.AudioFile(audio_file) as source:
            audio_data = recognizer.record(source)
        
        print("\n🔄 Transcribing with Google Speech API...")
        text = recognizer.recognize_google(audio_data)
        return text
    except ImportError:
        print("❌ SpeechRecognition not installed. Run: pip install SpeechRecognition")
        return None
    except sr.UnknownValueError:
        print("❌ Could not understand audio")
        return None
    except sr.RequestError as e:
        print(f"❌ API request failed: {e}")
        return None

def transcribe_with_offgrid(audio_file, server_url="http://localhost:11611"):
    """Transcribe using offgrid-llm server"""
    try:
        import requests
        
        print(f"\n🔄 Transcribing with offgrid-llm ({server_url})...")
        
        with open(audio_file, 'rb') as f:
            files = {'file': (os.path.basename(audio_file), f, 'audio/wav')}
            response = requests.post(
                f"{server_url}/v1/audio/transcriptions",
                files=files
            )
        
        if response.status_code == 200:
            result = response.json()
            return result.get('text', str(result))
        else:
            error = response.json() if response.headers.get('content-type') == 'application/json' else response.text
            print(f"❌ Server error: {error}")
            return None
    except Exception as e:
        print(f"❌ Request failed: {e}")
        return None

def main():
    print("🎙️  Voice Input Test for offgrid-llm")
    print("=" * 50)
    
    # Get recording duration
    duration = 5
    if len(sys.argv) > 1:
        try:
            duration = int(sys.argv[1])
        except ValueError:
            pass
    
    # Record audio
    audio_file = record_audio(duration)
    if not audio_file:
        return 1
    
    print(f"\n📁 Audio saved to: {audio_file}")
    
    # Try offgrid-llm first
    text = transcribe_with_offgrid(audio_file)
    
    if text:
        print("\n" + "=" * 50)
        print("📝 Transcription (offgrid-llm):")
        print(f"   \"{text}\"")
        print("=" * 50)
    else:
        # Fallback to Google
        print("\n⚠️  offgrid-llm ASR not available, trying Google Speech API...")
        text = transcribe_with_google(audio_file)
        
        if text:
            print("\n" + "=" * 50)
            print("📝 Transcription (Google):")
            print(f"   \"{text}\"")
            print("=" * 50)
    
    # Cleanup
    if os.path.exists(audio_file):
        os.remove(audio_file)
    
    return 0

if __name__ == '__main__':
    sys.exit(main())
