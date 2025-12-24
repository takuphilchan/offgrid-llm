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
    
    // Also update system status bar model name
    const statusModelName = document.getElementById('statusModelName');
    if (statusModelName && modelName) {
        // Extract short model name (last part after /)
        const shortName = modelName.split('/').pop().split(':')[0];
        statusModelName.textContent = shortName || modelName;
    }
    
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

// Track current model switch to cancel previous ones
let currentSwitchController = null;
let currentSwitchModelId = null;

// Unified model switch function - use this from any page that has a model selector
// This ensures the model is loaded and the footer/status is updated consistently
async function switchToModel(modelId, sourceSelectId = null) {
    if (!modelId) return;
    
    // Don't switch if already on this model
    if (typeof currentModel !== 'undefined' && currentModel === modelId) {
        console.log('[MODEL] Already on model:', modelId, '- skipping switch');
        // Just sync the dropdowns and ensure footer shows ready
        syncModelSelects(modelId, sourceSelectId);
        updateSidebarStatus('ready', modelId);
        return true;
    }
    
    // Cancel any previous pending switch
    if (currentSwitchController) {
        console.log('[MODEL] Cancelling previous switch to:', currentSwitchModelId);
        currentSwitchController.abort();
    }
    
    // Create new abort controller for this switch
    currentSwitchController = new AbortController();
    currentSwitchModelId = modelId;
    const thisController = currentSwitchController;
    
    console.log('[MODEL] switchToModel called:', modelId, 'from:', sourceSelectId);
    
    // Update footer status immediately to show loading
    updateSidebarStatus('loading', modelId);
    
    // Sync selection across all model dropdowns
    syncModelSelects(modelId, sourceSelectId);
    
    // Check if this switch was cancelled
    if (thisController.signal.aborted) return false;
    
    // Check if model is already loaded (cached)
    const isCached = await isModelCached(modelId);
    
    // Check if this switch was cancelled
    if (thisController.signal.aborted) return false;
    
    const loadState = isCached ? 'switching' : 'loading';
    
    let loadingMs = 0;
    const loadingStartTime = Date.now();
    updateSidebarStatus(loadState, modelId);
    
    const loadingInterval = setInterval(() => {
        // Stop updating if this switch was cancelled
        if (thisController.signal.aborted) {
            clearInterval(loadingInterval);
            return;
        }
        loadingMs = Date.now() - loadingStartTime;
        const seconds = (loadingMs / 1000).toFixed(1) + 's';
        updateSidebarStatus(loadState, modelId, seconds);
    }, 100);
    
    // Wait for model to be ready
    const isReady = await waitForModelReady(modelId);
    
    clearInterval(loadingInterval);
    
    // Check if this switch was cancelled (another switch took over)
    if (thisController.signal.aborted) {
        console.log('[MODEL] Switch cancelled, ignoring result for:', modelId);
        return false;
    }
    
    // Clear the controller since we're done
    if (currentSwitchController === thisController) {
        currentSwitchController = null;
        currentSwitchModelId = null;
    }
    
    if (isReady) {
        updateSidebarStatus('ready', modelId);
        // Update global currentModel if it exists
        if (typeof currentModel !== 'undefined') {
            currentModel = modelId;
        }
    } else {
        updateSidebarStatus('ready', modelId); // Still show as ready, may have issues
    }
    
    return isReady;
}

// Sync model selection across all model dropdowns (keeps UI consistent)
function syncModelSelects(modelId, sourceSelectId = null) {
    const modelSelectIds = ['chatModel', 'agentModel', 'benchmarkModel', 'loraBaseModel', 'voiceAssistantModel'];
    
    for (const selectId of modelSelectIds) {
        if (selectId === sourceSelectId) continue; // Skip the source
        
        const select = document.getElementById(selectId);
        if (select) {
            // Check if option exists
            const option = Array.from(select.options).find(opt => opt.value === modelId);
            if (option) {
                select.value = modelId;
            }
        }
    }
}

// Handle model change from Agent page
async function handleAgentModelChange() {
    const select = document.getElementById('agentModel');
    console.log('[MODEL] Agent model change triggered:', select?.value);
    if (select && select.value) {
        await switchToModel(select.value, 'agentModel');
    }
}

// Handle model change from Benchmark page
async function handleBenchmarkModelChange() {
    const select = document.getElementById('benchmarkModel');
    console.log('[MODEL] Benchmark model change triggered:', select?.value);
    if (select && select.value) {
        await switchToModel(select.value, 'benchmarkModel');
    }
}

// Handle model change from LoRA page
async function handleLoraModelChange() {
    const select = document.getElementById('loraBaseModel');
    console.log('[MODEL] LoRA model change triggered:', select?.value);
    if (select && select.value) {
        await switchToModel(select.value, 'loraBaseModel');
    }
}

// Cache for models to avoid redundant fetches
let cachedModels = null;
let lastModelsRefresh = 0;
const MODELS_CACHE_TTL = 30000; // 30 second cache

// Refresh models from server (called once, results cached)
async function refreshModelsCache(force = false) {
    const now = Date.now();
    
    // Return cache if valid and not empty
    if (!force && cachedModels && cachedModels.length > 0 && (now - lastModelsRefresh) < MODELS_CACHE_TTL) {
        return cachedModels;
    }
    
    try {
        await fetch('/models/refresh', { method: 'POST', signal: AbortSignal.timeout(5000) });
    } catch (e) {
        console.warn('Failed to refresh models, using cached list:', e);
    }
    
    try {
        const resp = await fetch('/models', { signal: AbortSignal.timeout(10000) });
        const data = await resp.json();
        if (data.data && data.data.length > 0) {
            cachedModels = data.data;
            lastModelsRefresh = now;
        }
    } catch (e) {
        console.error('Failed to fetch models:', e);
    }
    
    return cachedModels || [];
}

// Load models for chat
async function loadChatModels() {
    try {
        const models = await refreshModelsCache();
        
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
            // Format display name with size info if available
            const displayName = m.id.replace(/_/g, ' ').replace(/-/g, ' ');
            // Add size info if available from model metadata
            const sizeInfo = m.size ? ` (${formatFileSize(m.size)})` : '';
            opt.textContent = displayName + sizeInfo;
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
    // Skip if no model specified
    if (!modelName) {
        console.log('[MODEL] warmModelInBackground: No model specified, skipping');
        return;
    }
    
    // Skip embedding models - they use a different API and don't need warming
    const lowerName = modelName.toLowerCase();
    if (lowerName.includes('embed') || lowerName.includes('bge') || lowerName.includes('minilm') || lowerName.includes('nomic')) {
        console.log('[MODEL] warmModelInBackground: Skipping embedding model:', modelName);
        return;
    }
    
    // Check if already cached - if so, skip warming
    const cached = await isModelCached(modelName);
    console.log('[MODEL] warmModelInBackground:', modelName, 'cached:', cached);
    
    if (cached) {
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
            signal: AbortSignal.timeout(300000) // 5 minutes for low-end machines
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
        // Use cached models - no need to refresh again
        const allModels = await refreshModelsCache();
        
        // Filter only embedding models
        const models = allModels.filter(m => {
            return m.type === 'embedding' || m.tags?.includes('embedding');
        });
        
        const select = document.getElementById('embeddingModel');
        if (!select) {
            // Element doesn't exist on this page, skip
            return;
        }
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
        const select = document.getElementById('embeddingModel');
        if (select) {
            select.innerHTML = '<option value="">Error loading models</option>';
        }
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
    if (!modelName) return false;
    
    try {
        const resp = await fetch('/v1/cache/stats', { signal: AbortSignal.timeout(2000) });
        if (!resp.ok) {
            console.log('[MODEL] isModelCached: cache/stats returned', resp.status);
            return false;
        }
        const stats = await resp.json();
        
        // Check both model_cache and loaded_models
        const cachedModels = stats.model_cache?.cached_models || [];
        const loadedModels = stats.loaded_models || [];
        
        const inCache = cachedModels.some(m => m.model_id === modelName || m.id === modelName);
        const isLoaded = loadedModels.some(m => m === modelName || m.model_id === modelName);
        
        console.log('[MODEL] isModelCached:', modelName, 'inCache:', inCache, 'isLoaded:', isLoaded);
        return inCache || isLoaded;
    } catch (e) {
        console.log('[MODEL] isModelCached error:', e.message);
        return false;
    }
}

// Wait for model to be fully loaded and ready (simplified version)
async function waitForModelReady(modelName) {
    // In single-instance mode, we need to trigger a request to load the model
    // The server will unload the current model and load the new one
    
    const pollInterval = 1000; // Poll every 1s
    const maxWaitTime = 120000; // 120s max wait for cold loads
    const maxAttempts = Math.ceil(maxWaitTime / pollInterval);
    
    // Check if this is an embedding model
    let isEmbeddingModel = false;
    try {
        const resp = await fetch('/models');
        const data = await resp.json();
        const modelInfo = data.data.find(m => m.id === modelName);
        isEmbeddingModel = modelInfo?.type === 'embedding' || 
                           modelName.toLowerCase().includes('embed') || 
                           modelName.toLowerCase().includes('bge') ||
                           modelName.toLowerCase().includes('nomic');
    } catch (e) {
        console.warn('[MODEL] Could not determine model type:', e);
    }
    
    // Send a single request to trigger the model load
    // This kicks off the loading process on the server
    const loadController = new AbortController();
    const loadTimeout = setTimeout(() => loadController.abort(), maxWaitTime);
    
    const triggerLoad = async () => {
        try {
            if (isEmbeddingModel) {
                const resp = await fetch('/v1/embeddings', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ model: modelName, input: 'test' }),
                    signal: loadController.signal
                });
                return resp.ok;
            } else {
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
                    signal: loadController.signal
                });
                return resp.ok;
            }
        } catch (e) {
            if (e.name === 'AbortError') {
                console.warn('[MODEL] Load request timed out');
            }
            return false;
        } finally {
            clearTimeout(loadTimeout);
        }
    };
    
    // The request will block until the model is loaded
    // This is the simplest and most reliable approach
    console.log('[MODEL] Triggering model load for:', modelName);
    const success = await triggerLoad();
    
    if (success) {
        console.log('[MODEL] Model ready:', modelName);
        return true;
    }
    
    console.warn('[MODEL] Model load may have failed:', modelName);
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
    
    // If there are existing messages, ask user what to do (non-blocking)
    const shouldClearChat = messages.length > 0;
    if (shouldClearChat) {
        // Just show a quick toast notification - don't block the switch
        if (typeof showToast === 'function') {
            showToast('Chat history cleared for new model', 'info');
        }
        clearChatSilent();
    }
    
    currentModel = newModel;
    
    // Sync selection across other model dropdowns
    syncModelSelects(newModel, 'chatModel');
    
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
