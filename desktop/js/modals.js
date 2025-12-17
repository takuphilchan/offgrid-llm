function showModal({ type = 'info', title, message, confirmText = 'OK', cancelText, onConfirm, onCancel }) {
    const overlay = document.createElement('div');
    overlay.className = 'modal-overlay';
    
    const icon = type === 'error' ? 
        '<div class="w-12 h-12 rounded-full bg-red-100 text-red-500 flex items-center justify-center mb-4 mx-auto"><svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"></line><line x1="6" y1="6" x2="18" y2="18"></line></svg></div>' :
        type === 'warning' ?
        '<div class="w-12 h-12 rounded-full bg-yellow-100 text-yellow-500 flex items-center justify-center mb-4 mx-auto"><svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"></path><line x1="12" y1="9" x2="12" y2="13"></line><line x1="12" y1="17" x2="12.01" y2="17"></line></svg></div>' :
        '<div class="w-12 h-12 rounded-full bg-blue-100 text-blue-500 flex items-center justify-center mb-4 mx-auto"><svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"></circle><line x1="12" y1="16" x2="12" y2="12"></line><line x1="12" y1="8" x2="12.01" y2="8"></line></svg></div>';

    const cancelButton = cancelText ? `<button class="btn btn-secondary" data-action="cancel">${cancelText}</button>` : '';
    
    overlay.innerHTML = `
        <div class="modal-dialog ${type}">
            ${icon}
            <h3 class="text-lg font-bold mb-2 text-center" style="color: var(--text-primary)">${title}</h3>
            <p class="text-center mb-4" style="color: var(--text-secondary)">${message}</p>
            <div class="flex justify-center gap-3">
                ${cancelButton}
                <button class="btn ${type === 'error' ? 'btn-danger' : 'btn-primary'}" data-action="confirm">${confirmText}</button>
            </div>
        </div>
    `;
    
    document.body.appendChild(overlay);
    
    // Handle clicks
    overlay.addEventListener('click', (e) => {
        if (e.target.classList.contains('modal-overlay')) {
            overlay.remove();
            if (onCancel) onCancel();
        }
    });
    
    overlay.querySelector('[data-action="confirm"]')?.addEventListener('click', () => {
        overlay.remove();
        if (onConfirm) onConfirm();
    });
    
    overlay.querySelector('[data-action="cancel"]')?.addEventListener('click', () => {
        overlay.remove();
        if (onCancel) onCancel();
    });
}

// Simple alert dialog (replaces browser alert)
function showAlert(message, type = 'info', title = null) {
    const overlay = document.createElement('div');
    overlay.className = 'modal-overlay';
    
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
    
    const icon = type === 'error' ? 
        '<div class="w-12 h-12 rounded-full bg-red-500/20 text-red-400 flex items-center justify-center mb-4 mx-auto"><svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"></circle><line x1="15" y1="9" x2="9" y2="15"></line><line x1="9" y1="9" x2="15" y2="15"></line></svg></div>' :
        type === 'warning' ?
        '<div class="w-12 h-12 rounded-full bg-yellow-500/20 text-yellow-400 flex items-center justify-center mb-4 mx-auto"><svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"></path><line x1="12" y1="9" x2="12" y2="13"></line><line x1="12" y1="17" x2="12.01" y2="17"></line></svg></div>' :
        type === 'success' ?
        '<div class="w-12 h-12 rounded-full bg-green-500/20 text-green-400 flex items-center justify-center mb-4 mx-auto"><svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"></path><polyline points="22 4 12 14.01 9 11.01"></polyline></svg></div>' :
        '<div class="w-12 h-12 rounded-full bg-blue-500/20 text-blue-400 flex items-center justify-center mb-4 mx-auto"><svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"></circle><line x1="12" y1="16" x2="12" y2="12"></line><line x1="12" y1="8" x2="12.01" y2="8"></line></svg></div>';

    overlay.innerHTML = `
        <div class="modal-dialog ${type}">
            ${icon}
            <h3 class="text-lg font-bold mb-2 text-center" style="color: var(--text-primary)">${title}</h3>
            <p class="text-center mb-6" style="color: var(--text-secondary)">${message}</p>
            <div class="flex justify-center">
                <button class="btn btn-primary px-8" data-action="ok">OK</button>
            </div>
        </div>
    `;
    
    document.body.appendChild(overlay);
    
    // Focus the OK button
    overlay.querySelector('[data-action="ok"]').focus();
    
    // Handle clicks
    overlay.addEventListener('click', (e) => {
        if (e.target.classList.contains('modal-overlay') || e.target.dataset.action === 'ok') {
            overlay.remove();
        }
    });
    
    // Handle Enter/Escape keys
    const handleKey = (e) => {
        if (e.key === 'Enter' || e.key === 'Escape') {
            overlay.remove();
            document.removeEventListener('keydown', handleKey);
        }
    };
    document.addEventListener('keydown', handleKey);
}

// Confirmation dialog system (replaces native confirm())
function showConfirm(message, onConfirm, options = {}) {
    const title = options.title || 'Confirm';
    const confirmText = options.confirmText || 'OK';
    const cancelText = options.cancelText || 'Cancel';
    const type = options.type || 'info';
    
    const overlay = document.createElement('div');
    overlay.className = 'modal-overlay';
    
    // Icon based on type
    const icons = {
        error: '<div class="w-12 h-12 rounded-full bg-red-500/20 text-red-400 flex items-center justify-center mb-4 mx-auto"><svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"></circle><line x1="15" y1="9" x2="9" y2="15"></line><line x1="9" y1="9" x2="15" y2="15"></line></svg></div>',
        warning: '<div class="w-12 h-12 rounded-full bg-yellow-500/20 text-yellow-400 flex items-center justify-center mb-4 mx-auto"><svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"></path><line x1="12" y1="9" x2="12" y2="13"></line><line x1="12" y1="17" x2="12.01" y2="17"></line></svg></div>',
        success: '<div class="w-12 h-12 rounded-full bg-green-500/20 text-green-400 flex items-center justify-center mb-4 mx-auto"><svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"></path><polyline points="22 4 12 14.01 9 11.01"></polyline></svg></div>',
        info: '<div class="w-12 h-12 rounded-full bg-blue-500/20 text-blue-400 flex items-center justify-center mb-4 mx-auto"><svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"></circle><line x1="12" y1="16" x2="12" y2="12"></line><line x1="12" y1="8" x2="12.01" y2="8"></line></svg></div>'
    };
    const icon = icons[type] || icons.info;
    
    // Button style based on type
    const btnStyles = {
        error: 'bg-red-600 hover:bg-red-700 text-white',
        warning: 'bg-yellow-600 hover:bg-yellow-700 text-white',
        success: 'bg-green-600 hover:bg-green-700 text-white',
        info: 'bg-blue-600 hover:bg-blue-700 text-white'
    };
    const btnStyle = btnStyles[type] || btnStyles.info;

    overlay.innerHTML = `
        <div class="modal-dialog ${type}">
            ${icon}
            <h3 class="text-lg font-bold mb-2 text-center" style="color: var(--text-primary)">${title}</h3>
            <p class="text-center mb-6" style="color: var(--text-secondary)">${message}</p>
            <div class="flex justify-center gap-3">
                <button class="btn btn-secondary px-6" data-action="cancel">${cancelText}</button>
                <button class="btn px-6 ${btnStyle}" data-action="confirm">${confirmText}</button>
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
    const title = options.title || 'Notice';
    const buttonText = options.buttonText || 'OK';
    const type = options.type || 'info'; // info, success, warning, error
    const onClose = options.onClose || null;
    
    const overlay = document.createElement('div');
    overlay.className = 'modal-overlay';
    
    // Icon based on type
    const icons = {
        error: '<div class="w-12 h-12 rounded-full bg-red-500/20 text-red-400 flex items-center justify-center mb-4 mx-auto"><svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"></circle><line x1="15" y1="9" x2="9" y2="15"></line><line x1="9" y1="9" x2="15" y2="15"></line></svg></div>',
        warning: '<div class="w-12 h-12 rounded-full bg-yellow-500/20 text-yellow-400 flex items-center justify-center mb-4 mx-auto"><svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"></path><line x1="12" y1="9" x2="12" y2="13"></line><line x1="12" y1="17" x2="12.01" y2="17"></line></svg></div>',
        success: '<div class="w-12 h-12 rounded-full bg-green-500/20 text-green-400 flex items-center justify-center mb-4 mx-auto"><svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"></path><polyline points="22 4 12 14.01 9 11.01"></polyline></svg></div>',
        info: '<div class="w-12 h-12 rounded-full bg-blue-500/20 text-blue-400 flex items-center justify-center mb-4 mx-auto"><svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"></circle><line x1="12" y1="16" x2="12" y2="12"></line><line x1="12" y1="8" x2="12.01" y2="8"></line></svg></div>'
    };
    const icon = icons[type] || icons.info;
    
    // Button style based on type
    const btnStyles = {
        error: 'bg-red-600 hover:bg-red-700 text-white',
        warning: 'bg-yellow-600 hover:bg-yellow-700 text-white',
        success: 'bg-emerald-600 hover:bg-emerald-700 text-white',
        info: 'bg-accent hover:bg-accent/80 text-white'
    };
    const btnStyle = btnStyles[type] || btnStyles.info;
    
    // Border color based on type
    const borderClass = type === 'error' ? 'error' : type === 'warning' ? 'warning' : '';

    overlay.innerHTML = `
        <div class="modal-dialog ${borderClass}">
            ${icon}
            <h3 class="text-lg font-bold mb-2 text-center" style="color: var(--text-primary)">${title}</h3>
            <p class="text-center mb-6" style="color: var(--text-secondary)">${message}</p>
            <div class="flex justify-center">
                <button class="btn px-8 ${btnStyle}" data-action="close">${buttonText}</button>
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
            <h3 class="text-lg font-bold mb-2 text-center" style="color: var(--text-primary)">${title}</h3>
            <p class="text-center mb-4" style="color: var(--text-secondary)">${message}</p>
            <input type="text" class="input-theme w-full p-2 mb-6" value="${defaultValue}" placeholder="${placeholder}" id="promptInput">
            <div class="flex justify-center gap-3">
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
