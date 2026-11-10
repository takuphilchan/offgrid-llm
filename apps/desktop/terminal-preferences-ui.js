'use strict';
// Bundled presentation controls only. No command or terminal-output content is
// interpolated into the dialog, and changes never dispatch PTY input.
function mountTerminalPreferences({get,set,onOpen}) {
 const button=document.createElement('button');
 button.id='display-settings';button.title='Terminal display';button.setAttribute('aria-label','Terminal display');
 button.textContent='Aa';
 document.querySelector('.tools').prepend(button);
 const dialog=document.createElement('dialog');dialog.id='terminal-display';
 dialog.setAttribute('aria-labelledby','terminal-display-title');
 dialog.innerHTML=`<form method="dialog"><div class="display-heading"><h2 id="terminal-display-title">Terminal display</h2><button value="close" aria-label="Close terminal display">×</button></div>
 <label>Text size <select name="fontSize" aria-label="Text size">${[11,12,13,14,15,16,18,20,22,24].map(n=>`<option value="${n}">${n} px</option>`).join('')}</select></label>
 <label>Line spacing <select name="lineHeight" aria-label="Line spacing">${[1,1.15,1.25,1.4,1.6,1.8].map(n=>`<option value="${n}">${n}</option>`).join('')}</select></label>
 <label>Cursor <select name="cursorStyle" aria-label="Cursor"><option value="block">Block</option><option value="bar">Line</option><option value="underline">Underline</option></select></label>
 <label>Blink cursor <input name="cursorBlink" type="checkbox"></label>
 <p class="display-note">Reduced motion always disables blinking.</p>
 <label>Scrollback <select name="scrollback" aria-label="Scrollback">${[1000,5000,10000,20000].map(n=>`<option value="${n}">${n.toLocaleString('en-US')} lines</option>`).join('')}</select></label>
 <p class="display-note">Lowering this limit removes older displayed output. Saved agent conversations are unchanged.</p>
 <p id="display-save-status" role="status"></p><div class="display-actions"><button id="display-reset" type="button">Reset defaults</button><button value="close">Done</button></div></form>`;
 document.body.append(dialog);
 const form=dialog.querySelector('form');
 const fill=()=>{for(const [key,value] of Object.entries(get())){const field=form.elements.namedItem(key);if(field.type==='checkbox')field.checked=value;else {if(![...field.options].some(o=>Number(o.value)===value||o.value===value))field.add(new Option(String(value),String(value)));field.value=String(value);}}};
 button.onclick=()=>{fill();dialog.showModal();onOpen(true);};
 dialog.addEventListener('close',()=>{onOpen(false);button.focus();});
 form.addEventListener('change',()=>{const value={};for(const key of Object.keys(get())){const field=form.elements.namedItem(key);value[key]=field.type==='checkbox'?field.checked:key==='cursorStyle'?field.value:Number(field.value);}set(value);});
 dialog.querySelector('#display-reset').onclick=()=>{set(terminalPreferences.defaults);fill();};
 return {isOpen:()=>dialog.open,saved:ok=>{dialog.querySelector('#display-save-status').textContent=ok?'':'Applied for this window; preferences could not be saved.';}};
}
