import {expect, type Page} from '@playwright/test';
import {craftFixture} from '../helpers/craft-fixture';

export async function canonicalTalkFixture(page: Page, options: {agent?:boolean; lost?:boolean; question?:boolean} = {}) {
  await craftFixture(page,'en','dark');
  const model='Local-Research-7B-Instruct-Q4_K_M-long-model-name';
  const profile={id:'voice-agent',revision:1,actor:'local',workspace:'craft',settings:{name:'Voice agent',model,instructions:'',tools:[],knowledge:[],computer:false,max_iterations:10}};
  const conversation: any={id:'talk-conversation',revision:1,project_id:'default',configuration_known:true,focused_work_id:'existing-work',preferences:{model,restrictions:{computer:false,tools:[]},...(options.agent?{profile_id:profile.id,profile_revision:1}:{})}};
  if(options.question)conversation.pending_question={id:'worker-question',work_id:'existing-work',run_id:'existing-run',content:'Use the offline option?'};
  const original: any={id:'original-input',sequence:1,conversation_id:conversation.id,request_id:'original-request',input:{content:'Research the options'},state:'accepted',run_id:'existing-run',run_status:'running',accepted_at:'2026-11-22T00:00:00Z'};
  const inputs:any[]=[original], commands:any[]=[], spoken:string[]=[], writes:string[]=[];
  const session:any={name:'Demo briefing',canonical_id:conversation.id,model_id:model,created_at:original.accepted_at,messages:[{role:'user',content:original.input.content,turn_id:original.id,job_id:original.run_id}]};
  await page.addInitScript(()=>{
    const f=window as any; f.mic={starts:0,stops:0,permissions:0,pending:[]};f.players=[];
    Object.defineProperty(navigator,'mediaDevices',{configurable:true,value:{getUserMedia:async()=>{f.mic.permissions++;return{getTracks:()=>[{stop:()=>f.mic.stops++}]};}}});
    f.AudioContext=undefined;
    f.MediaRecorder=class {
      static isTypeSupported(){return true;} state='inactive';mimeType='audio/wav';ondataavailable?: (event:any)=>void;onstop?:()=>void;
      start(){this.state='recording';f.mic.starts++;}
      stop(){this.state='inactive';this.ondataavailable?.({data:new Blob(['synthetic fixture'],{type:'audio/wav'})});this.onstop?.();}
    };
    f.Audio=class extends EventTarget {
      onended:(()=>void)|null=null;onerror:(()=>void)|null=null;src='';paused=false;
      constructor(){super();this.addEventListener('ended',()=>this.onended?.());}
      play(){f.players.push(this);return Promise.resolve();}pause(){this.paused=true;}load(){}removeAttribute(){this.src='';}
    };
    const fetch=window.fetch.bind(window);
    window.fetch=(input,init)=>{
      if(!String(input).endsWith('/audio/transcriptions'))return fetch(input,init);
      return new Promise((resolve,reject)=>{
        f.mic.pending.push({finish:(text:string)=>resolve(new Response(JSON.stringify({text}),{headers:{'Content-Type':'application/json'}}))});
        init?.signal?.addEventListener('abort',()=>reject(new DOMException('Cancelled','AbortError')),{once:true});
      });
    };
  });
  page.on('request',request=>{if(request.method()!=='GET')writes.push(new URL(request.url()).pathname);});
  await page.route('**/api/v2/system',route=>route.fulfill({json:{product:'offgrid',workspace_id:'craft',capabilities:['conversation-inputs-v1','conversation-input-admission-v1','agent-profiles-v1','agent-conversations-v1']}}));
  await page.route('**/api/v2/agent-profiles/voice-agent*',route=>route.fulfill({json:profile}));
  await page.route('**/v1/sessions',route=>route.fulfill({json:{sessions:[session]}}));
  await page.route('**/v1/sessions/Demo%20briefing',route=>route.fulfill({json:session}));
  await page.route('**/v1/audio/status',route=>route.fulfill({json:{asr:{available:true},tts:{available:true}}}));
  await page.route('**/v1/audio/speech',route=>{spoken.push(route.request().postDataJSON().input);return route.fulfill({contentType:'audio/wav',body:Buffer.alloc(100)});});
  await page.route('**/api/v2/conversations**',route=>{
    const path=new URL(route.request().url()).pathname;
    if(route.request().method()==='GET')return route.fulfill({json:path.endsWith('/inputs')?{items:inputs,next_cursor:inputs.length,has_more:false}:path.endsWith('/configuration')?{...conversation.preferences,provenance:{},resources:[]}:conversation});
    const command=route.request().postDataJSON();commands.push(command);
    if(command.kind==='configure_conversation'){conversation.preferences=command.payload.preferences;conversation.revision++;return route.fulfill({status:202,json:{request_id:command.request_id,target_id:conversation.id,revision:conversation.revision,event_cursor:1,conversation}});}
    if(command.kind==='stop'){const input=inputs.find(input=>input.id===command.target_id);input.stopped_at='2026-11-22T00:00:02Z';input.state='stopping';return route.fulfill({status:202,json:{request_id:command.request_id,target_id:command.target_id,revision:++conversation.revision,event_cursor:3,input}});}
    if(command.kind!=='submit_input')return route.fulfill({status:400,json:{error:{message:'Unexpected command: '+command.kind}}});
    const input:any={id:'spoken-'+inputs.length,sequence:inputs.length+1,conversation_id:conversation.id,conversation_revision:++conversation.revision,request_id:command.request_id,input:command.payload,state:'accepted',run_id:'reply-'+inputs.length,run_status:'running',accepted_at:'2026-11-22T00:00:01Z'};
    if(command.payload.intent==='answer'){
      input.state='responded';input.incorporation='worker_answer';input.applied_at=input.accepted_at;
      delete input.run_id;delete input.run_status;delete conversation.pending_question;
    }
    inputs.push(input);session.messages.push({role:'user',content:command.payload.content,turn_id:input.id,job_id:input.run_id});
    if(options.lost)return route.abort('failed');
    return route.fulfill({status:202,json:{request_id:command.request_id,target_id:conversation.id,revision:conversation.revision,event_cursor:2,input}});
  });
  await page.goto('/ui/#/chat');
  await expect(page.locator('.canonical-response-summary')).toBeVisible();
  return {conversation,session,inputs,commands,spoken,writes,submissions:()=>commands.filter(command=>command.kind==='submit_input'),complete:(index:number,text:string)=>{
    Object.assign(inputs[index],{state:'responded',run_status:'completed',applied_at:'2026-11-22T00:00:03Z'});
    session.messages.push({role:'assistant',content:text,turn_id:inputs[index].id,job_id:inputs[index].run_id});
  }};
}

export async function utterance(page:Page,text:string,index:number){
  await expect.poll(()=>page.evaluate(()=>(window as any).mic.starts)).toBe(index+1);
  await page.getByRole('button',{name:'Stop recording',exact:true}).click();
  await expect.poll(()=>page.evaluate(()=>(window as any).mic.pending.length)).toBe(index+1);
  await page.evaluate(({text,index})=>(window as any).mic.pending[index].finish(text),{text,index});
}
