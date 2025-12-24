// Helper to format error messages properly
function formatError(error) {
    if (typeof error === 'string') return error;
    if (error && error.message) return error.message;
    if (error && typeof error === 'object') {
        try {
            return JSON.stringify(error);
        } catch (e) {
            return 'Unknown error occurred';
        }
    }
    return 'Unknown error occurred';
}

// Initialize USB paths based on server-detected OS
async function initUSBPaths() {
    try {
        const response = await fetch('/v1/filesystem/common-paths');
        if (response.ok) {
            const data = await response.json();
            if (data.paths && data.paths.length > 0) {
                // Find the first USB/removable path, or use home directory
                const usbPath = data.paths.find(p => p.exists && (
                    p.label.toLowerCase().includes('usb') || 
                    p.label.toLowerCase().includes('volume') ||
                    p.label.toLowerCase().includes('removable')
                ));
                
                // Update hint with detected OS info
                const hintEl = document.getElementById('usbPathHint');
                if (hintEl && data.os) {
                    const osHints = {
                        'linux': 'Linux: Check /media or /mnt for USB drives',
                        'darwin': 'macOS: Check /Volumes for USB drives',
                        'windows': 'Windows: Use drive letters like D:\\ or E:\\'
                    };
                    hintEl.textContent = osHints[data.os] || 'Click the folder icon to browse';
                }
            }
        }
    } catch (e) {
        // Could not initialize USB paths - non-critical
    }
}

// Initialize on page load
document.addEventListener('DOMContentLoaded', initUSBPaths);

async function searchModels() {
    const query = document.getElementById('searchQuery').value.trim();
    if (!query) {
        const results = document.getElementById('searchResults');
        results.innerHTML = '<p class="text-sm text-orange-500">Please enter a search query</p>';
        return;
    }

    const results = document.getElementById('searchResults');
    results.innerHTML = '<p class="text-sm text-secondary">Searching HuggingFace...</p>';

    try {
        // Add timeout to prevent hanging
        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), 180000); // 3 minute timeout for low-end machines
        
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

        // Get list of installed models to check for duplicates
        let installedModels = [];
        try {
            const installedResp = await fetch('/models');
            const installedData = await installedResp.json();
            installedModels = (installedData.data || []).map(m => m.id.toLowerCase());
        } catch (e) {
            // Ignore - we'll just show all as downloadable
        }

        results.innerHTML = '';
        models.slice(0, 10).forEach(model => {
            const div = document.createElement('div');
            div.className = 'model-item';
            const downloadCmd = model.download_command || `offgrid download ${model.id}`;
            const sizeInfo = model.size_gb ? `${model.size_gb} GB` : 'Size unknown';
            const quantInfo = model.best_quant ? ` · ${model.best_quant}` : '';
            const escapedCmd = downloadCmd.replace(/'/g, "\\'");
            
            // Check if this model is already installed (by checking if best_file matches any installed model)
            const bestFile = (model.best_file || '').replace('.gguf', '').toLowerCase();
            const modelName = (model.name || model.id || '').toLowerCase();
            const isInstalled = installedModels.some(installed => 
                installed.includes(bestFile) || 
                bestFile.includes(installed) ||
                installed.includes(modelName.split('/').pop())
            );
            
            const buttonHtml = isInstalled 
                ? `<span class="text-xs text-green-400 px-2 py-1">Installed</span>`
                : `<button onclick="downloadModelWithCommand('${escapedCmd}', this)" class="btn btn-primary btn-sm">Download</button>`;
            
            div.innerHTML = `
                <div class="flex justify-between items-center">
                    <div class="flex-1">
                        <div class="font-semibold text-sm text-accent">${model.name || model.id}</div>
                        <div class="text-xs text-secondary mt-1">${sizeInfo}${quantInfo} · ${model.downloads || 0} downloads</div>
                    </div>
                    ${buttonHtml}
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
                    <p class="text-red-400 mb-2">Search timeout: HuggingFace API not responding</p>
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

// Download model with specific command - uses download modal like quick start
async function downloadModelWithCommand(command, buttonEl) {
    // Parse command to extract repository, file_name, and quantization
    // Examples: 
    //   offgrid download TheBloke/Llama-2-7B-Chat-GGUF --quant Q4_K_M
    //   offgrid download nomic-ai/nomic-embed-text-v1.5-GGUF --file nomic-embed-text-v1.5.Q4_K_M.gguf
    const parts = command.split(' ');
    const downloadIdx = parts.indexOf('download');
    const repository = downloadIdx >= 0 ? parts[downloadIdx + 1] : '';
    
    // Check for --file flag first (exact filename)
    const fileIdx = parts.indexOf('--file');
    const fileName = fileIdx >= 0 ? parts[fileIdx + 1] : '';
    
    // Check for --quant flag
    const quantIdx = parts.indexOf('--quant');
    const quantization = quantIdx >= 0 ? parts[quantIdx + 1] : (fileName ? '' : 'Q4_K_M');
    
    const modelName = repository.split('/').pop() || repository;
    
    if (!repository) {
        showModal({ type: 'error', title: 'Error', message: 'Invalid download command', confirmText: 'OK' });
        return;
    }
    
    // Disable button
    if (buttonEl) {
        buttonEl.disabled = true;
        buttonEl.innerHTML = 'Starting...';
    }
    
    // Show download modal (from onboarding.js)
    if (typeof showDownloadModal === 'function') {
        showDownloadModal(modelName, 'Calculating...');
    }
    
    try {
        // Build request body - prefer file_name over quantization
        const requestBody = { repository };
        if (fileName) {
            requestBody.file_name = fileName;
        } else if (quantization) {
            requestBody.quantization = quantization;
        }
        
        // Start download via API
        const startResp = await fetch('/v1/models/download', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(requestBody)
        });

        if (!startResp.ok) {
            const err = await startResp.json().catch(() => ({}));
            if (typeof showDownloadError === 'function') {
                showDownloadError(err.error || `Failed to start download: HTTP ${startResp.status}`);
            } else {
                showModal({ type: 'error', title: 'Download Failed', message: err.error || 'Failed to start download', confirmText: 'OK' });
            }
            if (buttonEl) { buttonEl.disabled = false; buttonEl.innerHTML = 'Download'; }
            return;
        }

        const startData = await startResp.json();
        
        // Check if model already exists
        if (startData.exists) {
            if (typeof hideDownloadModal === 'function') {
                hideDownloadModal();
            }
            showModal({
                type: 'info',
                title: 'Model Already Installed',
                message: `<strong>${startData.file_name || modelName}</strong> is already installed.<br><br>You can use it right away from the Chat tab.`,
                confirmText: 'OK'
            });
            if (buttonEl) { buttonEl.disabled = false; buttonEl.innerHTML = 'Download'; }
            return;
        }
        
        if (!startData.success) {
            if (typeof showDownloadError === 'function') {
                showDownloadError(startData.message || 'Failed to start download');
            }
            if (buttonEl) { buttonEl.disabled = false; buttonEl.innerHTML = 'Download'; }
            return;
        }

        // Poll for progress
        let completed = false;
        while (!completed) {
            await new Promise(r => setTimeout(r, 300));
            
            // Check if download was cancelled via currentDownloadAbort
            if (typeof currentDownloadAbort !== 'undefined' && currentDownloadAbort?.signal?.aborted) {
                completed = true;
                break;
            }
            
            try {
                const progressResp = await fetch('/v1/models/download/progress');
                if (!progressResp.ok) continue;
                
                const progress = await progressResp.json();
                
                for (const [filename, p] of Object.entries(progress)) {
                    if (p.status === 'downloading') {
                        if (p.bytes_total > 0 && typeof updateDownloadProgress === 'function') {
                            updateDownloadProgress(p.bytes_done, p.bytes_total, p.speed || 0);
                        }
                    } else if (p.status === 'complete' && p.percent >= 99) {
                        completed = true;
                        if (typeof showDownloadComplete === 'function') {
                            showDownloadComplete();
                        }
                        // Invalidate ModelManager cache so new model appears
                        if (typeof ModelManager !== 'undefined') {
                            ModelManager.invalidateCache();
                        }
                        // Refresh model list
                        await loadInstalledModels();
                        break;
                    } else if (p.status === 'failed') {
                        if (typeof showDownloadError === 'function') {
                            showDownloadError(p.error || 'Download failed');
                        }
                        completed = true;
                        break;
                    } else if (p.status === 'cancelled') {
                        // Download was cancelled, just exit the loop
                        completed = true;
                        break;
                    }
                }
            } catch (e) {
                // Ignore polling errors
            }
        }
    } catch (error) {
        if (typeof showDownloadError === 'function') {
            showDownloadError(error.message);
        } else {
            showModal({ type: 'error', title: 'Download Failed', message: error.message, confirmText: 'OK' });
        }
    }
    
    // Reset button
    if (buttonEl) {
        buttonEl.disabled = false;
        buttonEl.innerHTML = 'Download';
    }
}

// Download model (fallback for installed models)
async function downloadModel(modelName, buttonEl) {
    downloadModelWithCommand(`offgrid download ${modelName}`, buttonEl);
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
            
            // Format size display
            const sizeDisplay = model.size_gb || '';
            
            div.innerHTML = `
                <div class="flex justify-between items-start gap-3">
                    <div class="flex-1 min-w-0">
                        <div class="font-semibold text-sm text-accent break-words" title="${model.id}">${model.id}</div>
                        <div class="flex gap-2 mt-1 flex-wrap">
                            <span class="text-xs bg-secondary px-1.5 py-0.5 rounded text-secondary font-mono">GGUF</span>
                            ${sizeDisplay ? `<span class="text-xs text-secondary">${sizeDisplay}</span>` : ''}
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

// Execute the actual removal via API
async function confirmRemoveModel(modelId) {
    try {
        // Clear saved model if it's the one being deleted
        if (currentModel === modelId) {
            currentModel = '';
            localStorage.removeItem('offgrid_current_model');
        }
        
        // Call delete API
        const resp = await fetch('/v1/models/delete', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ model_id: modelId })
        });
        
        const data = await resp.json();
        
        if (!resp.ok || !data.success) {
            showModal({
                type: 'error',
                title: 'Remove Failed',
                message: data.error || data.message || 'Failed to remove model',
                confirmText: 'OK'
            });
            return;
        }
        
        // Success - invalidate cache and refresh model list
        if (typeof ModelManager !== 'undefined') {
            ModelManager.invalidateCache();
        }
        
        showModal({
            type: 'success',
            title: 'Model Removed',
            message: `${modelId} has been deleted.`,
            confirmText: 'OK'
        });
        
        await loadInstalledModels();
        
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
        statusDiv.innerHTML = '<span class="text-orange-500">Please select a path using the folder icon or enter a path manually</span>';
        return;
    }
    
    statusDiv.innerHTML = '';
    resultsDiv.innerHTML = '<span class="text-accent">Scanning for models...</span>';
    
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
                resultsDiv.innerHTML = '<span class="text-secondary">No GGUF models found in this location</span>';
            }
        } else {
            const errorMsg = formatError(data.error) || 'Scan failed';
            resultsDiv.innerHTML = `<span class="text-red-400">${errorMsg}</span>`;
        }
    } catch (error) {
        resultsDiv.innerHTML = `<span class="text-red-400">Failed to scan: ${formatError(error)}</span>`;
    }
}

async function importFromUSB() {
    const usbPath = document.getElementById('usbImportPath').value.trim();
    const statusDiv = document.getElementById('usbImportStatus');
    const progressDiv = document.getElementById('usbImportProgress');
    const progressBar = document.getElementById('usbImportProgressBar');
    const progressText = document.getElementById('usbImportProgressText');
    
    if (!usbPath) {
        statusDiv.innerHTML = '<span class="text-orange-500">Please select a path using the folder icon or enter a path manually</span>';
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
            progressText.innerHTML = `Successfully imported ${count} model(s)`;
            setTimeout(() => {
                progressDiv.classList.add('hidden');
                statusDiv.innerHTML = `<span class="text-green-400">Import complete - ${count} model(s) added</span>`;
            }, 2000);
            loadInstalledModels(); // Refresh the models list
        } else {
            const errorMsg = formatError(data.error) || 'Import failed';
            progressDiv.classList.add('hidden');
            statusDiv.innerHTML = `<span class="text-red-400">${errorMsg}</span>`;
        }
    } catch (error) {
        progressDiv.classList.add('hidden');
        statusDiv.innerHTML = `<span class="text-red-400">Failed to import: ${formatError(error)}</span>`;
    }
}

async function exportToUSB() {
    const usbPath = document.getElementById('usbExportPath').value.trim();
    const statusDiv = document.getElementById('usbExportStatus');
    const progressDiv = document.getElementById('usbExportProgress');
    const progressBar = document.getElementById('usbExportProgressBar');
    const progressText = document.getElementById('usbExportProgressText');
    
    if (!usbPath) {
        statusDiv.innerHTML = '<span class="text-orange-500">Please select a path using the folder icon or enter a path manually</span>';
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
            progressText.innerHTML = `Exported ${count} model(s) (${size.toFixed(2)} GB)`;
            setTimeout(() => {
                progressDiv.classList.add('hidden');
                statusDiv.innerHTML = `
                    <div class="p-2 bg-secondary rounded border border-green-500">
                        <div class="text-green-400 font-medium">Export Complete</div>
                        <div class="text-xs text-secondary mt-1">
                            ${count} model(s) exported (${size.toFixed(2)} GB)<br>
                            Location: ${usbPath}/offgrid-models/<br>
                            Manifest and README included
                        </div>
                    </div>
                `;
            }, 2000);
        } else {
            const errorMsg = formatError(data.error) || 'Export failed';
            progressDiv.classList.add('hidden');
            statusDiv.innerHTML = `<span class="text-red-400">${errorMsg}</span>`;
        }
    } catch (error) {
        progressDiv.classList.add('hidden');
        statusDiv.innerHTML = `<span class="text-red-400">Failed to export: ${formatError(error)}</span>`;
    }
}

// Terminal commands
