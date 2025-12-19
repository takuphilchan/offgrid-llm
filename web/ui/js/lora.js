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
    try {
        const resp = await fetch('/v1/models');
        const data = await resp.json();
        const select = document.getElementById('loraBaseModel');
        select.innerHTML = '<option value="">Select model...</option>';
        data.data.forEach(m => {
            const opt = document.createElement('option');
            opt.value = m.id;
            opt.textContent = m.id;
            select.appendChild(opt);
        });
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
    
    try {
        const resp = await fetch('/v1/lora/adapters', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name, path })
        });
        
        if (!resp.ok) throw new Error('Failed to register adapter');
        
        hideRegisterLoRAModal();
        loadLoRAAdapters();
    } catch (e) {
        showAlert('Failed to register adapter: ' + e.message, { title: 'Error', type: 'error' });
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

