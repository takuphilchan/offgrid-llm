import {expect,test} from '@playwright/test';
import {canonicalTalkFixture,utterance} from './fixtures/canonical-talk';

for(const agent of [false,true])test(`explicit spoken correction and side question preserve active canonical ${agent?'agent':'conversation'} work`,async({page})=>{
  const f=await canonicalTalkFixture(page,{agent});
  // Entering Talk during work must not hide the existing explicit job Stop.
  await expect(page.getByRole('button',{name:'Stop',exact:true})).toBeVisible();
  if(!agent){
    await page.locator('.composer-context-menu > button').click();
    await page.locator('.composer-input-mode select').selectOption('new_work');
    await page.keyboard.press('Escape');
  }
  await page.getByRole('button',{name:'Talk',exact:true}).click();
  await utterance(page,'Actually, use the offline option.',0);
  await expect.poll(()=>f.submissions().length).toBe(1);
  const first=f.submissions()[0];
  expect(first.target_id).toBe(f.conversation.id);
  expect(first.payload).toMatchObject({content:'Actually, use the offline option.',origin:'dictation'});
  expect(first.payload.intent).toBeUndefined();
  await expect(page.getByRole('button',{name:'Speak again',exact:true})).toBeVisible();
  expect(await page.evaluate(()=>(window as any).mic.starts)).toBe(1);
  await page.getByRole('button',{name:'Speak again',exact:true}).click();
  await expect.poll(()=>page.evaluate(()=>(window as any).mic.starts)).toBe(2);
  f.complete(1,'Older correction response.');
  await expect(page.locator('.message.assistant').last()).toContainText('Older correction response.');
  expect(f.spoken).toEqual([]);
  await utterance(page,'What will that cost?',1);
  await expect.poll(()=>f.submissions().length).toBe(2);
  expect(f.submissions()[1].request_id).not.toBe(first.request_id);
  expect(f.submissions()[1].payload).toMatchObject({content:'What will that cost?',origin:'dictation'});
  expect(f.submissions()[1].payload.intent).toBeUndefined();
  expect(f.conversation.focused_work_id).toBe('existing-work');
  expect(f.inputs[0]).toMatchObject({id:'original-input',run_id:'existing-run',run_status:'running'});
  expect(f.commands.some(command=>command.kind==='stop')).toBe(false);
  expect(f.writes.filter(path=>/\/(generate|agent-turn)$/.test(path))).toEqual([]);
  f.complete(2,'The cost is unchanged.');
  await expect.poll(()=>f.spoken).toEqual(['The cost is unchanged.']);
  await expect.poll(()=>page.evaluate(()=>(window as any).players.length)).toBe(1);
  await page.evaluate(()=>(window as any).players[0].dispatchEvent(new Event('ended')));
  await expect.poll(()=>page.evaluate(()=>(window as any).mic.starts)).toBe(3);
  await page.getByRole('button',{name:'Stop Talk',exact:true}).click();
  await expect.poll(()=>f.commands.filter(command=>command.kind==='stop').length).toBe(1);
  expect(f.commands.find(command=>command.kind==='stop').target_id).toBe('original-input');
  expect(f.submissions()).toHaveLength(2);
});

test('lost spoken correction acknowledgment pauses with the retained draft and never resubmits on reload',async({page})=>{
  const f=await canonicalTalkFixture(page,{lost:true});
  await page.getByRole('button',{name:'Talk',exact:true}).click();
  await utterance(page,'Keep this correction for review.',0);
  await expect.poll(()=>f.submissions().length).toBe(1);
  await expect(page.locator('.talk-surface [role=status]')).toContainText('Talk paused');
  await expect(page.locator('.composer textarea')).toHaveValue('Keep this correction for review.');
  const request=f.submissions()[0].request_id;
  expect(f.inputs.filter(input=>input.request_id===request)).toHaveLength(1);
  expect(f.commands.some(command=>command.kind==='stop')).toBe(false);
  await page.reload();
  await expect(page.locator('.canonical-response-summary')).toBeVisible();
  await expect(page.locator('.composer textarea')).toHaveValue('Keep this correction for review.');
  expect(f.submissions()).toHaveLength(1);
  expect(await page.evaluate(()=>(window as any).mic.permissions)).toBe(0);
  expect(f.writes.filter(path=>/\/(generate|agent-turn)$/.test(path))).toEqual([]);
});

test('ending explicit correction capture sends no partial utterance and preserves normal job Stop',async({page})=>{
  const f=await canonicalTalkFixture(page);
  await page.getByRole('button',{name:'Talk',exact:true}).click();
  await expect.poll(()=>page.evaluate(()=>(window as any).mic.starts)).toBe(1);
  await page.getByRole('button',{name:'Stop Talk',exact:true}).click();
  await expect.poll(()=>f.commands.filter(command=>command.kind==='stop').length).toBe(1);
  expect(f.submissions()).toHaveLength(0);
  expect(await page.evaluate(()=>(window as any).mic.pending.length)).toBe(0);
  expect(await page.evaluate(()=>(window as any).mic.stops)).toBeGreaterThan(0);
});

for(const staleNewWork of [false,true])test(`spoken worker answer re-arms listening without a new reply in ${staleNewWork?'stale new-work':'automatic'} mode`,async({page})=>{
  const f=await canonicalTalkFixture(page,{question:true});
  await expect(page.locator('.composer-question')).toContainText('Use the offline option?');
  if(staleNewWork){
    await page.locator('.composer-context-menu > button').click();
    await page.locator('.composer-input-mode select').selectOption('new_work');
    await page.keyboard.press('Escape');
  }
  await page.getByRole('button',{name:'Talk',exact:true}).click();
  await utterance(page,'Yes, use the offline option.',0);
  await expect.poll(()=>f.submissions().length).toBe(1);
  expect(f.submissions()[0].payload).toMatchObject({intent:'answer',question_id:'worker-question',focused_work_id:'existing-work',origin:'dictation'});
  await expect.poll(()=>page.evaluate(()=>(window as any).mic.starts)).toBe(2);
  await expect(page.locator('.talk-surface-copy [role=status]')).toHaveText('Listening');
  expect(f.inputs[1]).toMatchObject({incorporation:'worker_answer',state:'responded'});
  expect(f.inputs[1].run_id).toBeUndefined();
  expect(f.spoken).toEqual([]);
  expect(f.commands.some(command=>command.kind==='stop')).toBe(false);
  expect(f.writes.filter(path=>/\/(generate|agent-turn)$/.test(path))).toEqual([]);
});
