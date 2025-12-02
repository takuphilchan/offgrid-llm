"""
Example: Model Management

Shows how to search, download, and manage models.
"""

from offgrid_llm import Client

client = Client()

# Check server
if not client.health():
    print("Error: OffGrid server is not running.")
    exit(1)

print("=== Model Management Example ===\n")

# List installed models
print("Installed Models:")
models = client.models.list()
if models:
    for m in models:
        size_gb = m.get('size', 0) / (1024**3)
        print(f"  - {m['id']} ({size_gb:.1f}GB)")
else:
    print("  No models installed")
print()

# Search for models
print("=== Searching HuggingFace ===\n")

print("Searching for 'llama' models that fit in 8GB RAM:")
results = client.models.search("llama", ram=8, limit=5)
for r in results:
    print(f"  - {r['id']}")
    print(f"    Size: {r.get('size_gb', '?')}GB")
    if r.get('best_file'):
        print(f"    File: {r['best_file']}")
    print()

# Example: Download a model (commented out to avoid long download)
print("=== Download Example ===\n")
print("To download a model, use:")
print("""
def on_progress(pct, done, total):
    print(f"\\rDownloading: {pct:.1f}% ({done/(1024**2):.1f}MB)", end="")

client.models.download(
    "bartowski/Llama-3.2-3B-Instruct-GGUF",
    "Llama-3.2-3B-Instruct-Q4_K_M.gguf",
    progress_callback=on_progress
)
print("\\nDone!")
""")

# USB Import/Export
print("=== USB Transfer ===\n")
print("Import all models from USB:")
print('  client.models.import_usb("/media/usb")')
print()
print("Export a model to USB:")
print('  client.models.export_usb("Llama-3.2-3B-Instruct-Q4_K_M", "/media/usb")')
print()

# Benchmark
print("=== Benchmarking ===\n")
if models:
    print(f"Benchmarking {models[0]['id']}...")
    print("(This would normally run benchmark tests)")
    print("""
result = client.models.benchmark(
    model_id=models[0]['id'],
    prompt_tokens=512,
    output_tokens=128,
    iterations=3
)
print(f"Speed: {result['results']['avg_generation_tokens_per_sec']:.1f} tok/s")
""")
else:
    print("No models to benchmark")
