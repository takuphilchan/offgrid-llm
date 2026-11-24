// Owns only a disposable ConPTY and gated test process. Never attaches a user
// terminal, starts a service, reads a real secret or touches installed state.
const assert=require('node:assert/strict'),fs=require('node:fs'),os=require('node:os'),path=require('node:path');
const pty=require('../../apps/desktop/node_modules/node-pty');
if(process.platform!=='win32'||!process.argv.includes('--isolated-native-test'))throw Error('Requires Windows and --isolated-native-test');
const binary=path.resolve(process.argv[2]);assert.ok(fs.statSync(binary).isFile());
const cwd=fs.mkdtempSync(path.join(os.tmpdir(),'offgrid-secret-input-'));
const clean=s=>s.replace(/\x1b\][^\x07]*(?:\x07|\x1b\\)/g,'').replace(/\x1b\[[0-?]*[ -/]*[@-~]/g,'');
async function run(mode){
 const child=pty.spawn(binary,['-test.run=^TestWorkspaceSecretTerminalHelper$'],{cols:100,rows:24,cwd,useConpty:true,useConptyDll:true,env:{SystemRoot:process.env.SystemRoot,PATH:process.env.PATH,TEMP:cwd,TMP:cwd,USERPROFILE:cwd,TERM:'xterm-256color',OFFGRID_WORKSPACE_SECRET_PTY:mode}});
 let output='',sent=false,confirmed=false;
 try{await new Promise((resolve,reject)=>{
  const timer=setTimeout(()=>{child.kill();reject(Error(`${mode}: protected input deadline`));},15000);
  child.onData(text=>{
   output+=text;const plain=clean(output);
   if(!sent&&plain.includes('Backup passphrase: ')){
    sent=true;
    if(mode==='interrupt')child.write('\x03');
    else if(mode==='oversize')child.write('x'.repeat(4097));
    else if(mode==='oversize-suffix')child.write('x'.repeat(4097)+'private-pasted-suffix\r');
    else if(mode!=='cancel')child.write('isolated café 日本語 😀\r');
   }
   if(!confirmed&&plain.includes('Confirm backup passphrase: ')){
    confirmed=true;child.write(mode==='confirm'?'isolated café 日本語 😀\r':'different secret\r');
   }
  });
  child.onExit(({exitCode})=>{clearTimeout(timer);try{
   const plain=clean(output);assert.equal(exitCode,0,plain);assert.ok(plain.includes('PROTECTED_INPUT_DONE'),plain);
   for(const secret of ['isolated café','different secret','private-pasted-suffix','x'.repeat(30)])assert.ok(!plain.includes(secret),'protected input echoed');
   resolve();
  }catch(error){reject(error);}});
 });console.log(`PASS ${mode}: no echo, bounded input and terminal restoration`);}
 finally{try{child.kill();}catch{}}
}
(async()=>{
 for(const mode of ['confirm','mismatch','interrupt','cancel','oversize','oversize-suffix'])await run(mode);
 process.stdout.write(JSON.stringify({passed:true,platform:'Windows ConPTY',fixtureOnly:true,directory:cwd})+'\n',()=>process.exit(0));
})().catch(error=>{console.error(error);process.exit(1);});
