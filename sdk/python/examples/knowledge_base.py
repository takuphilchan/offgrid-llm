"""
Example: Knowledge Base (RAG)

Shows how to use the Knowledge Base for document Q&A.
"""

from offgrid import Client

client = Client()

# Check server
if not client.health():
    print("Error: OffGrid server is not running.")
    exit(1)

print("=== Knowledge Base Example ===\n")

# Add some documents
print("Adding documents...")

# Add a document with direct content
client.kb.add("project-info", content="""
Project: OffGrid LLM
Goal: Run AI models completely offline
Features:
- OpenAI-compatible API
- Web UI and Desktop App
- Knowledge Base with RAG
- USB model transfer
Status: Active development
Next milestone: v0.3.0 with Python library
""")

client.kb.add("team-notes", content="""
Team Meeting Notes - December 2025
Attendees: Alice, Bob, Charlie

Action Items:
1. Alice: Complete Python library documentation
2. Bob: Fix macOS DMG build issue
3. Charlie: Test model switching on Windows

Next meeting: Friday at 2pm
""")

print("  Added 2 documents\n")

# List documents
print("Documents in Knowledge Base:")
for doc in client.kb.list():
    print(f"  - {doc.get('id', doc.get('name', 'unknown'))}")
print()

# Search the knowledge base
print("=== Searching Knowledge Base ===\n")

query = "What is the project goal?"
print(f"Query: {query}")
results = client.kb.search(query, top_k=2)
for r in results:
    print(f"  [{r['score']:.2f}] {r['content'][:80]}...")
print()

query = "What are the action items?"
print(f"Query: {query}")
results = client.kb.search(query, top_k=2)
for r in results:
    print(f"  [{r['score']:.2f}] {r['content'][:80]}...")
print()

# Chat with Knowledge Base context
print("=== Chat with Knowledge Base ===\n")

question = "What is the next milestone for the project?"
print(f"Question: {question}")
response = client.chat(question, use_kb=True)
print(f"Answer: {response}\n")

question = "Who needs to fix the macOS build issue?"
print(f"Question: {question}")
response = client.chat(question, use_kb=True)
print(f"Answer: {response}\n")

# Clean up
print("Cleaning up...")
client.kb.clear()
print("Done!")
