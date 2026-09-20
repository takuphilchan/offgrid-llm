// Developer/CLI entry; the installed desktop uses the same session core via IPC.
import { createInterface } from 'node:readline/promises';
import { stdin, stdout } from 'node:process';
import { join } from 'node:path';
import { homedir } from 'node:os';
import { BrowserDriver } from './browser.mjs';
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
  const origin = browserOrigin((await input.question('Browser target: type demo for an isolated local test, or a public HTTPS origin [demo]: ')).trim() || 'demo');
  console.log(`A separate browser will access only ${origin}. OffGrid may read its pages. Changes require approval. No personal profile, saved login or downloads are used. Ctrl+C closes this session immediately. Sessions expire after 10 minutes and are not resumed.`);
  if ((await input.question('Start this supervised session? Type yes: ')).trim() !== 'yes') throw Error('Local consent declined.');
  if (origin === DEMO_ORIGIN) demo = await startDemo();
  if (stopping) throw Error('stopped');
  driver = await BrowserDriver.open(demo?.origin ?? origin, {testLoopback:!!demo});
  if (stopping) throw Error('stopped');
  console.log('Browser preflight passed. In OffGrid, open Developer connection and click Pair browser for a fresh code.');
  const code = (await input.question('Paste the one-time code: ')).trim(); input.close();
  session = new CompanionSession({service:service.href, directory:join(homedir(),'.offgrid-llm','computer'), prepared:{driver,demo},
    emit:message=>{if(message.state==='error') console.error(`Browser stopped: ${message.code}. Inspect the task before starting another session.`);}});
  driver = null; demo = null;
  await session.start({origin,code});
  console.log('Paired. Keep this window open; Ctrl+C is the local emergency stop.');
  await session.running;
  if(session.state==='error') process.exitCode=1;
} catch (error) { if (!stopping) { console.error(error.message); process.exitCode=1; } }
finally { await close(); }
