const test=require('node:test'),assert=require('node:assert/strict'),vm=require('node:vm'),fs=require('node:fs'),path=require('node:path');
function fixture(){
 const handlers={},listeners={},calls=[];class Element{closest(selector){if(this.target==='settings')return selector.includes('#workspace-sidebar .settings-navigation button')?this:null;return this.target||null;}matches(){return !!this.disabled;}}
 const context={require:()=>({contextBridge:{exposeInMainWorld(){}},ipcRenderer:{on:(n,f)=>listeners[n]=f,invoke:async(n)=>calls.push(n)},webFrame:{}}),process:{platform:'linux',argv:[]},window:{addEventListener(){}},document:{addEventListener:(n,f)=>handlers[n]=f,documentElement:{toggleAttribute(){}},querySelector:()=>null},Element,cancelAnimationFrame(){},requestAnimationFrame:()=>1};
 vm.runInNewContext(fs.readFileSync(path.join(__dirname,'../preload.js'),'utf8'),context);
 listeners['terminal-visibility']({},true);
 return {handlers,calls,Element,listeners};
}
test('sidebar activation reveals the workspace without changing the route or closing a process',()=>{
 const {handlers,calls,Element}=fixture();const target=new Element();target.target=target;
 handlers.click({target,button:0});assert.deepEqual(calls,['terminal-hide']);
});
test('same-section Settings activation reveals its existing workspace without closing a process',()=>{
 const {handlers,calls,Element}=fixture();const target=new Element();target.target='settings';
 handlers.click({target,button:0});assert.deepEqual(calls,['terminal-hide']);
});
test('disabled, modified and unrelated clicks do not dismiss the terminal',()=>{
 const {handlers,calls,Element}=fixture();const target=new Element();
 handlers.click({target,button:0});target.target=target;target.disabled=true;
 handlers.click({target,button:0});target.disabled=false;
 handlers.click({target,button:0,ctrlKey:true});handlers.click({target,button:1});
 assert.deepEqual(calls,[]);
});
