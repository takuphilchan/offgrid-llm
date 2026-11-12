// Isolated real Windows ConPTY/editor checks. Only a gated Go test executable
// runs: no service, model, workspace grant or installed application is involved.
const assert=require('node:assert/strict'),fs=require('node:fs'),os=require('node:os'),path=require('node:path');
const pty=require('../../apps/desktop/node_modules/node-pty');
if(process.platform!=='win32'||!process.argv.includes('--isolated-native-test'))throw Error('Requires Windows and --isolated-native-test');
const binary=path.resolve(process.argv[2]);assert.ok(fs.statSync(binary).isFile());
const cwd=fs.mkdtempSync(path.join(os.tmpdir(),'offgrid-agent-input-'));
const clean=s=>s.replace(/\x1b\][^\x07]*(?:\x07|\x1b\\)/g,'').replace(/\x1b\[[0-?]*[ -/]*[@-~]/g,'');
async function run(label,keys,expected,resize=false){
 const child=pty.spawn(binary,['-test.run=^TestAgentSessionPTYInput$','-test.v'],{cols:90,rows:24,cwd,useConpty:true,useConptyDll:true,env:{SystemRoot:process.env.SystemRoot,PATH:process.env.PATH,TEMP:cwd,TMP:cwd,USERPROFILE:cwd,TERM:'xterm-256color',OFFGRID_TEST_SESSION_PTY:'1',OFFGRID_PLAIN:'0',OFFGRID_TUI:'1'}});
 let output='',sent=false,approved=false;
 try{await new Promise((resolve,reject)=>{
  const timer=setTimeout(()=>{child.kill();reject(Error(label+': timed out '+clean(output).slice(-1500)));},15000);
  child.onData(text=>{output+=text;const plain=clean(output);
   if(!sent&&plain.includes('Enter send')){sent=true;if(resize)child.resize(48,16);child.write(keys);}
   if(!approved&&plain.includes('APPROVAL_READY')){approved=true;child.write('y\r');}
  });
  child.onExit(({exitCode})=>{clearTimeout(timer);try{
   assert.equal(exitCode,0,clean(output));assert.ok(approved&&clean(output).includes('APPROVAL_RECEIVED'),label);
   const raw=clean(output).split('INPUT_RESULT:')[1].split('APPROVAL_READY')[0].replace(/[\r\n]/g,'').trim();
   assert.equal(JSON.parse(raw),expected,label);resolve();
  }catch(error){reject(error);}});
 });console.log('PASS '+label+': exact input and synthetic approval handoff');}
 finally{try{child.kill();}catch{}}
}
async function live(stopping=false,prefill=false){
 const child=pty.spawn(binary,['-test.run=^TestAgentLivePTYInput$','-test.v'],{cols:90,rows:24,cwd,useConpty:true,useConptyDll:true,env:{SystemRoot:process.env.SystemRoot,PATH:process.env.PATH,TEMP:cwd,TMP:cwd,USERPROFILE:cwd,TERM:'xterm-256color',OFFGRID_TEST_SESSION_PTY:'1',OFFGRID_TEST_LIVE_STOP:stopping?'1':'0',OFFGRID_TEST_LIVE_PREFILL:prefill?'1':'0',OFFGRID_PLAIN:'0',OFFGRID_TUI:'1'}});
 let output='',sent=false,queued=false,draft=false,approved=false;
 await new Promise((resolve,reject)=>{
  const timer=setTimeout(()=>{child.kill();reject(Error('live timed out '+clean(output).slice(-1800)));},15000);
  child.onData(text=>{output+=text;const plain=clean(output);
   if(!sent&&plain.includes('Enter steer')){sent=true;child.write(stopping?'retained on stop\x03':'shorter café\r');}
   if(!stopping&&!queued&&plain.includes('CORRECTION_ACCEPTED')&&plain.includes('Incremental fixture')){assert.ok(!plain.includes('Completed in'),'live answer buffered until completion');queued=true;child.write('/queue next question\r');}
   if(!stopping&&!draft&&plain.includes('Follow-up queued')){draft=true;fs.writeFileSync(path.join(cwd,prefill?'live-prefill-frame.ansi':'live-before-resize.ansi'),output);child.resize(72,20);child.write('unfinished 日本語');if(!prefill)setTimeout(()=>fs.writeFileSync(path.join(cwd,'live-frame.ansi'),output),150);}
   if(!approved&&plain.includes('APPROVAL_READY')){approved=true;child.write('y\r');}
  });
  child.onExit(({exitCode})=>{clearTimeout(timer);try{
   assert.equal(exitCode,0,clean(output).slice(-3000));assert.ok(approved&&clean(output).includes('APPROVAL_RECEIVED'));
   const raw=clean(output).split('LIVE_DRAFTS:')[1].split('APPROVAL_READY')[0].replace(/[\r\n]/g,'').trim();
   assert.deepEqual(JSON.parse(raw),stopping?['retained on stop']:['next question','unfinished 日本語']);resolve();
  }catch(e){reject(e);}});
 });
 console.log(stopping?'PASS live Stop: cancellation retains draft and releases input':prefill?'PASS live after prior conversation: compact streaming editor':'PASS live: steering, queued/unfinished drafts, resize, stream and approval ownership');
}
async function access(label,keys,expected){
 const child=pty.spawn(binary,['-test.run=^TestWorkspaceAccessPTY$','-test.v'],{cols:90,rows:24,cwd,useConpty:true,useConptyDll:true,env:{SystemRoot:process.env.SystemRoot,PATH:process.env.PATH,TEMP:cwd,TMP:cwd,USERPROFILE:cwd,TERM:'xterm-256color',OFFGRID_TEST_SESSION_PTY:'1',OFFGRID_PLAIN:'0',OFFGRID_TUI:'1'}});
 let output='',sent=false;
 await new Promise((resolve,reject)=>{
  const timer=setTimeout(()=>{child.kill();reject(Error(label+': timeout'));},15000);
  child.onData(text=>{output+=text;if(!sent&&clean(output).includes('Enter confirm')){sent=true;fs.writeFileSync(path.join(cwd,'access-frame.ansi'),output);child.write(keys);}});
  child.onExit(({exitCode})=>{clearTimeout(timer);try{assert.equal(exitCode,0,clean(output));assert.ok(clean(output).includes('ACCESS_RESULT:'+expected),clean(output));resolve();}catch(e){reject(e);}});
 });console.log('PASS access '+label);
}
(async()=>{
 await run('newline','first\nsecond\r','first\nsecond');
 await run('Unicode paste','\x1b[200~café 日本語\nsecond\x1b[201~\r','café 日本語\nsecond');
 await run('grapheme delete','a👨‍👩‍👧‍👦\x7f\r','a');
 await run('resized multiline','\x1b[200~'+('wrapped words '.repeat(12))+'\nlast\x1b[201~\r',('wrapped words '.repeat(12))+'\nlast',true);
 await live();
 await live(false,true);
 await live(true);
 await access('default cancels','\r','n');
 await access('remember requires confirmation','2\r','remember');
 await access('paste cannot approve','\x1b[200~2\r\x1b[201~\r','n');
 await access('escape cancels selected grant','1\x1b','n');
 // node-pty's Windows pipe workers may retain Node handles after every owned
 // child has emitted exit. All cases above await that exit before this point.
 process.stdout.write(JSON.stringify({passed:true,platform:'Windows ConPTY',fixtureOnly:true,directory:cwd})+'\n',()=>process.exit(0));
})().catch(error=>{console.error(error);process.exit(1);});
