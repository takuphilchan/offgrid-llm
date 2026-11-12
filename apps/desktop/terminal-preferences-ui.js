'use strict';
// Bundled presentation controls only. No command or terminal-output content is
// interpolated into the dialog, and changes never dispatch PTY input.
function mountTerminalPreferences({get,set,onOpen}) {
 const button=document.createElement('button');
 button.id='display-settings';button.title=terminalText("Terminal display");button.setAttribute('aria-label',terminalText("Terminal display"));
 button.textContent='Aa';
 button.dataset.terminalLabel='Terminal display';button.dataset.terminalTitle='Terminal display';
 document.querySelector('.tools').prepend(button);
 const dialog=document.createElement('dialog');dialog.id='terminal-display';
 dialog.setAttribute('aria-labelledby','terminal-display-title');
 dialog.innerHTML=`<form method="dialog"><div class="display-heading"><h2 id="terminal-display-title"><span data-terminal-text="Terminal display">Terminal display</span></h2><button value="close" aria-label="Close terminal display" data-terminal-label="Close terminal display">×</button></div>
 <label><span data-terminal-text="Text size">Text size </span><select name="fontSize" aria-label="Text size" data-terminal-label="Text size">${[11,12,13,14,15,16,18,20,22,24].map(n=>`<option value="${n}">${n} px</option>`).join('')}</select></label>
 <label><span data-terminal-text="Line spacing">Line spacing </span><select name="lineHeight" aria-label="Line spacing" data-terminal-label="Line spacing">${[1,1.15,1.25,1.4,1.6,1.8].map(n=>`<option value="${n}">${n}</option>`).join('')}</select></label>
 <label><span data-terminal-text="Cursor">Cursor </span><select name="cursorStyle" aria-label="Cursor" data-terminal-label="Cursor"><option value="block" data-terminal-text="Block">Block</option><option value="bar" data-terminal-text="Line">Line</option><option value="underline" data-terminal-text="Underline">Underline</option></select></label>
 <label><span data-terminal-text="Blink cursor">Blink cursor </span><input name="cursorBlink" type="checkbox"></label>
 <p class="display-note"><span data-terminal-text="Reduced motion always disables blinking.">Reduced motion always disables blinking.</span></p>
 <label><span data-terminal-text="Scrollback">Scrollback </span><select name="scrollback" aria-label="Scrollback" data-terminal-label="Scrollback">${[1000,5000,10000,20000].map(n=>`<option value="${n}">${n.toLocaleString('en-US')} lines</option>`).join('')}</select></label>
 <p class="display-note"><span data-terminal-text="Lowering this limit removes older displayed output. Saved agent conversations are unchanged.">Lowering this limit removes older displayed output. Saved agent conversations are unchanged.</span></p>
 <p id="display-save-status" role="status"></p><div class="display-actions"><button id="display-reset" type="button"><span data-terminal-text="Reset defaults">Reset defaults</span></button><button value="close"><span data-terminal-text="Done">Done</span></button></div></form>`;
 document.body.append(dialog);
 const form=dialog.querySelector('form');
 const fill=()=>{for(const [key,value] of Object.entries(get())){const field=form.elements.namedItem(key);if(field.type==='checkbox')field.checked=value;else {if(![...field.options].some(o=>Number(o.value)===value||o.value===value))field.add(new Option(String(value),String(value)));field.value=String(value);}}};
 button.onclick=()=>{fill();dialog.showModal();onOpen(true);};
 dialog.addEventListener('close',()=>{onOpen(false);button.focus();});
 form.addEventListener('change',()=>{const value={};for(const key of Object.keys(get())){const field=form.elements.namedItem(key);value[key]=field.type==='checkbox'?field.checked:key==='cursorStyle'?field.value:Number(field.value);}set(value);});
 dialog.querySelector('#display-reset').onclick=()=>{set(terminalPreferences.defaults);fill();};
 let saveFailed=false;
 const localize=()=>{
  for(const name of ['scrollback','lineHeight','fontSize'])for(const option of form.elements.namedItem(name).options){
   const count=Number(option.value).toLocaleString(document.documentElement.lang);
   option.textContent=name==='scrollback'?terminalText('{count} lines',{count}):name==='fontSize'?`${count} px`:count;
  }
  dialog.querySelector('#display-save-status').textContent=saveFailed?terminalText('Applied for this window; preferences could not be saved.'):'';
 };
 return {isOpen:()=>dialog.open,localize,saved:ok=>{saveFailed=!ok;localize();}};
}
