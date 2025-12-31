// =====================================================
// ONBOARDING & KEYBOARD SHORTCUTS
// Improves first-time user experience and power user efficiency
// =====================================================

// =====================================================
// ONBOARDING WIZARD
// =====================================================

const ONBOARDING_VERSION = 1; // Increment to show wizard again after major updates

// Quick Start recommended models for different hardware profiles
const quickStartModels = [
    {
        id: 'Qwen2.5-3B-Instruct-GGUF',
        name: 'Qwen 2.5 3B',
        description: 'Fast & capable, great for most tasks',
        size: '2.0 GB',
        ramRequired: '4 GB',
        tags: ['recommended', 'fast'],
        downloadCmd: 'offgrid download Qwen/Qwen2.5-3B-Instruct-GGUF --quant Q4_K_M'
    },
    {
        id: 'Llama-3.2-3B-Instruct-GGUF',
        name: 'Llama 3.2 3B',
        description: 'Meta\'s latest small model, excellent quality',
        size: '2.0 GB',
        ramRequired: '4 GB',
        tags: ['popular', 'balanced'],
        downloadCmd: 'offgrid download bartowski/Llama-3.2-3B-Instruct-GGUF --quant Q4_K_M'
    },
    {
        id: 'Qwen2.5-7B-Instruct-GGUF',
        name: 'Qwen 2.5 7B',
        description: 'High quality reasoning & coding',
        size: '4.7 GB',
        ramRequired: '8 GB',
        tags: ['quality', 'coding'],
        downloadCmd: 'offgrid download Qwen/Qwen2.5-7B-Instruct-GGUF --quant Q4_K_M'
    },
    {
        id: 'Mistral-7B-Instruct-GGUF',
        name: 'Mistral 7B',
        description: 'Versatile model, great all-rounder',
        size: '4.4 GB',
        ramRequired: '8 GB',
        tags: ['versatile'],
        downloadCmd: 'offgrid download TheBloke/Mistral-7B-Instruct-v0.2-GGUF --quant Q4_K_M'
    },
    {
        id: 'nomic-embed-text-v1.5-GGUF',
        name: 'Nomic Embed',
        description: 'For document search (Knowledge Base)',
        size: '0.3 GB',
        ramRequired: '2 GB',
        tags: ['embedding', 'rag'],
        downloadCmd: 'offgrid download nomic-ai/nomic-embed-text-v1.5-GGUF --quant Q8_0'
    },
    {
        id: 'DeepSeek-R1-Distill-Qwen-7B-GGUF',
        name: 'DeepSeek R1 7B',
        description: 'Advanced reasoning capabilities',
        size: '4.7 GB',
        ramRequired: '8 GB',
        tags: ['reasoning', 'new'],
        downloadCmd: 'offgrid download bartowski/DeepSeek-R1-Distill-Qwen-7B-GGUF --quant Q4_K_M'
    }
];

// Check if onboarding should be shown
function shouldShowOnboarding() {
    const completed = localStorage.getItem('offgrid_onboarding_completed');
    const version = parseInt(localStorage.getItem('offgrid_onboarding_version') || '0');
    return !completed || version < ONBOARDING_VERSION;
}

// Mark onboarding as completed
function completeOnboarding() {
    localStorage.setItem('offgrid_onboarding_completed', 'true');
    localStorage.setItem('offgrid_onboarding_version', ONBOARDING_VERSION.toString());
}

// Show onboarding wizard
function showOnboardingWizard() {
    const modal = document.getElementById('onboardingModal');
    if (modal) {
        modal.classList.add('active');
        showOnboardingStep(1);
    }
}

// Hide onboarding wizard
function hideOnboardingWizard() {
    const modal = document.getElementById('onboardingModal');
    if (modal) {
        modal.classList.remove('active');
    }
}

// Current onboarding step
let currentOnboardingStep = 1;
const totalOnboardingSteps = 3;

// Show specific onboarding step
function showOnboardingStep(step) {
    currentOnboardingStep = step;
    
    // Hide all steps
    document.querySelectorAll('.onboarding-step').forEach(el => el.classList.add('hidden'));
    
    // Show current step
    const stepEl = document.getElementById(`onboarding-step-${step}`);
    if (stepEl) {
        stepEl.classList.remove('hidden');
    }
    
    // Update progress dots
    document.querySelectorAll('.onboarding-dot').forEach((dot, index) => {
        dot.classList.toggle('active', index < step);
    });
    
    // Update buttons
    const prevBtn = document.getElementById('onboardingPrevBtn');
    const nextBtn = document.getElementById('onboardingNextBtn');
    const finishBtn = document.getElementById('onboardingFinishBtn');
    
    if (prevBtn) prevBtn.classList.toggle('hidden', step === 1);
    if (nextBtn) nextBtn.classList.toggle('hidden', step === totalOnboardingSteps);
    if (finishBtn) finishBtn.classList.toggle('hidden', step !== totalOnboardingSteps);
}

// Navigate onboarding
function nextOnboardingStep() {
    if (currentOnboardingStep < totalOnboardingSteps) {
        showOnboardingStep(currentOnboardingStep + 1);
    }
}

function prevOnboardingStep() {
    if (currentOnboardingStep > 1) {
        showOnboardingStep(currentOnboardingStep - 1);
    }
}

function finishOnboarding() {
    completeOnboarding();
    hideOnboardingWizard();
    
    // If no models installed, go to models tab
    const select = document.getElementById('chatModel');
    if (!select || !select.value || select.value === '') {
        switchTab('models');
    }
}

// Skip onboarding
function skipOnboarding() {
    completeOnboarding();
    hideOnboardingWizard();
}

// =====================================================
// DOWNLOAD MODAL
// =====================================================

let currentDownloadAbort = null;
let downloadStartTime = null;

// Show download modal
function showDownloadModal(modelName, size) {
    const modal = document.getElementById('downloadModal');
    document.getElementById('downloadModelName').textContent = modelName;
    document.getElementById('downloadModelSize').textContent = `Size: ${size}`;
    document.getElementById('downloadProgressBar').style.width = '0%';
    document.getElementById('downloadProgressText').textContent = '0%';
    document.getElementById('downloadSpeedText').textContent = '-- MB/s';
    document.getElementById('downloadDownloaded').textContent = '0 MB';
    document.getElementById('downloadTotal').textContent = size;
    document.getElementById('downloadETA').textContent = 'ETA: calculating...';
    document.getElementById('downloadCancelBtn').classList.remove('hidden');
    document.getElementById('downloadDoneBtn').classList.add('hidden');
    document.getElementById('downloadStatus').classList.add('hidden');
    modal.classList.add('active');
    downloadStartTime = Date.now();
}

// Hide download modal
function hideDownloadModal() {
    const modal = document.getElementById('downloadModal');
    modal.classList.remove('active');
    currentDownloadAbort = null;
}

// Cancel download
async function cancelDownload() {
    // Call server to cancel the download
    try {
        await fetch('/v1/models/download/cancel', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({}) // Cancel any active download
        });
    } catch (e) {
        console.warn('Failed to cancel download on server:', e);
    }
    
    // Also abort client-side polling
    if (currentDownloadAbort) {
        currentDownloadAbort.abort();
    }
    hideDownloadModal();
    showAlert('Download cancelled', { title: 'Cancelled', type: 'info' });
}

// Update download progress
function updateDownloadProgress(downloaded, total, speed) {
    const percent = total > 0 ? (downloaded / total * 100).toFixed(1) : 0;
    document.getElementById('downloadProgressBar').style.width = `${percent}%`;
    document.getElementById('downloadProgressText').textContent = `${percent}%`;
    document.getElementById('downloadDownloaded').textContent = formatBytes(downloaded);
    document.getElementById('downloadTotal').textContent = formatBytes(total);
    
    if (speed > 0) {
        document.getElementById('downloadSpeedText').textContent = `${(speed / 1024 / 1024).toFixed(1)} MB/s`;
        const remaining = (total - downloaded) / speed;
        if (remaining < 60) {
            document.getElementById('downloadETA').textContent = `ETA: ${Math.ceil(remaining)}s`;
        } else if (remaining < 3600) {
            document.getElementById('downloadETA').textContent = `ETA: ${Math.floor(remaining / 60)}m ${Math.ceil(remaining % 60)}s`;
        } else {
            document.getElementById('downloadETA').textContent = `ETA: ${Math.floor(remaining / 3600)}h ${Math.floor((remaining % 3600) / 60)}m`;
        }
    }
}

// Show download complete
function showDownloadComplete() {
    document.getElementById('downloadProgressBar').style.width = '100%';
    document.getElementById('downloadProgressText').textContent = '100%';
    document.getElementById('downloadSpeedText').textContent = 'Complete';
    document.getElementById('downloadETA').textContent = '';
    document.getElementById('downloadCancelBtn').classList.add('hidden');
    document.getElementById('downloadDoneBtn').classList.remove('hidden');
    document.getElementById('downloadModalTitle').textContent = 'Download Complete';
    
    // Refresh models list
    setTimeout(() => {
        if (typeof loadInstalledModels === 'function') loadInstalledModels();
        if (typeof loadChatModels === 'function') loadChatModels();
    }, 1000);
}

// Show download error
function showDownloadError(message) {
    document.getElementById('downloadStatus').classList.remove('hidden');
    document.getElementById('downloadStatusText').textContent = message;
    document.getElementById('downloadStatus').querySelector('svg').classList.add('hidden');
    document.getElementById('downloadCancelBtn').textContent = 'Close';
    document.getElementById('downloadModalTitle').textContent = 'Download Failed';
}

// Format bytes helper
function formatBytes(bytes) {
    if (bytes < 1024) return bytes + ' B';
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
    if (bytes < 1024 * 1024 * 1024) return (bytes / 1024 / 1024).toFixed(1) + ' MB';
    return (bytes / 1024 / 1024 / 1024).toFixed(2) + ' GB';
}

// Quick install model from onboarding - uses API with progress polling
async function quickInstallModel(downloadCmd, buttonEl) {
    // Parse model info from command
    // Format: offgrid download <repo> --quant <quant>
    const parts = downloadCmd.split(' ');
    const repoIndex = parts.indexOf('download') + 1;
    const repository = parts[repoIndex] || '';
    const quantIndex = parts.indexOf('--quant');
    const quantization = quantIndex >= 0 ? parts[quantIndex + 1] : 'Q4_K_M';
    const modelName = repository.split('/').pop() || repository;
    
    // Find model info from quickStartModels
    const modelInfo = quickStartModels.find(m => downloadCmd.includes(m.downloadCmd.split(' ')[2]));
    const size = modelInfo ? modelInfo.size : 'Unknown';
    
    // Update button state
    if (buttonEl) {
        buttonEl.disabled = true;
        buttonEl.innerHTML = `
            <svg class="w-4 h-4 animate-spin" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <circle cx="12" cy="12" r="10" stroke-opacity="0.25"/>
                <path d="M12 2a10 10 0 0 1 10 10" stroke-linecap="round"/>
            </svg>
            Starting...
        `;
    }
    
    // Show download modal
    hideOnboardingWizard();
    completeOnboarding();
    showDownloadModal(modelName, size);
    
    currentDownloadAbort = new AbortController();
    let downloadFileName = null;
    
    try {
        // Start download via API
        const startResp = await fetch('/v1/models/download', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                repository: repository,
                quantization: quantization
            })
        });

        if (!startResp.ok) {
            const err = await startResp.json().catch(() => ({}));
            showDownloadError(err.error || `Failed to start download: HTTP ${startResp.status}`);
            return;
        }

        const startData = await startResp.json();
        
        // Check if model already exists
        if (startData.exists) {
            hideDownloadModal();
            showModal({
                type: 'info',
                title: 'Model Already Installed',
                message: `"${startData.file_name || modelName}" is already installed.\n\nYou can use it right away from the Chat tab.`,
                confirmText: 'OK'
            });
            return;
        }
        
        if (!startData.success) {
            showDownloadError(startData.message || 'Failed to start download');
            return;
        }

        // Poll for progress
        let completed = false;
        let lastPercent = 0;
        
        while (!completed && !currentDownloadAbort.signal.aborted) {
            await new Promise(r => setTimeout(r, 300)); // Poll every 300ms
            
            try {
                const progressResp = await fetch('/v1/models/download/progress');
                if (!progressResp.ok) continue;
                
                const progress = await progressResp.json();
                
                // Find active download (most recent one that's downloading)
                for (const [filename, p] of Object.entries(progress)) {
                    if (p.status === 'downloading') {
                        downloadFileName = filename;
                        if (p.bytes_total > 0) {
                            updateDownloadProgress(p.bytes_done, p.bytes_total, p.speed || 0);
                            lastPercent = p.percent;
                        }
                    } else if (p.status === 'complete' && p.percent >= 99) {
                        completed = true;
                        showDownloadComplete();
                        break;
                    } else if (p.status === 'failed') {
                        showDownloadError(p.error || 'Download failed');
                        completed = true;
                        break;
                    }
                }
            } catch (e) {
                // Ignore polling errors
            }
        }
        
        // If we exited but didn't complete, check one more time
        if (!completed && lastPercent > 95) {
            showDownloadComplete();
        }

    } catch (error) {
        if (error.name !== 'AbortError') {
            showDownloadError(error.message);
        }
    }
    
    // Reset button
    if (buttonEl) {
        buttonEl.disabled = false;
        buttonEl.innerHTML = 'Download';
    }
}

// Parse size string to bytes
function parseSize(value, unit) {
    const num = parseFloat(value);
    switch (unit.toUpperCase()) {
        case 'KB': return num * 1024;
        case 'MB': return num * 1024 * 1024;
        case 'GB': return num * 1024 * 1024 * 1024;
        default: return num;
    }
}

// =====================================================
// KEYBOARD SHORTCUTS
// =====================================================

const keyboardShortcuts = {
    'Ctrl+N': { action: 'newChat', description: 'New chat' },
    'Ctrl+K': { action: 'focusModelSelect', description: 'Quick model switch' },
    'Ctrl+/': { action: 'focusChatInput', description: 'Focus chat input' },
    'Ctrl+,': { action: 'toggleSettings', description: 'Toggle settings' },
    'Ctrl+Shift+S': { action: 'toggleSessions', description: 'Toggle sessions panel' },
    'Ctrl+1': { action: () => switchTab('chat'), description: 'Go to Chat' },
    'Ctrl+2': { action: () => switchTab('models'), description: 'Go to Models' },
    'Ctrl+3': { action: () => switchTab('knowledge'), description: 'Go to Documents' },
    'Escape': { action: 'stopOrClose', description: 'Stop generation / Close modal' },
    '?': { action: 'showShortcutsHelp', description: 'Show keyboard shortcuts', requiresShift: true }
};

// Initialize keyboard shortcuts
function initKeyboardShortcuts() {
    document.addEventListener('keydown', handleKeyboardShortcut);
}

// Handle keyboard shortcut
function handleKeyboardShortcut(e) {
    // Don't trigger shortcuts when typing in inputs
    const activeEl = document.activeElement;
    const isTyping = activeEl && (
        activeEl.tagName === 'INPUT' || 
        activeEl.tagName === 'TEXTAREA' || 
        activeEl.isContentEditable
    );
    
    // Build shortcut key string
    let key = '';
    if (e.ctrlKey || e.metaKey) key += 'Ctrl+';
    if (e.shiftKey) key += 'Shift+';
    if (e.altKey) key += 'Alt+';
    
    // Handle special keys
    if (e.key === 'Escape') {
        key = 'Escape';
    } else if (e.key === '?') {
        key = '?';
    } else if (e.key >= '1' && e.key <= '9') {
        key += e.key;
    } else if (e.key.length === 1) {
        key += e.key.toUpperCase();
    }
    
    const shortcut = keyboardShortcuts[key];
    
    if (!shortcut) return;
    
    // Special handling for shortcuts that should work while typing
    if (key === 'Escape') {
        e.preventDefault();
        executeShortcutAction('stopOrClose');
        return;
    }
    
    // Don't trigger other shortcuts while typing (except Escape)
    if (isTyping && key !== 'Escape') {
        return;
    }
    
    // Check if shift is required
    if (shortcut.requiresShift && !e.shiftKey) return;
    
    e.preventDefault();
    
    if (typeof shortcut.action === 'function') {
        shortcut.action();
    } else {
        executeShortcutAction(shortcut.action);
    }
}

// Execute shortcut action
function executeShortcutAction(action) {
    switch (action) {
        case 'newChat':
            if (typeof newChat === 'function') newChat();
            break;
        case 'focusModelSelect':
            const modelSelect = document.getElementById('chatModel');
            if (modelSelect) {
                modelSelect.focus();
                modelSelect.click();
            }
            break;
        case 'focusChatInput':
            const chatInput = document.getElementById('chatInput');
            if (chatInput) chatInput.focus();
            break;
        case 'toggleSettings':
            if (typeof toggleChatSettings === 'function') toggleChatSettings();
            break;
        case 'toggleSessions':
            if (typeof toggleSessionsPanel === 'function') toggleSessionsPanel();
            break;
        case 'stopOrClose':
            // First try to close any open modal
            const activeModal = document.querySelector('.modal-backdrop.active');
            if (activeModal) {
                activeModal.classList.remove('active');
                return;
            }
            // Then try to stop generation
            if (isGenerating && typeof stopGeneration === 'function') {
                stopGeneration();
            }
            break;
        case 'showShortcutsHelp':
            showKeyboardShortcutsModal();
            break;
    }
}

// Show keyboard shortcuts modal
function showKeyboardShortcutsModal() {
    const modal = document.getElementById('shortcutsModal');
    if (modal) {
        modal.classList.add('active');
    }
}

// Hide keyboard shortcuts modal
function hideKeyboardShortcutsModal() {
    const modal = document.getElementById('shortcutsModal');
    if (modal) {
        modal.classList.remove('active');
    }
}

// =====================================================
// QUICK START MODELS SECTION
// =====================================================

// Render quick start models in the models page
function renderQuickStartModels() {
    const container = document.getElementById('quickStartModels');
    if (!container) return;
    
    container.innerHTML = quickStartModels.map(model => {
        const tagsHtml = model.tags.map(tag => {
            const colors = {
                recommended: 'bg-emerald-500/20 text-emerald-400',
                popular: 'bg-blue-500/20 text-blue-400',
                fast: 'bg-amber-500/20 text-amber-400',
                quality: 'bg-purple-500/20 text-purple-400',
                coding: 'bg-cyan-500/20 text-cyan-400',
                embedding: 'bg-pink-500/20 text-pink-400',
                rag: 'bg-orange-500/20 text-orange-400',
                reasoning: 'bg-indigo-500/20 text-indigo-400',
                versatile: 'bg-teal-500/20 text-teal-400',
                new: 'bg-rose-500/20 text-rose-400',
                balanced: 'bg-sky-500/20 text-sky-400'
            };
            return `<span class="text-xs px-1.5 py-0.5 rounded ${colors[tag] || 'bg-gray-500/20 text-gray-400'}">${tag}</span>`;
        }).join('');
        
        return `
            <div class="quick-start-card group" onclick="quickInstallFromModelsPage('${model.downloadCmd.replace(/'/g, "\\'")}', this)">
                <div class="flex justify-between items-start mb-2">
                    <h4 class="font-semibold text-primary group-hover:text-accent transition-colors">${model.name}</h4>
                    <div class="flex gap-1">${tagsHtml}</div>
                </div>
                <p class="text-sm text-secondary mb-3">${model.description}</p>
                <div class="flex items-center justify-between text-xs text-secondary">
                    <span>${model.size}</span>
                    <span>Needs ${model.ramRequired} RAM</span>
                </div>
                <div class="mt-3 opacity-0 group-hover:opacity-100 transition-opacity">
                    <button class="btn btn-primary btn-sm w-full">
                        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"/>
                        </svg>
                        Download
                    </button>
                </div>
            </div>
        `;
    }).join('');
}

// Quick install from models page - uses download modal
async function quickInstallFromModelsPage(downloadCmd, cardEl) {
    // Parse model info from command
    const parts = downloadCmd.split(' ');
    const modelIdIndex = parts.indexOf('download') + 1;
    const modelId = parts[modelIdIndex] || 'Model';
    const modelName = modelId.split('/').pop() || modelId;
    
    // Extract repository and quantization from command
    // Example: offgrid download Qwen/Qwen2.5-3B-Instruct-GGUF --quant Q4_K_M
    const repository = modelId;
    const quantIndex = parts.indexOf('--quant');
    const quantization = quantIndex > 0 ? parts[quantIndex + 1] : 'Q4_K_M';
    
    // Find model info from quickStartModels
    const modelInfo = quickStartModels.find(m => downloadCmd.includes(m.downloadCmd.split(' ')[2]));
    const size = modelInfo ? modelInfo.size : 'Unknown';
    
    // Visual feedback on card
    if (cardEl) {
        cardEl.style.opacity = '0.7';
        cardEl.style.pointerEvents = 'none';
    }
    
    // Show download modal
    showDownloadModal(modelName, size);
    
    currentDownloadAbort = new AbortController();
    
    try {
        // Start download via API
        const startResp = await fetch('/v1/models/download', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                repository: repository,
                quantization: quantization
            })
        });

        if (!startResp.ok) {
            const err = await startResp.json().catch(() => ({}));
            showDownloadError(err.error || `Failed to start download: HTTP ${startResp.status}`);
            if (cardEl) {
                cardEl.style.opacity = '1';
                cardEl.style.pointerEvents = 'auto';
            }
            return;
        }

        const startData = await startResp.json();
        
        // Check if model already exists
        if (startData.exists) {
            hideDownloadModal();
            showModal({
                type: 'info',
                title: 'Model Already Installed',
                message: `"${startData.file_name || modelName}" is already installed.\n\nYou can use it right away from the Chat tab.`,
                confirmText: 'OK'
            });
            if (cardEl) {
                cardEl.style.opacity = '1';
                cardEl.style.pointerEvents = 'auto';
            }
            return;
        }
        
        if (!startData.success) {
            showDownloadError(startData.message || 'Failed to start download');
            if (cardEl) {
                cardEl.style.opacity = '1';
                cardEl.style.pointerEvents = 'auto';
            }
            return;
        }

        // Poll for progress
        let completed = false;
        let lastPercent = 0;
        
        while (!completed && !currentDownloadAbort.signal.aborted) {
            await new Promise(r => setTimeout(r, 300)); // Poll every 300ms
            
            try {
                const progressResp = await fetch('/v1/models/download/progress');
                if (!progressResp.ok) continue;
                
                const progress = await progressResp.json();
                
                // Find active download
                for (const [filename, p] of Object.entries(progress)) {
                    if (p.status === 'downloading') {
                        if (p.bytes_total > 0) {
                            updateDownloadProgress(p.bytes_done, p.bytes_total, p.speed || 0);
                            lastPercent = p.percent;
                        }
                    } else if (p.status === 'complete' && p.percent >= 99) {
                        completed = true;
                        showDownloadComplete();
                        break;
                    } else if (p.status === 'failed') {
                        showDownloadError(p.error || 'Download failed');
                        completed = true;
                        break;
                    }
                }
            } catch (e) {
                // Ignore polling errors
            }
        }
        
        // If we exited but didn't complete, check one more time
        if (!completed && lastPercent > 95) {
            showDownloadComplete();
        }

    } catch (error) {
        if (error.name !== 'AbortError') {
            showDownloadError(error.message);
        }
    }
    
    // Reset card
    if (cardEl) {
        cardEl.style.opacity = '1';
        cardEl.style.pointerEvents = 'auto';
    }
}

// =====================================================
// MODEL SIZE IN DROPDOWN
// =====================================================

// Format file size
function formatModelSize(bytes) {
    if (!bytes) return '';
    const gb = bytes / (1024 * 1024 * 1024);
    if (gb >= 1) {
        return `${gb.toFixed(1)} GB`;
    }
    const mb = bytes / (1024 * 1024);
    return `${mb.toFixed(0)} MB`;
}

// =====================================================
// INITIALIZATION
// =====================================================

// Initialize onboarding and shortcuts on page load
document.addEventListener('DOMContentLoaded', function() {
    // Initialize keyboard shortcuts
    initKeyboardShortcuts();
    
    // Render quick start models
    renderQuickStartModels();
    
    // Check if onboarding should be shown (delay to let page load)
    setTimeout(() => {
        if (shouldShowOnboarding()) {
            showOnboardingWizard();
        }
    }, 500);
});
