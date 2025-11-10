#!/bin/bash
# JSON Output Mode Demo
# Demonstrates machine-readable output for automation

echo "=== OffGrid JSON Output Mode Demo ==="
echo ""

echo "1. List installed models (JSON)"
echo "$ offgrid list --json"
./offgrid list --json | jq '.count, .models[0].name'
echo ""

echo "2. Search HuggingFace (JSON)"
echo "$ offgrid search llama --limit 2 --json"
./offgrid search llama --limit 2 --json | jq '{count, first_result: .results[0].name}'
echo ""

echo "3. List sessions (JSON)"
echo "$ offgrid session list --json"
./offgrid session list --json | jq '{count, sessions: [.sessions[].name]}'
echo ""

echo "4. System info (JSON)"
echo "$ offgrid info --json"
./offgrid info --json | jq '{version, cpu: .system.cpu, models: .models.count}'
echo ""

echo "=== Use Cases ==="
echo ""

echo "5. Count installed models"
echo '$ offgrid list --json | jq ".count"'
./offgrid list --json | jq ".count"
echo ""

echo "6. Get model names only"
echo '$ offgrid list --json | jq -r ".models[].name"'
./offgrid list --json | jq -r ".models[].name"
echo ""

echo "7. Check if specific model exists"
echo '$ offgrid list --json | jq ".models[] | select(.name == \"Llama-3.2-3B-Instruct-Q4_K_M\") | .name"'
./offgrid list --json | jq '.models[] | select(.name == "Llama-3.2-3B-Instruct-Q4_K_M") | .name'
echo ""

echo "8. Get total model size"
echo '$ offgrid list --json | jq -r ".models[].size"'
./offgrid list --json | jq -r ".models[].size"
echo ""

echo "9. Get search results with downloads"
echo '$ offgrid search llama --limit 3 --json | jq ".results[] | {name, downloads}"'
./offgrid search llama --limit 3 --json | jq ".results[] | {name, downloads}"
echo ""

echo "10. Get most popular model from search"
echo '$ offgrid search llama --limit 5 --json | jq "[.results | sort_by(.downloads) | reverse | .[0]] | {name, downloads, likes}"'
./offgrid search llama --limit 5 --json | jq '[.results | sort_by(.downloads) | reverse | .[0]] | {name, downloads, likes}'
echo ""

echo "=== Integration Examples ==="
echo ""

echo "Example: Export to CSV"
echo '$ offgrid list --json | jq -r ".models[] | [.name, .size, .quantization] | @csv"'
./offgrid list --json | jq -r '.models[] | [.name, .size, .quantization] | @csv'
echo ""

echo "Example: Check system resources"
echo '$ offgrid info --json | jq "{cpu: .system.cpu, memory: .system.memory, available_models: .models.count}"'
./offgrid info --json | jq '{cpu: .system.cpu, memory: .system.memory, available_models: .models.count}'
echo ""

echo "Example: Session summary"
echo '$ offgrid session list --json | jq ".sessions[] | {name, model: .model_id, messages}"'
./offgrid session list --json | jq '.sessions[] | {name, model: .model_id, messages}'
echo ""

echo "=== Automation Script Example ==="
cat << 'EOF'

#!/bin/bash
# Auto-download recommended models if none installed

MODEL_COUNT=$(./offgrid list --json | jq '.count')

if [ "$MODEL_COUNT" -eq 0 ]; then
    echo "No models installed. Searching for recommended models..."
    
    # Get most downloaded llama model
    BEST_MODEL=$(./offgrid search llama --limit 1 --json | jq -r '.results[0].name')
    
    echo "Downloading: $BEST_MODEL"
    ./offgrid download-hf "$BEST_MODEL"
else
    echo "$MODEL_COUNT model(s) already installed:"
    ./offgrid list --json | jq -r '.models[].name'
fi
EOF

echo ""
echo "=== All commands supporting --json ==="
echo "  • offgrid list --json"
echo "  • offgrid search <query> --json"
echo "  • offgrid session list --json"
echo "  • offgrid info --json"
echo ""
echo "Perfect for:"
echo "  ✓ CI/CD pipelines"
echo "  ✓ Monitoring dashboards"
echo "  ✓ Custom automation scripts"
echo "  ✓ Integration with other tools"
echo ""
