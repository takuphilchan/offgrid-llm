import {randomUUID} from 'node:crypto';
import {BrowserDriver} from './browser.mjs';
import {BrowserFailure} from './browser-outcome.mjs';

export const workspaceActions = Object.freeze(['browser_tabs','browser_tab_open','browser_tab_select','browser_tab_close']);
const reject = message => {throw new BrowserFailure('computer_browser_precondition_failed',message);};
const displayText=(value,bytes)=>{
  let text=value;
  while(Buffer.byteLength(JSON.stringify(text))>bytes)text=text.slice(0,Math.floor(text.length*0.8));
  return text;
};

// Owns only the isolated context created for this task. A queue binds selection
// and input execution; each page driver independently checks exact observations.
export class BrowserWorkspace {
  static async open(origin, options = {}) {
    const root = await BrowserDriver.open(origin,{...options,managedTabs:true});
    return new BrowserWorkspace(root);
  }
  constructor(root) {
    this.root=root;this.browser=root.browser;this.allowedOrigins=root.allowedOrigins;
    this.tabs=new Map();this.selected='';this.tail=Promise.resolve();this.closed=false;this.control='automated';this.controlEpoch=0;
    this.register(root.page,root);
    root.registerTab=page=>this.register(page);
  }
  get page() {return this.tabs.get(this.selected)?.page;}
  disableFileTransfers(){this.root.disableFileTransfers();for(const driver of this.tabs.values())driver.disableFileTransfers();}
  async bindFileTask(directory,binding,checkStorage) {
    if(this.closed)throw Error('upload_store_unavailable');
    await this.root.bindFileTask(directory,binding,checkStorage);
    for(const driver of this.tabs.values()){driver.uploadStore=this.root.uploadStore;driver.checkFileStorage=checkStorage;driver.fileTransfersDisabled=false;}
  }
  register(page,existing) {
    for(const driver of this.tabs.values())if(driver.page===page)return driver;
    if(this.closed||this.tabs.size>=8){void page.close().catch(()=>{});return null;}
    const driver=existing??new BrowserDriver(this.browser,page,this.root.origin,{
      allowedOrigins:this.allowedOrigins,downloadDir:this.root.downloadDir,upload:this.root.upload,uploadStore:this.root.uploadStore,
    });
    driver.tabID=randomUUID();driver.blockedOrigins=this.root.blockedOrigins;
    driver.checkFileStorage=this.root.checkFileStorage;driver.fileTransfersDisabled=this.root.fileTransfersDisabled;
    page.setDefaultTimeout(8000);
    if(!existing)page.on('dialog',dialog=>void dialog.dismiss().catch(()=>{}));
    this.tabs.set(driver.tabID,driver);
    if(!this.selected)this.selected=driver.tabID;
    page.once('close',()=>{
      this.tabs.delete(driver.tabID);
      if(this.selected===driver.tabID){
        // Only an explicit close operation selects a replacement and returns
        // its fresh observation. Human/page closure must not retarget a
        // previously prepared navigation to a different page.
        this.selected=this.closingTab===driver.tabID?(this.tabs.keys().next().value??''):'';
        if(this.selected)this.tabs.get(this.selected).observation='';
      }
      if(!this.tabs.size&&!this.closed)this.onEmpty?.();
    });
    return driver;
  }
  serial(action) {
    if(this.control!=='automated')return Promise.reject(new BrowserFailure('computer_browser_precondition_failed','Browser is under manual control. Continue explicitly before automation.'));
    const next=this.tail.then(()=>{if(this.closed)reject('Browser workspace is closed.');if(this.control!=='automated')reject('Browser is under manual control.');return action();});
    this.tail=next.catch(()=>{});
    return next;
  }
  async clearObservations() {
    const disposed=[];
    for(const driver of this.tabs.values()){
      driver.observation='';driver.observedAt=0;driver.revision=-1;driver.formDigest='';driver.observedFrame=undefined;driver.searchQuery=undefined;
      for(const handle of driver.elements.values())disposed.push(handle.dispose());
      if(driver.observedDocument)disposed.push(driver.observedDocument.dispose());
      driver.elements.clear();driver.observedDocument=undefined;
      driver.targetDigests?.clear();driver.cursors?.clear();driver.observedOptions?.clear();driver.frames.issued.clear();
      // Manual selection never renews the previous file-upload grant.
      driver.upload=undefined;
    }
    await Promise.allSettled(disposed);
  }
  takeover({timeoutMs=15000}={}) {
    if(this.closed)return Promise.reject(Error('Browser workspace is closed.'));
    if(!Number.isSafeInteger(timeoutMs)||timeoutMs<1||timeoutMs>30000)return Promise.reject(Error('Invalid handoff timeout.'));
    if(this.control==='resuming'){
      // A later human request supersedes evidence disposal for an earlier
      // continuation. No automation has been admitted during that disposal.
      ++this.controlEpoch;this.control='manual';
    }
    if(!this.manualDrain){
      this.control='draining';
      const admitted=this.tail;
      this.manualDrain=(async()=>{
        await admitted;
        if(this.closed||!this.browser.isConnected())throw Error('Browser handoff could not be confirmed.');
        await this.clearObservations();
        if(this.closed||!this.browser.isConnected())throw Error('Browser handoff could not be confirmed.');
        this.control='manual';
        return {state:'manual',tabs:this.tabs.size};
      })();
    }
    let timer;
    return Promise.race([this.manualDrain,new Promise((_,reject)=>{timer=setTimeout(()=>reject(Error('Browser handoff could not be confirmed.')),timeoutMs);})]).finally(()=>clearTimeout(timer));
  }
  async resumeManual() {
    if(this.control!=='manual'||this.closed||!this.browser.isConnected()||!this.tabs.size)throw Error('Retained browser is unavailable.');
    const epoch=++this.controlEpoch;this.control='resuming';
    try{
      await this.clearObservations();
      if(epoch!==this.controlEpoch||this.closed||!this.browser.isConnected())throw Error('Retained browser is unavailable.');
      this.manualDrain=undefined;this.control='automated';
    }catch(error){if(epoch===this.controlEpoch&&this.control==='resuming')this.control='manual';throw error;}
  }
  current() {
    const driver=this.tabs.get(this.selected);
    if(!driver||driver.page.isClosed())reject('Selected task tab is unavailable.');
    return driver;
  }
  validate(kind,args) {
    const fields=kind==='browser_tabs'?[]:kind==='browser_tab_open'?['url']:['tab_id'];
    if(!args||typeof args!=='object'||Array.isArray(args)||Object.keys(args).sort().join(',')!==fields.join(','))reject('Invalid browser tab arguments.');
    if(kind==='browser_tab_open') {
      if(typeof args.url!=='string'||Buffer.byteLength(args.url)>2048||!this.root.allows(args.url))reject('Navigation is outside approved origins.');
      if(this.tabs.size>=8)reject('This task already has eight tabs. Close an owned tab first.');
    } else if(kind!=='browser_tabs') {
      if(typeof args.tab_id!=='string'||!this.tabs.has(args.tab_id)||this.tabs.get(args.tab_id).page.isClosed())reject('Unknown or closed task tab.');
      if(kind==='browser_tab_close'&&this.tabs.size===1)reject('Keep one task tab open, or stop the task.');
    }
  }
  prepare(kind,args) {
    return this.serial(async()=>{
      if(!workspaceActions.includes(kind)) {
        const driver=this.current();
        if(!driver.allows(driver.page.url()))reject('Selected tab is unavailable or outside approved sites. Select another task tab.');
        return driver.prepare(kind,args);
      }
      this.validate(kind,args);
      return {prepared:true,approval_class:kind==='browser_tabs'?'read_only':kind==='browser_tab_close'?'consequential':'reversible'};
    });
  }
  async list() {
    const tabs=[];
    for(const [id,driver] of this.tabs) {
      const permitted=!driver.page.isClosed()&&driver.allows(driver.page.url());
      const url=permitted?driver.page.url():'';
      const title=permitted?await driver.page.title().catch(()=>''):'';
      tabs.push({tab_id:id,selected:id===this.selected,available:permitted,
        ...(permitted?{url:displayText(url,2048),url_limited:Buffer.byteLength(JSON.stringify(url))>2048,title:displayText(title,512),title_limited:Buffer.byteLength(JSON.stringify(title))>512,document_id:driver.documentID}:{})});
    }
    return {tabs,max_tabs:8,selected_tab_id:this.selected,allowed_origins:this.allowedOrigins};
  }
  execute(kind,args,fileAction) {
    return this.serial(async()=>{
      if(!workspaceActions.includes(kind)) {
        const driver=this.current();
        if(!driver.allows(driver.page.url()))reject('Selected tab is unavailable or outside approved sites. Select another task tab.');
        return {...await driver.execute(kind,args,fileAction),tab_id:driver.tabID,document_id:driver.documentID};
      }
      this.validate(kind,args);
      if(kind==='browser_tabs')return this.list();
      if(kind==='browser_tab_open') {
        // Creation has effects even if navigation subsequently fails. Do not
        // wrap failures below as safe preconditions or automatically replay.
        const page=await this.root.page.context().newPage();
        const driver=this.register(page);
        if(!driver)throw Error('Task tab creation outcome unavailable.');
        this.selected=driver.tabID;
        await page.goto(args.url,{waitUntil:'domcontentloaded',timeout:15000});
      } else if(kind==='browser_tab_select') {
        this.selected=args.tab_id;
        this.current().observation='';
        await this.current().page.bringToFront();
      } else {
        this.closingTab=args.tab_id;
        try {await this.tabs.get(args.tab_id).page.close();}
        finally {this.closingTab='';}
      }
      // Keep tab inventory separate: combining eight long tab descriptions
      // with a full observation can exceed transport after input succeeded.
      const receipt={dispatched:true,effect_status:'completed',verified:false,tab_id:this.selected};
      return this.current().withObservation(receipt);
    });
  }
  async close() {this.closed=true;await this.root.close();}
}
