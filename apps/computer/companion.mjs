// Developer/CLI entry; the installed desktop uses the same session core via IPC.
import { createInterface } from 'node:readline/promises';
import { stdin, stdout } from 'node:process';
import { join } from 'node:path';
import { homedir } from 'node:os';
import { BrowserDriver } from './browser.mjs';
import { BrowserWorkspace } from './browser-workspace.mjs';
import { browserOrigins } from './browser-scope.mjs';
import { startDemo, DEMO_ORIGIN } from './demo.mjs';
import { CompanionSession, localService, browserOrigin } from './session.mjs';

const input = createInterface({input:stdin, output:stdout});
let session, driver, demo, stopping = false;
async function close() {
  stopping = true; input.close();
  await session?.stop();
  await driver?.close().catch(()=>{}); await demo?.close().catch(()=>{});
}
process.on('SIGINT',()=>void close());
process.on('SIGTERM',()=>void close());
try {
  const service = localService(await input.question('Local OffGrid service [http://127.0.0.1:11611]: ') || 'http://127.0.0.1:11611');
  const origin = browserOrigin((await input.question('HTTPS page address (or demo for an isolated practice page): ')).trim());
  const more=origin===DEMO_ORIGIN?'':(await input.question('Additional approved HTTPS origins, including needed resource sites (comma-separated; blank for this site only): ')).trim();
  const additionalOrigins=more?more.split(',').map(value=>value.trim()):[];
  const scope=origin===DEMO_ORIGIN?[origin]:browserOrigins(origin,additionalOrigins);
  const response=await fetch(new URL('/api/v2/system',service),{redirect:'error',signal:AbortSignal.timeout(20000)});
  if(!response.ok)throw Error('service_invalid');
  const identity=await response.json();
  if(identity.product!=='offgrid'||identity.api_version!==2)throw Error('service_invalid');
  const workspaceEnabled=identity.capabilities?.includes('browser-workspace-v1')===true;
  if(scope.length>1&&!workspaceEnabled)throw Error('computer_upgrade_required');
  const networkMode = origin === DEMO_ORIGIN ? 'direct' : (await input.question('Network: direct or trusted-vpn [direct]: ')).trim() || 'direct';
  if (!['direct','trusted-vpn'].includes(networkMode)) throw Error('Choose direct or trusted-vpn.');
  if (networkMode === 'trusted-vpn') console.log('Trust boundary: use only a VPN you trust. OffGrid permits fake-DNS routing for the selected site but cannot independently verify destinations hidden by your VPN. HTTPS and site restrictions remain enabled. Private-network access is not granted.');
  const approvalMode=(await input.question('Approvals: ask_every_time, scoped_changes, or full_task [scoped_changes]: ')).trim()||'scoped_changes';
  if(!['ask_every_time','scoped_changes','full_task'].includes(approvalMode))throw Error('Choose ask_every_time, scoped_changes, or full_task.');
  console.log(`A separate browser starts at ${origin}, with up to ${workspaceEnabled?8:1} task tabs. Approved sites: ${scope.join(', ')}. All page and resource requests are restricted to those sites. Approval policy: ${approvalMode}. Credentials, payments, privilege/security changes, installation, permanent deletion, scripts, and uncertain retries stay blocked. No personal profile or saved login is used. Ctrl+C closes this session immediately. Sessions expire after 10 minutes and are not resumed.`);
  if ((await input.question('Start this supervised session? Type yes: ')).trim() !== 'yes') throw Error('Local consent declined.');
  if (origin === DEMO_ORIGIN) demo = await startDemo();
  if (stopping) throw Error('stopped');
  driver = await (workspaceEnabled?BrowserWorkspace:BrowserDriver).open(demo?.origin ?? origin, {testLoopback:!!demo, networkMode,additionalOrigins:scope.slice(1)});
  if (stopping) throw Error('stopped');
  console.log('Browser preflight passed. In OffGrid, open Developer connection and click Pair browser for a fresh code.');
  const code = (await input.question('Paste the one-time code: ')).trim(); input.close();
  session = new CompanionSession({service:service.href, directory:join(homedir(),'.offgrid-llm','computer'), prepared:{driver,demo},
    emit:message=>{if(message.state==='error') console.error(`Browser stopped: ${message.code}. Inspect the task before starting another session.`);}});
  driver = null; demo = null;
  await session.start({origin,code,workspace:identity.workspace_id,networkMode,approvalMode,additionalOrigins:scope.slice(1),expectedBrowserWorkspace:workspaceEnabled});
  console.log('Paired. Keep this window open; Ctrl+C is the local emergency stop.');
  await session.running;
  if(session.state==='error') process.exitCode=1;
} catch (error) { if (!stopping) { console.error({network_blocked:'Private or reserved site address. If your trusted VPN uses fake DNS, restart and explicitly choose trusted-vpn; do not disable the VPN.',network_unavailable:'Could not open this HTTPS page. Check connectivity, certificates and redirects to other sites; outside origins remain blocked.'}[error.message] ?? error.message); process.exitCode=1; } }
finally { await close(); }
