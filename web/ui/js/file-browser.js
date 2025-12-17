// File Browser functionality
let currentBrowserPath = '';
let fileBrowserTarget = ''; // 'import' or 'export'

async function browseForPath(target) {
    fileBrowserTarget = target;
    const modal = document.getElementById('fileBrowserModal');
    modal.style.display = 'flex';
    
    // Load common paths
    await loadCommonPaths();
    
    // Browse to default path
    const currentPath = target === 'import' 
        ? document.getElementById('usbImportPath').value 
        : document.getElementById('usbExportPath').value;
    
    await browseTo(currentPath || '');
}

async function loadCommonPaths() {
    const container = document.getElementById('commonPathsList');
    container.innerHTML = '<span class="text-xs text-secondary">Loading...</span>';
    
    try {
        const response = await fetch('/v1/filesystem/common-paths');
        const data = await response.json();
        
        if (response.ok && data.paths) {
            container.innerHTML = data.paths
                .filter(p => p.exists)
                .map(p => `
                    <button onclick="browseTo('${p.path.replace(/'/g, "\\'")}')" 
                            class="px-3 py-1 text-xs bg-secondary hover:bg-accent hover:text-white rounded border border-theme transition-colors"
                            title="${p.description}">
                        ${p.label}
                    </button>
                `).join('');
        } else {
            container.innerHTML = '<span class="text-xs text-red-400">Failed to load paths</span>';
        }
    } catch (error) {
        container.innerHTML = '<span class="text-xs text-red-400">Error loading paths</span>';
    }
}

async function browseTo(path) {
    const listContainer = document.getElementById('browserFileList');
    const pathInput = document.getElementById('browserCurrentPath');
    
    listContainer.innerHTML = '<div class="p-4 text-center text-secondary text-sm">Loading...</div>';
    
    try {
        const response = await fetch('/v1/filesystem/browse', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path: path })
        });
        
        const data = await response.json();
        
        if (response.ok) {
            currentBrowserPath = data.current_path;
            pathInput.value = currentBrowserPath;
            
            let html = '';
            
            // Add directories
            if (data.directories && data.directories.length > 0) {
                const visibleDirs = data.directories.filter(d => !d.is_hidden);
                visibleDirs.forEach(dir => {
                    html += `
                        <div class="file-entry directory" onclick="browseTo('${dir.path.replace(/'/g, "\\'")}')">
                            <span class="file-icon">▸</span>
                            <span class="file-name">${dir.name}</span>
                        </div>
                    `;
                });
            }
            
            // Show message if no directories
            if (!html) {
                html = '<div class="p-4 text-center text-secondary text-sm">No subdirectories</div>';
            }
            
            listContainer.innerHTML = html;
        } else {
            listContainer.innerHTML = `<div class="p-4 text-center text-red-400 text-sm">${data.error || 'Failed to browse'}</div>`;
        }
    } catch (error) {
        listContainer.innerHTML = `<div class="p-4 text-center text-red-400 text-sm">Error: ${error.message}</div>`;
    }
}

function browseParentDir() {
    const pathInput = document.getElementById('browserCurrentPath');
    const currentPath = pathInput.value;
    
    // Extract parent path
    const parts = currentPath.split('/').filter(p => p);
    if (parts.length > 0) {
        parts.pop();
        const parentPath = '/' + parts.join('/');
        browseTo(parentPath || '/');
    }
}

function refreshBrowser() {
    const pathInput = document.getElementById('browserCurrentPath');
    browseTo(pathInput.value);
}

function selectCurrentPath() {
    const pathInput = document.getElementById('browserCurrentPath');
    const selectedPath = pathInput.value;
    
    if (fileBrowserTarget === 'import') {
        document.getElementById('usbImportPath').value = selectedPath;
    } else {
        document.getElementById('usbExportPath').value = selectedPath;
    }
    
    closeFileBrowser();
}

function closeFileBrowser() {
    const modal = document.getElementById('fileBrowserModal');
    modal.style.display = 'none';
}

// Close modal when clicking outside
document.getElementById('fileBrowserModal')?.addEventListener('click', (e) => {
    if (e.target.id === 'fileBrowserModal') {
        closeFileBrowser();
    }
});

function toggleSidebar() {
    const sidebar = document.getElementById('sidebar');
    sidebar.classList.toggle('collapsed');
    
    // Save preference
    const isCollapsed = sidebar.classList.contains('collapsed');
    localStorage.setItem('sidebarCollapsed', isCollapsed);
}

function copyCodeToClipboard(button) {
    const code = decodeURIComponent(button.getAttribute('data-code'));
    navigator.clipboard.writeText(code).then(() => {
        const span = button.querySelector('span');
        const originalText = span.innerText;
        span.innerText = 'Copied!';
        setTimeout(() => {
            span.innerText = originalText;
        }, 2000);
    }).catch(err => {
        console.error('Failed to copy:', err);
    });
}

// Initialize sidebar state
document.addEventListener('DOMContentLoaded', () => {
    const sidebar = document.getElementById('sidebar');
    const isCollapsed = localStorage.getItem('sidebarCollapsed') === 'true';
    if (isCollapsed) {
        sidebar.classList.add('collapsed');
    }
    
    // Auto-collapse on mobile
    if (window.innerWidth < 768) {
        sidebar.classList.add('collapsed');
    }
});
