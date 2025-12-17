function updateChatStats() {
    document.getElementById('messageCount').textContent = `${messages.length} messages`;
    const totalTokens = messages.reduce((sum, m) => sum + (m.content?.length || 0), 0);
    document.getElementById('tokenCount').textContent = `~${Math.ceil(totalTokens / 4)} tokens`;
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
        console.log('[LOAD MODELS] Sorted LLM models:', models.map(m => m.id));
        
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
            console.log('[LOAD MODELS] Restored previous selection:', previousSelection);
        } else if (!currentModel && models.length > 0) {
            // First time - select first model
            currentModel = models[0].id;
            select.value = currentModel;
            console.log('[LOAD MODELS] Auto-selected first model:', currentModel);
        }

        // Add change handler after models are loaded
        select.onchange = null; // Clear any existing handler
        select.onchange = function(e) {
            console.log('[DROPDOWN] Change event fired! Value:', e.target.value);
            handleModelChange();
        };
        console.log('[LOAD MODELS] Event listener attached to dropdown via onchange');
    } catch (e) {
        console.error('Failed to load models:', e);
        document.getElementById('chatModel').innerHTML = '<option value="">Error loading models</option>';
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
        console.log('[LOAD EMBEDDING MODELS] Sorted models:', models.map(m => m.id));
        
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

// Wait for model to be fully loaded and ready
async function waitForModelReady(modelName) {
    console.log('[HEALTH CHECK] Waiting for model to be ready:', modelName);
    const maxAttempts = 60; // 60 seconds max wait
    const pollInterval = 1000; // Check every second
    
    // Check if this is an embedding model
    const resp = await fetch('/models');
    const data = await resp.json();
    const modelInfo = data.data.find(m => m.id === modelName);
    // Check type from metadata, or fallback to name heuristics if metadata missing
    const isEmbeddingModel = modelInfo?.type === 'embedding' || 
                           modelName.toLowerCase().includes('embed') || 
                           modelName.toLowerCase().includes('bge') ||
                           modelName.toLowerCase().includes('nomic');
    console.log('[HEALTH CHECK] Model type:', modelInfo?.type, 'Is embedding:', isEmbeddingModel);
    
    for (let attempt = 1; attempt <= maxAttempts; attempt++) {
        try {
            console.log(`[HEALTH CHECK] Attempt ${attempt}/${maxAttempts}`);
            
            let testResponse;
            if (isEmbeddingModel) {
                // For embedding models, use embeddings endpoint
                testResponse = await fetch('/v1/embeddings', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        model: modelName,
                        input: 'test'
                    }),
                    signal: AbortSignal.timeout(5000)
                });
            } else {
                // For LLM models, use chat completions endpoint
                testResponse = await fetch('/v1/chat/completions', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        model: modelName,
                        messages: [{ role: 'user', content: 'test' }],
                        stream: false,
                        max_tokens: 1,
                        temperature: 0.1
                    }),
                    signal: AbortSignal.timeout(5000)
                });
            }
            
            console.log('[HEALTH CHECK] Response status:', testResponse.status);
            
            // If we get 200, model is ready
            if (testResponse.ok) {
                console.log('[HEALTH CHECK] Model is ready!');
                return true;
            }
            
            // If we get 503 or 500, model is still loading
            if (testResponse.status === 503 || testResponse.status === 500) {
                const errorData = await testResponse.json().catch(() => ({}));
                console.log('[HEALTH CHECK] Model loading:', errorData.error || 'Still initializing...');
            }
            
            console.log(`[HEALTH CHECK] Not ready yet, waiting ${pollInterval}ms...`);
            await new Promise(resolve => setTimeout(resolve, pollInterval));
            
        } catch (error) {
            console.log(`[HEALTH CHECK] Error on attempt ${attempt}:`, error.message);
            // Network errors or timeouts are expected while loading
            await new Promise(resolve => setTimeout(resolve, pollInterval));
        }
    }
    
    console.warn('[HEALTH CHECK] Timeout after 60s - model may not be ready');
    return false; // Return false if timeout
}

// Handle model dropdown change
async function handleModelChange() {
    const select = document.getElementById('chatModel');
    const newModel = select.value;
    
    console.log('[MODEL CHANGE] Dropdown changed to:', newModel);
    console.log('[MODEL CHANGE] Current model:', currentModel);
    
    if (!newModel) {
        console.log('[MODEL CHANGE] No model selected, ignoring');
        return;
    }
    
    // Don't switch if already on this model
    if (currentModel === newModel) {
        console.log('[MODEL CHANGE] Already using this model');
        return;
    }
    
    const oldModel = currentModel;
    
    // If there are existing messages, ask user what to do
    console.log('[MODEL CHANGE] Current message count:', messages.length);
    if (messages.length > 0) {
        console.log('[MODEL CHANGE] Showing confirmation dialog...');
        showModal({
            type: 'warning',
            title: 'Clear Chat History?',
            message: `Switching from <strong>${oldModel || 'no model'}</strong> to <strong>${newModel}</strong>.<br><br>Clear chat history? The new model won't have context from previous messages.`,
            confirmText: 'Start Fresh',
            cancelText: 'Keep History',
            onConfirm: () => {
                console.log('[MODEL CHANGE] User response: CLEAR');
                console.log('[MODEL CHANGE] Clearing chat history');
                clearChatSilent();
            },
            onCancel: () => {
                console.log('[MODEL CHANGE] User response: KEEP');
                console.log('[MODEL CHANGE] Keeping chat history');
            }
        });
    } else {
        console.log('[MODEL CHANGE] No messages, skipping confirmation');
    }
    
    currentModel = newModel;
    console.log('[MODEL CHANGE] Switching from', oldModel, 'to', newModel);

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
    const statusBadge = document.getElementById('statusBadge');
    
    chatInput.disabled = true;
    sendBtn.disabled = true;
    statusBadge.className = 'text-xs font-medium text-yellow-400';
    
    // Show loading progress
    let loadingSeconds = 0;
    statusBadge.textContent = `Loading ${newModel}...`;
    const loadingInterval = setInterval(() => {
        loadingSeconds++;
        statusBadge.textContent = `Loading ${newModel}... ${loadingSeconds}s`;
    }, 1000);
    
    console.log('[MODEL CHANGE] Starting health check for', newModel);
    
    // Wait for model to be ready before allowing messages
    const isReady = await waitForModelReady(newModel);
    
    // Clear loading interval
    clearInterval(loadingInterval);
    
    if (isReady) {
        console.log('[MODEL CHANGE] Model ready - enabling UI');
        statusBadge.className = 'badge badge-success';
        statusBadge.textContent = `Ready (${newModel})`;
    } else {
        console.warn('[MODEL CHANGE] Model not confirmed ready - may have issues');
        statusBadge.className = 'badge badge-warning';
        statusBadge.textContent = `${newModel} (not confirmed)`;
    }
    
    // Re-enable UI
    chatInput.disabled = false;
    sendBtn.disabled = false;
    chatInput.focus();
    
    console.log('[MODEL CHANGE] Model switch complete - ready for messages');
}

// Modal dialog system
