const test=require('node:test'),assert=require('node:assert/strict'),vm=require('node:vm'),fs=require('node:fs'),path=require('node:path');
const source=fs.readFileSync(path.join(__dirname,'../terminal-renderer.js'),'utf8');
function renderer(){
  let onEvent,resolveHandshake,frame=0;
  const nodes=new Map(),frames=new Map();
  const document={activeElement:null,documentElement:{dataset:{},style:{setProperty(){},removeProperty(){}}},
    querySelectorAll:()=>[],getElementById(id){
      if(!nodes.has(id))nodes.set(id,{id,hidden:id==='find',addEventListener(){},focus(){document.activeElement=this;}});
      return nodes.get(id);
    }};
  const controls=()=>({isOpen:()=>false,localize(){}});
  const context=vm.createContext({document,window:{terminalDesktop:{onEvent:callback=>{onEvent=callback;},request:()=>new Promise(resolve=>{resolveHandshake=resolve;})}},
    localStorage:{getItem:()=>null,setItem(){}},terminalPreferences:{load:()=>({})},
    terminalTranslations:{configure(){},render(){},text:(_locale,key)=>key},Terminal:{strings:{}},
    mountTerminalPreferences:controls,mountTerminalSearch:controls,mountTerminalCommands:controls,
    matchMedia:()=>({matches:false,addEventListener(){}}),ResizeObserver:class{observe(){}},
    requestAnimationFrame:callback=>{frames.set(++frame,callback);return frame;},cancelAnimationFrame:id=>frames.delete(id)});
  vm.runInContext(source,context);
  return {document,event:onEvent,visible:()=>vm.runInContext('visible',context),
    async handshake(visible){resolveHandshake({generation:'test',available:true,presentation:{visible,locale:'en'}});await Promise.resolve();await Promise.resolve();},
    flush(){for(const [id,callback] of frames){frames.delete(id);callback();}}};
}
test('a late hidden handshake cannot overwrite a newer terminal-open event',async()=>{
  const view=renderer();view.event({type:'visibility',visible:true});
  await view.handshake(false);view.flush();
  assert.equal(view.visible(),true);
  assert.equal(view.document.activeElement?.id,'terminal-welcome-title');
});
test('a late visible handshake cannot reopen a terminal the host has hidden',async()=>{
  const view=renderer();view.event({type:'visibility',visible:false});
  await view.handshake(true);view.flush();
  assert.equal(view.visible(),false);assert.equal(view.document.activeElement,null);
});
test('handshake initializes visibility when no newer host event has arrived',async()=>{
  const view=renderer();await view.handshake(true);view.flush();
  assert.equal(view.visible(),true);assert.equal(view.document.activeElement?.id,'terminal-welcome-title');
  view.event({type:'visibility',visible:false});assert.equal(view.visible(),false);
  view.event({type:'visibility',visible:true});assert.equal(view.visible(),true);
});
test('a late handshake preserves the host request to retain focus outside the terminal',async()=>{
  const view=renderer();view.event({type:'visibility',visible:true,focus:false});
  await view.handshake(false);view.flush();
  assert.equal(view.visible(),true);assert.equal(view.document.activeElement,null);
});
