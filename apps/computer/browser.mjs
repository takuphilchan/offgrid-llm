import { isIP } from 'node:net';
import { randomUUID, createHash } from 'node:crypto';
import { createReadStream } from 'node:fs';
import { mkdir, stat } from 'node:fs/promises';
import { basename, join } from 'node:path';
import { chromium } from 'playwright';
import {publicPage, pinnedDestination, startEgress} from './network.mjs';
import {classifyBrowserAction} from './approval-policy.mjs';
import {BrowserFailure, browserRead} from './browser-outcome.mjs';
import {assertFresh, observePage} from './browser-observation.mjs';
import {browserOrigins, allowsBrowserURL} from './browser-scope.mjs';
import {describeControl, documentState, installDocumentRevision} from './browser-dom.mjs';
import {BrowserFrames} from './browser-frames.mjs';
import {documentText} from './browser-text.mjs';
import {captureBrowser} from './browser-capture.mjs';
import {BrowserUploadStore} from './browser-file-snapshot.mjs';
import {BrowserOwnedUploadStore} from './browser-owned-upload-store.mjs';
import {registerFileReadback, verifySelectedBrowserFile} from './browser-file-readback.mjs';
export {publicIPv4} from './network.mjs';

// Validate before touching the page. The service repeats this check before
// approval; the companion must not trust the model or transport to do it.
export function validateBrowserAction(kind, args) {
  const schemas = {
    browser_observe: {}, browser_navigate: {url:'string'}, browser_verify: {text:'string'},
    browser_capture: {observation_id:'string'},
    browser_inspect: {observation_id:'string',cursor:'string'},
    browser_search: {role:'string',name:'string',text:'string'},
    browser_frames: {}, browser_frame_select:{frame_id:'string',document_id:'string'},
    browser_click: {observation_id:'string',element:'string'},
    browser_download: {observation_id:'string',element:'string'},
    browser_upload: {observation_id:'string',element:'string'},
    browser_fill: {observation_id:'string',element:'string',text:'string'},
    browser_select: {observation_id:'string',element:'string',option:'string'},
    browser_set_checked: {observation_id:'string',element:'string',checked:'boolean'},
  };
  const schema = schemas[kind];
  if (!schema) throw Error('Unsupported browser action.');
  if (!args || typeof args !== 'object' || Array.isArray(args) ||
      Object.keys(args).length !== Object.keys(schema).length ||
      Object.entries(schema).some(([key,type]) => typeof args[key] !== type)) throw Error('Invalid browser action arguments.');
  for (const [key,value] of Object.entries(args)) {
    if (typeof value !== 'string') continue;
    const limit = key === 'text' ? (kind === 'browser_search' ? 256 : kind === 'browser_verify' ? 1000 : 4000) : key === 'name' ? 256 : key === 'url' ? 2048 : 128;
    if (Buffer.byteLength(value, 'utf8') > limit || (kind!=='browser_search'&&key !== 'text' && !value.trim()) ||
        (kind === 'browser_verify' && !value.trim())) throw Error('Browser action argument exceeds scope.');
  }
  if(kind==='browser_search'&&(![args.role,args.name,args.text].some(value=>value.trim())||!['','button','link','textbox','checkbox','radio','switch','combobox','option','tab','menuitem'].includes(args.role)))throw Error('Use a supported role or a literal name/text query.');
}

export class BrowserDriver {
  static async open(origin, { headless = false, testLoopback = false, networkMode = 'direct', downloadDir, upload, additionalOrigins = [], managedTabs = false } = {}) {
    const url = new URL(origin);
    if (url.username || url.password ||
       (!testLoopback && (url.protocol !== 'https:' || isIP(url.hostname)))) throw Error('Select a public HTTPS origin.');
    if (testLoopback && (url.hostname !== '127.0.0.1' || url.protocol !== 'http:')) throw Error('Test fixture must be IPv4 loopback.');
    if (!['direct','trusted-vpn'].includes(networkMode) || (testLoopback && networkMode !== 'direct')) throw Error('network_mode_invalid');
    if (!testLoopback) publicPage(origin);
    const allowedOrigins = browserOrigins(origin, additionalOrigins, testLoopback);
    const egress = testLoopback ? null : await startEgress(await Promise.all(allowedOrigins.map(site => pinnedDestination(site,networkMode))));
    let browser;
    try {
      await registerFileReadback();
      browser = await chromium.launch({ headless, ...(egress ? {proxy:{server:egress.server,bypass:'<-loopback>'}} : {}), args: [
        '--disable-quic', '--force-webrtc-ip-handling-policy=disable_non_proxied_udp'
      ] });
      const context = await browser.newContext({ acceptDownloads: true, serviceWorkers: 'block' });
      const blockedOrigins = new Set();
      await context.route('**/*', route => {
        let allowed = false;
        try {
          const destination = new URL(route.request().url());
          allowed = allowsBrowserURL(allowedOrigins,destination.href);
          if (!allowed && blockedOrigins.size < 20) blockedOrigins.add(destination.origin);
        } catch { /* deny */ }
        return allowed ? route.continue() : route.abort('blockedbyclient');
      });
      await context.routeWebSocket('**/*', socket => socket.close());
      await context.addInitScript(installDocumentRevision);
      const page = await context.newPage();
      page.setDefaultTimeout(8000);
      page.on('dialog', dialog => void dialog.dismiss());
      const driver = new BrowserDriver(browser, page, url.origin, {downloadDir,upload,allowedOrigins});
      context.on('page', popup => {
        if (popup === page) return;
        if (managedTabs && driver.registerTab) driver.registerTab(popup);
        else void popup.close().catch(()=>{});
      });
      driver.egress = egress;
      driver.blockedOrigins = blockedOrigins;
      try { await page.goto(url.href, { waitUntil: 'domcontentloaded', timeout: 20000 }); }
      catch { throw Error('network_unavailable'); }
      return driver;
    } catch (error) { try { await browser?.close(); } finally { await egress?.close(); } throw error; }
  }
  constructor(browser, page, origin, {downloadDir,upload,uploadStore,allowedOrigins=[origin]} = {}) {
    Object.assign(this, { browser, page, origin, downloadDir, upload, allowedOrigins, elements: new Map(), observation: '', revision: -1, documentID:randomUUID() });
    this.uploadStore=uploadStore??new BrowserUploadStore();
    this.frames=new BrowserFrames(this);
    page.on('framenavigated', frame => {if(frame===page.mainFrame()){this.documentID=randomUUID();this.observation='';}});
  }
  allows(value) {return allowsBrowserURL(this.allowedOrigins,value);}
  context() {return this.frames.current();}
  disableFileTransfers(){this.fileTransfersDisabled=true;this.upload=undefined;}
  bindFileTask(directory,binding,checkStorage) {
    if(this.stopping||this.fileBindingPending)return Promise.reject(Error('upload_store_unavailable'));
    const pending=(async()=>{
      if(this.uploadStore instanceof BrowserOwnedUploadStore)this.uploadStore.rebind(binding);
      else {
        // A legacy selected File may still refer to its snapshot. Never replace
        // that owner or infer stronger authority from a new service capability.
        if(this.uploadStore.snapshot||this.uploadStore.verifier)throw Error('upload_store_scope_invalid');
        this.uploadStore=await BrowserOwnedUploadStore.open({directory,binding});
      }
      this.checkFileStorage=checkStorage;this.fileTransfersDisabled=false;
    })();
    this.fileBindingPending=pending;
    return pending.finally(()=>{if(this.fileBindingPending===pending)this.fileBindingPending=null;});
  }
  async close() {
    this.stopping=true;
    this.uploadStore?.stopPreparing();
    try {
      // Stop can race post-pairing owner creation. Keep the resulting store
      // attached to this browser until both process closure and cleanup settle.
      await this.fileBindingPending?.catch(()=>{});
      this.uploadStore?.stopPreparing();
      await this.browser.close();
      if(this.browser.isConnected())throw Error('computer_stop_unconfirmed');
      if(!this.uploadStore?.closed)await this.uploadStore?.browserClosed?.();
      await this.uploadStore?.close();
    }
    finally { await this.egress?.close(); }
  }
  async stateDigest() {
    // Property writes need not emit DOM mutations or input events. Keep only a
    // digest locally; field values never enter observations, events or logs.
    const state = await this.context().evaluate(documentState);
    return createHash('sha256').update(state).digest('hex');
  }
  async observe() {
    // Hydration/lazy rendering on real sites can invalidate a read. Retry only
    // the observation, never a dispatched click/edit or external submission.
    for (let attempt = 0; attempt < 3; attempt++) {
      try { return await this.observeOnce(); }
      catch (error) {
        if (error.message !== 'observation_changed' || attempt === 2) throw error;
        await new Promise(resolve => setTimeout(resolve, 150));
      }
    }
  }
  async observeOnce() { return observePage(this,describeControl); }
  async execute(kind, args, fileAction) {
    // Scope loss is terminal even for reads. Input-started is local to this
    // execution, so no subsequent read can erase an uncertain effect.
    if (this.page.isClosed() || !this.allows(this.page.url())) throw Error('Target unavailable or outside scope.');
    let inputStarted = false;
    try {
      return await this.executeAction(kind, args, () => { inputStarted = true; }, fileAction);
    } catch (error) {
      if (inputStarted||error.message==='computer_stop_unconfirmed') throw error;
      throw new BrowserFailure(browserRead(kind) ? 'computer_browser_observation_failed' : 'computer_browser_precondition_failed', error.message);
    }
  }
  async withObservation(receipt) {
    try { return {...await this.observe(), ...receipt, observation_status:'available'}; }
    catch {
      this.observation = '';
      return {...receipt, observation_status:'unavailable', message:'Input completed. Inspect the page to establish its outcome; do not repeat the input because its observation is unavailable.'};
    }
  }
  async afterEdit(check, readback) {
    let verified=false, verification_status='unavailable';
    try {verified=await readback();verification_status=verified?'verified':'mismatch';}catch{/* input already returned; keep its receipt */}
    return this.withObservation({effect_status:verified?'completed':'dispatched',dispatched:true,verified,verification_status,check,changed:true});
  }
  async executeAction(kind, args, beginInput, fileAction) {
    validateBrowserAction(kind, args);
    if(['browser_upload','browser_download'].includes(kind)&&this.fileTransfersDisabled)throw Error('Protected file transfers are unavailable.');
    if (this.page.isClosed() || !this.allows(this.page.url())) throw Error('Target unavailable or outside scope.');
    if(kind==='browser_frames')return this.frames.list();
    if(kind==='browser_frame_select')return this.frames.select(args);
    if(kind!=='browser_navigate')await this.frames.assertScope();
    if (kind === 'browser_observe') return this.observe();
    if (kind === 'browser_inspect') return observePage(this,describeControl,args);
    if (kind === 'browser_search') return observePage(this,describeControl,undefined,args);
    if (kind === 'browser_navigate') {
      const url = new URL(args.url);
      if (!this.allows(url.href)) throw Error('Navigation is outside approved origins.');
      this.observation = '';
      this.frames.selectMain();
      beginInput();
      await this.page.goto(url.href, { waitUntil: 'domcontentloaded', timeout: 15000 });
      return this.withObservation({dispatched:true,effect_status:'completed',verified:false});
    }
    if (kind === 'browser_verify') {
      if (typeof args.text !== 'string' || !args.text.trim() || args.text.length > 1000) throw Error('Expected text is required.');
      const verified = (await this.context().evaluate(documentText)).includes(args.text);
      return { verified, check: 'page_contains_text', text: args.text, url: this.context().url(),...this.frames.provenance() };
    }
    if (!['browser_capture','browser_click','browser_download','browser_upload','browser_fill','browser_select','browser_set_checked'].includes(kind)) throw Error('Unsupported browser action.');
    await assertFresh(this,args,kind==='browser_capture');
    if (kind === 'browser_capture') {
      return captureBrowser(this);
    }
    const handle = this.elements.get(args.element);
    if (!handle || !await handle.isVisible()) throw Error('Unknown or hidden element.');
    if ((await handle.evaluate(describeControl)).sensitive) throw Error('Credential/payment entry requires manual takeover.');
    if (!await handle.isEnabled()) throw Error('Disabled control cannot be changed.');
    if(kind!=='browser_upload')this.observation = '';
    if (kind === 'browser_select') {
      const index = Number(args.option) - 1;
      if (!/^[1-9][0-9]{0,8}$/.test(args.option) || !this.observedOptions?.get(args.element)?.has(args.option) || !await handle.evaluate((el,index) =>
        el.tagName === 'SELECT' && !el.multiple && !!el.options[index] && !el.options[index].disabled &&
        !(el.options[index].parentElement.tagName === 'OPTGROUP' && el.options[index].parentElement.disabled), index)) throw Error('Select one enabled option ID from the observation.');
      beginInput();
      await handle.selectOption({index});
      return this.afterEdit('selected_option_matches',()=>handle.evaluate((el,index)=>el.selectedIndex===index,index));
    }
    if (kind === 'browser_set_checked') {
      const checked=await handle.evaluate(el=>el.tagName==='INPUT'&&el.type==='checkbox'?{native:true,value:el.checked}:['checkbox','switch'].includes(el.getAttribute('role'))&&['true','false'].includes(el.getAttribute('aria-checked'))?{native:false,value:el.getAttribute('aria-checked')==='true'}:null);
      if(!checked)throw Error('Use a native checkbox or a custom checkbox/switch with explicit boolean checked state.');
      if(checked.value===args.checked)return this.withObservation({effect_status:'completed',dispatched:false,verified:true,verification_status:'verified',check:'checked_state_matches',changed:false});
      beginInput();
      if(checked.native)await handle.setChecked(args.checked);else await handle.click();
      return this.afterEdit('checked_state_matches',()=>handle.evaluate((el,expected)=>el.isConnected&&(el.tagName==='INPUT'&&el.type==='checkbox'?el.checked===expected:el.getAttribute('aria-checked')===String(expected)),args.checked));
    }
    if (kind === 'browser_download') {
      await this.checkFileStorage?.();
      if (!this.downloadDir) throw Error('Download staging is unavailable.');
      await mkdir(this.downloadDir, {recursive:true, mode:0o700});
      const suggested = await handle.getAttribute('download') || await handle.getAttribute('aria-label') || 'download';
      const safeName = basename(String(suggested)).replace(/[^\p{L}\p{N}._ -]/gu, '_').slice(0, 160) || 'download';
      // Observe both promises immediately: either the click or the download
      // wait can fail first. A late rejection must not crash the companion.
      // Dispatch once; failure still leaves the action for reconciliation.
      beginInput();
      const [download] = await Promise.all([
        this.page.waitForEvent('download', {timeout: 15000}),
        handle.click(),
      ]);
      if (await download.failure()) throw Error('Download failed.');
      const target = join(this.downloadDir, `${Date.now()}-${safeName}`);
      await download.saveAs(target);
      const file = await stat(target);
      if (!file.isFile() || file.size > 512 * 1024 * 1024) throw Error('Downloaded artifact is invalid.');
      const sha256 = await fileDigest(target);
      return {effect_status:'completed',dispatched:true,downloaded:true, verified:true, artifact:{name:safeName,size:file.size,sha256,staged:true}};
    }
    if (kind === 'browser_upload') {
      if (!this.upload || !/^[a-f0-9]{32}$/.test(this.upload.id) || !/^[a-f0-9]{64}$/.test(this.upload.sha256)) throw Error('No user-selected upload is available.');
      if (!await handle.evaluate(el=>el.tagName==='INPUT' && el.type==='file' && !el.multiple)) throw Error('Only one native file input is supported.');
      const grant=this.upload;
      await this.checkFileStorage?.();
      const snapshot=await this.uploadStore.get(grant,fileAction);
      await assertFresh(this,args);
      if(this.uploadStore instanceof BrowserOwnedUploadStore){
        await this.checkFileStorage?.();
        await this.uploadStore.inUse(snapshot,fileAction,this.frames.provenance().frame_document_id);
        await assertFresh(this,args);
      }
      if(this.stopping||snapshot.abort.signal.aborted||this.upload!==grant)throw Error('Upload grant is no longer available.');
      this.observation='';
      beginInput();
      await handle.setInputFiles(snapshot.path);
      const selected=await verifySelectedBrowserFile(handle,snapshot,{signal:snapshot.abort.signal,timeoutMs:120000});
      return {effect_status:'completed',dispatched:true,uploaded:true,verified:true,check:'file_input_matches_grant',verification_scope:'file_input_selection',
        file:{grant:grant.id,...selected},message:'Approved file selected. Remote receipt has not been verified.'};
    }
    if (kind === 'browser_fill') {
      if (typeof args.text !== 'string' || args.text.length > 4000) throw Error('Text exceeds scope.');
      if ((await handle.evaluate(describeControl)).sensitive) throw Error('Credential/payment entry requires manual takeover.');
      beginInput();
      await handle.fill(args.text);
      // Return fresh, scoped state after the mutation. Never make the model
      // guess whether old element IDs/observations can be reused.
      return this.afterEdit('field_value_matches',async()=>{
        if(!await handle.evaluate(el=>el.isConnected))return false;
        return await handle.evaluate(el=>el.isContentEditable)?await handle.innerText()===args.text:await handle.inputValue()===args.text;
      });
    }
    beginInput();
    await handle.click();
    return this.withObservation({effect_status:'dispatched',dispatched:true,verified:false,message:'Click dispatched. Independently verify the requested result.'});
  }

  async prepare(kind, args) {
    if (this.page.isClosed() || !this.allows(this.page.url())) throw Error('Target unavailable or outside scope.');
    try { return await this.prepareAction(kind,args); }
    catch(error) { throw new BrowserFailure('computer_browser_precondition_failed',error.message); }
  }
  async prepareAction(kind, args) {
    if(['browser_upload','browser_download'].includes(kind)&&this.fileTransfersDisabled)throw Error('Protected file transfers are unavailable.');
    validateBrowserAction(kind, args);
    if (this.page.isClosed() || !this.allows(this.page.url())) throw Error('Target unavailable or outside scope.');
    if (['browser_observe','browser_capture','browser_verify','browser_inspect','browser_search','browser_frames','browser_frame_select'].includes(kind)) return {prepared:true,approval_class:'read_only'};
    if (kind === 'browser_navigate') {
      const url = new URL(args.url);
      if (!this.allows(url.href)) throw Error('Navigation is outside approved origins.');
      return {prepared:true,approval_class:'reversible'};
    }
    await assertFresh(this,args);
    const handle = this.elements.get(args.element);
    if (!handle || !await handle.isVisible() || !await handle.isEnabled()) throw Error('Unknown, hidden, or disabled element.');
    const control = await handle.evaluate(describeControl);
    control.riskText = await handle.evaluate(el => `${document.title} ${el.closest('form')?.innerText?.slice(0,1000) ?? ''}`);
    let approvalClass = classifyBrowserAction(kind, control);
    // Custom checked-state actions dispatch a website click. Its semantic role
    // is useful for verification, not proof that the handler has no other effect.
    if(kind==='browser_set_checked'&&!(control.tag==='input'&&control.type==='checkbox')&&approvalClass!=='forbidden')approvalClass='consequential';
    return {prepared:true,approval_class:approvalClass};
  }
}

function fileDigest(file) {
  return new Promise((resolve, reject) => {
    const hash=createHash('sha256'); const stream=createReadStream(file);
    stream.on('data', chunk=>hash.update(chunk)); stream.once('error', reject); stream.once('end', ()=>resolve(hash.digest('hex')));
  });
}
