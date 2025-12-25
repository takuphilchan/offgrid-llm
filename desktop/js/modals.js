function showModal({ type = 'info', title, message, confirmText = 'OK', cancelText, onConfirm, onCancel }) {
    const overlay = document.createElement('div');
    overlay.className = 'modal-overlay';
    overlay.setAttribute('role', 'dialog');
    overlay.setAttribute('aria-modal', 'true');
    overlay.setAttribute('aria-labelledby', 'modal-title');
    overlay.setAttribute('aria-describedby', 'modal-message');
    
    // Icon SVGs based on type
    const iconSvgs = {
        error: '<svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"></circle><line x1="15" y1="9" x2="9" y2="15"></line><line x1="9" y1="9" x2="15" y2="15"></line></svg>',
        warning: '<svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"></path><line x1="12" y1="9" x2="12" y2="13"></line><line x1="12" y1="17" x2="12.01" y2="17"></line></svg>',
        success: '<svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"></path><polyline points="22 4 12 14.01 9 11.01"></polyline></svg>',
        info: '<svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"></circle><line x1="12" y1="16" x2="12" y2="12"></line><line x1="12" y1="8" x2="12.01" y2="8"></line></svg>'
    };
    const iconSvg = iconSvgs[type] || iconSvgs.info;
    
    // Escape text content for security
    const safeTitle = typeof escapeHtml === 'function' ? escapeHtml(title) : title;
    const safeMessage = typeof escapeHtml === 'function' ? escapeHtml(message) : message;
    const safeConfirmText = typeof escapeHtml === 'function' ? escapeHtml(confirmText) : confirmText;
    const safeCancelText = cancelText ? (typeof escapeHtml === 'function' ? escapeHtml(cancelText) : cancelText) : '';
    
    const cancelButton = cancelText ? `<button class="btn btn-secondary" data-action="cancel">${safeCancelText}</button>` : '';
    
    overlay.innerHTML = `
        <div class="modal-dialog">
            <div class="modal-dialog-header">
                <div class="modal-dialog-icon ${type}" aria-hidden="true">${iconSvg}</div>
                <div class="modal-dialog-title" id="modal-title">${safeTitle}</div>
            </div>
            <p class="modal-dialog-message" id="modal-message">${safeMessage}</p>
            <div class="modal-dialog-actions">
                ${cancelButton}
                <button class="btn ${type === 'error' ? 'btn-danger' : 'btn-primary'}" data-action="confirm">${safeConfirmText}</button>
            </div>
        </div>
    `;
    
    document.body.appendChild(overlay);
    
    // Store previously focused element to restore later
    const previouslyFocused = document.activeElement;
    
    // Focus first button (trap focus in modal)
    const firstButton = overlay.querySelector('button');
    if (firstButton) firstButton.focus();
    
    // Focus trap - keep focus within modal
    const focusableElements = overlay.querySelectorAll('button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])');
    const firstFocusable = focusableElements[0];
    const lastFocusable = focusableElements[focusableElements.length - 1];
    
    function trapFocus(e) {
        if (e.key === 'Tab') {
            if (e.shiftKey && document.activeElement === firstFocusable) {
                e.preventDefault();
                lastFocusable.focus();
            } else if (!e.shiftKey && document.activeElement === lastFocusable) {
                e.preventDefault();
                firstFocusable.focus();
            }
        }
        if (e.key === 'Escape') {
            closeModal();
        }
    }
    
    function closeModal() {
        document.removeEventListener('keydown', trapFocus);
        overlay.remove();
        if (previouslyFocused) previouslyFocused.focus();
    }
    
    document.addEventListener('keydown', trapFocus);
    
    // Handle clicks
    overlay.addEventListener('click', (e) => {
        if (e.target.classList.contains('modal-overlay')) {
            closeModal();
            if (onCancel) onCancel();
        }
    });
    
    overlay.querySelector('[data-action="confirm"]')?.addEventListener('click', () => {
        closeModal();
        if (onConfirm) onConfirm();
    });
    
    overlay.querySelector('[data-action="cancel"]')?.addEventListener('click', () => {
        closeModal();
        if (onCancel) onCancel();
    });
}

// Simple alert dialog (replaces browser alert) - Legacy function signature
function _showAlertLegacy(message, type = 'info', title = null) {
    // Auto-detect type from message content
    if (!title) {
        if (type === 'error' || message.toLowerCase().includes('failed') || message.toLowerCase().includes('error')) {
            type = 'error';
            title = 'Error';
        } else if (message.toLowerCase().includes('success') || message.toLowerCase().includes('added') || message.toLowerCase().includes('deleted')) {
            type = 'success';
            title = 'Success';
        } else if (message.toLowerCase().includes('please') || message.toLowerCase().includes('select') || message.toLowerCase().includes('enter')) {
            type = 'warning';
            title = 'Notice';
        } else {
            title = 'Info';
        }
    }
    
    showAlert(message, { title, type });
}

// Confirmation dialog system (replaces native confirm())
function showConfirm(message, onConfirm, options = {}) {
    const title = options.title || 'Confirm';
    const confirmText = options.confirmText || 'OK';
    const cancelText = options.cancelText || 'Cancel';
    const type = options.type || 'info';
    
    const overlay = document.createElement('div');
    overlay.className = 'modal-overlay';
    
    // Icon SVGs based on type
    const iconSvgs = {
        error: '<svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"></circle><line x1="15" y1="9" x2="9" y2="15"></line><line x1="9" y1="9" x2="15" y2="15"></line></svg>',
        warning: '<svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"></path><line x1="12" y1="9" x2="12" y2="13"></line><line x1="12" y1="17" x2="12.01" y2="17"></line></svg>',
        success: '<svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"></path><polyline points="22 4 12 14.01 9 11.01"></polyline></svg>',
        info: '<svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"></circle><line x1="12" y1="16" x2="12" y2="12"></line><line x1="12" y1="8" x2="12.01" y2="8"></line></svg>'
    };
    const iconSvg = iconSvgs[type] || iconSvgs.info;

    overlay.innerHTML = `
        <div class="modal-dialog">
            <div class="modal-dialog-header">
                <div class="modal-dialog-icon ${type}">${iconSvg}</div>
                <div class="modal-dialog-title">${title}</div>
            </div>
            <p class="modal-dialog-message">${message}</p>
            <div class="modal-dialog-actions">
                <button class="btn btn-secondary" data-action="cancel">${cancelText}</button>
                <button class="btn btn-primary" data-action="confirm">${confirmText}</button>
            </div>
        </div>
    `;
    
    document.body.appendChild(overlay);
    
    // Focus the confirm button
    overlay.querySelector('[data-action="confirm"]').focus();
    
    // Handle clicks
    overlay.addEventListener('click', (e) => {
        if (e.target.classList.contains('modal-overlay') || e.target.dataset.action === 'cancel') {
            overlay.remove();
        } else if (e.target.dataset.action === 'confirm') {
            overlay.remove();
            if (onConfirm) onConfirm();
        }
    });
    
    // Handle Enter/Escape keys
    const handleKey = (e) => {
        if (e.key === 'Escape') {
            overlay.remove();
            document.removeEventListener('keydown', handleKey);
        } else if (e.key === 'Enter') {
            overlay.remove();
            document.removeEventListener('keydown', handleKey);
            if (onConfirm) onConfirm();
        }
    };
    document.addEventListener('keydown', handleKey);
}

// Alert dialog system (replaces native alert())
function showAlert(message, options = {}) {
    // Handle legacy call signature: showAlert(message, 'type')
    if (typeof options === 'string') {
        options = { type: options };
    }
    
    // Auto-detect title from type if not provided
    let title = options.title;
    let type = options.type || 'info';
    
    if (!title) {
        if (type === 'error' || message.toLowerCase().includes('failed') || message.toLowerCase().includes('error')) {
            type = 'error';
            title = 'Error';
        } else if (type === 'success' || message.toLowerCase().includes('success') || message.toLowerCase().includes('added') || message.toLowerCase().includes('deleted')) {
            type = 'success';
            title = 'Success';
        } else if (type === 'warning' || message.toLowerCase().includes('please') || message.toLowerCase().includes('select')) {
            type = 'warning';
            title = 'Notice';
        } else {
            title = 'Info';
        }
    }
    
    const buttonText = options.buttonText || 'OK';
    const onClose = options.onClose || null;
    
    const overlay = document.createElement('div');
    overlay.className = 'modal-overlay';
    
    // Icon SVGs based on type
    const iconSvgs = {
        error: '<svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"></circle><line x1="15" y1="9" x2="9" y2="15"></line><line x1="9" y1="9" x2="15" y2="15"></line></svg>',
        warning: '<svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"></path><line x1="12" y1="9" x2="12" y2="13"></line><line x1="12" y1="17" x2="12.01" y2="17"></line></svg>',
        success: '<svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"></path><polyline points="22 4 12 14.01 9 11.01"></polyline></svg>',
        info: '<svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"></circle><line x1="12" y1="16" x2="12" y2="12"></line><line x1="12" y1="8" x2="12.01" y2="8"></line></svg>'
    };
    const iconSvg = iconSvgs[type] || iconSvgs.info;

    overlay.innerHTML = `
        <div class="modal-dialog">
            <div class="modal-dialog-header">
                <div class="modal-dialog-icon ${type}">${iconSvg}</div>
                <div class="modal-dialog-title">${title}</div>
            </div>
            <p class="modal-dialog-message">${message}</p>
            <div class="modal-dialog-actions">
                <button class="btn btn-primary" data-action="close">${buttonText}</button>
            </div>
        </div>
    `;
    
    document.body.appendChild(overlay);
    
    // Focus the button
    overlay.querySelector('[data-action="close"]').focus();
    
    // Handle clicks
    overlay.addEventListener('click', (e) => {
        if (e.target.classList.contains('modal-overlay') || e.target.dataset.action === 'close') {
            overlay.remove();
            if (onClose) onClose();
        }
    });
    
    // Handle Enter/Escape keys
    const handleKey = (e) => {
        if (e.key === 'Escape' || e.key === 'Enter') {
            overlay.remove();
            document.removeEventListener('keydown', handleKey);
            if (onClose) onClose();
        }
    };
    document.addEventListener('keydown', handleKey);
}

// Prompt dialog system
function showPrompt({ title, message, defaultValue = '', placeholder = '', confirmText = 'OK', cancelText = 'Cancel', onConfirm, onCancel }) {
    const overlay = document.createElement('div');
    overlay.className = 'modal-overlay';
    
    overlay.innerHTML = `
        <div class="modal-dialog">
            <div class="modal-dialog-header">
                <div class="modal-dialog-icon info"><svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"></path><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"></path></svg></div>
                <div class="modal-dialog-title">${title}</div>
            </div>
            <div class="modal-dialog-message" style="padding-bottom: 0;">
                <p style="margin-bottom: 12px;">${message}</p>
                <input type="text" class="input-theme w-full rounded-lg px-3 py-2.5" value="${defaultValue}" placeholder="${placeholder}" id="promptInput">
            </div>
            <div class="modal-dialog-actions">
                <button class="btn btn-secondary" data-action="cancel">${cancelText}</button>
                <button class="btn btn-primary" data-action="confirm">${confirmText}</button>
            </div>
        </div>
    `;
    
    document.body.appendChild(overlay);
    
    const input = overlay.querySelector('#promptInput');
    input.focus();
    // Select all text if there is a default value
    if (defaultValue) input.select();

    // Handle Enter key
    input.addEventListener('keyup', (e) => {
        if (e.key === 'Enter') {
            const value = input.value;
            overlay.remove();
            if (onConfirm) onConfirm(value);
        }
    });
    
    // Handle clicks
    overlay.addEventListener('click', (e) => {
        if (e.target.classList.contains('modal-overlay')) {
            overlay.remove();
            if (onCancel) onCancel();
        }
    });
    
    overlay.querySelector('[data-action="confirm"]')?.addEventListener('click', () => {
        const value = input.value;
        overlay.remove();
        if (onConfirm) onConfirm(value);
    });
    
    overlay.querySelector('[data-action="cancel"]')?.addEventListener('click', () => {
        overlay.remove();
        if (onCancel) onCancel();
    });
}

// New chat - offer to save current session first
