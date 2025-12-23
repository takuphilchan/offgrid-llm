function scrollTerminalToBottom() {
    if (userScrolledUp) return; // Don't auto-scroll if user scrolled up
    const terminal = document.getElementById('terminalOutput');
    terminal.scrollTop = terminal.scrollHeight;
}

function clearTerminal() {
    terminalOutputBuffer = ''; // Clear any buffered output
    userScrolledUp = false;
    const pre = document.getElementById('terminalPre');
    pre.textContent = 'OffGrid Terminal v0.2.9\nConnected to real offgrid binary\nType \'help\' for available commands\n';
}

function killCommand() {
    if (currentTerminalAbort) {
        currentTerminalAbort.abort();
        currentTerminalAbort = null;
    }
    terminalRunning = false;
    const inputLine = document.getElementById('terminalInputLine');
    inputLine.classList.remove('terminal-running');
    document.getElementById('terminalInput').disabled = false;
    document.getElementById('killBtn').classList.add('hidden');
    addTerminalOutput('Command killed', 'error');
}

function runQuickCommand(cmd) {
    document.getElementById('terminalInput').value = cmd;
    runCommand();
}

function runCommand() {
    const input = document.getElementById('terminalInput');
    const cmd = input.value.trim();
    if (!cmd) return;

    // Handle chat mode
    if (terminalChatMode) {
        // Prevent sending while already processing
        if (terminalRunning || pendingRequest) {
            return; // Silently ignore
        }
        
        // Throttle terminal chat requests
        const now = Date.now();
        if (now - lastRequestTime < requestCooldown) {
            return;
        }
        lastRequestTime = now;
        
        if (cmd === 'exit' || cmd === 'quit') {
            addTerminalOutput(cmd, 'prompt');
            addTerminalOutput('Exiting chat mode...', 'success');
            terminalChatMode = false;
            terminalChatModel = '';
            terminalChatHistory = [];
            input.value = '';
            return;
        } else if (cmd === 'clear') {
            clearTerminal();
            addTerminalOutput(`Chat mode with ${terminalChatModel}`, 'success');
            addTerminalOutput('Type your message or "exit" to quit');
            addTerminalOutput('> ', 'success');
            input.value = '';
            return;
        }
        
        // Send message in chat mode - disable input while processing
        addTerminalOutput(cmd, 'prompt');
        input.value = '';
        input.disabled = true;
        terminalRunning = true;
        pendingRequest = true;
        sendTerminalChat(cmd);
        return;
    }

    if (terminalRunning) {
        addTerminalOutput('Command already running. Use Kill button to stop it.', 'error');
        return;
    }
    
    // Add to history
    if (commandHistory[0] !== cmd) {
        commandHistory.unshift(cmd);
        if (commandHistory.length > 50) commandHistory.pop();
    }
    historyIndex = -1;

    addTerminalOutput(cmd, 'prompt');
    input.value = '';

    const parts = cmd.split(' ').filter(p => p);
    const command = parts[0];
    const args = parts.slice(1);

    // Set running state
    terminalRunning = true;
    const inputLine = document.getElementById('terminalInputLine');
    inputLine.classList.add('terminal-running');
    input.disabled = true;
    document.getElementById('killBtn').classList.remove('hidden');

    switch (command) {
        case 'offgrid':
            handleOffgridCommand(args);
            break;
        case 'clear':
            clearTerminal();
            resetTerminalState();
            break;
        case 'help':
            showHelp();
            resetTerminalState();
            break;
        case 'history':
            showHistory();
            resetTerminalState();
            break;
        default:
            addTerminalOutput(`Unknown command: ${command}. Type 'help' for available commands`, 'error');
            resetTerminalState();
    }
}

function resetTerminalState() {
    terminalRunning = false;
    const inputLine = document.getElementById('terminalInputLine');
    inputLine.classList.remove('terminal-running');
    document.getElementById('terminalInput').disabled = false;
    document.getElementById('killBtn').classList.add('hidden');
}

function showHelp() {
    const help = [
        'Available Commands:',
        '',
        'Terminal Commands:',
        '  clear                         Clear terminal screen',
        '  history                       Show command history',
        '  help                          Show this help',
        '',
        'OffGrid Commands (runs real offgrid binary):',
        '  offgrid list                  List installed models',
        '  offgrid search <query>        Search for models to download',
        '  offgrid download <model>      Download a model',
        '  offgrid remove <model>        Remove an installed model',
        '  offgrid run <model>           Load model and switch to Chat tab',
        '  offgrid info                  Show system information',
        '  offgrid doctor                Run diagnostics',
        '  offgrid --help                Show all offgrid commands',
        '  offgrid --version             Show version',
        '',
        'Tips:',
        '  - Use arrow keys (↑/↓) to navigate history',
        '  - Use Tab for command autocomplete',
        '  - Press Ctrl+C or click Kill to stop command',
        '  - For interactive chat, use "offgrid run <model>" or Chat tab'
    ];
    help.forEach(line => addTerminalOutput(line));
}

function showHistory() {
    if (commandHistory.length === 0) {
        addTerminalOutput('No command history');
        return;
    }
    addTerminalOutput('Command History:');
    commandHistory.slice(0, 10).forEach((cmd, i) => {
        addTerminalOutput(`  ${i + 1}. ${cmd}`);
    });
}

async function handleOffgridCommand(args) {
    if (args.length === 0) {
        addTerminalOutput('Usage: offgrid <command>', 'error');
        addTerminalOutput('Try: offgrid help');
        resetTerminalState();
        return;
    }

    // Create abort controller for this command
    currentTerminalAbort = new AbortController();
    
    try {
        // Execute real offgrid command via streaming endpoint for real-time output
        const resp = await fetch('/v1/terminal/exec/stream', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                command: 'offgrid',
                args: args
            }),
            signal: currentTerminalAbort?.signal
        });

        if (!resp.ok) {
            addTerminalOutput(`Command failed: HTTP ${resp.status}`, 'error');
            resetTerminalState();
            return;
        }

        // Read streaming response
        const reader = resp.body.getReader();
        const decoder = new TextDecoder();
        let buffer = '';
        let exitCode = 0;

        while (true) {
            const {done, value} = await reader.read();
            if (done) break;

            buffer += decoder.decode(value, {stream: true});
            
            // Process complete SSE events
            const lines = buffer.split('\n\n');
            buffer = lines.pop(); // Keep incomplete event in buffer

            for (const event of lines) {
                if (!event.trim()) continue;
                
                // Parse SSE format: "event: type\ndata: line1\ndata: line2\n..."
                const eventLines = event.split('\n');
                let eventType = '';
                const dataLines = [];
                
                for (const eLine of eventLines) {
                    if (eLine.startsWith('event:')) {
                        eventType = eLine.substring(6).trim();
                    } else if (eLine.startsWith('data:')) {
                        dataLines.push(eLine.substring(5).trim());
                    }
                }
                
                if (!eventType) continue;
                
                const data = dataLines.join('\n');
                
                if (eventType === 'output') {
                    // Display output in real-time
                    if (data) addTerminalRawOutput(data);
                } else if (eventType === 'exit') {
                    exitCode = parseInt(data);
                } else if (eventType === 'error') {
                    addTerminalOutput(`Error: ${data}`, 'error');
                }
            }
        }
        
        // Special handling for 'run' command - enable terminal chat mode
        const subcmd = args[0];
        if (subcmd === 'run' && args.length > 1 && exitCode === 0) {
            // Extract just the model name (first argument after run), ignoring flags
            // We assume the syntax is: offgrid run <model_id> [flags]

            const modelName = args[1];
            
            // Enable chat mode
            terminalChatMode = true;
            terminalChatModel = modelName;
            terminalChatHistory = [];
            addTerminalOutput('', 'normal');
            addTerminalOutput(`Chat ready (${modelName})`, 'success');
            addTerminalOutput('Type your message, or "exit" to quit', 'normal');
        }

        if (exitCode !== 0) {
            addTerminalOutput(`Command exited with code ${exitCode}`, 'error');
        }

        // Refresh UI after certain commands (only on success)
        if (['download', 'remove', 'delete'].includes(subcmd) && exitCode === 0) {
            // Wait longer to ensure backend has updated
            setTimeout(() => {
                loadInstalledModels();
                loadChatModels();
            }, 2500);
        }

    } catch (error) {
        if (error.name === 'AbortError') {
            addTerminalOutput('Command aborted', 'error');
        } else {
            addTerminalOutput(`Error: ${error.message}`, 'error');
        }
    } finally {
        flushTerminalBuffer(); // Flush any remaining buffered output
        resetTerminalState();
        currentTerminalAbort = null;
    }
}



// Send chat message in terminal mode
async function sendTerminalChat(message) {
    try {
        terminalChatHistory.push({ role: 'user', content: message });
        
        const resp = await fetch('/v1/chat/completions', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                model: terminalChatModel,
                messages: terminalChatHistory,
                stream: false,
                temperature: 0.7,
                max_tokens: 2000
            })
        });

        if (!resp.ok) {
            let errorMsg = `HTTP ${resp.status}`;
            try {
                const errorData = await resp.json();
                errorMsg = errorData.error?.message || errorMsg;
            } catch (e) {
                // Ignore JSON parse errors
            }
            addTerminalOutput(`Error: ${errorMsg}`, 'error');
            terminalChatHistory.pop(); // Remove failed message
            return;
        }

        const data = await resp.json();
        const assistantMsg = data.choices?.[0]?.message?.content || 'No response';
        const actualModel = data.model || terminalChatModel;
        
        terminalChatHistory.push({ role: 'assistant', content: assistantMsg });
        
        addTerminalOutput('', 'normal');
        addTerminalOutput(assistantMsg, 'success');
        
    } catch (error) {
        addTerminalOutput(`Error: ${error.message}`, 'error');
        if (terminalChatHistory.length > 0 && terminalChatHistory[terminalChatHistory.length - 1].role === 'user') {
            terminalChatHistory.pop(); // Remove failed message
        }
    } finally {
        // Re-enable input after processing
        terminalRunning = false;
        pendingRequest = false;
        document.getElementById('terminalInput').disabled = false;
        document.getElementById('terminalInput').focus();
    }
}

// Format progress line with visual progress bar
function formatProgressLine(text) {
    const cleanText = stripAnsi(text);
    
    // Extract percentage if present
    const percentMatch = cleanText.match(/(\d+(?:\.\d+)?)\s*%/);
    const percent = percentMatch ? parseFloat(percentMatch[1]) : null;
    
    // Clean up the text and format nicely
    let formattedText = parseAnsiColors(text);
    
    if (percent !== null) {
        // Create visual progress bar
        const progressBar = `
            <div style="display: flex; align-items: center; gap: 12px;">
                <span style="min-width: 50px; text-align: right; color: #7ee787;">${percent.toFixed(1)}%</span>
                <div style="flex: 1; max-width: 200px; height: 6px; background: #21262d; border-radius: 3px; overflow: hidden;">
                    <div style="width: ${Math.min(percent, 100)}%; height: 100%; background: linear-gradient(90deg, #238636, #2ea043); transition: width 0.15s ease;"></div>
                </div>
                <span style="color: #8b949e; font-size: 12px;">${cleanText.replace(/\d+(?:\.\d+)?\s*%/, '').trim()}</span>
            </div>
        `;
        return progressBar;
    }
    
    return formattedText;
}

// Strip ANSI escape codes
function stripAnsi(text) {
    // Strip all ANSI escape codes (colors, cursor movement, etc.)
    return text.replace(/\x1b\[[0-9;]*[a-zA-Z]/g, '');
}

// Parse ANSI color codes and convert to HTML with CSS classes
function parseAnsiColors(text) {
    // Simple escape - just escape HTML chars, preserve spaces
    function escapeHtml(str) {
        return str
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;');
    }
    
    // Strip ANSI codes completely for now - they cause formatting issues
    // Just return clean text
    const stripped = text.replace(/\\x1b\\[[0-9;]*m/g, '');
    return escapeHtml(stripped);
}

function addTerminalOutput(text, type = 'normal') {
    const pre = document.getElementById('terminalPre');
    
    // Simple escape
    function esc(str) {
        return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
    }
    
    // Strip literal \n from text
    const cleanText = text.replace(/\\n/g, '').replace(/\\r/g, '');
    
    let output = '';
    if (type === 'prompt') {
        output = '\n\n<span style="color: #06b6d4;">$ ' + esc(cleanText) + '</span>\n';
    } else if (type === 'error') {
        output = '<span style="color: #f7768e;">' + esc(cleanText) + '</span>\n';
    } else if (type === 'success') {
        output = '<span style="color: #06b6d4;">' + esc(cleanText) + '</span>\n';
    } else {
        output = esc(cleanText) + '\n';
    }
    
    pre.innerHTML += output;
    scrollTerminalToBottom();
}

// Buffer for incomplete lines across chunks
let terminalOutputBuffer = '';

// Strip ANSI escape codes from text
function stripAnsiCodes(str) {
    // Match ESC[ followed by any number of digits/semicolons, ending with a letter
    // This handles \x1b[, \033[, and actual escape character
    return str
        .replace(/\x1b\[[0-9;]*[a-zA-Z]/g, '')  // Actual escape char
        .replace(/\033\[[0-9;]*[a-zA-Z]/g, '')  // Octal escape
        .replace(/\[\d+m/g, '')                  // Bare codes like [36m
        .replace(/\[\d+;\d+m/g, '')              // Codes like [1;36m
        .replace(/\[0m/g, '')                    // Reset code
        .replace(/\[m/g, '');                    // Short reset
}

// Track progress line element for in-place updates
let terminalProgressLine = null;

function addTerminalRawOutput(text) {
    const pre = document.getElementById('terminalPre');
    
    // Simple escape function
    function esc(str) {
        return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
    }
    
    // Convert literal \n to actual newlines first
    let processedText = text.replace(/\\n/g, '\n').replace(/\\r/g, '\r');
    
    // Combine with any buffered text
    let currentText = terminalOutputBuffer + processedText;
    
    // Check for carriage return at the start (progress update pattern)
    // This handles "\r  Progress: XX%" style updates
    if (currentText.includes('\r') && !currentText.includes('\n')) {
        // This is a progress update - show in real-time
        const parts = currentText.split('\r');
        const lastPart = parts[parts.length - 1];
        const clean = stripAnsiCodes(lastPart);
        
        if (clean.trim()) {
            // Create or update progress line element
            if (!terminalProgressLine) {
                terminalProgressLine = document.createElement('span');
                terminalProgressLine.className = 'terminal-progress';
                pre.appendChild(terminalProgressLine);
            }
            terminalProgressLine.textContent = esc(clean);
            scrollTerminalToBottom();
        }
        
        // Keep the last part in buffer (will be replaced by next \r)
        terminalOutputBuffer = currentText;
        return;
    }
    
    // If we have a progress line and now getting a newline, finalize it
    if (terminalProgressLine && currentText.includes('\n')) {
        // Clear the progress line since we're moving to next line
        terminalProgressLine.remove();
        terminalProgressLine = null;
    }
    
    // Split by newlines
    const lines = currentText.split('\n');
    
    // Keep the last incomplete line in buffer
    terminalOutputBuffer = lines.pop() || '';
    
    // Append complete lines - strip ANSI codes but preserve text
    if (lines.length > 0) {
        let output = '';
        for (let line of lines) {
            // Handle carriage return (take last part)
            if (line.includes('\r')) {
                const parts = line.split('\r');
                line = parts[parts.length - 1];
            }
            // Strip ANSI codes and escape HTML
            const clean = stripAnsiCodes(line);
            // Skip empty prompt lines (just > or whitespace)
            if (clean.trim() === '>' || clean.trim() === '') continue;
            output += esc(clean) + '\n';
        }
        if (output) {
            pre.innerHTML += output;
            scrollTerminalToBottom();
        }
    }
}

// Flush any remaining buffer content (call this when command completes)
function flushTerminalBuffer() {
    // Clear any progress line element
    if (terminalProgressLine) {
        terminalProgressLine.remove();
        terminalProgressLine = null;
    }
    
    if (terminalOutputBuffer) {
        const pre = document.getElementById('terminalPre');
        // Handle carriage return - take last part
        let content = terminalOutputBuffer;
        if (content.includes('\r')) {
            const parts = content.split('\r');
            content = parts[parts.length - 1];
        }
        // Strip ANSI and escape
        const clean = stripAnsiCodes(content);
        if (clean.trim() && clean.trim() !== '>') {
            const escaped = clean.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
            pre.innerHTML += escaped + '\n';
        }
        terminalOutputBuffer = '';
        scrollTerminalToBottom();
    }
}

