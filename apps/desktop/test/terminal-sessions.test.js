const test=require('node:test'),assert=require('node:assert/strict');
const {TerminalSessions}=require('../terminal-sessions');
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
test('a supervisor that never started does not strand application shutdown',async()=>{
 const {EventEmitter}=require('node:events'),{PassThrough}=require('node:stream');
 const child=new EventEmitter();Object.assign(child,{stdin:new PassThrough(),stdout:new PassThrough(),stderr:new PassThrough()});
 const manager=new TerminalSessions({choose:async()=>process.cwd(),cli:process.execPath,service:()=> 'http://127.0.0.1:1',send:()=>{},spawnProcess:()=>{queueMicrotask(()=>{child.emit('error',Error('spawn rejected'));child.emit('close');});return child;}});
 await assert.rejects(manager.create({kind:'offgrid',cols:80,rows:24}),/spawn rejected/);
 await manager.stop();assert.equal(manager.sessions.size,0);
});
