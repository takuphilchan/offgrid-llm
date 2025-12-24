// ============================================
// LORA FUNCTIONS
// ============================================

async function loadLoRAAdapters() {
    try {
        const resp = await fetch('/v1/lora/adapters');
        const data = await resp.json();
        const list = document.getElementById('loraAdaptersList');
        const adapters = data.adapters || [];
        
        if (adapters.length === 0) {
            list.innerHTML = `
                <div class="text-center py-8 text-secondary">
                    <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1" class="mx-auto mb-3 opacity-50"><path d="M12 2L2 7l10 5 10-5-10-5z"/><path d="M2 17l10 5 10-5"/><path d="M2 12l10 5 10-5"/></svg>
                    <p>No LoRA adapters registered</p>
                    <p class="text-xs mt-1">Register an adapter to apply fine-tuned weights</p>
                </div>
            `;
            return;
        }
        
        list.innerHTML = adapters.map(a => `
            <div class="flex items-center justify-between p-3 bg-tertiary rounded-lg">
                <div>
                    <div class="font-medium">${a.name}</div>
                    <div class="text-xs text-secondary mt-1">${a.path}</div>
                </div>
                <div class="flex items-center gap-2">
                    <span class="text-sm text-secondary">Scale: ${a.scale || 1.0}</span>
                    <button onclick="removeLoRA('${a.id}')" class="btn btn-danger btn-sm">Remove</button>
                </div>
            </div>
        `).join('');
        
        // Update adapter select
        const select = document.getElementById('loraAdapterSelect');
        select.innerHTML = '<option value="">Select adapter...</option>' + 
            adapters.map(a => `<option value="${a.id}">${a.name}</option>`).join('');
            
    } catch (e) {
        console.error('Failed to load LoRA adapters:', e);
    }
}

async function loadLoRAModels() {
    // Use ModelManager if available (no redundant API calls)
    if (typeof ModelManager !== 'undefined') {
        await ModelManager.populateLLMSelect('loraBaseModel', handleLoraModelChange);
        return;
    }
    
    // Legacy fallback
    try {
        const resp = await fetch('/v1/models');
        const data = await resp.json();
        const select = document.getElementById('loraBaseModel');
        select.innerHTML = '';
        
        const models = (data.data || []).filter(m => 
            !m.id.toLowerCase().includes('embed') && 
            !m.id.toLowerCase().includes('minilm') &&
            !m.id.toLowerCase().includes('bge')
        );
        
        if (models.length === 0) {
            select.innerHTML = '<option value="">No models available</option>';
            return;
        }
        
        models.forEach((m) => {
            const opt = document.createElement('option');
            opt.value = m.id;
            opt.textContent = m.id;
            select.appendChild(opt);
        });
        
        if (typeof currentModel !== 'undefined' && currentModel) {
            const option = Array.from(select.options).find(opt => opt.value === currentModel);
            if (option) {
                select.value = currentModel;
            }
        } else if (models.length > 0) {
            select.value = models[0].id;
        }
    } catch (e) {
        console.error('Failed to load models for LoRA:', e);
    }
}

function updateLoraScaleLabel() {
    const val = document.getElementById('loraScale').value;
    document.getElementById('loraScaleLabel').textContent = parseFloat(val).toFixed(1);
}

function showRegisterLoRAModal() {
    document.getElementById('registerLoRAModal').classList.add('active');
}

function hideRegisterLoRAModal() {
    document.getElementById('registerLoRAModal').classList.remove('active');
}

async function registerLoRA() {
    const name = document.getElementById('loraName').value;
    const path = document.getElementById('loraPath').value;
    
    if (!name || !path) {
        showAlert('Please fill in all fields', { title: 'Missing Information', type: 'warning' });
        return;
    }
    
    // Show loading state on button
    const btn = document.querySelector('[onclick="registerLoRA()"]');
    const originalText = btn ? btn.innerHTML : '';
    if (btn) {
        btn.disabled = true;
        btn.innerHTML = '<svg class="w-4 h-4 mr-1 animate-spin inline" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path></svg>Registering...';
    }
    
    try {
        const resp = await fetch('/v1/lora/adapters', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name, path })
        });
        
        if (!resp.ok) {
            const err = await resp.json().catch(() => ({}));
            throw new Error(err.error || 'Failed to register adapter');
        }
        
        hideRegisterLoRAModal();
        loadLoRAAdapters();
        showAlert('LoRA adapter registered successfully!', { title: 'Success', type: 'success' });
    } catch (e) {
        showAlert('Failed to register adapter: ' + e.message, { title: 'Error', type: 'error' });
    } finally {
        // Restore button state
        if (btn) {
            btn.disabled = false;
            btn.innerHTML = originalText;
        }
    }
}

async function removeLoRA(id) {
    showConfirm('Are you sure you want to remove this adapter?', async () => {
        try {
            const resp = await fetch(`/v1/lora/adapters/${id}`, { method: 'DELETE' });
            if (!resp.ok) throw new Error('Failed to remove adapter');
            loadLoRAAdapters();
            showAlert('Adapter removed successfully', { title: 'Removed', type: 'success' });
        } catch (e) {
            showAlert('Failed to remove adapter: ' + e.message, { title: 'Error', type: 'error' });
        }
    }, { title: 'Remove Adapter?', type: 'warning', confirmText: 'Remove', cancelText: 'Cancel' });
}

async function loadLoraAdapter() {
    const model = document.getElementById('loraBaseModel').value;
    const adapter = document.getElementById('loraAdapterSelect').value;
    const scale = parseFloat(document.getElementById('loraScale').value);
    
    if (!model || !adapter) {
        showAlert('Please select a model and adapter', { title: 'Missing Selection', type: 'warning' });
        return;
    }
    
    try {
        const resp = await fetch('/v1/lora/load', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ model, adapter_id: adapter, scale })
        });
        
        if (!resp.ok) throw new Error('Failed to load adapter');
        
        const data = await resp.json();
        document.getElementById('activeLoraInfo').innerHTML = `
            <div class="text-left">
                <div class="text-sm"><span class="text-secondary">Model:</span> ${model}</div>
                <div class="text-sm"><span class="text-secondary">Adapter:</span> ${data.adapter_name || adapter}</div>
                <div class="text-sm"><span class="text-secondary">Scale:</span> ${scale}</div>
            </div>
        `;
    } catch (e) {
        showAlert('Failed to load adapter: ' + e.message, { title: 'Error', type: 'error' });
    }
}

