'use strict';
const bridge=window.terminalDesktop, $=id=>document.getElementById(id), tabs=new Map();
let generation='',current='',creating=false,ready=false,visible=false,fitFrame=0,focusPending=false;
let noticeMessageKey=null;
const clearErrorDetails=()=>{$('terminal-error-details').hidden=true;$('terminal-error-details').open=false;$('terminal-error-text').textContent='';};
const notice=message=>{noticeMessageKey=null;$('notice').textContent=message||'';clearErrorDetails();};
const noticeKey=key=>{noticeMessageKey=key;$('notice').textContent=terminalText(key);clearErrorDetails();};
const request=(action,fields={})=>bridge.request({protocol:1,generation,action,...fields});
const showError=error=>{noticeKey('The terminal request could not be confirmed. Check its state before trying again.');$('terminal-error-details').hidden=false;$('terminal-error-text').textContent=String(error?.message||error).replace(/^Error invoking remote method '[^']+': Error: /,'');};
const displayStorage={getItem:key=>localStorage.getItem(key),setItem:(key,value)=>localStorage.setItem(key,value)};
let paletteTokens=null, userReducedMotion=false;
let terminalLocale='en';
function terminalText(key,values){return terminalTranslations.text(terminalLocale,key,values);}
function localizeTerminal(locale){
 terminalLocale=typeof locale==='string'?locale:'en';
 document.documentElement.lang=terminalLocale;
 document.documentElement.dir=['ar','fa','he','ur'].includes(terminalLocale)?'rtl':'ltr';
 terminalTranslations.render(terminalLocale,document);
 Terminal.strings.promptLabel=terminalText('Terminal input');
 Terminal.strings.tooMuchOutput=terminalText('Too much output to read automatically. Navigate the terminal rows to read it.');
 for(const input of document.querySelectorAll('.xterm-helper-textarea'))input.setAttribute('aria-label',Terminal.strings.promptLabel);
 document.title='OffGrid · '+terminalText('Terminals');
 displayControls.localize();commandControls.localize();searchControls.localize();
 for(const tab of tabs.values())localizeTab(tab);
 updateIdentity(tabs.get(current));
 if(noticeMessageKey)$('notice').textContent=terminalText(noticeMessageKey);
}
function localizeTab(tab){
 const title=tab.kind==='agent'||tab.kind==='offgrid'?terminalText('Agent'):tab.title;
 tab.select.textContent=`${title==='OffGrid CLI'?'CLI':title} · ${folderLabel(tab.cwd)}`;
 tab.select.setAttribute('aria-label',title);tab.pane.setAttribute('aria-label',title);
 tab.button.querySelector('.close').setAttribute('aria-label',terminalText('Close {title}',{title}));
}
let displayPreferences=terminalPreferences.load(displayStorage);
const reducedMotion=matchMedia('(prefers-reduced-motion: reduce)'),forcedColors=matchMedia('(forced-colors: active)');
const theme=()=>{const style=getComputedStyle(document.body);return terminalPreferences.theme(document.documentElement.dataset.theme==='dark',forcedColors.matches?{background:style.backgroundColor,foreground:style.color}:null,paletteTokens);};
function appearance(value){terminalTranslations.configure(value.locale,value.translations);userReducedMotion=value.reduceMotion===true;paletteTokens=value.palette||null;for(const [key,token] of Object.entries({bg:'surface-0',fg:'text',muted:'muted',line:'line',hover:'surface-2'})){if(paletteTokens)document.documentElement.style.setProperty('--'+key,paletteTokens[token]);else document.documentElement.style.removeProperty('--'+key);}document.documentElement.dataset.theme=value.theme==='dark'?'dark':'light';localizeTerminal(value.locale);for(const tab of tabs.values()){tab.term.options.theme=theme();tab.term.options.cursorBlink=displayPreferences.cursorBlink&&!reducedMotion.matches&&!userReducedMotion;}}
function applyDisplay(){for(const tab of tabs.values()){Object.assign(tab.term.options,terminalPreferences.options(displayPreferences,reducedMotion.matches||userReducedMotion));tab.term.options.theme=theme();}activate(false);}
const displayControls=mountTerminalPreferences({get:()=>displayPreferences,set:value=>{displayPreferences=terminalPreferences.normalize(value);displayControls.saved(terminalPreferences.save(displayStorage,displayPreferences));applyDisplay();},onOpen:open=>{for(const tab of tabs.values())tab.term.options.disableStdin=open||!visible||searchControls.isOpen()||tab.id!==current;}});
const searchControls=mountTerminalSearch({current:()=>tabs.get(current),tabs,theme,fit,changed:()=>{for(const tab of tabs.values())tab.term.options.disableStdin=!visible||searchControls.isOpen()||tab.id!==current;}});
const commandControls=mountTerminalCommands({current:()=>tabs.get(current),request,onOpen:open=>{for(const tab of tabs.values())tab.term.options.disableStdin=open||!visible||searchControls.isOpen()||tab.id!==current;}});
reducedMotion.addEventListener('change',applyDisplay);forcedColors.addEventListener('change',applyDisplay);
function fit(tab){
 if(!visible||!tab||tab.pane.hidden||!tab.pane.clientWidth||!tab.pane.clientHeight)return;
 try{tab.fit.fit();const cols=Math.max(2,Math.min(500,tab.term.cols)),rows=Math.max(1,Math.min(200,tab.term.rows));
  if(tab.state==='running'&&(tab.cols!==cols||tab.rows!==rows)){tab.cols=cols;tab.rows=rows;void request('resize',{id:tab.id,cols,rows}).catch(showError);}
 }catch(error){showError(error);}
}
function activate(focus=true){focusPending=focusPending||focus;cancelAnimationFrame(fitFrame);fitFrame=requestAnimationFrame(()=>{const focusNow=focusPending;focusPending=false;const tab=tabs.get(current);fit(tab);if(visible&&focusNow&&!(displayControls.isOpen()||commandControls.isOpen())){if(!$('find').hidden){$('query').focus();}else if(tab){tab.term.refresh(0,tab.term.rows-1);tab.term.focus();}else $('terminal-welcome-title').focus();}});}
function folderLabel(path){const parts=String(path||'').replace(/[\\/]+$/,'').split(/[\\/]/).filter(Boolean);return parts.at(-1)||path||terminalText("Local folder");}
function updateIdentity(tab){
 const identity=$('identity'),cwd=tab?.commands?.cwd||tab?.cwd;
 const state=tab?.state==='running'?(tab.commands?.active?terminalText("Command running (shell reported)"):terminalText("Session open")):terminalText(tab?.state??'');
 identity.textContent=tab?`${folderLabel(cwd)} · ${state}`:terminalText("Local terminal · not sandboxed");identity.title=cwd||'';
}
function select(id){current=id;for(const tab of tabs.values()){tab.term.blur();tab.term.options.disableStdin=!visible||searchControls.isOpen()||(displayControls.isOpen()||commandControls.isOpen())||tab.id!==id;tab.pane.hidden=tab.id!==id;tab.select.setAttribute('aria-selected',String(tab.id===id));tab.select.tabIndex=tab.id===id?0:-1;}const tab=tabs.get(id);$('empty').hidden=!!tab;updateIdentity(tab);searchControls.select();commandControls.refresh();if(tab)activate();}function setLaunchDisabled(disabled){for(const id of ['new','shell','cli','agent','start','start-agent'])$(id).disabled=disabled;}
async function close(id){const tab=tabs.get(id);if(!tab)return;try{await request('close',{id});tab.commands.dispose();tab.term.dispose();tab.pane.remove();tab.button.remove();tabs.delete(id);setLaunchDisabled(!ready||tabs.size>=4);select([...tabs.keys()].at(-1)||'');notice('');}catch(error){if(!String(error.message).includes('terminal_close_cancelled'))showError(error);}}
async function create(kind){
 if(creating||!ready)return;creating=true;$('launch-menu').hidePopover();setLaunchDisabled(true);noticeKey('Choose a local folder…');
 try{
  const session=await request('create',{kind,cols:90,rows:24});if(!session){notice('');return;}
  const pane=document.createElement('div');pane.className='pane';pane.id=`pane-${session.id}`;pane.setAttribute('role','tabpanel');pane.setAttribute('aria-label',session.title);$('panes').append(pane);
  const surface=document.createElement('div');surface.className='terminal-surface';pane.append(surface);
  const button=document.createElement('div');button.className='terminal-tab';
  const choose=document.createElement('button');choose.setAttribute('role','tab');choose.setAttribute('aria-label',session.title);choose.setAttribute('aria-controls',pane.id);choose.textContent=`${session.title==='OffGrid CLI'?'CLI':session.title} \u00b7 ${folderLabel(session.cwd)}`;choose.title=session.cwd;choose.onclick=()=>select(session.id);
  const remove=document.createElement('button');remove.className='close';remove.textContent='×';remove.setAttribute('aria-label',terminalText('Close {title}',{title:session.title}));remove.onclick=()=>void close(session.id);button.append(choose,remove);$('tabs').append(button);
  // The pinned search addon uses xterm's marker/decoration API for matches.
  // This enables renderer APIs only; the authenticated desktop bridge remains
  // the sole path to processes, files and clipboard access.
  const term=new Terminal({...terminalPreferences.options(displayPreferences,reducedMotion.matches||userReducedMotion),fontFamily:'Cascadia Code, Consolas, monospace',screenReaderMode:true,allowProposedApi:true,theme:theme()});
  const fitAddon=new FitAddon.FitAddon(),search=new SearchAddon.SearchAddon();term.loadAddon(fitAddon);term.loadAddon(search);term.open(surface);
  // Shell escape sequences never write the clipboard, navigate or open URLs.
  term.parser.registerOscHandler(52,()=>true);
  const tab={...session,kind:session.kind||kind,cols:90,rows:24,term,fit:fitAddon,search,pane,button,select:choose};tabs.set(tab.id,tab);localizeTab(tab);searchControls.attach(tab);
  tab.commands=terminalCommandContext.create(term,{changed:()=>{if(current===tab.id)updateIdentity(tab);commandControls.refresh(tab);}});
  term.onSelectionChange(()=>commandControls.refresh());
  term.onData(text=>{if(tab.state!=='running'||!visible||current!==tab.id||searchControls.isOpen()||displayControls.isOpen()||commandControls.isOpen())return;for(let i=0;i<text.length;){let end=Math.min(text.length,i+4096);if(end<text.length&&/[\uD800-\uDBFF]/.test(text[end-1]))end--;void request('input',{id:tab.id,text:text.slice(i,end)}).catch(showError);i=end;}});
  term.attachCustomKeyEventHandler(event=>{
   const selection=term.getSelection(),shortcut=terminalShortcut(event,selection);
   if(shortcut==='copy'){if(selection)void request('copy',{text:selection}).catch(showError);event.preventDefault();return false;}
   if(shortcut==='paste'){event.preventDefault();if(tab.state==='running'&&visible&&current===tab.id&&!searchControls.isOpen()&&!displayControls.isOpen()&&!commandControls.isOpen())void request('paste',{id:tab.id}).then(text=>{if(text&&tab.state==='running'&&visible&&current===tab.id)term.paste(text);}).catch(showError);return false;}
   if(shortcut==='search'){searchControls.open();event.preventDefault();return false;}
   return true;
  });
  select(tab.id);await request('attach',{id:tab.id});notice('');
 }catch(error){showError(error);}finally{creating=false;setLaunchDisabled(!ready||tabs.size>=4);}
}
$('shell').onclick=()=>void create('shell');$('start').onclick=$('cli').onclick=()=>void create('cli');$('start-agent').onclick=$('agent').onclick=()=>void create('agent');
$('launch-menu').addEventListener('toggle',event=>{if(event.newState==='open'){const box=$('new').getBoundingClientRect();$('launch-menu').style.left=`${Math.max(8,Math.min(box.left,innerWidth-210))}px`;$('cli').focus();}});
$('hide').onclick=$('chat').onclick=()=>void request('hide').catch(showError);
$('tabs').onkeydown=e=>{if(!['ArrowLeft','ArrowRight','Home','End'].includes(e.key))return;const ids=[...tabs.keys()],index=ids.indexOf(current);const next=e.key==='Home'?0:e.key==='End'?ids.length-1:(index+(e.key==='ArrowRight'?1:-1)+ids.length)%ids.length;if(ids[next]){e.preventDefault();select(ids[next]);tabs.get(ids[next]).select.focus();}};
new ResizeObserver(()=>activate(false)).observe($('panes'));
bridge.onEvent(event=>{
 const tab=tabs.get(event.id);
 if(event.type==='data'&&tab)tab.term.write(event.text,()=>void request('ack',{id:tab.id,sequence:event.sequence}).catch(showError));
 else if(event.type==='exit'&&tab){tab.commands.interrupt();tab.state='exited';tab.term.writeln('\r\n['+terminalText('Process exited: {code}',{code:event.exitCode})+']');if(current===tab.id)select(tab.id);}
 else if(event.type==='stopped'&&tab){tab.commands.interrupt();tab.state='stopped';if(current===tab.id)select(tab.id);}
 else if(event.type==='error'){if(tab)tab.state='uncertain';showError(event.message);}
 else if(event.type==='layout'){activate(false);}
 else if(event.type==='appearance')appearance(event);
 else if(event.type==='availability'){ready=event.available;setLaunchDisabled(!ready||tabs.size>=4);if(ready)notice('');}
 else if(event.type==='visibility'){visible=event.visible;for(const item of tabs.values()){item.term.options.disableStdin=!visible||searchControls.isOpen()||(displayControls.isOpen()||commandControls.isOpen())||item.id!==current;if(!visible)item.term.blur();}if(visible)activate(event.focus!==false);}
});
void bridge.request({protocol:1,action:'handshake'}).then(value=>{generation=value.generation;ready=value.available;visible=value.presentation?.visible===true;appearance(value.presentation||{});activate();setLaunchDisabled(!ready);if(!ready)noticeKey('The native terminal runtime is unavailable on this build. Chat and Knowledge are still available.');}).catch(showError);
