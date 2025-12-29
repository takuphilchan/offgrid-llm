// =====================================================
// P2P NETWORK MANAGEMENT
// Peer discovery, model sharing, and transfers
// =====================================================

// State
let p2pState = {
    enabled: false,
    peers: [],
    transfers: [],
    refreshInterval: null,
    nodeID: null,
    sharedModels: [],
    remoteModels: 0
};

// Initialize P2P module
function initP2P() {
    // Start periodic refresh when the P2P tab is opened
    refreshP2PStatus();
}

// Refresh P2P status from server
async function refreshP2PStatus() {
    const btn = document.getElementById('refreshPeersBtn');
    if (btn) {
        btn.disabled = true;
        btn.innerHTML = '<svg class="w-4 h-4 animate-spin" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M23 4v6h-6M1 20v-6h6"/><path d="M3.51 9a9 9 0 0114.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0020.49 15"/></svg> Refreshing...';
    }
    
    try {
        // Fetch P2P status (includes shared models info)
        const statusResp = await fetch('/v1/p2p/status');
        if (statusResp.ok) {
            const status = await statusResp.json();
            p2pState.enabled = status.enabled;
            p2pState.nodeID = status.node_id;
            p2pState.sharedModels = status.shared_models || [];
            p2pState.remoteModels = status.remote_models || 0;
        }
        
        // Fetch peers list
        const peersResp = await fetch('/v1/p2p/peers');
        if (peersResp.ok) {
            p2pState.peers = await peersResp.json() || [];
        } else if (peersResp.status === 403) {
            p2pState.enabled = false;
            p2pState.peers = [];
        }
        
        updateP2PStatus();
        renderPeers(p2pState.peers);
        renderSharedModels();
        
    } catch (error) {
        console.error('P2P refresh error:', error);
        p2pState.enabled = false;
        updateP2PStatus();
        renderPeers([]);
    } finally {
        if (btn) {
            btn.disabled = false;
            btn.innerHTML = '<svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M23 4v6h-6M1 20v-6h6"/><path d="M3.51 9a9 9 0 0114.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0020.49 15"/></svg> Refresh';
        }
    }
}

// Alias for backwards compatibility
function refreshPeers() {
    return refreshP2PStatus();
}

// Update P2P status display
function updateP2PStatus() {
    const statusBadge = document.getElementById('p2pStatusBadge');
    const peerCount = document.getElementById('p2pPeerCount');
    const modelCount = document.getElementById('p2pModelCount');
    const localCount = document.getElementById('p2pLocalCount');
    const nodeID = document.getElementById('p2pNodeID');
    
    if (statusBadge) {
        if (p2pState.enabled) {
            statusBadge.textContent = 'Enabled';
            statusBadge.className = 'badge badge-success text-xs';
        } else {
            statusBadge.textContent = 'Disabled';
            statusBadge.className = 'badge badge-secondary text-xs';
        }
    }
    
    if (peerCount) {
        peerCount.textContent = p2pState.peers.length;
    }
    
    if (modelCount) {
        modelCount.textContent = p2pState.remoteModels;
    }
    
    if (localCount) {
        localCount.textContent = p2pState.sharedModels.length;
    }
    
    if (nodeID && p2pState.nodeID) {
        nodeID.textContent = p2pState.nodeID;
    }
}

// Render shared models section
function renderSharedModels() {
    const container = document.getElementById('sharedModelsList');
    if (!container) return;
    
    if (!p2pState.enabled) {
        container.innerHTML = '<div class="text-sm text-secondary text-center py-4">P2P is disabled</div>';
        return;
    }
    
    if (!p2pState.sharedModels || p2pState.sharedModels.length === 0) {
        container.innerHTML = '<div class="text-sm text-secondary text-center py-4">No models to share. Download models first.</div>';
        return;
    }
    
    container.innerHTML = p2pState.sharedModels.map(model => `
        <div class="flex items-center justify-between p-2 bg-tertiary rounded-lg" id="shared-model-${escapeHtml(model).replace(/[^a-zA-Z0-9]/g, '-')}">
            <div class="flex items-center gap-2 flex-1 min-w-0">
                <svg class="w-4 h-4 text-accent flex-shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"></path>
                </svg>
                <div class="min-w-0">
                    <span class="text-sm font-medium block truncate">${escapeHtml(model)}</span>
                    <span class="text-xs text-secondary hash-display" id="hash-${escapeHtml(model).replace(/[^a-zA-Z0-9]/g, '-')}"></span>
                </div>
            </div>
            <div class="flex items-center gap-2 flex-shrink-0">
                <button onclick="verifyModel('${escapeHtml(model)}')" class="btn btn-secondary btn-xs" title="Verify integrity">
                    <svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"></path>
                    </svg>
                </button>
                <span class="badge badge-success text-xs">Shared</span>
            </div>
        </div>
    `).join('');
}

// Render peer list
function renderPeers(peers) {
    const container = document.getElementById('peersList');
    if (!container) return;
    
    if (!p2pState.enabled) {
        container.innerHTML = `
            <div class="empty-state py-8">
                <svg class="empty-state-icon text-secondary" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <circle cx="12" cy="12" r="10"></circle>
                    <line x1="4.93" y1="4.93" x2="19.07" y2="19.07"></line>
                </svg>
                <p>P2P is disabled</p>
                <span class="text-xs text-secondary">Start the server with --enable-p2p flag</span>
            </div>
        `;
        return;
    }
    
    if (!peers || peers.length === 0) {
        container.innerHTML = `
            <div class="empty-state py-8">
                <svg class="empty-state-icon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <circle cx="18" cy="5" r="3"></circle>
                    <circle cx="6" cy="12" r="3"></circle>
                    <circle cx="18" cy="19" r="3"></circle>
                </svg>
                <p>No peers discovered yet</p>
                <span class="text-xs text-secondary">Other OffGrid nodes on your network will appear here</span>
            </div>
        `;
        return;
    }
    
    container.innerHTML = peers.map(peer => renderPeerCard(peer)).join('');
}

// Render a single peer card
function renderPeerCard(peer) {
    const lastSeen = peer.LastSeen ? formatTimeAgo(new Date(peer.LastSeen)) : 'Unknown';
    const models = peer.Models || [];
    
    return `
        <div class="p-4 bg-tertiary rounded-lg border border-theme hover:border-accent/30 transition-colors">
            <div class="flex items-start justify-between mb-3">
                <div class="flex items-center gap-3">
                    <div class="w-10 h-10 rounded-full bg-accent/20 flex items-center justify-center">
                        <svg class="w-5 h-5 text-accent" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <rect x="2" y="3" width="20" height="14" rx="2" ry="2"></rect>
                            <line x1="8" y1="21" x2="16" y2="21"></line>
                            <line x1="12" y1="17" x2="12" y2="21"></line>
                        </svg>
                    </div>
                    <div>
                        <div class="font-medium text-primary">${escapeHtml(peer.ID)}</div>
                        <div class="text-xs text-secondary">${escapeHtml(peer.Address)}:${peer.Port}</div>
                    </div>
                </div>
                <div class="text-xs text-secondary flex items-center gap-1">
                    <svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <circle cx="12" cy="12" r="10"></circle>
                        <polyline points="12 6 12 12 16 14"></polyline>
                    </svg>
                    ${lastSeen}
                </div>
            </div>
            
            ${models.length > 0 ? `
                <div class="mb-3">
                    <div class="text-xs text-secondary mb-2">Available Models (${models.length})</div>
                    <div class="flex flex-wrap gap-1">
                        ${models.slice(0, 5).map(m => `
                            <span class="px-2 py-0.5 bg-secondary/50 rounded text-xs text-primary">${escapeHtml(m)}</span>
                        `).join('')}
                        ${models.length > 5 ? `<span class="px-2 py-0.5 text-xs text-secondary">+${models.length - 5} more</span>` : ''}
                    </div>
                </div>
            ` : `
                <div class="text-xs text-secondary mb-3">No models shared</div>
            `}
            
            <div class="flex gap-2">
                <button onclick="showPeerModels('${escapeHtml(peer.ID)}')" class="btn btn-secondary btn-sm flex-1">
                    <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"></path>
                    </svg>
                    View Models
                </button>
                <button onclick="downloadFromPeer('${escapeHtml(peer.ID)}')" class="btn btn-primary btn-sm flex-1" ${models.length === 0 ? 'disabled' : ''}>
                    <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"></path>
                        <polyline points="7 10 12 15 17 10"></polyline>
                        <line x1="12" y1="15" x2="12" y2="3"></line>
                    </svg>
                    Download
                </button>
            </div>
        </div>
    `;
}

// Show peer models in a modal
function showPeerModels(peerID) {
    const peer = p2pState.peers.find(p => p.ID === peerID);
    if (!peer) return;
    
    const models = peer.Models || [];
    
    // Create modal content
    const content = models.length === 0 
        ? '<p class="text-secondary">This peer has no models available for download.</p>'
        : `
            <div class="space-y-2 max-h-96 overflow-y-auto">
                ${models.map(m => `
                    <div class="p-3 bg-tertiary rounded-lg flex items-center justify-between">
                        <div class="font-medium">${escapeHtml(m)}</div>
                        <button onclick="requestModelDownload('${escapeHtml(peerID)}', '${escapeHtml(m)}')" class="btn btn-primary btn-sm">
                            Download
                        </button>
                    </div>
                `).join('')}
            </div>
        `;
    
    showInfoModal('Models from ' + peerID, content);
}

// Show download dialog for peer
function downloadFromPeer(peerID) {
    const peer = p2pState.peers.find(p => p.ID === peerID);
    if (!peer || !peer.Models || peer.Models.length === 0) {
        showToast('No models available from this peer', 'warning');
        return;
    }
    
    showPeerModels(peerID);
}

// Request model download from peer
async function requestModelDownload(peerID, modelPath) {
    try {
        showToast(`Starting download of ${modelPath} from ${peerID}...`, 'info');
        
        const response = await fetch('/v1/p2p/download', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                peer_id: peerID,
                model_path: modelPath
            })
        });
        
        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || 'Download failed');
        }
        
        const result = await response.json();
        showToast(result.message || 'Download started', 'success');
        
        // Close modal
        closeInfoModal();
        
        // Start tracking transfer
        trackTransfer(peerID, modelPath);
        
    } catch (error) {
        console.error('Download error:', error);
        showToast(error.message || 'Failed to start download', 'error');
    }
}

// Track active transfer
function trackTransfer(peerID, modelPath) {
    const transferID = `p2p-${peerID}-${modelPath}`;
    p2pState.transfers.push({
        id: transferID,
        peerID,
        modelPath,
        status: 'starting',
        percent: 0
    });
    
    updateTransfersList();
    
    // Poll for progress
    const pollProgress = async () => {
        try {
            const response = await fetch('/v1/downloads');
            if (response.ok) {
                const downloads = await response.json();
                const transfer = downloads[transferID];
                
                if (transfer) {
                    const idx = p2pState.transfers.findIndex(t => t.id === transferID);
                    if (idx >= 0) {
                        p2pState.transfers[idx].status = transfer.status;
                        p2pState.transfers[idx].percent = transfer.percent;
                        p2pState.transfers[idx].error = transfer.error;
                        updateTransfersList();
                        
                        if (transfer.status !== 'complete' && transfer.status !== 'failed') {
                            setTimeout(pollProgress, 1000);
                        } else if (transfer.status === 'complete') {
                            showToast(`Downloaded ${modelPath} successfully`, 'success');
                            // Refresh local models
                            if (typeof loadModels === 'function') {
                                loadModels();
                            }
                        }
                    }
                }
            }
        } catch (e) {
            console.error('Progress poll error:', e);
        }
    };
    
    setTimeout(pollProgress, 1000);
}

// Update transfers list display
function updateTransfersList() {
    const container = document.getElementById('p2pTransfersList');
    if (!container) return;
    
    const activeTransfers = p2pState.transfers.filter(t => 
        t.status !== 'complete' && t.status !== 'failed'
    );
    
    if (activeTransfers.length === 0) {
        container.innerHTML = '<div class="text-sm text-secondary text-center py-4">No active transfers</div>';
        return;
    }
    
    container.innerHTML = activeTransfers.map(t => `
        <div class="p-3 bg-tertiary rounded-lg">
            <div class="flex items-center justify-between mb-2">
                <div class="font-medium text-sm">${escapeHtml(t.modelPath)}</div>
                <span class="text-xs text-secondary">${t.status}</span>
            </div>
            <div class="h-2 bg-secondary/30 rounded-full overflow-hidden">
                <div class="h-full bg-accent rounded-full transition-all" style="width: ${t.percent}%"></div>
            </div>
            ${t.error ? `<div class="text-xs text-red-400 mt-1">${escapeHtml(t.error)}</div>` : ''}
        </div>
    `).join('');
}

// Format time ago
function formatTimeAgo(date) {
    const seconds = Math.floor((new Date() - date) / 1000);
    
    if (seconds < 60) return 'Just now';
    if (seconds < 3600) return Math.floor(seconds / 60) + 'm ago';
    if (seconds < 86400) return Math.floor(seconds / 3600) + 'h ago';
    return Math.floor(seconds / 86400) + 'd ago';
}

// Escape HTML to prevent XSS
function escapeHtml(str) {
    if (!str) return '';
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
}

// Show info modal (use existing modal system if available)
function showInfoModal(title, content) {
    // Check if there's an existing modal system
    if (typeof showModal === 'function') {
        showModal(title, content);
        return;
    }
    
    // Fallback: create simple modal
    let modal = document.getElementById('p2pInfoModal');
    if (!modal) {
        modal = document.createElement('div');
        modal.id = 'p2pInfoModal';
        modal.className = 'modal-backdrop';
        modal.innerHTML = `
            <div class="modal-box">
                <div class="modal-box-header">
                    <h3 id="p2pModalTitle"></h3>
                    <button onclick="closeInfoModal()" class="modal-close-btn">
                        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18 6L6 18M6 6l12 12"></path></svg>
                    </button>
                </div>
                <div class="modal-box-body" id="p2pModalContent"></div>
            </div>
        `;
        document.body.appendChild(modal);
    }
    
    document.getElementById('p2pModalTitle').textContent = title;
    document.getElementById('p2pModalContent').innerHTML = content;
    modal.classList.add('active');
}

// Close info modal
function closeInfoModal() {
    const modal = document.getElementById('p2pInfoModal');
    if (modal) {
        modal.classList.remove('active');
    }
}

// Verify model integrity and show hash
async function verifyModel(modelID) {
    const hashEl = document.getElementById('hash-' + modelID.replace(/[^a-zA-Z0-9]/g, '-'));
    if (hashEl) {
        hashEl.textContent = 'Verifying...';
        hashEl.className = 'text-xs text-secondary hash-display';
    }
    
    try {
        const response = await fetch('/v1/models/verify?model=' + encodeURIComponent(modelID));
        if (!response.ok) {
            throw new Error('Verification failed');
        }
        
        const result = await response.json();
        
        if (hashEl) {
            if (result.sha256) {
                hashEl.textContent = 'SHA256: ' + result.sha256.substring(0, 16) + '...';
                hashEl.className = 'text-xs text-accent hash-display';
                hashEl.title = 'SHA256: ' + result.sha256;
            } else if (result.message) {
                hashEl.textContent = result.message;
                hashEl.className = 'text-xs text-warning hash-display';
            }
        }
        
        if (result.verified) {
            showToast('Model verified: ' + modelID, 'success');
        } else {
            showToast(result.message || 'Verification incomplete', 'warning');
        }
        
    } catch (error) {
        console.error('Verification error:', error);
        if (hashEl) {
            hashEl.textContent = 'Verification failed';
            hashEl.className = 'text-xs text-red-400 hash-display';
        }
        showToast('Failed to verify model', 'error');
    }
}

// Auto-refresh when P2P tab is active
function onP2PTabActive() {
    refreshPeers();
    
    // Set up periodic refresh
    if (p2pState.refreshInterval) {
        clearInterval(p2pState.refreshInterval);
    }
    p2pState.refreshInterval = setInterval(refreshPeers, 30000);
}

function onP2PTabInactive() {
    if (p2pState.refreshInterval) {
        clearInterval(p2pState.refreshInterval);
        p2pState.refreshInterval = null;
    }
}

// Hook into tab switching
const originalSwitchTab = window.switchTab;
if (typeof originalSwitchTab === 'function') {
    window.switchTab = function(tabId) {
        // Call original
        originalSwitchTab(tabId);
        
        // Handle P2P tab activation
        if (tabId === 'peers') {
            onP2PTabActive();
        } else if (p2pState.refreshInterval) {
            onP2PTabInactive();
        }
    };
}

// Initialize on load
document.addEventListener('DOMContentLoaded', function() {
    // Check if P2P tab is initially visible
    const peersContent = document.getElementById('content-peers');
    if (peersContent && !peersContent.classList.contains('hidden')) {
        onP2PTabActive();
    }
});
