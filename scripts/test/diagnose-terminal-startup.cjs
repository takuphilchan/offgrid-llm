// Diagnostic only: fixed commands, disposable profiles, no installed app/model.
const fs=require('node:fs'),os=require('node:os'),path=require('node:path');
const pty=require('../../apps/desktop/node_modules/node-pty');
const {Terminal}=require('../../apps/desktop/node_modules/@xterm/headless');
const {launchCommand}=require('../../apps/desktop/terminal-sessions');
const shell=path.join(process.env.SystemRoot,'System32/WindowsPowerShell/v1.0/powershell.exe');
const baseArgs=['-NoLogo','-NoProfile'];
async function probe(name,args,{fullEnv=false,useConptyDll=true,limit=30000}={}){
 const dir=fs.mkdtempSync(path.join(os.tmpdir(),'offgrid-ci-shell-probe-'));
 const env={...(fullEnv?process.env:{}),SystemRoot:process.env.SystemRoot,PATH:process.env.PATH||process.env.Path,PATHEXT:process.env.PATHEXT||'.COM;.EXE;.BAT;.CMD',ComSpec:path.join(process.env.SystemRoot,'System32/cmd.exe'),TEMP:dir,TMP:dir,USERPROFILE:dir,APPDATA:dir,LOCALAPPDATA:dir,TERM:'xterm-256color'};
 const terminal=new Terminal({cols:100,rows:30});let text='',exited=false,exitCode=null,replies=[];
 const start=Date.now();const child=pty.spawn(shell,args,{cols:100,rows:30,cwd:dir,name:'xterm-256color',useConpty:true,useConptyDll,env});
 terminal.onData(value=>{replies.push(value);if(!exited)child.write(value);});
 child.onData(value=>{text=(text+value).slice(-8000);terminal.write(value);});
 child.onExit(event=>{exited=true;exitCode=event.exitCode;});
 while(!exited&&!text.includes('OFFGRID_PROBE_READY')&&!text.includes(']633;OffGrid;')&&Date.now()-start<limit)await new Promise(r=>setTimeout(r,100));
 const result={name,elapsed:Date.now()-start,exited,exitCode,replies,text};
 if(!exited){child.kill();const stop=Date.now();while(!exited&&Date.now()-stop<5000)await new Promise(r=>setTimeout(r,100));}
 terminal.dispose();result.closed=exited;console.log(JSON.stringify(result));
 if(!exited)throw Error('Owned probe shell did not close');
}
(async()=>{
 const integrated=launchCommand('shell',path.join(os.tmpdir(),'fixture-offgrid.exe')).args;
 await probe('noninteractive',[...baseArgs,'-NonInteractive','-Command',"Write-Output 'OFFGRID_PROBE_READY'"]);
 await probe('interactive-simple',[...baseArgs,'-NoExit','-Command',"Write-Output 'OFFGRID_PROBE_READY'"]);
 await probe('original-integration',integrated,{limit:90000});
 await probe('encoded-integration',[...baseArgs,'-NoExit','-EncodedCommand',Buffer.from(integrated.at(-1),'utf16le').toString('base64')]);
 await probe('inherited-environment',integrated,{fullEnv:true});
 await probe('system-conpty',integrated,{useConptyDll:false});
})().catch(error=>{console.error(error);process.exitCode=1;});
