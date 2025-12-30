// =====================================================
// CENTRALIZED MODEL MANAGER
// Single source of truth for all model state & operations
// =====================================================

// Simple helper to update footer - called from multiple places
function updateFooterModel(modelId) {
    const statusModelName = document.getElementById('statusModelName');
    if (statusModelName && modelId) {
        const shortName = modelId.split('/').pop().split(':')[0];
        statusModelName.textContent = shortName || modelId;
        console.log('[Footer] Updated to:', shortName);
    }
}

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
    _lastSwitchTime: 0,            // Rate limiting - prevent rapid switching
    _switchCooldown: 1000,         // 1 second cooldown between switches
    _activeEventSources: [],       // Track SSE connections for cleanup
    _activeIntervals: [],          // Track intervals for cleanup
    
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
        // Skip fetch if currently loading a model (server may be busy)
        if (this._isLoading) {
            console.log('[ModelManager] Skipping model fetch during loading');
            return this._models;
        }
        
        try {
            if (triggerServerRefresh) {
                await fetch('/models/refresh', { 
                    method: 'POST', 
                    signal: AbortSignal.timeout(5000) 
                }).catch(() => {});
            }
            
            const resp = await fetch('/models', { 
                signal: AbortSignal.timeout(60000)  // 60s timeout for slow systems
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
        
        // Rate limiting - prevent rapid switching that causes zombie processes
        const now = Date.now();
        if (now - this._lastSwitchTime < this._switchCooldown && this._isLoading) {
            console.log('[ModelManager] Rate limited - switch too fast, waiting for current load');
            // If switching to same model, just return existing promise
            if (this._loadingModel === modelId && this._loadingPromise) {
                return this._loadingPromise;
            }
            // Otherwise wait a bit before allowing switch
            await new Promise(r => setTimeout(r, this._switchCooldown));
        }
        this._lastSwitchTime = now;
        
        // Already selected and loaded - instant return but still sync UI
        if (modelId === this._currentModel && modelId === this._loadedModel) {
            console.log('[ModelManager] Model already selected & loaded:', modelId);
            this._notify('ready', modelId);
            // Ensure all dropdowns are synced
            this.syncAllSelects(source);
            return true;
        }
        
        // Already loading this model - return existing promise
        if (this._isLoading && this._loadingModel === modelId && this._loadingPromise) {
            console.log('[ModelManager] Already loading, reusing promise:', modelId);
            return this._loadingPromise;
        }
        
        // Cancel any previous load and cleanup resources
        this._cleanupPendingLoad();
        
        // Update selection immediately
        this._currentModel = modelId;
        localStorage.setItem('offgrid_current_model', modelId);
        
        // Skip embedding models - they don't need warming
        if (this._isEmbeddingModel(modelId)) {
            console.log('[ModelManager] Skipping warm for embedding model:', modelId);
            this._notify('ready', modelId);
            this.syncAllSelects();
            return true;
        }
        
        // Check if already loaded on server
        const isCached = await this._checkServerCache(modelId);
        
        if (isCached) {
            console.log('[ModelManager] Model already in server cache:', modelId);
            this._loadedModel = modelId;
            this._notify('ready', modelId);
            // Sync all dropdowns to this model
            this.syncAllSelects();
            return true;
        }
        
        if (skipWarm) {
            this._notify('ready', modelId);
            this.syncAllSelects();
            return true;
        }
        
        // Start loading
        console.log('[ModelManager] Loading model:', modelId, 'from:', source);
        this._isLoading = true;
        this._loadingModel = modelId;
        this._abortController = new AbortController();
        
        const startTime = Date.now();
        this._notify('loading', modelId, '0%');
        
        // Use server-side progress tracking for accurate feedback
        let cleanupProgress = null;
        try {
            cleanupProgress = this.monitorLoadingProgress((progress) => {
                if (this._loadingModel !== modelId) return;
                
                const elapsed = ((Date.now() - startTime) / 1000).toFixed(1) + 's';
                const pct = progress.progress || 0;
                const phase = progress.phase || 'loading';
                const msg = progress.message || 'Loading...';
                
                console.log(`[ModelManager] Progress: ${pct}% phase=${phase} msg="${msg}"`);
                
                // If server says ready, we can notify immediately
                if (phase === 'ready') {
                    console.log('[ModelManager] Server reports model ready!');
                }
                
                // Provide detailed feedback
                this._notify('loading', modelId, `${pct}% - ${msg} (${elapsed})`);
            });
        } catch (e) {
            // Fallback to simple timer if SSE not available
            console.warn('[ModelManager] Progress monitoring unavailable, using fallback');
        }
        
        // Fallback progress updater (in case SSE fails)
        const progressInterval = setInterval(async () => {
            if (!this._isLoading || this._loadingModel !== modelId) {
                clearInterval(progressInterval);
                // Remove from tracking
                const idx = this._activeIntervals.indexOf(progressInterval);
                if (idx > -1) this._activeIntervals.splice(idx, 1);
                return;
            }
            // Try to get snapshot
            const progress = await this.getLoadingProgress();
            if (progress.phase !== 'idle') {
                const elapsed = ((Date.now() - startTime) / 1000).toFixed(1) + 's';
                this._notify('loading', modelId, `${progress.progress}% (${elapsed})`);
            }
        }, 500);
        
        // Track interval for cleanup
        this._activeIntervals.push(progressInterval);
        
        // Create loading promise
        this._loadingPromise = this._loadModel(modelId, this._abortController.signal);
        
        try {
            const success = await this._loadingPromise;
            clearInterval(progressInterval);
            if (cleanupProgress) cleanupProgress();
            
            if (success && this._loadingModel === modelId) {
                this._loadedModel = modelId;
                this._notify('ready', modelId);
                // Sync all dropdowns to this model
                this.syncAllSelects();
                console.log('[ModelManager] Model loaded:', modelId, 
                    'in', ((Date.now() - startTime) / 1000).toFixed(1) + 's');
            }
            
            return success;
        } catch (e) {
            clearInterval(progressInterval);
            if (cleanupProgress) cleanupProgress();
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
    
    /**
     * Clean up pending load resources (abort controller, SSE, intervals)
     */
    _cleanupPendingLoad() {
        // Abort any pending fetch
        if (this._abortController) {
            console.log('[ModelManager] Aborting previous load:', this._loadingModel);
            try {
                this._abortController.abort();
            } catch (e) {
                // Ignore abort errors
            }
            this._abortController = null;
        }
        
        // Close all SSE connections
        for (const es of this._activeEventSources) {
            try {
                es.close();
            } catch (e) {
                // Ignore close errors
            }
        }
        this._activeEventSources = [];
        
        // Clear all intervals
        for (const interval of this._activeIntervals) {
            try {
                clearInterval(interval);
            } catch (e) {
                // Ignore clear errors
            }
        }
        this._activeIntervals = [];
        
        // Reset loading state
        this._isLoading = false;
        this._loadingPromise = null;
    },
    
    async _checkServerCache(modelId) {
        try {
            const resp = await fetch('/v1/cache/stats', { 
                signal: AbortSignal.timeout(2000) 
            });
            if (!resp.ok) return false;
            
            const stats = await resp.json();
            const cachedModels = stats.model_cache?.cached_models || [];
            
            // Check if model is in the cached models list
            const isCached = cachedModels.some(m => m.model_id === modelId);
            console.log('[ModelManager] Cache check:', modelId, 'cached:', isCached, 'models:', cachedModels.map(m => m.model_id));
            return isCached;
        } catch (e) {
            console.warn('[ModelManager] Cache check failed:', e);
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
            const cachedModels = stats.model_cache?.cached_models || [];
            
            // Check if current model is loaded
            if (this._currentModel) {
                const isLoaded = cachedModels.some(m => m.model_id === this._currentModel);
                if (isLoaded) {
                    this._loadedModel = this._currentModel;
                    console.log('[ModelManager] Synced loaded state:', this._currentModel, 'is loaded');
                }
            }
            
            // If we don't have a current model but server has one loaded, sync to it
            if (!this._currentModel && cachedModels.length > 0) {
                this._currentModel = cachedModels[0].model_id;
                this._loadedModel = cachedModels[0].model_id;
                localStorage.setItem('offgrid_current_model', this._currentModel);
                console.log('[ModelManager] Synced to server model:', this._currentModel);
            }
        } catch (e) {
            console.warn('[ModelManager] Sync loaded state failed:', e);
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
    
    /**
     * Monitor loading progress using SSE stream
     * @param {function} onProgress - (progress) => void
     * @returns {function} Cleanup function
     */
    monitorLoadingProgress(onProgress) {
        const eventSource = new EventSource('/v1/loading/progress/stream');
        
        // Track for cleanup
        this._activeEventSources.push(eventSource);
        
        eventSource.onmessage = (event) => {
            try {
                const progress = JSON.parse(event.data);
                onProgress(progress);
                
                // Close when done
                if (progress.phase === 'ready' || progress.phase === 'failed') {
                    eventSource.close();
                    // Remove from tracking
                    const idx = this._activeEventSources.indexOf(eventSource);
                    if (idx > -1) this._activeEventSources.splice(idx, 1);
                }
            } catch (e) {
                console.warn('[ModelManager] Error parsing progress:', e);
            }
        };
        
        eventSource.onerror = () => {
            eventSource.close();
            // Remove from tracking
            const idx = this._activeEventSources.indexOf(eventSource);
            if (idx > -1) this._activeEventSources.splice(idx, 1);
        };
        
        return () => {
            eventSource.close();
            const idx = this._activeEventSources.indexOf(eventSource);
            if (idx > -1) this._activeEventSources.splice(idx, 1);
        };
    },
    
    /**
     * Get current loading progress snapshot
     */
    async getLoadingProgress() {
        try {
            const resp = await fetch('/v1/loading/progress', {
                signal: AbortSignal.timeout(2000)
            });
            if (resp.ok) {
                return await resp.json();
            }
        } catch (e) {
            // Ignore
        }
        return { phase: 'idle', progress: 0 };
    },
    
    /**
     * Pre-warm a model by path for faster switching (uses aggressive read-ahead)
     */
    async prewarmModelByPath(modelPath) {
        try {
            // Use the new fast prewarm endpoint with concurrent I/O
            await fetch('/v1/loading/prewarm', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ model_path: modelPath })
            });
        } catch (e) {
            console.warn('[ModelManager] Fast prewarm failed:', e);
        }
    },
    
    /**
     * Pre-warm a model by ID for faster switching
     */
    async warmModel(modelId) {
        try {
            // Find the model to get its path
            const models = await this.getModels();
            const model = models.find(m => m.id === modelId);
            if (model && model.path) {
                await this.prewarmModelByPath(model.path);
            }
        } catch (e) {
            console.warn('[ModelManager] Warm model failed:', e);
        }
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
        console.log('[ModelManager] _notify called:', state, modelId, extra);
        
        // Update loading modal based on state
        if (state === 'loading' || state === 'warming') {
            if (typeof showModelLoadingModal === 'function') {
                // Show modal on first loading notification
                const modal = document.getElementById('modelLoadingModal');
                if (modal && modal.classList.contains('hidden')) {
                    showModelLoadingModal(modelId);
                }
                // Parse progress from extra (e.g., "45% - Loading... (3.2s)")
                const pctMatch = extra.match(/(\d+)%/);
                const phaseMatch = extra.match(/- ([^(]+)/);
                const progress = pctMatch ? parseInt(pctMatch[1]) : 0;
                const phase = phaseMatch ? phaseMatch[1].trim() : 'Loading';
                if (typeof updateModelLoadingModal === 'function') {
                    updateModelLoadingModal(progress, phase, extra);
                }
            }
        } else if (state === 'ready' || state === 'error') {
            // Hide loading modal when done
            if (typeof hideModelLoadingModal === 'function') {
                hideModelLoadingModal();
            }
        }
        
        console.log('[ModelManager] Notifying', this._subscribers.size, 'subscribers');
        for (const callback of this._subscribers) {
            try {
                callback(state, modelId, extra);
            } catch (e) {
                console.error('[ModelManager] Subscriber error:', e);
            }
        }
        
        // Always sync all dropdowns on ready state
        if (state === 'ready') {
            console.log('[ModelManager] Syncing all selects on ready');
            this.syncAllSelects();
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
        
        // Sort models: alphabetically by ID, but group by type (LLM first, then embedding)
        models.sort((a, b) => {
            // First sort by type (llm before embedding)
            const aIsEmbed = (a.type === 'embedding' || a.id.toLowerCase().includes('embed'));
            const bIsEmbed = (b.type === 'embedding' || b.id.toLowerCase().includes('embed'));
            if (aIsEmbed !== bIsEmbed) {
                return aIsEmbed ? 1 : -1;
            }
            // Then sort by size (larger first for better visibility)
            if (a.size && b.size && a.size !== b.size) {
                return b.size - a.size;
            }
            // Finally alphabetically
            return a.id.localeCompare(b.id);
        });
        
        for (const m of models) {
            const opt = document.createElement('option');
            opt.value = m.id;
            
            // Format display with size
            let displayText = m.id;
            if (m.size_gb) {
                displayText = `${m.id} (${m.size_gb})`;
            } else if (m.size) {
                // Calculate size from bytes if size_gb not provided
                const sizeGB = m.size / (1024 * 1024 * 1024);
                if (sizeGB >= 1) {
                    displayText = `${m.id} (${sizeGB.toFixed(1)} GB)`;
                } else {
                    const sizeMB = m.size / (1024 * 1024);
                    displayText = `${m.id} (${sizeMB.toFixed(0)} MB)`;
                }
            }
            
            opt.textContent = displayText;
            opt.title = m.id + (m.size_gb ? ` - ${m.size_gb}` : '');
            // Store model info for hover pre-warming
            opt.dataset.modelId = m.id;
            opt.dataset.modelSize = m.size || 0;
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
        
        // Set up change handler that ALWAYS updates footer + syncs
        // Remove any existing listener first
        if (select._modelChangeHandler) {
            select.removeEventListener('change', select._modelChangeHandler);
        }
        
        // Create unified handler that updates footer and calls optional callback
        select._modelChangeHandler = async (e) => {
            const modelId = e.target.value;
            console.log('[populateLLMSelect] Change detected on', selectId, '→', modelId);
            
            // Prevent double handling
            if (select._lastHandledModel === modelId && Date.now() - (select._lastHandledTime || 0) < 500) {
                console.log('[populateLLMSelect] Ignoring duplicate change event');
                return;
            }
            select._lastHandledModel = modelId;
            select._lastHandledTime = Date.now();
            
            // Always update footer immediately
            updateFooterModel(modelId);
            
            // Always switch model and sync
            await switchToModel(modelId, selectId);
            
            // Call additional handler if provided
            if (onChange) {
                onChange(e);
            }
        };
        
        select.addEventListener('change', select._modelChangeHandler);
        
        // Add focus handler for pre-warming on hover/focus
        // When user opens dropdown, pre-warm hovered model
        this._setupPrewarmOnHover(select);
        
        return select.value;
    },
    
    /**
     * Set up pre-warming when user hovers over model options
     */
    _setupPrewarmOnHover(select) {
        let prewarmTimeout = null;
        let lastPrewarmed = null;
        
        // Pre-warm on focus (when dropdown opens)
        select.addEventListener('focus', () => {
            // Mark that dropdown is open
            select.dataset.dropdownOpen = 'true';
        });
        
        select.addEventListener('blur', () => {
            select.dataset.dropdownOpen = 'false';
            if (prewarmTimeout) {
                clearTimeout(prewarmTimeout);
                prewarmTimeout = null;
            }
        });
        
        // Pre-warm on mouseover of options (works in some browsers)
        select.addEventListener('mouseover', (e) => {
            if (e.target.tagName === 'OPTION') {
                const modelId = e.target.value;
                if (modelId && modelId !== this._loadedModel && modelId !== lastPrewarmed) {
                    // Debounce to avoid spamming
                    if (prewarmTimeout) clearTimeout(prewarmTimeout);
                    prewarmTimeout = setTimeout(() => {
                        lastPrewarmed = modelId;
                        this.warmModel(modelId).catch(() => {});
                    }, 300);
                }
            }
        });
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
        
        // Sort by size (smaller first for embeddings), then alphabetically
        models.sort((a, b) => {
            if (a.size && b.size && a.size !== b.size) {
                return a.size - b.size;
            }
            return a.id.localeCompare(b.id);
        });
        
        for (const m of models) {
            const opt = document.createElement('option');
            opt.value = m.id;
            
            // Format display with size
            let displayText = m.id;
            if (m.size_gb) {
                displayText = `${m.id} (${m.size_gb})`;
            } else if (m.size) {
                const sizeMB = m.size / (1024 * 1024);
                if (sizeMB >= 1024) {
                    displayText = `${m.id} (${(sizeMB / 1024).toFixed(1)} GB)`;
                } else {
                    displayText = `${m.id} (${sizeMB.toFixed(0)} MB)`;
                }
            }
            
            opt.textContent = displayText;
            opt.title = m.id + (m.size_gb ? ` - ${m.size_gb}` : '');
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
        
        console.log('[ModelManager] syncAllSelects - syncing to:', this._currentModel);
        
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
        
        // Also directly update footer status bar
        const statusModelName = document.getElementById('statusModelName');
        if (statusModelName && this._currentModel) {
            const shortName = this._currentModel.split('/').pop().split(':')[0];
            statusModelName.textContent = shortName || this._currentModel;
            console.log('[ModelManager] Updated footer model to:', shortName);
        }
    }
};

// =====================================================
// GLOBAL HELPERS (for backward compatibility)
// =====================================================

// currentModel is already declared in utils.js - just use it

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
    console.log('[switchToModel] Called with:', modelId, 'from:', sourceSelectId);
    if (!modelId) return false;
    
    const success = await ModelManager.selectModel(modelId, { source: sourceSelectId });
    console.log('[switchToModel] selectModel returned:', success);
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
