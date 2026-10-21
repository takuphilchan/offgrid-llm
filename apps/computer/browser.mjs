import { isIP } from 'node:net';
import { randomUUID, createHash } from 'node:crypto';
import { createReadStream } from 'node:fs';
import { mkdir, stat } from 'node:fs/promises';
import { basename, join } from 'node:path';
import { chromium } from 'playwright';
import {publicPage, pinnedDestination, startEgress} from './network.mjs';
import {classifyBrowserAction} from './approval-policy.mjs';
export {publicIPv4} from './network.mjs';

// Validate before touching the page. The service repeats this check before
// approval; the companion must not trust the model or transport to do it.
export function validateBrowserAction(kind, args) {
  const schemas = {
    browser_observe: {}, browser_navigate: {url:'string'}, browser_verify: {text:'string'},
    browser_capture: {observation_id:'string'},
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
    const limit = key === 'text' ? (kind === 'browser_verify' ? 1000 : 4000) : key === 'url' ? 2048 : 128;
    if (Buffer.byteLength(value, 'utf8') > limit || (key !== 'text' && !value.trim()) ||
        (kind === 'browser_verify' && !value.trim())) throw Error('Browser action argument exceeds scope.');
  }
}

// Runs in the isolated browser. Use the same accessible-name checks when
// observing and immediately before input; labels are not security-neutral.
function describeControl(el) {
  const labelledBy = (el.getAttribute('aria-labelledby') || '').split(/\s+/).map(id => document.getElementById(id)?.textContent || '').join(' ');
  const labels = Array.from(el.labels || []).map(label => label.innerText).join(' ');
  const names = [el.type, el.name, el.id, el.autocomplete, el.getAttribute('aria-label'), labelledBy, labels, el.getAttribute('placeholder')].join(' ');
  return { tag: el.tagName.toLowerCase(), type: el.getAttribute('type') || '',
    label: (el.getAttribute('aria-label') || labelledBy.trim() || labels.trim() || el.getAttribute('placeholder') || el.innerText || el.getAttribute('name') || '').slice(0, 160),
    sensitive: /password|passcode|secret|credential|token|username|one[- ]time[- ]code|verification[- ]code|credit[- ]card|card[- ]number|cc-number|cc-csc|cvv|cvc/i.test(names) };
}

export class BrowserDriver {
  static async open(origin, { headless = false, testLoopback = false, networkMode = 'direct', downloadDir, upload } = {}) {
    const url = new URL(origin);
    if (url.username || url.password ||
       (!testLoopback && (url.protocol !== 'https:' || isIP(url.hostname)))) throw Error('Select a public HTTPS origin.');
    if (testLoopback && (url.hostname !== '127.0.0.1' || url.protocol !== 'http:')) throw Error('Test fixture must be IPv4 loopback.');
    if (!['direct','trusted-vpn'].includes(networkMode) || (testLoopback && networkMode !== 'direct')) throw Error('network_mode_invalid');
    if (!testLoopback) publicPage(origin);
    const egress = testLoopback ? null : await startEgress(await pinnedDestination(origin,networkMode));
    let browser;
    try {
      browser = await chromium.launch({ headless, ...(egress ? {proxy:{server:egress.server,bypass:'<-loopback>'}} : {}), args: [
        '--disable-quic', '--force-webrtc-ip-handling-policy=disable_non_proxied_udp'
      ] });
      const context = await browser.newContext({ acceptDownloads: true, serviceWorkers: 'block' });
      const blockedOrigins = new Set();
      await context.route('**/*', route => {
        let allowed = false;
        try {
          const destination = new URL(route.request().url());
          allowed = destination.origin === url.origin && !destination.username && !destination.password;
          if (!allowed && blockedOrigins.size < 20) blockedOrigins.add(destination.origin);
        } catch { /* deny */ }
        return allowed ? route.continue() : route.abort('blockedbyclient');
      });
      await context.routeWebSocket('**/*', socket => socket.close());
      await context.addInitScript(() => {
        window.__offgridRevision = 0;
        new MutationObserver(() => window.__offgridRevision++).observe(document, { childList: true, subtree: true, characterData: true, attributes: true });
        for (const type of ['input', 'change']) document.addEventListener(type, () => window.__offgridRevision++, true);
      });
      const page = await context.newPage();
      page.setDefaultTimeout(8000);
      page.on('dialog', dialog => void dialog.dismiss());
      context.on('page', popup => { if (popup !== page) void popup.close(); });
      const driver = new BrowserDriver(browser, page, url.origin, {downloadDir,upload});
      driver.egress = egress;
      driver.blockedOrigins = blockedOrigins;
      try { await page.goto(url.href, { waitUntil: 'domcontentloaded', timeout: 20000 }); }
      catch { throw Error('network_unavailable'); }
      return driver;
    } catch (error) { try { await browser?.close(); } finally { await egress?.close(); } throw error; }
  }
  constructor(browser, page, origin, {downloadDir,upload} = {}) { Object.assign(this, { browser, page, origin, downloadDir, upload, elements: new Map(), observation: '', revision: -1 }); }
  async close() { try { await this.browser.close(); } finally { await this.egress?.close(); } }
  async stateDigest() {
    // Property writes need not emit DOM mutations or input events. Keep only a
    // digest locally; field values never enter observations, events or logs.
    const state = await this.page.evaluate(() => JSON.stringify(Array.from(document.querySelectorAll('input, textarea, select')).map(el => [el.value, el.checked, el.selectedIndex, el.disabled, el.readOnly])));
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
  async observeOnce() {
    if (this.page.isClosed() || new URL(this.page.url()).origin !== this.origin) throw Error('Target unavailable or outside scope.');
    this.elements.clear(); this.observation = randomUUID();
    const initialRevision = await this.page.evaluate(() => window.__offgridRevision);
    const initialState = await this.stateDigest();
    const controls = await this.page.locator('button, input:not([type="hidden"]), textarea, select, a[href], [role="button"]').elementHandles();
    const elements = [];
    for (const handle of controls.slice(0, 400)) {
      if (elements.length >= 80) break;
      if (!await handle.isVisible()) continue;
      const info = await handle.evaluate(describeControl);
      if (info.sensitive) continue;
      const id = String(elements.length + 1);
      const state = await handle.evaluate(el => ({disabled:el.matches(':disabled') || el.getAttribute('aria-disabled') === 'true',
        ...(el.tagName === 'SELECT' ? {multiple:el.multiple, options:Array.from(el.options).slice(0,100).map((option,index) => ({id:String(index+1),label:option.label.slice(0,160),disabled:option.disabled || (option.parentElement.tagName === 'OPTGROUP' && option.parentElement.disabled),selected:option.selected}))} : {}),
        ...(el.tagName === 'INPUT' && el.type === 'checkbox' ? {checked:el.checked} : {})}));
      let href;
      if (info.tag === 'a') {
        const candidate = await handle.getAttribute('href');
        try { const link = new URL(candidate, this.page.url()); if (link.origin === this.origin && !link.username && !link.password && link.href.length <= 2048) href = link.href; } catch { /* do not expose unsafe destinations */ }
      }
      this.elements.set(id, handle); elements.push({ id, tag: info.tag, type: info.type, label: info.label, ...(href ? {href} : {}), ...state });
    }
    const title = await this.page.title();
    const text = (await this.page.locator('body').innerText()).slice(0,10000);
    this.revision = await this.page.evaluate(() => window.__offgridRevision);
    this.observedURL = this.page.url(); this.observedAt = Date.now();
    this.formDigest = await this.stateDigest();
    if (this.revision !== initialRevision || this.formDigest !== initialState) {
      this.observation = '';
      throw Error('observation_changed');
    }
    // No field values, screenshots, cookies or password content are collected.
    return { observation_id: this.observation, url: this.observedURL, title,
      text, elements, blocked_origins: Array.from(this.blockedOrigins ?? []),
      controls_limited: controls.length > 400 || elements.length >= 80 };
  }
  async execute(kind, args) {
    validateBrowserAction(kind, args);
    if (this.page.isClosed() || new URL(this.page.url()).origin !== this.origin) throw Error('Target unavailable or outside scope.');
    if (kind === 'browser_observe') return this.observe();
    if (kind === 'browser_navigate') {
      const url = new URL(args.url);
      if (url.origin !== this.origin || url.username || url.password) throw Error('Navigation is outside approved origin.');
      this.observation = '';
      await this.page.goto(url.href, { waitUntil: 'domcontentloaded', timeout: 15000 });
      return this.observe();
    }
    if (kind === 'browser_verify') {
      if (typeof args.text !== 'string' || !args.text.trim() || args.text.length > 1000) throw Error('Expected text is required.');
      const verified = (await this.page.locator('body').innerText()).includes(args.text);
      return { verified, check: 'page_contains_text', text: args.text, url: this.page.url() };
    }
    if (!['browser_capture','browser_click','browser_download','browser_upload','browser_fill','browser_select','browser_set_checked'].includes(kind)) throw Error('Unsupported browser action.');
    if (args.observation_id !== this.observation || Date.now() - this.observedAt > 120000 ||
        this.page.url() !== this.observedURL || await this.page.evaluate(() => window.__offgridRevision) !== this.revision ||
        await this.stateDigest() !== this.formDigest) throw Error('Stale observation; inspect the page again.');
    if (kind === 'browser_capture') {
      const viewport = this.page.viewportSize();
      if (!viewport || viewport.width < 1 || viewport.height < 1 || viewport.width > 8192 || viewport.height > 8192 || viewport.width * viewport.height > 32 * 1024 * 1024) throw Error('Browser surface exceeds the vision safety limit.');
      const mask = this.page.locator('input[type="password"], input[autocomplete="current-password"], input[autocomplete="new-password"], input[autocomplete="one-time-code"], input[autocomplete="cc-number"], input[autocomplete="cc-csc"], [data-offgrid-private]');
      const content = await this.page.screenshot({type:'png',animations:'disabled',caret:'hide',mask:[mask],maskColor:'#000'});
      if (!content.length || content.length > 5 * 1024 * 1024) throw Error('Browser capture exceeds the vision safety limit.');
      return {captured:true,observation_id:this.observation,media_type:'image/png',width:viewport.width,height:viewport.height,data:content.toString('base64')};
    }
    const handle = this.elements.get(args.element);
    if (!handle || !await handle.isVisible()) throw Error('Unknown or hidden element.');
    if ((await handle.evaluate(describeControl)).sensitive) throw Error('Credential/payment entry requires manual takeover.');
    if (!await handle.isEnabled()) throw Error('Disabled control cannot be changed.');
    this.observation = '';
    if (kind === 'browser_select') {
      const index = Number(args.option) - 1;
      if (!/^(?:[1-9][0-9]?|100)$/.test(args.option) || !await handle.evaluate((el,index) =>
        el.tagName === 'SELECT' && !el.multiple && !!el.options[index] && !el.options[index].disabled &&
        !(el.options[index].parentElement.tagName === 'OPTGROUP' && el.options[index].parentElement.disabled), index)) throw Error('Select one enabled option ID from the observation.');
      await handle.selectOption({index});
      if (!await handle.evaluate((el,index) => el.selectedIndex === index, index)) throw Error('Selection could not be verified.');
      return {...await this.observe(), verified:true, check:'selected_option_matches', changed:true};
    }
    if (kind === 'browser_set_checked') {
      if (!await handle.evaluate(el => el.tagName === 'INPUT' && el.type === 'checkbox')) throw Error('Only a native checkbox is supported.');
      await handle.setChecked(args.checked);
      if (await handle.isChecked() !== args.checked) throw Error('Checkbox change could not be verified.');
      return {...await this.observe(), verified:true, check:'checked_state_matches', changed:true};
    }
    if (kind === 'browser_download') {
      if (!this.downloadDir) throw Error('Download staging is unavailable.');
      await mkdir(this.downloadDir, {recursive:true, mode:0o700});
      const suggested = await handle.getAttribute('download') || await handle.getAttribute('aria-label') || 'download';
      const safeName = basename(String(suggested)).replace(/[^\p{L}\p{N}._ -]/gu, '_').slice(0, 160) || 'download';
      const downloadPromise = this.page.waitForEvent('download', {timeout: 15000});
      await handle.click();
      const download = await downloadPromise;
      if (await download.failure()) throw Error('Download failed.');
      const target = join(this.downloadDir, `${Date.now()}-${safeName}`);
      await download.saveAs(target);
      const file = await stat(target);
      if (!file.isFile() || file.size > 512 * 1024 * 1024) throw Error('Downloaded artifact is invalid.');
      const sha256 = await fileDigest(target);
      return {downloaded:true, verified:true, artifact:{name:safeName,size:file.size,sha256,staged:true}};
    }
    if (kind === 'browser_upload') {
      if (!this.upload || !/^[a-f0-9]{32}$/.test(this.upload.id) || !/^[a-f0-9]{64}$/.test(this.upload.sha256)) throw Error('No user-selected upload is available.');
      if (!await handle.evaluate(el=>el.tagName==='INPUT' && el.type==='file' && !el.multiple)) throw Error('Only one native file input is supported.');
      const current=await stat(this.upload.path);
      if (!current.isFile() || current.size!==this.upload.size || await fileDigest(this.upload.path)!==this.upload.sha256) throw Error('Selected upload changed; select it again.');
      await handle.setInputFiles(this.upload.path);
      const verified=await handle.evaluate((el,name)=>el.files?.length===1 && el.files[0].name===name,this.upload.name);
      if(!verified)throw Error('Upload selection could not be verified.');
      return {uploaded:true,verified:true,file:{grant:this.upload.id,name:this.upload.name,size:this.upload.size,sha256:this.upload.sha256}};
    }
    if (kind === 'browser_fill') {
      if (typeof args.text !== 'string' || args.text.length > 4000) throw Error('Text exceeds scope.');
      if ((await handle.evaluate(describeControl)).sensitive) throw Error('Credential/payment entry requires manual takeover.');
      await handle.fill(args.text);
      if (await handle.inputValue() !== args.text) throw Error('Field change could not be verified.');
      // Return fresh, scoped state after the mutation. Never make the model
      // guess whether old element IDs/observations can be reused.
      return { ...await this.observe(), verified: true, check: 'field_value_matches', changed: true };
    }
    await handle.click();
    return { ...await this.observe(), dispatched: true, verified: false, message: 'Click dispatched. This is a fresh observation; independently verify the requested result.' };
  }

  async prepare(kind, args) {
    validateBrowserAction(kind, args);
    if (this.page.isClosed() || new URL(this.page.url()).origin !== this.origin) throw Error('Target unavailable or outside scope.');
    if (['browser_observe','browser_capture','browser_verify'].includes(kind)) return {prepared:true,approval_class:'read_only'};
    if (kind === 'browser_navigate') {
      const url = new URL(args.url);
      if (url.origin !== this.origin || url.username || url.password) throw Error('Navigation is outside approved origin.');
      return {prepared:true,approval_class:'reversible'};
    }
    if (args.observation_id !== this.observation || Date.now() - this.observedAt > 120000 || this.page.url() !== this.observedURL ||
        await this.page.evaluate(() => window.__offgridRevision) !== this.revision || await this.stateDigest() !== this.formDigest) throw Error('Stale observation; inspect the page again.');
    const handle = this.elements.get(args.element);
    if (!handle || !await handle.isVisible() || !await handle.isEnabled()) throw Error('Unknown, hidden, or disabled element.');
    const control = await handle.evaluate(describeControl);
    control.riskText = await handle.evaluate(el => `${document.title} ${el.closest('form')?.innerText?.slice(0,1000) ?? ''}`);
    const approvalClass = classifyBrowserAction(kind, control);
    return {prepared:true,approval_class:approvalClass};
  }
}

function fileDigest(file) {
  return new Promise((resolve, reject) => {
    const hash=createHash('sha256'); const stream=createReadStream(file);
    stream.on('data', chunk=>hash.update(chunk)); stream.once('error', reject); stream.once('end', ()=>resolve(hash.digest('hex')));
  });
}
