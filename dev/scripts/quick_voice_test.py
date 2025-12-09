#!/usr/bin/env python3
"""
Quick voice test - records and transcribes using offgrid audio service
"""
import subprocess
import tempfile
import requests
import sys
import os

def main():
    duration = int(sys.argv[1]) if len(sys.argv) > 1 else 5
    server = sys.argv[2] if len(sys.argv) > 2 else "http://localhost:11612"
    
    # Record
    print(f"\n🎤 Recording for {duration} seconds... Speak now!")
    temp_file = tempfile.mktemp(suffix='.wav')
    subprocess.run(['arecord', '-f', 'cd', '-t', 'wav', '-d', str(duration), '-q', temp_file], check=True)
    print("✅ Recording complete!")
    
    # Transcribe
    print(f"\n🔄 Transcribing via {server}...")
    with open(temp_file, 'rb') as f:
        response = requests.post(
            f"{server}/v1/audio/transcriptions",
            files={'file': ('audio.wav', f, 'audio/wav')}
        )
    
    os.unlink(temp_file)
    
    if response.status_code == 200:
        result = response.json()
        print(f"\n📝 Transcription: \"{result['text']}\"")
        print(f"   Language: {result.get('language', 'en')}")
        print(f"   Duration: {result.get('duration', 0):.1f}s")
    else:
        print(f"❌ Error: {response.text}")

if __name__ == '__main__':
    main()
