"""
Example: Basic Chat Usage

Shows simple chat interactions with OffGrid LLM.
"""

import offgrid

# Check if server is running
client = offgrid.Client()
if not client.health():
    print("Error: OffGrid server is not running.")
    print("Start it with: offgrid serve")
    exit(1)

# List available models
print("Available models:")
models = client.list_models()
if not models:
    print("  No models installed!")
    print("  Download one with:")
    print("    offgrid download-hf bartowski/Llama-3.2-3B-Instruct-GGUF --file Llama-3.2-3B-Instruct-Q4_K_M.gguf")
    exit(1)

for m in models:
    print(f"  - {m['id']}")

print()

# Simple chat
print("=== Simple Chat ===")
response = client.chat("What is Python in one sentence?")
print(f"Response: {response}")
print()

# Chat with system prompt
print("=== With System Prompt ===")
response = client.chat(
    "Write a haiku about coding",
    system="You are a creative poet who writes in the traditional Japanese haiku style.",
    temperature=0.9
)
print(f"Response:\n{response}")
print()

# Streaming
print("=== Streaming Response ===")
print("Response: ", end="")
for chunk in client.chat("Count from 1 to 5, one number per line", stream=True):
    print(chunk, end="", flush=True)
print("\n")

# Conversation
print("=== Multi-turn Conversation ===")
messages = [
    {"role": "system", "content": "You are a helpful math tutor."},
    {"role": "user", "content": "What is 2 + 2?"},
]

response = client.chat(messages=messages)
print(f"User: What is 2 + 2?")
print(f"Assistant: {response}")

# Continue conversation
messages.append({"role": "assistant", "content": response})
messages.append({"role": "user", "content": "And if I multiply that by 3?"})

response = client.chat(messages=messages)
print(f"User: And if I multiply that by 3?")
print(f"Assistant: {response}")
