const {TerminalSessions}=require('../../apps/desktop/terminal-sessions');
const fs=require('node:fs'),os=require('node:os'),path=require('node:path'),assert=require('node:assert/strict'),{execFileSync}=require('node:child_process');
if(!process.argv.includes('--isolated-native-test'))throw Error('Explicit native test scope is required.');
const cwd=fs.mkdtempSync(path.join(os.tmpdir(),'offgrid-terminal-tree-'));let text='', errors=[];
const sessions=new TerminalSessions({host:path.resolve('build/terminal-runtime/win-x64/offgrid-terminal-host.exe'),runner:process.execPath,worker:path.resolve('apps/desktop/terminal-worker.js'),cli:path.resolve('build/windows/offgrid.exe'),service:()=> 'http://127.0.0.1:1',choose:async()=>cwd,confirm:async()=>true,send:e=>{if(e.type==='data'){text+=e.text;sessions.ack(e.id,e.sequence);}if(e.type==='error')errors.push(e.message);}});
const wait=async predicate=>{const end=Date.now()+15000;while(!predicate()){if(Date.now()>end)throw Error('Timed out: '+text.slice(-2000));await new Promise(r=>setTimeout(r,50));}};
(async()=>{
 try{
  const shell=await sessions.create({kind:'shell',cols:90,rows:24});sessions.attach(shell.id);
  sessions.resize(shell.id,110,28);
  sessions.input(shell.id,"[Console]::WriteLine(('UNICODE_' + [char]0x65e5 + [char]0x672c + [char]0x8a9e))\r");
  await wait(()=>text.includes('UNICODE_日本語'));
  sessions.input(shell.id,'Start-Sleep -Seconds 60\r');await new Promise(r=>setTimeout(r,500));const beforeInterrupt=text.length;sessions.input(shell.id,'\x03');await wait(()=>text.slice(beforeInterrupt).includes('PS '));
  sessions.input(shell.id,"[Console]::WriteLine(('INTERRUPT_' + 'OK'))\r");await wait(()=>text.includes('INTERRUPT_OK'));
  sessions.input(shell.id,"$p=Start-Process powershell.exe -ArgumentList '-NoProfile','-Command','Start-Sleep -Seconds 300' -WindowStyle Hidden -PassThru; [Console]::WriteLine(('DESC' + 'ENDANT:' + $p.Id))\r");
  await wait(()=>/DESCENDANT:(\d+)/.test(text));const pid=Number(text.match(/DESCENDANT:(\d+)/)[1]);
  const ps=path.join(process.env.SystemRoot,'System32','WindowsPowerShell','v1.0','powershell.exe');
  const alive=()=>execFileSync(ps,['-NoProfile','-NonInteractive','-Command',`if(Get-Process -Id ${pid} -ErrorAction SilentlyContinue){'alive'}else{'gone'}`],{encoding:'utf8',windowsHide:true}).trim();
  assert.equal(alive(),'alive');await sessions.close(shell.id);assert.equal(alive(),'gone');assert.equal(sessions.sessions.size,0);assert.deepEqual(errors,[]);
  console.log(JSON.stringify({passed:true,unicode:true,resize:true,ctrlC:true,descendantStopped:true,cwd}));
 }finally{await sessions.stop();}
})().catch(e=>{console.error(e);process.exitCode=1;});
