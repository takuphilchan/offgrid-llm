// ========================================
// Audio Functions (Speech-to-Text & Text-to-Speech)
// ========================================

let audioFile = null;
let mediaRecorder = null;
let audioChunks = [];
let isRecording = false;
let ttsAudioBlob = null;

// Initialize audio tab when switched to
function initAudioTab() {
    refreshAudioStatus();
    loadVoiceAssistantModels();
    loadAvailableVoices();
    loadAvailableWhisperModels();
}

// Global voice data for filtering
let allVoices = [];

// Global whisper model data for filtering
let allWhisperModels = [];

async function loadAvailableVoices() {
    try {
        const resp = await fetch('/v1/audio/voices');
        if (!resp.ok) return;
        
        const data = await resp.json();
        allVoices = data.voices || [];
        
        // Update Voice Assistant dropdown
        const voiceAssistantVoice = document.getElementById('voiceAssistantVoice');
        if (voiceAssistantVoice) {
            const installedVoices = allVoices.filter(v => v.installed);
            if (installedVoices.length > 0) {
                voiceAssistantVoice.innerHTML = installedVoices.map(v => 
                    `<option value="${v.name}">${v.name.replace(/-/g, ' ').replace(/_/g, ' ')} (${v.language})</option>`
                ).join('');
                
                // Restore saved selection
                const saved = localStorage.getItem('voiceAssistantVoice');
                if (saved && installedVoices.some(v => v.name === saved)) {
                    voiceAssistantVoice.value = saved;
                }
            } else {
                voiceAssistantVoice.innerHTML = '<option value="">No voices installed</option>';
            }
        }
        
        // Update TTS dropdown
        const ttsVoice = document.getElementById('ttsVoice');
        if (ttsVoice) {
            const installedVoices = allVoices.filter(v => v.installed);
            if (installedVoices.length > 0) {
                ttsVoice.innerHTML = installedVoices.map(v => 
                    `<option value="${v.name}">${v.name} (${v.language})</option>`
                ).join('');
            } else {
                ttsVoice.innerHTML = '<option value="">No voices installed</option>';
            }
        }
        
        // Populate language filter dropdown
        const langFilter = document.getElementById('voiceLanguageFilter');
        if (langFilter) {
            const languages = [...new Set(allVoices.map(v => v.language))].sort();
            langFilter.innerHTML = '<option value="">All Languages (' + languages.length + ')</option>' + 
                languages.map(l => `<option value="${l}">${l}</option>`).join('');
        }
        
        // Update voice count
        const voiceCount = document.getElementById('voiceCount');
        if (voiceCount) {
            const installed = allVoices.filter(v => v.installed).length;
            voiceCount.textContent = `${installed}/${allVoices.length} installed`;
        }
        
        // Render voices
        renderVoices(allVoices);
    } catch (e) {
        console.error('Failed to load voices:', e);
    }
}

function filterVoices() {
    const search = (document.getElementById('voiceSearch')?.value || '').toLowerCase();
    const langFilter = document.getElementById('voiceLanguageFilter')?.value || '';
    const statusFilter = document.getElementById('voiceStatusFilter')?.value || '';
    
    let filtered = allVoices;
    
    if (search) {
        filtered = filtered.filter(v => 
            v.name.toLowerCase().includes(search) || 
            v.language.toLowerCase().includes(search)
        );
    }
    
    if (langFilter) {
        filtered = filtered.filter(v => v.language === langFilter);
    }
    
    if (statusFilter === 'installed') {
        filtered = filtered.filter(v => v.installed);
    } else if (statusFilter === 'available') {
        filtered = filtered.filter(v => !v.installed);
    }
    
    renderVoices(filtered);
}

function renderVoices(voices) {
    const voiceLibrary = document.getElementById('voiceLibrary');
    if (!voiceLibrary) return;
    
    if (voices.length === 0) {
        voiceLibrary.innerHTML = '<div class="text-secondary text-center py-4 text-sm">No voices found matching your filters</div>';
        return;
    }
    
    // Group by language
    const byLanguage = {};
    voices.forEach(v => {
        const lang = v.language || 'Other';
        if (!byLanguage[lang]) byLanguage[lang] = [];
        byLanguage[lang].push(v);
    });
    
    const sortedLanguages = Object.keys(byLanguage).sort();
    
    voiceLibrary.innerHTML = sortedLanguages.map(lang => {
        const langVoices = byLanguage[lang];
        const installedCount = langVoices.filter(v => v.installed).length;
        const langId = lang.replace(/[^a-zA-Z]/g, '');
        
        return `
            <details class="voice-lang-group" open>
                <summary class="flex items-center justify-between cursor-pointer select-none py-1.5 px-2 rounded hover:bg-tertiary">
                    <span class="flex items-center gap-2 text-sm font-medium">
                        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="voice-chevron transition-transform"><polyline points="9 18 15 12 9 6"/></svg>
                        ${lang}
                    </span>
                    <span class="text-xs ${installedCount > 0 ? 'text-green-400' : 'text-secondary'}">${installedCount}/${langVoices.length}</span>
                </summary>
                <div class="grid grid-cols-2 sm:grid-cols-3 gap-1 py-1.5 pl-5">
                    ${langVoices.map(v => {
                        const displayName = formatVoiceName(v.name);
                        return `
                            <div class="voice-card ${v.installed ? 'voice-installed' : ''} flex items-center justify-between gap-1 px-2 py-1 rounded text-xs bg-tertiary/50 hover:bg-tertiary" data-voice="${v.name}">
                                <span class="truncate" title="${v.name}">
                                    <span class="font-medium">${displayName}</span>
                                    <span class="text-secondary">${v.quality}</span>
                                </span>
                                ${v.installed 
                                    ? '<span class="text-green-400 flex-shrink-0">✓</span>'
                                    : `<button onclick="downloadVoice('${v.name}')" class="text-cyan-400 hover:text-cyan-300 flex-shrink-0 p-0.5" title="Download">
                                        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>
                                       </button>`
                                }
                            </div>
                        `;
                    }).join('')}
                </div>
            </details>
        `;
    }).join('');
}

function formatVoiceName(name) {
    // Convert "en_US-lessac-medium" to "Lessac"
    const parts = name.split('-');
    if (parts.length >= 2) {
        const voiceName = parts[1];
        return voiceName.charAt(0).toUpperCase() + voiceName.slice(1);
    }
    return name;
}

async function downloadVoice(name) {
    // Find the voice card and show downloading state
    const voiceCard = document.querySelector(`.voice-card[data-voice="${name}"]`);
    if (voiceCard) {
        voiceCard.classList.add('voice-downloading');
        const btn = voiceCard.querySelector('button');
        if (btn) btn.innerHTML = '<span class="animate-pulse">⏳</span>';
    }
    
    showAlert(`Downloading voice: ${formatVoiceName(name)}... This may take a moment.`, { title: 'Downloading', type: 'info' });
    
    try {
        const resp = await fetch('/v1/audio/download', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ type: 'piper', name: name })
        });
        
        if (!resp.ok) {
            const err = await resp.json();
            throw new Error(err.error?.message || 'Download failed');
        }
        
        showAlert(`Voice "${formatVoiceName(name)}" downloaded successfully!`, { title: 'Success', type: 'success' });
        loadAvailableVoices(); // Refresh the list
    } catch (e) {
        showAlert(`Failed to download voice: ${e.message}`, { title: 'Error', type: 'error' });
        // Remove downloading state on error
        if (voiceCard) {
            voiceCard.classList.remove('voice-downloading');
            const btn = voiceCard.querySelector('button');
            if (btn) btn.innerHTML = '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>';
        }
    }
}

// ========================================
// Whisper Model Library Functions
// ========================================

async function loadAvailableWhisperModels() {
    try {
        const resp = await fetch('/v1/audio/whisper-models');
        if (!resp.ok) return;
        
        const data = await resp.json();
        allWhisperModels = data.models || [];
        
        // Update count
        const modelCount = document.getElementById('whisperModelCount');
        if (modelCount) {
            const installed = allWhisperModels.filter(m => m.installed).length;
            modelCount.textContent = `${installed}/${allWhisperModels.length} installed`;
        }
        
        // Update Voice Assistant whisper dropdown
        updateVoiceAssistantWhisperDropdown();
        
        // Render model library
        renderWhisperModels(allWhisperModels);
    } catch (e) {
        console.error('Failed to load whisper models:', e);
    }
}

function updateVoiceAssistantWhisperDropdown() {
    const voiceWhisperSelect = document.getElementById('voiceAssistantWhisper');
    if (!voiceWhisperSelect || allWhisperModels.length === 0) return;
    
    const installedModels = allWhisperModels.filter(m => m.installed);
    
    if (installedModels.length > 0) {
        // Sort: multilingual first
        const sorted = [...installedModels].sort((a, b) => {
            const aIsEn = a.name.endsWith('.en');
            const bIsEn = b.name.endsWith('.en');
            if (aIsEn && !bIsEn) return 1;
            if (!aIsEn && bIsEn) return -1;
            return a.name.localeCompare(b.name);
        });
        
        voiceWhisperSelect.innerHTML = sorted.map(m => 
            `<option value="${m.name}">${m.name} (${m.language}) - ${m.size}</option>`
        ).join('');
        
        // Restore saved selection
        const saved = localStorage.getItem('voiceAssistantWhisper');
        if (saved && sorted.some(m => m.name === saved)) {
            voiceWhisperSelect.value = saved;
        }
    } else {
        voiceWhisperSelect.innerHTML = '<option value="">No models installed - download below</option>';
    }
    
    checkWhisperModelCompatibility();
}

function filterWhisperModels() {
    const langFilter = document.getElementById('whisperLanguageFilter')?.value || '';
    const statusFilter = document.getElementById('whisperStatusFilter')?.value || '';
    
    let filtered = allWhisperModels;
    
    if (langFilter === 'multilingual') {
        filtered = filtered.filter(m => !m.name.endsWith('.en'));
    } else if (langFilter === 'english') {
        filtered = filtered.filter(m => m.name.endsWith('.en'));
    }
    
    if (statusFilter === 'installed') {
        filtered = filtered.filter(m => m.installed);
    } else if (statusFilter === 'available') {
        filtered = filtered.filter(m => !m.installed);
    }
    
    renderWhisperModels(filtered);
}

function renderWhisperModels(models) {
    const library = document.getElementById('whisperModelLibrary');
    if (!library) return;
    
    if (models.length === 0) {
        library.innerHTML = '<div class="col-span-full text-secondary text-center py-4">No models found matching your filters</div>';
        return;
    }
    
    // Sort by size order: tiny, base, small, medium, large
    const sizeOrder = ['tiny', 'base', 'small', 'medium', 'large'];
    const sorted = [...models].sort((a, b) => {
        const aBase = a.name.replace('.en', '');
        const bBase = b.name.replace('.en', '');
        const aIdx = sizeOrder.findIndex(s => aBase.startsWith(s));
        const bIdx = sizeOrder.findIndex(s => bBase.startsWith(s));
        if (aIdx !== bIdx) return aIdx - bIdx;
        // Within same size, multilingual before .en
        if (a.name.endsWith('.en') && !b.name.endsWith('.en')) return 1;
        if (!a.name.endsWith('.en') && b.name.endsWith('.en')) return -1;
        return 0;
    });
    
    library.innerHTML = sorted.map(m => {
        const isMultilingual = !m.name.endsWith('.en');
        const langIcon = isMultilingual ? '🌐' : '🇺🇸';
        const langClass = isMultilingual ? 'text-cyan-400' : 'text-blue-400';
        
        return `
            <div class="whisper-card ${m.installed ? 'whisper-installed' : ''} bg-tertiary rounded-lg p-3 flex flex-col gap-2" data-model="${m.name}">
                <div class="flex items-center justify-between">
                    <span class="font-medium">${m.name}</span>
                    <span class="${langClass} text-sm">${langIcon}</span>
                </div>
                <div class="flex items-center justify-between text-xs text-secondary">
                    <span>${m.size}</span>
                    <span>${m.language}</span>
                </div>
                <div class="mt-1">
                    ${m.installed 
                        ? '<span class="text-green-400 text-xs flex items-center gap-1"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="20 6 9 17 4 12"/></svg> Installed</span>'
                        : `<button onclick="downloadWhisperModel('${m.name}')" class="btn btn-secondary text-xs py-1 px-2 w-full">
                            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>
                            Download
                           </button>`
                    }
                </div>
            </div>
        `;
    }).join('');
}

async function downloadWhisperModel(name) {
    const card = document.querySelector(`.whisper-card[data-model="${name}"]`);
    if (card) {
        const btn = card.querySelector('button');
        if (btn) {
            btn.disabled = true;
            btn.innerHTML = '<span class="animate-pulse">⏳ Downloading...</span>';
        }
    }
    
    showAlert(`Downloading whisper model: ${name}... This may take a while depending on model size.`, { title: 'Downloading', type: 'info' });
    
    try {
        const resp = await fetch('/v1/audio/download', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ type: 'whisper', name: name })
        });
        
        if (!resp.ok) {
            const err = await resp.json();
            throw new Error(err.error?.message || 'Download failed');
        }
        
        showAlert(`Whisper model "${name}" downloaded successfully!`, { title: 'Success', type: 'success' });
        loadAvailableWhisperModels();
        refreshAudioStatus();
    } catch (e) {
        showAlert(`Failed to download model: ${e.message}`, { title: 'Error', type: 'error' });
        if (card) {
            const btn = card.querySelector('button');
            if (btn) {
                btn.disabled = false;
                btn.innerHTML = '<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg> Download';
            }
        }
    }
}

function onVoiceVoiceChange() {
    const select = document.getElementById('voiceAssistantVoice');
    if (select && select.value) {
        localStorage.setItem('voiceAssistantVoice', select.value);
    }
}

function onVoiceLangChange() {
    const select = document.getElementById('voiceAssistantLang');
    if (select && select.value) {
        const prevLang = localStorage.getItem('voiceAssistantLang');
        localStorage.setItem('voiceAssistantLang', select.value);
        checkWhisperModelCompatibility();
        
        // Clear conversation when language changes to avoid confusion
        if (prevLang && prevLang !== select.value && voiceChatHistory.length > 0) {
            clearVoiceConversation();
            showNotification('Conversation cleared for new language', 'info');
        }
    }
}

function onVoiceWhisperChange() {
    const select = document.getElementById('voiceAssistantWhisper');
    if (select && select.value) {
        localStorage.setItem('voiceAssistantWhisper', select.value);
        checkWhisperModelCompatibility();
    }
}

function checkWhisperModelCompatibility() {
    const lang = document.getElementById('voiceAssistantLang')?.value || 'en';
    const model = document.getElementById('voiceAssistantWhisper')?.value || 'base';
    const hint = document.getElementById('whisperModelHint');
    
    // Show warning if non-English language selected with English-only model
    if (lang !== 'en' && lang !== 'auto' && model.endsWith('.en')) {
        if (hint) hint.classList.remove('hidden');
    } else {
        if (hint) hint.classList.add('hidden');
    }
}

// Restore saved language selection on page load
function restoreVoiceLanguageSettings() {
    const savedLang = localStorage.getItem('voiceAssistantLang');
    if (savedLang) {
        const langSelect = document.getElementById('voiceAssistantLang');
        if (langSelect) langSelect.value = savedLang;
    }
    const savedSttLang = localStorage.getItem('sttLanguage');
    if (savedSttLang) {
        const sttSelect = document.getElementById('sttLanguage');
        if (sttSelect) sttSelect.value = savedSttLang;
    }
    const savedWhisper = localStorage.getItem('voiceAssistantWhisper');
    if (savedWhisper) {
        const whisperSelect = document.getElementById('voiceAssistantWhisper');
        if (whisperSelect) whisperSelect.value = savedWhisper;
    }
    checkWhisperModelCompatibility();
}

// Save STT language when changed
document.addEventListener('DOMContentLoaded', () => {
    const sttLangSelect = document.getElementById('sttLanguage');
    if (sttLangSelect) {
        sttLangSelect.addEventListener('change', () => {
            localStorage.setItem('sttLanguage', sttLangSelect.value);
        });
    }
    restoreVoiceLanguageSettings();
});

async function loadVoiceAssistantModels() {
    const select = document.getElementById('voiceAssistantModel');
    if (!select) return;
    
    try {
        const resp = await fetch('/models');
        const data = await resp.json();
        const models = data.data || [];
        
        // Filter to only LLM models (not embedding models)
        const llmModels = models.filter(m => m.type !== 'embedding');
        
        select.innerHTML = '';
        
        if (llmModels.length === 0) {
            select.innerHTML = '<option value="">No models available</option>';
            return;
        }
        
        llmModels.forEach(m => {
            const opt = document.createElement('option');
            opt.value = m.id;
            opt.textContent = m.id;
            select.appendChild(opt);
        });
        
        // Try to restore saved selection
        const saved = localStorage.getItem('voiceAssistantModel');
        if (saved && llmModels.some(m => m.id === saved)) {
            select.value = saved;
        }
        
        console.log('[VOICE] Loaded models:', llmModels.length);
    } catch (e) {
        console.error('Failed to load voice assistant models:', e);
        select.innerHTML = '<option value="">Error loading models</option>';
    }
}

function onVoiceModelChange() {
    const select = document.getElementById('voiceAssistantModel');
    if (select && select.value) {
        localStorage.setItem('voiceAssistantModel', select.value);
        console.log('[VOICE] Model changed to:', select.value);
    }
}

async function refreshAudioStatus() {
    try {
        // Check audio status
        const statusResp = await fetch('/v1/audio/status');
        if (statusResp.ok) {
            const status = await statusResp.json();
            
            // Update ASR status
            const asrAvailable = status.asr?.available || false;
            const whisperPath = status.asr?.whisper_path || '';
            const whisperModels = status.asr?.models || [];
            
            const whisperStatus = document.getElementById('whisperStatus');
            const whisperModelsDiv = document.getElementById('whisperModels');
            const whisperSetup = document.getElementById('whisperSetup');
            
            if (whisperStatus) {
                if (asrAvailable) {
                    whisperStatus.className = 'badge badge-success';
                    whisperStatus.textContent = 'Ready';
                } else if (whisperPath) {
                    whisperStatus.className = 'badge badge-warning';
                    whisperStatus.textContent = 'No models';
                } else {
                    whisperStatus.className = 'badge badge-error';
                    whisperStatus.textContent = 'Not installed';
                }
            }
            
            if (whisperModelsDiv) {
                if (asrAvailable && whisperModels.length > 0) {
                    whisperModelsDiv.innerHTML = whisperModels.map(m => `
                        <div class="flex items-center justify-between">
                            <span>${m}</span>
                            <span class="text-green-400">✓</span>
                        </div>
                    `).join('');
                } else if (whisperPath) {
                    whisperModelsDiv.innerHTML = '<div class="text-secondary">No models installed. Download one below.</div>';
                } else {
                    whisperModelsDiv.innerHTML = '<div class="text-secondary">Whisper.cpp not installed</div>';
                }
            }
            
            // Update whisper model dropdown with installed models
            const whisperModelSelect = document.getElementById('whisperModel');
            if (whisperModelSelect && whisperModels.length > 0) {
                whisperModelSelect.innerHTML = whisperModels.map(m => 
                    `<option value="${m}">${m}</option>`
                ).join('');
            }
            
            // Update Voice Assistant whisper dropdown with installed models
            const voiceWhisperSelect = document.getElementById('voiceAssistantWhisper');
            if (voiceWhisperSelect && whisperModels.length > 0) {
                // Sort models: multilingual first, then .en models
                const sortedModels = [...whisperModels].sort((a, b) => {
                    const aIsEn = a.endsWith('.en');
                    const bIsEn = b.endsWith('.en');
                    if (aIsEn && !bIsEn) return 1;
                    if (!aIsEn && bIsEn) return -1;
                    return a.localeCompare(b);
                });
                
                voiceWhisperSelect.innerHTML = sortedModels.map(m => {
                    const isEnglishOnly = m.endsWith('.en');
                    const label = isEnglishOnly 
                        ? `${m} (English only)` 
                        : `${m} (multilingual)`;
                    return `<option value="${m}">${label}</option>`;
                }).join('');
                
                // Restore saved selection or default to first multilingual
                const savedWhisper = localStorage.getItem('voiceAssistantWhisper');
                if (savedWhisper && sortedModels.includes(savedWhisper)) {
                    voiceWhisperSelect.value = savedWhisper;
                } else {
                    // Default to first multilingual model
                    const firstMultilingual = sortedModels.find(m => !m.endsWith('.en'));
                    if (firstMultilingual) {
                        voiceWhisperSelect.value = firstMultilingual;
                    }
                }
                checkWhisperModelCompatibility();
            } else if (voiceWhisperSelect) {
                voiceWhisperSelect.innerHTML = '<option value="">No models installed</option>';
            }
            
            if (whisperSetup) {
                whisperSetup.classList.toggle('hidden', !!whisperPath);
            }
            
            // Update TTS status
            const ttsAvailable = status.tts?.available || false;
            const piperPath = status.tts?.piper_path || '';
            const piperVoicesCount = status.tts?.voices || 0;
            
            const piperStatus = document.getElementById('piperStatus');
            const piperVoicesDiv = document.getElementById('piperVoices');
            const piperSetup = document.getElementById('piperSetup');
            
            if (piperStatus) {
                if (ttsAvailable) {
                    piperStatus.className = 'badge badge-success';
                    piperStatus.textContent = `${piperVoicesCount} voice${piperVoicesCount !== 1 ? 's' : ''}`;
                } else if (piperPath) {
                    piperStatus.className = 'badge badge-warning';
                    piperStatus.textContent = 'No voices';
                } else {
                    piperStatus.className = 'badge badge-error';
                    piperStatus.textContent = 'Not installed';
                }
            }
            
            if (piperVoicesDiv) {
                if (ttsAvailable) {
                    piperVoicesDiv.innerHTML = `<div class="text-secondary">${piperVoicesCount} voice${piperVoicesCount !== 1 ? 's' : ''} available</div>`;
                } else if (piperPath) {
                    piperVoicesDiv.innerHTML = '<div class="text-secondary">No voices installed. Download one below.</div>';
                } else {
                    piperVoicesDiv.innerHTML = '<div class="text-secondary">Piper not installed</div>';
                }
            }
            
            if (piperSetup) {
                piperSetup.classList.toggle('hidden', !!piperPath);
            }
            
            // Enable/disable transcribe button
            const transcribeBtn = document.getElementById('transcribeBtn');
            if (transcribeBtn && audioFile) {
                transcribeBtn.disabled = !asrAvailable;
            }
        }
    } catch (e) {
        console.error('Failed to refresh audio status:', e);
    }
}

async function setupWhisperBinary() {
    const btn = event.target;
    btn.disabled = true;
    btn.innerHTML = '<svg class="animate-spin h-4 w-4" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" fill="none"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg> Installing...';
    
    try {
        const resp = await fetch('/v1/audio/setup/whisper', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ install_binary: true })
        });
        
        if (!resp.ok) {
            const err = await resp.json();
            throw new Error(err.error?.message || 'Installation failed');
        }
        
        showAlert('Whisper.cpp installed successfully!', { title: 'Success', type: 'success' });
        refreshAudioStatus();
    } catch (e) {
        showAlert('Failed to install Whisper.cpp: ' + e.message, { title: 'Error', type: 'error' });
    } finally {
        btn.disabled = false;
        btn.innerHTML = '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg> Install Whisper.cpp';
    }
}

async function setupPiperBinary() {
    const btn = event.target;
    btn.disabled = true;
    btn.innerHTML = '<svg class="animate-spin h-4 w-4" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" fill="none"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg> Installing...';
    
    try {
        const resp = await fetch('/v1/audio/setup/piper', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ install_binary: true })
        });
        
        if (!resp.ok) {
            const err = await resp.json();
            throw new Error(err.error?.message || 'Installation failed');
        }
        
        showAlert('Piper installed successfully!', { title: 'Success', type: 'success' });
        refreshAudioStatus();
    } catch (e) {
        showAlert('Failed to install Piper: ' + e.message, { title: 'Error', type: 'error' });
    } finally {
        btn.disabled = false;
        btn.innerHTML = '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg> Install Piper';
    }
}

function handleAudioFileSelect(event) {
    const file = event.target.files[0];
    if (file) {
        audioFile = file;
        document.getElementById('audioFileName').textContent = file.name;
        document.getElementById('transcribeBtn').disabled = false;
    }
}

async function toggleRecording() {
    if (isRecording) {
        stopRecording();
    } else {
        startRecording();
    }
}

async function startRecording() {
    try {
        const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
        
        // Try to use wav format if supported, otherwise use webm
        const mimeType = MediaRecorder.isTypeSupported('audio/webm;codecs=opus') 
            ? 'audio/webm;codecs=opus' 
            : 'audio/webm';
        
        mediaRecorder = new MediaRecorder(stream, { mimeType });
        audioChunks = [];
        
        mediaRecorder.ondataavailable = (e) => {
            if (e.data.size > 0) {
                audioChunks.push(e.data);
            }
        };
        
        mediaRecorder.onstop = async () => {
            const blob = new Blob(audioChunks, { type: mimeType });
            // Create a file with webm extension so whisper knows the format
            const ext = mimeType.includes('webm') ? 'webm' : 'wav';
            audioFile = new File([blob], `recording.${ext}`, { type: mimeType });
            document.getElementById('audioFileName').textContent = 'Recording captured (' + (blob.size / 1024).toFixed(1) + ' KB)';
            document.getElementById('transcribeBtn').disabled = false;
            stream.getTracks().forEach(t => t.stop());
            
            // Auto-transcribe if enabled
            if (document.getElementById('autoTranscribe')?.checked) {
                await transcribeAudio();
            }
        };
        
        mediaRecorder.start(100); // Collect data every 100ms
        isRecording = true;
        document.getElementById('recordBtn').classList.add('bg-red-600', 'hover:bg-red-700');
        document.getElementById('recordBtn').classList.remove('btn-secondary');
        document.getElementById('recordIcon').innerHTML = '<rect x="6" y="6" width="12" height="12" rx="2"></rect>';
        document.getElementById('recordText').textContent = 'Stop';
    } catch (e) {
        showAlert('Could not access microphone: ' + e.message, { title: 'Microphone Error', type: 'error' });
    }
}

function stopRecording() {
    if (mediaRecorder && isRecording) {
        mediaRecorder.stop();
        isRecording = false;
        document.getElementById('recordBtn').classList.remove('bg-red-600', 'hover:bg-red-700');
        document.getElementById('recordBtn').classList.add('btn-secondary');
        document.getElementById('recordIcon').innerHTML = '<circle cx="12" cy="12" r="6"></circle>';
        document.getElementById('recordText').textContent = 'Record';
    }
}

async function transcribeAudio() {
    if (!audioFile) {
        showAlert('Please select or record an audio file first', { title: 'No Audio', type: 'warning' });
        return;
    }
    
    const model = document.getElementById('whisperModel').value;
    const language = document.getElementById('sttLanguage').value;
    const formData = new FormData();
    formData.append('file', audioFile);
    formData.append('model', model);
    if (language && language !== 'auto') {
        formData.append('language', language);
    }
    
    const btn = document.getElementById('transcribeBtn');
    btn.disabled = true;
    btn.innerHTML = '<svg class="animate-spin h-4 w-4" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" fill="none"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg> Transcribing...';
    
    try {
        const resp = await fetch('/v1/audio/transcriptions', {
            method: 'POST',
            body: formData
        });
        
        if (!resp.ok) {
            const err = await resp.json();
            throw new Error(err.error?.message || 'Transcription failed');
        }
        
        const data = await resp.json();
        document.getElementById('transcriptionText').textContent = data.text;
        document.getElementById('transcriptionResult').classList.remove('hidden');
    } catch (e) {
        showAlert('Transcription failed: ' + e.message, { title: 'Error', type: 'error' });
    } finally {
        btn.disabled = false;
        btn.innerHTML = '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 1a3 3 0 0 0-3 3v8a3 3 0 0 0 6 0V4a3 3 0 0 0-3-3z"></path><path d="M19 10v2a7 7 0 0 1-14 0v-2"></path></svg> Transcribe';
    }
}

function copyTranscription() {
    const text = document.getElementById('transcriptionText').textContent;
    navigator.clipboard.writeText(text);
    showAlert('Copied to clipboard!', { title: 'Copied', type: 'success' });
}

function sendTranscriptionToChat() {
    const text = document.getElementById('transcriptionText').textContent;
    document.getElementById('chatInput').value = text;
    switchTab('chat');
}

// Chat Voice Input (like ChatGPT mic button)
let chatVoiceRecording = false;
let chatVoiceRecorder = null;
let chatVoiceChunks = [];

async function startChatVoiceInput() {
    if (chatVoiceRecording) return;
    
    try {
        const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
        const mimeType = MediaRecorder.isTypeSupported('audio/webm;codecs=opus') 
            ? 'audio/webm;codecs=opus' 
            : 'audio/webm';
        
        chatVoiceRecorder = new MediaRecorder(stream, { mimeType });
        chatVoiceChunks = [];
        
        chatVoiceRecorder.ondataavailable = (e) => {
            if (e.data.size > 0) chatVoiceChunks.push(e.data);
        };
        
        chatVoiceRecorder.onstop = async () => {
            stream.getTracks().forEach(t => t.stop());
            if (chatVoiceChunks.length === 0) return;
            
            const blob = new Blob(chatVoiceChunks, { type: mimeType });
            if (blob.size < 1000) return; // Too short
            
            await transcribeChatVoice(blob);
        };
        
        chatVoiceRecorder.start(100);
        chatVoiceRecording = true;
        
        // Update UI
        const btn = document.getElementById('chatVoiceBtn');
        btn.classList.add('bg-red-500', 'text-white');
        btn.classList.remove('bg-tertiary');
        document.getElementById('chatVoiceIcon').innerHTML = '<circle cx="12" cy="12" r="8" fill="currentColor"/>';
        
    } catch (e) {
        showAlert('Could not access microphone: ' + e.message, { title: 'Microphone Error', type: 'error' });
    }
}

function stopChatVoiceInput() {
    if (!chatVoiceRecording || !chatVoiceRecorder) return;
    
    chatVoiceRecorder.stop();
    chatVoiceRecording = false;
    
    // Reset UI
    const btn = document.getElementById('chatVoiceBtn');
    btn.classList.remove('bg-red-500', 'text-white');
    btn.classList.add('bg-tertiary');
    document.getElementById('chatVoiceIcon').innerHTML = '<path d="M12 1a3 3 0 0 0-3 3v8a3 3 0 0 0 6 0V4a3 3 0 0 0-3-3z"></path><path d="M19 10v2a7 7 0 0 1-14 0v-2"></path><line x1="12" y1="19" x2="12" y2="23"></line><line x1="8" y1="23" x2="16" y2="23"></line>';
}

async function transcribeChatVoice(audioBlob) {
    const statusBadge = document.getElementById('statusBadge');
    const originalStatus = statusBadge.textContent;
    statusBadge.textContent = 'Transcribing...';
    statusBadge.className = 'text-xs font-medium text-yellow-400';
    
    try {
        // Use Chat tab voice settings
        const whisperModel = document.getElementById('chatWhisperModel')?.value || localStorage.getItem('chatWhisperModel') || 'base';
        const language = document.getElementById('chatVoiceLang')?.value || localStorage.getItem('chatVoiceLang') || 'en';
        const formData = new FormData();
        formData.append('file', new File([audioBlob], 'voice.webm', { type: audioBlob.type }));
        formData.append('model', whisperModel);
        if (language && language !== 'auto') {
            formData.append('language', language);
        }
        
        const resp = await fetch('/v1/audio/transcriptions', {
            method: 'POST',
            body: formData
        });
        
        if (!resp.ok) {
            throw new Error('Transcription failed');
        }
        
        const data = await resp.json();
        let text = data.text || '';
        
        // Clean up whisper timestamps
        text = text.replace(/\[\d+:\d+:\d+\.\d+ --> \d+:\d+:\d+\.\d+\]\s*/g, '').trim();
        
        if (text) {
            // Put transcribed text in chat input
            const input = document.getElementById('chatInput');
            input.value = text;
            input.focus();
            autoResizeTextarea(input);
        }
        
        statusBadge.textContent = 'Ready';
        statusBadge.className = 'text-xs font-medium text-green-400';
        
    } catch (e) {
        console.error('Voice transcription error:', e);
        statusBadge.textContent = 'Transcription failed';
        statusBadge.className = 'text-xs font-medium text-red-400';
        setTimeout(() => {
            statusBadge.textContent = originalStatus;
            statusBadge.className = 'text-xs font-medium';
        }, 2000);
    }
}

// Chat Voice Settings Functions
function saveChatVoiceSettings() {
    const lang = document.getElementById('chatVoiceLang')?.value;
    const model = document.getElementById('chatWhisperModel')?.value;
    const voice = document.getElementById('chatTTSVoice')?.value;
    
    if (lang) localStorage.setItem('chatVoiceLang', lang);
    if (model) localStorage.setItem('chatWhisperModel', model);
    if (voice) localStorage.setItem('chatTTSVoice', voice);
}

async function loadChatVoiceSettings() {
    // Load whisper models
    try {
        const resp = await fetch('/v1/audio/whisper-models');
        if (resp.ok) {
            const data = await resp.json();
            const models = (data.models || []).filter(m => m.installed);
            const select = document.getElementById('chatWhisperModel');
            if (select && models.length > 0) {
                select.innerHTML = models.map(m => 
                    `<option value="${m.name}">${m.name} (${m.language})</option>`
                ).join('');
                const saved = localStorage.getItem('chatWhisperModel');
                if (saved && models.some(m => m.name === saved)) {
                    select.value = saved;
                }
            } else if (select) {
                select.innerHTML = '<option value="">No models - see Audio tab</option>';
            }
        }
    } catch (e) { console.error('Failed to load whisper models:', e); }
    
    // Load TTS voices
    try {
        const resp = await fetch('/v1/audio/voices');
        if (resp.ok) {
            const data = await resp.json();
            const voices = (data.voices || []).filter(v => v.installed);
            const select = document.getElementById('chatTTSVoice');
            if (select && voices.length > 0) {
                select.innerHTML = voices.map(v => 
                    `<option value="${v.name}">${v.name} (${v.language})</option>`
                ).join('');
                const saved = localStorage.getItem('chatTTSVoice');
                if (saved && voices.some(v => v.name === saved)) {
                    select.value = saved;
                }
            } else if (select) {
                select.innerHTML = '<option value="">No voices - see Audio tab</option>';
            }
        }
    } catch (e) { console.error('Failed to load TTS voices:', e); }
    
    // Restore language setting
    const savedLang = localStorage.getItem('chatVoiceLang');
    if (savedLang) {
        const langSelect = document.getElementById('chatVoiceLang');
        if (langSelect) langSelect.value = savedLang;
    }
}

function updateTTSSpeedLabel() {
    const val = document.getElementById('ttsSpeed').value;
    document.getElementById('ttsSpeedLabel').textContent = parseFloat(val).toFixed(1) + 'x';
}

async function generateSpeech() {
    const text = document.getElementById('ttsText').value.trim();
    if (!text) {
        showAlert('Please enter text to speak', { title: 'No Text', type: 'warning' });
        return;
    }
    
    const voice = document.getElementById('ttsVoice').value;
    const speed = parseFloat(document.getElementById('ttsSpeed').value);
    
    const btn = document.getElementById('speakBtn');
    btn.disabled = true;
    btn.innerHTML = '<svg class="animate-spin h-4 w-4" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" fill="none"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg> Generating...';
    
    try {
        const resp = await fetch('/v1/audio/speech', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                input: text,
                voice: voice,
                speed: speed,
                response_format: 'wav'
            })
        });
        
        if (!resp.ok) {
            const err = await resp.json();
            throw new Error(err.error?.message || 'Speech generation failed');
        }
        
        ttsAudioBlob = await resp.blob();
        const audioUrl = URL.createObjectURL(ttsAudioBlob);
        
        const audioEl = document.getElementById('ttsAudio');
        audioEl.src = audioUrl;
        document.getElementById('ttsPlayer').classList.remove('hidden');
        audioEl.play();
    } catch (e) {
        showAlert('Speech generation failed: ' + e.message, { title: 'Error', type: 'error' });
    } finally {
        btn.disabled = false;
        btn.innerHTML = '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5"></polygon><path d="M15.54 8.46a5 5 0 0 1 0 7.07"></path></svg> Generate Speech';
    }
}

function downloadTTSAudio() {
    if (!ttsAudioBlob) return;
    const a = document.createElement('a');
    a.href = URL.createObjectURL(ttsAudioBlob);
    a.download = 'speech.wav';
    a.click();
}

// Voice Assistant Functions
let voiceChatRecording = false;
let voiceChatRecorder = null;
let voiceChatChunks = [];
let voiceChatHistory = [];

async function startVoiceChat() {
    if (voiceChatRecording) return;
    
    try {
        const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
        const mimeType = MediaRecorder.isTypeSupported('audio/webm;codecs=opus') 
            ? 'audio/webm;codecs=opus' 
            : 'audio/webm';
        
        voiceChatRecorder = new MediaRecorder(stream, { mimeType });
        voiceChatChunks = [];
        
        voiceChatRecorder.ondataavailable = (e) => {
            if (e.data.size > 0) voiceChatChunks.push(e.data);
        };
        
        voiceChatRecorder.onstop = async () => {
            stream.getTracks().forEach(t => t.stop());
            if (voiceChatChunks.length === 0) return;
            
            const blob = new Blob(voiceChatChunks, { type: mimeType });
            if (blob.size < 1000) return; // Too short, ignore
            
            await processVoiceChat(blob);
        };
        
        voiceChatRecorder.start(100);
        voiceChatRecording = true;
        
        // Update UI
        const btn = document.getElementById('voicePTTBtn');
        btn.classList.add('ring-4', 'ring-cyan-300', 'scale-110');
        document.getElementById('voicePTTIcon').innerHTML = '<circle cx="12" cy="12" r="8" fill="currentColor"/>';
        document.getElementById('voiceAssistantStatus').textContent = 'Listening...';
        document.getElementById('voiceWaveform').classList.remove('hidden');
        
    } catch (e) {
        showAlert('Could not access microphone: ' + e.message, { title: 'Microphone Error', type: 'error' });
    }
}

function stopVoiceChat() {
    if (!voiceChatRecording || !voiceChatRecorder) return;
    
    voiceChatRecorder.stop();
    voiceChatRecording = false;
    
    // Reset UI
    const btn = document.getElementById('voicePTTBtn');
    btn.classList.remove('ring-4', 'ring-cyan-300', 'scale-110');
    document.getElementById('voicePTTIcon').innerHTML = '<path d="M12 1a3 3 0 0 0-3 3v8a3 3 0 0 0 6 0V4a3 3 0 0 0-3-3z"></path><path d="M19 10v2a7 7 0 0 1-14 0v-2"></path><line x1="12" y1="19" x2="12" y2="23"></line><line x1="8" y1="23" x2="16" y2="23"></line>';
    document.getElementById('voiceWaveform').classList.add('hidden');
    document.getElementById('voiceAssistantStatus').textContent = 'Processing...';
}

async function processVoiceChat(audioBlob) {
    const statusEl = document.getElementById('voiceAssistantStatus');
    const conversationEl = document.getElementById('voiceConversation');
    const clearBtn = document.getElementById('voiceClearBtn');
    
    try {
        // Step 1: Transcribe audio
        statusEl.textContent = 'Transcribing...';
        const model = document.getElementById('voiceAssistantWhisper')?.value || 'base';
        const language = document.getElementById('voiceAssistantLang')?.value || 'en';
        const formData = new FormData();
        formData.append('file', new File([audioBlob], 'voice.webm', { type: audioBlob.type }));
        formData.append('model', model);
        if (language && language !== 'auto') {
            formData.append('language', language);
        }
        
        const transcribeResp = await fetch('/v1/audio/transcriptions', {
            method: 'POST',
            body: formData
        });
        
        if (!transcribeResp.ok) {
            throw new Error('Transcription failed');
        }
        
        const transcription = await transcribeResp.json();
        let userText = transcription.text || '';
        
        // Clean up whisper timestamps if present
        userText = userText.replace(/\[\d+:\d+:\d+\.\d+ --> \d+:\d+:\d+\.\d+\]\s*/g, '').trim();
        
        if (!userText || userText.length < 2) {
            statusEl.textContent = 'Could not understand audio. Try again.';
            setTimeout(() => { statusEl.textContent = 'Press and hold to speak'; }, 2000);
            return;
        }
        
        // Show user message
        conversationEl.classList.remove('hidden');
        clearBtn.classList.remove('hidden');
        addVoiceMessage('user', userText);
        
        // Step 2: Send to LLM
        voiceChatHistory.push({ role: 'user', content: userText });
        
        // Get the selected model from the Voice Assistant dropdown
        const selectedModel = document.getElementById('voiceAssistantModel')?.value;
        console.log('[VOICE CHAT] Using model:', selectedModel);
        
        if (!selectedModel) {
            throw new Error('No model selected. Please select a model above.');
        }
        
        statusEl.textContent = 'Thinking... (' + selectedModel.substring(0, 20) + ')';
        
        // Build messages with system prompt for voice assistant
        const messages = [
            { 
                role: 'system', 
                content: 'You are a helpful voice assistant. Keep your responses concise and conversational since they will be spoken aloud. Aim for 1-3 sentences unless the user asks for detailed information.'
            },
            ...voiceChatHistory.slice(-10) // Keep last 10 messages for context
        ];
        
        console.log('[VOICE CHAT] Sending request with messages:', JSON.stringify(messages));
        
        const chatResp = await fetch('/v1/chat/completions', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                model: selectedModel,
                messages: messages,
                stream: false,
                temperature: parseFloat(document.getElementById('temperature')?.value || 0.7),
                max_tokens: parseInt(document.getElementById('maxTokens')?.value || 512)
            })
        });
        
        console.log('[VOICE CHAT] Response status:', chatResp.status);
        
        if (!chatResp.ok) {
            const errText = await chatResp.text();
            console.error('[VOICE CHAT] Error response:', errText);
            let errMsg = 'Chat request failed';
            try {
                const errData = JSON.parse(errText);
                errMsg = errData.error?.message || errMsg;
            } catch(e) {}
            throw new Error(errMsg + ` (${chatResp.status})`);
        }
        
        const chatData = await chatResp.json();
        console.log('[VOICE CHAT] Response data:', chatData);
        const assistantText = chatData.choices?.[0]?.message?.content || 'Sorry, I could not generate a response.';
        
        // Show assistant message
        addVoiceMessage('assistant', assistantText);
        voiceChatHistory.push({ role: 'assistant', content: assistantText });
        
        // Step 3: Speak response (if enabled)
        const autoSpeak = document.getElementById('voiceAutoSpeak').checked;
        if (autoSpeak) {
            statusEl.textContent = 'Speaking...';
            await speakText(assistantText);
        }
        
        statusEl.textContent = 'Press and hold to speak';
        
    } catch (e) {
        console.error('Voice chat error:', e);
        // Show error as a message in the conversation
        addVoiceMessage('assistant', 'Error: ' + e.message);
        statusEl.textContent = 'Error - try again';
        setTimeout(() => { statusEl.textContent = 'Press and hold to speak'; }, 5000);
    }
}

function addVoiceMessage(role, text) {
    const conversationEl = document.getElementById('voiceConversation');
    const msgDiv = document.createElement('div');
    msgDiv.className = role === 'user' 
        ? 'bg-tertiary rounded-lg p-3 ml-8'
        : 'bg-accent-alpha rounded-lg p-3 mr-8 border border-accent';
    
    const roleLabel = document.createElement('div');
    roleLabel.className = 'text-xs font-medium mb-1 ' + (role === 'user' ? 'text-secondary' : 'text-accent');
    roleLabel.textContent = role === 'user' ? 'You' : 'Assistant';
    
    const textP = document.createElement('p');
    textP.className = 'text-sm';
    textP.textContent = text;
    
    msgDiv.appendChild(roleLabel);
    msgDiv.appendChild(textP);
    conversationEl.appendChild(msgDiv);
    conversationEl.scrollTop = conversationEl.scrollHeight;
}

async function speakText(text) {
    try {
        // Use Voice Assistant's voice dropdown, fallback to TTS section dropdown
        const voice = document.getElementById('voiceAssistantVoice')?.value || 
                      document.getElementById('ttsVoice')?.value || 
                      'en_US-amy-medium';
        const resp = await fetch('/v1/audio/speech', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                input: text,
                voice: voice,
                response_format: 'wav'
            })
        });
        
        if (!resp.ok) {
            console.error('TTS failed');
            return;
        }
        
        const audioBlob = await resp.blob();
        const audioUrl = URL.createObjectURL(audioBlob);
        const audio = new Audio(audioUrl);
        
        return new Promise((resolve) => {
            audio.onended = () => {
                URL.revokeObjectURL(audioUrl);
                resolve();
            };
            audio.onerror = () => {
                URL.revokeObjectURL(audioUrl);
                resolve();
            };
            audio.play();
        });
    } catch (e) {
        console.error('TTS error:', e);
    }
}

function clearVoiceConversation() {
    voiceChatHistory = [];
    document.getElementById('voiceConversation').innerHTML = '';
    document.getElementById('voiceConversation').classList.add('hidden');
    document.getElementById('voiceClearBtn').classList.add('hidden');
    document.getElementById('voiceAssistantStatus').textContent = 'Press and hold to speak';
}

async function setupWhisper() {
    const model = document.getElementById('whisperSetupModel').value;
    showAlert(`Downloading Whisper ${model} model... This may take a while.`, { title: 'Downloading', type: 'info' });
    
    try {
        const resp = await fetch('/v1/audio/setup/whisper', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ model })
        });
        
        if (!resp.ok) {
            const err = await resp.json();
            throw new Error(err.error?.message || 'Setup failed');
        }
        
        showAlert(`Whisper ${model} model installed successfully!`, { title: 'Success', type: 'success' });
        refreshAudioStatus();
    } catch (e) {
        showAlert('Setup failed: ' + e.message, { title: 'Error', type: 'error' });
    }
}

async function setupPiper() {
    const voice = document.getElementById('piperSetupVoice').value;
    showAlert(`Downloading ${voice} voice... This may take a while.`, { title: 'Downloading', type: 'info' });
    
    try {
        const resp = await fetch('/v1/audio/setup/piper', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ voice })
        });
        
        if (!resp.ok) {
            const err = await resp.json();
            throw new Error(err.error?.message || 'Setup failed');
        }
        
        showAlert(`Voice ${voice} installed successfully!`, { title: 'Success', type: 'success' });
        refreshAudioStatus();
    } catch (e) {
        showAlert('Setup failed: ' + e.message, { title: 'Error', type: 'error' });
    }
}

