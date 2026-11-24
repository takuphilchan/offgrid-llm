import {randomUUID} from 'node:crypto';
const bounded=(value,bytes)=>{while(Buffer.byteLength(JSON.stringify(value))>bytes)value=value.slice(0,Math.floor(value.length*.8));return value;};

// Frame objects come from the browser transport. Website IDs, iframe names and
// URLs never identify an input target. Reloads replace document identity even
// when the URL and all visible controls remain the same.
export class BrowserFrames {
  constructor(driver){
    this.driver=driver;this.page=driver.page;this.entries=new Map();this.selected=this.page.mainFrame();this.epoch=0;this.issued=new Map();this.originCache=new Map();this.processSessions=new Map();
    for(const frame of this.page.frames())this.register(frame);
    this.page.on('frameattached',frame=>{this.register(frame);this.epoch++;});
    this.page.on('framenavigated',frame=>{
      this.releaseProcessSession(frame);
      const entry=this.register(frame);entry.documentID=randomUUID();this.issued.delete(entry.id);this.epoch++;
      if(this.selected===frame)this.driver.observation='';
      else for(let parent=this.selected?.parentFrame();parent;parent=parent.parentFrame())if(parent===frame){this.selected=null;this.driver.observation='';break;}
    });
    this.page.on('framedetached',frame=>{
      this.releaseProcessSession(frame);
      const entry=this.entries.get(frame);if(entry)this.issued.delete(entry.id);
      this.entries.delete(frame);this.epoch++;
      if(this.selected===frame){this.selected=null;this.driver.observation='';}
    });
  }
  releaseProcessSession(frame){
    const session=this.processSessions.get(frame);this.processSessions.delete(frame);
    if(session)void session.detach().catch(()=>{});
  }
  register(frame){
    let entry=this.entries.get(frame);
    if(!entry){entry={id:randomUUID(),documentID:randomUUID(),frame};this.entries.set(frame,entry);}
    return entry;
  }
  current(){
    if(!this.selected||this.selected.isDetached())throw Error('Selected frame is unavailable. List frames and explicitly select a current frame.');
    return this.selected;
  }
  provenance(){const entry=this.entries.get(this.current());return {frame_id:entry.id,frame_document_id:entry.documentID};}
  async origins(){
    if(this.entries.size>64)throw Error('Frame tree exceeds the 64-frame observation bound.');
    this.cdp??=await this.page.context().newCDPSession(this.page);
    const {frameTree}=await this.cdp.send('Page.getFrameTree');
    const byID=new Map();this.originSessions=new Map();this.processRootIDs=new Set();
    const collect=(node,session)=>{
      if(!byID.has(node.frame.id)&&byID.size>=64)throw Error('Frame tree exceeds the 64-frame observation bound.');
      byID.set(node.frame.id,node.frame);this.originSessions.set(node.frame.id,session);
      for(const child of node.childFrames??[])collect(child,session);
    };
    collect(frameTree,this.cdp);this.processRootIDs.add(frameTree.frame.id);
    // Chromium moves distinct-site frames into separate targets. Use the public
    // Frame -> CDPSession binding, never a guessed target ID or global tab scan.
    for(const frame of this.entries.keys()){
      if(frame===this.page.mainFrame())continue;
      if(!this.processSessions.has(frame)){
        let session=null;try{session=await this.page.context().newCDPSession(frame);}catch{/* inline frame belongs to an existing target */}
        this.processSessions.set(frame,session);
      }
      const session=this.processSessions.get(frame);
      if(session){
        try{const tree=await session.send('Page.getFrameTree');collect(tree.frameTree,session);this.processRootIDs.add(tree.frameTree.frame.id);}
        catch{this.releaseProcessSession(frame);throw Error('Frame process changed during inspection.');}
      }
    }
    const frames=Array.from(byID.values());
    const retained=new Map();
    for(const frame of frames){
      const key=frame.id+':'+frame.loaderId;
      let origin=this.originCache.get(key);
      if(origin===undefined){
        try{
          // Frame.securityOrigin can expose an opaque document's precursor
          // origin. Read the actual origin in a fresh browser-isolated world:
          // page-authored window.origin overrides cannot enter this realm.
          const session=this.originSessions.get(frame.id);
          const world=await session.send('Page.createIsolatedWorld',{frameId:frame.id,worldName:'offgrid-origin-v1'});
          const value=await session.send('Runtime.evaluate',{contextId:world.executionContextId,expression:'globalThis.origin',returnByValue:true});
          origin=!value.exceptionDetails&&typeof value.result?.value==='string'?value.result.value:'';
        }catch{origin='';}
      }
      frame.actualOrigin=origin;retained.set(key,origin);
    }
    this.originCache=retained;return frames;
  }
  permitted(frame,origins){
    for(let current=frame;current;current=current.parentFrame()){
      if(current.isDetached()||!this.driver.allows(current.url()))return false;
      // Browser-isolated origin reads exclude opaque sandbox documents. Page
      // main-world location.origin/window.origin and iframe src are not authority.
      // URL matching here only checks scope conservatively: every browser frame
      // with that URL must have an allowed origin. It never selects the target.
      // Ambiguous approved/opaque duplicates are unavailable, not guessed.
      const candidates=origins.filter(candidate=>candidate.url===current.url());
      if(!candidates.length||candidates.some(candidate=>!this.driver.allows(candidate.actualOrigin)))return false;
    }
    return true;
  }
  async assertScope(frame=this.current()){
    const epoch=this.epoch,origins=await this.origins();
    if(epoch!==this.epoch||!this.permitted(frame,origins))throw Error('Selected frame is unavailable or outside approved origins.');
    return frame;
  }
  async list(){
    const epoch=this.epoch,origins=await this.origins(),frames=[];
    this.issued.clear();
    for(const [frame,entry] of this.entries){
      const available=this.permitted(frame,origins),parent=this.entries.get(frame.parentFrame());
      const value={frame_id:entry.id,document_id:entry.documentID,parent_frame_id:parent?.id??'',selected:frame===this.selected,available};
      if(available){
        value.url=bounded(frame.url(),256);value.url_limited=value.url!==frame.url();value.name=bounded(frame.name(),80);
        this.issued.set(entry.id,{...entry});
      }else value.reason='origin_or_document_unavailable';
      frames.push(value);
    }
    if(epoch!==this.epoch){this.issued.clear();throw Error('Frame inventory changed; list frames again.');}
    return {frames,max_frames:64,selected_frame_id:this.selected?this.entries.get(this.selected)?.id??'':'',coverage:'complete'};
  }
  async select(args){
    const issued=this.issued.get(args.frame_id),current=issued&&this.entries.get(issued.frame);
    if(!issued||!current||issued.documentID!==args.document_id||current.documentID!==args.document_id)throw Error('Unknown or stale frame reference. List frames again.');
    await this.assertScope(issued.frame);
    if(this.entries.get(issued.frame)?.documentID!==args.document_id)throw Error('Frame changed during selection.');
    this.selected=issued.frame;this.driver.observation='';
    return this.driver.observe();
  }
  selectMain(){this.selected=this.page.mainFrame();this.driver.observation='';}
}
