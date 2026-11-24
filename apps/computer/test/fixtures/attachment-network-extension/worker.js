// Diagnostic fixture only: loaded exclusively in the test-owned empty profile.
// No native messaging, service authority, page listener, or external RPC exists.
const resourceTypes = ['main_frame','sub_frame','stylesheet','script','image','font','object','xmlhttprequest','ping','csp_report','media','websocket','webtransport','webbundle','other'];
const escaped = text => text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
let owner;
const bounded = (list, value) => { if (list.length < 128) list.push(value); };
chrome.debugger.onEvent.addListener((source, method, params) => {
  const current = owner;
  if (!current || source.tabId !== current.tabId) return;
  if (method === 'Fetch.requestPaused') {
    const allowed = current.origins.includes(new URL(params.request.url).origin);
    bounded(current.intercepted, {url:params.request.url,allowed});
    void chrome.debugger.sendCommand({tabId:current.tabId}, allowed ? 'Fetch.continueRequest' : 'Fetch.failRequest',
      {requestId:params.requestId,...(!allowed ? {errorReason:'BlockedByClient'} : {})})
      .catch(error => bounded(current.errors, String(error.message)));
  }
  if (method === 'Network.responseReceived') bounded(current.responses,
    {url:params.response.url,ip:params.response.remoteIPAddress ?? ''});
});

globalThis.installNetworkProbe = async ({origin,privateOrigin}) => {
  if (owner) throw Error('probe_already_attached');
  for (const text of [origin,privateOrigin]) {
    const url = new URL(text);
    if (url.origin !== text || url.protocol !== 'https:' || !/^(selected|private)\.offgrid-fixture\.example$/.test(url.hostname)) throw Error('fixture_scope_invalid');
  }
  const matches = await chrome.tabs.query({url:`${origin}/selected`});
  if (matches.length !== 1 || !Number.isInteger(matches[0].id)) throw Error('fixture_tab_ambiguous');
  const tabId = matches[0].id;
  const origins = [origin,privateOrigin];
  const rules = [
    {id:1,priority:1,action:{type:'block'},condition:{tabIds:[tabId],resourceTypes,urlFilter:'*'}},
    ...origins.map((url,index) => ({id:index+2,priority:2,action:{type:'allow'},
      condition:{tabIds:[tabId],resourceTypes,regexFilter:`^${escaped(url)}/`}})),
  ];
  await chrome.declarativeNetRequest.updateSessionRules({removeRuleIds:[1,2,3],addRules:rules});
  owner = {tabId,origins,intercepted:[],responses:[],errors:[]};
  try {
    await chrome.debugger.attach({tabId}, '1.3');
    await chrome.debugger.sendCommand({tabId}, 'Network.enable');
    await chrome.debugger.sendCommand({tabId}, 'Fetch.enable', {patterns:[{urlPattern:'*',requestStage:'Request'}]});
    return {tabId,rules:await chrome.declarativeNetRequest.getSessionRules()};
  } catch (error) {
    await chrome.debugger.detach({tabId}).catch(()=>{});
    await chrome.declarativeNetRequest.updateSessionRules({removeRuleIds:[1,2,3]});
    owner=undefined;throw error;
  }
};
globalThis.networkProbeStatus = () => owner ? structuredClone(owner) : null;
globalThis.releaseNetworkProbe = async () => {
  if (!owner) return;
  await chrome.debugger.detach({tabId:owner.tabId});
  await chrome.declarativeNetRequest.updateSessionRules({removeRuleIds:[1,2,3]});
  owner=undefined;
};
