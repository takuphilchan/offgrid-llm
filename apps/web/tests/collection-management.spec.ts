import {expect,test,type Page} from '@playwright/test';
import {craftFixture} from './helpers/craft-fixture';
import {collectionManagement} from '../src/i18n/collection-management';
import {collectionDestination} from '../src/i18n/collection-destination';
import {documentReader} from '../src/i18n/document-reader';
import {languageRegistry} from '../src/i18n/language';
import type {LocaleCode} from '../src/i18n';

const source={id:'collection-source',version:1,revision:4,name:'Records',owner:'local',readers:['reader'],editors:['editor']};
const target={...source,id:'collection-target',revision:7,name:'Department',readers:[],editors:[]};
async function setup(page:Page,locale:LocaleCode='en',theme='light',owner='local'){
 await craftFixture(page,locale,theme);
 const state={policies:[{...source,owner},{...target}],moves:[] as Record<string,unknown>[],updates:[] as Record<string,unknown>[],created:[] as object[],destination:source.id};
 await page.route('**/api/v2/system',r=>r.fulfill({json:{product:'offgrid',version:'fixture',workspace_id:'craft',api_version:2,capabilities:['knowledge-collections-v1','document-imports-v1']}}));
 await page.route('**/v1/users',r=>r.fulfill({json:{users:[{id:'local',username:'Owner',role:'admin'},{id:'reader',username:'Reader account',role:'user'},{id:'editor',username:'Editor account',role:'user'}]}}));
 await page.route('**/v1/collections*',r=>{
  const id=new URL(r.request().url()).searchParams.get('id');
  if(r.request().method()==='POST'){const body=r.request().postDataJSON();state.created.push(body);const p={...target,id:'collection-created',revision:1,name:body.name};state.policies.push(p);return r.fulfill({json:p});}
  if(r.request().method()==='PUT'){const body=r.request().postDataJSON();state.updates.push(body);const i=state.policies.findIndex(p=>p.id===id);state.policies[i]={...state.policies[i],...body,revision:body.expected_revision+1};return r.fulfill({json:state.policies[i]});}
  return r.fulfill({json:id?state.policies.find(p=>p.id===id):{collections:state.policies}});
 });
 await page.route('**/v1/documents',r=>r.fulfill({json:{documents:[
  {id:'record',name:'Record.txt',size:128,index_status:'ready',source_retained:true,chunk_count:1,collection_id:state.destination,collection_revision:8},
  {id:'other',name:'Department.txt',size:128,index_status:'ready',source_retained:true,chunk_count:1,collection_id:target.id,collection_revision:1},
  {id:'legacy',name:'Unclassified.txt',size:128,index_status:'ready',source_retained:true,chunk_count:1},
 ]}}));
 await page.route('**/v1/documents/collection*',r=>{
  if(r.request().method()==='PUT'){const body=r.request().postDataJSON();state.moves.push(body);state.destination=body.destination_id;return r.fulfill({json:{document_id:'record',revision:9,collection:state.policies.find(p=>p.id===state.destination)}});}
  return r.fulfill({json:{document_id:'record',revision:8,collection:state.policies[0]}});
 });
 await page.goto('/ui/#/knowledge');return state;
}
async function openManager(page:Page,locale:LocaleCode='en'){
 await page.getByRole('button',{name:collectionManagement(locale).title,exact:true}).click();
 const dialog=page.getByRole('dialog');await expect(dialog.locator('.collection-manager-list').getByRole('button',{name:'Records',exact:true})).toBeEnabled();
 await dialog.locator('.collection-manager-list').getByRole('button',{name:'Records',exact:true}).click();await expect(dialog.locator('.collection-editor')).toBeVisible();return dialog;
}
async function openMove(page:Page,locale:LocaleCode='en'){
 const card=page.locator('.resource-card').filter({has:page.getByRole('heading',{name:'Record.txt',exact:true})});
 await card.locator('.utility-button').click();await page.getByRole('button',{name:collectionManagement(locale).move,exact:true}).click();
 const dialog=page.getByRole('dialog');await expect(dialog.locator('.collection-destination select')).toBeEnabled();return dialog;
}
for(const [index,locale] of (Object.keys(languageRegistry) as LocaleCode[]).entries())test(`collection review, filtering and move in ${locale}`,async({page},info)=>{
 const state=await setup(page,locale,index%2?'dark':'light'),copy=collectionManagement(locale),labels=collectionDestination(locale);
 await page.setViewportSize({width:390,height:844});
 const filter=page.locator('.collection-library-controls select');await filter.selectOption(source.id);
 await expect(page.locator('.resource-card')).toHaveCount(1);
 const manager=await openManager(page,locale);await manager.getByLabel(copy.name,{exact:true}).fill('Updated records');
 await manager.locator('.collection-members li').filter({hasText:'Reader account'}).getByRole('combobox').selectOption('editor');
 await manager.getByRole('button',{name:copy.review,exact:true}).click();
 expect(state.updates).toHaveLength(0);await expect(manager.getByRole('button',{name:copy.apply,exact:true})).toBeDisabled();
 await manager.getByRole('checkbox',{name:labels.confirmFiles}).check();await manager.getByRole('button',{name:copy.apply,exact:true}).click();
 await expect(manager.getByRole('status')).toContainText(copy.saved);expect(state.updates).toEqual([{expected_revision:4,name:'Updated records',readers:[],editors:['editor','reader']}]);
 expect(await manager.evaluate(node=>node.scrollWidth<=node.clientWidth)).toBe(true);
 await page.keyboard.press('Escape');await expect(manager).toHaveCount(0);
 const move=await openMove(page,locale);await move.locator('.collection-destination select').selectOption(target.id);
 await expect(move.getByRole('button',{name:copy.move,exact:true})).toBeDisabled();
 await move.getByRole('checkbox',{name:labels.confirmFiles}).check();
 expect(await move.evaluate(node=>node.scrollWidth<=node.clientWidth)).toBe(true);
 if(locale==='ar'||locale==='en')await page.screenshot({path:info.outputPath(`move-${locale}.png`),fullPage:true});
 await move.getByRole('button',{name:copy.move,exact:true}).click();await expect(move).toHaveCount(0);
 expect(state.moves).toEqual([{document_id:'record',expected_revision:8,expected_source_revision:5,destination_id:target.id,expected_destination_revision:7}]);
 await expect(page.locator('.resource-card')).toHaveCount(0);await filter.selectOption(target.id);await expect(page.locator('.resource-card')).toHaveCount(2);
 expect(await page.evaluate(()=>document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
});

test('administrator cannot edit another owner’s private audience',async({page})=>{
 const state=await setup(page,'en','light','another-owner');const dialog=await openManager(page);
 await expect(dialog).toContainText(collectionManagement('en').readOnly);await expect(dialog.getByRole('button',{name:'Review changes'})).toHaveCount(0);expect(state.updates).toHaveLength(0);
 await page.keyboard.press('Escape');const card=page.locator('.resource-card').filter({has:page.getByRole('heading',{name:'Record.txt',exact:true})});await card.locator('.utility-button').click();await page.getByRole('button',{name:'Move to collection',exact:true}).click();
 await expect(page.getByRole('dialog')).toContainText(collectionManagement('en').readOnly);await expect(page.locator('.collection-destination')).toHaveCount(0);expect(state.moves).toHaveLength(0);
});
test('stale membership update is not replayed and requires a fresh review',async({page})=>{
 await setup(page);let writes=0;await page.route('**/v1/collections?*',r=>{if(r.request().method()!=='PUT')return r.fallback();writes++;return r.fulfill({status:409,json:{error_code:'collection_conflict'}});});
 const dialog=await openManager(page);await dialog.getByRole('button',{name:'Review changes'}).click();await dialog.getByRole('checkbox').check();await dialog.getByRole('button',{name:'Save changes'}).click();
 await expect(dialog.getByRole('alert')).toBeVisible();await expect(dialog.getByRole('button',{name:'Review changes'})).toBeDisabled();expect(writes).toBe(1);
 await dialog.getByRole('button',{name:collectionDestination('en').refresh}).click();await expect(dialog.getByRole('button',{name:'Review changes'})).toBeEnabled();await expect(dialog.getByRole('button',{name:'Save changes'})).toHaveCount(0);expect(writes).toBe(1);
});
test('unknown move outcome disables confirmation until source and destination are reviewed again',async({page})=>{
 await setup(page);let writes=0;await page.route('**/v1/documents/collection',r=>{writes++;return r.fulfill({status:503,json:{error_code:'unavailable'}});});
 const dialog=await openMove(page);await dialog.locator('.collection-destination select').selectOption(target.id);await dialog.getByRole('checkbox').check();await dialog.getByRole('button',{name:'Move to collection',exact:true}).click();
 await expect(dialog.getByRole('alert')).toBeVisible();await expect(dialog.getByRole('button',{name:'Move to collection',exact:true})).toBeDisabled();expect(writes).toBe(1);
 await dialog.getByRole('button',{name:collectionDestination('en').refresh}).click();await expect(dialog.locator('.collection-destination select')).toHaveValue('');expect(writes).toBe(1);
});

test('unchanged binding receipt does not claim a cross-collection move succeeded',async({page})=>{
 await setup(page);let writes=0;await page.route('**/v1/documents/collection',r=>{writes++;return r.fulfill({json:{document_id:'record',revision:8,collection:target}});});
 const dialog=await openMove(page);await dialog.locator('.collection-destination select').selectOption(target.id);await dialog.getByRole('checkbox').check();await dialog.getByRole('button',{name:'Move to collection',exact:true}).click();
 await expect(dialog.getByRole('alert')).toBeVisible();await expect(dialog.getByRole('button',{name:'Move to collection',exact:true})).toBeDisabled();expect(writes).toBe(1);
});
test('directory failure preserves unlisted members during rename',async({page})=>{
 const state=await setup(page);await page.route('**/v1/users',r=>r.fulfill({status:403,json:{error:'denied'}}));const dialog=await openManager(page);
 await expect(dialog.locator('.collection-members select').first()).toBeDisabled();await dialog.getByLabel('Collection name',{exact:true}).fill('Renamed');await dialog.getByRole('button',{name:'Review changes'}).click();await dialog.getByRole('checkbox').check();await dialog.getByRole('button',{name:'Save changes'}).click();
 await expect(dialog.getByRole('status').filter({hasText:'Changes saved'})).toBeVisible();expect(state.updates[0]).toEqual({expected_revision:4,name:'Renamed',readers:['reader'],editors:['editor']});
});
test('create is private and same-collection moves stay disabled',async({page})=>{
 const state=await setup(page);const dialog=await openManager(page);await dialog.locator('.collection-manager-list').getByRole('button',{name:collectionDestination('en').create,exact:true}).click();await dialog.getByLabel(collectionDestination('en').name,{exact:true}).last().fill('New private');await dialog.locator('.collection-create').getByRole('button',{name:collectionDestination('en').create,exact:true}).click();
 await expect(dialog.getByRole('status')).toContainText('Changes saved');expect(state.created).toEqual([{name:'New private'}]);await expect(dialog.locator('.collection-audience')).not.toContainText('reader');
 await page.keyboard.press('Escape');const move=await openMove(page);await move.locator('.collection-destination select').selectOption(source.id);await move.getByRole('checkbox').check();await expect(move.getByRole('button',{name:'Move to collection',exact:true})).toBeDisabled();expect(state.moves).toHaveLength(0);
});

test('library pagination bounds mounted cards and search resets the page',async({page})=>{
 await setup(page);await page.route('**/v1/documents',r=>r.fulfill({json:{documents:Array.from({length:65},(_,i)=>({id:`record-${i}`,name:`Record ${String(i).padStart(2,'0')}.txt`,size:128,index_status:'ready',source_retained:true,collection_id:source.id,collection_revision:1}))}}));
 await page.reload();await expect(page.locator('.resource-card')).toHaveCount(30);
 const pagination=page.locator('.library-pagination'),copy=documentReader('en');await pagination.getByRole('button',{name:copy.next}).click();await expect(page.locator('.resource-card').first()).toContainText('Record 30.txt');
 await pagination.getByRole('button',{name:copy.next}).click();await expect(page.locator('.resource-card')).toHaveCount(5);await expect(pagination.getByRole('button',{name:copy.next})).toBeDisabled();
 await page.locator('#knowledge-document-search').fill('Record 00');await expect(page.locator('.resource-card')).toHaveCount(1);await expect(page.locator('.resource-card')).toContainText('Record 00.txt');
 await page.locator('#knowledge-document-search').fill('');await expect(page.locator('.resource-card').first()).toContainText('Record 00.txt');
});

for(const locale of ['en','ar'] as const)test(`returning to library restores the filtered page and selection: ${locale}`,async({page})=>{
 await setup(page,locale,locale==='ar'?'dark':'light');
 const writes:string[]=[];page.on('request',request=>{if(request.method()!=='GET')writes.push(new URL(request.url()).pathname);});
 let count=65;
 await page.route('**/v1/documents',r=>r.fulfill({json:{documents:Array.from({length:count},(_,i)=>({id:`record-${i}`,name:`Record ${String(i).padStart(2,'0')}.txt`,size:128,index_status:'ready',source_retained:true,collection_id:source.id,collection_revision:1}))}}));
 await page.reload();await expect(page.locator('.resource-card')).toHaveCount(30);
 await page.locator('#knowledge-document-search').fill('Record');
 const pagination=page.locator('.library-pagination'),copy=documentReader(locale);
 await pagination.getByRole('button',{name:copy.next,exact:true}).click();
 await pagination.getByRole('button',{name:copy.next,exact:true}).click();
 await expect(page.locator('.resource-card')).toHaveCount(5);
 await page.locator('.resource-card').first().getByRole('checkbox').check();
 await page.evaluate(()=>location.hash='#/chat/new');
 await page.locator('.composer textarea').fill('Keep the document list context');
 await page.evaluate(()=>location.hash='#/knowledge');
 await expect(page.locator('#knowledge-document-search')).toHaveValue('Record');
 await expect(page.locator('.resource-card').first()).toContainText('Record 60.txt');
 await expect(page.locator('.resource-card').first().getByRole('checkbox')).toBeChecked();
 // A shorter fresh list must still show a valid page, with no empty phantom page.
 await page.evaluate(()=>location.hash='#/chat');count=35;
 await page.evaluate(()=>location.hash='#/knowledge');
 await expect(page.locator('.resource-card').first()).toContainText('Record 30.txt');
 await expect(page.locator('.resource-card')).toHaveCount(5);
 await expect(pagination.getByRole('button',{name:copy.next,exact:true})).toBeDisabled();
 await page.locator('#knowledge-document-search').fill('Record 0');
 await expect(page.locator('.resource-card').first()).toContainText('Record 00.txt');
 await page.locator('#knowledge-document-search').fill('Record');
 await expect(page.locator('.resource-card')).toHaveCount(30);
 await expect(page.locator('.resource-card').first()).toContainText('Record 00.txt');
 await page.evaluate(()=>location.hash='#/chat');
 await expect(page.locator('.composer textarea')).toHaveValue('Keep the document list context');
 expect(writes).toEqual([]);
});

test('member search is bounded and preserves unseen roles at 200% text size',async({page})=>{
 const state=await setup(page,'en','dark');await page.route('**/v1/users',r=>r.fulfill({json:{users:Array.from({length:100},(_,i)=>({id:`person-${i}`,username:`Person ${i}`,role:'user'}))}}));
 await page.setViewportSize({width:640,height:850});await page.evaluate(()=>{document.documentElement.style.fontSize='200%';});
 const dialog=await openManager(page);await expect(dialog.locator('.collection-members li')).toHaveCount(40);
 await dialog.getByRole('textbox',{name:'People',exact:true}).fill('person-99');await expect(dialog.locator('.collection-members li')).toHaveCount(1);
 await dialog.locator('.collection-members select').selectOption('reader');await dialog.getByRole('button',{name:'Review changes'}).click();
 expect(await dialog.evaluate(node=>node.scrollWidth<=node.clientWidth)).toBe(true);await dialog.getByRole('checkbox').check();await dialog.getByRole('button',{name:'Save changes'}).click();
 await expect(dialog.getByRole('status').filter({hasText:'Changes saved'})).toBeVisible();expect(state.updates[0].readers).toEqual(['reader','person-99']);expect(state.updates[0].editors).toEqual(['editor']);
});
