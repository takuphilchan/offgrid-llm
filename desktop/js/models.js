// =====================================================
// MODEL LOADING MODAL HELPERS
// Shows a progress modal during model loading
// =====================================================

const modelLoadingTips = [
    "Tip: First load takes longer - subsequent loads are faster",
    "Tip: Keep the model loaded for faster responses",
    "Tip: Smaller models load faster on low-RAM systems",
    "Tip: Model weights are cached for faster reloading",
    "Tip: Pre-warm models by hovering over them in the list",
    "Tip: Use quantized models (Q4/Q5) for faster loading",
    "Tip: SSDs dramatically improve model load times",
    "Tip: Close unused applications to free up memory"
];

let modelLoadingTipInterval = null;
let modelLoadingStartTime = 0;
let modelLoadingTimeInterval = null;

function showModelLoadingModal(modelName) {
    const modal = document.getElementById('modelLoadingModal');
    if (!modal) return;
    
    // Set initial state
    const nameEl = document.getElementById('modelLoadingName');
    const statusEl = document.getElementById('modelLoadingStatus');
    const progressBar = document.getElementById('modelLoadingProgressBar');
    const phaseEl = document.getElementById('modelLoadingPhase');
    const percentEl = document.getElementById('modelLoadingPercent');
    const timeEl = document.getElementById('modelLoadingTime');
    const tipEl = document.getElementById('modelLoadingTip');
    
    if (nameEl) nameEl.textContent = modelName || 'Loading Model';
    if (statusEl) statusEl.textContent = 'Initializing...';
    if (progressBar) progressBar.style.width = '0%';
    if (phaseEl) phaseEl.textContent = 'Starting';
    if (percentEl) percentEl.textContent = '0%';
    if (timeEl) timeEl.textContent = '0s elapsed';
    
    // Show random tip
    if (tipEl) {
        const randomTip = modelLoadingTips[Math.floor(Math.random() * modelLoadingTips.length)];
        tipEl.innerHTML = `<p class="text-xs text-secondary italic">${randomTip}</p>`;
    }
    
    // Start tip rotation
    modelLoadingStartTime = Date.now();
    let tipIndex = 0;
    modelLoadingTipInterval = setInterval(() => {
        if (tipEl) {
            tipIndex = (tipIndex + 1) % modelLoadingTips.length;
            tipEl.innerHTML = `<p class="text-xs text-secondary italic">${modelLoadingTips[tipIndex]}</p>`;
        }
    }, 5000);
    
    // Start elapsed time counter
    modelLoadingTimeInterval = setInterval(() => {
        if (timeEl) {
            const elapsed = ((Date.now() - modelLoadingStartTime) / 1000).toFixed(0);
            timeEl.textContent = `${elapsed}s elapsed`;
        }
    }, 1000);
    
    modal.classList.remove('hidden');
}

function updateModelLoadingModal(progress, phase, status) {
    const progressBar = document.getElementById('modelLoadingProgressBar');
    const phaseEl = document.getElementById('modelLoadingPhase');
    const percentEl = document.getElementById('modelLoadingPercent');
    const statusEl = document.getElementById('modelLoadingStatus');
    
    if (progressBar) progressBar.style.width = `${progress}%`;
    if (phaseEl) phaseEl.textContent = phase || 'Loading';
    if (percentEl) percentEl.textContent = `${progress}%`;
    if (statusEl && status) statusEl.textContent = status;
}

function hideModelLoadingModal() {
    const modal = document.getElementById('modelLoadingModal');
    if (modal) modal.classList.add('hidden');
    
    // Clean up intervals
    if (modelLoadingTipInterval) {
        clearInterval(modelLoadingTipInterval);
        modelLoadingTipInterval = null;
    }
    if (modelLoadingTimeInterval) {
        clearInterval(modelLoadingTimeInterval);
        modelLoadingTimeInterval = null;
    }
}

// =====================================================
// CHAT STATS AND STATUS
// =====================================================

function updateChatStats() {
    document.getElementById('messageCount').textContent = `${messages.length} messages`;
    const totalTokens = messages.reduce((sum, m) => sum + (m.content?.length || 0), 0);
    document.getElementById('tokenCount').textContent = `~${Math.ceil(totalTokens / 4)} tokens`;
}

// Update sidebar status display
function updateSidebarStatus(state, modelName = '', extra = '') {
    console.log('[updateSidebarStatus] Called with:', state, modelName, extra);
    
    const statusDot = document.getElementById('statusDot');
    const statusText = document.getElementById('statusText');
    const modelBadge = document.getElementById('activeModelBadge');
    const modelNameEl = document.getElementById('activeModelName');
    
    // Also update system status bar model name (footer)
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
            // Show model badge only when ready
            if (modelName && modelBadge && modelNameEl) {
                modelBadge.classList.remove('hidden');
                modelNameEl.textContent = modelName;
                modelNameEl.title = modelName;
            }
            break;
        case 'loading':
        case 'switching':
        case 'warming':
            statusDot.className = 'w-2 h-2 rounded-full bg-orange-500 animate-pulse';
            statusText.className = 'text-xs font-medium text-orange-500';
            statusText.textContent = state === 'loading' ? 'Loading...' : 
                                     state === 'switching' ? 'Switching...' : 'Warming...';
            // Hide model badge during loading (modal shows the model name)
            if (modelBadge) modelBadge.classList.add('hidden');
            break;
        case 'error':
            statusDot.className = 'w-2 h-2 rounded-full bg-red-500';
            statusText.className = 'text-xs font-medium text-red-400';
            statusText.textContent = 'Error';
            if (modelBadge) modelBadge.classList.add('hidden');
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
// NOTE: These are kept for backward compatibility, actual logic is in ModelManager
let currentSwitchController = null;
let currentSwitchModelId = null;

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
        console.log('[MODEL] Calling switchToModel for Agent:', select.value);
        const result = await switchToModel(select.value, 'agentModel');
        console.log('[MODEL] switchToModel result:', result);
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
// NOTE: Prefer using ModelManager.getModels() instead
let cachedModels = null;
let lastModelsRefresh = 0;
const MODELS_CACHE_TTL = 30000; // 30 second cache

// Refresh models from server - delegates to ModelManager if available
async function refreshModelsCache(force = false) {
    // Use ModelManager if available
    if (typeof ModelManager !== 'undefined') {
        return ModelManager.getModels(force);
    }
    
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

// Load models for chat - uses ModelManager for efficient caching
async function loadChatModels() {
    // Use ModelManager if available (preferred - no redundant API calls)
    if (typeof ModelManager !== 'undefined') {
        const selectedModel = await ModelManager.populateLLMSelect('chatModel', handleModelChange);
        
        // Update empty state UI
        if (typeof updateEmptyState === 'function') {
            updateEmptyState();
        }
        
        // Warm model in background if not already loaded
        if (selectedModel && !ModelManager.isModelLoaded(selectedModel)) {
            ModelManager.warmModel(selectedModel);
        }
        return;
    }
    
    // Legacy fallback
    try {
        const models = await refreshModelsCache();
        
        const select = document.getElementById('chatModel');
        const previousSelection = select.value || currentModel;
        
        select.innerHTML = '';
        
        if (models.length === 0) {
            select.innerHTML = '<option value="">No LLM models available</option>';
            return;
        }
        
        // Sort: LLM before embeddings, larger first, then alphabetically
        models.sort((a, b) => {
            const aIsEmbed = (a.type === 'embedding' || a.id.toLowerCase().includes('embed'));
            const bIsEmbed = (b.type === 'embedding' || b.id.toLowerCase().includes('embed'));
            if (aIsEmbed !== bIsEmbed) return aIsEmbed ? 1 : -1;
            if (a.size && b.size && a.size !== b.size) return b.size - a.size;
            return a.id.localeCompare(b.id);
        });
        
        models.forEach(m => {
            const opt = document.createElement('option');
            opt.value = m.id;
            // Format with size
            let sizeInfo = '';
            if (m.size_gb) {
                sizeInfo = ` (${m.size_gb})`;
            } else if (m.size) {
                sizeInfo = ` (${formatFileSize(m.size)})`;
            }
            opt.textContent = m.id + sizeInfo;
            opt.title = m.id + sizeInfo;
            select.appendChild(opt);
        });
        
        if (previousSelection && models.some(m => m.id === previousSelection)) {
            currentModel = previousSelection;
            select.value = previousSelection;
        } else if (!currentModel && models.length > 0) {
            currentModel = models[0].id;
            select.value = currentModel;
        }

        select.onchange = null;
        select.onchange = function(e) {
            handleModelChange();
        };
        
        if (typeof updateEmptyState === 'function') {
            updateEmptyState();
        }
        
        if (currentModel) {
            warmModelInBackground(currentModel);
        }
    } catch (e) {
        console.error('Failed to load models:', e);
        document.getElementById('chatModel').innerHTML = '<option value="">Error loading models</option>';
    }
}

// Warm up a model in the background - delegates to ModelManager if available
async function warmModelInBackground(modelName) {
    // Use ModelManager if available
    if (typeof ModelManager !== 'undefined') {
        return ModelManager.warmModel(modelName);
    }
    
    // Legacy fallback
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

// Load embedding models - uses ModelManager for efficient caching
async function loadEmbeddingModels() {
    // Use ModelManager if available
    if (typeof ModelManager !== 'undefined') {
        await ModelManager.populateEmbeddingSelect('embeddingModel');
        return;
    }
    
    // Legacy fallback
    try {
        const allModels = await refreshModelsCache();
        
        const models = allModels.filter(m => {
            return m.type === 'embedding' || m.tags?.includes('embedding');
        });
        
        const select = document.getElementById('embeddingModel');
        if (!select) {
            return;
        }
        select.innerHTML = '';
        
        if (models.length === 0) {
            select.innerHTML = '<option value="">No embedding models available</option>';
            return;
        }
        
        // Sort by size (smaller first for embeddings), then alphabetically
        models.sort((a, b) => {
            if (a.size && b.size && a.size !== b.size) return a.size - b.size;
            return a.id.localeCompare(b.id);
        });
        
        models.forEach(m => {
            const opt = document.createElement('option');
            opt.value = m.id;
            // Format with size
            let sizeInfo = '';
            if (m.size_gb) {
                sizeInfo = ` (${m.size_gb})`;
            } else if (m.size) {
                sizeInfo = ` (${formatFileSize(m.size)})`;
            }
            opt.textContent = m.id + sizeInfo;
            opt.title = m.id + sizeInfo;
            select.appendChild(opt);
        });
        
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

// Handle model dropdown change - uses ModelManager for efficient loading
async function handleModelChange() {
    const select = document.getElementById('chatModel');
    const newModel = select.value;
    
    if (!newModel) {
        return;
    }
    
    // Use ModelManager for efficient loading (deduplicates, cancels previous, tracks state)
    if (typeof ModelManager !== 'undefined') {
        // Don't switch if already on this model (use ModelManager's state)
        if (ModelManager.getCurrentModel() === newModel && ModelManager.isModelLoaded(newModel)) {
            console.log('[handleModelChange] Already on this model:', newModel);
            return;
        }
        
        // If there are existing messages, clear them
        if (typeof messages !== 'undefined' && messages.length > 0) {
            if (typeof showToast === 'function') {
                showToast('Chat history cleared for new model', 'info');
            }
            if (typeof clearChatSilent === 'function') {
                clearChatSilent();
            }
        }
        
        // Disable UI while loading
        const chatInput = document.getElementById('chatInput');
        const sendBtn = document.getElementById('sendBtn');
        if (chatInput) chatInput.disabled = true;
        if (sendBtn) sendBtn.disabled = true;
        
        // Check VLM support
        const models = await ModelManager.getModels();
        const modelInfo = models.find(m => m.id === newModel);
        const isVLM = typeof modelSupportsVision === 'function' && modelSupportsVision(modelInfo, newModel);
        
        const imageUploadBtn = document.getElementById('imageUploadBtn');
        if (imageUploadBtn) {
            if (isVLM) {
                imageUploadBtn.classList.remove('hidden');
            } else {
                imageUploadBtn.classList.add('hidden');
                if (typeof clearAllImages === 'function') clearAllImages();
            }
        }
        
        // Update global currentModel to stay in sync
        currentModel = newModel;
        
        // Let ModelManager handle loading with proper state tracking
        await ModelManager.selectModel(newModel, { source: 'chatModel' });
        ModelManager.syncAllSelects('chatModel');
        
        // Re-enable UI
        if (chatInput) chatInput.disabled = false;
        if (sendBtn) sendBtn.disabled = false;
        if (chatInput) chatInput.focus();
        
        if (typeof updateEmptyState === 'function') {
            updateEmptyState();
        }
        return;
    }
    
    // Legacy fallback - Don't switch if already on this model
    if (currentModel === newModel) {
        return;
    }
    
    currentModel = newModel;
    syncModelSelects(newModel, 'chatModel');
    
    if (typeof updateCacheIndicator === 'function') {
        updateCacheIndicator(false);
    }

    // Check if VLM and toggle upload button
    try {
        const resp = await fetch('/models');
        const data = await resp.json();
        const modelInfo = data.data.find(m => m.id === newModel);
        const isVLM = typeof modelSupportsVision === 'function' && modelSupportsVision(modelInfo, newModel);
        
        const imageUploadBtn = document.getElementById('imageUploadBtn');
        if (imageUploadBtn) {
            if (isVLM) {
                imageUploadBtn.classList.remove('hidden');
            } else {
                imageUploadBtn.classList.add('hidden');
                if (typeof clearAllImages === 'function') clearAllImages();
            }
        }
    } catch (e) {
        console.error('Error checking model type:', e);
    }
    
    // Disable UI while loading model
    const chatInput = document.getElementById('chatInput');
    const sendBtn = document.getElementById('sendBtn');
    
    if (chatInput) chatInput.disabled = true;
    if (sendBtn) sendBtn.disabled = true;
    
    const isCached = await isModelCached(newModel);
    
    let loadingMs = 0;
    const loadingStartTime = Date.now();
    const loadState = isCached ? 'switching' : 'loading';
    updateSidebarStatus(loadState, newModel);
    
    const loadingInterval = setInterval(() => {
        loadingMs = Date.now() - loadingStartTime;
        const seconds = (loadingMs / 1000).toFixed(1) + 's';
        updateSidebarStatus(loadState, newModel, seconds);
    }, 100);
    
    const isReady = await waitForModelReady(newModel);
    
    clearInterval(loadingInterval);
    
    if (isReady) {
        updateSidebarStatus('ready', newModel);
    } else {
        console.warn('[MODEL CHANGE] Model not confirmed ready - may have issues');
        updateSidebarStatus('ready', newModel);
    }
    
    if (chatInput) chatInput.disabled = false;
    if (sendBtn) sendBtn.disabled = false;
    if (chatInput) chatInput.focus();
    
    if (typeof updateEmptyState === 'function') {
        updateEmptyState();
    }
}

// Modal dialog system
