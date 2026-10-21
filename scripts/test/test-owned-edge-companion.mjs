// Isolated diagnostic harness. Product NativeSession and NativeWorker perform
// every native operation, including local consent and dispatch journaling.
import {spawn} from 'node:child_process';
import {mkdtemp} from 'node:fs/promises';
import {join,resolve} from 'node:path';
import {tmpdir} from 'node:os';
import {NativeWorker} from '../../apps/computer/native-worker.cjs';
import {NativeSession} from '../../apps/computer/native-session.mjs';
const log=x=>console.log(JSON.stringify(x));
if(process.env.OFFGRID_TEST_OWNED_EDGE!=='public-tourism-with-local-consent')throw Error('explicit_native_test_scope_required');
const directory=process.env.OFFGRID_FIXTURE_STATE;
const profile=await mkdtemp(join(tmpdir(),'offgrid-owned-edge-'));
const browser=spawn('C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',[
  `--user-data-dir=${profile}`,'--no-first-run','--no-default-browser-check','--disable-sync','--disable-extensions',
  '--new-window','https://www.bing.com/'
],{stdio:'ignore',windowsHide:false});
const worker=new NativeWorker({root:resolve('build/computer-runtime/win-x64'),directory:join(directory,'worker')});
let session;
try {
  await worker.start();
  let target;
  const until=Date.now()+20000;
  while(Date.now()<until) {
    const targets=await worker.request('targets');
    const own=targets.filter(t=>t.identity.process_generation.startsWith(`${browser.pid}:`));
    if(own.length===1){target=own[0];break;}
    if(own.length>1)throw Error('ambiguous_owned_target');
    await new Promise(r=>setTimeout(r,250));
  }
  if(!target)throw Error('owned_target_unavailable');
  session=new NativeSession({service:process.env.OFFGRID_FIXTURE_SERVICE,directory:join(directory,'transport'),worker,emit:log});
  const execute=session.execute.bind(session);
  session.execute=async action=>{
    const kind=action.kind==='computer_prepare'?action.arguments.tool:action.kind;
    const args=action.kind==='computer_prepare'?action.arguments.arguments:action.arguments;
    if(!['computer_observe','computer_verify','computer_replace_text','computer_shortcut'].includes(kind))throw Error('computer_prohibited_action');
    if(['computer_replace_text','computer_shortcut'].includes(kind)) {
      const field=session.view?.elements.find(e=>e.id===args.element);
      if(!field){log({state:'test-scope-rejected',error:'unknown_element'});throw Error('computer_stale_observation');}
      if(!field.writable || !/address.*(search|bar)|search.*address/i.test(field.name)){log({state:'test-scope-rejected',error:'not_address_bar'});throw Error('computer_prohibited_action');}
      if(kind==='computer_shortcut' && args.shortcut!=='enter')throw Error('computer_prohibited_action');
      if(kind==='computer_replace_text') {
        const text=args.text;
        const query=/^tourism( in)? Zimbabwe$/i.test(text);
        let publicURL=false;
        try {const u=new URL(text);publicURL=u.protocol==='https:'&&!u.username&&!u.password&&['www.bing.com','www.google.com','zimbabwetourism.net','www.zimbabwetourism.net','whc.unesco.org','zimparks.org.zw','www.zimparks.org.zw'].includes(u.hostname);}catch{}
        if(!query&&!publicURL)throw Error('computer_prohibited_action');
      }
    }
    return execute(action);
  };
  const paired=await session.start({code:process.env.OFFGRID_FIXTURE_PAIR,workspace:'owned-edge-fixture',target,approvalMode:'full_task'});
  log({session:paired.id,state:'paired-awaiting-local-consent'});
  process.stdin.resume();
  await Promise.race([new Promise(r=>process.stdin.once('end',r)),session.running]);
} catch(e){log({error:e.message});process.exitCode=1;}
finally {if(session)await session.stop();else await worker.stop();browser.kill();}
