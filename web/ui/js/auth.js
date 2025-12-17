// ============================================
// AUTHENTICATION FUNCTIONS
// ============================================

let currentAuthUser = null;

async function checkAuthStatus() {
    try {
        // Try to get current user info from a protected endpoint
        const resp = await fetch('/v1/users/me', { credentials: 'include' });
        if (resp.ok) {
            const data = await resp.json();
            if (data.user) {
                setAuthUser(data.user);
                return;
            }
        }
    } catch (e) {
        console.log('Auth check failed, treating as guest');
    }
    setAuthUser(null);
}

function setAuthUser(user) {
    currentAuthUser = user;
    const loggedIn = document.getElementById('authLoggedIn');
    const loggedOut = document.getElementById('authLoggedOut');
    
    if (user) {
        loggedIn.classList.remove('hidden');
        loggedOut.classList.add('hidden');
        document.getElementById('authUserInitial').textContent = user.username.charAt(0).toUpperCase();
        document.getElementById('authUsername').textContent = user.username;
        document.getElementById('authRole').textContent = user.role;
    } else {
        loggedIn.classList.add('hidden');
        loggedOut.classList.remove('hidden');
    }
}

function showLoginModal() {
    document.getElementById('loginModal').classList.remove('hidden');
    document.getElementById('loginModal').classList.add('flex');
    document.getElementById('loginError').classList.add('hidden');
    document.getElementById('loginUsername').value = '';
    document.getElementById('loginPassword').value = '';
    document.getElementById('loginUsername').focus();
}

function hideLoginModal() {
    document.getElementById('loginModal').classList.add('hidden');
    document.getElementById('loginModal').classList.remove('flex');
}

async function handleLogin() {
    const username = document.getElementById('loginUsername').value;
    const password = document.getElementById('loginPassword').value;
    const errorEl = document.getElementById('loginError');
    
    if (!username) {
        errorEl.textContent = 'Please enter a username';
        errorEl.classList.remove('hidden');
        return;
    }
    
    try {
        const resp = await fetch('/v1/auth/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ username, password }),
            credentials: 'include'
        });
        
        const data = await resp.json();
        
        if (resp.ok) {
            hideLoginModal();
            setAuthUser(data.user);
            showAlert('Logged in successfully', { title: 'Welcome', type: 'success' });
        } else {
            errorEl.textContent = data.error || 'Login failed';
            errorEl.classList.remove('hidden');
        }
    } catch (e) {
        errorEl.textContent = 'Connection error: ' + e.message;
        errorEl.classList.remove('hidden');
    }
}

async function handleLogout() {
    try {
        await fetch('/v1/auth/logout', {
            method: 'POST',
            credentials: 'include'
        });
        setAuthUser(null);
        showAlert('Logged out successfully', { type: 'info' });
    } catch (e) {
        console.error('Logout failed:', e);
    }
}

// System configuration (feature flags)
let systemConfig = { multi_user_mode: false, features: {} };

async function loadSystemConfig() {
    try {
        const resp = await fetch('/v1/system/config');
        if (resp.ok) {
            systemConfig = await resp.json();
            applyFeatureFlags();
        }
    } catch (e) {
        console.log('Using default config (single-user mode)');
    }
}

function applyFeatureFlags() {
    const multiUserNav = document.getElementById('multiUserNav');
    const authStatus = document.getElementById('authStatus');
    const createAccountLink = document.getElementById('loginCreateAccountLink');
    
    if (systemConfig.multi_user_mode) {
        // Multi-user mode: show Users, Metrics, and auth status
        if (multiUserNav) multiUserNav.classList.remove('hidden');
        if (authStatus) authStatus.classList.remove('hidden');
        if (createAccountLink) createAccountLink.classList.remove('hidden');
    } else {
        // Single-user mode: hide Users, Metrics, and auth status
        if (multiUserNav) multiUserNav.classList.add('hidden');
        if (authStatus) authStatus.classList.add('hidden');
        if (createAccountLink) createAccountLink.classList.add('hidden');
    }
}

// Load config and check auth on page load
loadSystemConfig();
checkAuthStatus();

