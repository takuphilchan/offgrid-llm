import { lookup } from 'node:dns/promises';
import { isIP } from 'node:net';
import { randomUUID, createHash } from 'node:crypto';
import { chromium } from 'playwright';

// Validate before touching the page. The service repeats this check before
// approval; the companion must not trust the model or transport to do it.
export function validateBrowserAction(kind, args) {
  const schemas = {
    browser_observe: {}, browser_navigate: {url:'string'}, browser_verify: {text:'string'},
    browser_click: {observation_id:'string',element:'string'},
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

export function publicIPv4(address) {
  const p = address.split('.').map(Number);
  return isIP(address) === 4 && ![0, 10, 127].includes(p[0]) && p[0] < 224 &&
    !(p[0] === 169 && p[1] === 254) && !(p[0] === 172 && p[1] >= 16 && p[1] <= 31) &&
    !(p[0] === 192 && (p[1] === 168 || p[1] === 0 || p[1] === 2)) &&
    !(p[0] === 100 && p[1] >= 64 && p[1] <= 127) && !(p[0] === 198 && [18,19,51].includes(p[1])) &&
    !(p[0] === 203 && p[1] === 0);
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
  static async open(origin, { headless = false, testLoopback = false } = {}) {
    const url = new URL(origin);
    if (url.username || url.password || url.search || url.hash || url.pathname !== '/' ||
       (!testLoopback && (url.protocol !== 'https:' || isIP(url.hostname)))) throw Error('Select a public HTTPS origin.');
    if (testLoopback && (url.hostname !== '127.0.0.1' || url.protocol !== 'http:')) throw Error('Test fixture must be IPv4 loopback.');
    const addresses = testLoopback ? [{ address: '127.0.0.1' }] : await lookup(url.hostname, { all: true, family: 4 });
    if (!addresses.length || (!testLoopback && addresses.some(a => !publicIPv4(a.address)))) throw Error(`Browser network preflight failed: ${url.hostname} resolves to a private or reserved address. VPN fake DNS can cause this. Keep required VPN access enabled; choose "demo" to test locally. No browser was paired and no code is needed. Arbitrary private-network access remains blocked.`);
    // Pin resolution for the entire session to prevent DNS rebinding into local services.
    const browser = await chromium.launch({ headless, args: [
      `--host-resolver-rules=MAP ${url.hostname} ${addresses[0].address},EXCLUDE localhost`,
      '--disable-quic', '--force-webrtc-ip-handling-policy=disable_non_proxied_udp'
    ] });
    try {
      const context = await browser.newContext({ acceptDownloads: false, serviceWorkers: 'block' });
      await context.route('**/*', route => {
        let allowed = false;
        try { allowed = new URL(route.request().url()).origin === url.origin; } catch { /* deny */ }
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
      page.on('download', download => void download.cancel());
      context.on('page', popup => { if (popup !== page) void popup.close(); });
      const driver = new BrowserDriver(browser, page, url.origin);
      await page.goto(url.origin, { waitUntil: 'domcontentloaded', timeout: 15000 });
      return driver;
    } catch (error) { await browser.close(); throw error; }
  }
  constructor(browser, page, origin) { Object.assign(this, { browser, page, origin, elements: new Map(), observation: '', revision: -1 }); }
  async close() { await this.browser.close(); }
  async stateDigest() {
    // Property writes need not emit DOM mutations or input events. Keep only a
    // digest locally; field values never enter observations, events or logs.
    const state = await this.page.evaluate(() => JSON.stringify(Array.from(document.querySelectorAll('input, textarea, select')).map(el => [el.value, el.checked, el.selectedIndex, el.disabled, el.readOnly])));
    return createHash('sha256').update(state).digest('hex');
  }
  async observe() {
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
      this.elements.set(id, handle); elements.push({ id, tag: info.tag, type: info.type, label: info.label, ...state });
    }
    this.revision = await this.page.evaluate(() => window.__offgridRevision);
    this.observedURL = this.page.url(); this.observedAt = Date.now();
    this.formDigest = await this.stateDigest();
    if (this.revision !== initialRevision || this.formDigest !== initialState) {
      this.observation = '';
      throw Error('Page changed during observation; inspect it again.');
    }
    // No field values, screenshots, cookies or password content are collected.
    return { observation_id: this.observation, url: this.observedURL, title: await this.page.title(),
      text: (await this.page.locator('body').innerText()).slice(0, 10000), elements,
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
    if (!['browser_click','browser_fill','browser_select','browser_set_checked'].includes(kind)) throw Error('Unsupported browser action.');
    if (args.observation_id !== this.observation || Date.now() - this.observedAt > 120000 ||
        this.page.url() !== this.observedURL || await this.page.evaluate(() => window.__offgridRevision) !== this.revision ||
        await this.stateDigest() !== this.formDigest) throw Error('Stale observation; inspect the page again.');
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
}
