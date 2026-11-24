import {expect, test} from '@playwright/test';
import {craftFixture} from './helpers/craft-fixture';

test('canonical acceptance binds speech to its input and poll reads only that committed answer', async ({page}) => {
  await craftFixture(page, 'en', 'dark');
  const model = 'Local-Research-7B-Instruct-Q4_K_M-long-model-name';
  const conversation: any = {id:'voice-conversation',revision:1,project_id:'default',configuration_known:true,preferences:{model,restrictions:{computer:false,tools:[]}}};
  const session: any = {name:'Demo briefing',canonical_id:conversation.id,model_id:model,created_at:'2026-11-22T00:00:00Z',messages:[]};
  const inputs: any[] = [], commands: any[] = [], spoken: string[] = [];
  await page.addInitScript(() => {
    const f=window as any; f.players=[];
    f.Audio=class extends EventTarget {
      onended: (()=>void)|null=null; onerror: (()=>void)|null=null; src='';
      constructor(){super();this.addEventListener('ended',()=>this.onended?.());}
      play(){f.players.push(this);return Promise.resolve();} pause(){} load(){} removeAttribute(){this.src='';}
    };
  });
  await page.route('**/api/v2/system', route => route.fulfill({json:{product:'offgrid',workspace_id:'craft',capabilities:['conversation-inputs-v1','conversation-input-admission-v1']}}));
  await page.route('**/v1/sessions', route => route.fulfill({json:{sessions:[session]}}));
  await page.route('**/v1/sessions/Demo%20briefing', route => route.fulfill({json:session}));
  await page.route('**/v1/audio/status', route => route.fulfill({json:{tts:{available:true}}}));
  await page.route('**/v1/audio/speech', route => {
    spoken.push(route.request().postDataJSON().input);
    return route.fulfill({contentType:'audio/wav',body:Buffer.alloc(100)});
  });
  await page.route('**/api/v2/conversations**', route => {
    const path=new URL(route.request().url()).pathname;
    if(route.request().method()==='GET') {
      const json=path.endsWith('/inputs')?{items:inputs,next_cursor:inputs.length,has_more:false}:path.endsWith('/configuration')?{...conversation.preferences,provenance:{},resources:[]}:conversation;
      return route.fulfill({json});
    }
    const command=route.request().postDataJSON(); commands.push(command);
    if(command.kind==='configure_conversation') {
      conversation.preferences=command.payload.preferences; conversation.revision++;
      return route.fulfill({status:202,json:{request_id:command.request_id,target_id:conversation.id,revision:conversation.revision,event_cursor:1,conversation}});
    }
    if(command.kind!=='submit_input') return route.fulfill({status:400,json:{error:{message:'Unexpected command: '+command.kind}}});
    const input={id:'canonical-input',conversation_id:conversation.id,request_id:command.request_id,sequence:1,conversation_revision:++conversation.revision,input:command.payload,state:'accepted',accepted_at:'2026-11-22T00:00:01Z'};
    inputs.push(input);
    session.messages.push({role:'user',content:command.payload.content,turn_id:input.id});
    return route.fulfill({status:202,json:{request_id:command.request_id,target_id:conversation.id,revision:conversation.revision,event_cursor:2,input}});
  });
  await page.goto('/ui/#/chat');
  await expect(page.locator('.composer textarea')).toBeEnabled();
  await page.locator('.composer-context-menu > button').click();
  await page.getByRole('checkbox',{name:'Speak responses'}).click();
  await expect(page.getByRole('checkbox',{name:'Speak responses'})).toBeChecked();
  await page.keyboard.press('Escape');
  await page.locator('.composer textarea').fill('Canonical spoken request');
  await page.locator('.composer-submit').click();
  await expect.poll(()=>inputs.length).toBe(1);
  expect(spoken).toEqual([]);
  Object.assign(inputs[0],{state:'responded',run_status:'completed',run_id:'canonical-run'});
  session.messages.push({role:'assistant',content:'Other input must remain silent.',turn_id:'other-input',job_id:'canonical-run'});
  // A run ID alone cannot make another input's answer speak.
  await expect(page.locator('.message.assistant')).toContainText(['Other input must remain silent.']);
  expect(spoken).toEqual([]);
  session.messages.push({role:'assistant',content:'Exact committed answer.',turn_id:inputs[0].id,job_id:'canonical-run'});
  await expect.poll(()=>spoken).toEqual(['Exact committed answer.']);
  await expect.poll(()=>page.evaluate(()=>(window as any).players.length)).toBe(1);
  await page.evaluate(()=>(window as any).players[0].dispatchEvent(new Event('ended')));
  await expect(page.locator('.chat-speech-controls').getByRole('button',{name:'Stop reading'})).toHaveCount(0);
  const submissions=commands.filter(command=>command.kind==='submit_input');
  expect(submissions).toHaveLength(1);
  expect(submissions[0].payload).toMatchObject({content:'Canonical spoken request',origin:'typed'});
  expect(session.messages.at(-1).content).toBe('Exact committed answer.');
});
