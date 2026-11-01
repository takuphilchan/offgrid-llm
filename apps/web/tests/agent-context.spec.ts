import {test,expect,type Page} from '@playwright/test';
import {agentContextText} from '../src/i18n/agent-context';
import {agentProfilesText} from '../src/i18n/agent-profiles';
import {skillLibraryText} from '../src/i18n/skill-library';
import type {LocaleCode} from '../src/i18n';
import {mkdtemp,writeFile} from 'node:fs/promises';
import {join} from 'node:path';
import {tmpdir} from 'node:os';

const digest='a'.repeat(64),model='local-model';
const skill={id:'skill-one',revision:1,digest,name:'test-writer',description:'Write exact notes',draft:false,created_at:'2026-10-25T00:00:00Z'};
const content='---\nname: test-writer\ndescription: Write exact notes\n---\nKeep Unicode — café 日本語 exactly.\n';

test('skill folder selection uses a keyboard button and permits selecting the same folder again',async({page},info)=>{
  const {writes}=await fixture(page),copy=agentContextText.en;
  const panel=page.getByRole('region',{name:`${copy.skills} / ${copy.memory}`});
  await panel.locator('summary').first().click();
  const picker=panel.getByRole('button',{name:copy.import,exact:true});
  await expect(panel.locator('input[type=file]')).toBeHidden();
  await picker.focus();const chooser=page.waitForEvent('filechooser');await picker.press('Enter');
  const directory=await mkdtemp(join(tmpdir(),'offgrid-skill-picker-'));
  await writeFile(join(directory,'SKILL.md'),content);
  await(await chooser).setFiles(directory);
  await expect.poll(()=>writes.filter(item=>item.path==='/api/v2/agent-skills'&&item.method==='POST').length).toBe(1);
  await expect(panel.locator('.file-picker-selection')).toContainText('offgrid-skill-picker-');
  await expect(picker).toBeEnabled();await picker.focus();const second=page.waitForEvent('filechooser');await picker.press('Enter');await(await second).setFiles(directory);
  await expect.poll(()=>writes.filter(item=>item.path==='/api/v2/agent-skills'&&item.method==='POST').length).toBe(2);
  expect(writes.some(item=>item.path.includes('/turns')||item.path==='/api/v2/jobs')).toBe(false);
  await page.screenshot({path:info.outputPath('skill-folder-picker.png'),fullPage:true});
});
async function fixture(page:Page,locale:LocaleCode='en') {
  await page.addInitScript(locale=>{localStorage.setItem('offgrid.locale',locale);localStorage.setItem('offgrid.onboarding.complete','true');},locale);
  const profile={id:'Writer',actor:'local',workspace:'context-test',revision:1,digest:'profile',archived:false,created_at:'2026-10-25T00:00:00Z',settings:{name:'Writer',model,max_iterations:10,skills:[] as unknown[]}};
  let consent={revision:0,enabled:false},entries:any[]=[];const writes:{path:string;body:any;method:string}[]=[];
  await page.route('**/health',route=>route.fulfill({json:{status:'healthy'}}));
  await page.route(/\/(?:api\/v2|v1)\//,async route=>{
    const req=route.request(),url=new URL(req.url()),path=url.pathname;let data:unknown={};
    if(req.method()!=='GET'){writes.push({path,body:req.postData()?req.postDataJSON():null,method:req.method()});}
    if(path==='/api/v2/system')data={product:'offgrid',api_version:2,version:'fixture',workspace_id:'context-test',capabilities:['task-first-agents-v2','durable-agent-events-v2','agent-profiles-v1','agent-context-resources-v1']};
    else if(path==='/v1/system/config')data={version:'fixture',require_auth:false};
    else if(path==='/v1/users/me')data={user:null,authenticated:false,guest:true};
    else if(path==='/v1/models')data={data:[{id:model,type:'chat',context_window:32768}]};
    else if(path==='/v1/documents')data={documents:[]};
    else if(path==='/v1/rag/status')data={enabled:true};
    else if(path==='/v1/sessions')data={sessions:[]};
    else if(path==='/api/v2/jobs')data=[];
    else if(path==='/api/v2/computer/sessions')data={sessions:[]};
    else if(path==='/api/v2/agent-profiles/resources')data={computer_permitted:true,connection_scope:'workspace-admin',tools:{}};
    else if(path==='/api/v2/agent-profiles')data={profiles:[profile]};
    else if(path==='/api/v2/agent-profiles/Writer/effective')data={profile,ready:true,issues:[],capabilities:{}};
    else if(path==='/api/v2/agent-profiles/Writer'){if(req.method()==='PUT'){profile.settings=req.postDataJSON().settings;profile.revision++;}data=profile;}
    else if(path==='/api/v2/agent-skills')data={skills:[skill]};
    else if(path==='/api/v2/agent-skills/skill-one')data={skill,bundle:{name:skill.name,description:skill.description,digest,files:[{path:'SKILL.md',content:Buffer.from(content).toString('base64')}]},executable:false};
    else if(path==='/api/v2/agent-memory/Writer/consent'){consent={revision:consent.revision+1,enabled:req.postDataJSON().enabled};data=consent;}
    else if(path==='/api/v2/agent-memory/Writer'){
      if(req.method()==='POST'){const body=req.postDataJSON();const entry={id:body.id||'memory-one',revision:body.expected_revision+1,label:body.label,content:body.content,updated_at:'2026-10-25T00:00:00Z'};entries=[entry,...entries.filter(x=>x.id!==entry.id)];data=entry;}
      else data={consent,entries,deletion_scope:'Historical task evidence is retained.'};
    }else if(path==='/api/v2/agent-memory/Writer/memory-one'&&req.method()==='DELETE'){entries=[];data={deleted:true,historical_evidence_erased:false};}
    return route.fulfill({json:data});
  });
  await page.goto('/ui/#/agents/profiles');await page.getByRole('button',{name:/Writer/}).click();
  await page.locator('.profile-advanced > summary').click();return {writes};
}

test('skills pin exact revisions; memory is opt-in, inspectable and deletable without submitting work',async({page})=>{
  const {writes}=await fixture(page),copy=agentContextText.en;
  const panel=page.getByRole('region',{name:`${copy.skills} / ${copy.memory}`});
  await panel.locator('summary').filter({hasText:/^Skills ·/}).click();
  await panel.getByRole('checkbox',{name:/test-writer/}).check();
  await panel.getByRole('button',{name:`${copy.inspect}: test-writer`}).click();
  await expect(panel.getByText(content,{exact:true})).toBeVisible();
  await page.getByRole('button',{name:agentProfilesText.en.save,exact:true}).click();
  const saved=writes.find(x=>x.path==='/api/v2/agent-profiles/Writer');expect(saved?.body.settings.skills).toEqual([{id:skill.id,revision:1,digest}]);
  await panel.locator('summary').filter({hasText:copy.memory}).click();
  const consent=panel.getByRole('checkbox',{name:copy.enabled});await expect(consent).not.toBeChecked();await consent.click();await expect(consent).toBeChecked();
  await expect.poll(()=>writes.filter(x=>x.path.endsWith('/consent')).length).toBe(1);
  await panel.getByLabel(copy.label,{exact:true}).fill('Writing style');await panel.getByLabel(copy.label,{exact:true}).press('Enter');expect(writes.filter(x=>x.path==='/api/v2/agent-profiles/Writer')).toHaveLength(1);await panel.getByLabel(copy.content,{exact:true}).fill('Keep Unicode — café 日本語 exactly.');await panel.getByRole('button',{name:copy.save,exact:true}).click();
  await expect(panel.getByRole('heading',{name:'Writing style'})).toBeVisible();
  await panel.getByRole('button',{name:copy.edit,exact:true}).click();await panel.getByLabel(copy.content,{exact:true}).fill('Prefer brief reports.');await panel.getByRole('button',{name:copy.save,exact:true}).click();
  await expect(panel.getByText('Prefer brief reports.',{exact:true})).toBeVisible();
  await panel.getByRole('button',{name:copy.delete,exact:true}).click();const dialog=page.getByRole('dialog',{name:copy.delete});await expect(dialog).toContainText(copy.retention);await dialog.getByRole('button',{name:copy.delete,exact:true}).click();
  await expect(panel.getByText(copy.empty,{exact:true})).toBeVisible();
  expect(writes.some(x=>x.path==='/api/v2/jobs'||x.path.includes('/turns'))).toBe(false);
});

test('discover, inspect and attach a skill before explicitly using the saved agent',async({page},info)=>{
  const {writes}=await fixture(page),copy=agentContextText.en,library=skillLibraryText.en;
  const panel=page.getByRole('region',{name:`${copy.skills} / ${copy.memory}`});
  await panel.locator('summary').first().click();
  const search=panel.getByRole('searchbox',{name:library.search});
  await search.fill('EXACT NOTES'); await search.press('Enter');
  await expect(panel.getByRole('checkbox',{name:/test-writer/})).not.toBeChecked(); expect(writes).toEqual([]);
  await panel.getByRole('button',{name:`${copy.inspect}: test-writer`}).click();
  await expect(panel.getByText(library.prompt.replace('{name}','test-writer'),{exact:true})).toBeVisible();
  await panel.getByRole('button',{name:library.attach}).click();
  await search.fill('absent'); await expect(panel.getByText(library.noMatch)).toBeVisible();
  await search.fill(''); await expect(panel.getByRole('checkbox',{name:/test-writer/})).toBeChecked();
  expect(writes).toEqual([]);
  await page.getByRole('button',{name:agentProfilesText.en.save,exact:true}).click();
  expect(writes[0].body.settings.skills).toEqual([{id:skill.id,revision:1,digest}]);
  await page.screenshot({path:info.outputPath('skill-library.png'),fullPage:true});
  await page.getByRole('button',{name:'Use in Chat',exact:true}).click();
  await expect(page).toHaveURL(/#\/chat$/);
  await expect(page.locator('.chat-agent-selector > button')).toContainText('Writer');
  await expect(page.locator('.composer textarea')).toHaveValue('');
  expect(writes).toHaveLength(1);
});

test('unavailable pinned skills remain explicit and never become another revision',async({page})=>{
  const {writes}=await fixture(page),copy=agentContextText.en,library=skillLibraryText.en;
  const panel=page.getByRole('region',{name:`${copy.skills} / ${copy.memory}`});
  await panel.locator('summary').first().click();
  await panel.getByRole('checkbox',{name:/test-writer/}).check();
  await panel.getByRole('button',{name:`${copy.inspect}: test-writer`}).click();
  await page.route('**/api/v2/agent-skills',r=>r.fulfill({json:{skills:[{...skill,revision:2,digest:'b'.repeat(64)}]}}));
  await panel.getByRole('button',{name:copy.refresh}).click();
  await expect(panel.locator('.skill-missing')).toContainText(library.unavailable);
  await expect(panel.getByRole('checkbox',{name:/test-writer/})).not.toBeChecked();
  await page.route('**/api/v2/agent-skills/skill-one?*',r=>r.fulfill({status:404,json:{error:'This pinned revision is no longer available.'}}));
  await panel.locator('.skill-missing').getByRole('button',{name:copy.inspect,exact:true}).click();
  await expect(panel.getByRole('alert')).toContainText('This pinned revision is no longer available.');
  await expect(panel.locator('.skill-inspection')).toHaveCount(0);
  expect(writes).toEqual([]);
  await panel.getByRole('button',{name:library.remove}).click();
  await expect(panel.locator('.skill-missing')).toHaveCount(0);
  await expect(panel.getByRole('checkbox',{name:/test-writer/})).not.toBeChecked();
});

test('draft review failure keeps the skill inactive, then explicit review pins the returned revision',async({page})=>{
  const {writes}=await fixture(page),copy=agentContextText.en,library=skillLibraryText.en;
  let current={...skill,draft:true},reject=true;const reviews:any[]=[];
  await page.route('**/api/v2/agent-skills',r=>r.fulfill({json:{skills:[current]}}));
  await page.route('**/api/v2/agent-skills/skill-one?*',r=>r.fulfill({json:{skill:current,bundle:{files:[{path:'SKILL.md',content:Buffer.from(content).toString('base64')}]},executable:false}}));
  await page.route('**/api/v2/agent-skills/skill-one/review',r=>{
    reviews.push(r.request().postDataJSON());
    if(reject)return r.fulfill({status:409,json:{error:'Review conflict. Refresh and inspect the current revision.'}});
    current={...current,draft:false,revision:2,digest:'b'.repeat(64)};
    return r.fulfill({json:current});
  });
  const panel=page.getByRole('region',{name:`${copy.skills} / ${copy.memory}`});
  await panel.locator('summary').first().click();await panel.getByRole('button',{name:copy.refresh}).click();
  await expect(panel.getByRole('checkbox',{name:/test-writer/})).toBeDisabled();
  await panel.getByRole('button',{name:`${copy.inspect}: test-writer`}).click();
  await expect(panel.getByRole('button',{name:library.attach})).toHaveCount(0);
  await panel.getByRole('button',{name:copy.review}).click();
  await expect(panel.getByRole('alert')).toContainText('Review conflict');
  await expect(panel.getByRole('checkbox',{name:/test-writer/})).toBeDisabled();
  reject=false;await panel.getByRole('button',{name:copy.refresh}).click();
  await panel.getByRole('button',{name:`${copy.inspect}: test-writer`}).click();
  await panel.getByRole('button',{name:copy.review}).click();
  await expect(panel.getByRole('checkbox',{name:/test-writer/})).not.toBeChecked();
  await panel.getByRole('button',{name:library.attach}).click();
  await page.getByRole('button',{name:agentProfilesText.en.save,exact:true}).click();
  expect(writes[0].body.settings.skills).toEqual([{id:skill.id,revision:2,digest:'b'.repeat(64)}]);
  expect(reviews).toEqual([{expected_revision:1,digest},{expected_revision:1,digest}]);
});

for(const locale of Object.keys(agentContextText) as LocaleCode[])test(`context controls: ${locale}, narrow layout, no implicit writes`,async({page})=>{
  await page.setViewportSize({width:320,height:480});const {writes}=await fixture(page,locale),copy=agentContextText[locale];
  const panel=page.getByRole('region',{name:`${copy.skills} / ${copy.memory}`});await panel.locator('summary').first().click();
  await expect(panel.getByRole('button',{name:copy.import,exact:true})).toBeVisible();
  await panel.getByRole('searchbox',{name:skillLibraryText[locale].search}).fill('no matching skill');
  await expect(panel.getByText(skillLibraryText[locale].noMatch,{exact:true})).toBeVisible();
  await panel.locator('summary').filter({hasText:copy.memory}).click();await expect(panel.getByRole('checkbox',{name:copy.enabled})).not.toBeChecked();
  await expect.poll(()=>page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);expect(writes).toEqual([]);
});
