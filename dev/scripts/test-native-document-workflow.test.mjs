import { test } from 'node:test';
import assert from 'node:assert/strict';
import { options, isolatedEnvironment, requiredTest, testEvidence, cleanEnvironment } from './test-native-document-workflow.mjs';

const args=['--platform','windows','--suite','fixture','--repeat','3'];
test('native harness accepts only a bounded closed CLI, never command strings or paths',()=>{
  assert.equal(options(args).repeat,3);
  for(const extra of [['--url','http://127.0.0.1:11611'],['--profile','Default'],['--task','saved'],['--repeat','2'],['--model','download']]) assert.throws(()=>options([...args,...extra]));
  for(const repeat of ['0','-1','1.5','100','Infinity','1;whoami']) assert.throws(()=>options([...args.slice(0,-1),repeat]));
  for(const model of ['../personal','C:\\models',';command','http://host','--help','']) assert.throws(()=>options(['--platform','windows','--suite','model','--repeat','1','--model',model]));
  assert.equal(options(['--platform','linux','--suite','model','--repeat','1','--model','Qwen2.5-7B-Instruct-Q4_K_M']).model,'Qwen2.5-7B-Instruct-Q4_K_M');
});
test('personal account, platform mismatch, absent opt-in and root refuse before execution',()=>{
  const opts=options(args),env={OFFGRID_NATIVE_ISOLATED_DESKTOP:'1',OFFGRID_TEST_NATIVE_WINDOWS:'1'};
  assert.equal(isolatedEnvironment(opts,'win32','MACHINE\\OffGridTest-docs',env).platform,'windows');
  for(const [platform,account,environment,uid] of [['win32','phil',env],['win32','OffGridTest-',env],['win32','OffGridTest-docs',{}],['darwin','OffGridTest-docs',env],['win32','OffGridTest-docs',env,0]]) assert.throws(()=>isolatedEnvironment(opts,platform,account,environment,uid));
});
test('Linux fixture and Wayland qualification are not interchangeable',()=>{
  const fixture=options(['--platform','linux','--suite','fixture','--repeat','1']);
  const wayland={OFFGRID_NATIVE_ISOLATED_DESKTOP:'1',DBUS_SESSION_BUS_ADDRESS:'unix:path=fixture',XDG_SESSION_TYPE:'wayland',WAYLAND_DISPLAY:'wayland-0'};
  assert.throws(()=>isolatedEnvironment(fixture,'linux','offgridtest-docs',wayland,1000));
  assert.equal(isolatedEnvironment({...fixture,suite:'installed'},'linux','offgridtest-docs',wayland,1000).session,'wayland');
  assert.throws(()=>isolatedEnvironment({...fixture,suite:'installed'},'linux','offgridtest-docs',{...wayland,WAYLAND_DISPLAY:''},1000));
});
test('missing, skipped, failing and incomplete tests never count as native passes',()=>{
  const expected=requiredTest(options(args));
  const pass=[{Test:expected,Action:'run'},{Test:expected,Action:'pass',Elapsed:1.25}];
  assert.equal(testEvidence(pass,expected).elapsed_seconds,1.25);
  for(const events of [[],[{Action:'pass'}],pass.slice(1),[{Test:expected,Action:'skip'}],[...pass,{Test:expected+'/no-permission',Action:'skip'}],[...pass,{Action:'fail'}],[{Test:expected,Action:'run'},{Test:expected,Action:'pass',Elapsed:-1}]]) assert.throws(()=>testEvidence(events,expected));
});
test('owned environment cannot inherit production workspace, auth, profile or native target overrides',()=>{
  const env=cleanEnvironment({PATH:'tools',OFFGRID_DATA_DIR:'personal',OFFGRID_URL:'http://production',OFFGRID_API_KEY:'secret',OFFGRID_NATIVE_DOCUMENT_FIXTURE:'unrelated',NODE_OPTIONS:'--require personal'},'/tmp/owned',12345);
  assert.equal(env.PATH,'tools');assert.equal(env.OFFGRID_URL,undefined);assert.equal(env.OFFGRID_API_KEY,undefined);assert.equal(env.NODE_OPTIONS,undefined);
  assert.match(env.OFFGRID_NATIVE_DOCUMENT_FIXTURE,/owned[\\/]owned-documents$/);
  assert.match(env.OFFGRID_DATA_DIR,/owned[\\/]data$/);
  assert.equal(env.OFFGRID_DOCUMENT_TEST_NO_DOWNLOADS,'1');
  assert.equal(env.OFFGRID_DOCUMENT_TEST_SERVICE,'http://127.0.0.1:12345');
});
