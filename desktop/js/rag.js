// ==================== RAG (Knowledge Base) Functions ====================

let ragEnabled = false;

async function loadRAGStatus() {
    try {
        const response = await fetch('/v1/rag/status');
        const data = await response.json();
        ragEnabled = data.enabled;
        
        const badge = document.getElementById('ragStatusBadge');
        const disableBtn = document.getElementById('ragDisableBtn');
        const modelSelect = document.getElementById('ragEmbeddingModel');
        
        // Skip if elements don't exist on this page
        if (!badge) return;
        
        if (data.enabled) {
            badge.className = 'badge badge-success';
            badge.textContent = 'Enabled';
            if (disableBtn) disableBtn.classList.remove('hidden');
            // Set the current model in the dropdown
            if (data.embedding_model && modelSelect) {
                modelSelect.value = data.embedding_model;
            }
        } else {
            badge.className = 'badge badge-secondary';
            badge.textContent = 'Disabled';
            if (disableBtn) disableBtn.classList.add('hidden');
        }
        
        // Update stats
        if (data.stats) {
            const docCount = document.getElementById('ragDocCount');
            const chunkCount = document.getElementById('ragChunkCount');
            const embeddingCount = document.getElementById('ragEmbeddingCount');
            if (docCount) docCount.textContent = data.stats.document_count || 0;
            if (chunkCount) chunkCount.textContent = data.stats.chunk_count || 0;
            if (embeddingCount) embeddingCount.textContent = data.stats.embedding_count || 0;
        }
    } catch (e) {
        console.error('Failed to load RAG status:', e);
    }
}

async function loadRAGEmbeddingModels() {
    try {
        // First get current RAG status to know which model is active
        const statusResp = await fetch('/v1/rag/status');
        const status = await statusResp.json();
        const ragCurrentModel = status.embedding_model || '';
        const isEnabled = status.enabled;
        
        const select = document.getElementById('ragEmbeddingModel');
        select.innerHTML = '';
        
        // Use ModelManager if available
        let embeddingModels = [];
        if (typeof ModelManager !== 'undefined') {
            embeddingModels = await ModelManager.getEmbeddingModels();
        } else {
            const response = await fetch('/v1/models');
            const data = await response.json();
            embeddingModels = data.data.filter(m => 
                m.id.toLowerCase().includes('embed') || 
                m.id.toLowerCase().includes('minilm') ||
                m.id.toLowerCase().includes('bge') ||
                m.id.toLowerCase().includes('nomic')
            );
        }
        
        let modelsToShow = embeddingModels.length > 0 ? embeddingModels : [];
        
        if (modelsToShow.length === 0) {
            select.innerHTML = '<option value="">No embedding models available</option>';
            return;
        }
        
        modelsToShow.forEach(model => {
            const option = document.createElement('option');
            option.value = model.id;
            option.textContent = model.id;
            select.appendChild(option);
        });
        
        // If RAG is already enabled, set the dropdown to current model
        if (isEnabled && ragCurrentModel) {
            select.value = ragCurrentModel;
        } else if (modelsToShow.length > 0) {
            // Auto-select first embedding model and enable RAG
            select.value = modelsToShow[0].id;
            // Trigger enable (this will call onEmbeddingModelChange behavior)
            await enableRAGWithModel(modelsToShow[0].id);
        }
    } catch (e) {
        console.error('Failed to load embedding models:', e);
    }
}

// Helper to enable RAG silently (without showing alert on auto-enable)
async function enableRAGWithModel(model) {
    try {
        const response = await fetch('/v1/rag/enable', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ embedding_model: model })
        });
        if (response.ok) {
            ragEnabled = true;
            await loadRAGStatus();
        }
    } catch (e) {
        // Silent failure for auto-enable
    }
}

// Called when embedding model dropdown changes
async function onEmbeddingModelChange() {
    const model = document.getElementById('ragEmbeddingModel').value;
    
    if (!model) {
        // If cleared, disable RAG
        if (ragEnabled) {
            await disableRAG();
        }
        return;
    }
    
    // Enable RAG with selected model
    try {
        const response = await fetch('/v1/rag/enable', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ embedding_model: model })
        });
        if (!response.ok) {
            const err = await response.text();
            throw new Error(err);
        }
        // Update local state immediately and refresh UI
        ragEnabled = true;
        await loadRAGStatus();
        showAlert('Knowledge Base enabled with ' + model, 'success');
    } catch (e) {
        showAlert('Failed to enable RAG: ' + e.message, 'error');
    }
}

async function disableRAG() {
    try {
        await fetch('/v1/rag/disable', { method: 'POST' });
        document.getElementById('ragEmbeddingModel').value = '';
        // Update local state immediately and refresh UI
        ragEnabled = false;
        await loadRAGStatus();
    } catch (e) {
        showAlert('Failed to disable RAG: ' + e.message, 'error');
    }
}

// Legacy function - kept for compatibility
async function toggleRAG() {
    if (ragEnabled) {
        await disableRAG();
    } else {
        await onEmbeddingModelChange();
    }
}

// Toggle developer tools section
function toggleDevTools() {
    const content = document.getElementById('devToolsContent');
    const icon = document.getElementById('devToolsIcon');
    content.classList.toggle('hidden');
    icon.style.transform = content.classList.contains('hidden') ? '' : 'rotate(180deg)';
}

async function refreshRAGDocuments() {
    try {
        const response = await fetch('/v1/documents');
        const data = await response.json();
        const list = document.getElementById('ragDocumentsList');
        
        if (!data.documents || data.documents.length === 0) {
            list.innerHTML = `
                <div class="empty-state" style="padding: 1.5rem;">
                    <p class="empty-state-title">No files yet</p>
                    <p class="empty-state-desc">Upload files in Step 2 above</p>
                </div>
            `;
            updateBulkDeleteButton(0);
            return;
        }
        
        // Calculate total size
        const totalSize = data.documents.reduce((sum, doc) => sum + (doc.size || 0), 0);
        
        list.innerHTML = `
            <div class="flex items-center justify-between mb-3 pb-2 border-b border-theme">
                <label class="flex items-center gap-2 text-sm text-secondary cursor-pointer">
                    <input type="checkbox" id="ragSelectAll" onchange="toggleSelectAllDocs()" class="rounded border-theme">
                    <span>Select all (${data.documents.length} files, ${formatBytes(totalSize)})</span>
                </label>
                <button id="ragBulkDeleteBtn" onclick="bulkDeleteDocs()" class="btn btn-danger btn-sm hidden">
                    Delete Selected
                </button>
            </div>
            ${data.documents.map(doc => `
                <div class="flex items-center gap-3 p-3 bg-tertiary rounded-lg hover:bg-tertiary/80 transition-colors group">
                    <input type="checkbox" class="rag-doc-checkbox rounded border-theme" data-id="${doc.id}" onchange="updateBulkDeleteVisibility()">
                    ${getFileTypeIcon(doc.name)}
                    <div class="flex-1 min-w-0">
                        <div class="font-medium text-sm truncate" title="${doc.name}">${doc.name}</div>
                        <div class="text-xs text-secondary">${doc.chunk_count} chunks • ${formatBytes(doc.size)}</div>
                    </div>
                    <button onclick="deleteRAGDocument('${doc.id}')" class="btn btn-secondary btn-sm text-red-400 hover:text-red-300 opacity-0 group-hover:opacity-100 transition-opacity">
                        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/>
                        </svg>
                    </button>
                </div>
            `).join('')}
        `;
        
        loadRAGStatus(); // Refresh stats
    } catch (e) {
        console.error('Failed to load documents:', e);
    }
}

// Get appropriate icon based on file extension
function getFileTypeIcon(filename) {
    const ext = filename.split('.').pop().toLowerCase();
    const iconClass = 'w-8 h-8 flex-shrink-0 p-1.5 rounded-lg';
    
    const icons = {
        // Documents
        docx: { bg: 'bg-blue-500/20', color: 'text-blue-400', icon: 'M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z' },
        doc: { bg: 'bg-blue-500/20', color: 'text-blue-400', icon: 'M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z' },
        txt: { bg: 'bg-gray-500/20', color: 'text-gray-400', icon: 'M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z' },
        md: { bg: 'bg-purple-500/20', color: 'text-purple-400', icon: 'M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z' },
        // Data
        json: { bg: 'bg-yellow-500/20', color: 'text-yellow-400', icon: 'M4 7v10c0 2 1 3 3 3h10c2 0 3-1 3-3V7c0-2-1-3-3-3H7c-2 0-3 1-3 3zm4 3h2m-2 4h6' },
        csv: { bg: 'bg-green-500/20', color: 'text-green-400', icon: 'M3 10h18M3 14h18M9 4v16m6-16v16' },
        xml: { bg: 'bg-orange-500/20', color: 'text-orange-400', icon: 'M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4' },
        // Web
        html: { bg: 'bg-red-500/20', color: 'text-red-400', icon: 'M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4' },
        htm: { bg: 'bg-red-500/20', color: 'text-red-400', icon: 'M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4' },
        // Code
        py: { bg: 'bg-blue-500/20', color: 'text-blue-400', icon: 'M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4' },
        js: { bg: 'bg-yellow-500/20', color: 'text-yellow-400', icon: 'M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4' },
        ts: { bg: 'bg-blue-500/20', color: 'text-blue-400', icon: 'M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4' },
        go: { bg: 'bg-cyan-500/20', color: 'text-cyan-400', icon: 'M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4' },
        rs: { bg: 'bg-orange-500/20', color: 'text-orange-400', icon: 'M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4' },
    };
    
    const config = icons[ext] || { bg: 'bg-accent/20', color: 'text-accent', icon: 'M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z' };
    
    return `
        <div class="${iconClass} ${config.bg}">
            <svg class="w-full h-full ${config.color}" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="${config.icon}"/>
            </svg>
        </div>
    `;
}

// Toggle select all documents
function toggleSelectAllDocs() {
    const selectAll = document.getElementById('ragSelectAll');
    const checkboxes = document.querySelectorAll('.rag-doc-checkbox');
    checkboxes.forEach(cb => cb.checked = selectAll.checked);
    updateBulkDeleteVisibility();
}

// Update bulk delete button visibility
function updateBulkDeleteVisibility() {
    const checkboxes = document.querySelectorAll('.rag-doc-checkbox:checked');
    const btn = document.getElementById('ragBulkDeleteBtn');
    if (btn) {
        if (checkboxes.length > 0) {
            btn.classList.remove('hidden');
            btn.textContent = `Delete Selected (${checkboxes.length})`;
        } else {
            btn.classList.add('hidden');
        }
    }
    
    // Update select all state
    const selectAll = document.getElementById('ragSelectAll');
    const allCheckboxes = document.querySelectorAll('.rag-doc-checkbox');
    if (selectAll && allCheckboxes.length > 0) {
        selectAll.checked = checkboxes.length === allCheckboxes.length;
        selectAll.indeterminate = checkboxes.length > 0 && checkboxes.length < allCheckboxes.length;
    }
}

// Bulk delete documents
async function bulkDeleteDocs() {
    const checkboxes = document.querySelectorAll('.rag-doc-checkbox:checked');
    const ids = Array.from(checkboxes).map(cb => cb.dataset.id);
    
    if (ids.length === 0) return;
    
    showConfirm(`Delete ${ids.length} document${ids.length > 1 ? 's' : ''}?`, async () => {
        let deleted = 0;
        for (const id of ids) {
            try {
                await fetch('/v1/documents/delete', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ id })
                });
                deleted++;
            } catch (e) {
                console.error('Failed to delete:', id, e);
            }
        }
        showAlert(`Deleted ${deleted} document${deleted > 1 ? 's' : ''}`, 'success');
        refreshRAGDocuments();
    }, { title: 'Delete Documents', confirmText: 'Delete All', type: 'error' });
}

// Helper to update bulk delete button count
function updateBulkDeleteButton(count) {
    const btn = document.getElementById('ragBulkDeleteBtn');
    if (btn) btn.classList.add('hidden');
}

function formatBytes(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

async function ingestRAGText() {
    const content = document.getElementById('ragTextInput').value.trim();
    const name = document.getElementById('ragTextName').value.trim() || 'untitled.txt';
    
    if (!content) {
        showAlert('Please enter some text content', 'warning');
        return;
    }
    
    if (!ragEnabled) {
        showAlert('Please enable RAG first by selecting an embedding model', 'warning');
        return;
    }
    
    // Show loading state on button
    const btn = document.querySelector('[onclick="ingestRAGText()"]');
    const originalText = btn ? btn.innerHTML : '';
    if (btn) {
        btn.disabled = true;
        btn.innerHTML = '<svg class="w-4 h-4 mr-1 animate-spin inline" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path></svg>Adding...';
    }
    
    try {
        const response = await fetch('/v1/documents/ingest', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name, content })
        });
        
        if (!response.ok) {
            const err = await response.json();
            throw new Error(err.error || 'Failed to ingest document');
        }
        
        document.getElementById('ragTextInput').value = '';
        document.getElementById('ragTextName').value = '';
        refreshRAGDocuments();
        showAlert('Document added successfully!', 'success');
    } catch (e) {
        showAlert('Failed to ingest document: ' + e.message, 'error');
    } finally {
        // Restore button state
        if (btn) {
            btn.disabled = false;
            btn.innerHTML = originalText;
        }
    }
}

async function deleteRAGDocument(docId) {
    showConfirm('Are you sure you want to delete this document?', async () => {
        try {
            await fetch('/v1/documents/delete', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ id: docId })
            });
            refreshRAGDocuments();
        } catch (e) {
            showAlert('Failed to delete document: ' + e.message, 'error');
        }
    }, { title: 'Delete Document', confirmText: 'Delete', type: 'error' });
}

async function searchRAGDocuments() {
    const query = document.getElementById('ragSearchQuery').value.trim();
    if (!query) {
        showAlert('Please enter a search query', 'warning');
        return;
    }
    
    const resultsDiv = document.getElementById('ragSearchResults');
    resultsDiv.innerHTML = `
        <div class="flex items-center gap-2 text-secondary">
            <svg class="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
            </svg>
            <span>Searching...</span>
        </div>
    `;
    
    try {
        const response = await fetch('/v1/documents/search', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ query, top_k: 5 })
        });
        
        const data = await response.json();
        
        if (!data.results || data.results.length === 0) {
            resultsDiv.innerHTML = `
                <div class="text-center py-4">
                    <svg class="w-8 h-8 mx-auto mb-2 text-secondary" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"/>
                    </svg>
                    <p class="text-secondary text-sm">No results found</p>
                    <p class="text-xs text-secondary mt-1">Try different keywords</p>
                </div>
            `;
            return;
        }
        
        resultsDiv.innerHTML = data.results.map((r, i) => {
            // Highlight query terms in the content
            const content = r.chunk.content.substring(0, 350);
            const highlighted = highlightTerms(content, query);
            const scorePercent = (r.score * 100).toFixed(0);
            const scoreColor = scorePercent >= 70 ? 'text-green-400' : scorePercent >= 40 ? 'text-yellow-400' : 'text-secondary';
            
            return `
                <div class="p-3 bg-tertiary rounded-lg border border-transparent hover:border-accent/30 transition-colors">
                    <div class="flex items-center justify-between mb-2">
                        <div class="flex items-center gap-2">
                            <span class="text-xs font-mono px-1.5 py-0.5 rounded bg-accent/20 text-accent">#${i + 1}</span>
                            <span class="text-sm font-medium truncate">${r.document_name}</span>
                        </div>
                        <div class="flex items-center gap-1">
                            <div class="w-12 h-1.5 bg-tertiary rounded-full overflow-hidden">
                                <div class="h-full bg-accent rounded-full" style="width: ${scorePercent}%"></div>
                            </div>
                            <span class="text-xs ${scoreColor}">${scorePercent}%</span>
                        </div>
                    </div>
                    <p class="text-sm text-secondary leading-relaxed">${highlighted}${r.chunk.content.length > 350 ? '...' : ''}</p>
                </div>
            `;
        }).join('');
    } catch (e) {
        resultsDiv.innerHTML = `
            <div class="text-center py-4">
                <svg class="w-8 h-8 mx-auto mb-2 text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/>
                </svg>
                <p class="text-red-400 text-sm">Search failed</p>
                <p class="text-xs text-secondary mt-1">${e.message}</p>
            </div>
        `;
    }
}

// Highlight search terms in text
function highlightTerms(text, query) {
    const terms = query.toLowerCase().split(/\s+/).filter(t => t.length > 2);
    let result = escapeHtml(text);
    
    terms.forEach(term => {
        const regex = new RegExp(`(${escapeRegex(term)})`, 'gi');
        result = result.replace(regex, '<mark class="bg-accent/30 text-accent px-0.5 rounded">$1</mark>');
    });
    
    return result;
}

// Escape HTML special characters
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Escape regex special characters
function escapeRegex(string) {
    return string.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

// Drag & Drop handlers for RAG
function handleRAGDragOver(event) {
    event.preventDefault();
    event.currentTarget.classList.add('border-accent');
}

function handleRAGDragLeave(event) {
    event.currentTarget.classList.remove('border-accent');
}

async function handleRAGDrop(event) {
    event.preventDefault();
    event.currentTarget.classList.remove('border-accent');
    
    const files = event.dataTransfer.files;
    await uploadRAGFiles(files);
}

async function handleRAGFileSelect(event) {
    const files = event.target.files;
    await uploadRAGFiles(files);
    event.target.value = ''; // Reset for next upload
}

async function uploadRAGFiles(files) {
    if (!ragEnabled) {
        showAlert('Please enable RAG first by selecting an embedding model', 'warning');
        return;
    }
    
    const fileArray = Array.from(files);
    const total = fileArray.length;
    let completed = 0;
    let failed = 0;
    
    // Show upload progress UI
    const dropZone = document.getElementById('ragDropZone');
    const originalContent = dropZone.innerHTML;
    
    function updateProgress() {
        const percent = Math.round((completed / total) * 100);
        dropZone.innerHTML = `
            <div class="text-center py-4">
                <svg class="w-8 h-8 mx-auto mb-2 text-accent animate-spin" fill="none" viewBox="0 0 24 24">
                    <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                    <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
                </svg>
                <p class="text-sm font-medium mb-1">Processing ${completed + 1} of ${total}...</p>
                <div class="w-48 mx-auto bg-tertiary rounded-full h-2 mb-2">
                    <div class="bg-accent h-2 rounded-full transition-all" style="width: ${percent}%"></div>
                </div>
                <p class="text-xs text-secondary">${fileArray[completed]?.name || ''}</p>
            </div>
        `;
    }
    
    updateProgress();
    
    for (const file of fileArray) {
        const formData = new FormData();
        formData.append('file', file);
        
        try {
            const response = await fetch('/v1/documents/ingest', {
                method: 'POST',
                body: formData
            });
            
            if (!response.ok) {
                const err = await response.json();
                throw new Error(err.error || 'Failed to upload');
            }
        } catch (e) {
            failed++;
            console.error(`Failed to upload ${file.name}:`, e.message);
        }
        
        completed++;
        if (completed < total) updateProgress();
    }
    
    // Restore drop zone
    dropZone.innerHTML = originalContent;
    
    // Show result message
    if (failed === 0) {
        showAlert(`Successfully uploaded ${total} file${total > 1 ? 's' : ''}!`, 'success');
    } else if (failed < total) {
        showAlert(`Uploaded ${total - failed} files, ${failed} failed`, 'warning');
    } else {
        showAlert('All uploads failed. Check file formats.', 'error');
    }
    
    refreshRAGDocuments();
}

