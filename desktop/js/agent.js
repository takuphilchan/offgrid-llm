// ============================================
// AGENT FUNCTIONS
// ============================================

let agentAbortController = null;

async function loadAgentModels() {
    // Use ModelManager if available (no redundant API calls)
    if (typeof ModelManager !== 'undefined') {
        await ModelManager.populateLLMSelect('agentModel', handleAgentModelChange);
        console.log('[AGENT] Loaded models via ModelManager');
        return;
    }
    
    // Legacy fallback
    try {
        const resp = await fetch('/models');
        const data = await resp.json();
        const allModels = data.data || [];
        
        const select = document.getElementById('agentModel');
        if (!select) {
            console.error('[AGENT] agentModel select not found');
            return;
        }
        
        select.innerHTML = '';
        
        const llmModels = allModels.filter(m => 
            m.type !== 'embedding' &&
            !m.id.toLowerCase().includes('embed') && 
            !m.id.toLowerCase().includes('minilm') &&
            !m.id.toLowerCase().includes('bge')
        );
        
        if (llmModels.length === 0) {
            select.innerHTML = '<option value="">No models available</option>';
            return;
        }
        
        llmModels.forEach((m) => {
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
        } else if (llmModels.length > 0) {
            select.value = llmModels[0].id;
        }
        
        console.log('[AGENT] Loaded', llmModels.length, 'models, selected:', select.value);
    } catch (e) {
        console.error('Failed to load agent models:', e);
        const select = document.getElementById('agentModel');
        if (select) {
            select.innerHTML = '<option value="">Error loading models</option>';
        }
    }
}

async function loadAgentTools() {
    try {
        const resp = await fetch('/v1/agents/tools?all=true');
        const data = await resp.json();
        const tools = data.tools || [];
        const list = document.getElementById('agentToolsList');
        const count = document.getElementById('toolCount');
        
        if (tools.length === 0) {
            list.innerHTML = '<p class="text-secondary text-sm text-center py-4">No tools available</p>';
            count.textContent = '0';
            return;
        }
        
        count.textContent = data.enabled_count || tools.filter(t => t.enabled).length;
        list.innerHTML = tools.map(t => `
            <div class="p-2 bg-tertiary rounded-lg flex items-start justify-between gap-2 ${!t.enabled ? 'opacity-50' : ''}">
                <div class="flex-1 min-w-0">
                    <div class="font-medium text-sm flex items-center gap-2 flex-wrap">
                        <span>${t.name}</span>
                        ${t.source && t.source !== 'builtin' ? `<span class="text-xs px-1 py-0.5 rounded bg-blue-500/20 text-blue-400">${t.source}</span>` : ''}
                    </div>
                    <div class="text-xs text-secondary mt-1">${t.description || 'No description'}</div>
                </div>
                <label class="relative inline-flex items-center cursor-pointer flex-shrink-0">
                    <input type="checkbox" class="sr-only peer" ${t.enabled ? 'checked' : ''} onchange="toggleTool('${t.name}', this.checked)">
                    <div class="w-9 h-5 bg-gray-600 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-accent"></div>
                </label>
            </div>
        `).join('');
    } catch (e) {
        console.error('Failed to load tools:', e);
    }
}

async function toggleTool(name, enabled) {
    try {
        const resp = await fetch('/v1/agents/tools', {
            method: 'PATCH',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name, enabled })
        });
        const data = await resp.json();
        if (resp.ok) {
            document.getElementById('toolCount').textContent = data.enabled_count;
            // Re-render with updated state
            loadAgentTools();
        } else {
            showAlert(data.error || 'Failed to toggle tool', { type: 'error' });
        }
    } catch (e) {
        showAlert('Failed to toggle tool: ' + e.message, { type: 'error' });
    }
}

async function loadMCPServers() {
    try {
        const resp = await fetch('/v1/agents/mcp');
        const data = await resp.json();
        const servers = data.servers || [];
        
        const list = document.getElementById('mcpServersList');
        if (servers.length === 0) {
            list.innerHTML = '<p class="text-secondary text-sm text-center py-4">No MCP servers configured</p>';
            return;
        }
        
        list.innerHTML = servers.map(s => `
            <div class="flex items-center justify-between p-2 bg-tertiary rounded-lg">
                <div class="flex items-center gap-2">
                    <span class="w-2 h-2 rounded-full ${s.status === 'connected' ? 'bg-emerald-500' : 'bg-red-500'}"></span>
                    <div>
                        <div class="text-sm font-medium">${s.name}</div>
                        <div class="text-xs text-secondary">${s.transport} • ${s.tools} tools</div>
                    </div>
                </div>
                <button onclick="removeMCPServer('${s.name}')" class="text-red-400 hover:text-red-300 p-1" title="Remove Server">
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18 6L6 18M6 6l12 12"></path></svg>
                </button>
            </div>
        `).join('');
    } catch (e) {
        console.error('Failed to load MCP servers:', e);
    }
}

async function removeMCPServer(name) {
    showConfirm(`Are you sure you want to remove the MCP server "${name}"?`, async () => {
        try {
            // Note: DELETE endpoint not yet implemented in backend, so this is a placeholder
            // In a real implementation, we would call:
            // await fetch('/v1/agents/mcp?name=' + name, { method: 'DELETE' });
            showAlert('Removing MCP servers is not yet supported in this version.', { type: 'info' });
        } catch (e) {
            showAlert('Failed to remove server: ' + e.message, { type: 'error' });
        }
    }, { title: 'Remove MCP Server?', type: 'warning', confirmText: 'Remove', cancelText: 'Cancel' });
}

async function runAgent() {
    const model = document.getElementById('agentModel').value;
    const task = document.getElementById('agentTask').value;
    const style = document.getElementById('agentStyle').value;
    const maxSteps = parseInt(document.getElementById('agentMaxSteps').value) || 10;
    
    if (!model || !task) {
        showAlert('Please select a model and enter a task', { title: 'Missing Information', type: 'warning' });
        return;
    }
    
    // Reset token buffer
    tokenBuffer = '';
    tokenContainer = null;
    
    // Reset duplicate content tracking
    displayedContents = new Set();
    lastStepId = -1;
    
    document.getElementById('runAgentBtn').classList.add('hidden');
    document.getElementById('stopAgentBtn').classList.remove('hidden');
    document.getElementById('agentOutput').innerHTML = '';
    updateAgentStatus('Running...', 'running');
    
    agentAbortController = new AbortController();
    
    try {
        const resp = await fetch('/v1/agents/run', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ model, prompt: task, style, max_steps: maxSteps, stream: true }),
            signal: agentAbortController.signal
        });
        
        const reader = resp.body.getReader();
        const decoder = new TextDecoder();
        let output = document.getElementById('agentOutput');
        output.innerHTML = '';
        
        while (true) {
            const { done, value } = await reader.read();
            if (done) break;
            
            const text = decoder.decode(value);
            const lines = text.split('\n');
            
            for (const line of lines) {
                if (line.startsWith('data: ')) {
                    const data = line.substring(6);
                    if (data === '[DONE]') continue;
                    
                    try {
                        const step = JSON.parse(data);
                        appendAgentStep(step);
                        // Update status based on step type
                        if (step.step_type === 'done' || step.type === 'done') {
                            updateAgentStatus('Completed', 'success');
                        } else if (step.step_type === 'error' || step.type === 'error') {
                            updateAgentStatus('Error', 'error');
                        } else if (step.step_type === 'action' || step.type === 'action') {
                            updateAgentStatus(`Executing: ${step.tool_name || 'action'}`, 'running');
                        } else if (step.step_type === 'thought' || step.type === 'thought') {
                            updateAgentStatus('Thinking...', 'running');
                        }
                    } catch (e) { /* Partial streaming data - ignore */ }
                }
            }
        }
    } catch (e) {
        if (e.name !== 'AbortError') {
            document.getElementById('agentOutput').innerHTML += `<div class="agent-step agent-step-error"><div class="agent-step-header"><span class="agent-step-indicator"></span><span class="agent-step-label">Error</span></div><div class="agent-step-content">${e.message}</div></div>`;
            updateAgentStatus('Error', 'error');
        } else {
            updateAgentStatus('Stopped', 'idle');
        }
    } finally {
        document.getElementById('runAgentBtn').classList.remove('hidden');
        document.getElementById('stopAgentBtn').classList.add('hidden');
        agentAbortController = null;
    }
}

// Update agent status display
function updateAgentStatus(text, state) {
    const el = document.getElementById('agentStatus');
    if (!el) return;
    el.textContent = text;
    el.className = 'text-xs';
    if (state === 'running') {
        el.classList.add('text-emerald-400'); // Green for running
    } else if (state === 'success') {
        el.classList.add('text-emerald-400');
    } else if (state === 'error') {
        el.classList.add('text-red-400');
    } else {
        el.classList.add('text-secondary');
    }
}

async function showAgentHistory() {
    document.getElementById('agentHistoryModal').classList.add('active');
    
    const tbody = document.getElementById('agentHistoryList');
    tbody.innerHTML = '<tr><td colspan="5" class="p-4 text-center text-secondary">Loading...</td></tr>';
    
    try {
        const resp = await fetch('/v1/agents/tasks');
        const data = await resp.json();
        const tasks = data.tasks || [];
        
        if (tasks.length === 0) {
            tbody.innerHTML = '<tr><td colspan="5" class="p-4 text-center text-secondary">No history found</td></tr>';
            return;
        }
        
        // Sort by created_at desc
        tasks.sort((a, b) => new Date(b.created_at) - new Date(a.created_at));
        
        tbody.innerHTML = tasks.map(t => `
            <tr class="border-b border-theme hover:bg-tertiary/50">
                <td class="p-3 font-mono text-xs text-secondary">${t.id.substring(0, 8)}...</td>
                <td class="p-3" title="${t.prompt}">${t.prompt}</td>
                <td class="p-3">
                    <span class="px-2 py-0.5 rounded text-xs ${
                        t.status === 'completed' ? 'bg-emerald-500/20 text-emerald-400' :
                        t.status === 'failed' ? 'bg-red-500/20 text-red-400' :
                        'bg-blue-500/20 text-blue-400'
                    }">${t.status}</span>
                </td>
                <td class="p-3 text-xs">${t.steps_count || 0}</td>
                <td class="p-3 text-xs text-secondary">${new Date(t.created_at).toLocaleString()}</td>
            </tr>
        `).join('');
    } catch (e) {
        tbody.innerHTML = `<tr><td colspan="5" class="p-4 text-center text-red-400">Error: ${e.message}</td></tr>`;
    }
}

function hideAgentHistory() {
    document.getElementById('agentHistoryModal').classList.remove('active');
}


// Token aggregator for streaming
let tokenBuffer = '';
let tokenContainer = null;
let lastStepId = -1;

// Clean up messy agent output - remove tool descriptions that leak into thoughts
function cleanAgentOutput(text) {
    if (!text) return '';
    
    // Remove tool description blocks that the model echoes
    let cleaned = text;
    
    // Remove lines starting with "### tool_name" (tool headers)
    cleaned = cleaned.replace(/^###\s+\w+.*$/gm, '');
    
    // Remove "Description:" lines
    cleaned = cleaned.replace(/^Description:.*$/gm, '');
    
    // Remove "Parameters:" blocks
    cleaned = cleaned.replace(/^Parameters:[\s\S]*?(?=\n\n|Thought:|Action:|Answer:|$)/gm, '');
    
    // Remove "  - param (type):" lines
    cleaned = cleaned.replace(/^\s+-\s+\w+\s+\(\w+\):.*$/gm, '');
    
    // Remove "Required:" lines
    cleaned = cleaned.replace(/^Required:.*$/gm, '');
    
    // Remove "Action Input:" lines (model echoing examples)
    cleaned = cleaned.replace(/^Action Input:\s*\{.*\}$/gm, '');
    cleaned = cleaned.replace(/^Action Input:\s*$/gm, '');
    
    // Remove "Action:" lines that aren't tool calls
    cleaned = cleaned.replace(/^Action:\s*\w+\s*$/gm, '');
    
    // Remove "Observation:" prefix if followed by same content
    cleaned = cleaned.replace(/^Observation:\s*/gm, '');
    
    // Remove "Thought:" prefix - we already label it as Thought
    cleaned = cleaned.replace(/^Thought:\s*/gm, '');
    
    // Remove multiple newlines
    cleaned = cleaned.replace(/\n{3,}/g, '\n\n');
    
    // Trim
    return cleaned.trim();
}

// Extract just the actual thought from messy output
function extractThought(text) {
    if (!text) return '';
    
    // Look for content between "Thought:" and next section
    const thoughtMatch = text.match(/(?:^|Thought:\s*)([^]*?)(?=Action:|Answer:|Observation:|Action Input:|$)/i);
    if (thoughtMatch && thoughtMatch[1]) {
        let thought = thoughtMatch[1].trim();
        // Clean remaining noise
        return cleanAgentOutput(thought);
    }
    
    // If no explicit Thought:, clean the whole text
    return cleanAgentOutput(text);
}

// Track displayed content to avoid duplicates
let displayedContents = new Set();

function appendAgentStep(step) {
    const output = document.getElementById('agentOutput');
    const stepType = step.step_type || step.type || 'unknown';
    const stepId = step.step_id || step.id;
    
    // Handle token events - aggregate into a single streaming container
    if (stepType === 'token') {
        tokenBuffer += step.token || '';
        if (!tokenContainer) {
            tokenContainer = document.createElement('div');
            tokenContainer.className = 'agent-step agent-step-streaming';
            tokenContainer.id = 'token-stream';
            tokenContainer.innerHTML = `
                <div class="agent-step-header">
                    <span class="agent-step-indicator streaming"></span>
                    <span class="agent-step-label">Thinking...</span>
                </div>
                <div class="agent-step-content" id="token-content"></div>
            `;
            output.appendChild(tokenContainer);
        }
        document.getElementById('token-content').textContent = tokenBuffer;
        output.scrollTop = output.scrollHeight;
        return;
    }
    
    // When we get a real step, finalize the token buffer and remove streaming container
    if (tokenContainer && tokenBuffer.trim()) {
        // Clean and format the streamed content
        const cleanedContent = extractThought(tokenBuffer);
        if (cleanedContent && !displayedContents.has(cleanedContent.substring(0, 100))) {
            // Track this content to avoid showing it again
            displayedContents.add(cleanedContent.substring(0, 100));
            // Keep the content but change the style to a finalized thought
            tokenContainer.className = 'agent-step agent-step-thought';
            tokenContainer.innerHTML = `
                <div class="agent-step-header">
                    <span class="agent-step-indicator"></span>
                    <span class="agent-step-label">Reasoning</span>
                </div>
                <div class="agent-step-content">${escapeHtml(cleanedContent)}</div>
            `;
        } else {
            // Content was all noise or duplicate, remove it
            tokenContainer.remove();
        }
    } else if (tokenContainer) {
        // Remove empty streaming container
        tokenContainer.remove();
    }
    
    // Reset token state
    tokenBuffer = '';
    tokenContainer = null;
    
    // Skip status updates - they clutter the output
    if (stepType === 'status') return;
    
    // Skip duplicate steps with same ID
    if (stepId !== undefined && stepId === lastStepId) return;
    lastStepId = stepId;
    
    const stepDiv = document.createElement('div');
    
    const typeConfig = {
        'thought': { class: 'thought', label: 'Thought' },
        'action': { class: 'action', label: 'Action' },
        'observation': { class: 'observation', label: 'Observation' },
        'answer': { class: 'answer', label: 'Answer' },
        'step': { class: 'step', label: 'Step' },
        'done': { class: 'complete', label: 'Complete' },
        'error': { class: 'error', label: 'Error' }
    };
    
    const config = typeConfig[stepType] || { class: 'default', label: stepType };
    
    stepDiv.className = `agent-step agent-step-${config.class}`;
    
    let content = step.content || step.output || '';
    if (step.error) content = step.error;
    
    // Clean thought content to remove tool descriptions that leak through
    if (stepType === 'thought' || stepType === 'step') {
        content = extractThought(content);
    } else {
        content = cleanAgentOutput(content);
    }
    
    // Skip empty steps after cleaning
    if (!content && !step.tool_name && !step.tool_result) return;
    
    // Skip duplicate content (Reasoning already showed it from streaming)
    const contentKey = content.substring(0, 100);
    if (stepType === 'thought' && displayedContents.has(contentKey)) {
        return; // Already shown as Reasoning from streaming
    }
    if (content) {
        displayedContents.add(contentKey);
    }
    
    content = escapeHtml(content);
    
    let html = `
        <div class="agent-step-header">
            <span class="agent-step-indicator"></span>
            <span class="agent-step-label">${config.label}</span>
            ${stepId !== undefined ? `<span class="agent-step-number">#${stepId}</span>` : ''}
            ${step.tool_name ? `<span class="agent-step-tool">${step.tool_name}</span>` : ''}
        </div>
        <div class="agent-step-content">${content}</div>
    `;
    
    // Add tool arguments if present
    if (step.tool_args && Object.keys(step.tool_args).length > 0) {
        const argsStr = typeof step.tool_args === 'string' ? step.tool_args : JSON.stringify(step.tool_args, null, 2);
        html += `<div class="agent-step-meta">
            <div class="agent-step-meta-label">Input</div>
            <pre class="agent-step-code">${escapeHtml(argsStr)}</pre>
        </div>`;
    }
    
    // Add tool result if present  
    if (step.tool_result) {
        const resultStr = typeof step.tool_result === 'string' ? step.tool_result : JSON.stringify(step.tool_result, null, 2);
        html += `<div class="agent-step-meta">
            <div class="agent-step-meta-label">Output</div>
            <pre class="agent-step-code result">${escapeHtml(resultStr)}</pre>
        </div>`;
    }
    
    stepDiv.innerHTML = html;
    output.appendChild(stepDiv);
    output.scrollTop = output.scrollHeight;
}

function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

function stopAgent() {
    if (agentAbortController) {
        agentAbortController.abort();
    }
}

function showAddMCPModal() {
    document.getElementById('addMCPModal').classList.add('active');
    document.getElementById('mcpServerName').value = '';
    document.getElementById('mcpServerUrl').value = '';
}

function hideAddMCPModal() {
    document.getElementById('addMCPModal').classList.remove('active');
    // Reset connection status
    document.getElementById('mcpConnectionStatus').classList.add('hidden');
}

function fillMCPExample(name, command) {
    document.getElementById('mcpServerName').value = name;
    document.getElementById('mcpServerUrl').value = command;
    // Reset connection status when changing server
    document.getElementById('mcpConnectionStatus').classList.add('hidden');
}

function showMCPStatus(status, message) {
    const statusDiv = document.getElementById('mcpConnectionStatus');
    const indicator = document.getElementById('mcpStatusIndicator');
    const text = document.getElementById('mcpStatusText');
    
    statusDiv.classList.remove('hidden', 'bg-green-500/10', 'bg-red-500/10', 'bg-yellow-500/10');
    indicator.classList.remove('bg-green-500', 'bg-red-500', 'bg-yellow-500', 'animate-pulse');
    
    if (status === 'testing') {
        statusDiv.classList.add('bg-yellow-500/10');
        indicator.classList.add('bg-yellow-500', 'animate-pulse');
    } else if (status === 'success') {
        statusDiv.classList.add('bg-green-500/10');
        indicator.classList.add('bg-green-500');
    } else {
        statusDiv.classList.add('bg-red-500/10');
        indicator.classList.add('bg-red-500');
    }
    
    text.textContent = message;
}

async function testMCPConnection() {
    const urlOrCommand = document.getElementById('mcpServerUrl').value;
    
    if (!urlOrCommand) {
        showAlert('Please enter a server URL or command first', { title: 'Missing Input', type: 'warning' });
        return;
    }
    
    const isCommand = urlOrCommand.startsWith('npx ') || urlOrCommand.startsWith('node ') || 
                      urlOrCommand.startsWith('python ') || urlOrCommand.startsWith('./') || 
                      urlOrCommand.startsWith('/');
    
    showMCPStatus('testing', isCommand ? 'Starting server and testing...' : 'Testing connection...');
    
    try {
        const resp = await fetch('/v1/agents/mcp/test', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ url: urlOrCommand })
        });
        
        const data = await resp.json();
        
        if (resp.ok) {
            showMCPStatus('success', `Connected! Found ${data.tools_count || 0} tools available.`);
        } else {
            showMCPStatus('error', data.error || 'Connection failed');
        }
    } catch (e) {
        showMCPStatus('error', 'Connection failed: ' + e.message);
    }
}

async function addMCPServer() {
    const name = document.getElementById('mcpServerName').value;
    const url = document.getElementById('mcpServerUrl').value;
    
    if (!name || !url) {
        showAlert('Please fill in all fields', { title: 'Missing Information', type: 'warning' });
        return;
    }
    
    try {
        // Add to tools.json config
        const resp = await fetch('/v1/agents/mcp', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name, url })
        });
        
        if (resp.ok) {
            const data = await resp.json();
            showAlert(`MCP server "${name}" added successfully!<br><br>Found ${data.tools_added || 0} tools from this server.`, { title: 'Server Added', type: 'success' });
            hideAddMCPModal();
            loadAgentTools();
            loadMCPServers();
        } else {
            const err = await resp.json();
            throw new Error(err.error || 'Failed to add server');
        }
    } catch (e) {
        showAlert('Failed to add MCP server: ' + e.message + '<br><br>Make sure the MCP server is running first. Use the "Test Connection" button to verify.', { title: 'Error', type: 'error' });
    }
}

