import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {writeFile} from 'node:fs/promises';
import {join} from 'node:path';

// Exercise the real reviewed-profile UI and governed runner. All outputs belong
// to the disposable service; no host executor, desktop grant or connection exists.
export async function runAgentJourney({page,origin,api,evidence,expect}) {
  await page.goto(origin+'/ui/#/agents/profiles');
  await page.getByRole('button',{name:'New agent',exact:true}).click();
  await page.getByRole('button',{name:'Draft and refine',exact:true}).click();
  const editor=page.locator('.profile-editor');
  await editor.getByRole('textbox',{name:'Name',exact:true}).fill('Qualification writer');
  await editor.getByRole('textbox',{name:'Instructions',exact:true}).fill('Create only the private OffGrid task artifacts explicitly requested by the user. Preserve requested content exactly. Do not claim to create files on the computer. After the tool succeeds, briefly describe the downloadable result.');
  await editor.getByRole('button',{name:'Model',exact:true}).click();
  await page.getByRole('listbox',{name:'Model',exact:true}).getByRole('option').click();
  await editor.getByRole('button',{name:'Save agent',exact:true}).click();
  await expect(page.getByRole('status').filter({hasText:'Agent saved'})).toBeVisible();
  const profiles=(await api('/api/v2/agent-profiles')).profiles;
  assert.equal(profiles.length,1);
  const profile=profiles[0];
  assert.equal(profile.settings.name,'Qualification writer');
  assert.equal(profile.revision,1);
  assert.equal(profile.settings.computer??false,false);
  for(const key of ['tools','knowledge','delegates'])assert.equal((profile.settings[key]??[]).length,0);
  await page.getByRole('button',{name:'Use in Chat',exact:true}).click();
  await expect(page.locator('.chat-agent-selector > button')).toContainText('Qualification writer');
  const content='Project Mariner\nLaunch code: ORCHID-742\n';
  const prompt='Create a private downloadable OffGrid task artifact named mariner.txt, format text. Its exact content is the following two lines, with a final newline:\n'+content+'Do not create or edit host files. After saving, tell me the artifact is available for download.';
  await page.locator('.composer textarea').fill(prompt);
  const started=Date.now();
  await page.getByRole('button',{name:'Send',exact:true}).click();
  let run;
  try {
  await expect.poll(async()=>{
    const jobs=await api('/api/v2/jobs');
    if(jobs.length===0)return 'admitting';
    assert.equal(jobs.length,1,'One Chat submission must own one root job');
    run=await api('/api/v2/jobs/'+jobs[0].id);
    if(['failed','cancelled','interrupted','waiting_for_input','waiting_for_approval','uncertain'].includes(run.status)) {
      throw Error('Agent did not complete: '+run.status+' '+(run.error??''));
    }
    return run.status;
  },{timeout:180000,intervals:[500,1000]}).toBe('completed');
  } finally {
    if(run)await writeFile(join(evidence,'agent-run-state.json'),JSON.stringify({id:run.run_id,status:run.status,error:run.error,progress:run.progress?{phase:run.progress.phase,iteration:run.progress.iteration}:null,artifacts:run.artifacts,actions:(run.steps??[]).filter(step=>step.type==='action').map(step=>({tool:step.tool_name,result:step.tool_result})),elapsedMs:Date.now()-started},null,2));
  }
  assert.equal(run.artifacts?.length,1,'Exactly one requested artifact');
  const artifact=run.artifacts[0];
  assert.equal(artifact.name,'mariner.txt');
  assert.equal(artifact.storage_location,'offgrid_task_artifact');
  assert.equal(artifact.host_file_created,false);
  const response=await fetch(origin+'/api/v2/jobs/'+run.run_id+'/artifact?digest='+artifact.sha256,{signal:AbortSignal.timeout(10000)});
  assert.equal(response.status,200);
  const bytes=Buffer.from(await response.arrayBuffer());
  assert.equal(bytes.toString('utf8'),content,'Model must preserve the requested bytes');
  assert.equal(bytes.length,artifact.bytes);
  assert.equal(createHash('sha256').update(bytes).digest('hex'),artifact.sha256);
  const actions=run.steps.filter(step=>step.type==='action');
  assert.equal(actions.length,1,'No duplicate or unrelated action');
  assert.equal(actions[0].tool_name,'task_save_artifact');
  await expect(page.locator('.task-artifacts')).toContainText('mariner.txt');
  await page.getByRole('button',{name:'Preview: mariner.txt',exact:true}).click();
  await expect(page.locator('.artifact-preview-content pre')).toHaveText(content);
  await page.screenshot({path:join(evidence,'agent-artifact.png')});
  let saved;
  await expect.poll(async()=>{
    for(const item of (await api('/v1/sessions')).sessions??[]){
      const session=await api('/v1/sessions/'+encodeURIComponent(item.name));
      if(session.messages.some(message=>message.role==='user'&&message.content===prompt)&&session.messages.at(-1)?.role==='assistant')saved={name:item.name,session};
    }
    return !!saved;
  },{timeout:15000}).toBe(true);
  const result={...saved,answer:saved.session.messages.at(-1).content,runID:run.run_id,profileID:profile.id,profileRevision:profile.revision,artifact,elapsedMs:Date.now()-started};
  await writeFile(join(evidence,'agent-journey.json'),JSON.stringify(result,null,2));
  return result;
}
