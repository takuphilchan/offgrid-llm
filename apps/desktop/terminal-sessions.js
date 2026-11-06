'use strict';
const { spawn } = require('node:child_process');
const { randomUUID } = require('node:crypto');
const path = require('node:path');
const fs = require('node:fs');
const { StringDecoder } = require('node:string_decoder');

const shellArgs=['-NoLogo','-NoProfile','-NoExit','-Command','[Console]::InputEncoding = [Text.UTF8Encoding]::new($false); [Console]::OutputEncoding = [Text.UTF8Encoding]::new($false); $OutputEncoding = [Console]::OutputEncoding'];
function launchCommand(kind,cli,systemRoot=process.env.SystemRoot) {
  if(kind==='agent'||kind==='offgrid') {
    // Invoke the pinned executable as a literal PowerShell string. The parent
    // shell remains interactive after /exit; no input is replayed or restarted.
    const args=[...shellArgs];
    args[args.length-1]+="; & '"+cli.replace(/'/g,"''")+"' agent";
    return {file:path.win32.join(systemRoot,'System32','WindowsPowerShell','v1.0','powershell.exe'),args};
  }
  if(kind==='shell'||kind==='cli')return {file:path.win32.join(systemRoot,'System32','WindowsPowerShell','v1.0','powershell.exe'),args:shellArgs};
  throw Error('terminal_invalid_kind');
}
function launchEnvironment(base,cli,service) {
  const env={...base};
  delete env.ELECTRON_RUN_AS_NODE;
  const pathValue=Object.entries(env).find(([key])=>key.toLowerCase()==='path')?.[1]||'';
  for(const key of Object.keys(env))if(key.toLowerCase()==='path')delete env[key];
  env.Path=path.win32.dirname(cli)+path.win32.delimiter+pathValue;
  env.TERM='xterm-256color';
  if(env.OFFGRID_UNICODE===undefined)env.OFFGRID_UNICODE='1';
  env.OFFGRID_SERVER_URL=service;
  return env;
}

class TerminalSessions {
  constructor({host,runner,worker,choose,confirm,send,cli,service,spawnProcess=spawn}) { Object.assign(this,{host,runner,worker,choose,confirm,send,cli,service,spawnProcess}); this.sessions=new Map(); this.closing=false; this.generation=0; }
  available() {return process.platform==='win32' && fs.existsSync(this.host) && fs.existsSync(this.worker);}
  async create({kind,cols,rows}) {
    if(this.closing)throw Error('terminal_stopping');
    const generation=this.generation;const cwd=await this.choose();if(!cwd)return null;
    if(this.closing||generation!==this.generation)throw Error('terminal_stopping');
    const {file,args}=launchCommand(kind,this.cli);
    if(!fs.existsSync(file))throw Error('The matching local executable is unavailable.');
    const id=randomUUID(), record={id,cwd,kind,cols,rows,title:kind==='agent'||kind==='offgrid'?'Agent':kind==='cli'?'OffGrid CLI':'PowerShell',state:'starting',confirmed:false,attached:false,frames:[]};
    const child=this.spawnProcess(this.host,[this.runner,this.worker],{stdio:['pipe','pipe','pipe'],windowsHide:true,env:{...process.env,ELECTRON_RUN_AS_NODE:'1'}});
    record.child=child; this.sessions.set(id,record);
    record.stopped=new Promise(resolve=>{record.resolveStop=resolve;});
    let buffer='';const decoder=new StringDecoder('utf8');
    child.stderr.on('data',()=>{}); // Never retain terminal/private content in application logs.
    const result=new Promise((resolve,reject)=>{
      const timer=setTimeout(()=>{this.closeInBackground(id);reject(Error('Terminal startup timed out.'));},20000);
      const settled=(error)=>{clearTimeout(timer);error?reject(error):resolve(this.snapshot(record));};
      child.once('error',error=>{record.state='failed';if(!child.pid){record.confirmed=true;record.resolveStop(true);this.sessions.delete(id);}else this.closeInBackground(id);settled(error);});
      child.stdout.on('data',data=>{
        buffer+=decoder.write(data); if(Buffer.byteLength(buffer)>1024*1024){this.closeInBackground(id);settled(Error('Invalid terminal output.'));return;}
        let index;while((index=buffer.indexOf('\n'))>=0){const line=buffer.slice(0,index);buffer=buffer.slice(index+1);let event;try{event=JSON.parse(line);}catch{this.closeInBackground(id);settled(Error('Invalid terminal frame.'));return;}
          if(event.type==='ready'){record.state='running';record.pid=event.pid;settled();}
          else if(event.type==='stopped'){record.confirmed=event.confirmed===true;}
          else if(event.type==='exit'){record.state='exited';this.send({id,...event});this.closeInBackground(id);}
          else if(event.type==='error'){record.state='failed';this.send({id,...event});settled(Error(event.message));}
          else if(event.type==='data'){if(record.attached)this.send({id,...event});else record.frames.push({id,...event});}
        }
      });
      child.once('close',()=>{
        const stopped=record.confirmed;record.state=stopped?'stopped':'uncertain';record.resolveStop(stopped);if(stopped)this.sessions.delete(id);
        this.send({id,type:stopped?'stopped':'error',message:stopped?'':'Terminal stopping could not be confirmed.'});
        if(record.pid===undefined)settled(Error('Terminal worker exited before becoming ready.'));
      });
    });
    // Use exactly one Windows PATH key. Mixed Path/PATH keys can make the CLI
    // disappear from the PTY even though the packaged executable exists.
    const env=launchEnvironment(process.env,this.cli,this.service());
    child.stdin.on('error',()=>{});
    child.stdin.write(JSON.stringify({action:'start',file,args,cwd,cols,rows,env})+'\n');
    return result;
  }
  closeInBackground(id){void this.close(id,false).catch(()=>this.send({id,type:'error',message:'Terminal stopping could not be confirmed. Close the terminal again to inspect its state.'}));}
  snapshot(r){return {id:r.id,cwd:r.cwd,kind:r.kind,title:r.title,state:r.state};}
  attach(id){const r=this.sessions.get(id);if(!r)throw Error('terminal_unknown_session');if(r.attached)return;r.attached=true;for(const frame of r.frames)this.send(frame);r.frames=[];}
  write(id,value){const r=this.sessions.get(id);if(!r||r.state!=='running')throw Error('terminal_not_running');if(r.child.stdin.writableLength>65536)throw Error('terminal_input_busy');r.child.stdin.write(JSON.stringify(value)+'\n');}
  input(id,text){this.write(id,{action:'input',text});}
  resize(id,cols,rows){const record=this.sessions.get(id);if(!record)throw Error('terminal_unknown_session');if(record.cols===cols&&record.rows===rows)return;this.write(id,{action:'resize',cols,rows});record.cols=cols;record.rows=rows;}
  ack(id,sequence){const r=this.sessions.get(id);if(r&&['running','exited'].includes(r.state))r.child.stdin.write(JSON.stringify({action:'ack',sequence})+'\n');}
  async close(id,ask=true){
    const r=this.sessions.get(id);if(!r)return;
    if(ask&&['running','starting'].includes(r.state)&&!await this.confirm(r))throw Error('terminal_close_cancelled');
    if(!['stopped','uncertain'].includes(r.state)){r.state='stopping';r.child.stdin.end(JSON.stringify({action:'stop'})+'\n');}
    let timer;const stopped=await Promise.race([r.stopped,new Promise(resolve=>{timer=setTimeout(()=>resolve(false),8000);})]);clearTimeout(timer);
    if(!stopped)throw Error('terminal_stop_unconfirmed');
    this.sessions.delete(id);
  }
  async stop(){this.closing=true;this.generation++;const results=await Promise.allSettled([...this.sessions.keys()].map(id=>this.close(id,false)));this.closing=false;if(results.some(r=>r.status==='rejected'))throw Error('terminal_stop_unconfirmed');}
}
module.exports={TerminalSessions,launchCommand,launchEnvironment};
