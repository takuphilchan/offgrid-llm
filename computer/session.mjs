// Shared by the interactive CLI and the desktop-managed utility process.
// No renderer receives the session credential or arbitrary execution APIs.
import { mkdir, chmod } from 'node:fs/promises';
import { join } from 'node:path';
import { DispatchJournal } from './journal.mjs';
import { BrowserDriver } from './browser.mjs';
import { publicPage } from './network.mjs';
import { startDemo, DEMO_ORIGIN } from './demo.mjs';

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
  constructor({service, directory, emit = () => {}, openBrowser = BrowserDriver.open, prepared}) {
    this.service = localService(service); this.directory = directory; this.emit = emit; this.openBrowser = openBrowser;
    this.abort = new AbortController(); this.stopping = false; this.token = ''; this.state = 'idle';
    this.driver = prepared?.driver; this.demo = prepared?.demo;
  }
  publish(state, code) { this.state = state; this.emit({state, ...(code ? {code} : {})}); }
  async request(path, body) {
    const response = await fetch(new URL(path, this.service), { method: body === undefined ? 'GET' : 'POST', redirect: 'error',
      headers: {'Content-Type':'application/json', ...(this.token ? {Authorization:`Bearer ${this.token}`} : {})},
      ...(body === undefined ? {} : {body:JSON.stringify(body)}), signal:AbortSignal.any([this.abort.signal, AbortSignal.timeout(20000)]) });
    if (!response.ok) throw Error('connection_lost');
    return response.json();
  }
  companion(path, body = {}) { return this.request(`/api/v2/computer/companion/${path}`, body); }
  async start({origin, code, workspace, networkMode = 'direct'}) {
    if (this.state !== 'idle' || this.stopping || !/^[a-f0-9]{64}$/.test(code)) throw Error('invalid_session');
    try {
      this.publish('starting');
      const identity = await this.request('/api/v2/system');
      if (identity.product !== 'offgrid' || identity.api_version !== 2 || (workspace && identity.workspace_id !== workspace)) throw Error('workspace_changed');
      origin = browserOrigin(origin);
      if (!['direct','trusted-vpn'].includes(networkMode) || (origin === DEMO_ORIGIN && networkMode !== 'direct')) throw Error('network_mode_invalid');
      if (origin === DEMO_ORIGIN && !this.demo) this.demo = await startDemo();
      if (this.stopping) throw Error('stopped');
      if (!this.driver) this.driver = await this.openBrowser(this.demo?.origin ?? origin, {testLoopback: !!this.demo, networkMode});
      if (this.stopping) throw Error('stopped');
      this.driver.page?.once('close', () => { void this.stop(); });
      await mkdir(this.directory, {recursive:true, mode:0o700});
      this.journal = new DispatchJournal(join(this.directory, 'dispatch.sqlite'));
      await chmod(join(this.directory, 'dispatch.sqlite'), 0o600);
      const paired = await this.companion('pair', {code, origin:origin === DEMO_ORIGIN ? origin : new URL(origin).origin, protocol_version:1});
      this.token = paired.token;
      this.binding = JSON.stringify([this.service.origin, identity.workspace_id ?? '', paired.session.id]);
      this.journal.discardPreviousSessionResults(this.binding);
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
      while (!this.stopping) {
        const {action} = await this.companion('poll');
        if (this.stopping) break;
        if (!action) { await new Promise(resolve => setTimeout(resolve, 300)); continue; }
        const journal = this.journal;
        const dispatch = journal.prepare(action, this.binding);
        if (!dispatch.execute) {
          await this.companion('reply', dispatch.reply);
          continue;
        }
        let result;
        try { result = await this.driver.execute(action.kind,action.arguments); }
        catch {
          if (!this.stopping) await this.companion('reply',{id:action.id,result:'',error:'Action could not be verified. Inspect the browser before reconciling this task.'});
          throw Error('action_uncertain');
        }
        if (this.stopping) break;
        const reply = {id:action.id,result:JSON.stringify(result)};
        journal.complete(action.id, reply);
        await this.companion('reply', reply);
      }
    } catch (error) {
      if (!this.stopping) this.publish('error', ['action_conflict','action_uncertain'].includes(error.message) ? error.message : 'connection_lost');
    } finally { await this.stop(); }
  }
  async stop() {
    this.stopping = true; this.abort.abort();
    const driver = this.driver, demo = this.demo, journal = this.journal, binding = this.binding;
    this.driver = null; this.demo = null; this.journal = null; this.token = '';
    const previous = this.closing;
    this.closing = (async () => {
      await previous;
      await driver?.close().catch(() => {});
      await demo?.close().catch(() => {});
      if (journal) {
        try { if (binding) journal.forgetResults(binding); }
        finally { journal.close(); }
      }
      if (this.state !== 'error') this.publish('stopped');
    })();
    return this.closing;
  }
}
