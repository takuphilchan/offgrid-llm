import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {BrowserDriver,validateBrowserAction} from '../browser.mjs';

async function fixture(html,run){
 const server=createServer((req,res)=>{res.setHeader('Content-Type','text/html; charset=utf-8');res.end(html);});
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));let driver;
 try{driver=await BrowserDriver.open(`http://127.0.0.1:${server.address().port}`,{headless:true,testLoopback:true});await run(driver);}
 finally{await driver?.close();await new Promise(resolve=>server.close(resolve));}
}

test('literal Unicode search traverses hidden controls and preserves ambiguous exact identities',async()=>{
 await fixture(`${'<button hidden>Hidden</button>'.repeat(410)}<button>Cafe\u0301 語😀</button><button>Café 語😀</button><button>.*</button>`,async driver=>{
  let view=await driver.execute('browser_search',{role:'button',name:'CAFÉ 語😀',text:''});
  assert.equal(view.elements.length,0);assert.equal(view.controls_limited,true);
  const first=view;
  view=await driver.execute('browser_inspect',{observation_id:view.observation_id,cursor:view.next_cursor});
  assert.equal(view.elements.length,2);assert.notEqual(view.elements[0].id,view.elements[1].id);
  assert.deepEqual(view.search,{role:'button',name:'CAFÉ 語😀',text:''});
  assert.equal(view.coverage.controls_scanned,413);assert.equal(view.controls_limited,false);
  const other=await driver.execute('browser_search',{role:'button',name:'.*',text:''});
  await assert.rejects(driver.execute('browser_inspect',{observation_id:other.observation_id,cursor:first.next_cursor}),/cursor/);
  const literal=await driver.execute('browser_inspect',{observation_id:other.observation_id,cursor:other.next_cursor});
  assert.equal(literal.elements.length,1);assert.equal(literal.elements[0].label,'.*');
  assert.ok(Buffer.byteLength(JSON.stringify(literal))<48*1024);
 });
 assert.throws(()=>validateBrowserAction('browser_search',{role:'button[onclick]',name:'',text:''}));
 assert.throws(()=>validateBrowserAction('browser_search',{role:'',name:'',text:''}));
 assert.throws(()=>validateBrowserAction('browser_search',{role:'button',name:'ok',text:'',selector:'button'}));
});

const shadowPage=`<span id="label">Incorrect outer label</span><div id="host"></div><script>
const root=document.querySelector('#host').attachShadow({mode:'open'});
root.innerHTML='<span id="label">Message</span><div role="textbox" contenteditable="true" aria-labelledby="label">Original</div><input aria-label="Other field"><button>Apply</button>';
</script>`;

test('open shadow controls use root labels, exact Unicode editable read-back and silent-state freshness',async()=>{
 await fixture(shadowPage,async driver=>{
  let view=await driver.observe();let edit=view.elements.find(el=>el.role==='textbox'&&el.editable);
  assert.equal(edit.label,'Message');
  const text='First line 語😀\nSecond café';
  let result=await driver.execute('browser_fill',{observation_id:view.observation_id,element:edit.id,text});
  assert.equal(result.verified,true);assert.equal(await driver.page.locator('[contenteditable]').innerText(),text);
  view=await driver.observe();edit=view.elements.find(el=>el.editable);
  await driver.page.locator('input').evaluate(el=>el.value='silent change');
  await assert.rejects(driver.execute('browser_fill',{observation_id:view.observation_id,element:edit.id,text:'wrong'}),/Stale observation/);
  view=await driver.observe();edit=view.elements.find(el=>el.editable);
  await driver.page.locator('#host').evaluate(el=>el.setAttribute('data-owner','changed'));
  await assert.rejects(driver.execute('browser_fill',{observation_id:view.observation_id,element:edit.id,text:'wrong'}),/Stale observation/);
  view=await driver.observe();edit=view.elements.find(el=>el.editable);
  await driver.page.evaluate(()=>document.body.appendChild(document.createElement('hr')));
  result=await driver.execute('browser_fill',{observation_id:view.observation_id,element:edit.id,text:'Permitted after decoration'});
  assert.equal(result.verified,true);
 });
});

test('semantic switch read-back distinguishes completed, unchanged, mixed and mismatched states',async()=>{
 await fixture(`<div role="switch" tabindex="0" aria-label="Option" aria-checked="false" onclick="this.setAttribute('aria-checked',String(this.getAttribute('aria-checked')!=='true'))">Option</div>
<div role="checkbox" aria-label="Mixed" aria-checked="mixed">Mixed</div><div role="switch" aria-label="Broken" aria-checked="false">Broken</div>`,async driver=>{
  let view=await driver.observe(),control=view.elements.find(el=>el.label==='Option');
  let result=await driver.execute('browser_set_checked',{observation_id:view.observation_id,element:control.id,checked:true});
  assert.equal(result.verified,true);
  view=await driver.observe();control=view.elements.find(el=>el.label==='Option');
  result=await driver.execute('browser_set_checked',{observation_id:view.observation_id,element:control.id,checked:true});
  assert.equal(result.changed,false);assert.equal(result.dispatched,false);assert.equal(result.verified,true);
  view=await driver.observe();control=view.elements.find(el=>el.label==='Mixed');
  await assert.rejects(driver.execute('browser_set_checked',{observation_id:view.observation_id,element:control.id,checked:true}),/explicit boolean/);
  view=await driver.observe();control=view.elements.find(el=>el.label==='Broken');
  result=await driver.execute('browser_set_checked',{observation_id:view.observation_id,element:control.id,checked:true});
  assert.equal(result.verified,false);assert.equal(result.verification_status,'mismatch');assert.equal(result.effect_status,'dispatched');
 });
});

test('dynamic combobox exposes freshly rendered options and stale choices cannot act',async()=>{
 await fixture(`<button role="combobox" aria-expanded="false">Choices</button><div id="options"></div><script>
document.querySelector('button').onclick=function(){this.setAttribute('aria-expanded','true');document.querySelector('#options').innerHTML='<div role="option" tabindex="0">Choice 語</div>';};</script>`,async driver=>{
  const view=await driver.observe(),combo=view.elements.find(el=>el.role==='combobox');
  assert.equal(combo.expanded,false);
  const next=await driver.execute('browser_click',{observation_id:view.observation_id,element:combo.id});
  const option=next.elements.find(el=>el.role==='option');assert.equal(option.label,'Choice 語');
  assert.equal((await driver.prepare('browser_click',{observation_id:next.observation_id,element:option.id})).approval_class,'consequential');
  await driver.page.locator('[role=option]').evaluate(el=>el.outerHTML=el.outerHTML);
  await assert.rejects(driver.execute('browser_click',{observation_id:next.observation_id,element:option.id}),/Stale observation/);
 });
});

test('shadow credentials and explicitly private editable controls are omitted',async()=>{
 await fixture(`<div id="host"></div><script>document.querySelector('#host').attachShadow({mode:'open'}).innerHTML='<label>Verification code<input></label><div data-offgrid-private><div contenteditable=true>private value</div></div><input aria-label="Public field">'</script>`,async driver=>{
  const view=await driver.observe();assert.deepEqual(view.elements.map(el=>el.label),['Public field']);assert.ok(!view.text.includes('private value'));
 });
});

test('search uses full labels, binds its cursor and keeps dense Unicode replies bounded',async()=>{
 await fixture(`<p>${'語😀'.repeat(5000)}</p><button>${'Long label '.repeat(50)}EXACT-SUFFIX</button>${Array.from({length:95},(_,i)=>`<button>${'語😀'.repeat(100)} ${i}</button>`).join('')}`,async driver=>{
  let view=await driver.execute('browser_search',{role:'button',name:'EXACT-SUFFIX',text:''});assert.equal(view.elements.length,1);
  assert.ok(!view.elements[0].label.includes('EXACT-SUFFIX'),'search matches beyond the bounded display label');
  view=await driver.execute('browser_search',{role:'button',name:'語😀',text:''});
  let total=0;
  for(let calls=0;;calls++){
   assert.ok(calls<30);assert.ok(Buffer.byteLength(JSON.stringify(view))<48*1024);total+=view.elements.length;
   if(!view.next_cursor)break;
   view=await driver.execute('browser_inspect',{observation_id:view.observation_id,cursor:view.next_cursor});
  }
  assert.equal(total,95);
  view=await driver.execute('browser_search',{role:'button',name:'語😀',text:''});
  await driver.page.locator('button').first().evaluate(el=>el.textContent='changed');
  await assert.rejects(driver.execute('browser_inspect',{observation_id:view.observation_id,cursor:view.next_cursor}),/Stale observation/);
 });
});

test('contenteditable read-back failure retains dispatched input and host replacement invalidates references',async()=>{
 await fixture(shadowPage,async driver=>{
  let view=await driver.observe(),edit=view.elements.find(el=>el.editable);
  const handle=driver.elements.get(edit.id);handle.innerText=async()=>{throw Error('readback transport lost');};
  const result=await driver.execute('browser_fill',{observation_id:view.observation_id,element:edit.id,text:'Applied once'});
  assert.equal(result.dispatched,true);assert.equal(result.verified,false);assert.equal(result.verification_status,'unavailable');
  assert.equal(await driver.page.locator('[contenteditable]').innerText(),'Applied once');
  view=await driver.observe();edit=view.elements.find(el=>el.editable);
  await driver.page.locator('#host').evaluate(el=>{const next=document.createElement('div');next.id='host';next.attachShadow({mode:'open'}).innerHTML=el.shadowRoot.innerHTML;el.replaceWith(next);});
  await assert.rejects(driver.execute('browser_fill',{observation_id:view.observation_id,element:edit.id,text:'Wrong host'}),/Stale observation/);
 });
});

test('every advertised explicit role is searchable and private descendant/label text cannot leak',async()=>{
 const roles=['button','link','textbox','checkbox','radio','switch','combobox','option','tab','menuitem'];
 await fixture(`${roles.map(role=>`<div role="${role}" aria-label="Reviewed ${role}">Public</div>`).join('')}
<button>Visible <span data-offgrid-private>REVIEW-PRIVATE-SENTINEL</span></button>
<label>Visible <span data-offgrid-private>LABEL-PRIVATE-SENTINEL</span><input></label>
<span id="named" data-offgrid-private>ARIA-PRIVATE-SENTINEL</span><button aria-labelledby="named">Public</button>`,async driver=>{
  const view=await driver.observe();assert.equal(view.elements.length,roles.length);assert.ok(!JSON.stringify(view).includes('PRIVATE-SENTINEL'));
  for(const role of roles){const found=await driver.execute('browser_search',{role,name:'Reviewed '+role,text:''});assert.equal(found.elements.length,1);assert.equal(found.elements[0].role,role);}
 });
});
