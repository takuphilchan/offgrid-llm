// =====================================================
// CENTRALIZED MODEL MANAGER
// Single source of truth for all model state & operations
// =====================================================

const ModelManager = {
    // State
    _models: [],                    // Cached model list
    _lastFetch: 0,                 // Timestamp of last fetch
    _cacheTTL: 60000,              // 1 minute cache TTL
    _currentModel: null,           // Currently selected model
    _loadedModel: null,            // Actually loaded in server memory
    _isLoading: false,             // Loading in progress
    _loadingPromise: null,         // Current loading promise (deduplication)
    _loadingModel: null,           // Model being loaded
    _abortController: null,        // For cancelling loads
    _subscribers: new Set(),       // UI update callbacks
    _initialized: false,
    
    // =========================================
    // INITIALIZATION
    // =========================================
    
    async init() {
        if (this._initialized) return;
        this._initialized = true;
        
        // Restore persisted model selection
        const savedModel = localStorage.getItem('offgrid_current_model');
        if (savedModel) {
            this._currentModel = savedModel;
        }
        
        // Initial model list fetch
        await this.refreshModels(true);
        
        // Check what's actually loaded on server
        await this._syncLoadedState();
        
        console.log('[ModelManager] Initialized:', {
            models: this._models.length,
            current: this._currentModel,
            loaded: this._loadedModel
        });
    },
    
    // =========================================
    // MODEL LIST MANAGEMENT
    // =========================================
    
    /**
     * Get cached models or fetch if stale
     * @param {boolean} forceRefresh - Force a server fetch
     * @returns {Promise<Array>} List of models
     */
    async getModels(forceRefresh = false) {
        const now = Date.now();
        const isStale = (now - this._lastFetch) > this._cacheTTL;
        
        if (!forceRefresh && !isStale && this._models.length > 0) {
            return this._models;
        }
        
        return this.refreshModels(forceRefresh);
    },
    
    /**
     * Fetch fresh model list from server
     */
    async refreshModels(triggerServerRefresh = false) {
        try {
            if (triggerServerRefresh) {
                await fetch('/models/refresh', { 
                    method: 'POST', 
                    signal: AbortSignal.timeout(5000) 
                }).catch(() => {});
            }
            
            const resp = await fetch('/models', { 
                signal: AbortSignal.timeout(10000) 
            });
            const data = await resp.json();
            
            if (data.data && data.data.length > 0) {
                this._models = data.data;
                this._lastFetch = Date.now();
            }
        } catch (e) {
            console.error('[ModelManager] Failed to fetch models:', e);
        }
        
        return this._models;
    },
    
    /**
     * Get LLM models only (exclude embeddings)
     */
    async getLLMModels() {
        const models = await this.getModels();
        return models.filter(m => 
            m.type !== 'embedding' &&
            !m.id.toLowerCase().includes('embed') &&
            !m.id.toLowerCase().includes('minilm') &&
            !m.id.toLowerCase().includes('bge') &&
            !m.id.toLowerCase().includes('nomic')
        );
    },
    
    /**
     * Get embedding models only
     */
    async getEmbeddingModels() {
        const models = await this.getModels();
        return models.filter(m => 
            m.type === 'embedding' ||
            m.tags?.includes('embedding')
        );
    },
    
    /**
     * Invalidate cache (call after model download/delete)
     */
    invalidateCache() {
        this._lastFetch = 0;
    },
    
    // =========================================
    // MODEL LOADING & SWITCHING
    // =========================================
    
    /**
     * Get currently selected model
     */
    getCurrentModel() {
        return this._currentModel;
    },
    
    /**
     * Get model actually loaded in server memory
     */
    getLoadedModel() {
        return this._loadedModel;
    },
    
    /**
     * Check if a specific model is loaded
     */
    isModelLoaded(modelId) {
        return this._loadedModel === modelId;
    },
    
    /**
     * Check if we're currently loading a model
     */
    isLoading() {
        return this._isLoading;
    },
    
    /**
     * Select and load a model. Deduplicates concurrent requests.
     * @param {string} modelId - Model to load
     * @param {object} options - { skipWarm: bool, source: string }
     * @returns {Promise<boolean>} Success
     */
    async selectModel(modelId, options = {}) {
        if (!modelId) return false;
        
        const { skipWarm = false, source = 'unknown' } = options;
        
        // Already selected and loaded - instant return
        if (modelId === this._currentModel && modelId === this._loadedModel) {
            console.log('[ModelManager] Model already selected & loaded:', modelId);
            this._notify('ready', modelId);
            return true;
        }
        
        // Already loading this model - return existing promise
        if (this._isLoading && this._loadingModel === modelId && this._loadingPromise) {
            console.log('[ModelManager] Already loading, reusing promise:', modelId);
            return this._loadingPromise;
        }
        
        // Cancel any previous load
        if (this._abortController) {
            console.log('[ModelManager] Cancelling previous load:', this._loadingModel);
            this._abortController.abort();
        }
        
        // Update selection immediately
        this._currentModel = modelId;
        localStorage.setItem('offgrid_current_model', modelId);
        
        // Skip embedding models - they don't need warming
        if (this._isEmbeddingModel(modelId)) {
            console.log('[ModelManager] Skipping warm for embedding model:', modelId);
            this._notify('ready', modelId);
            return true;
        }
        
        // Check if already loaded on server
        const isCached = await this._checkServerCache(modelId);
        
        if (isCached) {
            console.log('[ModelManager] Model already in server cache:', modelId);
            this._loadedModel = modelId;
            this._notify('ready', modelId);
            return true;
        }
        
        if (skipWarm) {
            this._notify('ready', modelId);
            return true;
        }
        
        // Start loading
        console.log('[ModelManager] Loading model:', modelId, 'from:', source);
        this._isLoading = true;
        this._loadingModel = modelId;
        this._abortController = new AbortController();
        
        const startTime = Date.now();
        this._notify('loading', modelId);
        
        // Progress updater
        const progressInterval = setInterval(() => {
            if (!this._isLoading || this._loadingModel !== modelId) {
                clearInterval(progressInterval);
                return;
            }
            const elapsed = ((Date.now() - startTime) / 1000).toFixed(1) + 's';
            this._notify('loading', modelId, elapsed);
        }, 100);
        
        // Create loading promise
        this._loadingPromise = this._loadModel(modelId, this._abortController.signal);
        
        try {
            const success = await this._loadingPromise;
            clearInterval(progressInterval);
            
            if (success && this._loadingModel === modelId) {
                this._loadedModel = modelId;
                this._notify('ready', modelId);
                console.log('[ModelManager] Model loaded:', modelId, 
                    'in', ((Date.now() - startTime) / 1000).toFixed(1) + 's');
            }
            
            return success;
        } catch (e) {
            clearInterval(progressInterval);
            if (e.name !== 'AbortError') {
                console.error('[ModelManager] Load failed:', e);
                this._notify('error', modelId);
            }
            return false;
        } finally {
            if (this._loadingModel === modelId) {
                this._isLoading = false;
                this._loadingModel = null;
                this._loadingPromise = null;
                this._abortController = null;
            }
        }
    },
    
    /**
     * Warm model without changing selection (background load)
     */
    async warmModel(modelId) {
        if (!modelId || this._isEmbeddingModel(modelId)) return;
        
        const isCached = await this._checkServerCache(modelId);
        if (isCached) {
            this._loadedModel = modelId;
            return true;
        }
        
        // Don't interrupt active loading
        if (this._isLoading) return false;
        
        console.log('[ModelManager] Warming model in background:', modelId);
        this._notify('warming', modelId);
        
        try {
            await this._loadModel(modelId);
            this._loadedModel = modelId;
            this._notify('ready', modelId);
            return true;
        } catch (e) {
            console.warn('[ModelManager] Warm failed:', e);
            return false;
        }
    },
    
    // =========================================
    // INTERNAL HELPERS
    // =========================================
    
    _isEmbeddingModel(modelId) {
        if (!modelId) return false;
        const lower = modelId.toLowerCase();
        return lower.includes('embed') || 
               lower.includes('bge') || 
               lower.includes('minilm') || 
               lower.includes('nomic');
    },
    
    async _checkServerCache(modelId) {
        try {
            const resp = await fetch('/v1/cache/stats', { 
                signal: AbortSignal.timeout(2000) 
            });
            if (!resp.ok) return false;
            
            const stats = await resp.json();
            const cachedModels = stats.model_cache?.cached_models || [];
            const loadedModels = stats.loaded_models || [];
            
            return cachedModels.some(m => m.model_id === modelId || m.id === modelId) ||
                   loadedModels.some(m => m === modelId || m.model_id === modelId);
        } catch (e) {
            return false;
        }
    },
    
    async _syncLoadedState() {
        try {
            const resp = await fetch('/v1/cache/stats', { 
                signal: AbortSignal.timeout(2000) 
            });
            if (!resp.ok) return;
            
            const stats = await resp.json();
            const loadedModels = stats.loaded_models || [];
            const cachedModels = stats.model_cache?.cached_models || [];
            
            // Check if current model is loaded
            if (this._currentModel) {
                const isLoaded = loadedModels.includes(this._currentModel) ||
                    cachedModels.some(m => m.model_id === this._currentModel);
                if (isLoaded) {
                    this._loadedModel = this._currentModel;
                }
            }
        } catch (e) {
            // Ignore
        }
    },
    
    async _loadModel(modelId, signal = null) {
        const options = {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                model: modelId,
                messages: [{ role: 'user', content: 'hi' }],
                stream: false,
                max_tokens: 1,
                temperature: 0
            })
        };
        
        if (signal) {
            options.signal = signal;
        } else {
            options.signal = AbortSignal.timeout(300000); // 5 min timeout
        }
        
        const resp = await fetch('/v1/chat/completions', options);
        return resp.ok;
    },
    
    // =========================================
    // UI UPDATES & SUBSCRIPTIONS
    // =========================================
    
    /**
     * Subscribe to model state changes
     * @param {function} callback - (state, modelId, extra) => void
     */
    subscribe(callback) {
        this._subscribers.add(callback);
        return () => this._subscribers.delete(callback);
    },
    
    /**
     * Notify all subscribers of state change
     */
    _notify(state, modelId, extra = '') {
        for (const callback of this._subscribers) {
            try {
                callback(state, modelId, extra);
            } catch (e) {
                console.error('[ModelManager] Subscriber error:', e);
            }
        }
    },
    
    // =========================================
    // UI HELPERS
    // =========================================
    
    /**
     * Populate a select element with LLM models
     */
    async populateLLMSelect(selectId, onChange = null) {
        const select = document.getElementById(selectId);
        if (!select) return;
        
        const models = await this.getLLMModels();
        const previousValue = select.value || this._currentModel;
        
        select.innerHTML = '';
        
        if (models.length === 0) {
            select.innerHTML = '<option value="">No models available</option>';
            return;
        }
        
        models.sort((a, b) => a.id.localeCompare(b.id));
        
        for (const m of models) {
            const opt = document.createElement('option');
            opt.value = m.id;
            opt.textContent = m.id;
            opt.title = m.id;
            select.appendChild(opt);
        }
        
        // Restore/sync selection
        if (previousValue && models.some(m => m.id === previousValue)) {
            select.value = previousValue;
        } else if (this._currentModel && models.some(m => m.id === this._currentModel)) {
            select.value = this._currentModel;
        } else if (models.length > 0) {
            select.value = models[0].id;
            this._currentModel = models[0].id;
        }
        
        // Set up change handler
        if (onChange) {
            select.onchange = onChange;
        }
        
        return select.value;
    },
    
    /**
     * Populate a select element with embedding models
     */
    async populateEmbeddingSelect(selectId) {
        const select = document.getElementById(selectId);
        if (!select) return;
        
        const models = await this.getEmbeddingModels();
        
        select.innerHTML = '';
        
        if (models.length === 0) {
            select.innerHTML = '<option value="">No embedding models available</option>';
            return;
        }
        
        models.sort((a, b) => a.id.localeCompare(b.id));
        
        for (const m of models) {
            const opt = document.createElement('option');
            opt.value = m.id;
            opt.textContent = m.id;
            opt.title = m.id;
            select.appendChild(opt);
        }
        
        select.value = models[0].id;
        return select.value;
    },
    
    /**
     * Sync all model selects to current model
     */
    syncAllSelects(excludeId = null) {
        const selectIds = ['chatModel', 'agentModel', 'benchmarkModel', 'loraBaseModel', 'voiceAssistantModel'];
        
        for (const id of selectIds) {
            if (id === excludeId) continue;
            const select = document.getElementById(id);
            if (select && this._currentModel) {
                const hasOption = Array.from(select.options).some(o => o.value === this._currentModel);
                if (hasOption) {
                    select.value = this._currentModel;
                }
            }
        }
    }
};

// =====================================================
// GLOBAL HELPERS (for backward compatibility)
// =====================================================

// Keep currentModel as global for backward compat
let currentModel = '';

// Initialize on load
document.addEventListener('DOMContentLoaded', async () => {
    await ModelManager.init();
    currentModel = ModelManager.getCurrentModel() || '';
    
    // Subscribe to updates for sidebar status
    ModelManager.subscribe((state, modelId, extra) => {
        // Update global
        if (state === 'ready' || state === 'loading') {
            currentModel = modelId;
        }
        
        // Update sidebar if function exists
        if (typeof updateSidebarStatus === 'function') {
            updateSidebarStatus(state, modelId, extra);
        }
    });
});

// Backward-compatible function - delegates to ModelManager
async function switchToModel(modelId, sourceSelectId = null) {
    if (!modelId) return false;
    
    const success = await ModelManager.selectModel(modelId, { source: sourceSelectId });
    if (success) {
        ModelManager.syncAllSelects(sourceSelectId);
    }
    return success;
}

// Backward-compatible cache refresh
async function refreshModelsCache(force = false) {
    return ModelManager.getModels(force);
}

// Backward-compatible cache check
async function isModelCached(modelName) {
    return ModelManager._checkServerCache(modelName);
}

// Backward-compatible model warming
async function warmModelInBackground(modelName) {
    return ModelManager.warmModel(modelName);
}
