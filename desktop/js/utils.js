let currentModel = '';
let messages = [];
let isGenerating = false;
let abortController = null;
let commandHistory = [];
let historyIndex = -1;
let terminalRunning = false;
let currentTerminalAbort = null;
let terminalChatMode = false;
let terminalChatModel = '';
let terminalChatHistory = [];
let userScrolledUp = false;

// Session management
let sessions = JSON.parse(localStorage.getItem('offgrid_sessions') || '[]');
let currentSessionId = null;
let currentSystemPrompt = '';

// System prompts for different use cases
const systemPrompts = {
    research: "You are a knowledgeable research assistant. Help users understand complex topics, find relevant information, and think critically about academic questions. Provide detailed, well-sourced responses with references when possible.",
    tutor: "You are a patient and encouraging tutor. Break down complex concepts into simple explanations, use analogies and examples, ask questions to check understanding, and adapt your teaching style to the student's level.",
    coder: "You are an expert code reviewer and programming mentor. Provide detailed code reviews, suggest improvements for readability and performance, explain best practices, and help debug issues. Format all code with proper syntax highlighting.",
    writer: "You are an academic writing assistant. Help with essay structure, grammar, clarity, citation formats, and academic tone. Provide constructive feedback and suggestions for improvement.",
};

// Request throttling to prevent system overload
let lastRequestTime = 0;
let requestCooldown = 300; // Minimum 300ms between requests
let pendingRequest = false;

// Save messages to localStorage
function saveMessages() {
    try {
        localStorage.setItem('offgrid_messages', JSON.stringify(messages));
        localStorage.setItem('offgrid_current_model', currentModel);
    } catch (e) {
        console.error('Failed to save messages:', e);
    }
}

function normalizeMessageContent(rawContent) {
    if (Array.isArray(rawContent)) {
        const textParts = rawContent
            .filter(part => part?.type === 'text')
            .map(part => part.text || '');
        const imageParts = rawContent
            .filter(part => part?.type === 'image_url' && part.image_url?.url)
            .map(part => part.image_url.url);
        return {
            text: textParts.join('\n\n').trim(),
            images: imageParts
        };
    }

    if (typeof rawContent === 'string') {
        return { text: rawContent, images: [] };
    }

    if (rawContent && typeof rawContent === 'object') {
        if (typeof rawContent.text === 'string') {
            return { text: rawContent.text, images: [] };
        }
    }

    return { text: rawContent ? String(rawContent) : '', images: [] };
}

function escapeAttribute(value) {
    return (value || '').replace(/"/g, '&quot;');
}

function modelSupportsVision(modelInfo, fallbackId = '') {
    const hasCapability = Array.isArray(modelInfo?.capabilities) && modelInfo.capabilities.includes('vision');
    if (hasCapability || modelInfo?.type === 'vlm') {
        return true;
    }

    const tags = Array.isArray(modelInfo?.tags) ? modelInfo.tags : [];
    if (tags.some(tag => ['vision', 'vlm', 'multimodal', 'image'].includes(tag?.toLowerCase?.() || tag))) {
        return true;
    }

    const identifier = (fallbackId || modelInfo?.id || '').toLowerCase();
    const keywords = ['llava', 'bakllava', 'vision', 'yi-vl', 'vlm', 'qwen', 'moondream', 'minicpm', 'minicpm-v', 'pixtral'];
    return keywords.some(keyword => identifier.includes(keyword));
}

async function buildAPIError(response) {
    let payload = null;
    try {
        payload = await response.json();
    } catch (e) {
        // Ignore JSON parse errors
    }

    const message = payload?.error?.message || `Server error (${response.status})`;
    const code = payload?.error?.code || null;
    const err = new Error(message);
    err.code = code;
    err.status = response.status;
    err.payload = payload;
    return err;
}

// Load messages from localStorage
function loadMessages() {
    try {
        const saved = localStorage.getItem('offgrid_messages');
        const savedModel = localStorage.getItem('offgrid_current_model');
        const chatMessages = document.getElementById('chatMessages');
        
        if (saved) {
            messages = JSON.parse(saved);
            if (savedModel) {
                currentModel = savedModel;
            }
            // Only render if there are actual messages
            if (messages.length > 0) {
                chatMessages.innerHTML = '';
                messages.forEach(msg => {
                    if (msg.role === 'user' || msg.role === 'assistant') {
                        const { text, images } = normalizeMessageContent(msg.content);
                        addChatMessage(msg.role, text, images);
                    }
                });
            }
            // If empty array, leave the initial placeholder
            updateChatStats();
        }
        // If no saved messages, leave the initial placeholder
    } catch (e) {
        console.error('Failed to load messages:', e);
    }
}

// Tab switching
function switchTab(tab) {
    document.querySelectorAll('[id^="content-"]').forEach(el => el.classList.add('hidden'));
    document.querySelectorAll('.nav-item').forEach(el => el.classList.remove('active'));
    document.getElementById('content-' + tab).classList.remove('hidden');
    document.getElementById('tab-' + tab).classList.add('active');
    
    // Save current tab to localStorage
    localStorage.setItem('offgrid_active_tab', tab);
    
    if (tab === 'models') {
        loadInstalledModels();
        // Clear search input
        document.getElementById('searchQuery').value = '';
        document.getElementById('searchResults').innerHTML = '';
    }
    if (tab === 'chat') {
        loadChatModels();
        updateChatStats();
        renderSessions(); // Load sessions panel within chat
        loadChatVoiceSettings(); // Load voice settings for chat
    }
    if (tab === 'knowledge') {
        loadRAGStatus();
        loadRAGEmbeddingModels();
        refreshRAGDocuments();
        loadEmbeddingModels(); // Load embedding models for embeddings section
    }
    if (tab === 'benchmark') {
        loadBenchmarkModels();
        loadSystemInfo();
        renderBenchmarkHistory();
    }
    if (tab === 'agent') {
        loadAgentModels();
        loadAgentTools();
        loadMCPServers();
    }
    if (tab === 'users') {
        loadUsers();
    }
    if (tab === 'metrics') {
        loadMetrics();
    }
    if (tab === 'lora') {
        loadLoRAAdapters();
        loadLoRAModels();
    }
    if (tab === 'audio') {
        initAudioTab();
    }
}

// Audio sub-tab switching
function switchAudioSubTab(subTab) {
    // Hide all audio sub-content
    document.querySelectorAll('.audio-subcontent').forEach(el => el.classList.add('hidden'));
    // Remove active class from all sub-tabs
    document.querySelectorAll('.audio-subtab').forEach(el => el.classList.remove('active'));
    // Show selected sub-content
    document.getElementById('audio-content-' + subTab).classList.remove('hidden');
    // Add active class to selected sub-tab
    document.getElementById('audio-subtab-' + subTab).classList.add('active');
}

// Chat keyboard handler
function handleChatKeydown(event) {
    if (event.key === 'Enter' && !event.shiftKey) {
        event.preventDefault();
        sendChat();
    }
}

// Auto-resize textarea
function autoResizeTextarea(textarea) {
    textarea.style.height = 'auto';
    textarea.style.height = Math.min(textarea.scrollHeight, 200) + 'px';
}

// Terminal keyboard handler
function handleTerminalKeydown(event) {
    const input = event.target;
    
    if (event.key === 'Enter') {
        event.preventDefault();
        runCommand();
    } else if (event.key === 'ArrowUp') {
        event.preventDefault();
        if (historyIndex < commandHistory.length - 1) {
            historyIndex++;
            input.value = commandHistory[historyIndex] || '';
        }
    } else if (event.key === 'ArrowDown') {
        event.preventDefault();
        if (historyIndex > 0) {
            historyIndex--;
            input.value = commandHistory[historyIndex] || '';
        } else if (historyIndex === 0) {
            historyIndex = -1;
            input.value = '';
        }
    } else if (event.key === 'Tab') {
        event.preventDefault();
        autocompleteCommand(input);
    }
}

function autocompleteCommand(input) {
    const val = input.value;
    const commands = [
        'offgrid list',
        'offgrid recommend',
        'offgrid download ',
        'offgrid remove ',
        'offgrid search ',
        'offgrid run ',
        'offgrid serve',
        'offgrid --help',
        'offgrid --version',
        'help',
        'clear',
        'history'
    ];
    const match = commands.find(cmd => cmd.startsWith(val));
    if (match) {
        input.value = match;
    }
}

