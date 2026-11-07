'use strict';
const bridge=window.terminalDesktop, $=id=>document.getElementById(id), tabs=new Map();
let generation='',current='',creating=false,ready=false,visible=false,fitFrame=0,focusPending=false;
const notice=message=>{$('notice').textContent=message||'';};
const request=(action,fields={})=>bridge.request({protocol:1,generation,action,...fields});
const showError=error=>notice(String(error.message||error).replace(/^Error invoking remote method '[^']+': Error: /,''));
const theme=()=>document.documentElement.dataset.theme==='dark'?{background:'#191919',foreground:'#e6e6e6',cursor:'#e6e6e6',selectionBackground:'#555555',brightBlack:'#999999',blue:'#82aaff',green:'#a3cc88',cyan:'#7fd7d2'}:{background:'#ffffff',foreground:'#242424',cursor:'#242424',selectionBackground:'#cbd7ef',brightBlack:'#666666',blue:'#2858a8',green:'#286639',cyan:'#006870',yellow:'#826000',white:'#555555',brightWhite:'#242424',brightGreen:'#276039',brightYellow:'#735c00',brightBlue:'#2858a8',brightCyan:'#006870',brightMagenta:'#813980'};
function appearance(value){document.documentElement.dataset.theme=value.theme==='dark'?'dark':'light';for(const tab of tabs.values())tab.term.options.theme=theme();}
function fit(tab){
 if(!visible||!tab||tab.pane.hidden||!tab.pane.clientWidth||!tab.pane.clientHeight)return;
 try{tab.fit.fit();const cols=Math.max(2,Math.min(500,tab.term.cols)),rows=Math.max(1,Math.min(200,tab.term.rows));
  if(tab.state==='running'&&(tab.cols!==cols||tab.rows!==rows)){tab.cols=cols;tab.rows=rows;void request('resize',{id:tab.id,cols,rows}).catch(showError);}
 }catch(error){showError(error);}
}
function activate(focus=true){focusPending=focusPending||focus;cancelAnimationFrame(fitFrame);fitFrame=requestAnimationFrame(()=>{const focusNow=focusPending;focusPending=false;const tab=tabs.get(current);fit(tab);if(visible&&focusNow){if(tab){tab.term.refresh(0,tab.term.rows-1);tab.term.focus();}else $('start').focus();}});}
function folderLabel(path){const parts=String(path||'').replace(/[\\/]+$/,'').split(/[\\/]/).filter(Boolean);return parts.at(-1)||path||'Local folder';}
function select(id){current=id;for(const tab of tabs.values()){tab.term.blur();tab.term.options.disableStdin=tab.id!==id;tab.pane.hidden=tab.id!==id;tab.select.setAttribute('aria-selected',String(tab.id===id));tab.select.tabIndex=tab.id===id?0:-1;}const tab=tabs.get(id);$('empty').hidden=!!tab;const identity=$('identity');identity.textContent=tab?`${folderLabel(tab.cwd)} \u00b7 ${tab.state==='running'?'Session open':tab.state}`:'Local terminal \u00b7 not sandboxed';identity.title=tab?.cwd||'';if(tab){if(visible)tab.term.focus();activate();}}
function setLaunchDisabled(disabled){for(const id of ['new','shell','cli','agent','start','start-agent'])$(id).disabled=disabled;}
async function close(id){const tab=tabs.get(id);if(!tab)return;try{await request('close',{id});tab.term.dispose();tab.pane.remove();tab.button.remove();tabs.delete(id);setLaunchDisabled(!ready||tabs.size>=4);select([...tabs.keys()].at(-1)||'');notice('');}catch(error){if(!String(error.message).includes('terminal_close_cancelled'))showError(error);}}
async function create(kind){
 if(creating||!ready)return;creating=true;$('launch-menu').hidePopover();setLaunchDisabled(true);notice('Choose a local folder…');
 try{
  const session=await request('create',{kind,cols:90,rows:24});if(!session){notice('');return;}
  const pane=document.createElement('div');pane.className='pane';pane.id=`pane-${session.id}`;pane.setAttribute('role','tabpanel');pane.setAttribute('aria-label',session.title);$('panes').append(pane);
  const surface=document.createElement('div');surface.className='terminal-surface';pane.append(surface);
  const button=document.createElement('div');button.className='terminal-tab';
  const choose=document.createElement('button');choose.setAttribute('role','tab');choose.setAttribute('aria-label',session.title);choose.setAttribute('aria-controls',pane.id);choose.textContent=`${session.title==='OffGrid CLI'?'CLI':session.title} \u00b7 ${folderLabel(session.cwd)}`;choose.title=session.cwd;choose.onclick=()=>select(session.id);
  const remove=document.createElement('button');remove.className='close';remove.textContent='×';remove.setAttribute('aria-label',`Close ${session.title}`);remove.onclick=()=>void close(session.id);button.append(choose,remove);$('tabs').append(button);
  const term=new Terminal({cursorBlink:!matchMedia('(prefers-reduced-motion: reduce)').matches,scrollback:5000,fontFamily:'Cascadia Code, Consolas, monospace',fontSize:14,lineHeight:1.25,screenReaderMode:true,allowProposedApi:false,theme:theme()});
  const fitAddon=new FitAddon.FitAddon(),search=new SearchAddon.SearchAddon();term.loadAddon(fitAddon);term.loadAddon(search);term.open(surface);
  // Shell escape sequences never write the clipboard, navigate or open URLs.
  term.parser.registerOscHandler(52,()=>true);
  const tab={...session,cols:90,rows:24,term,fit:fitAddon,search,pane,button,select:choose};tabs.set(tab.id,tab);
  term.onData(text=>{if(tab.state!=='running')return;for(let i=0;i<text.length;){let end=Math.min(text.length,i+4096);if(end<text.length&&/[\uD800-\uDBFF]/.test(text[end-1]))end--;void request('input',{id:tab.id,text:text.slice(i,end)}).catch(showError);i=end;}});
  term.attachCustomKeyEventHandler(event=>{
   const selection=term.getSelection(),shortcut=terminalShortcut(event,selection);
   if(shortcut==='copy'){if(selection)void request('copy',{text:selection}).catch(showError);event.preventDefault();return false;}
   if(shortcut==='search'){openSearch();event.preventDefault();return false;}
   return true;
  });
  select(tab.id);await request('attach',{id:tab.id});notice('');
 }catch(error){showError(error);}finally{creating=false;setLaunchDisabled(!ready||tabs.size>=4);}
}
function openSearch(){$('find').hidden=false;$('query').focus();fit(tabs.get(current));}
const find=previous=>{const tab=tabs.get(current);if(tab&&$('query').value)tab.search[previous?'findPrevious':'findNext']($('query').value);};
$('query').oninput=()=>find(false);$('next').onclick=()=>find(false);$('previous').onclick=()=>find(true);
$('query').onkeydown=e=>{if(e.key==='Enter'){e.preventDefault();find(e.shiftKey);}if(e.key==='Escape')$('find-close').click();};
$('find-close').onclick=()=>{$('find').hidden=true;fit(tabs.get(current));tabs.get(current)?.term.focus();};
$('search').onclick=openSearch;$('shell').onclick=()=>void create('shell');$('start').onclick=$('cli').onclick=()=>void create('cli');$('start-agent').onclick=$('agent').onclick=()=>void create('agent');
$('launch-menu').addEventListener('toggle',event=>{if(event.newState==='open'){const box=$('new').getBoundingClientRect();$('launch-menu').style.left=`${Math.max(8,Math.min(box.left,innerWidth-210))}px`;$('cli').focus();}});
$('hide').onclick=$('chat').onclick=()=>void request('hide').catch(showError);
$('tabs').onkeydown=e=>{if(!['ArrowLeft','ArrowRight','Home','End'].includes(e.key))return;const ids=[...tabs.keys()],index=ids.indexOf(current);const next=e.key==='Home'?0:e.key==='End'?ids.length-1:(index+(e.key==='ArrowRight'?1:-1)+ids.length)%ids.length;if(ids[next]){e.preventDefault();select(ids[next]);tabs.get(ids[next]).select.focus();}};
new ResizeObserver(()=>activate(false)).observe($('panes'));
bridge.onEvent(event=>{
 const tab=tabs.get(event.id);
 if(event.type==='data'&&tab)tab.term.write(event.text,()=>void request('ack',{id:tab.id,sequence:event.sequence}).catch(showError));
 else if(event.type==='exit'&&tab){tab.state='exited';tab.term.writeln(`\r\n[Process exited: ${event.exitCode}]`);if(current===tab.id)select(tab.id);}
 else if(event.type==='stopped'&&tab){tab.state='stopped';if(current===tab.id)select(tab.id);}
 else if(event.type==='error'){if(tab)tab.state='uncertain';notice(event.message);}
 else if(event.type==='layout'){activate(false);}
 else if(event.type==='appearance')appearance(event);
 else if(event.type==='availability'){ready=event.available;setLaunchDisabled(!ready||tabs.size>=4);if(ready)notice('');}
 else if(event.type==='visibility'){visible=event.visible;for(const item of tabs.values()){item.term.options.disableStdin=!visible||item.id!==current;if(!visible)item.term.blur();}if(visible)activate(event.focus!==false);}
});
void bridge.request({protocol:1,action:'handshake'}).then(value=>{generation=value.generation;ready=value.available;visible=value.presentation?.visible===true;appearance(value.presentation||{});activate();setLaunchDisabled(!ready);if(!ready)notice('The native terminal runtime is unavailable on this build. Chat and Knowledge are still available.');}).catch(showError);
