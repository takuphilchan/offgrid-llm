// Capture must not silently expose a closed shadow tree that DOM locators cannot
// mask. Chromium's inspector reports shadow-root kind without asking website JS.
// Read only scoped documents; opaque child surfaces are masked by the caller.
export async function assertCaptureShadowCoverage(frames,origins){
  const cdp=frames.cdp;
  const counters=await cdp.send('Memory.getDOMCounters');
  if(counters.nodes>50000)throw Error('Capture exceeds the browser DOM inspection bound.');
  const {root}=await cdp.send('DOM.getDocument',{depth:0,pierce:false});
  let visited=0;
  const walk=async(session,backendNodeId)=>{
    // Do not pierce document boundaries implicitly. Expand known open roots
    // explicitly; all child documents are checked against browser origin data.
    const {node}=await session.send('DOM.describeNode',{backendNodeId,depth:-1,pierce:false});
    const pending=[node];
    while(pending.length){
      const current=pending.pop();if(++visited>50000)throw Error('Capture exceeds the browser DOM inspection bound.');
      for(const shadow of current.shadowRoots??[]){
        if(shadow.shadowRootType==='closed')throw Error('Closed shadow content prevents protected capture. Use structured controls or manual inspection.');
        if(shadow.shadowRootType==='open')await walk(session,shadow.backendNodeId);
      }
      pending.push(...current.children??[]);
      // contentDocument is intentionally not traversed here.
    }
  };
  await walk(cdp,root.backendNodeId);
  for(const origin of origins.filter(value=>value.parentId)){
    const allowed=Array.from(frames.entries.keys()).some(frame=>frame.url()===origin.url&&frames.permitted(frame,origins));
    if(!allowed)continue;
    const session=frames.originSessions.get(origin.id);
    if(frames.processRootIDs.has(origin.id)){
      const {root}=await session.send('DOM.getDocument',{depth:0,pierce:false});
      await walk(session,root.backendNodeId);continue;
    }
    const owner=await session.send('DOM.getFrameOwner',{frameId:origin.id});
    const {node}=await session.send('DOM.describeNode',{backendNodeId:owner.backendNodeId,depth:0,pierce:false});
    if(!node.contentDocument)throw Error('Child document shadow coverage is unavailable.');
    await walk(session,node.contentDocument.backendNodeId);
  }
}
