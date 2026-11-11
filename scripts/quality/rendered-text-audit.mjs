// Runs inside an isolated browser document. No input values or terminal buffer
// are collected: the report contains authored UI text, not users' work.
export function renderedText() {
 const rows=[];
 const normalize=value=>value.replace(/\s+/gu,' ').trim();
 const visible=element=>element.checkVisibility({checkVisibilityCSS:true})&&!element.closest('script,style,[aria-hidden="true"],.xterm-screen,.xterm-accessibility,.xterm-helpers');
 const location=element=>element.id?'#'+element.id:[...function*(node){for(let depth=0;node&&depth<5;node=node.parentElement,depth++)yield node.localName+(node.classList.length?'.'+[...node.classList].join('.'):'');}(element)].reverse().join(' > ');
 const add=(element,kind,value)=>{const text=normalize(value||'');if(text)rows.push({element:location(element),kind,text});};
 const walker=document.createTreeWalker(document.body,NodeFilter.SHOW_TEXT);
 for(let node;node=walker.nextNode();){
  const parent=node.parentElement;if(!parent||!visible(parent)||parent.closest('textarea,input,option'))continue;
  const range=document.createRange();range.selectNodeContents(node);
  if([...range.getClientRects()].some(rect=>rect.width&&rect.height))add(parent,'text',node.textContent);
 }
 for(const element of document.body.querySelectorAll('*')){
  if(!visible(element))continue;
  for(const name of ['aria-label','title','placeholder','alt'])if(element.hasAttribute(name))add(element,name,element.getAttribute(name));
  for(const name of ['aria-labelledby','aria-describedby'])for(const id of (element.getAttribute(name)||'').split(/\s+/)){
   if(id)add(element,name,document.getElementById(id)?.textContent);
  }
  if(element instanceof HTMLSelectElement)for(const option of element.options)add(element,'option',option.textContent);
 }
 return rows;
}

const normalize=value=>value.replace(/\s+/gu,' ').trim();
const escape=value=>value.replace(/[.*+?^${}()|[\]\\]/g,'\\$&');
function pattern(value){return new RegExp('^'+value.split(/(\{[A-Za-z][\w]*\})/).map(part=>/^\{/.test(part)?'.+?':escape(part)).join('')+'$','u');}
export function classifyRendered(rows,entries,language,exemptions=[]){
 const exact=new Map(),dynamic=[];
 for(const entry of entries){
  const text=normalize(entry.currentText),source=normalize(entry.source);
  const reference={namespace:entry.namespace,key:entry.key,source:entry.source,coverage:entry.coverage,context:entry.context};
  if(/\{[A-Za-z][\w]*\}/.test(text))dynamic.push({match:pattern(text),reference});
  else{if(!exact.has(text))exact.set(text,[]);exact.get(text).push(reference);}
  // Also catch stale English left on screen despite an authored translation.
  if(text!==source){if(!exact.has(source))exact.set(source,[]);exact.get(source).push({...reference,coverage:'stale-source'});}
 }
 return rows.map(row=>{
  const references=exact.get(normalize(row.text))??dynamic.filter(item=>item.match.test(normalize(row.text))).map(item=>item.reference);
  const exemption=exemptions.find(item=>item.text===row.text&&(!item.element||row.element.includes(item.element)));
  const status=exemption?'exempt':references?.length
   ?references.every(item=>['missing','stale-source'].includes(item.coverage))?'untranslated':'catalogued'
   :/^[\p{Number}\p{Punctuation}\p{Symbol}\s]+$/u.test(row.text)?'nonlinguistic':'unclassified';
  return {...row,language,status,references:references??[],...(exemption?{reason:exemption.reason}:{})};
 });
}
