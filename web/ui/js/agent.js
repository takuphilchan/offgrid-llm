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

// ============================================
// CUSTOM TOOL CREATION
// ============================================

function showCreateToolModal() {
    document.getElementById('createToolModal').classList.add('show');
    document.getElementById('customToolName').value = '';
    document.getElementById('customToolDesc').value = '';
    document.getElementById('customToolType').value = 'shell';
    document.getElementById('customToolCommand').value = '';
    document.getElementById('customToolUrl').value = '';
    toggleToolTypeFields();
}

function hideCreateToolModal() {
    document.getElementById('createToolModal').classList.remove('show');
}

function toggleToolTypeFields() {
    const type = document.getElementById('customToolType').value;
    const shellFields = document.getElementById('shellToolFields');
    const httpFields = document.getElementById('httpToolFields');
    
    if (type === 'shell') {
        shellFields.classList.remove('hidden');
        httpFields.classList.add('hidden');
    } else {
        shellFields.classList.add('hidden');
        httpFields.classList.remove('hidden');
    }
}

function fillToolTemplate(name, description, type, commandOrUrl) {
    document.getElementById('customToolName').value = name;
    document.getElementById('customToolDesc').value = description;
    document.getElementById('customToolType').value = type;
    toggleToolTypeFields();
    
    if (type === 'shell') {
        document.getElementById('customToolCommand').value = commandOrUrl;
    } else {
        document.getElementById('customToolUrl').value = commandOrUrl;
    }
}

async function createCustomTool() {
    const name = document.getElementById('customToolName').value.trim();
    const description = document.getElementById('customToolDesc').value.trim();
    const type = document.getElementById('customToolType').value;
    const command = document.getElementById('customToolCommand').value.trim();
    const url = document.getElementById('customToolUrl').value.trim();
    
    // Validation
    if (!name) {
        showAlert('Please enter a tool name', { title: 'Missing Information', type: 'warning' });
        return;
    }
    
    if (!/^[a-z][a-z0-9_]*$/.test(name)) {
        showAlert('Tool name must start with a letter and contain only lowercase letters, numbers, and underscores', { title: 'Invalid Name', type: 'warning' });
        return;
    }
    
    if (!description) {
        showAlert('Please enter a description', { title: 'Missing Information', type: 'warning' });
        return;
    }
    
    if (type === 'shell' && !command) {
        showAlert('Please enter a shell command', { title: 'Missing Information', type: 'warning' });
        return;
    }
    
    if (type === 'http' && !url) {
        showAlert('Please enter a URL', { title: 'Missing Information', type: 'warning' });
        return;
    }
    
    try {
        const payload = {
            name,
            description,
            type,
            parameters: {}
        };
        
        if (type === 'shell') {
            payload.command = command;
        } else {
            payload.url = url;
        }
        
        const resp = await fetch('/v1/agents/tools', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });
        
        const data = await resp.json();
        
        if (resp.ok) {
            showAlert(`Custom tool "${name}" created successfully!`, { title: 'Tool Created', type: 'success' });
            hideCreateToolModal();
            loadAgentTools();
        } else {
            throw new Error(data.error || 'Failed to create tool');
        }
    } catch (e) {
        showAlert('Failed to create tool: ' + e.message, { title: 'Error', type: 'error' });
    }
}

// ============================================
// MULTI-AGENT ORCHESTRATION
// ============================================

const orchestrationModeDescriptions = {
    'sequential': 'Agents run one after another, passing context',
    'parallel': 'All agents work simultaneously on the task',
    'debate': 'Agents discuss and refine their answers',
    'voting': 'Agents vote on the best answer',
    'hierarchy': 'Supervisor delegates to specialist agents'
};

// Update mode description when selection changes
document.addEventListener('DOMContentLoaded', function() {
    const modeSelect = document.getElementById('orchestrationMode');
    if (modeSelect) {
        modeSelect.addEventListener('change', function() {
            const desc = document.getElementById('orchestrationModeDesc');
            if (desc) {
                desc.textContent = orchestrationModeDescriptions[this.value] || '';
            }
        });
    }
});

async function runOrchestration() {
    const task = document.getElementById('agentTask')?.value?.trim();
    if (!task) {
        showAlert('Please enter a task first', { type: 'warning' });
        return;
    }
    
    const mode = document.getElementById('orchestrationMode')?.value || 'sequential';
    const model = document.getElementById('agentModel')?.value;
    
    if (!model) {
        showAlert('Please select a model first', { type: 'warning' });
        return;
    }
    
    // Show progress
    const output = document.getElementById('agentOutput');
    const status = document.getElementById('agentStatus');
    
    output.innerHTML = `
        <div class="p-4">
            <div class="flex items-center gap-3 mb-4">
                <div class="animate-spin w-5 h-5 border-2 border-accent border-t-transparent rounded-full"></div>
                <span class="text-accent font-medium">Running multi-agent orchestration (${mode})...</span>
            </div>
            <div class="text-sm text-secondary">
                Multiple agents are collaborating on your task. This may take a few minutes.
            </div>
        </div>
    `;
    status.textContent = `Multi-agent: ${mode}`;
    
    // Define default agents based on mode
    const defaultAgents = [
        { name: 'Researcher', template: 'researcher', description: 'Research and analysis specialist' },
        { name: 'Coder', template: 'coder', description: 'Code implementation specialist' }
    ];
    
    // For debate/voting, add a third agent for more perspectives
    if (mode === 'debate' || mode === 'voting') {
        defaultAgents.push({ name: 'Analyst', template: 'analyst', description: 'Critical analysis specialist' });
    }
    
    // For hierarchy, mark first as supervisor
    if (mode === 'hierarchy') {
        defaultAgents[0].priority = 0;
        defaultAgents[1].priority = 1;
    }
    
    try {
        const resp = await fetch('/v1/agents/orchestrate', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                prompt: task,
                mode: mode,
                agents: defaultAgents,
                config: {
                    mode: mode,
                    max_rounds: 3,
                    voting_quorum: 0.5,
                    supervisor: mode === 'hierarchy' ? 'Researcher' : '',
                    final_aggregator: 'combine'
                }
            })
        });
        
        const result = await resp.json();
        
        if (result.error) {
            output.innerHTML = `
                <div class="p-4">
                    <div class="text-red-400 font-medium mb-2">Orchestration Error</div>
                    <div class="text-sm text-secondary">${result.error}</div>
                </div>
            `;
            status.textContent = 'Error';
            return;
        }
        
        // Display results
        output.innerHTML = renderOrchestrationResult(result);
        status.textContent = `Complete (${(result.total_duration / 1000000000).toFixed(1)}s)`;
        
    } catch (e) {
        output.innerHTML = `
            <div class="p-4">
                <div class="text-red-400 font-medium mb-2">Request Failed</div>
                <div class="text-sm text-secondary">${e.message}</div>
            </div>
        `;
        status.textContent = 'Error';
    }
}

function renderOrchestrationResult(result) {
    let html = '<div class="space-y-4 p-4">';
    
    // Mode badge
    html += `
        <div class="flex items-center gap-2 text-sm">
            <span class="px-2 py-1 rounded bg-accent/20 text-accent font-medium">${result.mode}</span>
            <span class="text-secondary">${result.agent_results?.length || 0} agents participated</span>
        </div>
    `;
    
    // Consensus for voting mode
    if (result.mode === 'voting' && result.consensus !== undefined) {
        const consensusPercent = Math.round(result.consensus * 100);
        html += `
            <div class="p-3 bg-tertiary rounded-lg">
                <div class="text-sm font-medium mb-1">Consensus: ${consensusPercent}%</div>
                <div class="w-full bg-gray-600 rounded-full h-2">
                    <div class="bg-accent h-2 rounded-full" style="width: ${consensusPercent}%"></div>
                </div>
            </div>
        `;
    }
    
    // Debate rounds
    if (result.debate_rounds && result.debate_rounds.length > 0) {
        html += '<div class="space-y-2">';
        result.debate_rounds.forEach(round => {
            html += `
                <div class="p-3 bg-tertiary rounded-lg">
                    <div class="text-xs text-secondary mb-2">Round ${round.round}</div>
                    <div class="space-y-2">
                        ${round.arguments.map(arg => `
                            <div class="border-l-2 border-accent/50 pl-3">
                                <div class="font-medium text-sm">${arg.agent_name}</div>
                                <div class="text-xs text-secondary mt-1">${arg.position.substring(0, 200)}...</div>
                            </div>
                        `).join('')}
                    </div>
                </div>
            `;
        });
        html += '</div>';
    }
    
    // Agent results (for non-debate modes)
    if (result.agent_results && result.agent_results.length > 0 && (!result.debate_rounds || result.debate_rounds.length === 0)) {
        html += '<div class="space-y-2">';
        result.agent_results.forEach(ar => {
            html += `
                <div class="p-3 bg-tertiary rounded-lg">
                    <div class="flex items-center justify-between mb-2">
                        <span class="font-medium text-sm">${ar.agent_name}</span>
                        <span class="text-xs text-secondary">${(ar.duration / 1000000000).toFixed(1)}s</span>
                    </div>
                    <div class="text-xs text-secondary">${ar.result.substring(0, 300)}...</div>
                </div>
            `;
        });
        html += '</div>';
    }
    
    // Final result
    if (result.final_result) {
        html += `
            <div class="p-4 bg-accent/10 border border-accent/30 rounded-lg">
                <div class="font-medium text-accent mb-2">Final Result</div>
                <div class="text-sm prose prose-invert max-w-none">
                    ${formatMarkdown(result.final_result)}
                </div>
            </div>
        `;
    }
    
    html += '</div>';
    return html;
}

function formatMarkdown(text) {
    // Simple markdown formatting
    if (!text) return '';
    return text
        .replace(/^### (.*$)/gm, '<h4 class="font-medium mt-3 mb-1">$1</h4>')
        .replace(/^## (.*$)/gm, '<h3 class="font-medium mt-4 mb-2">$1</h3>')
        .replace(/^# (.*$)/gm, '<h2 class="font-bold mt-4 mb-2">$1</h2>')
        .replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>')
        .replace(/\*(.*?)\*/g, '<em>$1</em>')
        .replace(/`(.*?)`/g, '<code class="px-1 py-0.5 bg-gray-700 rounded text-xs">$1</code>')
        .replace(/\n/g, '<br>');
}

// ============================================
// MCP MARKETPLACE FUNCTIONS
// ============================================

let mcpMarketplaceData = [];

async function showMCPMarketplace() {
    document.getElementById('mcpMarketplaceModal').classList.add('show');
    await loadMCPMarketplace();
}

function hideMCPMarketplace() {
    document.getElementById('mcpMarketplaceModal').classList.remove('show');
}

async function loadMCPMarketplace(category = '', search = '') {
    try {
        let url = '/v1/agents/mcp/marketplace';
        const params = new URLSearchParams();
        if (category) params.append('category', category);
        if (search) params.append('search', search);
        if (params.toString()) url += '?' + params.toString();
        
        const resp = await fetch(url);
        const data = await resp.json();
        
        mcpMarketplaceData = data.servers || [];
        document.getElementById('mcpMarketplaceInstalledCount').textContent = data.installed_count || 0;
        
        renderMCPMarketplace();
    } catch (e) {
        console.error('Failed to load MCP marketplace:', e);
        document.getElementById('mcpMarketplaceList').innerHTML = 
            '<div class="text-center py-8 text-red-400">Failed to load servers</div>';
    }
}

function renderMCPMarketplace() {
    const container = document.getElementById('mcpMarketplaceList');
    if (!mcpMarketplaceData || mcpMarketplaceData.length === 0) {
        container.innerHTML = '<div class="col-span-2 text-center py-8 text-secondary">No servers found</div>';
        return;
    }
    
    container.innerHTML = mcpMarketplaceData.map(server => `
        <div class="p-4 bg-secondary rounded-xl border border-transparent hover:border-accent/30 transition-colors">
            <div class="flex items-start justify-between mb-2">
                <div>
                    <h4 class="font-medium text-sm">${server.name}</h4>
                    <span class="text-xs px-1.5 py-0.5 bg-tertiary rounded">${server.category}</span>
                </div>
                ${server.installed 
                    ? `<span class="text-xs px-2 py-1 bg-green-500/20 text-green-400 rounded-full">Installed</span>`
                    : `<button onclick="installMCPServer('${server.id}')" class="btn btn-primary btn-sm">Install</button>`
                }
            </div>
            <p class="text-xs text-secondary mb-2 line-clamp-2">${server.description || ''}</p>
            <div class="flex flex-wrap gap-1 mt-2">
                ${(server.tags || []).slice(0, 3).map(tag => 
                    `<span class="text-xs px-1.5 py-0.5 bg-tertiary rounded">${tag}</span>`
                ).join('')}
            </div>
            ${server.tools && server.tools.length > 0 ? `
                <div class="text-xs text-secondary mt-2">
                    <span class="text-accent">${server.tools.length}</span> tools: ${server.tools.slice(0, 3).join(', ')}${server.tools.length > 3 ? '...' : ''}
                </div>
            ` : ''}
            ${server.installed ? `
                <div class="flex gap-2 mt-3">
                    <button onclick="configureMCPServer('${server.id}')" class="btn btn-secondary btn-sm text-xs">Configure</button>
                    <button onclick="uninstallMCPServer('${server.id}')" class="btn btn-danger btn-sm text-xs">Uninstall</button>
                </div>
            ` : ''}
        </div>
    `).join('');
}

function searchMCPMarketplace(query) {
    loadMCPMarketplace('', query);
}

function filterMCPMarketplace(category) {
    loadMCPMarketplace(category, '');
}

async function installMCPServer(serverId) {
    try {
        // Check if server needs environment variables
        const serverResp = await fetch(`/v1/agents/mcp/marketplace/${serverId}`);
        const serverData = await serverResp.json();
        
        let env = {};
        if (serverData.server && serverData.server.env) {
            // Server needs API keys - prompt user
            const envKeys = Object.keys(serverData.server.env);
            if (envKeys.length > 0) {
                for (const key of envKeys) {
                    const value = prompt(`Enter value for ${key}:`, '');
                    if (value !== null && value !== '') {
                        env[key] = value;
                    }
                }
            }
        }
        
        const resp = await fetch(`/v1/agents/mcp/marketplace/${serverId}/install`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ env })
        });
        const data = await resp.json();
        
        if (data.success) {
            showToast && showToast(`Installed ${serverId}`, 'success');
            await loadMCPMarketplace();
            await loadAgentTools(); // Refresh tools
        } else {
            showToast && showToast(data.error || 'Installation failed', 'error');
        }
    } catch (e) {
        console.error('Failed to install MCP server:', e);
        showToast && showToast('Installation failed: ' + e.message, 'error');
    }
}

async function uninstallMCPServer(serverId) {
    if (!confirm(`Uninstall ${serverId}?`)) return;
    
    try {
        const resp = await fetch(`/v1/agents/mcp/marketplace/${serverId}/uninstall`, {
            method: 'POST'
        });
        const data = await resp.json();
        
        if (data.success) {
            showToast && showToast(`Uninstalled ${serverId}`, 'success');
            await loadMCPMarketplace();
            await loadAgentTools();
        } else {
            showToast && showToast(data.error || 'Uninstall failed', 'error');
        }
    } catch (e) {
        console.error('Failed to uninstall MCP server:', e);
    }
}

async function configureMCPServer(serverId) {
    try {
        const resp = await fetch(`/v1/agents/mcp/marketplace/${serverId}`);
        const data = await resp.json();
        
        if (!data.config || !data.config.env) {
            showToast && showToast('No configuration options', 'info');
            return;
        }
        
        // Simple prompt-based configuration
        const env = data.config.env || {};
        const envKeys = Object.keys(env);
        
        for (const key of envKeys) {
            const currentValue = env[key] || '';
            const newValue = prompt(`${key}:`, currentValue);
            if (newValue !== null && newValue !== currentValue) {
                env[key] = newValue;
            }
        }
        
        // Save updated env
        await fetch(`/v1/agents/mcp/marketplace/${serverId}/configure`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ env })
        });
        
        showToast && showToast('Configuration saved', 'success');
    } catch (e) {
        console.error('Failed to configure MCP server:', e);
    }
}

