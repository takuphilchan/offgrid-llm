function updateChatStats() {
    document.getElementById('messageCount').textContent = `${messages.length} messages`;
    const totalTokens = messages.reduce((sum, m) => sum + (m.content?.length || 0), 0);
    document.getElementById('tokenCount').textContent = `~${Math.ceil(totalTokens / 4)} tokens`;
}

// Update sidebar status display
function updateSidebarStatus(state, modelName = '', extra = '') {
    const statusDot = document.getElementById('statusDot');
    const statusText = document.getElementById('statusText');
    const modelBadge = document.getElementById('activeModelBadge');
    const modelNameEl = document.getElementById('activeModelName');
    
    if (!statusDot || !statusText) return;
    
    switch (state) {
        case 'ready':
            statusDot.className = 'w-2 h-2 rounded-full bg-emerald-500';
            statusText.className = 'text-xs font-medium text-emerald-400';
            statusText.textContent = 'Ready';
            if (modelName && modelBadge && modelNameEl) {
                modelBadge.classList.remove('hidden');
                modelNameEl.textContent = modelName;
                modelNameEl.title = modelName;
            }
            break;
        case 'loading':
            statusDot.className = 'w-2 h-2 rounded-full bg-orange-500 animate-pulse';
            statusText.className = 'text-xs font-medium text-orange-500';
            statusText.textContent = extra ? `Loading... ${extra}` : 'Loading...';
            if (modelName && modelBadge && modelNameEl) {
                modelBadge.classList.remove('hidden');
                modelNameEl.textContent = modelName;
                modelNameEl.title = modelName;
            }
            break;
        case 'switching':
            statusDot.className = 'w-2 h-2 rounded-full bg-orange-500 animate-pulse';
            statusText.className = 'text-xs font-medium text-orange-500';
            statusText.textContent = extra ? `Switching... ${extra}` : 'Switching...';
            if (modelName && modelBadge && modelNameEl) {
                modelBadge.classList.remove('hidden');
                modelNameEl.textContent = modelName;
                modelNameEl.title = modelName;
            }
            break;
        case 'warming':
            statusDot.className = 'w-2 h-2 rounded-full bg-orange-500 animate-pulse';
            statusText.className = 'text-xs font-medium text-orange-500';
            statusText.textContent = 'Warming...';
            if (modelName && modelBadge && modelNameEl) {
                modelBadge.classList.remove('hidden');
                modelNameEl.textContent = modelName;
                modelNameEl.title = modelName;
            }
            break;
        case 'error':
            statusDot.className = 'w-2 h-2 rounded-full bg-red-500';
            statusText.className = 'text-xs font-medium text-red-400';
            statusText.textContent = 'Error';
            break;
        case 'offline':
            statusDot.className = 'w-2 h-2 rounded-full bg-gray-500';
            statusText.className = 'text-xs font-medium text-secondary';
            statusText.textContent = 'Offline';
            if (modelBadge) modelBadge.classList.add('hidden');
            break;
    }
}

// Load models for chat
async function loadChatModels() {
    try {
        // Force refresh models from filesystem
        try {
            await fetch('/models/refresh', { method: 'POST' });
        } catch (e) {
            console.warn('Failed to refresh models, using cached list:', e);
        }
        
        const resp = await fetch('/models');
        const data = await resp.json();
        const models = data.data || [];
        
        // Load all models - both LLM and embedding models
        // We'll detect type and route to correct API when sending
        
        const select = document.getElementById('chatModel');
        
        // Remember the current selection before clearing
        const previousSelection = select.value || currentModel;
        
        select.innerHTML = '';
        
        if (models.length === 0) {
            select.innerHTML = '<option value="">No LLM models available</option>';
            return;
        }
        
        // Sort models alphabetically by ID
        models.sort((a, b) => a.id.localeCompare(b.id));
        
        models.forEach(m => {
            const opt = document.createElement('option');
            opt.value = m.id;
            // Format display name: replace underscores and hyphens with spaces for readability
            const displayName = m.id.replace(/_/g, ' ').replace(/-/g, ' ');
            opt.textContent = displayName;
            opt.title = m.id; // Show original name on hover
            select.appendChild(opt);
        });
        
        // Restore previous selection, or auto-select first model if none selected
        if (previousSelection && models.some(m => m.id === previousSelection)) {
            // Restore the previous selection
            currentModel = previousSelection;
            select.value = previousSelection;
        } else if (!currentModel && models.length > 0) {
            // First time - select first model
            currentModel = models[0].id;
            select.value = currentModel;
        }

        // Add change handler after models are loaded
        select.onchange = null; // Clear any existing handler
        select.onchange = function(e) {
            handleModelChange();
        };
        
        // Update empty state UI
        if (typeof updateEmptyState === 'function') {
            updateEmptyState();
        }
        
        // Proactive model warming: trigger current model to load in background
        // This ensures the model is ready when user starts typing
        if (currentModel) {
            warmModelInBackground(currentModel);
        }
    } catch (e) {
        console.error('Failed to load models:', e);
        document.getElementById('chatModel').innerHTML = '<option value="">Error loading models</option>';
    }
}

// Warm up a model in the background so it's ready for instant use
async function warmModelInBackground(modelName) {
    // Check if already cached - if so, skip warming
    if (await isModelCached(modelName)) {
        updateSidebarStatus('ready', modelName);
        if (typeof updateCacheIndicator === 'function') {
            updateCacheIndicator(true);
        }
        return;
    }
    
    // Show warming status
    updateSidebarStatus('warming', modelName);
    
    try {
        // Make a minimal request to trigger model loading
        const resp = await fetch('/v1/chat/completions', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                model: modelName,
                messages: [{ role: 'user', content: 'hi' }],
                stream: false,
                max_tokens: 1,
                temperature: 0
            }),
            signal: AbortSignal.timeout(30000)
        });
        
        if (resp.ok) {
            updateSidebarStatus('ready', modelName);
            if (typeof updateCacheIndicator === 'function') {
                updateCacheIndicator(true);
            }
        } else {
            updateSidebarStatus('ready', modelName);
        }
    } catch (e) {
        updateSidebarStatus('ready', modelName);
    }
}

// Load embedding models
async function loadEmbeddingModels() {
    try {
        // Force refresh models from filesystem
        try {
            await fetch('/models/refresh', { method: 'POST' });
        } catch (e) {
            console.warn('Failed to refresh models, using cached list:', e);
        }
        
        const resp = await fetch('/models');
        const data = await resp.json();
        const allModels = data.data || [];
        
        // Filter only embedding models
        const models = allModels.filter(m => {
            return m.type === 'embedding' || m.tags?.includes('embedding');
        });
        
        const select = document.getElementById('embeddingModel');
        select.innerHTML = '';
        
        if (models.length === 0) {
            select.innerHTML = '<option value="">No embedding models available</option>';
            return;
        }
        
        // Sort models alphabetically by ID
        models.sort((a, b) => a.id.localeCompare(b.id));
        
        models.forEach(m => {
            const opt = document.createElement('option');
            opt.value = m.id;
            const displayName = m.id.replace(/_/g, ' ').replace(/-/g, ' ');
            opt.textContent = displayName;
            opt.title = m.id;
            select.appendChild(opt);
        });
        
        // Auto-select first model
        if (models.length > 0) {
            select.value = models[0].id;
        }
    } catch (e) {
       
        console.error('Failed to load embedding models:', e);
        document.getElementById('embeddingModel').innerHTML = '<option value="">Error loading models</option>';
    }
}

// Generate embedding
let currentEmbeddingData = null;

async function generateEmbedding() {
    const model = document.getElementById('embeddingModel').value;
    const input = document.getElementById('embeddingInput').value.trim();
    const btn = document.getElementById('generateEmbeddingBtn');
    
    if (!model) {
        showModal({
            type: 'warning',
            title: 'Selection Required',
            message: 'Please select an embedding model',
            confirmText: 'OK'
        });
        return;
    }
    
    if (!input) {
        showModal({
            type: 'warning',
            title: 'Input Required',
            message: 'Please enter text to embed',
            confirmText: 'OK'
        });
        return;
    }
    
    btn.disabled = true;
    btn.textContent = 'Generating...';
    
    const startTime = Date.now();
    
    try {
        const response = await fetch('/v1/embeddings', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                model: model,
                input: input
            })
        });
        
        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || 'Failed to generate embedding');
        }
        
        const data = await response.json();
        const endTime = Date.now();
        const timeMs = endTime - startTime;
        
        // Store the full embedding data
        currentEmbeddingData = data.data[0].embedding;
        
        // Display results
        displayEmbeddingResults(data, timeMs);
        
    } catch (error) {
        console.error('Embedding error:', error);
        showModal({
            type: 'error',
            title: 'Embedding Failed',
            message: 'Failed to generate embedding: ' + error.message,
            confirmText: 'OK'
        });
    } finally {
        btn.disabled = false;
        btn.textContent = 'Generate Embedding';
    }
}

function displayEmbeddingResults(data, timeMs) {
    const resultsDiv = document.getElementById('embeddingResults');
    const embedding = data.data[0].embedding;
    const dimensions = embedding.length;
    
    // Show results section
    resultsDiv.classList.remove('hidden');
    
    // Update stats
    document.getElementById('embeddingDimensions').textContent = dimensions;
    document.getElementById('embeddingTokens').textContent = data.usage?.prompt_tokens || '-';
    document.getElementById('embeddingTime').textContent = timeMs + 'ms';
    
    // Display first 100 values
    const preview = embedding.slice(0, 100);
    const previewText = '[' + preview.map(v => v.toFixed(6)).join(', ') + 
                      (dimensions > 100 ? ', ...' : '') + ']';
    document.getElementById('embeddingVector').textContent = previewText;
    
    // Store full vector for later display
    const fullText = '[' + embedding.map(v => v.toFixed(6)).join(', ') + ']';
    document.getElementById('embeddingVectorFull').textContent = fullText;
    
    // Reset toggle button
    document.getElementById('toggleVectorBtn').textContent = 'Show Full Vector';
    document.getElementById('fullVectorContainer').classList.add('hidden');
}

function toggleFullVector() {
    const container = document.getElementById('fullVectorContainer');
    const btn = document.getElementById('toggleVectorBtn');
    
    if (container.classList.contains('hidden')) {
        container.classList.remove('hidden');
        btn.textContent = 'Hide Full Vector';
    } else {
        container.classList.add('hidden');
        btn.textContent = 'Show Full Vector';
    }
}

function copyEmbedding() {
    if (!currentEmbeddingData) {
        showModal({
            type: 'warning',
            title: 'No Data',
            message: 'No embedding to copy',
            confirmText: 'OK'
        });
        return;
    }
    
    const text = '[' + currentEmbeddingData.map(v => v.toFixed(6)).join(', ') + ']';
    
    navigator.clipboard.writeText(text).then(() => {
        const btn = event.target;
        const originalText = btn.textContent;
        btn.textContent = 'Copied!';
        setTimeout(() => {
            btn.textContent = originalText;
        }, 2000);
    }).catch(err => {
        console.error('Failed to copy:', err);
        showModal({
            type: 'error',
            title: 'Copy Failed',
            message: 'Failed to copy to clipboard',
            confirmText: 'OK'
        });
    });
}

function clearEmbedding() {
    document.getElementById('embeddingInput').value = '';
    document.getElementById('embeddingResults').classList.add('hidden');
    currentEmbeddingData = null;
}

// Check if a model is already cached (instant switch)
async function isModelCached(modelName) {
    try {
        const resp = await fetch('/v1/cache/stats', { signal: AbortSignal.timeout(2000) });
        if (!resp.ok) return false;
        const stats = await resp.json();
        const cachedModels = stats.model_cache?.cached_models || [];
        return cachedModels.some(m => m.model_id === modelName);
    } catch (e) {
        return false;
    }
}

// Wait for model to be fully loaded and ready (optimized version)
async function waitForModelReady(modelName) {
    // FAST PATH: Check if model is already cached (instant switch!)
    const wasCached = await isModelCached(modelName);
    
    // Use faster polling for cached models, slower for cold loads
    const pollInterval = wasCached ? 200 : 500;
    const maxWaitTime = wasCached ? 5000 : 30000; // 5s for cached, 30s for cold
    const maxAttempts = Math.ceil(maxWaitTime / pollInterval);
    
    // Check if this is an embedding model
    const resp = await fetch('/models');
    const data = await resp.json();
    const modelInfo = data.data.find(m => m.id === modelName);
    const isEmbeddingModel = modelInfo?.type === 'embedding' || 
                           modelName.toLowerCase().includes('embed') || 
                           modelName.toLowerCase().includes('bge') ||
                           modelName.toLowerCase().includes('nomic');
    
    // Trigger the model load immediately with one request
    const triggerLoad = async () => {
        try {
            if (isEmbeddingModel) {
                return await fetch('/v1/embeddings', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ model: modelName, input: 'hi' }),
                    signal: AbortSignal.timeout(15000)
                });
            } else {
                return await fetch('/v1/chat/completions', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        model: modelName,
                        messages: [{ role: 'user', content: 'hi' }],
                        stream: false,
                        max_tokens: 1,
                        temperature: 0
                    }),
                    signal: AbortSignal.timeout(15000)
                });
            }
        } catch (e) {
            return null;
        }
    };
    
    // For cached models, just make one request - it should work immediately
    if (wasCached) {
        const result = await triggerLoad();
        if (result?.ok) {
            return true;
        }
    }
    
    // Start the load in background
    triggerLoad();
    
    // Poll until ready
    for (let attempt = 1; attempt <= maxAttempts; attempt++) {
        await new Promise(resolve => setTimeout(resolve, pollInterval));
        
        try {
            // Quick cache check first (very fast)
            if (await isModelCached(modelName)) {
                // Model is in cache, try a quick inference test
                const testResp = await triggerLoad();
                if (testResp?.ok) {
                    return true;
                }
            }
        } catch (error) {
            // Continue polling
        }
    }
    
    // Final attempt with longer timeout
    const finalResult = await triggerLoad();
    if (finalResult?.ok) {
        return true;
    }
    
    return false;
}

// Handle model dropdown change
async function handleModelChange() {
    const select = document.getElementById('chatModel');
    const newModel = select.value;
    
    if (!newModel) {
        return;
    }
    
    // Don't switch if already on this model
    if (currentModel === newModel) {
        return;
    }
    
    const oldModel = currentModel;
    
    // If there are existing messages, ask user what to do
    if (messages.length > 0) {
        showModal({
            type: 'warning',
            title: 'Clear Chat History?',
            message: `Switching from <strong>${oldModel || 'no model'}</strong> to <strong>${newModel}</strong>.<br><br>Clear chat history? The new model won't have context from previous messages.`,
            confirmText: 'Start Fresh',
            cancelText: 'Keep History',
            onConfirm: () => {
                clearChatSilent();
            },
            onCancel: () => {
                // Keep history, do nothing
            }
        });
    }
    
    currentModel = newModel;
    
    // Hide cache indicator when switching models (new model needs to warm up)
    if (typeof updateCacheIndicator === 'function') {
        updateCacheIndicator(false);
    }

    // Check if VLM and toggle upload button
    try {
        const resp = await fetch('/models');
        const data = await resp.json();
        const modelInfo = data.data.find(m => m.id === newModel);
        const isVLM = modelSupportsVision(modelInfo, newModel);
        
        const imageUploadBtn = document.getElementById('imageUploadBtn');
        if (imageUploadBtn) {
            if (isVLM) {
                imageUploadBtn.classList.remove('hidden');
            } else {
                imageUploadBtn.classList.add('hidden');
                clearAllImages(); // Clear any pending image if switching away from VLM
            }
        }
    } catch (e) {
        console.error('Error checking model type:', e);
    }
    
    // Disable UI while loading model
    const chatInput = document.getElementById('chatInput');
    const sendBtn = document.getElementById('sendBtn');
    
    chatInput.disabled = true;
    sendBtn.disabled = true;
    
    // Check if model is cached for better UX messaging
    const isCached = await isModelCached(newModel);
    
    // Show loading progress with context
    let loadingMs = 0;
    const loadingStartTime = Date.now();
    const loadState = isCached ? 'switching' : 'loading';
    updateSidebarStatus(loadState, newModel);
    
    const loadingInterval = setInterval(() => {
        loadingMs = Date.now() - loadingStartTime;
        const seconds = (loadingMs / 1000).toFixed(1) + 's';
        updateSidebarStatus(loadState, newModel, seconds);
    }, 100); // Update every 100ms for smoother display
    
    // Wait for model to be ready before allowing messages
    const isReady = await waitForModelReady(newModel);
    
    // Clear loading interval
    clearInterval(loadingInterval);
    
    if (isReady) {
        updateSidebarStatus('ready', newModel);
    } else {
        console.warn('[MODEL CHANGE] Model not confirmed ready - may have issues');
        updateSidebarStatus('ready', newModel);
    }
    
    // Re-enable UI
    chatInput.disabled = false;
    sendBtn.disabled = false;
    chatInput.focus();
    
    // Update empty state to show prompts
    if (typeof updateEmptyState === 'function') {
        updateEmptyState();
    }
}

// Modal dialog system
