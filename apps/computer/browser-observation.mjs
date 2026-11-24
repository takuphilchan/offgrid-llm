import {createHash, randomUUID} from 'node:crypto';
import {controlSelector, targetState} from './browser-dom.mjs';
import {documentText} from './browser-text.mjs';

const digest = value => createHash('sha256').update(JSON.stringify(value)).digest('hex');

export async function targetDigest(handle) {return digest(await handle.evaluate(targetState));}

async function optionsPage(driver,handle,id,offset=0) {
  const data=await handle.evaluate((el,offset)=>({total:el.options.length,multiple:el.multiple,options:Array.from(el.options).slice(offset,offset+100).map((o,i)=>({id:String(offset+i+1),label:o.label.slice(0,160),disabled:o.disabled||(o.parentElement.tagName==='OPTGROUP'&&o.parentElement.disabled),selected:o.selected}))}),offset);
  while(Buffer.byteLength(JSON.stringify(data.options))>24000) data.options.pop();
  const observed=driver.observedOptions.get(id)??new Set();
  for(const option of data.options)observed.add(option.id);
  driver.observedOptions.set(id,observed);
  const result={multiple:data.multiple,options:data.options,options_limited:offset+data.options.length<data.total};
  if(result.options_limited) {
    result.options_cursor=randomUUID();driver.cursors.set(result.options_cursor,{options:id,offset:offset+data.options.length});
  }
  return result;
}

export async function assertFresh(driver, args, wholePage=false) {
  const frame=await driver.frames.assertScope();
  if (!driver.observation || args.observation_id!==driver.observation || Date.now()-driver.observedAt>120000 ||
      frame!==driver.observedFrame||frame.url()!==driver.observedURL || await driver.stateDigest()!==driver.formDigest) throw Error('Stale observation; inspect the page again.');
  let sameDocument=false;
  try {sameDocument=await driver.observedDocument?.evaluate(doc=>doc===document);}catch{/* replaced execution context */}
  if(!sameDocument)throw Error('Stale observation; inspect the page again.');
  if (wholePage && await frame.evaluate(()=>window.__offgridRevision)!==driver.revision) throw Error('Stale observation; inspect the page again.');
  if (args.element) {
    const handle=driver.elements.get(args.element);
    if (!handle || !await handle.evaluate(el=>el.isConnected) ||
        (driver.targetDigests?.has(args.element) && await targetDigest(handle)!==driver.targetDigests.get(args.element))) throw Error('Stale observation; inspect the page again.');
  }
}

// A cursor is an issued opaque reference, not model-controlled offsets. Every
// response stays below the hub's 48KiB result limit, including Unicode/escaping.
export async function observePage(driver, describeControl, continuation, query) {
  if(driver.page.isClosed())throw Error('Target unavailable or outside scope.');
  const page=await driver.frames.assertScope();
  const frameEpoch=driver.frames.epoch;
  if (continuation) await assertFresh(driver,continuation,true);
  const position=continuation ? driver.cursors.get(continuation.cursor) : {control:0,text:0};
  if (!position) throw Error('Stale or unknown observation cursor.');
  if(position.options) {
    await assertFresh(driver,{...continuation,element:position.options},true);
    const options=await optionsPage(driver,driver.elements.get(position.options),position.options,position.offset);
    await assertFresh(driver,{...continuation,element:position.options},true);
    return {observation_id:driver.observation,element:position.options,...options};
  }
  if (!continuation) {
    await Promise.allSettled(Array.from(driver.elements.values(),handle=>handle.dispose()));
    await driver.observedDocument?.dispose().catch(()=>{});
    driver.observedDocument=await page.evaluateHandle(()=>document);
    driver.elements.clear();driver.targetDigests=new Map();driver.cursors=new Map();driver.observedOptions=new Map();
    driver.observation='';
    driver.searchQuery=query;
  }
  query=driver.searchQuery;
  const observedURL=page.url(), initialState=await driver.stateDigest();
  const revision=await page.evaluate(()=>window.__offgridRevision);
  const controls=page.locator(controlSelector), count=await controls.count();
  const visibleText=await page.evaluate(documentText);
  const textPage={text:visibleText.slice(position.text,position.text+6001),total:visibleText.length};
  const rawText=textPage.text;
  textPage.text=rawText.slice(0,6000);
  while (Buffer.byteLength(JSON.stringify(textPage.text))>8000) textPage.text=textPage.text.slice(0,Math.floor(textPage.text.length*0.8));
  // Do not split a UTF-16 surrogate pair across continuation boundaries.
  // A malformed unpaired surrogate must still advance the cursor.
  const nextCode=rawText.charCodeAt(textPage.text.length);
  if (/[\uD800-\uDBFF]$/.test(textPage.text) && nextCode>=0xdc00 && nextCode<=0xdfff) textPage.text=textPage.text.slice(0,-1);
  textPage.end=position.text+textPage.text.length;
  const result={observation_id:continuation ? driver.observation : randomUUID(),url:observedURL.slice(0,2048),url_limited:observedURL.length>2048,title:(await page.title()).slice(0,256),text:textPage.text,elements:[],blocked_origins:Array.from(driver.blockedOrigins??[]).map(origin=>origin.slice(0,256))};
  const normalized=value=>value.normalize('NFC').toLowerCase().replace(/\s+/gu,' ').trim();
  if(query){
    result.search=query;
    result.text_match=!!query.text&&normalized(textPage.text).includes(normalized(query.text));
    // Text chunks remain complete and bounded so literal matches spanning page
    // boundaries can be checked without pretending the scan covered all text.
  }
  if(driver.tabID) Object.assign(result,{tab_id:driver.tabID,document_id:driver.documentID});
  Object.assign(result,driver.frames.provenance());
  let index=position.control;
  // Scan at most 400 candidates per page, including hidden/sensitive controls.
  // Even an empty page can continue, so hidden controls never hide later work.
  for (;index<count && index<position.control+400 && result.elements.length<80;index++) {
    const handle=await controls.nth(index).elementHandle();
    if (!handle) throw Error('observation_changed');
    let retained=false;
    try {
      if (!await handle.isVisible()) continue;
      const before=await targetDigest(handle);
      const info=await handle.evaluate(describeControl,query);
      if (info.sensitive) continue;
      if(!info.matches)continue;
      const state=await handle.evaluate(el=>({disabled:el.matches(':disabled')||el.getAttribute('aria-disabled')==='true',
        ...(el.tagName==='INPUT'&&el.type==='checkbox'?{checked:el.checked}:['checkbox','switch'].includes(el.getAttribute('role'))?{checked:el.getAttribute('aria-checked')==='true'?true:el.getAttribute('aria-checked')==='false'?false:'unsupported'}:{}),
        ...(el.hasAttribute('aria-expanded')?{expanded:el.getAttribute('aria-expanded')==='true'}:{})}));
      if(info.tag==='select')Object.assign(state,await optionsPage(driver,handle,String(index+1)));
      let href;
      if(info.tag==='a') {
        try {const link=new URL(await handle.getAttribute('href'),observedURL);if(driver.allows(link.href)&&link.href.length<=2048)href=link.href;}catch{/* no unsafe destinations */}
      }
      const element={id:String(index+1),tag:info.tag,type:info.type,role:info.role,label:info.label,...(info.editable?{editable:true}:{}),...(href?{href}:{}),...state};
      if(Buffer.byteLength(JSON.stringify({...result,elements:[...result.elements,element]}))>40000) break;
      if(before!==await targetDigest(handle)) throw Error('observation_changed');
      driver.elements.set(element.id,handle);driver.targetDigests.set(element.id,before);retained=true;
      result.elements.push(element);
    } finally {if(!retained)await handle.dispose();}
  }
  if(page.url()!==observedURL || initialState!==await driver.stateDigest() ||
      (continuation && revision!==await page.evaluate(()=>window.__offgridRevision))) {
    driver.observation='';throw Error('observation_changed');
  }
  await driver.frames.assertScope(page);
  let sameDocument=false;
  try{sameDocument=await driver.observedDocument?.evaluate(doc=>doc===document);}catch{/* replaced execution context */}
  if(!sameDocument||frameEpoch!==driver.frames.epoch){driver.observation='';throw Error('observation_changed');}
  driver.observation=result.observation_id;driver.observedFrame=page;driver.observedURL=observedURL;driver.observedAt=Date.now();driver.formDigest=initialState;driver.revision=revision;
  result.controls_limited=index<count;result.text_limited=textPage.end<textPage.total;
  result.coverage={controls_scanned:index,controls_total:count,closed_shadow_roots:'unsupported'};
  if(result.controls_limited||result.text_limited) {
    result.next_cursor=randomUUID();driver.cursors.set(result.next_cursor,{control:index,text:textPage.end});
  }
  return result;
}
