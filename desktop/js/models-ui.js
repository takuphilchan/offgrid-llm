async function searchModels() {
    const query = document.getElementById('searchQuery').value.trim();
    if (!query) {
        const results = document.getElementById('searchResults');
        results.innerHTML = '<p class="text-sm text-yellow-400">⚠ Please enter a search query</p>';
        return;
    }

    const results = document.getElementById('searchResults');
    results.innerHTML = '<p class="text-sm text-secondary">🔍 Searching HuggingFace...</p>';

    try {
        // Add timeout to prevent hanging
        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), 35000); // 35 second timeout (backend has 30s)
        
        const resp = await fetch(`/v1/search?query=${encodeURIComponent(query)}`, {
            signal: controller.signal
        });
        clearTimeout(timeoutId);
        
        if (!resp.ok) {
            throw new Error(`HTTP ${resp.status}: ${resp.statusText}`);
        }
        
        const data = await resp.json();
        
        if (data.error) {
            throw new Error(typeof data.error === 'string' ? data.error : JSON.stringify(data.error));
        }
        
        const models = data.results || [];

        if (models.length === 0) {
            results.innerHTML = '<p class="text-sm text-secondary">No models found. Try different keywords like "llama", "phi", "mistral".</p>';
            return;
        }

        results.innerHTML = '';
        models.slice(0, 10).forEach(model => {
            const div = document.createElement('div');
            div.className = 'model-item';
            const downloadCmd = model.download_command || `offgrid download ${model.id}`;
            const sizeInfo = model.size_gb ? `${model.size_gb} GB` : 'Size unknown';
            const quantInfo = model.best_quant ? ` · ${model.best_quant}` : '';
            const escapedCmd = downloadCmd.replace(/'/g, "\\'");
            
            div.innerHTML = `
                <div class="flex justify-between items-center">
                    <div class="flex-1">
                        <div class="font-semibold text-sm text-accent">${model.name || model.id}</div>
                        <div class="text-xs text-secondary mt-1">${sizeInfo}${quantInfo} · ${model.downloads || 0} downloads</div>
                    </div>
                    <button onclick="downloadModelWithCommand('${escapedCmd}')" class="btn btn-primary btn-sm">Download</button>
                </div>
            `;
            results.appendChild(div);
        });
    } catch (error) {
        console.error('Search error:', error);
        
        // Provide helpful error message based on error type
        let errorHtml = '';
        if (error.name === 'AbortError' || error.message.includes('Timeout') || error.message.includes('exceeded')) {
            errorHtml = `
                <div class="text-sm">
                    <p class="text-red-400 mb-2">⚠ Search timeout: HuggingFace API not responding</p>
                    <p class="text-secondary mb-3">This usually means no internet connection or HuggingFace is slow.</p>
                    <div class="text-xs text-secondary">
                        <p class="font-semibold text-accent mb-2">Try instead:</p>
                        <p class="mb-1">• Use the terminal: <span class="font-mono bg-secondary px-2 py-1 rounded">offgrid search llama</span></p>
                        <p class="mb-1">• Download directly: <span class="font-mono bg-secondary px-2 py-1 rounded">offgrid download TheBloke/TinyLlama-1.1B-Chat-v1.0-GGUF</span></p>
                    </div>
                </div>
            `;
        } else {
            errorHtml = `
                <div class="text-sm">
                    <p class="text-red-400 mb-2">Search error: ${error.message}</p>
                    <p class="text-secondary mb-2">Use the Terminal tab to search instead:</p>
                    <p class="text-xs font-mono bg-secondary px-2 py-1 rounded inline-block">offgrid search ${query}</p>
                </div>
            `;
        }
        results.innerHTML = errorHtml;
    }
}

// Download model with specific command
async function downloadModelWithCommand(command) {
    // Switch to terminal tab and execute download command
    switchTab('terminal');
    document.getElementById('terminalInput').value = command;
    runCommand();
}

// Download model (fallback for installed models)
async function downloadModel(modelName) {
    // Switch to terminal tab and execute download command
    switchTab('terminal');
    document.getElementById('terminalInput').value = `offgrid download ${modelName}`;
    runCommand();
}

// Load installed models
async function loadInstalledModels() {
    const container = document.getElementById('installedModels');
    container.innerHTML = '<p class="text-sm text-secondary">Loading...</p>';

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

        if (models.length === 0) {
            container.innerHTML = '<p class="text-sm text-secondary text-center py-8">No models installed yet.<br><span class="text-xs">Search and download models from the right panel.</span></p>';
            updateModelCount();
            return;
        }

        container.innerHTML = '';
        models.forEach(model => {
            const div = document.createElement('div');
            div.className = 'model-card model-item';
            const modelId = (model.id || '').replace(/'/g, "\\'");
            
            // Format size if available (assuming it might be in metadata)
            // Since the API might not return size directly in the root object, we check
            // But for now we'll just show the ID cleanly
            
            div.innerHTML = `
                <div class="flex justify-between items-center gap-3">
                    <div class="flex-1 min-w-0">
                        <div class="font-semibold text-sm text-accent truncate" title="${model.id}">${model.id}</div>
                        <div class="flex gap-2 mt-1">
                            <span class="text-xs bg-secondary px-1.5 py-0.5 rounded text-secondary font-mono">GGUF</span>
                        </div>
                    </div>
                    <button onclick="removeModel('${modelId}')" class="btn btn-danger btn-sm flex-shrink-0">Remove</button>
                </div>
            `;
            container.appendChild(div);
        });
        updateModelCount();
    } catch (error) {
        container.innerHTML = '<p class="text-sm text-red-400">Failed to load models</p>';
        updateModelCount();
    }
}

// Remove model
async function removeModel(modelId) {
    showModal({
        type: 'error',
        title: 'Remove Model?',
        message: `Are you sure you want to remove <strong>${modelId}</strong>?<br><br>This will delete the model file from your system.`,
        confirmText: 'Remove',
        cancelText: 'Cancel',
        onConfirm: () => {
            confirmRemoveModel(modelId);
        }
    });
}

// Execute the actual removal via terminal
async function confirmRemoveModel(modelId) {
    try {
        // Clear saved model if it's the one being deleted
        if (currentModel === modelId) {
            currentModel = '';
            localStorage.removeItem('offgrid_current_model');
        }
        
        // Switch to terminal tab to show progress
        switchTab('terminal');
        
        // Execute remove command
        const input = document.getElementById('terminalInput');
        input.value = `offgrid remove ${modelId} --yes`;
        
        // Trigger the command
        const event = new KeyboardEvent('keydown', { key: 'Enter', code: 'Enter', keyCode: 13 });
        input.dispatchEvent(event);
        
        // Note: Model list refresh happens in handleOffgridCommand after command completes
    } catch (error) {
        showModal({
            type: 'error',
            title: 'Remove Failed',
            message: `Failed to remove model: ${error.message}`,
            confirmText: 'OK'
        });
    }
}

// Toggle USB section visibility
function toggleUSBSection() {
    const content = document.getElementById('usbSectionContent');
    const arrow = document.getElementById('usbSectionArrow');
    content.classList.toggle('hidden');
    arrow.style.transform = content.classList.contains('hidden') ? '' : 'rotate(180deg)';
}

// Refresh all models
async function refreshAllModels() {
    await fetchModels();
    updateModelCount();
}

// Update model count badge
function updateModelCount() {
    const countEl = document.getElementById('modelCount');
    if (countEl) {
        const models = document.querySelectorAll('#installedModels .model-card');
        countEl.textContent = models.length + ' model' + (models.length !== 1 ? 's' : '');
    }
}

// USB Import/Export Functions

// Scan USB drive for models
async function scanUSB() {
    const usbPath = document.getElementById('usbImportPath').value.trim();
    const resultsDiv = document.getElementById('usbScanResults');
    const statusDiv = document.getElementById('usbImportStatus');
    
    if (!usbPath) {
        statusDiv.innerHTML = '<span class="text-yellow-400">⚠ Please enter a USB path</span>';
        return;
    }
    
    statusDiv.innerHTML = '';
    resultsDiv.innerHTML = '<span class="text-accent">🔍 Scanning USB drive...</span>';
    
    try {
        const response = await fetch('/v1/usb/scan', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ usb_path: usbPath })
        });
        
        const data = await response.json();
        
        if (response.ok) {
            const count = data.count || 0;
            if (count > 0) {
                const totalSize = data.models.reduce((sum, m) => sum + m.size, 0);
                const totalGB = (totalSize / (1024 * 1024 * 1024)).toFixed(2);
                
                resultsDiv.innerHTML = `
                    <div class="p-2 bg-secondary rounded border border-accent">
                        <div class="font-medium text-accent mb-2">Found ${count} model(s) - ${totalGB} GB</div>
                        <div class="space-y-1 max-h-32 overflow-y-auto">
                            ${data.models.map(m => `
                                <div class="text-xs flex justify-between">
                                    <span class="font-mono">${m.file_name}</span>
                                    <span class="text-secondary">${(m.size / (1024*1024*1024)).toFixed(2)} GB</span>
                                </div>
                            `).join('')}
                        </div>
                    </div>
                `;
            } else {
                resultsDiv.innerHTML = '<span class="text-secondary">No GGUF models found</span>';
            }
        } else {
            const error = data.error || 'Scan failed';
            resultsDiv.innerHTML = `<span class="text-red-400">✗ ${error}</span>`;
        }
    } catch (error) {
        resultsDiv.innerHTML = `<span class="text-red-400">✗ Error: ${error.message}</span>`;
    }
}

async function importFromUSB() {
    const usbPath = document.getElementById('usbImportPath').value.trim();
    const statusDiv = document.getElementById('usbImportStatus');
    const progressDiv = document.getElementById('usbImportProgress');
    const progressBar = document.getElementById('usbImportProgressBar');
    const progressText = document.getElementById('usbImportProgressText');
    
    if (!usbPath) {
        statusDiv.innerHTML = '<span class="text-yellow-400">⚠ Please enter a USB path</span>';
        return;
    }
    
    statusDiv.innerHTML = '';
    progressDiv.classList.remove('hidden');
    progressBar.style.width = '0%';
    progressText.innerHTML = 'Starting import...';
    
    try {
        const response = await fetch('/v1/usb/import', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path: usbPath })
        });
        
        const data = await response.json();
        
        if (response.ok) {
            const count = data.imported_count || 0;
            progressBar.style.width = '100%';
            progressText.innerHTML = `✓ Successfully imported ${count} model(s)`;
            setTimeout(() => {
                progressDiv.classList.add('hidden');
                statusDiv.innerHTML = `<span class="text-green-400">✓ Import complete - ${count} model(s) added</span>`;
            }, 2000);
            loadInstalledModels(); // Refresh the models list
        } else {
            const error = data.error || 'Import failed';
            progressDiv.classList.add('hidden');
            statusDiv.innerHTML = `<span class="text-red-400">✗ ${error}</span>`;
        }
    } catch (error) {
        progressDiv.classList.add('hidden');
        statusDiv.innerHTML = `<span class="text-red-400">✗ Error: ${error.message}</span>`;
    }
}

async function exportToUSB() {
    const usbPath = document.getElementById('usbExportPath').value.trim();
    const statusDiv = document.getElementById('usbExportStatus');
    const progressDiv = document.getElementById('usbExportProgress');
    const progressBar = document.getElementById('usbExportProgressBar');
    const progressText = document.getElementById('usbExportProgressText');
    
    if (!usbPath) {
        statusDiv.innerHTML = '<span class="text-yellow-400">⚠ Please enter a USB path</span>';
        return;
    }
    
    statusDiv.innerHTML = '';
    progressDiv.classList.remove('hidden');
    progressBar.style.width = '0%';
    progressText.innerHTML = 'Starting export...';
    
    try {
        const response = await fetch('/v1/usb/export', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path: usbPath })
        });
        
        const data = await response.json();
        
        if (response.ok) {
            const count = data.exported_count || 0;
            const size = data.total_size_gb || 0;
            progressBar.style.width = '100%';
            progressText.innerHTML = `✓ Exported ${count} model(s) (${size.toFixed(2)} GB)`;
            setTimeout(() => {
                progressDiv.classList.add('hidden');
                statusDiv.innerHTML = `
                    <div class="p-2 bg-secondary rounded border border-green-500">
                        <div class="text-green-400 font-medium">✓ Export Complete</div>
                        <div class="text-xs text-secondary mt-1">
                            ${count} model(s) exported (${size.toFixed(2)} GB)<br>
                            Location: ${usbPath}/offgrid-models/<br>
                            Manifest and README included
                        </div>
                    </div>
                `;
            }, 2000);
        } else {
            const error = data.error || 'Export failed';
            progressDiv.classList.add('hidden');
            statusDiv.innerHTML = `<span class="text-red-400">✗ ${error}</span>`;
        }
    } catch (error) {
        progressDiv.classList.add('hidden');
        statusDiv.innerHTML = `<span class="text-red-400">✗ Error: ${error.message}</span>`;
    }
}

// Terminal commands
