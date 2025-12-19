async function sendChat() {
    console.log('[SEND CHAT] Function called');
    console.log('[SEND CHAT] Flags - isGenerating:', isGenerating, 'pendingRequest:', pendingRequest);
    
    const input = document.getElementById('chatInput');
    const msg = input.value.trim();

    if (!msg && imageAttachments.length === 0) {
        return;
    }

    // Prevent rapid-fire requests
    if (isGenerating || pendingRequest) {
        console.warn('[SEND CHAT] Request already in progress, ignoring...');
        // If stuck, reset after 30 seconds
        const now = Date.now();
        if (lastRequestTime && (now - lastRequestTime) > 30000) {
            console.warn('[SEND CHAT] Flags stuck for >30s, force resetting...');
            isGenerating = false;
            pendingRequest = false;
        } else {
            return;
        }
    }
    
    lastRequestTime = Date.now();
    pendingRequest = true;
    console.log('[SEND CHAT] Set pendingRequest = true');

    const model = document.getElementById('chatModel').value;
    console.log('[SEND CHAT] Selected model from dropdown:', model);
    console.log('[SEND CHAT] Current model variable:', currentModel);
    console.log('[SEND CHAT] Dropdown element value:', document.getElementById('chatModel').value);
    
    // Check if this is an embedding model
    const resp = await fetch('/models');
    const data = await resp.json();
    console.log('[SEND CHAT] ALL MODELS DATA:', JSON.stringify(data.data, null, 2));
    const modelInfo = data.data.find(m => m.id === model);
    console.log('[SEND CHAT] Found model info:', JSON.stringify(modelInfo, null, 2));
    // Check type from metadata, or fallback to name heuristics if metadata missing
    const isEmbeddingModel = modelInfo?.type === 'embedding' || 
                           model.toLowerCase().includes('embed') || 
                           model.toLowerCase().includes('bge') ||
                           model.toLowerCase().includes('nomic');
    console.log('[SEND CHAT] Model type:', modelInfo?.type, 'Is embedding:', isEmbeddingModel);
    const supportsVision = modelSupportsVision(modelInfo, model);
    console.log('[SEND CHAT] Supports vision:', supportsVision);
    
    if (!model) {
        console.log('[SEND CHAT] No model selected!');
        pendingRequest = false; // Clear flag
        const statusBadge = document.getElementById('statusBadge');
        statusBadge.className = 'badge badge-warning';
        statusBadge.textContent = 'Select Model';
        setTimeout(() => {
            statusBadge.className = 'badge badge-success';
            statusBadge.textContent = 'Ready';
        }, 3000);
        return;
    }

    // ALWAYS sync currentModel with dropdown before sending
    currentModel = model;
    console.log('[SEND CHAT] Synced currentModel to:', currentModel);

    const sendBtn = document.getElementById('sendBtn');
    const stopBtn = document.getElementById('stopBtn');
    const streamEnabled = document.getElementById('streamToggle').checked;
    const temperature = parseFloat(document.getElementById('temperature').value);
    const maxTokens = parseInt(document.getElementById('maxTokens').value);

    isGenerating = true;
    input.disabled = true;
    sendBtn.disabled = true;
    stopBtn.classList.remove('hidden');

    // Handle VLM content
    let messageContent = msg;
    let inlineImages = [];
    if (imageAttachments.length > 0) {
        if (!supportsVision) {
            showToast('Selected model does not support image input', 'error');
            pendingRequest = false;
            isGenerating = false;
            input.disabled = false;
            sendBtn.disabled = false;
            stopBtn.classList.add('hidden');
            return;
        }
        inlineImages = imageAttachments.map(att => att.url);
        messageContent = [
            { type: "text", text: msg },
            ...inlineImages.map(url => {
                // Convert WebP to JPEG if needed, or just strip the prefix if llama-server expects raw base64
                // But standard OpenAI API expects data URI.
                // The issue is likely that llama-server's stb_image doesn't support WebP.
                // We should convert to JPEG/PNG before sending.
                // For now, let's try to ensure it's a format stb_image supports (JPEG/PNG).
                return {
                    type: "image_url",
                    image_url: { url }
                };
            })
        ];
    }

    messages.push({ role: 'user', content: messageContent });
    addChatMessage('user', msg, inlineImages);
    
    // Clear image attachments after adding to chat
    clearAllImages();
    
    input.value = '';
    updateChatStats();
    saveMessages();
    scrollToBottom();

    // Show thinking indicator immediately after user message
    const thinkingIndicator = addThinkingIndicator();

    const statusBadge = document.getElementById('statusBadge');
    statusBadge.className = 'text-xs font-medium text-yellow-400';
    
    // Show loading indicator with elapsed time
    let loadingInterval;
    let elapsedSeconds = 0;
    statusBadge.textContent = 'Loading model...';
    loadingInterval = setInterval(() => {
        elapsedSeconds++;
        if (elapsedSeconds < 60) {
            statusBadge.textContent = `Loading model... ${elapsedSeconds}s`;
        } else {
            const mins = Math.floor(elapsedSeconds / 60);
            const secs = elapsedSeconds % 60;
            statusBadge.textContent = `Loading model... ${mins}m ${secs}s`;
        }
    }, 1000);

    abortController = new AbortController();
    const startTime = Date.now();

    // Prepare messages with system prompt if present
    let apiMessages = [...messages];
    if (currentSystemPrompt && currentSystemPrompt.trim()) {
        // Check if first message is already a system message
        if (apiMessages.length === 0 || apiMessages[0].role !== 'system') {
            apiMessages = [{ role: 'system', content: currentSystemPrompt }, ...apiMessages];
        }
    }

    try {
        // If it's an embedding model, use embeddings API
        if (isEmbeddingModel) {
            console.log('[SEND CHAT] Routing to embeddings API for embedding model');
            const embeddingResponse = await fetch('/v1/embeddings', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ model: model, input: msg }),
                signal: abortController.signal
            });
            
            if (!embeddingResponse.ok) {
                let errorDetails = '';
                try {
                    const errorData = await embeddingResponse.json();
                    errorDetails = errorData.error?.message || JSON.stringify(errorData);
                } catch (e) {
                    errorDetails = embeddingResponse.statusText;
                }
                throw new Error(`Server error (${embeddingResponse.status}): ${errorDetails}`);
            }
            
            const embeddingData = await embeddingResponse.json();
            const embedding = embeddingData.data[0].embedding;
            const dimensions = embedding.length;
            
            // Remove thinking indicator
            if (thinkingIndicator) {
                thinkingIndicator.remove();
            }
            
            // Display embedding result as a message
            const embeddingText = `**Embedding Generated**\n\n` +
                `**Dimensions:** ${dimensions}\n` +
                `**First 10 values:** [${embedding.slice(0, 10).map(v => v.toFixed(4)).join(', ')}...]\n\n` +
                `<details>\n<summary>View full embedding vector (${dimensions} dimensions)</summary>\n\n\`\`\`json\n${JSON.stringify(embedding, null, 2)}\n\`\`\`\n</details>`;
            
            addChatMessage('assistant', embeddingText, null, startTime);
            updateChatStats();
            saveMessages();
            
            if (loadingInterval) clearInterval(loadingInterval);
            statusBadge.className = 'text-xs font-medium text-green-400';
            statusBadge.textContent = 'Ready';
            isGenerating = false;
            pendingRequest = false;
            input.disabled = false;
            sendBtn.disabled = false;
            stopBtn.classList.add('hidden');
            scrollToBottom();
            return;
        }
        
        const requestPayload = {
            model: model,
            messages: apiMessages,
            stream: streamEnabled,
            temperature: temperature,
            max_tokens: maxTokens,
            use_knowledge_base: document.getElementById('useKnowledgeBase').checked
        };
        
        console.log('[SEND CHAT] REQUEST PAYLOAD:', JSON.stringify(requestPayload, null, 2));
        console.log('[SEND CHAT] Model being sent:', model);
        console.log('[SEND CHAT] Knowledge Base enabled:', document.getElementById('useKnowledgeBase').checked);
        
        const response = await fetch('/v1/chat/completions', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(requestPayload),
            signal: abortController.signal
        });

        if (!response.ok) {
            throw await buildAPIError(response);
        }

        if (streamEnabled) {
            const reader = response.body.getReader();
            const decoder = new TextDecoder();
            let assistantMsg = '';
            let msgDiv = null;
            let firstTokenReceived = false;
            let lastRender = 0;

            while (true) {
                const { value, done } = await reader.read();
                if (done) break;

                const chunk = decoder.decode(value);
                const lines = chunk.split('\n').filter(line => line.trim() !== '');

                for (const line of lines) {
                    if (!line.startsWith('data: ')) continue;
                    const data = line.slice(6);
                    if (data === '[DONE]') continue;

                    try {
                        const json = JSON.parse(data);

                        if (json.error) {
                            const streamError = new Error(json.error.message || 'Stream error');
                            streamError.code = json.error.code || json.error.type || null;
                            throw streamError;
                        }

                        const content = json.choices?.[0]?.delta?.content || '';
                        if (content) {
                            // Remove thinking indicator on first token
                            if (!firstTokenReceived && thinkingIndicator) {
                                thinkingIndicator.remove();
                                firstTokenReceived = true;
                            }
                            
                            if (loadingInterval) {
                                clearInterval(loadingInterval);
                                loadingInterval = null;
            statusBadge.textContent = 'Generating...';
                            }
                            
                            assistantMsg += content;
                            if (!msgDiv) {
                                msgDiv = addChatMessage('assistant', '', null, startTime);
                            }

                            // Throttle rendering to avoid jitter (max 12fps)
                            const now = Date.now();
                            if (now - lastRender > 80) {
                                const contentDiv = msgDiv.querySelector('.message-content');
                                // Append invisible char to preserve trailing whitespace/newlines during stream
                                const streamContent = assistantMsg + (assistantMsg.endsWith(' ') ? ' ' : ''); 
                                contentDiv.innerHTML = marked.parse(streamContent || '');
                                contentDiv.querySelectorAll('pre code').forEach((block) => {
                                    hljs.highlightElement(block);
                                });
                                scrollToBottom();
                                lastRender = now;
                            }
                        }
                    } catch (streamErr) {
                        if (streamErr instanceof SyntaxError) {
                            continue; // Wait for more data
                        }
                        throw streamErr;
                    }
                }
            }

            // Final render to ensure complete message is shown
            if (msgDiv && assistantMsg) {
                const contentDiv = msgDiv.querySelector('.message-content');
                contentDiv.innerHTML = marked.parse(assistantMsg || '');
                contentDiv.querySelectorAll('pre code').forEach((block) => {
                    hljs.highlightElement(block);
                });
                scrollToBottom();
            }

            // Remove thinking indicator if still present
            if (thinkingIndicator) {
                thinkingIndicator.remove();
            }

            if (assistantMsg) {
                messages.push({ role: 'assistant', content: assistantMsg });
                const elapsed = Date.now() - startTime;
                msgDiv.querySelector('.time-badge').textContent = `${elapsed}ms`;
                saveMessages();
            }
        } else {
            // Non-streaming mode
            // Model is loaded when we get response
            if (loadingInterval) {
                clearInterval(loadingInterval);
                loadingInterval = null;
                statusBadge.textContent = 'Generating...';
            }
            
            const result = await response.json();
            
            // Remove thinking indicator
            if (thinkingIndicator) {
                thinkingIndicator.remove();
            }
            
            const assistantMsg = result.choices[0]?.message?.content || 'No response';
            const elapsed = Date.now() - startTime;
            if (assistantMsg) {
                messages.push({ role: 'assistant', content: assistantMsg });
                addChatMessage('assistant', assistantMsg, null, startTime);
                saveMessages();
                scrollToBottom();
            }
        }

        updateChatStats();
        if (loadingInterval) {
            clearInterval(loadingInterval);
            loadingInterval = null;
        }
        statusBadge.className = 'text-xs font-medium text-green-400';
        statusBadge.textContent = 'Ready';
    } catch (error) {
        if (loadingInterval) {
            clearInterval(loadingInterval);
            loadingInterval = null;
        }
        
        // Remove thinking indicator on error
        if (thinkingIndicator) {
            thinkingIndicator.remove();
        }
        
        if (error.name === 'AbortError') {
            addChatMessage('assistant', '[Generation stopped by user]', null, startTime);
            statusBadge.className = 'text-xs font-medium text-green-400';
            statusBadge.textContent = 'Ready';
        } else {
            const errorMsg = error.message || 'Unknown error occurred';
            console.error('Chat error:', error);
            
            // Better error messages for common issues
            let userMessage = `⚠ Error: ${errorMsg}`;
            if (error.code === 'missing_mmproj') {
                userMessage = '⚠ This vision model is missing its mmproj adapter. Download the matching .mmproj file, place it next to the GGUF, reload the model, and try again.';
            } else if (errorMsg.includes('503') || errorMsg.includes('Failed to load model')) {
                userMessage = `⚠ Model is taking longer than expected to load.\n\nThis can happen on slower systems. Please try again in a few moments.`;
            } else if (errorMsg.includes('500')) {
                userMessage = `⚠ Server error occurred.\n\nThe model may still be loading. Please wait a moment and try again.`;
            }
            
            addChatMessage('assistant', userMessage, null, startTime);
            statusBadge.className = 'text-xs font-medium text-red-400';
            statusBadge.textContent = 'Error';
        }
    } finally {
        if (loadingInterval) {
            clearInterval(loadingInterval);
        }
        isGenerating = false;
        pendingRequest = false;
        console.log('[SEND CHAT] Cleaned up - pendingRequest = false, isGenerating = false');
        abortController = null;
        input.disabled = false;
        sendBtn.disabled = false;
        stopBtn.classList.add('hidden');
        input.focus();
    }
}

// Add thinking indicator
function addThinkingIndicator() {
    const container = document.getElementById('chatMessages');
    const div = document.createElement('div');
    div.className = 'message message-assistant thinking-indicator';
    
    div.innerHTML = `
        <div class="message-wrapper">
            <div class="message-avatar">AI</div>
            <div class="message-body">
                <div class="flex items-center gap-2 mb-2">
                    <span class="text-xs font-medium text-secondary">Assistant</span>
                </div>
                <div class="flex items-center gap-2 text-sm text-secondary">
                    <div class="thinking-dots">
                        <span class="dot"></span>
                        <span class="dot"></span>
                        <span class="dot"></span>
                    </div>
                    <span class="thinking-text">Thinking...</span>
                </div>
            </div>
        </div>
    `;
    container.appendChild(div);
    scrollToBottom();
    return div;
}

// Add thinking indicator
function addThinkingIndicator() {
    const container = document.getElementById('chatMessages');
    const div = document.createElement('div');
    div.className = 'message message-assistant thinking-indicator';
    
    div.innerHTML = `
        <div class="message-wrapper">
            <div class="message-avatar">AI</div>
            <div class="message-body">
                <div class="flex items-center gap-2 mb-2">
                    <span class="text-xs font-medium text-secondary">Assistant</span>
                </div>
                <div class="flex items-center gap-2 text-sm text-secondary">
                    <div class="thinking-dots">
                        <span class="dot"></span>
                        <span class="dot"></span>
                        <span class="dot"></span>
                    </div>
                    <span class="thinking-text">Thinking...</span>
                </div>
            </div>
        </div>
    `;
    container.appendChild(div);
    scrollToBottom();
    return div;
}

// Helper function to smoothly scroll chat to bottom
function scrollToBottom() {
    const container = document.getElementById('chatMessages');
    if (container) {
        container.scrollTop = container.scrollHeight;
    }
}

function addChatMessage(role, content, images = null, startTime = null) {
    const container = document.getElementById('chatMessages');
    if (container.querySelector('.text-center')) {
        container.innerHTML = '';
    }

    const div = document.createElement('div');
    div.className = 'message message-' + role;
    
    // Configure marked.js with syntax highlighting and copy button
    const renderer = new marked.Renderer();
    renderer.code = function(entry, langParam) {
        // Handle Marked v12+ breaking change where arguments are passed as an object
        let code = entry;
        let language = langParam;
        if (typeof entry === 'object' && entry !== null && entry.text) {
            code = entry.text;
            language = entry.lang;
        }

        code = String(code || '');
        const validLang = !!(language && hljs.getLanguage(language));
        const highlighted = validLang
            ? hljs.highlight(code, { language: language }).value
            : hljs.highlightAuto(code).value;
        
        const id = 'code-' + Math.random().toString(36).substr(2, 9);
        // Escape code for the onclick handler to avoid syntax errors
        const escapedCode = code.replace(/\\/g, '\\\\').replace(/'/g, "\\'").replace(/"/g, '&quot;').replace(/\n/g, '\\n');
        
        return `
            <div class="code-block-wrapper group">
                <div class="code-block-header">
                    <span class="font-mono">${language || 'text'}</span>
                    <button onclick="copyCodeToClipboard(this)" data-code="${encodeURIComponent(code)}" class="flex items-center gap-1 hover:text-white transition-colors">
                        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path></svg>
                        <span>Copy</span>
                    </button>
                </div>
                <div class="overflow-x-auto">
                    <pre><code class="hljs ${language} block p-4 text-sm leading-relaxed">${highlighted}</code></pre>
                </div>
            </div>
        `;
    };

    marked.setOptions({
        renderer: renderer,
        breaks: true,
        gfm: true
    });

    // Parse markdown content
    const formattedContent = marked.parse(String(content || ''));
    const avatar = role === 'user' ? 'U' : 'AI';
    const timestamp = new Date().toLocaleTimeString([], {hour: '2-digit', minute:'2-digit'});
    const elapsed = startTime ? `${Date.now() - startTime}ms` : '';
    
    const attachmentList = Array.isArray(images) ? images : (images ? [images] : []);
    let imageHtml = '';
    if (attachmentList.length) {
        const imageItems = attachmentList.map(url => {
            const safeUrl = escapeAttribute(url);
            return `<img src="${safeUrl}" class="max-h-64 rounded-lg border border-theme shadow-md object-cover">`;
        }).join('');
        imageHtml = `<div class="flex flex-wrap gap-3 mb-3">${imageItems}</div>`;
    }
    
    div.innerHTML = `
        <div class="message-wrapper">
            <div class="message-avatar">${avatar}</div>
            <div class="message-body">
                <div class="flex items-center gap-2 mb-2">
                    <span class="text-xs font-medium text-secondary">${role === 'user' ? 'You' : 'Assistant'}</span>
                    <span class="text-xs text-secondary">${timestamp}</span>
                    ${elapsed ? `<span class="time-badge text-xs text-accent">${elapsed}</span>` : ''}
                </div>
                ${imageHtml}
                <div class="text-sm message-content prose prose-invert max-w-none">${formattedContent}</div>
                ${role === 'assistant' ? `
                <div class="message-actions mt-3">
                    <button onclick="copyMessage(this)" class="btn btn-secondary btn-sm">Copy</button>
                    <button onclick="regenerateMessage()" class="btn btn-info btn-sm">Regenerate</button>
                </div>` : ''}
            </div>
        </div>
    `;
    container.appendChild(div);
    scrollToBottom();
    
    // Apply highlighting to all code blocks in this message
    div.querySelectorAll('pre code').forEach((block) => {
        hljs.highlightElement(block);
    });
    
    return div;
}

function copyMessage(btn) {
    const content = btn.closest('.message').querySelector('.message-content').textContent;
    navigator.clipboard.writeText(content).then(() => {
        const original = btn.textContent;
        btn.textContent = 'Copied!';
        setTimeout(() => btn.textContent = original, 2000);
    });
}

function regenerateMessage() {
    if (messages.length < 2) return;
    messages.pop(); // Remove last assistant message
    const lastUserMsg = messages[messages.length - 1];
    messages.pop(); // Remove last user message
    document.getElementById('chatInput').value = lastUserMsg.content;
    sendChat();
}

// Performance mode configuration for consumer hardware optimization
const performanceModes = {
    balanced: { temperature: 0.7, top_p: 0.9 },
    speed: { temperature: 0.5, top_p: 0.8 },  // Lower diversity = faster generation
    quality: { temperature: 0.8, top_p: 0.95 }  // Higher diversity = better quality
};

function applyPerformanceMode() {
    const mode = document.getElementById('performanceMode').value;
    const config = performanceModes[mode] || performanceModes.balanced;
    
    document.getElementById('temperature').value = config.temperature;
    
    // Save preference
    localStorage.setItem('offgrid_performance_mode', mode);
    
    console.log('[PERFORMANCE] Applied mode:', mode, config);
}

// Load saved performance mode on init
function loadPerformanceMode() {
    const saved = localStorage.getItem('offgrid_performance_mode');
    if (saved && performanceModes[saved]) {
        const select = document.getElementById('performanceMode');
        if (select) {
            select.value = saved;
            applyPerformanceMode();
        }
    }
}

// Update cache indicator based on model state
function updateCacheIndicator(isActive) {
    const indicator = document.getElementById('cacheIndicator');
    if (indicator) {
        if (isActive) {
            indicator.classList.remove('hidden');
        } else {
            indicator.classList.add('hidden');
        }
    }
}

// Show cache indicator when model is warmed (has active KV cache)
function onModelWarmed(modelName) {
    console.log('[CACHE] Model warmed with KV cache:', modelName);
    updateCacheIndicator(true);
}

// Search models
