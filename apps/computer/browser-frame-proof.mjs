import {randomUUID} from 'node:crypto';
import {selectors} from 'playwright';

const engine='offgrid_native_frame_v1';
let registered;
export function registerFrameProof(){
 return registered??=selectors.register(engine,()=>{
  const connected=Object.getOwnPropertyDescriptor(Node.prototype,'isConnected').get;
  const owner=Object.getOwnPropertyDescriptor(Node.prototype,'ownerDocument').get;
  const query=(root,selector)=>{
   if(!connected.call(root)||owner.call(root)!==document)throw Error('Native frame target changed.');
   const command=JSON.parse(selector);
   if(command.op==='prime')return root;
   if(command.op!=='identify'||!/^offgrid_frame_[a-f0-9]{32}$/.test(command.name??'')||!/^[a-f0-9]{32}$/.test(command.nonce??''))throw Error('Invalid native frame proof.');
   if(typeof globalThis[command.name]!=='function')throw Error('Native frame binding is unavailable.');
   globalThis[command.name](command.nonce);return root;
  };
  return {query,queryAll:(root,selector)=>[query(root,selector)]};
 },{contentScript:true});
}

const findFrame=(tree,id)=>{
 if(tree.frame.id===id)return tree.frame;
 for(const child of tree.childFrames??[]){const found=findFrame(child,id);if(found)return found;}
};

// A public Frame->CDPSession binding is available for page and OOP frame targets.
// Inline frames share their nearest target ancestor. The isolated selector's
// native binding event identifies the exact retained node's frame in that target;
// neither URLs, DOM attributes nor private Playwright channel IDs are identity.
export async function acquireBrowserFrameProof(handle,{signal,timeoutMs=5000}={}){
 if(!Number.isSafeInteger(timeoutMs)||timeoutMs<1||timeoutMs>10000)throw Error('Invalid frame proof timeout.');
 signal?.throwIfAborted();
 const frame=await handle.ownerFrame();if(!frame||frame.isDetached())throw Error('Native frame target unavailable.');
 const context=frame.page().context();let session;
 for(let candidate=frame;candidate;candidate=candidate.parentFrame()){
  try{session=await context.newCDPSession(candidate);break;}catch{/* inline frames share an ancestor target */}
 }
 if(!session)throw Error('Native frame session unavailable.');
 const contexts=new Map(),name='offgrid_frame_'+randomUUID().replaceAll('-',''),nonce=randomUUID().replaceAll('-','');
 let exhausted=false,closed=false,sessionClosed=false,bound=false,settled=false,conflict=false,proof,timer,resolveMatch,rejectMatch;
 const match=new Promise((yes,no)=>{resolveMatch=yes;rejectMatch=no;});match.catch(()=>{});
 const created=({context:value})=>{if(contexts.size>=256){exhausted=true;return;}contexts.set(value.id,value);};
 const destroyed=event=>{for(const [id,value] of contexts)if(id===event.executionContextId||value.uniqueId===event.executionContextUniqueId)contexts.delete(id);};
 const cleared=()=>contexts.clear();
 const sessionEnded=()=>{sessionClosed=true;contexts.clear();if(!settled)rejectMatch(Error('Native frame session closed.'));};
 const abort=()=>rejectMatch(signal.reason??Error('Native frame proof cancelled.'));
 const binding=event=>{
  if(event.name!==name||event.payload!==nonce)return;
  const value=contexts.get(event.executionContextId);
  if(!value||value.auxData?.isDefault!==false||value.auxData?.type!=='isolated'||typeof value.auxData.frameId!=='string'||!value.uniqueId)return;
  if(proof&&proof!==value){conflict=true;rejectMatch(Error('Native frame proof is ambiguous.'));return;}
  proof=value;resolveMatch(value);
 };
 session.on('Runtime.executionContextCreated',created);session.on('Runtime.executionContextDestroyed',destroyed);
 session.on('Runtime.executionContextsCleared',cleared);session.on('Runtime.bindingCalled',binding);
 session.on('close',sessionEnded);
 const query=async command=>{const result=await handle.$(`${engine}=${JSON.stringify(command)}`);if(!result)throw Error('Native frame target unavailable.');await result.dispose();};
 const removeBinding=async()=>{
  if(!bound||sessionClosed){bound=false;return;}
  await session.send('Runtime.removeBinding',{name});
  // removeBinding only retires notifications. Remove the temporary global in
  // each still-live isolated context where this exact native binding was added.
  for(const value of contexts.values())if(value.auxData?.isDefault===false&&value.auxData?.type==='isolated'&&value.uniqueId){
   const result=await session.send('Runtime.evaluate',{uniqueContextId:value.uniqueId,expression:`delete globalThis[${JSON.stringify(name)}]`,returnByValue:true});
   if(result.exceptionDetails||result.result?.value!==true)throw Error('Native frame binding cleanup unconfirmed.');
  }
  bound=false;
 };
 const close=async()=>{
  if(closed)return;
  clearTimeout(timer);signal?.removeEventListener('abort',abort);
  const errors=[];
  try{await removeBinding();}catch(error){errors.push(error);}
  try{if(!sessionClosed)await session.detach();closed=true;}catch(error){if(sessionClosed)closed=true;else errors.push(error);}
  if(closed){
   session.removeListener('Runtime.executionContextCreated',created);session.removeListener('Runtime.executionContextDestroyed',destroyed);
   session.removeListener('Runtime.executionContextsCleared',cleared);session.removeListener('Runtime.bindingCalled',binding);contexts.clear();
   session.removeListener('close',sessionEnded);
  }
  if(errors.length)throw new AggregateError(errors,'Native frame proof cleanup unconfirmed.');
 };
 try{
  await session.send('Runtime.enable');await query({op:'prime'});
  const names=new Set([...contexts.values()].filter(value=>value.auxData?.isDefault===false&&value.auxData?.type==='isolated').map(value=>value.name));
  if(exhausted||!names.size||names.size>8||[...names].some(value=>typeof value!=='string'||!value||value.length>256))throw Error('Native frame context unavailable.');
  bound=true;
  for(const executionContextName of names)await session.send('Runtime.addBinding',{name,executionContextName});
  timer=setTimeout(()=>rejectMatch(Error('Native frame proof timed out.')),timeoutMs);
  signal?.addEventListener('abort',abort,{once:true});signal?.throwIfAborted();
  await query({op:'identify',name,nonce});await match;settled=true;
  clearTimeout(timer);signal?.removeEventListener('abort',abort);signal?.throwIfAborted();
  if(conflict||exhausted||contexts.get(proof.id)!==proof)throw Error('Native frame context changed.');
  const tree=await session.send('Page.getFrameTree'),native=findFrame(tree.frameTree,proof.auxData.frameId);
  if(!native?.loaderId)throw Error('Native frame document unavailable.');
  const frameId=native.id,loaderId=native.loaderId,uniqueContextId=proof.uniqueId;
  await removeBinding();session.removeListener('Runtime.bindingCalled',binding);
  const validate=async()=>{
   signal?.throwIfAborted();
   if(closed||exhausted||frame.isDetached()||contexts.get(proof.id)!==proof)throw Error('Native frame document changed.');
   await query({op:'prime'});
   const current=findFrame((await session.send('Page.getFrameTree')).frameTree,frameId);
   if(current?.loaderId!==loaderId||contexts.get(proof.id)!==proof)throw Error('Native frame document changed.');
  };
  await validate();return {session,frameId,loaderId,uniqueContextId,validate,close};
 }catch(error){
  if(!settled)rejectMatch(error);
  try{await close();}catch(cleanup){throw new AggregateError([error,cleanup],'Native frame proof failed and cleanup is unconfirmed.');}
  throw error;
 }
}
