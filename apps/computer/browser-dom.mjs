// Functions in this module run inside the selected browser document. They must
// be self-contained: Playwright serializes them without their module scope.
export const controlSelector = 'button, input:not([type="hidden"]), textarea, select, a[href], [role="button"], [role="link"], [role="textbox"], [role="checkbox"], [role="radio"], [role="switch"], [role="combobox"], [role="option"], [role="tab"], [role="menuitem"], [contenteditable=""], [contenteditable="true"], [contenteditable="plaintext-only"]';

export function describeControl(el,query) {
  const root=el.getRootNode();
  const labelNodes=(el.getAttribute('aria-labelledby')||'').split(/\s+/).map(id=>root.getElementById?.(id)).filter(Boolean);
  const labelledBy=labelNodes.map(node=>node.textContent||'').join(' ');
  const nativeLabels=Array.from(el.labels||[]),labels=nativeLabels.map(label=>label.innerText).join(' ');
  const names=[el.type,el.name,el.id,el.autocomplete,el.getAttribute('aria-label'),labelledBy,labels,el.getAttribute('placeholder')].join(' ');
  let privateParent=false;
  for(let node=el;node;node=node.parentElement??node.getRootNode().host)if(node.hasAttribute('data-offgrid-private'))privateParent=true;
  const privateContent=node=>{
    if(node.hasAttribute?.('data-offgrid-private')||node.querySelector?.('[data-offgrid-private]'))return true;
    const walker=document.createTreeWalker(node,NodeFilter.SHOW_ELEMENT);
    if(node.shadowRoot&&privateContent(node.shadowRoot))return true;
    for(let child=walker.nextNode();child;child=walker.nextNode())if(child.shadowRoot&&privateContent(child.shadowRoot))return true;
    for(let parent=node.parentElement??node.getRootNode().host;parent;parent=parent.parentElement??parent.getRootNode().host)if(parent.hasAttribute('data-offgrid-private'))return true;
    return false;
  };
  const protectedLabel=[el,...labelNodes,...nativeLabels].some(privateContent);
  const tag=el.tagName.toLowerCase(), type=el.getAttribute('type')||'';
  const implicit=tag==='button'?'button':tag==='a'?'link':tag==='select'?'combobox':tag==='textarea'||el.isContentEditable?'textbox':tag==='input'?(type==='checkbox'?'checkbox':type==='radio'?'radio':['button','submit','reset'].includes(type)?'button':'textbox'):'';
  const role=el.getAttribute('role')||implicit;
  const label=el.getAttribute('aria-label')||labelledBy.trim()||labels.trim()||el.getAttribute('placeholder')||el.innerText||el.getAttribute('name')||'';
  const normalized=value=>value.normalize('NFC').toLowerCase().replace(/\s+/gu,' ').trim();
  return {tag,type,role,editable:el.isContentEditable,label:label.slice(0,160),
    matches:!query||(!query.role||query.role===role)&&(!query.name||normalized(label).includes(normalized(query.name)))&&(!query.text||normalized(el.innerText||'').includes(normalized(query.text))),
    sensitive:privateParent||protectedLabel||/password|passcode|secret|credential|token|username|one[- ]time[- ]code|verification[- ]code|credit[- ]card|card[- ]number|cc-number|cc-csc|cvv|cvc/i.test(names)};
}

export function targetState(el) {
  const attrs=node=>Array.from(node.attributes??[]).map(a=>[a.name,a.value]);
  const parents=[];
  for(let node=el.parentElement??el.getRootNode().host;node;node=node.parentElement??node.getRootNode().host)parents.push(attrs(node));
  const root=el.getRootNode();
  let form=el.form;
  for(let node=el;!form&&node;node=node.parentElement??node.getRootNode().host)if(node.tagName==='FORM')form=node;
  return {connected:el.isConnected,html:el.outerHTML,labels:Array.from(el.labels??[]).map(l=>l.textContent),
    labelledBy:(el.getAttribute('aria-labelledby')??'').split(/\s+/).map(id=>root.getElementById?.(id)?.textContent),
    ancestors:parents,form:form?[attrs(form),form.textContent]:null,title:document.title};
}

// Field properties can change without DOM mutations or input events. Include
// open shadow trees and semantic custom state; never return values to the model.
export function documentState() {
  const state=[];let visited=0;
  const walk=root=>{
    const walker=document.createTreeWalker(root,NodeFilter.SHOW_ELEMENT);
    for(let el=walker.nextNode();el;el=walker.nextNode()){
      if(++visited>50000)throw Error('Document exceeds structured observation bounds.');
      if(el.matches('input,textarea,select,[contenteditable],[role="checkbox"],[role="switch"],[role="combobox"],[role="option"]'))
        state.push([el.value,el.checked,el.selectedIndex,el.disabled,el.readOnly,el.getAttribute('aria-checked'),el.getAttribute('aria-selected'),el.getAttribute('aria-expanded'),el.isContentEditable?el.innerText:null]);
      if(el.shadowRoot)walk(el.shadowRoot);
    }
  };
  walk(document);return JSON.stringify(state);
}

export function installDocumentRevision() {
  window.__offgridRevision=0;
  const observed=new WeakSet();
  const watch=root=>{
    if(observed.has(root))return;observed.add(root);
    new MutationObserver(records=>{
      window.__offgridRevision++;
      for(const record of records)for(const node of record.addedNodes)scan(node);
    }).observe(root,{childList:true,subtree:true,characterData:true,attributes:true});
    for(const type of ['input','change'])root.addEventListener(type,()=>window.__offgridRevision++,true);
  };
  const scan=node=>{
    if(node.shadowRoot){watch(node.shadowRoot);scan(node.shadowRoot);}
    for(const child of node.children??[])scan(child);
  };
  watch(document);
  const attach=Element.prototype.attachShadow;
  Element.prototype.attachShadow=function(options){const root=attach.call(this,options);if(options.mode==='open')watch(root);return root;};
}
