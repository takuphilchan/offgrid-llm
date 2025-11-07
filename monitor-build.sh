#!/bin/bash
# Monitor llama.cpp build progress

echo "🔍 Build Monitor"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Check if build is running
if ! pgrep -f "make.*llama-server" > /dev/null; then
    echo "❌ No build process found"
    echo ""
    echo "Checking if build completed..."
    
    if [ -f "/usr/local/bin/llama-server" ]; then
        echo "✅ llama-server binary exists!"
        echo ""
        echo "Binary info:"
        file /usr/local/bin/llama-server
        echo ""
        echo "Size: $(du -h /usr/local/bin/llama-server | cut -f1)"
        echo ""
        echo "Dependencies:"
        ldd /usr/local/bin/llama-server 2>&1 | head -10
    elif [ -f "/root/llama.cpp/build/bin/llama-server" ]; then
        echo "⚠️  Binary built but not installed"
        echo "Run: sudo install -o0 -g0 -m755 /root/llama.cpp/build/bin/llama-server /usr/local/bin/llama-server"
    else
        echo "❌ Binary not found"
        echo ""
        echo "Last build log entries:"
        tail -20 /tmp/llama-build.log 2>/dev/null || echo "No build log found"
    fi
    exit 0
fi

echo "✅ Build is running"
echo ""

# Show progress
echo "Progress:"
tail -3 /tmp/llama-build.log 2>/dev/null | grep -E "\[[0-9]+%\]" | tail -1

echo ""
echo "Active processes:"
ps aux | grep -E "(make|nvcc|cicc|gcc.*ggml)" | grep -v grep | wc -l | xargs echo "  Compiler processes:"

echo ""
echo "Build has been running for:"
ps -eo pid,etime,cmd | grep "make.*llama-server" | grep -v grep | awk '{print "  " $2}'

echo ""
echo "Recent activity:"
tail -5 /tmp/llama-build.log 2>/dev/null

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Press Ctrl+C to exit, or run: tail -f /tmp/llama-build.log"
