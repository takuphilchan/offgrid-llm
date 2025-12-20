// Update empty state based on whether a model is selected
function updateEmptyState() {
    const noModelState = document.getElementById('noModelState');
    const modelReadyState = document.getElementById('modelReadyState');
    const examplePrompts = document.getElementById('examplePrompts');
    
    if (!noModelState || !modelReadyState || !examplePrompts) return;
    
    const hasModel = currentModel && currentModel.length > 0;
    
    if (hasModel) {
        noModelState.classList.add('hidden');
        modelReadyState.classList.remove('hidden');
        examplePrompts.classList.remove('hidden');
    } else {
        noModelState.classList.remove('hidden');
        modelReadyState.classList.add('hidden');
        examplePrompts.classList.add('hidden');
    }
}

function newChat() {
    if (messages.length === 0) {
        // No messages, just reset
        resetChatUI();
        return;
    }
    
    // Show custom 3-button dialog
    const overlay = document.createElement('div');
    overlay.className = 'modal-overlay';
    const msgCount = messages.length;
    
    overlay.innerHTML = `
        <div class="modal-dialog">
            <div class="modal-dialog-header">
                <div class="modal-dialog-icon warning">
                    <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"></path>
                    </svg>
                </div>
                <div class="modal-dialog-title">Start New Chat?</div>
            </div>
            <p class="modal-dialog-message">You have <strong>${msgCount} messages</strong> in the current conversation.</p>
            <div class="modal-dialog-actions">
                <button class="btn btn-secondary" data-action="cancel">Cancel</button>
                <button class="btn btn-primary" data-action="save-new">Save & New</button>
                <button class="btn btn-secondary" data-action="new">New Chat</button>
            </div>
        </div>
    `;
    
    document.body.appendChild(overlay);
    
    // Handle clicks
    overlay.addEventListener('click', (e) => {
        if (e.target.classList.contains('modal-overlay')) {
            overlay.remove();
        }
    });
    
    overlay.querySelector('[data-action="save-new"]')?.addEventListener('click', () => {
        overlay.remove();
        // Save with callback to reset after save completes
        saveSessionAndThen(() => {
            resetChatUI();
        });
    });
    
    overlay.querySelector('[data-action="new"]')?.addEventListener('click', () => {
        overlay.remove();
        resetChatUI();
    });
    
    overlay.querySelector('[data-action="cancel"]')?.addEventListener('click', () => {
        overlay.remove();
    });
}

// Save session with a callback after completion
function saveSessionAndThen(onComplete) {
    if (messages.length === 0) {
        if (onComplete) onComplete();
        return;
    }
    
    // Copy messages before any reset happens
    const messagesToSave = [...messages];
    const modelToSave = currentModel;
    const systemPromptToSave = currentSystemPrompt;
    const sessionIdToSave = currentSessionId;
    
    showPrompt({
        title: 'Save Session',
        message: 'Enter a title for this session:',
        defaultValue: `Chat ${new Date().toLocaleDateString()}`,
        confirmText: 'Save',
        onConfirm: (title) => {
            if (!title) {
                if (onComplete) onComplete();
                return;
            }
            
            const session = {
                id: sessionIdToSave || Date.now(),
                title: title,
                model: modelToSave,
                messages: messagesToSave,
                systemPrompt: systemPromptToSave,
                timestamp: new Date().toISOString(),
                messageCount: messagesToSave.length
            };
            
            const existingIndex = sessions.findIndex(s => s.id === session.id);
            if (existingIndex >= 0) {
                sessions[existingIndex] = session;
            } else {
                sessions.unshift(session);
            }
            
            localStorage.setItem('offgrid_sessions', JSON.stringify(sessions));
            renderSessions();
            
            if (onComplete) onComplete();
        },
        onCancel: () => {
            // User cancelled save, don't proceed with new chat
        }
    });
}

// Reset chat UI to initial state
function resetChatUI() {
    messages = [];
    currentSessionId = null;
    const chatMessages = document.getElementById('chatMessages');
    chatMessages.innerHTML = `
        <div id="emptyStatePrompts" class="flex flex-col items-center justify-center h-full max-w-2xl mx-auto px-4">
            <div class="text-center mb-8">
                <h2 class="text-2xl font-semibold text-primary mb-2">What can I help you with?</h2>
                <p class="text-sm text-secondary">Select a model above and try one of these prompts</p>
            </div>
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 w-full">
                <button onclick="useExamplePrompt('Explain how neural networks work in simple terms')" class="example-prompt-card">
                    <div class="card-title">Explain a concept</div>
                    <div class="card-desc">"Explain how neural networks work in simple terms"</div>
                </button>
                <button onclick="useExamplePrompt('Write a Python function that sorts a list using quicksort')" class="example-prompt-card">
                    <div class="card-title">Write code</div>
                    <div class="card-desc">"Write a Python function that sorts a list using quicksort"</div>
                </button>
                <button onclick="useExamplePrompt('Help me brainstorm ideas for a mobile app that helps people learn new languages')" class="example-prompt-card">
                    <div class="card-title">Brainstorm ideas</div>
                    <div class="card-desc">"Help me brainstorm ideas for a mobile app..."</div>
                </button>
                <button onclick="useExamplePrompt('Summarize the key differences between REST and GraphQL APIs')" class="example-prompt-card">
                    <div class="card-title">Compare topics</div>
                    <div class="card-desc">"Summarize the key differences between REST and GraphQL"</div>
                </button>
            </div>
        </div>
    `;
    document.getElementById('chatInput').value = '';
    updateChatStats();
    saveMessages();
}

// Clear chat (legacy, now redirects to newChat)
function clearChat() {
    newChat();
}

// Export chat with enhanced options
function showExportOptions() {
    if (messages.length === 0) {
        showModal({
            type: 'warning',
            title: 'No Messages',
            message: 'There are no messages to export yet.',
            confirmText: 'OK',
            cancelText: null
        });
        return;
    }
    
    const overlay = document.createElement('div');
    overlay.className = 'modal-overlay';
    
    overlay.innerHTML = `
        <div class="modal-dialog">
            <h3 class="text-lg font-bold mb-4 text-center" style="color: var(--text-primary)">Export Chat</h3>
            <p class="text-center mb-6" style="color: var(--text-secondary)">Choose a format for your export:</p>
            <div class="flex flex-col gap-3">
                <button class="btn btn-secondary w-full justify-start p-4" data-format="1">
                    <div class="text-left">
                        <div class="font-bold text-base mb-1" style="color: var(--text-primary)">Markdown</div>
                        <div class="text-xs opacity-70" style="color: var(--text-secondary)">Best for research & reading</div>
                    </div>
                </button>
                <button class="btn btn-secondary w-full justify-start p-4" data-format="2">
                    <div class="text-left">
                        <div class="font-bold text-base mb-1" style="color: var(--text-primary)">Plain Text</div>
                        <div class="text-xs opacity-70" style="color: var(--text-secondary)">Simple text file</div>
                    </div>
                </button>
                <button class="btn btn-secondary w-full justify-start p-4" data-format="3">
                    <div class="text-left">
                        <div class="font-bold text-base mb-1" style="color: var(--text-primary)">JSON</div>
                        <div class="text-xs opacity-70" style="color: var(--text-secondary)">For programmatic use</div>
                    </div>
                </button>
            </div>
            <div class="flex justify-center mt-6">
                <button class="btn btn-secondary" data-action="cancel">Cancel</button>
            </div>
        </div>
    `;
    
    document.body.appendChild(overlay);
    
    const handleExport = (format) => {
        overlay.remove();
        processExport(format);
    };

    overlay.querySelectorAll('[data-format]').forEach(btn => {
        btn.addEventListener('click', () => handleExport(btn.dataset.format));
    });
    
    overlay.querySelector('[data-action="cancel"]').addEventListener('click', () => {
        overlay.remove();
    });
    
    overlay.addEventListener('click', (e) => {
        if (e.target.classList.contains('modal-overlay')) {
            overlay.remove();
        }
    });
}

function processExport(format) {
    if (!format) return;
    
    let content, mimeType, extension;
    const timestamp = new Date().toISOString().split('T')[0];
    const filename = `chat-${currentModel}-${timestamp}`;
    
    if (format === '1') {
        // Markdown format with code blocks
        content = `# Chat with ${currentModel}\n**Date:** ${new Date().toLocaleString()}\n**Messages:** ${messages.length}\n\n---\n\n`;
        messages.forEach(m => {
            content += `## ${m.role === 'user' ? 'User' : 'Assistant'}\n\n`;
            // Detect and format code blocks
            const hasCode = m.content.includes('```');
            content += hasCode ? m.content : m.content.replace(/`([^`]+)`/g, '`$1`');
            content += '\n\n---\n\n';
        });
        mimeType = 'text/markdown';
        extension = 'md';
    } else if (format === '3') {
        // JSON format for programmatic access
        content = JSON.stringify({
            model: currentModel,
            timestamp: new Date().toISOString(),
            messageCount: messages.length,
            messages: messages
        }, null, 2);
        mimeType = 'application/json';
        extension = 'json';
    } else {
        // Plain text
        content = messages.map(m => `${m.role.toUpperCase()}: ${m.content}`).join('\n\n');
        mimeType = 'text/plain';
        extension = 'txt';
    }
    
    const blob = new Blob([content], { type: mimeType });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `${filename}.${extension}`;
    a.click();
    URL.revokeObjectURL(url);
}

// System prompt functions
function toggleChatSettings() {
    const panel = document.getElementById('chatSettingsPanel');
    panel.classList.toggle('hidden');
    
    // Close when clicking outside
    if (!panel.classList.contains('hidden')) {
        setTimeout(() => {
            document.addEventListener('click', closeChatSettingsOnOutsideClick);
        }, 10);
    }
}

function closeChatSettingsOnOutsideClick(e) {
    const panel = document.getElementById('chatSettingsPanel');
    const button = e.target.closest('button[onclick="toggleChatSettings()"]');
    if (!panel.contains(e.target) && !button) {
        panel.classList.add('hidden');
        document.removeEventListener('click', closeChatSettingsOnOutsideClick);
    }
}

function useExamplePrompt(prompt) {
    const input = document.getElementById('chatInput');
    input.value = prompt;
    input.focus();
    autoResizeTextarea(input);
    // Hide empty state
    const emptyState = document.getElementById('emptyStatePrompts');
    if (emptyState) {
        emptyState.style.display = 'none';
    }
}

function setSystemPrompt(preset) {
    const select = document.getElementById('systemPrompt');
    select.value = preset;
    applySystemPrompt();
    updatePromptIndicator(preset);
}

function updatePromptIndicator(preset) {
    const indicator = document.getElementById('promptIndicator');
    const labels = {
        'research': 'Research Mode',
        'tutor': 'Tutor Mode',
        'coder': 'Code Mode',
        'writer': 'Writer Mode',
        'custom': 'Custom Prompt'
    };
    if (preset && labels[preset]) {
        indicator.textContent = labels[preset];
        indicator.classList.remove('hidden');
    } else {
        indicator.classList.add('hidden');
    }
}

function applySystemPrompt() {
    const select = document.getElementById('systemPrompt');
    const value = select.value;
    
    if (value === 'custom') {
        showPrompt({
            title: 'Custom System Prompt',
            message: 'Enter your custom system prompt:',
            placeholder: 'You are a helpful assistant...',
            confirmText: 'Apply',
            onConfirm: (custom) => {
                if (custom) {
                    currentSystemPrompt = custom;
                    updatePromptIndicator('custom');
                }
            },
            onCancel: () => {
                select.value = '';
                updatePromptIndicator('');
            }
        });
    } else if (value) {
        currentSystemPrompt = systemPrompts[value];
        updatePromptIndicator(value);
    } else {
        currentSystemPrompt = '';
        updatePromptIndicator('');
    }
}

// Session Management Functions
function saveCurrentSession() {
    if (messages.length === 0) {
        showModal({
            type: 'error',
            title: 'Cannot Save',
            message: 'No messages to save',
            confirmText: 'OK'
        });
        return;
    }
    
    showPrompt({
        title: 'Save Session',
        message: 'Enter a title for this session:',
        defaultValue: `Chat ${new Date().toLocaleDateString()}`,
        confirmText: 'Save',
        onConfirm: (title) => {
            if (!title) return;
            
            const session = {
                id: currentSessionId || Date.now(),
                title: title,
                model: currentModel,
                messages: [...messages],
                systemPrompt: currentSystemPrompt,
                timestamp: new Date().toISOString(),
                messageCount: messages.length
            };
            
            const existingIndex = sessions.findIndex(s => s.id === session.id);
            if (existingIndex >= 0) {
                sessions[existingIndex] = session;
            } else {
                sessions.unshift(session);
                currentSessionId = session.id;
            }
            
            localStorage.setItem('offgrid_sessions', JSON.stringify(sessions));
            renderSessions();
            showModal({
                type: 'success',
                title: 'Saved',
                message: 'Session saved successfully',
                confirmText: 'OK'
            });
        }
    });
}

// Toggle sessions panel in chat tab
function toggleSessionsPanel() {
    const panel = document.getElementById('sessionsPanel');
    panel.classList.toggle('hidden');
    if (!panel.classList.contains('hidden')) {
        renderSessions();
    }
}

// Toggle embeddings section in knowledge tab
function toggleEmbeddingsSection() {
    const content = document.getElementById('embeddingsContent');
    const icon = document.getElementById('embeddingsSectionIcon');
    content.classList.toggle('hidden');
    icon.style.transform = content.classList.contains('hidden') ? '' : 'rotate(180deg)';
}

function renderSessions() {
    const container = document.getElementById('sessionsList');
    
    if (sessions.length === 0) {
        container.innerHTML = '<p class="text-xs text-secondary text-center py-4">No saved sessions yet.</p>';
        return;
    }
    
    container.innerHTML = sessions.map(session => `
        <div class="p-2 rounded bg-tertiary hover:bg-white/5 cursor-pointer transition-colors group" data-session-id="${session.id}">
            <div class="flex items-start justify-between gap-2">
                <div class="flex-1 min-w-0" onclick="loadSession(${session.id})">
                    <h4 class="font-medium text-sm text-accent">${session.title}</h4>
                    <p class="text-xs text-secondary mt-0.5">
                        ${session.messageCount} messages • ${new Date(session.timestamp).toLocaleDateString()}
                    </p>
                </div>
                <button onclick="deleteSession(${session.id}); event.stopPropagation();" 
                        class="opacity-0 group-hover:opacity-100 p-1 hover:bg-red-500/20 rounded text-red-400 transition-opacity"
                        title="Delete session">
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"></polyline><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path></svg>
                </button>
            </div>
        </div>
    `).join('');
}

function loadSession(id) {
    const session = sessions.find(s => s.id === id);
    if (!session) return;
    
    if (messages.length > 0) {
        showModal({
            type: 'warning',
            title: 'Load Session?',
            message: 'Current chat will be replaced. Continue?',
            confirmText: 'Load Session',
            cancelText: 'Cancel',
            onConfirm: () => {
                messages = [...session.messages];
                currentModel = session.model;
                currentSessionId = session.id;
                currentSystemPrompt = session.systemPrompt || '';
                
                // Update UI
                document.getElementById('chatModel').value = currentModel;
                const chatMessages = document.getElementById('chatMessages');
                chatMessages.innerHTML = '';
                messages.forEach(m => {
                    const { text, images } = normalizeMessageContent(m.content);
                    addChatMessage(m.role, text, images);
                });
                
                // Close sessions panel after loading
                document.getElementById('sessionsPanel').classList.add('hidden');
                switchTab('chat');
                showModal({
                    type: 'success',
                    title: 'Loaded',
                    message: `Session loaded: "${session.title}"`,
                    confirmText: 'OK'
                });
            }
        });
        return;
    }
    
    messages = [...session.messages];
    currentModel = session.model;
    currentSessionId = session.id;
    currentSystemPrompt = session.systemPrompt || '';
    
    // Update UI
    document.getElementById('chatModel').value = currentModel;
    const chatMessages = document.getElementById('chatMessages');
    chatMessages.innerHTML = '';
    messages.forEach(m => {
        const { text, images } = normalizeMessageContent(m.content);
        addChatMessage(m.role, text, images);
    });
    
    // Close sessions panel after loading
    document.getElementById('sessionsPanel').classList.add('hidden');
    switchTab('chat');
}

function deleteSession(id) {
    showModal({
        type: 'error',
        title: 'Delete Session?',
        message: `This action cannot be undone.`,
        confirmText: 'Delete',
        cancelText: 'Cancel',
        onConfirm: () => {
            sessions = sessions.filter(s => s.id !== id);
            localStorage.setItem('offgrid_sessions', JSON.stringify(sessions));
            
            if (currentSessionId === id) {
                currentSessionId = null;
            }
            
            renderSessions();
        }
    });
}

function createNewSession() {
    if (messages.length > 0) {
        showModal({
            type: 'warning',
            title: 'Start New Session?',
            message: 'Current chat will be cleared.',
            confirmText: 'New Session',
            cancelText: 'Cancel',
            onConfirm: () => {
                clearChatSilent();
                currentSessionId = null;
                switchTab('chat');
                showModal({
                    type: 'success',
                    title: 'New Session',
                    message: 'New session started',
                    confirmText: 'OK'
                });
            }
        });
        return;
    }
    
    clearChatSilent();
    currentSessionId = null;
    switchTab('chat');
    showModal({
        type: 'success',
        title: 'New Session',
        message: 'New session started',
        confirmText: 'OK'
    });
}

function filterSessions() {
    const query = document.getElementById('sessionSearch').value.toLowerCase();
    const sessionCards = document.querySelectorAll('[data-session-id]');
    
    sessionCards.forEach(card => {
        const text = card.textContent.toLowerCase();
        card.style.display = text.includes(query) ? 'block' : 'none';
    });
}

function clearChatSilent() {
    messages = [];
    document.getElementById('chatMessages').innerHTML = `
        <div id="emptyStatePrompts" class="flex flex-col items-center justify-center h-full max-w-2xl mx-auto px-4">
            <div class="text-center mb-8">
                <h2 class="text-2xl font-semibold text-primary mb-2">What can I help you with?</h2>
                <p class="text-sm text-secondary">Select a model above and try one of these prompts</p>
            </div>
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 w-full">
                <button onclick="useExamplePrompt('Explain how neural networks work in simple terms')" class="example-prompt-card">
                    <div class="card-title">Explain a concept</div>
                    <div class="card-desc">"Explain how neural networks work in simple terms"</div>
                </button>
                <button onclick="useExamplePrompt('Write a Python function that sorts a list using quicksort')" class="example-prompt-card">
                    <div class="card-title">Write code</div>
                    <div class="card-desc">"Write a Python function that sorts a list using quicksort"</div>
                </button>
                <button onclick="useExamplePrompt('Help me brainstorm ideas for a mobile app that helps people learn new languages')" class="example-prompt-card">
                    <div class="card-title">Brainstorm ideas</div>
                    <div class="card-desc">"Help me brainstorm ideas for a mobile app..."</div>
                </button>
                <button onclick="useExamplePrompt('Summarize the key differences between REST and GraphQL APIs')" class="example-prompt-card">
                    <div class="card-title">Compare topics</div>
                    <div class="card-desc">"Summarize the key differences between REST and GraphQL"</div>
                </button>
            </div>
        </div>
    `;
    document.getElementById('chatInput').value = '';
    updateChatStats();
    saveMessages();
}

// Stop generation
function stopGeneration() {
    if (abortController) {
        abortController.abort();
        abortController = null;
    }
    resetChatState();
}

// Reset chat state (useful for debugging stuck states)
function resetChatState() {
    isGenerating = false;
    pendingRequest = false;
    const sendBtn = document.getElementById('sendBtn');
    const stopBtn = document.getElementById('stopBtn');
    const chatInput = document.getElementById('chatInput');
    
    if (sendBtn) sendBtn.disabled = false;
    if (stopBtn) stopBtn.classList.add('hidden');
    if (chatInput) chatInput.disabled = false;
    
    // Update sidebar status using helper if available
    if (typeof updateSidebarStatus === 'function') {
        updateSidebarStatus('ready', currentModel || '');
    }
}

// Make resetChatState globally accessible for debugging
window.resetChatState = resetChatState;

// Make handleModelChange globally accessible for manual testing
window.handleModelChange = handleModelChange;
window.testModelSwitch = function(modelName) {
    const select = document.getElementById('chatModel');
    select.value = modelName;
    handleModelChange();
};

// Image handling
const MAX_IMAGE_ATTACHMENTS = 5;
let imageAttachments = [];
let nextImageAttachmentId = 1;

function handleImageUpload(event) {
    const files = Array.from(event.target.files || []);
    if (!files.length) return;

    const remainingSlots = MAX_IMAGE_ATTACHMENTS - imageAttachments.length;
    if (remainingSlots <= 0) {
        showToast(`Maximum of ${MAX_IMAGE_ATTACHMENTS} images reached`, 'error');
        event.target.value = '';
        return;
    }

    const filesToProcess = files.slice(0, remainingSlots);
    if (files.length > filesToProcess.length) {
        showToast(`Only ${remainingSlots} more image${remainingSlots === 1 ? '' : 's'} allowed`, 'warning');
    }

    filesToProcess.forEach(file => {
        if (file.size > 10 * 1024 * 1024) {
            showToast(`${file.name} is larger than 10MB`, 'error');
            return;
        }

        const reader = new FileReader();
        reader.onload = function(e) {
            // Convert to JPEG to ensure compatibility with llama.cpp (stb_image)
            const img = new Image();
            img.onload = function() {
                const canvas = document.createElement('canvas');
                canvas.width = img.width;
                canvas.height = img.height;
                const ctx = canvas.getContext('2d');
                // Fill white background for transparent images (PNG/WebP)
                ctx.fillStyle = '#FFFFFF';
                ctx.fillRect(0, 0, canvas.width, canvas.height);
                ctx.drawImage(img, 0, 0);
                
                // Convert to JPEG
                const jpegUrl = canvas.toDataURL('image/jpeg', 0.95);
                
                imageAttachments.push({
                    id: nextImageAttachmentId++,
                    name: file.name.replace(/\.[^/.]+$/, "") + ".jpg",
                    url: jpegUrl
                });
                updateImagePreview();
            };
            img.src = e.target.result;
        };
        reader.readAsDataURL(file);
    });

    // Reset input so same files can be selected again
    event.target.value = '';
}

function updateImagePreview() {
    const preview = document.getElementById('imagePreview');
    const list = document.getElementById('imagePreviewList');
    const countLabel = document.getElementById('imageAttachmentCount');

    if (!preview || !list || !countLabel) return;

    list.innerHTML = '';

    if (imageAttachments.length === 0) {
        preview.classList.add('hidden');
        countLabel.textContent = '0 attachments';
        return;
    }

    preview.classList.remove('hidden');
    countLabel.textContent = `${imageAttachments.length} attachment${imageAttachments.length === 1 ? '' : 's'}`;

    imageAttachments.forEach(att => {
        const wrapper = document.createElement('div');
        wrapper.className = 'relative group';
        const altText = escapeAttribute(att.name || 'Attachment');
        const imageUrl = escapeAttribute(att.url);
        wrapper.innerHTML = `
            <img src="${imageUrl}" alt="${altText}" class="h-24 w-24 object-cover rounded-lg border border-white/10 shadow-sm">
            <button type="button" onclick="removeImageAttachment(${att.id})" class="absolute -top-2 -right-2 bg-red-500 text-white rounded-full w-6 h-6 flex items-center justify-center text-xs hover:bg-red-600 shadow-md opacity-0 group-hover:opacity-100 transition-opacity">
                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"></line><line x1="6" y1="6" x2="18" y2="18"></line></svg>
            </button>`;
        list.appendChild(wrapper);
    });
}

function removeImageAttachment(id) {
    imageAttachments = imageAttachments.filter(att => att.id !== id);
    updateImagePreview();
}

function clearAllImages() {
    imageAttachments = [];
    updateImagePreview();
}
