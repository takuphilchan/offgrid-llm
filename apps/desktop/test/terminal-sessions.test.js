const test=require('node:test'),assert=require('node:assert/strict');
const {TerminalSessions,launchCommand,launchEnvironment}=require('../terminal-sessions');
test('OffGrid CLI opens a shell with commands ready; Agent opens profile selection',()=>{
 const cli='C:\\OffGrid\\offgrid.exe',root='C:\\Windows';
 const ordinary=launchCommand('cli',cli,root),agent=launchCommand('agent',cli,root);
 assert.match(ordinary.file,/powershell\.exe$/i);
 assert.equal(ordinary.args.includes('agent'),false);
 assert.equal(agent.file,ordinary.file);
 assert.ok(agent.args.includes('-NoExit'));
 assert.ok(agent.args.at(-1).endsWith("; & 'C:\\OffGrid\\offgrid.exe' agent"));
 assert.ok(launchCommand('agent',"C:\\O'Brien $test\\offgrid.exe",root).args.at(-1).endsWith("; & 'C:\\O''Brien $test\\offgrid.exe' agent"));
 assert.throws(()=>launchCommand('arbitrary-command',cli,root),/invalid_kind/);
 const env=launchEnvironment({PATH:'old-path',Path:'older-path',ELECTRON_RUN_AS_NODE:'1'},cli,'http://127.0.0.1:11611');
 assert.equal(Object.keys(env).filter(key=>key.toLowerCase()==='path').length,1);
 assert.ok(env.Path.startsWith('C:\\OffGrid'));
 assert.equal(env.OFFGRID_SERVER_URL,'http://127.0.0.1:11611');
 assert.equal(env.ELECTRON_RUN_AS_NODE,undefined);
 assert.equal(env.OFFGRID_UNICODE,'1');
 assert.equal(launchEnvironment({OFFGRID_UNICODE:'0'},cli,'http://127.0.0.1:11611').OFFGRID_UNICODE,'0');
});
test('returning to an unchanged terminal does not trigger a PTY redraw',()=>{
 const frames=[],manager=new TerminalSessions({});
 manager.sessions.set('owned',{state:'running',cols:80,rows:24,child:{stdin:{writableLength:0,write:line=>frames.push(JSON.parse(line))}}});
 manager.resize('owned',80,24);manager.resize('owned',80,24);
 assert.deepEqual(frames,[]);
 manager.resize('owned',120,40);manager.resize('owned',120,40);
 assert.deepEqual(frames,[{action:'resize',cols:120,rows:40}]);
});
test('a folder picker completed after shutdown cannot start a late process',async()=>{
 let choose,spawns=0;
 const manager=new TerminalSessions({choose:()=>new Promise(resolve=>{choose=resolve;}),spawnProcess:()=>{spawns++;throw Error('must not spawn');}});
 const pending=manager.create({kind:'shell',cols:80,rows:24});
 await manager.stop();choose('C:\\isolated');
 await assert.rejects(pending,/terminal_stopping/);assert.equal(spawns,0);
});
test('unconfirmed stop keeps the owned session and surfaces failure',async()=>{
 const manager=new TerminalSessions({});
 manager.sessions.set('owned',{state:'uncertain',stopped:Promise.resolve(false)});
 await assert.rejects(manager.stop(),/stop_unconfirmed/);
 assert.equal(manager.sessions.size,1);
});
test('normal stop waits for confirmation before releasing ownership',async()=>{
 let confirm;const frames=[];const manager=new TerminalSessions({});
 manager.sessions.set('owned',{state:'running',child:{stdin:{end:frame=>frames.push(JSON.parse(frame))}},stopped:new Promise(resolve=>{confirm=resolve;})});
 const pending=manager.stop();assert.equal(manager.sessions.size,1);assert.deepEqual(frames,[{action:'stop'}]);
 confirm(true);await pending;assert.equal(manager.sessions.size,0);
});
test('a supervisor that never started does not strand application shutdown',async(t)=>{
 const {EventEmitter}=require('node:events'),{PassThrough}=require('node:stream');
 // This is a supervisor failure fixture, not a native PowerShell launch. Keep
 // its Windows prerequisites explicit on Linux/macOS contract-test runners.
 const systemRoot=process.env.SystemRoot;process.env.SystemRoot='C:\\Windows';
 t.after(()=>{if(systemRoot===undefined)delete process.env.SystemRoot;else process.env.SystemRoot=systemRoot;});
 t.mock.method(require('node:fs'),'existsSync',()=>true);
 const child=new EventEmitter();Object.assign(child,{stdin:new PassThrough(),stdout:new PassThrough(),stderr:new PassThrough()});
 const manager=new TerminalSessions({choose:async()=>process.cwd(),cli:process.execPath,service:()=> 'http://127.0.0.1:1',send:()=>{},spawnProcess:()=>{queueMicrotask(()=>{child.emit('error',Error('spawn rejected'));child.emit('close');});return child;}});
 await assert.rejects(manager.create({kind:'offgrid',cols:80,rows:24}),/spawn rejected/);
 await manager.stop();assert.equal(manager.sessions.size,0);
});
