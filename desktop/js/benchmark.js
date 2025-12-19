// ==================== Benchmark Functions ====================

let benchmarkHistory = JSON.parse(localStorage.getItem('offgrid_benchmarks') || '[]');

async function loadBenchmarkModels() {
    try {
        const response = await fetch('/v1/models');
        const data = await response.json();
        const select = document.getElementById('benchmarkModel');
        select.innerHTML = '<option value="">Choose a model...</option>';
        
        // Filter for LLM models (not embeddings)
        const llmModels = data.data.filter(m => 
            !m.id.toLowerCase().includes('embed') && 
            !m.id.toLowerCase().includes('minilm') &&
            !m.id.toLowerCase().includes('bge')
        );
        
        llmModels.forEach(model => {
            const option = document.createElement('option');
            option.value = model.id;
            option.textContent = model.id;
            select.appendChild(option);
        });
    } catch (e) {
        console.error('Failed to load models:', e);
    }
}

async function loadSystemInfo() {
    try {
        const response = await fetch('/v1/system/info');
        const data = await response.json();
        
        // Format CPU - truncate if too long
        let cpuName = data.cpu || 'Unknown';
        if (cpuName.length > 40) {
            cpuName = cpuName.substring(0, 37) + '...';
        }
        document.getElementById('sysInfoCPU').textContent = cpuName;
        document.getElementById('sysInfoCPU').title = data.cpu || 'Unknown'; // Full name on hover
        
        // Format RAM
        if (data.total_memory) {
            const totalGB = (data.total_memory / 1024 / 1024 / 1024).toFixed(1);
            const freeGB = data.free_memory ? (data.free_memory / 1024 / 1024 / 1024).toFixed(1) : '?';
            document.getElementById('sysInfoRAM').textContent = `${totalGB} GB`;
            document.getElementById('sysInfoRAM').title = `${freeGB} GB available`;
        } else {
            document.getElementById('sysInfoRAM').textContent = 'Unknown';
        }
        
        // Format GPU
        let gpuName = data.gpu || 'CPU only';
        if (gpuName.length > 30) {
            gpuName = gpuName.substring(0, 27) + '...';
        }
        document.getElementById('sysInfoGPU').textContent = gpuName;
        if (data.gpu_memory) {
            const gpuMemGB = (data.gpu_memory / 1024 / 1024 / 1024).toFixed(1);
            document.getElementById('sysInfoGPU').title = `${data.gpu} (${gpuMemGB} GB VRAM)`;
        } else {
            document.getElementById('sysInfoGPU').title = data.gpu || 'CPU only';
        }
        
        // Backend
        document.getElementById('sysInfoBackend').textContent = data.backend || 'llama.cpp';
        
    } catch (e) {
        console.error('Failed to load system info:', e);
        document.getElementById('sysInfoCPU').textContent = 'Error loading';
        document.getElementById('sysInfoRAM').textContent = 'Error loading';
        document.getElementById('sysInfoGPU').textContent = 'Error loading';
    }
}

async function runBenchmark() {
    const model = document.getElementById('benchmarkModel').value;
    if (!model) {
        showAlert('Please select a model first', 'warning');
        return;
    }
    
    const promptLength = document.getElementById('benchmarkPromptLength').value;
    const outputTokens = parseInt(document.getElementById('benchmarkOutputTokens').value);
    
    const btn = document.getElementById('benchmarkRunBtn');
    const progress = document.getElementById('benchmarkProgress');
    btn.disabled = true;
    btn.innerHTML = '<svg class="w-4 h-4 mr-2 animate-spin" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path></svg>Running...';
    progress.classList.remove('hidden');
    
    // Generate test prompt based on length
    let prompt = '';
    switch (promptLength) {
        case 'short':
            prompt = 'Write a haiku about programming.';
            break;
        case 'medium':
            prompt = 'Explain the concept of recursion in programming. Include an example and discuss when it should be used versus iteration.';
            break;
        case 'long':
            prompt = 'Write a comprehensive guide on building a REST API. Cover the following topics: 1) What is REST and its principles, 2) HTTP methods and status codes, 3) Authentication and authorization, 4) Error handling, 5) Best practices for API design. Include code examples where appropriate.';
            break;
    }
    
    const startTime = performance.now();
    let firstTokenTime = null;
    let tokenCount = 0;
    
    try {
        const response = await fetch('/v1/chat/completions', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                model: model,
                messages: [{ role: 'user', content: prompt }],
                max_tokens: outputTokens,
                stream: true
            })
        });
        
        const reader = response.body.getReader();
        const decoder = new TextDecoder();
        
        while (true) {
            const { value, done } = await reader.read();
            if (done) break;
            
            const chunk = decoder.decode(value);
            const lines = chunk.split('\n');
            
            for (const line of lines) {
                if (line.startsWith('data: ') && line !== 'data: [DONE]') {
                    try {
                        const data = JSON.parse(line.slice(6));
                        if (data.choices && data.choices[0].delta && data.choices[0].delta.content) {
                            if (!firstTokenTime) {
                                firstTokenTime = performance.now();
                            }
                            tokenCount++;
                            
                            // Update progress
                            const percent = Math.min((tokenCount / outputTokens) * 100, 100);
                            document.getElementById('benchmarkProgressBar').style.width = percent + '%';
                            document.getElementById('benchmarkPercent').textContent = Math.round(percent) + '%';
                            
                            // Update live metrics
                            const elapsed = (performance.now() - startTime) / 1000;
                            document.getElementById('benchLiveTokensPerSec').textContent = (tokenCount / elapsed).toFixed(1);
                            if (firstTokenTime) {
                                document.getElementById('benchLiveTimeToFirst').textContent = Math.round(firstTokenTime - startTime);
                            }
                            document.getElementById('benchLiveTotalTime').textContent = elapsed.toFixed(2);
                        }
                    } catch (e) { }
                }
            }
        }
        
        const endTime = performance.now();
        const totalTime = (endTime - startTime) / 1000;
        const ttft = firstTokenTime ? (firstTokenTime - startTime) : 0;
        const tokensPerSec = tokenCount / totalTime;
        
        // Get memory usage from health endpoint
        let memoryGB = '--';
        try {
            const healthRes = await fetch('/health');
            const healthData = await healthRes.json();
            if (healthData.system && healthData.system.memory_mb) {
                memoryGB = (healthData.system.memory_mb / 1024).toFixed(2);
                document.getElementById('benchLiveMemory').textContent = memoryGB;
            }
        } catch (e) { }
        
        // Save to history
        const result = {
            model,
            tokensPerSec: tokensPerSec.toFixed(1),
            ttft: Math.round(ttft),
            totalTime: totalTime.toFixed(2),
            memory: memoryGB,
            date: new Date().toLocaleString(),
            promptLength,
            outputTokens
        };
        
        benchmarkHistory.unshift(result);
        benchmarkHistory = benchmarkHistory.slice(0, 20); // Keep last 20
        localStorage.setItem('offgrid_benchmarks', JSON.stringify(benchmarkHistory));
        
        renderBenchmarkHistory();
        
    } catch (e) {
        showAlert('Benchmark failed: ' + e.message, 'error');
    } finally {
        btn.disabled = false;
        btn.innerHTML = '<svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z"></path><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg>Run Benchmark';
        progress.classList.add('hidden');
    }
}

function renderBenchmarkHistory() {
    const table = document.getElementById('benchmarkHistoryTable');
    const chart = document.getElementById('benchmarkChart');
    
    if (benchmarkHistory.length === 0) {
        table.innerHTML = '<tr><td colspan="6" class="text-center py-4 text-secondary">No benchmarks yet</td></tr>';
        chart.innerHTML = '<p class="text-secondary text-sm text-center py-4">Run benchmarks to see comparison charts</p>';
        return;
    }
    
    // Render table
    table.innerHTML = benchmarkHistory.map(r => `
        <tr class="border-b border-theme">
            <td class="py-2 px-3">${r.model}</td>
            <td class="text-right py-2 px-3 font-mono">${r.tokensPerSec}</td>
            <td class="text-right py-2 px-3 font-mono">${r.ttft}</td>
            <td class="text-right py-2 px-3 font-mono">${r.totalTime}s</td>
            <td class="text-right py-2 px-3 font-mono">${r.memory}</td>
            <td class="text-right py-2 px-3 text-secondary text-xs">${r.date}</td>
        </tr>
    `).join('');
    
    // Render simple bar chart for tokens/sec
    const maxTokens = Math.max(...benchmarkHistory.map(r => parseFloat(r.tokensPerSec)));
    chart.innerHTML = benchmarkHistory.slice(0, 5).map(r => {
        const width = (parseFloat(r.tokensPerSec) / maxTokens) * 100;
        return `
            <div class="flex items-center gap-3">
                <div class="w-32 truncate text-sm">${r.model.split('/').pop()}</div>
                <div class="flex-1 bg-tertiary rounded-full h-4 overflow-hidden">
                    <div class="bg-accent h-full rounded-full transition-all" style="width: ${width}%"></div>
                </div>
                <div class="w-20 text-right text-sm font-mono">${r.tokensPerSec} t/s</div>
            </div>
        `;
    }).join('');
}

function clearBenchmarkHistory() {
    showConfirm('Clear all benchmark history?', () => {
        benchmarkHistory = [];
        localStorage.removeItem('offgrid_benchmarks');
        renderBenchmarkHistory();
    }, { title: 'Clear History', confirmText: 'Clear All', type: 'warning' });
}

// Initialize
window.addEventListener('DOMContentLoaded', () => {
    // Setup terminal scroll tracking
    const terminalOutput = document.getElementById('terminalOutput');
    terminalOutput.addEventListener('scroll', () => {
        const isAtBottom = terminalOutput.scrollHeight - terminalOutput.scrollTop <= terminalOutput.clientHeight + 50;
        userScrolledUp = !isAtBottom;
    });
    
    // Restore active tab
    const savedTab = localStorage.getItem('offgrid_active_tab');
    if (savedTab && document.getElementById('tab-' + savedTab)) {
        switchTab(savedTab);
    } else {
        switchTab('chat');
    }
    
    // Load saved messages
    loadMessages();
    
    // Load chat models
    loadChatModels();
    
    // Load saved performance mode
    if (typeof loadPerformanceMode === 'function') {
        loadPerformanceMode();
    }
    
    // Setup Knowledge Base toggle indicator
    const kbCheckbox = document.getElementById('useKnowledgeBase');
    const ragIndicator = document.getElementById('ragIndicator');
    
    // Restore saved preference
    const savedKbPref = localStorage.getItem('offgrid_use_knowledge_base');
    if (savedKbPref === 'true') {
        kbCheckbox.checked = true;
        ragIndicator.classList.remove('hidden');
    }
    
    kbCheckbox.addEventListener('change', async () => {
        if (kbCheckbox.checked) {
            // Check if RAG is enabled on the server
            try {
                const response = await fetch('/v1/rag/status');
                const data = await response.json();
                if (!data.enabled) {
                    showAlert('Knowledge Base is not enabled on the server. Go to the Knowledge Base tab and enable RAG first.', 'warning');
                    kbCheckbox.checked = false;
                    return;
                }
                if (!data.stats || data.stats.document_count === 0) {
                    showAlert('No documents in knowledge base. Upload some documents in the Knowledge Base tab first.', 'warning');
                }
            } catch (e) {
                console.error('Failed to check RAG status:', e);
            }
            ragIndicator.classList.remove('hidden');
            localStorage.setItem('offgrid_use_knowledge_base', 'true');
        } else {
            ragIndicator.classList.add('hidden');
            localStorage.setItem('offgrid_use_knowledge_base', 'false');
        }
    });
});

// Theme Toggle Logic
const themeToggleBtn = document.getElementById('theme-toggle');
const htmlElement = document.documentElement;

// Check for saved theme preference or system preference
const savedTheme = localStorage.getItem('theme');
const systemTheme = window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';

// Set initial theme
if (savedTheme === 'dark' || (!savedTheme && systemTheme === 'dark')) {
    htmlElement.setAttribute('data-theme', 'dark');
} else {
    htmlElement.removeAttribute('data-theme');
}

themeToggleBtn.addEventListener('click', () => {
    if (htmlElement.getAttribute('data-theme') === 'dark') {
        htmlElement.removeAttribute('data-theme');
        localStorage.setItem('theme', 'light');
    } else {
        htmlElement.setAttribute('data-theme', 'dark');
        localStorage.setItem('theme', 'dark');
    }
});

