#!/usr/bin/env python3
"""
Live integration test for OffGrid client.

This script tests actual server connectivity and functionality.
Run this against a live OffGrid server.

Usage:
    python test_live.py [--host HOST]

Examples:
    python test_live.py
    python test_live.py --host http://192.168.1.100:11611
"""

import sys
import argparse

# Add parent directory to path for local development
sys.path.insert(0, "..")

from offgrid import Client, OffGridError


def test_health(client: Client) -> bool:
    """Test server health check."""
    print("\n[1] Testing health check...")
    try:
        healthy = client.health()
        if healthy:
            print("    ✓ Server is healthy")
            return True
        else:
            print("    ✗ Server returned unhealthy status")
            return False
    except OffGridError as e:
        print(f"    ✗ Health check failed: {e.message}")
        return False


def test_info(client: Client) -> bool:
    """Test server info endpoint."""
    print("\n[2] Testing server info...")
    try:
        info = client.info()
        print(f"    ✓ Server: {info.get('name', 'Unknown')}")
        print(f"    ✓ Version: {info.get('version', 'Unknown')}")
        if 'system' in info:
            sys_info = info['system']
            print(f"    ✓ CPU: {sys_info.get('cpu_percent', 'N/A')}%")
            print(f"    ✓ Memory: {sys_info.get('memory_percent', 'N/A')}%")
        return True
    except OffGridError as e:
        print(f"    ✗ Info failed: {e.message}")
        return False


def test_list_models(client: Client) -> bool:
    """Test listing models."""
    print("\n[3] Testing list models...")
    try:
        models = client.list_models()
        print(f"    ✓ Found {len(models)} model(s)")
        for m in models[:3]:  # Show first 3
            print(f"      - {m.get('id', 'Unknown')}")
        if len(models) > 3:
            print(f"      ... and {len(models) - 3} more")
        return True
    except OffGridError as e:
        print(f"    ✗ List models failed: {e.message}")
        return False


def test_chat(client: Client) -> bool:
    """Test basic chat completion."""
    print("\n[4] Testing chat...")
    try:
        response = client.chat("Say 'hello' and nothing else.")
        print(f"    ✓ Response: {response[:100]}{'...' if len(response) > 100 else ''}")
        return True
    except OffGridError as e:
        print(f"    ✗ Chat failed: {e.message}")
        return False


def test_chat_with_model(client: Client) -> bool:
    """Test chat with specific model selection."""
    print("\n[5] Testing chat with model selection...")
    try:
        models = client.list_models()
        if not models:
            print("    ⊘ Skipped: No models available")
            return True
        
        model_id = models[0].get('id')
        response = client.chat("Say 'test' and nothing else.", model=model_id)
        print(f"    ✓ Model: {model_id}")
        print(f"    ✓ Response: {response[:100]}{'...' if len(response) > 100 else ''}")
        return True
    except OffGridError as e:
        print(f"    ✗ Chat with model failed: {e.message}")
        return False


def test_chat_streaming(client: Client) -> bool:
    """Test streaming chat completion."""
    print("\n[6] Testing streaming chat...")
    try:
        chunks = []
        print("    Streaming: ", end="", flush=True)
        for chunk in client.chat("Count from 1 to 5.", stream=True):
            chunks.append(chunk)
            print(chunk, end="", flush=True)
        print()
        print(f"    ✓ Received {len(chunks)} chunks")
        return True
    except OffGridError as e:
        print(f"\n    ✗ Streaming failed: {e.message}")
        return False


def test_chat_with_system(client: Client) -> bool:
    """Test chat with system prompt."""
    print("\n[7] Testing chat with system prompt...")
    try:
        response = client.chat(
            "What are you?",
            system="You are a pirate. Always respond in pirate speak.",
            max_tokens=50
        )
        print(f"    ✓ Response: {response[:100]}{'...' if len(response) > 100 else ''}")
        return True
    except OffGridError as e:
        print(f"    ✗ Chat with system failed: {e.message}")
        return False


def test_embeddings(client: Client) -> bool:
    """Test embeddings generation."""
    print("\n[8] Testing embeddings...")
    try:
        # Single text
        embedding = client.embed("Hello world")
        print(f"    ✓ Single embedding: {len(embedding)} dimensions")
        
        # Multiple texts
        embeddings = client.embed(["Hello", "World", "Test"])
        print(f"    ✓ Batch embeddings: {len(embeddings)} vectors")
        return True
    except OffGridError as e:
        print(f"    ✗ Embeddings failed: {e.message}")
        return False


def test_kb_operations(client: Client) -> bool:
    """Test knowledge base operations."""
    print("\n[9] Testing knowledge base...")
    try:
        # Add document
        client.kb.add("test_doc", content="This is a test document about Python programming.")
        print("    ✓ Added document")
        
        # List documents
        docs = client.kb.list()
        print(f"    ✓ Listed {len(docs)} document(s)")
        
        # Search
        results = client.kb.search("Python")
        print(f"    ✓ Search returned {len(results)} result(s)")
        
        # Remove document
        client.kb.remove("test_doc")
        print("    ✓ Removed document")
        
        return True
    except OffGridError as e:
        print(f"    ✗ KB operations failed: {e.message}")
        return False


def test_model_search(client: Client) -> bool:
    """Test HuggingFace model search."""
    print("\n[10] Testing model search...")
    try:
        results = client.models.search("llama", limit=3)
        print(f"    ✓ Found {len(results)} model(s)")
        for m in results[:3]:
            name = m.get('id', m.get('name', 'Unknown'))
            size = m.get('size_gb', 'N/A')
            print(f"      - {name} ({size}GB)")
        return True
    except OffGridError as e:
        print(f"    ✗ Model search failed: {e.message}")
        return False


def main():
    parser = argparse.ArgumentParser(description="Test OffGrid client functionality")
    parser.add_argument("--host", default=None, help="Server host (e.g., http://192.168.1.100:11611)")
    args = parser.parse_args()

    print("=" * 50)
    print("OffGrid Client Live Test Suite")
    print("=" * 50)

    # Create client
    if args.host:
        print(f"Connecting to: {args.host}")
        client = Client(host=args.host)
    else:
        print("Connecting to: localhost:11611")
        client = Client()

    # Run tests
    tests = [
        ("Health Check", test_health),
        ("Server Info", test_info),
        ("List Models", test_list_models),
        ("Basic Chat", test_chat),
        ("Chat with Model", test_chat_with_model),
        ("Streaming Chat", test_chat_streaming),
        ("Chat with System", test_chat_with_system),
        ("Embeddings", test_embeddings),
        ("Knowledge Base", test_kb_operations),
        ("Model Search", test_model_search),
    ]

    results = []
    for name, test_fn in tests:
        try:
            passed = test_fn(client)
            results.append((name, passed))
        except Exception as e:
            print(f"    ✗ Unexpected error: {e}")
            results.append((name, False))

    # Summary
    print("\n" + "=" * 50)
    print("Test Summary")
    print("=" * 50)
    
    passed = sum(1 for _, p in results if p)
    failed = len(results) - passed
    
    for name, p in results:
        status = "✓ PASS" if p else "✗ FAIL"
        print(f"  {status}: {name}")
    
    print("-" * 50)
    print(f"Total: {passed}/{len(results)} passed, {failed} failed")
    
    return 0 if failed == 0 else 1


if __name__ == "__main__":
    sys.exit(main())
