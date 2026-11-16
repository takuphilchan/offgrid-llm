const {test} = require('node:test');
const assert = require('node:assert/strict');
const {createCommandWindowMotion,createCommandTransitionBridge} = require('../command-window-motion');
const {createCommandWindow} = require('../command-window');
const delay = ms => new Promise(resolve => setTimeout(resolve, ms));
function fixture(options = {}) {
 let bounds={x:100,y:80,width:1200,height:800},visible=true,destroyed=false;
 const frames=[],messages=[];
 const window={isDestroyed:()=>destroyed,isVisible:()=>visible,isMinimized:()=>false,isMaximized:()=>false,isFullScreen:()=>false,
  getBounds:()=>({...bounds}),getNormalBounds:()=>({...bounds}),setBounds:next=>{assert.equal(destroyed,false);bounds=next;frames.push(next);},
  setAlwaysOnTop(){},setResizable(){},setMaximizable(){},setFullScreenable(){},setMaximumSize(){},setMinimumSize(){},
  show:()=>{visible=true;},hide:()=>{visible=false;},focus(){},webContents:{send:(...value)=>messages.push(value)}};
 const motion=createCommandWindowMotion({reducedMotion:()=>false,...options});
 const area={x:0,y:0,width:1920,height:1080};
 const control=createCommandWindow({window:()=>window,screen:{getDisplayMatching:()=>({workArea:area}),getCursorScreenPoint:()=>({}),getDisplayNearestPoint:()=>({workArea:area})},globalShortcut:{},ready:()=>true,changed:()=>{},motion});
 return {window,motion,control,frames,messages,bounds:()=>bounds,destroy:()=>{destroyed=true;}};
}
test('resize uses bounded eased frames and commits exact final geometry once',async()=>{
 const f=fixture();await f.control.open();f.frames.length=0;
 await f.control.setLayout('voice',{prepared:true});
 assert.ok(f.frames.length>3 && f.frames.length<25);
 assert.deepEqual([f.bounds().width,f.bounds().height],[420,540]);
 assert.ok(f.frames.every((frame,i)=>!i || frame.height>=f.frames[i-1].height));
 assert.equal(f.frames.filter(frame=>frame.height===540).length>=1,true);
 assert.equal(f.control.fullState().bounds.width,1200);
});
test('rapid reversal cancels old frames and full expansion preserves original geometry',async()=>{
 const f=fixture();await f.control.open();
 const voice=f.control.setLayout('voice',{prepared:true});await delay(35);
 const text=f.control.setLayout('input',{prepared:true});await Promise.all([voice,text]);
 assert.equal(f.bounds().height,164);
 const expansion=f.control.expand();await delay(30);await f.control.open();await expansion;
 assert.equal(f.bounds().height,164);
 await f.control.expand();assert.deepEqual(f.bounds(),{x:100,y:80,width:1200,height:800});
});
test('dismiss and destruction cancel resizing without late writes or showing again',async()=>{
 const f=fixture();await f.control.open();
 const voice=f.control.setLayout('voice',{prepared:true});await delay(25);f.control.dismiss();
 const count=f.frames.length;await voice;await delay(25);
 assert.equal(f.frames.length,count);assert.equal(f.window.isVisible(),false);
 await f.control.open();assert.deepEqual([f.bounds().width,f.bounds().height],[420,540]);
 const other=fixture();const pending=other.control.open();other.destroy();await pending;
 assert.equal(other.motion.active,false);
});

test('repeated expansion waits for the final full bounds instead of freezing halfway',async()=>{
 const f=fixture();await f.control.open();
 const first=f.control.expand();await delay(35);
 const second=f.control.expand();await Promise.all([first,second]);
 assert.deepEqual(f.bounds(),{x:100,y:80,width:1200,height:800});
 assert.equal(f.control.snapshot().mode,'full');assert.equal(f.motion.active,false);
});
test('OS renderer preference, application preference and hidden windows skip interpolation',async()=>{
 for (const type of ['os','app','hidden']) {
  const f=fixture({reducedMotion:()=>type==='app'});
  if(type==='hidden')f.window.hide();
  await f.motion.run(f.window,resize=>resize({x:20,y:30,width:420,height:540}),{prepared:true,reduceMotion:type==='os'});
  assert.equal(f.frames.length,1,type);
 }
});
test('preparation is bounded, requires the current token and releases on cancellation',async()=>{
 const bridge=createCommandTransitionBridge({timeout:30}),f=fixture({prepare:bridge.prepare});
 const pending=f.control.open();assert.equal(f.frames.length,0);
 const {id}=f.messages.at(-1)[1];
 assert.equal(bridge.acknowledge({id:id+1}),false);
 assert.equal(bridge.acknowledge({id,reduceMotion:true}),true);await pending;
 assert.equal(f.frames.length,1);assert.equal(bridge.acknowledge({id}),false);
 await f.control.expand(); // Missing renderer acknowledgment falls back without animation.
 assert.equal(f.bounds().width,1200);
 const opening=f.control.open();f.control.dismiss();await opening;
 assert.equal(f.motion.active,false);
});

test('an unavailable renderer or already-cancelled preparation cannot reject navigation',async()=>{
 const bridge=createCommandTransitionBridge(),controller=new AbortController(),f=fixture();
 f.window.webContents.send=()=>{throw Error('renderer closed');};
 assert.equal(await bridge.prepare(f.window,controller.signal),true);
 controller.abort();assert.equal(await bridge.prepare(f.window,controller.signal),true);
 assert.equal(bridge.acknowledge({id:1}),false);
});
