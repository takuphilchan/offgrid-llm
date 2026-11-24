import {randomUUID} from 'node:crypto';

// This function runs only in a separate Chromium world. Page scripts cannot
// replace its MutationObserver, clear its state, or impersonate a capture mask.
function installWitness(key){
  let changed=false,visited=0;const observers=[],roots=[];
  const mark=()=>{changed=true;};
  const watch=root=>{
    if(root.getAnimations().length||root.querySelector('animate,animateMotion,animateTransform,set'))throw Error('Protected capture cannot establish animation coverage.');
    const observer=new MutationObserver(mark);observer.observe(root,{subtree:true,childList:true,attributes:true,characterData:true});observers.push(observer);roots.push(root);
    for(const event of ['input','change','scroll'])root.addEventListener(event,mark,true);
    const walker=document.createTreeWalker(root,NodeFilter.SHOW_ELEMENT);
    for(let node=walker.nextNode();node;node=walker.nextNode()){
      if(++visited>50000)throw Error('Capture witness exceeds document bounds.');
      if(node.shadowRoot)watch(node.shadowRoot);
    }
  };
  const stop=()=>{for(const observer of observers)observer.disconnect();for(const root of roots)for(const event of ['input','change','scroll'])root.removeEventListener(event,mark,true);window.removeEventListener('resize',mark,true);};
  try{watch(document);window.addEventListener('resize',mark,true);}
  catch(error){stop();throw error;}
  globalThis[key]={changed:()=>changed||observers.some(observer=>observer.takeRecords().length>0)||roots.some(root=>root.getAnimations().length>0),stop};
  return true;
}

export async function captureWitness(frames,origins){
  const witnesses=[],sessions=new Map();
  const call=(witness,fn)=>witness.session.send('Runtime.callFunctionOn',{executionContextId:witness.context,functionDeclaration:fn.toString(),arguments:[{value:witness.key}],returnByValue:true});
  const dispose=async()=>{
    await Promise.allSettled(witnesses.map(witness=>call(witness,function(key){globalThis[key]?.stop();delete globalThis[key];})));
    await Promise.allSettled(Array.from(sessions,async([session,state])=>{
      for(const [event,listener] of state.listeners)session.off(event,listener);
      await Promise.allSettled([session.send('CSS.disable'),session.send('Animation.disable')]);
    }));
  };
  try{
    for(const origin of origins){
      if(!Array.from(frames.entries.keys()).some(frame=>frame.url()===origin.url&&frames.permitted(frame,origins)))continue;
      const session=frames.originSessions.get(origin.id);
      if(!sessions.has(session)){
        // CSSOM edits have no DOM mutation records. Listen to Chromium itself;
        // page scripts cannot restore this retained event history. Initial CSS
        // inventory is drained before arming, and before any pixel geometry.
        // Page.captureScreenshot itself emits mediaQueryResultChanged even for
        // an unchanged viewport. Actual viewport changes are witnessed by the
        // isolated resize/scroll listeners and the retained pixel dimensions.
        const state={changed:false,armed:false,events:['CSS.styleSheetAdded','CSS.styleSheetRemoved','CSS.styleSheetChanged','CSS.fontsUpdated','Animation.animationCreated','Animation.animationStarted','Animation.animationUpdated']};
        state.listeners=state.events.map(event=>[event,()=>{if(state.armed)state.changed=event;}]);sessions.set(session,state);
        for(const [event,listener] of state.listeners)session.on(event,listener);
        await session.send('DOM.enable');await session.send('CSS.enable');await session.send('Animation.enable');state.armed=true;
      }
      const world=await session.send('Page.createIsolatedWorld',{frameId:origin.id,worldName:'offgrid-capture-witness-v1'});
      const witness={session,context:world.executionContextId,key:'capture_'+randomUUID().replaceAll('-','')};witnesses.push(witness);
      const result=await call(witness,installWitness);
      if(result.exceptionDetails||result.result?.value!==true){
        if(result.exceptionDetails?.exception?.description?.includes('animation coverage'))throw Error('Protected capture cannot establish animation coverage. Wait for animations to finish or use structured controls.');
        throw Error('Protected capture witness is unavailable.');
      }
    }
    return {dispose,assertUnchanged:async()=>{
      for(const witness of witnesses){
        const result=await call(witness,function(key){return globalThis[key]?.changed();});
        if(result.exceptionDetails||result.result?.value!==false)throw Error('Document changed during protected capture. Inspect again.');
      }
      const changed=Array.from(sessions.values()).find(state=>state.changed);
      if(changed)throw Error('Rendering changed during protected capture. Inspect again.');
    }};
  }catch(error){await dispose();throw error;}
}
