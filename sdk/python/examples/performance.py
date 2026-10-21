"""
Example: Performance Optimization

Shows how to use the new performance features in the OffGrid Python client.
"""

import time
import offgrid

# Connect to server
client = offgrid.Client()

if not client.health():
    print("Error: OffGrid server is not running.")
    print("Start it with: offgrid serve")
    exit(1)

print("=== OffGrid Performance Features ===\n")

# List available models
models = client.list_models()
if not models:
    print("No models installed!")
    exit(1)

model = models[0]["id"]
print(f"Using model: {model}\n")

# Check cache stats
print("=== Cache Statistics ===")
try:
    stats = client.cache_stats()
    print(f"Models in cache: {stats.get('cache_size', 0)}/{stats.get('max_size', 0)}")
    print(f"System RAM: {stats.get('system_ram_mb', 0)} MB")
    print(f"Mlock enabled: {stats.get('mlock_enabled', False)}")
    
    if stats.get("mmap_warmer"):
        warmer = stats["mmap_warmer"]
        print(f"Pre-warmed: {warmer.get('total_warmed', 0)} models ({warmer.get('total_gb', 0):.1f} GB)")
    
    print("\nLoaded models:")
    for m in stats.get("loaded_models", []):
        print(f"  - {m.get('id', 'unknown')}: {m.get('size_mb', 0):.0f} MB")
except Exception as e:
    print(f"Cache stats not available: {e}")

print()

# Check if model is cached
print("=== Model Cache Check ===")
is_cached = client.is_model_cached(model)
print(f"Model '{model}' cached: {is_cached}")

if not is_cached:
    print("\nWarming model (this ensures fast first response)...")
    start = time.time()
    success = client.warm_model(model)
    elapsed = time.time() - start
    print(f"Warm completed: {success} ({elapsed:.2f}s)")
else:
    print("Model already cached - instant responses available!")

print()

# Benchmark cached vs non-cached response times
print("=== Response Time Benchmark ===")

# First request (should be fast if cached)
start = time.time()
response = client.chat("Say 'Hello' and nothing else", model=model, max_tokens=10)
first_time = time.time() - start
print(f"First response: {first_time:.3f}s")
print(f"  Response: {response.strip()}")

# Second request (should benefit from KV cache)
start = time.time()
response = client.chat("Say 'World' and nothing else", model=model, max_tokens=10)
second_time = time.time() - start
print(f"Second response: {second_time:.3f}s")
print(f"  Response: {response.strip()}")

# Third request - similar prefix (benefits from cache-reuse)
start = time.time()
response = client.chat("Say 'Goodbye' and nothing else", model=model, max_tokens=10)
third_time = time.time() - start
print(f"Third response: {third_time:.3f}s")
print(f"  Response: {response.strip()}")

print()
print("=== Summary ===")
print(f"Average response time: {(first_time + second_time + third_time) / 3:.3f}s")
print(f"Speedup (1st → 3rd): {first_time / third_time:.1f}x")

# Connection keep-alive info
print(f"\nConnection keep-alive: {client.keep_alive}")
print("Keep-alive reduces connection overhead for multiple requests.")
