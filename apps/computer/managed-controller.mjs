// Private utility-process protocol. The host owns consent and task binding;
// this owner keeps a browser resource alive only through an explicit handoff.
import {CompanionSession} from './session.mjs';
import {BrowserStorageCheck} from './browser-storage-check.mjs';
import {fileURLToPath} from 'node:url';

const safeCode=error=>[
  'workspace_changed','service_invalid','target_invalid','network_blocked','network_unavailable',
  'network_mode_invalid','computer_browser_scope_changed','browser_scope_invalid',
  'computer_takeover_unavailable','computer_takeover_unconfirmed',
  'computer_storage_unavailable',
].includes(error?.message)?error.message:'browser_unavailable';
const validID=value=>typeof value==='string'&&value.length>0&&value.length<=128;

export class ManagedBrowserController {
  constructor({send,exit,createSession=options=>new CompanionSession(options)}) {
    this.send=send;this.exit=exit;this.createSession=createSession;this.stopping=false;this.started=false;
  }
  boot(){this.send({state:'booted',browserManualProtocol:1,browserFileBindingProtocol:1});}
  finish(code){if(!this.exited){this.exited=true;this.exit(code);}}
  publish(message){if(!this.stopping)this.send(message);}
  makeSession(data,prepared){
    let owner;
    owner=this.createSession({service:data.service,directory:data.directory,upload:data.upload,prepared,
      storageCheck:new BrowserStorageCheck({root:fileURLToPath(new URL('.',import.meta.url)),directory:data.directory,workspaceRoots:data.browserWorkspaceRoots}),
      emit:message=>{
        if(this.session!==owner||this.stopping)return;
        // Readiness is acknowledged only with its exact target and request ID.
        if(['ready','manual','starting','taking_over'].includes(message.state))return;
        if(message.state==='stopped'){
          if(this.starting)return;
          this.publish(message);this.finish(0);return;
        }
        this.publish(message);
      }});
    return owner;
  }
  startOptions(data){return {origin:data.origin,code:data.code,workspace:data.workspace,
    networkMode:data.networkMode,approvalMode:data.approvalMode,additionalOrigins:data.additionalOrigins,
    expectedBrowserWorkspace:data.expectedBrowserWorkspace,fileTask:data.fileTask};}
  async activate(data,prepared,requestId){
    let owner;
    try{
      owner=this.makeSession(data,prepared);this.session=owner;
      const target=await owner.start(this.startOptions(data));
      if(this.stopping||this.session!==owner){await owner.stop();return;}
      if(owner.state!=='ready')throw Error('browser_unavailable');
      this.target=target;
      void owner.running?.then(()=>{
        if(this.session===owner&&!this.stopping&&!owner.retained&&!owner.driver&&!owner.demo)this.finish(owner.state==='error'?1:0);
      }).catch(()=>{this.publish({state:'error',code:'computer_stop_unconfirmed'});});
      const reply={state:'ready',target,...(requestId?{requestId}:{}),browserManualProtocol:1};
      this.publish(reply);return reply;
    }catch(error){
      // If construction fails after transfer, keep an owner capable of retrying
      // cleanup. A failed close must never be turned into process success/exit.
      if(!owner&&prepared){owner=new CompanionSession({service:this.session?.service?.href??'http://127.0.0.1:1',directory:this.session?.directory??data.directory,prepared});this.session=owner;}
      try{await owner?.stop();}
      catch{this.publish({state:'error',code:'computer_stop_unconfirmed',...(requestId?{requestId}:{})});return;}
      if(this.stopping)return;
      this.publish({state:'error',code:safeCode(error),...(requestId?{requestId}:{})});this.finish(1);
    }
  }
  async stop(){
    if(this.stopped)return this.stopped;
    this.stopping=true;
    this.stopped=(async()=>{
      await this.session?.stop();await this.starting;await this.session?.stop();
      this.send({state:'stopped'});this.finish(0);
    })().catch(error=>{this.stopped=undefined;this.send({state:'error',code:'computer_stop_unconfirmed'});throw error;});
    return this.stopped;
  }
  async command(data){
    if(data?.type==='stop')return this.stop();
    if(this.stopping)return;
    if(data?.type==='start'){
      if(this.started)return;this.started=true;
      this.starting=this.activate(data);
      try{return await this.starting;}finally{this.starting=undefined;}
    }
    if(!['takeover','resume'].includes(data?.type)||!validID(data.requestId))return;
    const key=JSON.stringify([data.type,data.type==='takeover'?data.session:data.previousSession]);
    if(this.operation?.requestId===data.requestId){
      if(this.operation.key!==key)return this.publish({state:'error',requestId:data.requestId,code:'computer_takeover_unavailable'});
      const result=await this.operation.promise;if(result)this.publish(result);return;
    }
    if(this.operation?.pending||!this.session||this.starting||this.target?.id!==(data.type==='takeover'?data.session:data.previousSession)){
      return this.publish({state:'error',requestId:data.requestId,code:'computer_takeover_unavailable'});
    }
    const operation={requestId:data.requestId,key,pending:true};this.operation=operation;
    operation.promise=(async()=>{
      try{
        if(data.type==='takeover'){
          const owner=this.session,target=this.target;await owner.takeover();
          if(this.stopping||this.session!==owner)return;
          const reply={state:'manual',requestId:data.requestId,target,browserManualProtocol:1};
          this.publish(reply);return reply;
        }
        const prepared=this.session.takeRetained();
        this.starting=this.activate(data,prepared,data.requestId);
        try{return await this.starting;}finally{this.starting=undefined;}
      }catch(error){
        if(this.stopping)return;
        const reply={state:'error',requestId:data.requestId,code:safeCode(error)};this.publish(reply);return reply;
      }finally{operation.pending=false;}
    })();
    return operation.promise;
  }
}
