// =====================================================
// SYSTEM STATUS & STREAMING METRICS
// Real-time system info, streaming speed, and feedback
// =====================================================

// =====================================================
// STREAMING METRICS TRACKING
// =====================================================
let streamingMetrics = {
    startTime: 0,
    tokenCount: 0,
    tokensPerSecond: 0,
    lastTokenTime: 0,
    intervalId: null
};

// Start tracking streaming metrics
function startStreamingMetrics() {
    streamingMetrics = {
        startTime: Date.now(),
        tokenCount: 0,
        tokensPerSecond: 0,
        lastTokenTime: Date.now(),
        intervalId: null
    };
    
    // Update metrics display every 500ms
    streamingMetrics.intervalId = setInterval(updateStreamingDisplay, 500);
    
    // Show streaming indicator
    showStreamingIndicator();
}

// Record a token received
function recordToken(tokenText) {
    if (!tokenText) return;
    
    // Estimate token count (rough: ~4 chars per token)
    const estimatedTokens = Math.max(1, Math.ceil(tokenText.length / 4));
    streamingMetrics.tokenCount += estimatedTokens;
    streamingMetrics.lastTokenTime = Date.now();
    
    // Calculate tokens per second
    const elapsedSeconds = (Date.now() - streamingMetrics.startTime) / 1000;
    if (elapsedSeconds > 0) {
        streamingMetrics.tokensPerSecond = Math.round(streamingMetrics.tokenCount / elapsedSeconds);
    }
}

// Update the streaming display
function updateStreamingDisplay() {
    const indicator = document.getElementById('streamingIndicator');
    if (!indicator) return;
    
    const elapsed = ((Date.now() - streamingMetrics.startTime) / 1000).toFixed(1);
    const tps = streamingMetrics.tokensPerSecond;
    
    indicator.innerHTML = `
        <div class="streaming-stats">
            <span class="streaming-speed">${tps} tok/s</span>
            <span class="streaming-separator">•</span>
            <span class="streaming-tokens">${streamingMetrics.tokenCount} tokens</span>
            <span class="streaming-separator">•</span>
            <span class="streaming-time">${elapsed}s</span>
        </div>
    `;
}

// Stop tracking and show final metrics
function stopStreamingMetrics() {
    if (streamingMetrics.intervalId) {
        clearInterval(streamingMetrics.intervalId);
        streamingMetrics.intervalId = null;
    }
    
    // Final update
    updateStreamingDisplay();
    
    // Hide after a delay
    setTimeout(hideStreamingIndicator, 3000);
    
    return {
        totalTokens: streamingMetrics.tokenCount,
        tokensPerSecond: streamingMetrics.tokensPerSecond,
        totalTime: (Date.now() - streamingMetrics.startTime) / 1000
    };
}

// Show streaming indicator
function showStreamingIndicator() {
    let indicator = document.getElementById('streamingIndicator');
    if (!indicator) {
        indicator = document.createElement('div');
        indicator.id = 'streamingIndicator';
        indicator.className = 'streaming-indicator';
        
        // Insert after chat header
        const chatHeader = document.querySelector('#content-chat > .border-b');
        if (chatHeader) {
            chatHeader.after(indicator);
        }
    }
    indicator.classList.remove('hidden');
    indicator.classList.add('active');
}

// Hide streaming indicator
function hideStreamingIndicator() {
    const indicator = document.getElementById('streamingIndicator');
    if (indicator) {
        indicator.classList.remove('active');
        indicator.classList.add('hidden');
    }
}

// =====================================================
// SYSTEM STATUS BAR
// =====================================================

// Fetch and display system metrics
async function updateSystemStatus() {
    try {
        const response = await fetch('/v1/stats');
        if (!response.ok) return;
        
        const data = await response.json();
        
        // Get resources data
        const resources = data.resources || {};
        
        // Update RAM display (API returns MB)
        const ramUsedMB = resources.memory_used_mb || 0;
        const ramTotalMB = resources.memory_total_mb || 0;
        const ramUsedGB = ramUsedMB / 1024;
        const ramTotalGB = ramTotalMB / 1024;
        const ramPercent = resources.memory_usage_percent || 0;
        
        const ramEl = document.getElementById('systemRamUsage');
        if (ramEl) {
            ramEl.textContent = `${ramUsedGB.toFixed(1)}/${ramTotalGB.toFixed(0)} GB`;
            ramEl.title = `RAM: ${ramPercent.toFixed(1)}% used`;
            
            // Color based on usage
            if (ramPercent > 85) {
                ramEl.className = 'status-value text-red-400';
            } else if (ramPercent > 70) {
                ramEl.className = 'status-value text-amber-400';
            } else {
                ramEl.className = 'status-value text-emerald-400';
            }
        }
        
        // Update CPU display
        const cpuPercent = resources.cpu_usage_percent || 0;
        const cpuEl = document.getElementById('systemCpuUsage');
        if (cpuEl) {
            cpuEl.textContent = `${cpuPercent.toFixed(0)}%`;
        }
        
        // Update GPU if available
        const gpuEl = document.getElementById('systemGpuUsage');
        if (gpuEl) {
            const sysInfo = data.system || {};
            if (sysInfo.gpu) {
                gpuEl.textContent = sysInfo.gpu;
                gpuEl.parentElement.classList.remove('hidden');
            } else {
                gpuEl.parentElement.classList.add('hidden');
            }
        }
        
    } catch (e) {
        // Silent fail - stats endpoint might not exist
    }
}

// Start periodic system status updates
let systemStatusInterval = null;

function startSystemStatusUpdates() {
    // Initial update
    updateSystemStatus();
    
    // Update every 5 seconds
    if (!systemStatusInterval) {
        systemStatusInterval = setInterval(updateSystemStatus, 5000);
    }
}

function stopSystemStatusUpdates() {
    if (systemStatusInterval) {
        clearInterval(systemStatusInterval);
        systemStatusInterval = null;
    }
}

// =====================================================
// BUTTON MICRO-INTERACTIONS
// =====================================================

// Add pulse animation to send button during generation
function setSendButtonGenerating(isGenerating) {
    const sendBtn = document.getElementById('sendBtn');
    const stopBtn = document.getElementById('stopBtn');
    
    if (sendBtn && stopBtn) {
        if (isGenerating) {
            sendBtn.classList.add('hidden');
            stopBtn.classList.remove('hidden');
            stopBtn.classList.add('btn-generating');
        } else {
            sendBtn.classList.remove('hidden');
            stopBtn.classList.add('hidden');
            stopBtn.classList.remove('btn-generating');
        }
    }
}

// Copy button feedback
function showCopySuccess(button) {
    const originalContent = button.innerHTML;
    button.innerHTML = `
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3">
            <polyline points="20 6 9 17 4 12"></polyline>
        </svg>
        <span>Copied!</span>
    `;
    button.classList.add('copy-success');
    
    setTimeout(() => {
        button.innerHTML = originalContent;
        button.classList.remove('copy-success');
    }, 2000);
}

// =====================================================
// IMPROVED ERROR MESSAGES
// =====================================================

// Show error with recovery options
function showErrorWithRecovery(error, options = {}) {
    const errorCode = error.code || error.status || '';
    const errorMessage = error.message || 'Something went wrong';
    
    // Determine recovery actions based on error type
    let recoveryActions = [];
    let helpText = '';
    
    if (errorCode === 'MODEL_NOT_LOADED' || errorMessage.includes('model') || errorMessage.includes('not loaded')) {
        helpText = 'The model needs to be loaded before chatting.';
        recoveryActions = [
            { label: 'Load Model', action: () => switchTab('models'), primary: true },
            { label: 'Try Again', action: options.onRetry }
        ];
    } else if (errorCode === 'CONTEXT_LENGTH' || errorMessage.includes('context') || errorMessage.includes('too long')) {
        helpText = 'The conversation is too long for this model.';
        recoveryActions = [
            { label: 'Start New Chat', action: () => newChat(), primary: true },
            { label: 'Use Larger Model', action: () => switchTab('models') }
        ];
    } else if (errorCode === 'OOM' || errorMessage.includes('memory') || errorMessage.includes('OOM')) {
        helpText = 'The system ran out of memory.';
        recoveryActions = [
            { label: 'Use Smaller Model', action: () => switchTab('models'), primary: true },
            { label: 'Try Again', action: options.onRetry }
        ];
    } else if (errorMessage.includes('503') || errorMessage.includes('busy') || errorMessage.includes('slot')) {
        helpText = 'The model is busy processing another request.';
        recoveryActions = [
            { label: 'Try Again', action: options.onRetry, primary: true },
            { label: 'Wait', action: null }
        ];
    } else if (errorMessage.includes('timeout') || errorMessage.includes('Timeout')) {
        helpText = 'The request took too long to complete.';
        recoveryActions = [
            { label: 'Try Again', action: options.onRetry, primary: true },
            { label: 'Use Faster Model', action: () => switchTab('models') }
        ];
    } else {
        helpText = 'An unexpected error occurred.';
        recoveryActions = [
            { label: 'Try Again', action: options.onRetry, primary: true },
            { label: 'Report Issue', action: () => window.open('https://github.com/takuphilchan/offgrid-llm/issues', '_blank') }
        ];
    }
    
    // Create error message element
    const errorDiv = document.createElement('div');
    errorDiv.className = 'error-message-card';
    
    const actionsHtml = recoveryActions
        .filter(a => a.action)
        .map(a => `<button class="btn ${a.primary ? 'btn-primary' : 'btn-secondary'} btn-sm" data-action="${a.label}">${a.label}</button>`)
        .join('');
    
    errorDiv.innerHTML = `
        <div class="error-header">
            <svg class="error-icon" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <circle cx="12" cy="12" r="10"></circle>
                <line x1="12" y1="8" x2="12" y2="12"></line>
                <line x1="12" y1="16" x2="12.01" y2="16"></line>
            </svg>
            <span class="error-title">Error</span>
        </div>
        <p class="error-message">${errorMessage}</p>
        <p class="error-help">${helpText}</p>
        <div class="error-actions">${actionsHtml}</div>
    `;
    
    // Add click handlers
    recoveryActions.forEach(action => {
        if (action.action) {
            const btn = errorDiv.querySelector(`[data-action="${action.label}"]`);
            if (btn) {
                btn.addEventListener('click', () => {
                    errorDiv.remove();
                    action.action();
                });
            }
        }
    });
    
    return errorDiv;
}

// =====================================================
// HARDWARE DETECTION & RECOMMENDATIONS
// =====================================================

// Detect system capabilities and recommend models
async function detectHardwareAndRecommend() {
    try {
        const response = await fetch('/v1/system/info');
        if (!response.ok) return null;
        
        const data = await response.json();
        const ramGB = data.memory_total_gb || 8;
        const hasGPU = data.gpu_available || false;
        
        let recommendation = {
            ramGB,
            hasGPU,
            tier: 'basic',
            models: [],
            message: ''
        };
        
        if (ramGB >= 32 && hasGPU) {
            recommendation.tier = 'high-end';
            recommendation.models = ['Qwen2.5-14B', 'Llama-3.1-8B', 'Mistral-7B'];
            recommendation.message = 'Your system can run large models with excellent performance.';
        } else if (ramGB >= 16) {
            recommendation.tier = 'mid-range';
            recommendation.models = ['Qwen2.5-7B', 'Llama-3.2-3B', 'Phi-3-mini'];
            recommendation.message = 'Your system can run most 7B models comfortably.';
        } else if (ramGB >= 8) {
            recommendation.tier = 'standard';
            recommendation.models = ['Phi-3-mini', 'Llama-3.2-3B', 'TinyLlama'];
            recommendation.message = 'We recommend smaller models (3B-4B) for best performance.';
        } else {
            recommendation.tier = 'basic';
            recommendation.models = ['Phi-3-mini', 'TinyLlama'];
            recommendation.message = 'For your system, we recommend lightweight models.';
        }
        
        return recommendation;
    } catch (e) {
        return null;
    }
}

// =====================================================
// INITIALIZATION
// =====================================================

document.addEventListener('DOMContentLoaded', function() {
    // Start system status updates
    startSystemStatusUpdates();
    
    // Detect hardware on first load (for onboarding)
    detectHardwareAndRecommend().then(recommendation => {
        if (recommendation) {
            window.hardwareRecommendation = recommendation;
        }
    });
});

// Cleanup on page unload
window.addEventListener('beforeunload', function() {
    stopSystemStatusUpdates();
});
