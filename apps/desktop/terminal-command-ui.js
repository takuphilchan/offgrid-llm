'use strict';
function mountTerminalCommands({current,request,onOpen}) {
  const button=document.createElement('button');button.id='commands';button.title='Inspect shell-reported commands or selected output';button.setAttribute('aria-label','Commands');
  button.innerHTML='<svg class="terminal-icon" aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="m4 5 4 4-4 4m8-8h8m-8 4h8M4 19h16"/></svg><span>Commands</span>';
  button.hidden=true;
  document.querySelector('.tools').prepend(button);
  const dialog=document.createElement('dialog');dialog.id='terminal-commands';dialog.setAttribute('aria-label','Command context');
  dialog.innerHTML=`<div class="display-heading"><h2>Command context</h2><button type="button" aria-label="Close command context">×</button></div>
    <p class="display-note">Review terminal output before sharing it with Chat. Shell reports are not verified agent actions.</p>
    <section id="context-empty" hidden><h3>No output selected</h3><p>Run a command, or select text in the terminal, then open Commands to review and share it.</p><button id="context-return">Return to terminal</button></section><div id="context-content"><label id="context-command-label">Recent command<select id="context-command"></select></label><p id="context-state" role="status"></p>
    <div class="context-navigation"><button id="context-jump">Go to output</button><button id="context-selection">Use selected text</button></div>
    <details id="context-metadata"><summary>Command and folder</summary><label>Command<input id="context-title" maxlength="4096"></label><label>Folder<input id="context-folder" maxlength="2048"></label></details>
    <label>Output<textarea id="context-output" rows="5" maxlength="32768" spellcheck="false"></textarea></label>
    <p id="context-limits" class="display-note"></p>
    <label>Your question<textarea id="context-question" rows="2" maxlength="2000" placeholder="What would you like help with?"></textarea></label>
    <p id="context-error" role="alert"></p><div class="display-actions"><button id="context-copy">Copy context</button><button id="context-share">Continue in Chat</button></div></div>`;
  document.body.append(dialog);
  const field=id=>dialog.querySelector('#context-'+id);
  let tab=null,selected=null,truncated=false,edited=false,pending=false,generation=0;
  function actions(){field('copy').disabled=pending||!field('output').value.trim();field('share').disabled=pending||!field('output').value.trim()||!field('question').value.trim();field('command').disabled=pending;field('selection').disabled=pending||!tab?.term.hasSelection();field('share').textContent=pending?'Opening Chat…':'Continue in Chat';}
  function content(){field('empty').hidden=true;field('content').hidden=false;actions();}
  function state(message){field('state').textContent=message;}
  function choose(refresh=false) {
    if(!refresh){selected=tab?.commands.commands.find(item=>String(item.id)===field('command').value);edited=false;field('error').textContent='';}
    if(!selected)return;
    const result=tab.commands.output(selected);truncated=result.truncated;
    if(!refresh){field('title').value=selected.command;field('folder').value=selected.cwd;}
    if(!edited)field('output').value=result.text;
    state(!selected.finished?'Command still running':selected.exit===null?(selected.failed?'Shell reported failure; exact exit code unavailable':'Outcome unknown'):`Shell reported exit ${selected.exit} · ${(selected.duration/1000).toFixed(1)}s`);
    field('limits').textContent=!selected.finished?'Output will appear here when the command finishes.':!result.available?'Output unavailable or removed from scrollback. Select visible output instead.':!result.text.trim()?'This command produced no text output.':truncated?'Output or command shortened to the preview limit. Review before sharing.':'Only this command’s visible output is included. Edit to remove private content.';
    field('jump').disabled=!selected.start||selected.start.isDisposed;content();
  }
  function selection() {
    const text=tab?.term.getSelection()||'';
    if(!text){state('Select terminal text first, then reopen command context.');return;}
    selected=null;edited=false;truncated=text.length>32768;
    field('title').value='';field('folder').value=tab.commands.cwd||tab.cwd;
    field('output').value=text.slice(0,32768).replace(/[\uD800-\uDBFF]$/,'');field('jump').disabled=true;
    field('command').selectedIndex=-1;content();
    state('Manually selected output · command and outcome not verified');field('limits').textContent=truncated?'Selection shortened to 32,768 characters.':'Review the selected text before sharing.';
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
  field('copy').onclick=()=>void request('copy',{text:JSON.stringify(payload(),null,2)}).catch(error=>field('error').textContent=String(error.message));
  field('share').onclick=async()=>{
    if(pending)return;
    const context=payload();if(!context.output.trim()||!context.question){field('error').textContent='Include selected output and a question before continuing.';return;}
    const attempt=generation;pending=true;field('error').textContent='';actions();
    try{await request('share',{id:tab.id,context});if(attempt===generation)dialog.close();}catch(error){if(attempt===generation)field('error').textContent=String(error.message);}finally{if(attempt===generation){pending=false;actions();}}
  };
  function refresh(changed){
    const active=current();
    button.hidden=!active||(!active.commands?.commands.length&&!active.term.hasSelection());
    button.querySelector('span').textContent=active?.term.hasSelection()?'Ask about selection':'Commands';
    button.setAttribute('aria-label',active?.term.hasSelection()?'Ask about selection':'Commands');
    if(dialog.open&&changed===tab&&selected&&!pending)choose(true);
  }
  return {isOpen:()=>dialog.open,refresh};
}
