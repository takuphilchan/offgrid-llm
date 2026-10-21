// Explicitly isolated native test orchestration. No current task, desktop grant,
// personal profile, model download or installed service is reused here.
import { spawn } from 'node:child_process';
import { mkdtemp, mkdir, writeFile, readFile, realpath } from 'node:fs/promises';
import { tmpdir, userInfo } from 'node:os';
import { join, resolve } from 'node:path';
import { pathToFileURL } from 'node:url';
import { createServer } from 'node:net';
import { setTimeout as delay } from 'node:timers/promises';

const platforms = {windows:'win32', macos:'darwin', linux:'linux'};
const suites = new Set(['fixture', 'installed', 'model']);
const root = resolve(import.meta.dirname, '../..');

export function options(argv) {
  const result = {};
  for (let i = 0; i < argv.length; i += 2) {
    const key = argv[i]?.slice(2), value = argv[i+1];
    if (!argv[i]?.startsWith('--') || !['platform','suite','repeat','model'].includes(key) || Object.hasOwn(result,key) || !value || value.startsWith('--')) throw Error('invalid_arguments');
    result[key] = value;
  }
  if (!Object.hasOwn(platforms,result.platform) || !suites.has(result.suite) || !/^[1-9][0-9]?$/.test(result.repeat ?? '')) throw Error('invalid_arguments');
  if (result.suite === 'model') {
    if (!/^[A-Za-z0-9][A-Za-z0-9._-]{0,159}$/.test(result.model ?? '')) throw Error('invalid_model_identity');
  } else if (Object.hasOwn(result,'model')) throw Error('unexpected_model');
  return Object.freeze({...result, repeat:Number(result.repeat)});
}

export function isolatedEnvironment(opts, platform, account, env, uid) {
  if (platforms[opts.platform] !== platform) throw Error('platform_unavailable');
  const name = account.split('\\').at(-1);
  if (!/^offgridtest-[a-z0-9][a-z0-9_-]*$/i.test(name) || env.OFFGRID_NATIVE_ISOLATED_DESKTOP !== '1' || uid === 0) throw Error('isolated_account_required');
  if (platform === 'win32' && env.OFFGRID_TEST_NATIVE_WINDOWS !== '1') throw Error('windows_native_opt_in_required');
  if (platform === 'linux' && (!env.DBUS_SESSION_BUS_ADDRESS || !['x11','wayland'].includes(env.XDG_SESSION_TYPE) || (env.XDG_SESSION_TYPE === 'x11' ? !env.DISPLAY : !env.WAYLAND_DISPLAY))) throw Error('desktop_session_unavailable');
  // The owned GTK health fixture uses X11; do not advertise it as Wayland.
  if (platform === 'linux' && opts.suite === 'fixture' && env.XDG_SESSION_TYPE !== 'x11') throw Error('fixture_requires_x11');
  return Object.freeze({platform:opts.platform, architecture:process.arch, session:platform === 'linux' ? env.XDG_SESSION_TYPE : 'isolated-account'});
}

export function requiredTest(opts) {
  const platform = {windows:'Windows', macos:'Darwin', linux:'Linux'}[opts.platform];
  return `TestNative${platform}Document${{fixture:'Fixture', installed:'InstalledWorkflow', model:'ModelWorkflow'}[opts.suite]}`;
}

// Installed/model qualification must be emitted by the owned native test after
// its independent oracles run. A Go pass event alone says only that a test
// function returned; it does not establish the document workflow's outcome.
export function workflowEvidence(events, expected, suite, model) {
  if (suite === 'fixture') return null;
  const records = events.filter(e => e.Test === expected && e.Action === 'output' && typeof e.Output === 'string' && e.Output.includes('OFFGRID_NATIVE_WORKFLOW '));
  if (records.length !== 1) throw Error('workflow_evidence_missing_or_ambiguous');
  const marker = 'OFFGRID_NATIVE_WORKFLOW ';
  const raw = records[0].Output.split(marker)[1].trim();
  if (raw.length > 2048 || raw.includes('\n')) throw Error('invalid_workflow_evidence');
  let report;
  try { report = JSON.parse(raw); } catch { throw Error('invalid_workflow_evidence'); }
  const keys = ['version','suite','model','application_families','checks'];
  const checks = ['new_document_distinct','exact_unicode_readback','other_documents_unchanged','not_saved','browser_blank_tab_verified','single_dispatch'];
  const exact = (value,names) => value && typeof value === 'object' && !Array.isArray(value) && Object.keys(value).length === names.length && names.every(name => Object.hasOwn(value,name));
  if (!exact(report,keys) || report.version !== 1 || report.suite !== suite || report.model !== (model ?? null) ||
      !Number.isSafeInteger(report.application_families) || report.application_families < 2 || report.application_families > 16 ||
      !exact(report.checks,checks) || checks.some(check => report.checks[check] !== true)) throw Error('invalid_workflow_evidence');
  return report;
}

export function testEvidence(events, expected, suite = 'fixture', model = null) {
  const record = events.filter(e => e?.Test === expected);
  if (!record.some(e => e.Action === 'run')) throw Error('required_native_test_missing');
  if (events.some(e => e.Action === 'skip')) throw Error('native_test_unavailable');
  if (events.some(e => e.Action === 'fail') || !record.some(e => e.Action === 'pass')) throw Error('native_test_failed');
  const pass = record.find(e => e.Action === 'pass');
  if (!Number.isFinite(pass.Elapsed) || pass.Elapsed < 0) throw Error('invalid_test_evidence');
  return {test:expected,status:'passed',elapsed_seconds:pass.Elapsed,workflow:workflowEvidence(record,expected,suite,model),measurements:measurementEvidence(record)};
}

// Only a closed numeric record is retained. Provider output, titles, arguments,
// private text and unknown labels must never become performance evidence.
export function measurementEvidence(events) {
  const stages=['discovery','readiness','observation','preparation','input','verification','model','total'];
  const empty=()=>({version:1,stages:Object.fromEntries(stages.map(stage=>[stage,null])),model_calls:null,prompt_tokens:null});
  const records=events.filter(e=>e.Action==='output'&&typeof e.Output==='string'&&e.Output.includes('OFFGRID_NATIVE_METRICS '));
  if(!records.length)return empty();
  if(records.length!==1)throw Error('ambiguous_measurement_evidence');
  const raw=records[0].Output.split('OFFGRID_NATIVE_METRICS ')[1].trim();
  if(raw.length>4096)throw Error('invalid_measurement_evidence');
  let report;
  try {report=JSON.parse(raw);} catch {throw Error('invalid_measurement_evidence');}
  const exact=(value,keys)=>value&&typeof value==='object'&&!Array.isArray(value)&&Object.keys(value).length===keys.length&&keys.every(key=>Object.hasOwn(value,key));
  if(!exact(report,['version','stages','model_calls','prompt_tokens'])||report.version!==1||!exact(report.stages,stages))throw Error('invalid_measurement_evidence');
  for(const value of Object.values(report.stages)) {
    if(value===null)continue;
    if(!exact(value,['calls','milliseconds'])||!Number.isSafeInteger(value.calls)||value.calls<1||!Number.isFinite(value.milliseconds)||value.milliseconds<0)throw Error('invalid_measurement_evidence');
  }
  for(const key of ['model_calls','prompt_tokens'])if(report[key]!==null&&(!Number.isSafeInteger(report[key])||report[key]<0))throw Error('invalid_measurement_evidence');
  if(report.prompt_tokens!==null&&report.model_calls===null)throw Error('invalid_measurement_evidence');
  return report;
}

export function cleanEnvironment(env, directory, port) {
  const clean = Object.fromEntries(Object.entries(env).filter(([key]) => !/^OFFGRID_/i.test(key) && !['NODE_OPTIONS','NODE_PATH','ELECTRON_RUN_AS_NODE'].includes(key.toUpperCase())));
  Object.assign(clean, {
    OFFGRID_NATIVE_ISOLATED_DESKTOP:'1', OFFGRID_TEST_NATIVE_WINDOWS:'1',
    OFFGRID_NATIVE_TEST_DIRECTORY:directory, OFFGRID_NATIVE_DOCUMENT_FIXTURE:join(directory,'owned-documents'),
    OFFGRID_HOST:'127.0.0.1', OFFGRID_PORT:String(port), OFFGRID_DATA_DIR:join(directory,'data'),
    OFFGRID_MODELS_DIR:join(directory,'models'), OFFGRID_BIN_DIR:join(directory,'bin'),
    OFFGRID_CONFIG:join(directory,'config.yaml'), OFFGRID_REQUIRE_AUTH:'false', OFFGRID_ENABLE_P2P:'false',
    OFFGRID_UI_DIR:join(root,'apps/web/dist'), OFFGRID_DOCUMENT_TEST_SERVICE:`http://127.0.0.1:${port}`,
    OFFGRID_DOCUMENT_TEST_STATE:directory, OFFGRID_DOCUMENT_TEST_NO_DOWNLOADS:'1',
  });
  return clean;
}

async function availablePort() {
  const socket = createServer();
  await new Promise((resolve,reject) => { socket.once('error',reject); socket.listen(0,'127.0.0.1',resolve); });
  const port = socket.address().port;
  await new Promise((resolve,reject) => socket.close(error=>error?reject(error):resolve()));
  return port;
}

// Output is bounded and transient. Do not write native app content or complete
// test/service logs to evidence. Only parsed pass/fail metadata is retained.
async function command(executable, args, env, signal, timeout = 180000) {
  const child = spawn(executable,args,{cwd:root,env,windowsHide:true,shell:false,stdio:['ignore','pipe','pipe'],signal});
  let output = '', tooLarge = false;
  const timer = setTimeout(()=>child.kill(),timeout);
  for (const stream of [child.stdout,child.stderr]) stream.on('data',chunk=>{
    if (tooLarge) return;
    if (Buffer.byteLength(output)+chunk.length > 8<<20) { tooLarge=true; output=''; child.kill(); return; }
    output += chunk.toString('utf8');
  });
  try {
    const code = await new Promise((resolve,reject)=>{child.once('error',reject);child.once('close',resolve);});
    if (signal.aborted) throw Error('cancelled');
    if (tooLarge) throw Error('test_output_limit');
    if (code !== 0) throw Error('test_command_failed');
    return output;
  } finally { clearTimeout(timer); }
}

async function startService(binary, env, directory, signal) {
  const child = spawn(binary,['serve'],{cwd:directory,env,windowsHide:true,shell:false,stdio:['ignore','ignore','ignore']});
  let spawnError;
  child.once('error',error=>{spawnError=error;});
  const exited = new Promise(resolve=>child.once('close',resolve));
  const stop = async()=>{
    if (child.exitCode === null && child.signalCode === null && !spawnError) {
      child.kill('SIGTERM');
      const timer = setTimeout(()=>child.kill('SIGKILL'),5000);
      try { await exited; } finally { clearTimeout(timer); }
    }
  };
  try {
    for(let i=0;i<100;i++) {
      if(signal.aborted) throw Error('cancelled');
      if(spawnError || child.exitCode !== null) throw Error('isolated_service_failed');
      try {
        const response = await fetch(`${env.OFFGRID_DOCUMENT_TEST_SERVICE}/api/v2/system`,{redirect:'error',signal:AbortSignal.timeout(500)});
        const system = await response.json();
        const ownedWorkspace = (await readFile(join(directory,'data','workspace-id'),'utf8')).trim();
        if(response.ok && system.product === 'offgrid' && /^[a-f0-9]{32}$/.test(ownedWorkspace) && system.workspace_id === ownedWorkspace) return stop;
      } catch {}
      await delay(100);
    }
    throw Error('isolated_service_timeout');
  } catch(error) { await stop(); throw error; }
}

export async function run(argv, environment = process.env) {
  const opts = options(argv);
  // Refuse before compilation, service creation or any native test process.
  const platform = isolatedEnvironment(opts,process.platform,userInfo().username,environment,process.getuid?.());
  const controller = new AbortController();
  const cancel = ()=>controller.abort();
  process.once('SIGINT',cancel); process.once('SIGTERM',cancel);
  const report = {version:1,platform,suite:opts.suite,model:opts.model ?? null,status:'incomplete',runs:[]};
  let directory;
  try {
    directory = await realpath(await mkdtemp(join(tmpdir(),'offgrid-native-documents-')));
    const binary = join(directory,process.platform === 'win32' ? 'offgrid.exe' : 'offgrid');
    const test = requiredTest(opts);
    const tags = process.platform === 'linux' ? ['-tags','offgrid_native'] : [];
    const base = cleanEnvironment(environment,directory,await availablePort());
    // -list compiles but never executes a test. Absent installed/model suites
    // are unavailable, not a successful "no tests to run" qualification.
    const listed = await command('go',['test',...tags,'./internal/computer','-list',`^${test}$`],base,controller.signal);
    if(!listed.split(/\r?\n/).includes(test)) throw Error('required_native_test_missing');
    await command('go',['build','-o',binary,'./cmd/offgrid'],base,controller.signal);
    for(let repetition=1;repetition<=opts.repeat;repetition++) {
      if(controller.signal.aborted) throw Error('cancelled');
      const state = await realpath(await mkdtemp(join(tmpdir(),'offgrid-native-documents-')));
      const env = cleanEnvironment(environment,state,await availablePort());
      env.OFFGRID_DOCUMENT_TEST_MODEL = opts.model ?? '';
      for(const name of ['data','models','bin']) await mkdir(join(state,name),{mode:0o700});
      await writeFile(join(state,'config.yaml'),'{}',{mode:0o600});
      if(process.platform === 'darwin') await command('xcrun',['clang','-fobjc-arc','-framework','AppKit','apps/computer/native/macos/document-fixture.m','-o',env.OFFGRID_NATIVE_DOCUMENT_FIXTURE],env,controller.signal);
      if(process.platform === 'linux') {
        const flags = (await command('pkg-config',['--cflags','--libs','gtk+-3.0','json-glib-1.0'],env,controller.signal)).trim().split(/\s+/);
        if(flags.some(flag=> !/^-[IDflLWp][A-Za-z0-9_/.,+=:-]*$/.test(flag))) throw Error('unsupported_compiler_flags');
        await command('cc',['apps/computer/native/linux/document-fixture.c','-o',env.OFFGRID_NATIVE_DOCUMENT_FIXTURE,...flags],env,controller.signal);
      }
      const stop = await startService(binary,env,state,controller.signal);
      try {
        const output = await command('go',['test',...tags,'-json','-p','1','-timeout','120s','./internal/computer','-run',`^${test}$`,'-count=1'],env,controller.signal);
        const events = output.split(/\r?\n/).filter(line=>line.startsWith('{')).map(line=>JSON.parse(line));
        report.runs.push({repetition,...testEvidence(events,test,opts.suite,opts.model)});
      } finally { await stop(); }
    }
    report.status='passed';
  } catch(error) {
    report.status=controller.signal.aborted?'cancelled':'unavailable_or_failed';
    report.code=/^[a-z_]+$/.test(error.message)?error.message:'test_command_failed';
  } finally {
    process.removeListener('SIGINT',cancel);process.removeListener('SIGTERM',cancel);
  }
  if(directory) {
    await writeFile(join(directory,'result.json'),JSON.stringify(report,null,2)+'\n',{mode:0o600});
    process.stderr.write(`Redacted test evidence: ${join(directory,'result.json')}\n`);
  }
  return report;
}

if(process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  try {
    const report=await run(process.argv.slice(2));
    process.stdout.write(JSON.stringify(report)+'\n');
    process.exitCode=report.status==='passed'?0:report.status==='cancelled'?130:2;
  } catch(error) {
    process.stdout.write(JSON.stringify({status:'refused',code:/^[a-z_]+$/.test(error.message)?error.message:'test_command_failed'})+'\n');
    process.exitCode=2;
  }
}
