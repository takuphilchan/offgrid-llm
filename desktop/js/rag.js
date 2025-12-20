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
        
        if (data.enabled) {
            badge.className = 'badge badge-success';
            badge.textContent = 'Enabled';
            disableBtn.classList.remove('hidden');
            // Set the current model in the dropdown
            if (data.embedding_model) {
                modelSelect.value = data.embedding_model;
            }
        } else {
            badge.className = 'badge badge-secondary';
            badge.textContent = 'Disabled';
            disableBtn.classList.add('hidden');
        }
        
        // Update stats
        if (data.stats) {
            document.getElementById('ragDocCount').textContent = data.stats.document_count || 0;
            document.getElementById('ragChunkCount').textContent = data.stats.chunk_count || 0;
            document.getElementById('ragEmbeddingCount').textContent = data.stats.embedding_count || 0;
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
        const currentModel = status.embedding_model || '';
        const isEnabled = status.enabled;
        
        const response = await fetch('/v1/models');
        const data = await response.json();
        const select = document.getElementById('ragEmbeddingModel');
        select.innerHTML = '';
        
        const embeddingModels = data.data.filter(m => 
            m.id.toLowerCase().includes('embed') || 
            m.id.toLowerCase().includes('minilm') ||
            m.id.toLowerCase().includes('bge') ||
            m.id.toLowerCase().includes('nomic')
        );
        
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
        if (isEnabled && currentModel) {
            select.value = currentModel;
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
            list.innerHTML = '<p class="text-secondary text-sm">No documents uploaded yet.</p>';
            return;
        }
        
        list.innerHTML = data.documents.map(doc => `
            <div class="flex items-center justify-between p-3 bg-tertiary rounded-lg">
                <div class="flex items-center gap-3">
                    <svg class="w-5 h-5 text-accent" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"></path>
                    </svg>
                    <div>
                        <div class="font-medium">${doc.name}</div>
                        <div class="text-xs text-secondary">${doc.chunk_count} chunks • ${formatBytes(doc.size)}</div>
                    </div>
                </div>
                <button onclick="deleteRAGDocument('${doc.id}')" class="btn btn-secondary btn-sm text-red-400 hover:text-red-300">
                    Delete
                </button>
            </div>
        `).join('');
        
        loadRAGStatus(); // Refresh stats
    } catch (e) {
        console.error('Failed to load documents:', e);
    }
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
    resultsDiv.innerHTML = '<p class="text-secondary">Searching...</p>';
    
    try {
        const response = await fetch('/v1/documents/search', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ query, top_k: 5 })
        });
        
        const data = await response.json();
        
        if (!data.results || data.results.length === 0) {
            resultsDiv.innerHTML = '<p class="text-secondary">No results found.</p>';
            return;
        }
        
        resultsDiv.innerHTML = data.results.map((r, i) => `
            <div class="p-3 bg-tertiary rounded-lg">
                <div class="flex items-center justify-between mb-2">
                    <span class="text-sm font-medium">${r.document_name}</span>
                    <span class="text-xs text-accent">Score: ${(r.score * 100).toFixed(1)}%</span>
                </div>
                <p class="text-sm text-secondary">${r.chunk.content.substring(0, 300)}${r.chunk.content.length > 300 ? '...' : ''}</p>
            </div>
        `).join('');
    } catch (e) {
        resultsDiv.innerHTML = `<p class="text-red-400">Search failed: ${e.message}</p>`;
    }
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
    
    for (const file of files) {
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
            showAlert(`Failed to upload ${file.name}: ${e.message}`, 'error');
        }
    }
    
    refreshRAGDocuments();
}

