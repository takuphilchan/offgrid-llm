'use strict';
function mountTerminalCommands({current,request,onOpen}) {
  const button=document.createElement('button');button.id='commands';button.title=terminalText("Inspect shell-reported commands or selected output");button.setAttribute('aria-label',terminalText("Commands"));
  button.dataset.terminalTitle='Inspect shell-reported commands or selected output';
  button.innerHTML="<svg class=\"terminal-icon\" aria-hidden=\"true\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"1.8\" stroke-linecap=\"round\" stroke-linejoin=\"round\"><path d=\"m4 5 4 4-4 4m8-8h8m-8 4h8M4 19h16\"/></svg><span><span data-terminal-text=\"Commands\">Commands</span></span>";
  button.hidden=true;
  document.querySelector('.tools').prepend(button);
  const dialog=document.createElement('dialog');dialog.id='terminal-commands';dialog.setAttribute('aria-label',terminalText("Command context"));
  dialog.dataset.terminalLabel='Command context';
  dialog.innerHTML=`<div class="display-heading"><h2><span data-terminal-text="Command context">Command context</span></h2><button type="button" aria-label="Close command context" data-terminal-label="Close command context">×</button></div>
    <p class="display-note"><span data-terminal-text="Review terminal output before sharing it with Chat. Shell reports are not verified agent actions.">Review terminal output before sharing it with Chat. Shell reports are not verified agent actions.</span></p>
    <section id="context-empty" hidden><h3><span data-terminal-text="No output selected">No output selected</span></h3><p><span data-terminal-text="Run a command, or select text in the terminal, then open Commands to review and share it.">Run a command, or select text in the terminal, then open Commands to review and share it.</span></p><button id="context-return"><span data-terminal-text="Return to terminal">Return to terminal</span></button></section><div id="context-content"><label id="context-command-label"><span data-terminal-text="Recent command">Recent command</span><select id="context-command"></select></label><p id="context-state" role="status"></p>
    <div class="context-navigation"><button id="context-jump"><span data-terminal-text="Go to output">Go to output</span></button><button id="context-selection"><span data-terminal-text="Use selected text">Use selected text</span></button></div>
    <details id="context-metadata"><summary><span data-terminal-text="Command and folder">Command and folder</span></summary><label><span data-terminal-text="Command">Command</span><input id="context-title" maxlength="4096"></label><label><span data-terminal-text="Folder">Folder</span><input id="context-folder" maxlength="2048"></label></details>
    <label><span data-terminal-text="Output">Output</span><textarea id="context-output" rows="5" maxlength="32768" spellcheck="false"></textarea></label>
    <p id="context-limits" class="display-note"></p>
    <label><span data-terminal-text="Your question">Your question</span><textarea id="context-question" rows="2" maxlength="2000" placeholder="What would you like help with?" data-terminal-placeholder="What would you like help with?"></textarea></label>
    <div id="context-error" role="alert"></div><div class="display-actions"><button id="context-copy"><span data-terminal-text="Copy context">Copy context</span></button><button id="context-share"><span data-terminal-text="Continue in Chat">Continue in Chat</span></button></div></div>`;
  document.body.append(dialog);
  const field=id=>dialog.querySelector('#context-'+id);
  function showError(error){
    const message=document.createElement('p'),details=document.createElement('details'),summary=document.createElement('summary'),technical=document.createElement('pre');
    message.dataset.terminalText='The terminal request could not be confirmed. Check its state before trying again.';
    message.textContent=terminalText(message.dataset.terminalText);summary.dataset.terminalText='Technical details';summary.textContent=terminalText('Technical details');
    technical.textContent=String(error?.message||error);technical.dir='auto';details.append(summary,technical);field('error').replaceChildren(message,details);
  }
  let tab=null,selected=null,truncated=false,edited=false,pending=false,generation=0;
  function actions(){field('copy').disabled=pending||!field('output').value.trim();field('share').disabled=pending||!field('output').value.trim()||!field('question').value.trim();field('command').disabled=pending;field('selection').disabled=pending||!tab?.term.hasSelection();field('share').textContent=pending?terminalText("Opening Chat…"):terminalText("Continue in Chat");}
  function content(){field('empty').hidden=true;field('content').hidden=false;actions();}
  function state(message){field('state').textContent=message;}
  function choose(refresh=false) {
    if(!refresh){selected=tab?.commands.commands.find(item=>String(item.id)===field('command').value);edited=false;field('error').textContent='';}
    if(!selected)return;
    const result=tab.commands.output(selected);truncated=result.truncated;
    if(!refresh){field('title').value=selected.command;field('folder').value=selected.cwd;}
    if(!edited)field('output').value=result.text;
    state(!selected.finished?terminalText("Command still running"):selected.exit===null?(selected.failed?terminalText("Shell reported failure; exact exit code unavailable"):terminalText("Outcome unknown")):terminalText('Shell reported exit {code} · {seconds}s',{code:selected.exit,seconds:(selected.duration/1000).toLocaleString(document.documentElement.lang,{minimumFractionDigits:1,maximumFractionDigits:1})}));
    field('limits').textContent=!selected.finished?terminalText("Output will appear here when the command finishes."):!result.available?terminalText("Output unavailable or removed from scrollback. Select visible output instead."):!result.text.trim()?terminalText("This command produced no text output."):truncated?terminalText("Output or command shortened to the preview limit. Review before sharing."):terminalText("Only this command’s visible output is included. Edit to remove private content.");
    field('jump').disabled=!selected.start||selected.start.isDisposed;content();
  }
  function selection() {
    const text=tab?.term.getSelection()||'';
    if(!text){state(terminalText("Select terminal text first, then reopen command context."));return;}
    selected=null;edited=false;truncated=text.length>32768;
    field('title').value='';field('folder').value=tab.commands.cwd||tab.cwd;
    field('output').value=text.slice(0,32768).replace(/[\uD800-\uDBFF]$/,'');field('jump').disabled=true;
    field('command').selectedIndex=-1;content();
    state(terminalText("Manually selected output · command and outcome not verified"));field('limits').textContent=truncated?terminalText("Selection shortened to 32,768 characters."):terminalText("Review the selected text before sharing.");
  }
  function payload(){return {version:1,command:field('title').value,cwd:field('folder').value,output:field('output').value,question:field('question').value.trim(),truncated,source:'terminal_selection'};}
  function open() {
    if(dialog.open)return;tab=current();if(!tab)return;generation++;pending=false;edited=false;
    selected=null;truncated=false;field('limits').textContent='';
    field('error').textContent='';field('question').value='';field('command').replaceChildren();field('metadata').open=false;field('command-label').hidden=!tab.commands.commands.length;field('selection').disabled=!tab.term.hasSelection();
    for(const command of [...tab.commands.commands].reverse()) {const option=document.createElement('option');option.value=String(command.id);option.textContent=command.command.slice(0,100);field('command').append(option);}
    if(tab.term.hasSelection())selection();else if(tab.commands.commands.length)choose();else{field('title').value='';field('folder').value=tab.commands.cwd||tab.cwd;field('output').value='';field('jump').disabled=true;field('empty').hidden=false;field('content').hidden=true;}
    dialog.showModal();onOpen(true);(field('content').hidden?field('return'):field('question')).focus();
  }
  field('output').oninput=()=>{edited=true;actions();};field('question').oninput=actions;field('return').onclick=()=>dialog.close();
  button.onclick=open;field('command').onchange=()=>choose();field('selection').onclick=selection;
  field('jump').onclick=()=>{if(selected?.start&&!selected.start.isDisposed){dialog.close();tab.term.scrollToLine(selected.start.line);}};
  dialog.querySelector('[aria-label]').onclick=()=>dialog.close();
  dialog.onclose=()=>{generation++;pending=false;onOpen(false);tab?.term.focus();};
  field('copy').onclick=()=>void request('copy',{text:JSON.stringify(payload(),null,2)}).catch(showError);
  field('share').onclick=async()=>{
    if(pending)return;
    const context=payload();if(!context.output.trim()||!context.question){field('error').textContent=terminalText("Include selected output and a question before continuing.");return;}
    const attempt=generation;pending=true;field('error').textContent='';actions();
    try{await request('share',{id:tab.id,context});if(attempt===generation)dialog.close();}catch(error){if(attempt===generation)showError(error);}finally{if(attempt===generation){pending=false;actions();}}
  };
  function refresh(changed){
    const active=current();
    button.hidden=!active||(!active.commands?.commands.length&&!active.term.hasSelection());
    button.querySelector('span').textContent=active?.term.hasSelection()?terminalText("Ask about selection"):terminalText("Commands");
    button.setAttribute('aria-label',active?.term.hasSelection()?terminalText("Ask about selection"):terminalText("Commands"));
    if(dialog.open&&changed===tab&&selected&&!pending)choose(true);
  }
  function localize(){
    refresh();if(!dialog.open)return;
    if(selected&&!pending)choose(true);
    else if(!selected&&!field('content').hidden){
      state(terminalText('Manually selected output · command and outcome not verified'));
      field('limits').textContent=terminalText(truncated?'Selection shortened to 32,768 characters.':'Review the selected text before sharing.');
    }
    actions();
  }
  return {isOpen:()=>dialog.open,refresh,localize};
}
