#!/usr/bin/env python3
"""
OffGrid LLM - Python Client Example

This script demonstrates how to interact with the OffGrid LLM API
using Python and the requests library.
"""

import json
import requests
from typing import List, Dict, Any

BASE_URL = "http://localhost:8080"


class OffGridClient:
    """Simple client for OffGrid LLM API"""
    
    def __init__(self, base_url: str = BASE_URL):
        self.base_url = base_url.rstrip("/")
    
    def health_check(self) -> Dict[str, Any]:
        """Check if the server is healthy"""
        response = requests.get(f"{self.base_url}/health")
        response.raise_for_status()
        return response.json()
    
    def list_models(self) -> List[Dict[str, Any]]:
        """List all available models"""
        response = requests.get(f"{self.base_url}/v1/models")
        response.raise_for_status()
        return response.json()["data"]
    
    def chat_completion(
        self,
        model: str,
        messages: List[Dict[str, str]],
        temperature: float = 0.7,
        max_tokens: int = 100
    ) -> Dict[str, Any]:
        """Create a chat completion"""
        payload = {
            "model": model,
            "messages": messages,
            "temperature": temperature,
            "max_tokens": max_tokens
        }
        
        response = requests.post(
            f"{self.base_url}/v1/chat/completions",
            json=payload
        )
        response.raise_for_status()
        return response.json()
    
    def completion(
        self,
        model: str,
        prompt: str,
        temperature: float = 0.7,
        max_tokens: int = 100
    ) -> Dict[str, Any]:
        """Create a text completion"""
        payload = {
            "model": model,
            "prompt": prompt,
            "temperature": temperature,
            "max_tokens": max_tokens
        }
        
        response = requests.post(
            f"{self.base_url}/v1/completions",
            json=payload
        )
        response.raise_for_status()
        return response.json()


def main():
    """Run example interactions"""
    print("🌐 OffGrid LLM - Python Client Example")
    print("=" * 40)
    print()
    
    # Initialize client
    client = OffGridClient()
    
    # 1. Health check
    print("1️⃣  Checking server health...")
    try:
        health = client.health_check()
        print(f"✅ Server is healthy: {health}")
    except requests.exceptions.RequestException as e:
        print(f"❌ Server is not running: {e}")
        print("   Start it with: ./offgrid")
        return
    print()
    
    # 2. List models
    print("2️⃣  Listing available models...")
    models = client.list_models()
    print(f"Found {len(models)} model(s):")
    for model in models:
        print(f"  - {model['id']}")
    print()
    
    if not models:
        print("⚠️  No models available. Please add models to ~/.offgrid-llm/models/")
        print("   See docs/MODEL_SETUP.md for instructions.")
        return
    
    model_id = models[0]["id"]
    print(f"Using model: {model_id}")
    print()
    
    # 3. Chat completion
    print("3️⃣  Testing chat completion...")
    messages = [
        {"role": "system", "content": "You are a helpful assistant."},
        {"role": "user", "content": "What is OffGrid LLM?"}
    ]
    
    chat_response = client.chat_completion(
        model=model_id,
        messages=messages,
        temperature=0.7,
        max_tokens=100
    )
    
    print("Response:")
    print(json.dumps(chat_response, indent=2))
    print()
    
    # Extract and display the assistant's message
    if chat_response.get("choices"):
        assistant_msg = chat_response["choices"][0]["message"]["content"]
        print(f"Assistant: {assistant_msg}")
    print()
    
    # 4. Text completion
    print("4️⃣  Testing text completion...")
    completion_response = client.completion(
        model=model_id,
        prompt="The future of AI in offline environments is",
        temperature=0.7,
        max_tokens=50
    )
    
    print("Response:")
    print(json.dumps(completion_response, indent=2))
    print()
    
    # Extract and display the completion
    if completion_response.get("choices"):
        completion_text = completion_response["choices"][0]["text"]
        print(f"Completion: {completion_text}")
    print()
    
    print("✅ API examples completed!")
    print()
    print("Usage Statistics:")
    print(f"  Prompt tokens: {chat_response.get('usage', {}).get('prompt_tokens', 0)}")
    print(f"  Completion tokens: {chat_response.get('usage', {}).get('completion_tokens', 0)}")
    print(f"  Total tokens: {chat_response.get('usage', {}).get('total_tokens', 0)}")


if __name__ == "__main__":
    main()
