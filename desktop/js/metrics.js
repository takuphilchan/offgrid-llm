// ============================================
// METRICS FUNCTIONS
// ============================================

async function loadMetrics() {
    try {
        // Fetch both prometheus metrics and system stats
        const [metricsResp, statsResp] = await Promise.all([
            fetch('/metrics'),
            fetch('/v1/system/stats')
        ]);
        
        const metricsText = await metricsResp.text();
        const rawMetricsEl = document.getElementById('rawMetrics');
        
        // Format the raw metrics for better readability
        if (rawMetricsEl) {
            const formatted = formatPrometheusMetrics(metricsText);
            rawMetricsEl.innerHTML = formatted;
        }
        
        // Use real system stats
        const stats = await statsResp.json();
        
        // Update top cards (with null checks)
        const reqEl = document.getElementById('metricRequests');
        const errEl = document.getElementById('metricErrors');
        const tokEl = document.getElementById('metricTokens');
        const latEl = document.getElementById('metricLatency');
        const avgLatEl = document.getElementById('metricAvgLatency');
        
        if (reqEl) reqEl.textContent = (stats.requests_total || 0).toLocaleString();
        if (errEl) errEl.textContent = (stats.errors_total || 0).toLocaleString();
        if (tokEl) tokEl.textContent = (stats.tokens_generated || 0).toLocaleString();
        
        // Update latency (use actual average latency)
        const avgLatency = stats.avg_latency_ms || 0;
        if (latEl) latEl.textContent = avgLatency.toFixed(1) + 'ms';
        if (avgLatEl) avgLatEl.textContent = avgLatency.toFixed(1) + 'ms';
        
        // Update resource usage (with null checks)
        const cpuPercent = stats.cpu_percent || 0;
        const cpuUsageEl = document.getElementById('cpuUsage');
        const cpuBarEl = document.getElementById('cpuBar');
        if (cpuUsageEl) cpuUsageEl.textContent = cpuPercent.toFixed(1) + '%';
        if (cpuBarEl) cpuBarEl.style.width = Math.min(cpuPercent, 100) + '%';
        
        const memBytes = stats.memory_bytes || 0;
        const memMB = (memBytes / 1024 / 1024).toFixed(0);
        const memUsageEl = document.getElementById('memoryUsage');
        const memBarEl = document.getElementById('memoryBar');
        if (memUsageEl) memUsageEl.textContent = memMB + ' MB';
        const memTotal = stats.memory_total || (8 * 1024 * 1024 * 1024);
        if (memBarEl) memBarEl.style.width = Math.min(memBytes / memTotal * 100, 100) + '%';
        
        // Disk usage
        const diskUsageEl = document.getElementById('diskUsage');
        const diskBarEl = document.getElementById('diskBar');
        if (diskUsageEl) diskUsageEl.textContent = '0.0 GB';
        if (diskBarEl) diskBarEl.style.width = '0%';
        
        // Update active connections (with null checks)
        const wsEl = document.getElementById('wsConnections');
        const sessEl = document.getElementById('activeSessions');
        const modelsEl = document.getElementById('loadedModels');
        const ragEl = document.getElementById('ragDocuments');
        
        if (wsEl) wsEl.textContent = stats.websocket_connections || 0;
        if (sessEl) sessEl.textContent = stats.active_sessions || 0;
        if (modelsEl) modelsEl.textContent = stats.models_loaded || 0;
        if (ragEl) ragEl.textContent = stats.rag_documents || 0;
        
    } catch (e) {
        console.error('Failed to load metrics:', e);
        const rawMetricsEl = document.getElementById('rawMetrics');
        if (rawMetricsEl) rawMetricsEl.textContent = 'Failed to load metrics: ' + e.message;
    }
}

function formatPrometheusMetrics(text) {
    // Parse and format Prometheus metrics for better readability
    const lines = text.split('\n');
    let html = '';
    let currentMetric = '';
    
    for (const line of lines) {
        if (!line.trim()) continue;
        
        if (line.startsWith('# HELP')) {
            // Help line - extract metric name and description
            const match = line.match(/# HELP (\S+) (.+)/);
            if (match) {
                html += `<div class="mt-3 text-accent font-medium">${match[1]}</div>`;
                html += `<div class="text-xs text-secondary italic mb-1">${match[2]}</div>`;
                currentMetric = match[1];
            }
        } else if (line.startsWith('# TYPE')) {
            // Type line - skip (redundant for display)
            continue;
        } else {
            // Metric value line
            const match = line.match(/^(\S+?)(\{[^}]+\})?\s+(.+)$/);
            if (match) {
                const [, name, labels, value] = match;
                const numValue = parseFloat(value);
                const formattedValue = Number.isInteger(numValue) ? numValue.toLocaleString() : numValue.toFixed(2);
                const labelStr = labels ? `<span class="text-blue-400">${labels}</span>` : '';
                html += `<div class="pl-2">${labelStr} <span class="text-emerald-400 font-mono">${formattedValue}</span></div>`;
            }
        }
    }
    
    return html || '<span class="text-secondary">No metrics available</span>';
}

function copyMetrics() {
    const el = document.getElementById('rawMetrics');
    const text = el.innerText || el.textContent;
    navigator.clipboard.writeText(text);
    showAlert('Metrics copied to clipboard', { title: 'Copied', type: 'success' });
}

