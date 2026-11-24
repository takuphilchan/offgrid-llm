import {createHash} from 'node:crypto';
import {controlSelector,describeControl,documentState} from './browser-dom.mjs';
import {assertCaptureShadowCoverage} from './browser-shadow-coverage.mjs';
import {captureWitness} from './browser-capture-witness.mjs';
import {maskBrowserPNG,maskRectangles} from './browser-png-mask.mjs';

const stateHash=async frame=>createHash('sha256').update(await frame.evaluate(documentState)).digest('hex');
const markupHash=async frame=>createHash('sha256').update(await frame.evaluate(()=>{
  let visited=0,total=0;const markup=[];
  const read=root=>{
    const html=root===document?document.documentElement.outerHTML:root.innerHTML;
    total+=html.length;if(total>4*1024*1024)throw Error('Capture markup exceeds bounds.');markup.push(html);
    const walker=document.createTreeWalker(root,NodeFilter.SHOW_ELEMENT);
    for(let node=walker.nextNode();node;node=walker.nextNode()){
      if(++visited>50000)throw Error('Capture document exceeds bounds.');
      if(node.shadowRoot)read(node.shadowRoot);
    }
  };
  read(document);return JSON.stringify(markup);
})).digest('hex');

// Mask in every readable frame, including open shadow controls. When a child
// document's origin cannot be established, mask its embedding surface without
// inspecting its contents. A changed tree/document invalidates the whole capture.
export async function captureBrowser(driver){
  const {page,frames}=driver,viewport=page.viewportSize();
  if(!viewport||viewport.width<1||viewport.height<1||viewport.width>8192||viewport.height>8192||viewport.width*viewport.height>32*1024*1024)throw Error('Browser surface exceeds the vision safety limit.');
  const epoch=frames.epoch,origins=await frames.origins(),mask=[],snapshots=[],wholeParents=new Set();
  let controls=0,witness,raw;
  try{
    witness=await captureWitness(frames,origins);
    await assertCaptureShadowCoverage(frames,origins);
    for(const frame of frames.entries.keys()){
      if(!frames.permitted(frame,origins)){
        let parent=frame.parentFrame();
        while(parent&&!frames.permitted(parent,origins))parent=parent.parentFrame();
        if(!parent)throw Error('Capture cannot establish private frame coverage.');
        // Conservatively mask all child surfaces at this ancestor, avoiding
        // selector reconstruction or ambiguous iframe names/URLs.
        if(!wholeParents.has(parent)){mask.push({frame:parent,locator:parent.locator('iframe,frame')});wholeParents.add(parent);}
        continue;
      }
      const snapshot={frame,document:await frame.evaluateHandle(()=>document),markup:await markupHash(frame),state:await stateHash(frame)};
      snapshots.push(snapshot);
      mask.push({frame,locator:frame.locator('[data-offgrid-private], [data-offgrid-private] *')});
      const candidates=frame.locator(controlSelector),count=await candidates.count();controls+=count;
      if(controls>1000)throw Error('Capture exceeds the protected-control inspection bound.');
      for(let index=0;index<count;index++)if((await candidates.nth(index).evaluate(describeControl)).sensitive){
        const locator=candidates.nth(index);mask.push({frame,locator},{frame,locator:locator.locator('*')});
      }
    }
    if(epoch!==frames.epoch)throw Error('Frame tree changed before capture.');
    const rectangles=await maskRectangles(mask);
    await witness.assertUnchanged();
    // Preserve the document's styles. Hiding a caret rewrites/normalizes input
    // style attributes even after Playwright restores them, invalidating exact
    // markup checks. Protected editables are masked independently of caret state.
    raw=await page.screenshot({type:'png',animations:'allow',caret:'initial'});
    if(!raw.length||raw.length>5*1024*1024)throw Error('Browser capture exceeds the vision safety limit.');
    await witness.assertUnchanged();
    if(epoch!==frames.epoch)throw Error('Frame tree changed during capture.');
    await assertCaptureShadowCoverage(frames,await frames.origins());
    for(const snapshot of snapshots){
      if(!await snapshot.document.evaluate(doc=>doc===document)||snapshot.markup!==await markupHash(snapshot.frame)||snapshot.state!==await stateHash(snapshot.frame))throw Error('Document changed during protected capture. Inspect again.');
    }
    if(JSON.stringify(rectangles)!==JSON.stringify(await maskRectangles(mask)))throw Error('Protected capture geometry changed. Inspect again.');
    await witness.assertUnchanged();
    await frames.assertScope();
    const content=maskBrowserPNG(raw,viewport,rectangles);
    if(content.length>5*1024*1024)throw Error('Masked capture exceeds the vision safety limit.');
    return {captured:true,observation_id:driver.observation,...frames.provenance(),media_type:'image/png',width:viewport.width,height:viewport.height,data:content.toString('base64'),privacy:{protected_controls_masked:true,whole_frame_groups_masked:wholeParents.size,closed_shadow_roots:'unsupported'}};
  }finally{raw?.fill(0);await witness?.dispose();await Promise.allSettled(snapshots.map(snapshot=>snapshot.document.dispose()));}
}
