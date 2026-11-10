// Explicitly isolated native PowerShell/ConPTY check: only temporary folders and
// fixed print/status commands. No service, model, user profile or saved workspace.
const assert=require('node:assert/strict'),fs=require('node:fs'),os=require('node:os'),path=require('node:path');
if(process.platform!=='win32'||!process.argv.includes('--isolated-native-test'))throw Error('Requires Windows and --isolated-native-test');
const pty=require('../../apps/desktop/node_modules/node-pty');
const {launchCommand}=require('../../apps/desktop/terminal-sessions');
const dir=fs.mkdtempSync(path.join(os.tmpdir(),'offgrid-command-check-'));fs.mkdirSync(path.join(dir,'child'));
const launch=launchCommand('shell',path.join(dir,'offgrid.exe'));
let text='',exited=false;
const child=pty.spawn(launch.file,launch.args,{name:'xterm-256color',cols:100,rows:30,cwd:dir,useConpty:true,useConptyDll:true,env:{SystemRoot:process.env.SystemRoot,PATH:process.env.PATH||process.env.Path,PATHEXT:process.env.PATHEXT||'.COM;.EXE;.BAT;.CMD',ComSpec:path.join(process.env.SystemRoot,'System32/cmd.exe'),TEMP:dir,TMP:dir,USERPROFILE:dir,APPDATA:dir,LOCALAPPDATA:dir,TERM:'xterm-256color'}});
child.onData(value=>{text+=value;if(text.length>200000)text=text.slice(-200000);});child.onExit(()=>{exited=true;});
const markers=()=>Array.from(text.matchAll(/\x1b\]633;OffGrid;([A-Za-z0-9+/=]+)(?:\x07|\x1b\\)/g),m=>JSON.parse(Buffer.from(m[1],'base64').toString('utf8')));
async function until(check){const end=Date.now()+20000;while(Date.now()<end){if(check())return;await new Promise(r=>setTimeout(r,50));}throw Error('Timed out: '+JSON.stringify({markers:markers(),tail:text.slice(-1500)}));}
(async()=>{try{
 await until(()=>markers().some(m=>m.event==='prompt'));
 for(const [command,exit] of [["[Console]::Write('café 日本語')",0],["& $env:SystemRoot\\System32\\cmd.exe /d /c exit 7",7],["Get-Item ./missing-test-file -ErrorAction SilentlyContinue",null],["& $env:SystemRoot\\System32\\cmd.exe /d /c exit 7",null],["Set-Location child",0]]){
  const count=markers().filter(m=>m.event==='end').length;child.write(command+'\r');
  await until(()=>markers().filter(m=>m.event==='end').length>count);
  assert.equal(markers().filter(m=>m.event==='end').at(-1).exit,exit,JSON.stringify(markers()));
  if(exit===null)assert.equal(markers().filter(m=>m.event==='end').at(-1).failed,true);
  assert.equal(markers().filter(m=>m.event==='begin').at(-1).command,command);
 }
 await until(()=>markers().filter(m=>m.event==='prompt').at(-1).cwd===path.join(dir,'child'));
 child.write('exit\r');await until(()=>exited);
 console.log(JSON.stringify({passed:true,checks:['native command boundaries','Unicode','native exit status','changed directory','owned shell exit'],state:dir}));
}finally{if(!exited){child.kill();await until(()=>exited);}}})().then(()=>process.exit(0),error=>{console.error(error);process.exit(1);});
