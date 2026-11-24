// Read visible text across open shadow roots without exposing protected editable
// values. Hard bounds reject oversized documents instead of claiming complete
// coverage over content we did not inspect. Values never enter metadata digests.
export function documentText(){
  const pieces=[];let visited=0,length=0;
  const sensitive=/password|passcode|secret|credential|token|username|one[- ]time[- ]code|verification[- ]code|credit[- ]card|card[- ]number|cc-number|cc-csc|cvv|cvc/i;
  const walk=node=>{
    if(++visited>50000)throw Error('Document exceeds text observation bounds.');
    if(node.nodeType===Node.TEXT_NODE){
      const text=node.textContent;length+=text.length;
      if(length>2*1024*1024)throw Error('Document exceeds text observation bounds.');
      pieces.push(text);return;
    }
    if(node.nodeType!==Node.ELEMENT_NODE&&node.nodeType!==Node.DOCUMENT_FRAGMENT_NODE)return;
    if(node.nodeType===Node.ELEMENT_NODE){
      if(node.matches('script,style,noscript,template,input,textarea,select,[data-offgrid-private]'))return;
      const style=getComputedStyle(node);if(style.display==='none'||style.visibility==='hidden'||style.visibility==='collapse')return;
      if(node.isContentEditable){
        const root=node.getRootNode();
        const labelled=(node.getAttribute('aria-labelledby')||'').split(/\s+/).map(id=>root.getElementById?.(id)?.textContent||'').join(' ');
        if(sensitive.test([node.id,node.getAttribute('aria-label'),node.getAttribute('name'),node.getAttribute('autocomplete'),labelled].join(' ')))return;
      }
      if(node.shadowRoot){walk(node.shadowRoot);return;}
      if(node.tagName==='SLOT'){
        const assigned=node.assignedNodes();if(assigned.length){for(const child of assigned)walk(child);return;}
      }
      if(node.tagName==='BR'){pieces.push('\n');return;}
      const block=!['inline','contents','inline-block'].includes(style.display);
      if(block)pieces.push('\n');
      for(const child of node.childNodes)walk(child);
      if(block)pieces.push('\n');return;
    }
    for(const child of node.childNodes)walk(child);
  };
  if(document.body)walk(document.body);
  return pieces.join('').trim();
}
