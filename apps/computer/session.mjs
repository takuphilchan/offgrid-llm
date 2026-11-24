// Shared by the interactive CLI and the desktop-managed utility process.
// No renderer receives the session credential or arbitrary execution APIs.
import { mkdir, chmod } from 'node:fs/promises';
import { join } from 'node:path';
import { DispatchJournal } from './journal.mjs';
import { BrowserDriver } from './browser.mjs';
import { BrowserWorkspace } from './browser-workspace.mjs';
import { browserOrigins } from './browser-scope.mjs';
import { publicPage } from './network.mjs';
import { startDemo, DEMO_ORIGIN } from './demo.mjs';
import {policyAllows, validActionClass, validApprovalMode} from './approval-policy.mjs';
import {safeBrowserFailure} from './browser-outcome.mjs';
import {browserFileTask,browserFileBinding,browserFileAction,browserFileCapabilities} from './browser-file-binding.mjs';

export function localService(value) {
  const url = new URL(value);
  if (!['http:', 'https:'].includes(url.protocol) || !['127.0.0.1', '[::1]'].includes(url.hostname) || url.username || url.password || url.pathname !== '/' || url.search || url.hash) throw Error('service_invalid');
  return url;
}
export function browserOrigin(value) {
  if (value === 'demo' || value === DEMO_ORIGIN) return DEMO_ORIGIN;
  return publicPage(value).href;
}

export class CompanionSession {
  constructor({service, directory, emit = () => {}, openBrowser = (origin,options) => options.workspaceEnabled ? BrowserWorkspace.open(origin,options) : BrowserDriver.open(origin,options), prepared, upload, storageCheck}) {
    this.service = localService(service); this.directory = directory; this.emit = emit; this.openBrowser = openBrowser; this.upload=upload;
    this.abort = new AbortController(); this.pollAbort = new AbortController(); this.stopping = false; this.token = ''; this.state = 'idle';
    this.driver = prepared?.driver; this.demo = prepared?.demo;
    this.storageCheck=storageCheck;
  }
  publish(state, code) { this.state = state; this.emit({state, ...(code ? {code} : {})}); }
  async request(path, body) {
    const signals=[this.abort.signal,AbortSignal.timeout(20000)];
    if(path==='/api/v2/computer/companion/poll')signals.push(this.pollAbort.signal);
    const response = await fetch(new URL(path, this.service), { method: body === undefined ? 'GET' : 'POST', redirect: 'error',
      headers: {'Content-Type':'application/json', ...(this.token ? {Authorization:`Bearer ${this.token}`} : {})},
      ...(body === undefined ? {} : {body:JSON.stringify(body)}), signal:AbortSignal.any(signals) });
    if (!response.ok) throw Error('connection_lost');
    return response.json();
  }
  companion(path, body = {}) { return this.request(`/api/v2/computer/companion/${path}`, body); }
  async start({origin, code, workspace, networkMode = 'direct', approvalMode = 'scoped_changes',additionalOrigins=[],expectedBrowserWorkspace,fileTask}) {
    if (this.state !== 'idle' || this.stopping || !/^[a-f0-9]{64}$/.test(code)) throw Error('invalid_session');
    try {
      this.publish('starting');
      const expectedFileTask=fileTask===undefined?null:browserFileTask(fileTask,workspace);
      const identity = await this.request('/api/v2/system');
      if (identity.product !== 'offgrid' || identity.api_version !== 2 || (workspace && identity.workspace_id !== workspace)) throw Error('workspace_changed');
      if(!validApprovalMode(approvalMode)) throw Error('invalid_session');
      this.approvalMode=approvalMode;
      origin = browserOrigin(origin);
      const workspaceEnabled=identity.capabilities?.includes('browser-workspace-v1')===true;
      if(expectedFileTask&&!browserFileCapabilities.every(capability=>identity.capabilities?.includes(capability)))throw Error('computer_upgrade_required');
      if(expectedBrowserWorkspace!==undefined && (typeof expectedBrowserWorkspace!=='boolean'||expectedBrowserWorkspace!==workspaceEnabled))throw Error('computer_browser_scope_changed');
      if(!Array.isArray(additionalOrigins)||additionalOrigins.length>15||(origin===DEMO_ORIGIN&&additionalOrigins.length))throw Error('browser_scope_invalid');
      const scope=origin===DEMO_ORIGIN?[origin]:browserOrigins(origin,additionalOrigins);
      if((scope.length>1||this.driver instanceof BrowserWorkspace)&&!workspaceEnabled)throw Error('computer_upgrade_required');
      if (!['direct','trusted-vpn'].includes(networkMode) || (origin === DEMO_ORIGIN && networkMode !== 'direct')) throw Error('network_mode_invalid');
      if (origin === DEMO_ORIGIN && !this.demo) this.demo = await startDemo();
      if (this.stopping) throw Error('stopped');
      this.fileStorageReady=false;
      if(expectedFileTask&&this.storageCheck){
        this.fileStorageReady=await this.storageCheck.prepare()===true;
      }
      if (this.stopping) throw Error('stopped');
      await mkdir(this.directory, {recursive:true, mode:0o700});
      if (!this.driver) this.driver = await this.openBrowser(this.demo?.origin ?? origin, {testLoopback: !!this.demo, networkMode, downloadDir:join(this.directory,'downloads'),upload:this.fileStorageReady?this.upload:undefined,workspaceEnabled,additionalOrigins:scope.slice(1)});
      if(!this.fileStorageReady)this.driver.disableFileTransfers?.();
      if (this.stopping) throw Error('stopped');
      if(this.driver.allowedOrigins) {
        const expected=this.demo?[this.demo.origin]:scope;
        if(JSON.stringify(this.driver.allowedOrigins)!==JSON.stringify(expected))throw Error('browser_scope_invalid');
      }
      if(this.driver instanceof BrowserWorkspace) {
        this.emptyHandler=()=>{if(!this.stopping)void this.stop().catch(()=>{});};
        this.disconnectHandler=()=>{if(!this.stopping)void this.stop().catch(()=>{});};
        this.driver.onEmpty=this.emptyHandler;
        this.driver.browser.once('disconnected',this.disconnectHandler);
      } else this.driver.page?.once('close', () => { void this.stop().catch(()=>{}); });
      await mkdir(this.directory, {recursive:true, mode:0o700});
      this.journal = new DispatchJournal(join(this.directory, 'dispatch.sqlite'));
      await chmod(join(this.directory, 'dispatch.sqlite'), 0o600);
      const features = identity.capabilities?.includes('browser-discovery-v1') ? {browser_features:{discovery:1}} : {};
      if(workspaceEnabled&&this.driver instanceof BrowserWorkspace) {
        features.browser_features={discovery:1,workspace:1};features.browser_origins=scope;
        if(identity.capabilities?.includes('browser-structured-v1'))features.browser_features.structured=1;
      } else if(scope.length>1)throw Error('computer_upgrade_required');
      if(expectedFileTask){
        if(features.browser_features?.structured!==1)throw Error('computer_upgrade_required');
        features.browser_features.transfers=1;
      }
      const paired = await this.companion('pair', {code, origin:origin === DEMO_ORIGIN ? origin : new URL(origin).origin, protocol_version:1, approval_mode:approvalMode, ...features});
      if(expectedFileTask)this.fileBinding=browserFileBinding(paired.file_binding,expectedFileTask,paired.session?.id);
      else if(Object.hasOwn(paired,'file_binding'))throw Error('computer_browser_file_scope_invalid');
      this.token = paired.token;
      this.binding = JSON.stringify([this.service.origin, identity.workspace_id ?? '', paired.session.id,...(this.fileBinding?['browser-files-v1',this.fileBinding.actorId,this.fileBinding.runId,this.fileBinding.inputId]:[])]);
      this.journal.discardPreviousSessionResults(this.binding);
      if (this.stopping) throw Error('stopped');
      if(this.fileBinding&&this.fileStorageReady)await this.driver.bindFileTask(this.directory,this.fileBinding,async()=>{
        if(this.stopping||await this.storageCheck.prepare()!==true)throw Error('computer_storage_unavailable');
      });
      if (this.stopping) throw Error('stopped');
      // Retained browser state is not retained authority. Activate only after
      // this new session's identity, scope and pairing have succeeded.
      if(this.driver instanceof BrowserWorkspace&&this.driver.control==='manual')await this.driver.resumeManual();
      if (this.stopping) throw Error('stopped');
      this.publish('ready');
      this.running = this.loop();
      return {id:paired.session.id, origin:paired.session.origin};
    } catch (error) {
      await this.stop();
      throw error;
    }
  }
  async loop() {
    try {
      while (!this.stopping&&!this.handingOver) {
        const {action} = await this.companion('poll');
        if (this.stopping||this.handingOver) break;
        if (!action) { await new Promise(resolve => setTimeout(resolve, 300)); continue; }
        // Validate before touching dispatch storage: foreign messages must not
        // poison an owning task's tombstone or authorize even a preparation read.
        const fileAction=this.fileBinding?browserFileAction(action,this.fileBinding):undefined;
        if(!this.fileBinding&&Object.hasOwn(action,'file_binding'))throw Error('computer_browser_file_scope_invalid');
        const journal = this.journal;
        const dispatch = journal.prepare(action, this.binding);
        if (!dispatch.execute) {
          await this.companion('reply', dispatch.reply);
          continue;
        }
        let result;
        try {
          if(action.kind==='computer_prepare') {
            if(!action.arguments || Object.keys(action.arguments).sort().join(',')!=='arguments,tool')throw Error('action_conflict');
            result=await this.driver.prepare(action.arguments.tool,action.arguments.arguments);
            if(!validActionClass(result?.approval_class))throw Error('computer_protocol_invalid');
          } else {
            const prepared=await this.driver.prepare(action.kind,action.arguments);
            if(!validActionClass(prepared?.approval_class))throw Error('computer_protocol_invalid');
            if(prepared.approval_class==='forbidden')throw Error('computer_prohibited_action');
            if(!['auto','exact'].includes(action.authorization) || action.authorization==='auto'&&!policyAllows(this.approvalMode,prepared.approval_class))throw Error('computer_approval_invalid');
            result = await this.driver.execute(action.kind,action.arguments,fileAction);
          }
          if (action.kind === 'browser_capture') {
            const stored = await this.companion('capture', {id:action.id,observation_id:result.observation_id,media_type:result.media_type,width:result.width,height:result.height,data:result.data});
            result = {captured:true,image_ref:stored.image_ref,observation_id:result.observation_id,width:result.width,height:result.height};
          }
        }
        catch(error) {
          if (safeBrowserFailure(error) && !this.stopping) {
            const reply = {id:action.id,result:'',error:error.code};
            journal.complete(action.id,reply);
            await this.companion('reply',reply);
            continue;
          }
          const code=['computer_prohibited_action','computer_approval_invalid'].includes(error.message)?error.message:'computer_uncertain_outcome';
          if (!this.stopping) await this.companion('reply',{id:action.id,result:'',error:code});
          throw Error(code==='computer_uncertain_outcome'?'action_uncertain':code);
        }
        if (this.stopping) break;
        const reply = {id:action.id,result:JSON.stringify(result)};
        journal.complete(action.id, reply);
        await this.companion('reply', reply);
      }
    } catch (error) {
      if (!this.stopping) {
        const code=/^(?:action_|computer_)/.test(error.message)?error.message:'connection_lost';
        if(this.handingOver){
          if(!(error.name==='AbortError'&&this.pollAbort.signal.aborted&&!this.abort.signal.aborted))this.handoffError=code;
        }else this.publish('error',code);
      }
    } finally {
      if(this.handingOver&&!this.stopping){
        try{await this.retainManual();}
        catch{this.publish('error','computer_takeover_unconfirmed');await this.stop().catch(()=>{});}
      }else await this.stop().catch(()=>{});
    }
  }
  takeover() {
    if(this.state==='manual'&&!this.stopping&&this.retained)return Promise.resolve({state:'manual'});
    if(this.handoff)return this.handoff;
    if(!['ready','taking_over'].includes(this.state)||this.stopping||!(this.driver instanceof BrowserWorkspace))return Promise.reject(Error('computer_takeover_unavailable'));
    this.handingOver=true;this.publish('taking_over');
    const drain=this.driver.takeover();
    // Cancel only an idle poll. Result acknowledgement for admitted input gets
    // its existing bounded opportunity to finish before retiring old authority.
    this.pollAbort.abort();
    const pending=(async()=>{
      await drain;await this.running;
      if(this.state!=='manual'||!this.retained)throw Error('computer_takeover_unconfirmed');
      return {state:'manual'};
    })();
    this.handoff=pending.catch(error=>{this.handoff=undefined;throw error;});
    return this.handoff;
  }
  detachWorkspace(driver) {
    if(!(driver instanceof BrowserWorkspace))return;
    if(driver.onEmpty===this.emptyHandler)driver.onEmpty=undefined;
    if(this.disconnectHandler)driver.browser.removeListener('disconnected',this.disconnectHandler);
  }
  async retainManual() {
    const driver=this.driver;
    await driver.takeover();
    if(this.stopping||this.driver!==driver)return;
    this.detachWorkspace(driver);
    this.retained={driver,demo:this.demo};this.driver=null;this.demo=null;
    this.emptyHandler=()=>{if(this.retained?.driver===driver)void this.stop().catch(()=>{});};
    this.disconnectHandler=this.emptyHandler;
    driver.onEmpty=this.emptyHandler;driver.browser.once('disconnected',this.disconnectHandler);
    this.abort.abort();this.token='';
    const journal=this.journal;this.journal=null;
    if(journal){try{if(this.binding)journal.forgetResults(this.binding);}finally{journal.close();}}
    this.publish('manual',this.handoffError);
  }
  takeRetained() {
    if(this.state!=='manual'||this.stopping||!this.retained)throw Error('computer_takeover_unavailable');
    const retained=this.retained;this.detachWorkspace(retained.driver);this.retained=undefined;this.state='released';return retained;
  }
  async stop() {
    this.stopping = true; this.abort.abort();
    this.pollAbort.abort();
    if(this.closePending)return this.closing;
    const driver = this.driver??this.retained?.driver, demo = this.demo??this.retained?.demo, journal = this.journal, binding = this.binding;
    this.detachWorkspace(driver);
    this.journal = null; this.token = '';this.closePending=true;
    this.closing = (async () => {
      const results=await Promise.allSettled([
        (async()=>{
          await driver?.close();
          if(driver?.browser?.isConnected())throw Error('computer_stop_unconfirmed');
          if(this.driver===driver)this.driver=null;
          if(this.retained&&this.retained.driver===driver)this.retained.driver=undefined;
        })(),
        (async()=>{
          await demo?.close();
          if(this.demo===demo)this.demo=null;
          if(this.retained&&this.retained.demo===demo)this.retained.demo=undefined;
        })(),
        (async()=>{if(journal){try{if(binding)journal.forgetResults(binding);}finally{journal.close();}}})(),
        this.storageCheck?.stop(),
      ]);
      if(results.some(result=>result.status==='rejected')){
        this.stopFailed=true;
        this.publish('error','computer_stop_unconfirmed');throw Error('computer_stop_unconfirmed');
      }
      this.retained=undefined;
      if (this.state !== 'error'||this.stopFailed) this.publish('stopped');
      this.stopFailed=false;
    })().finally(()=>{this.closePending=false;});
    return this.closing;
  }
}
