// ============================================
// USER MANAGEMENT FUNCTIONS
// ============================================

async function loadUsers() {
    try {
        const resp = await fetch('/v1/users');
        const data = await resp.json();
        
        const tbody = document.getElementById('usersTableBody');
        const users = data.users || [];
        
        document.getElementById('totalUsersCount').textContent = users.length;
        document.getElementById('adminUsersCount').textContent = users.filter(u => u.role === 'admin').length;
        
        if (users.length === 0) {
            tbody.innerHTML = '<tr><td colspan="5" class="text-center py-8 text-secondary">No users found</td></tr>';
            return;
        }
        
        tbody.innerHTML = users.map(u => `
            <tr class="border-b border-theme hover:bg-tertiary/50">
                <td class="py-3">
                    <div class="flex items-center gap-3">
                        <div class="w-8 h-8 rounded-full bg-accent/20 flex items-center justify-center text-accent font-bold text-sm">
                            ${u.username.charAt(0).toUpperCase()}
                        </div>
                        <div>
                            <div class="font-medium">${u.username}</div>
                            <div class="text-xs text-secondary">${u.id.substring(0, 8)}...</div>
                        </div>
                    </div>
                </td>
                <td class="py-3">
                    <span class="px-2 py-1 rounded text-xs font-medium ${getRoleBadgeClass(u.role)}">${u.role}</span>
                </td>
                <td class="py-3 text-sm text-secondary">${formatDate(u.created_at)}</td>
                <td class="py-3 text-sm text-secondary">${u.last_login_at ? formatDate(u.last_login_at) : 'Never'}</td>
                <td class="py-3 text-right">
                    <button onclick="showUserDetails('${u.id}')" class="btn btn-secondary btn-sm mr-1">View</button>
                    <button onclick="deleteUser('${u.id}')" class="btn btn-danger btn-sm">Delete</button>
                </td>
            </tr>
        `).join('');
    } catch (e) {
        console.error('Failed to load users:', e);
        document.getElementById('usersTableBody').innerHTML = '<tr><td colspan="5" class="text-center py-8 text-red-400">Failed to load users</td></tr>';
    }
}

function getRoleBadgeClass(role) {
    const classes = {
        'admin': 'bg-red-500/20 text-red-400',
        'user': 'bg-blue-500/20 text-blue-400',
        'viewer': 'bg-green-500/20 text-green-400',
        'guest': 'bg-gray-500/20 text-gray-400'
    };
    return classes[role] || classes['user'];
}

function formatDate(dateStr) {
    if (!dateStr) return 'N/A';
    return new Date(dateStr).toLocaleDateString();
}

function togglePasswordVisibility(inputId, btn) {
    const input = document.getElementById(inputId);
    const showIcon = btn.querySelector('.show-icon');
    const hideIcon = btn.querySelector('.hide-icon');
    if (input.type === 'password') {
        input.type = 'text';
        showIcon.classList.add('hidden');
        hideIcon.classList.remove('hidden');
    } else {
        input.type = 'password';
        showIcon.classList.remove('hidden');
        hideIcon.classList.add('hidden');
    }
}

function showCreateUserModal() {
    document.getElementById('createUserModal').classList.add('active');
}

function hideCreateUserModal() {
    document.getElementById('createUserModal').classList.remove('active');
}

async function createUser() {
    const username = document.getElementById('newUsername').value;
    const password = document.getElementById('newUserPassword').value;
    const role = document.getElementById('newUserRole').value;
    
    if (!username) {
        showAlert('Please enter a username', { title: 'Missing Information', type: 'warning' });
        return;
    }
    
    if (!password) {
        showAlert('Please enter a password', { title: 'Missing Information', type: 'warning' });
        return;
    }
    
    try {
        const resp = await fetch('/v1/users', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ username, password, role })
        });
        
        if (!resp.ok) {
            const err = await resp.json();
            throw new Error(err.error || 'Failed to create user');
        }
        
        const data = await resp.json();
        hideCreateUserModal();
        loadUsers();
        
        // Clear form
        document.getElementById('newUsername').value = '';
        document.getElementById('newUserPassword').value = '';
        document.getElementById('newUserRole').value = 'user';
        
        // Offer to login as new user
        showConfirm(
            `User "${username}" created!<br><br>` +
            `<strong>API Key:</strong><br><code class="bg-tertiary px-2 py-1 rounded text-xs break-all">${data.api_key}</code><br><br>` +
            `<small class="text-secondary">Save this key - it won't be shown again.</small><br><br>` +
            `Would you like to login as this user now?`,
            async () => {
                // Auto-login with the credentials
                const loginResp = await fetch('/v1/auth/login', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ username, password }),
                    credentials: 'include'
                });
                if (loginResp.ok) {
                    const loginData = await loginResp.json();
                    setAuthUser(loginData.user);
                    showAlert(`Logged in as ${username}`, { title: 'Welcome!', type: 'success' });
                }
            },
            { title: 'User Created', type: 'success', confirmText: 'Login Now', cancelText: 'Not Now' }
        );
    } catch (e) {
        showAlert('Failed to create user: ' + e.message, { title: 'Error', type: 'error' });
    }
}

async function deleteUser(id) {
    showConfirm('Are you sure you want to delete this user? This action cannot be undone.', async () => {
        try {
            const resp = await fetch(`/v1/users/${id}`, { method: 'DELETE' });
            if (!resp.ok) throw new Error('Failed to delete user');
            loadUsers();
            showAlert('User deleted successfully', { title: 'Deleted', type: 'success' });
        } catch (e) {
            showAlert('Failed to delete user: ' + e.message, { title: 'Error', type: 'error' });
        }
    }, { title: 'Delete User?', type: 'warning', confirmText: 'Delete', cancelText: 'Cancel' });
}

function filterUsers() {
    const query = document.getElementById('userSearch').value.toLowerCase();
    const rows = document.querySelectorAll('#usersTableBody tr');
    rows.forEach(row => {
        const text = row.textContent.toLowerCase();
        row.style.display = text.includes(query) ? '' : 'none';
    });
}

let currentUserDetails = null;

async function showUserDetails(userId) {
    try {
        const [userResp, quotaResp] = await Promise.all([
            fetch(`/v1/users/${userId}`),
            fetch(`/v1/quota/${userId}`)
        ]);
        
        if (!userResp.ok) throw new Error('Failed to fetch user');
        const user = await userResp.json();
        const quotaData = quotaResp.ok ? await quotaResp.json() : { quotas: [] };
        currentUserDetails = user;
        
        const quotas = quotaData.quotas || [];
        let quotaHTML = '';
        
        if (quotas.length > 0) {
            quotaHTML = quotas.map(q => {
                const percent = q.limit > 0 ? Math.min(100, Math.round((q.current / q.limit) * 100)) : 0;
                const barColor = percent >= 90 ? 'bg-red-500' : percent >= 70 ? 'bg-amber-500' : 'bg-emerald-500';
                return `
                    <div class="bg-tertiary rounded-lg p-3">
                        <div class="flex justify-between items-center mb-2">
                            <span class="text-xs font-medium capitalize">${q.type.replace(/_/g, ' ')}</span>
                            <span class="text-xs text-secondary">per ${q.period}</span>
                        </div>
                        <div class="flex items-baseline gap-1 mb-2">
                            <span class="text-lg font-bold">${formatNumber(q.current)}</span>
                            <span class="text-xs text-secondary">/ ${formatNumber(q.limit)}</span>
                        </div>
                        <div class="h-1.5 bg-primary rounded-full overflow-hidden">
                            <div class="h-full ${barColor} transition-all" style="width: ${percent}%"></div>
                        </div>
                        ${q.exceeded ? '<span class="text-xs text-red-400 mt-1 block">Quota exceeded</span>' : ''}
                    </div>
                `;
            }).join('');
        } else {
            quotaHTML = `
                <div class="col-span-2 text-center py-4">
                    <p class="text-secondary text-sm">No quotas configured</p>
                    <p class="text-xs text-secondary mt-1">Default role-based limits apply</p>
                </div>
            `;
        }
        
        const content = document.getElementById('userDetailsContent');
        content.innerHTML = `
            <div class="flex items-center gap-4 pb-4 border-b border-theme">
                <div class="w-16 h-16 rounded-full bg-accent/20 flex items-center justify-center text-accent text-2xl font-bold">
                    ${user.username.charAt(0).toUpperCase()}
                </div>
                <div>
                    <h4 class="text-xl font-semibold">${user.username}</h4>
                    <span class="px-2 py-1 rounded text-xs font-medium ${getRoleBadgeClass(user.role)}">${user.role}</span>
                </div>
            </div>
            
            <div class="grid grid-cols-2 gap-4">
                <div>
                    <label class="text-xs text-secondary block mb-1">User ID</label>
                    <code class="text-sm bg-tertiary px-2 py-1 rounded block break-all">${user.id}</code>
                </div>
                <div>
                    <label class="text-xs text-secondary block mb-1">Created</label>
                    <span class="text-sm">${formatDate(user.created_at)}</span>
                </div>
                <div>
                    <label class="text-xs text-secondary block mb-1">Last Active</label>
                    <span class="text-sm">${user.last_login_at ? formatDate(user.last_login_at) : 'Never'}</span>
                </div>
                <div>
                    <label class="text-xs text-secondary block mb-1">API Key</label>
                    <span class="text-sm text-secondary italic">Hidden for security</span>
                </div>
            </div>
            
            <div class="pt-4 border-t border-theme">
                <div class="flex justify-between items-center mb-3">
                    <h5 class="text-sm font-semibold">Quota & Usage</h5>
                    <div class="flex gap-2">
                        <button onclick="showSetQuotaModal('${user.id}')" class="text-xs text-accent hover:underline">Set Quota</button>
                        <button onclick="resetUserQuota('${user.id}')" class="text-xs text-amber-400 hover:underline">Reset Usage</button>
                    </div>
                </div>
                <div class="grid grid-cols-2 gap-3">
                    ${quotaHTML}
                </div>
            </div>
        `;
        
        document.getElementById('userDetailsModal').classList.add('active');
    } catch (e) {
        showAlert('Failed to load user details: ' + e.message, { title: 'Error', type: 'error' });
    }
}

function hideUserDetailsModal() {
    document.getElementById('userDetailsModal').classList.remove('active');
    currentUserDetails = null;
}

async function regenerateApiKey() {
    if (!currentUserDetails) return;
    
    showConfirm('Regenerate API key? The old key will stop working immediately.', async () => {
        try {
            const resp = await fetch(`/v1/users/${currentUserDetails.id}/regenerate-key`, {
                method: 'POST'
            });
            if (!resp.ok) throw new Error('Failed to regenerate key');
            const data = await resp.json();
            showAlert(`New API Key:<br><br><code class="bg-tertiary px-2 py-1 rounded text-sm break-all">${data.api_key}</code><br><br><small>Copy this - it won't be shown again.</small>`, { title: 'API Key Regenerated', type: 'success' });
            hideUserDetailsModal();
            loadUsers();
        } catch (e) {
            showAlert('Failed to regenerate API key: ' + e.message, { title: 'Error', type: 'error' });
        }
    }, { title: 'Regenerate API Key?', type: 'warning', confirmText: 'Regenerate', cancelText: 'Cancel' });
}

async function editUserRole() {
    if (!currentUserDetails) return;
    
    const roles = ['admin', 'user', 'viewer', 'guest'];
    const currentRole = currentUserDetails.role;
    
    // Create a simple role selector
    const roleOptions = roles.map(r => 
        `<option value="${r}" ${r === currentRole ? 'selected' : ''}>${r}</option>`
    ).join('');
    
    const html = `
        <select id="newRoleSelect" class="w-full input-theme rounded-lg px-3 py-2">
            ${roleOptions}
        </select>
    `;
    
    // Show a custom modal with the select
    showInputPrompt('Select new role:', async (newRole) => {
        if (!newRole || newRole === currentRole) return;
        try {
            const resp = await fetch(`/v1/users/${currentUserDetails.id}/role`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ role: newRole })
            });
            if (!resp.ok) throw new Error('Failed to update role');
            showAlert(`Role updated to ${newRole}`, { title: 'Role Updated', type: 'success' });
            hideUserDetailsModal();
            loadUsers();
        } catch (e) {
            showAlert('Failed to update role: ' + e.message, { title: 'Error', type: 'error' });
        }
    }, { 
        title: 'Change User Role',
        inputType: 'select',
        selectOptions: roles,
        defaultValue: currentRole
    });
}

// Add showInputPrompt function for role editing
function showInputPrompt(message, onSubmit, options = {}) {
    const { title = 'Input', inputType = 'text', selectOptions = [], defaultValue = '' } = options;
    
    let inputHtml = '';
    if (inputType === 'select') {
        const opts = selectOptions.map(o => 
            `<option value="${o}" ${o === defaultValue ? 'selected' : ''}>${o}</option>`
        ).join('');
        inputHtml = `<select id="promptInput" class="w-full input-theme rounded-lg px-3 py-2 mt-3">${opts}</select>`;
    } else {
        inputHtml = `<input type="text" id="promptInput" value="${defaultValue}" class="w-full input-theme rounded-lg px-3 py-2 mt-3">`;
    }
    
    const modal = document.createElement('div');
    modal.className = 'fixed inset-0 bg-black/50 flex items-center justify-center z-[100]';
    modal.innerHTML = `
        <div class="bg-card border border-theme rounded-xl p-6 w-full max-w-sm mx-4 shadow-2xl">
            <h3 class="text-lg font-semibold mb-2">${title}</h3>
            <p class="text-secondary text-sm">${message}</p>
            ${inputHtml}
            <div class="flex gap-3 mt-4">
                <button id="promptCancel" class="btn btn-secondary flex-1">Cancel</button>
                <button id="promptSubmit" class="btn btn-primary flex-1">Confirm</button>
            </div>
        </div>
    `;
    document.body.appendChild(modal);
    
    const input = document.getElementById('promptInput');
    input.focus();
    
    modal.querySelector('#promptCancel').addEventListener('click', () => {
        modal.remove();
    });
    
    modal.querySelector('#promptSubmit').addEventListener('click', () => {
        onSubmit(input.value);
        modal.remove();
    });
    
    input.addEventListener('keydown', (e) => {
        if (e.key === 'Enter') {
            onSubmit(input.value);
            modal.remove();
        }
    });
}

// Helper function to format numbers with K/M suffixes
function formatNumber(num) {
    if (num >= 1000000) {
        return (num / 1000000).toFixed(1) + 'M';
    }
    if (num >= 1000) {
        return (num / 1000).toFixed(1) + 'K';
    }
    return num.toString();
}

// Reset user quota usage
async function resetUserQuota(userId) {
    showConfirm('Reset all quota usage counters for this user? This will not change the limits.', async () => {
        try {
            const resp = await fetch(`/v1/quota/${userId}?action=reset`, {
                method: 'DELETE'
            });
            if (!resp.ok) throw new Error('Failed to reset quota');
            showAlert('Quota usage reset successfully', { title: 'Reset Complete', type: 'success' });
            showUserDetails(userId); // Refresh the modal
        } catch (e) {
            showAlert('Failed to reset quota: ' + e.message, { title: 'Error', type: 'error' });
        }
    }, { title: 'Reset Quota Usage?', type: 'warning', confirmText: 'Reset', cancelText: 'Cancel' });
}

// Show modal to set quota
function showSetQuotaModal(userId) {
    const modal = document.createElement('div');
    modal.id = 'setQuotaModal';
    modal.className = 'fixed inset-0 bg-black/50 flex items-center justify-center z-[100]';
    modal.innerHTML = `
        <div class="bg-card border border-theme rounded-xl p-6 w-full max-w-md mx-4 shadow-2xl">
            <h3 class="text-lg font-semibold mb-4">Set Quota Limit</h3>
            
            <div class="space-y-4">
                <div>
                    <label class="text-xs text-secondary block mb-1">Quota Type</label>
                    <select id="quotaType" class="w-full input-theme rounded-lg px-3 py-2">
                        <option value="requests">Requests</option>
                        <option value="tokens_input">Input Tokens</option>
                        <option value="tokens_output">Output Tokens</option>
                        <option value="tokens_total">Total Tokens</option>
                        <option value="documents">Documents</option>
                        <option value="rag_queries">RAG Queries</option>
                    </select>
                </div>
                <div>
                    <label class="text-xs text-secondary block mb-1">Period</label>
                    <select id="quotaPeriod" class="w-full input-theme rounded-lg px-3 py-2">
                        <option value="minute">Per Minute</option>
                        <option value="hour">Per Hour</option>
                        <option value="day" selected>Per Day</option>
                        <option value="month">Per Month</option>
                    </select>
                </div>
                <div>
                    <label class="text-xs text-secondary block mb-1">Limit</label>
                    <input type="number" id="quotaLimit" min="1" value="1000" class="w-full input-theme rounded-lg px-3 py-2">
                </div>
            </div>
            
            <div class="flex gap-3 mt-6">
                <button onclick="document.getElementById('setQuotaModal').remove()" class="btn btn-secondary flex-1">Cancel</button>
                <button onclick="submitQuotaLimit('${userId}')" class="btn btn-primary flex-1">Set Limit</button>
            </div>
        </div>
    `;
    document.body.appendChild(modal);
}

// Submit quota limit
async function submitQuotaLimit(userId) {
    const type = document.getElementById('quotaType').value;
    const period = document.getElementById('quotaPeriod').value;
    const limit = parseInt(document.getElementById('quotaLimit').value, 10);
    
    if (!limit || limit <= 0) {
        showAlert('Please enter a valid limit', { title: 'Invalid Input', type: 'warning' });
        return;
    }
    
    try {
        const resp = await fetch(`/v1/quota/${userId}`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ type, period, limit })
        });
        
        if (!resp.ok) {
            const err = await resp.json();
            throw new Error(err.error || 'Failed to set quota');
        }
        
        document.getElementById('setQuotaModal').remove();
        showAlert(`Quota set: ${formatNumber(limit)} ${type.replace(/_/g, ' ')} per ${period}`, { title: 'Quota Updated', type: 'success' });
        showUserDetails(userId); // Refresh the modal
    } catch (e) {
        showAlert('Failed to set quota: ' + e.message, { title: 'Error', type: 'error' });
    }
}
