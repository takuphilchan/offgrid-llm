'use strict';
const {ipcRenderer}=require('electron');
window.addEventListener('DOMContentLoaded',()=>{
  let content;
  const $=id=>document.getElementById(id);
  let layoutFrame=0;
  const fit=()=>{
    cancelAnimationFrame(layoutFrame);
    layoutFrame=requestAnimationFrame(()=>{
      if(!content)return;
      const main=document.querySelector('main'),style=getComputedStyle(main);
      const height=[...main.children].reduce((sum,node)=>sum+Math.max(node.scrollHeight,node.getBoundingClientRect().height),0)
        +parseFloat(style.paddingTop)+parseFloat(style.paddingBottom)+parseFloat(style.rowGap)*2+2;
      ipcRenderer.send('offgrid-dialog-layout',height);
    });
  };
  const observer=new ResizeObserver(fit);
  document.querySelectorAll('main > *, .body > *').forEach(node=>observer.observe(node));
  window.addEventListener('resize',fit);
  document.fonts.ready.then(fit);
  const answer=response=>{
    if(!content)return;
    const secret=$('secret').value;
    if(content.secret&&response!==content.cancelId&&(!secret||new TextEncoder().encode(secret).length>4096||/[\x00\r\n]/.test(secret)||content.secret.confirm&&secret!==$('secret-confirm').value)){
      $('secret-error').textContent=content.secret.invalid;$('secret-error').hidden=false;$('secret').focus();return;
    }
    ipcRenderer.send('offgrid-dialog-answer',{response,checkboxChecked:$('remember').checked,...(content.secret&&response!==content.cancelId?{secret}:{})});
    $('secret').value='';$('secret-confirm').value='';
  };
  ipcRenderer.once('offgrid-dialog-content',(_event,value)=>{
    content=value;
    document.documentElement.dataset.theme=value.theme;
    document.documentElement.dataset.reduceMotion=String(value.reduceMotion===true);
    document.documentElement.lang=value.locale || 'en';
    document.documentElement.dir=value.direction==='rtl'?'rtl':'ltr';
    for(const key of ['surface-1','surface-2','text','muted','line','accent','on-accent']) {
      if(typeof value.palette?.[key]==='string')document.documentElement.style.setProperty('--'+key,value.palette[key]);
    }
    $('title').textContent=value.title || value.message || 'OffGrid';
    $('message').textContent=value.title && value.title!==value.message ? value.message || '' : '';
    $('message').hidden=!$('message').textContent;
    $('detail').textContent=value.detail || '';$('detail').hidden=!value.detail;
    $('remember-row').hidden=!value.checkboxLabel;
    $('remember-label').textContent=value.checkboxLabel || '';$('remember').checked=value.checkboxChecked;
    $('secret-fields').hidden=!value.secret;
    $('secret-label').textContent=value.secret?.label||'';
    $('secret-confirm-label').textContent=value.secret?.confirmLabel||'';
    $('secret-confirm-row').hidden=!value.secret?.confirm;
    fit();
    $('close').setAttribute('aria-label',value.buttons[value.cancelId]);$('close').onclick=()=>answer(value.cancelId);
    value.buttons.forEach((label,index)=>{
      const button=document.createElement('button');button.type='button';button.textContent=label;
      if(index!==value.cancelId)button.className='confirm';
      button.onclick=()=>answer(index);$('actions').append(button);
    });
    if(value.secret)$('secret').focus();else $('actions').children[value.defaultId]?.focus();
  });
  document.addEventListener('keydown',event=>{
    if(event.key==='Escape' && content){event.preventDefault();answer(content.cancelId);}
    if(event.key==='Enter'&&content?.secret&&event.target instanceof HTMLInputElement){event.preventDefault();answer(content.buttons.findIndex((_label,index)=>index!==content.cancelId));}
    if(event.key==='Tab'){
      const controls=[...document.querySelectorAll('button,input:not(:disabled)')].filter(el=>el.getClientRects().length);
      const index=controls.indexOf(document.activeElement),next=(index+(event.shiftKey?-1:1)+controls.length)%controls.length;
      event.preventDefault();controls[next]?.focus();
    }
  });
  ipcRenderer.send('offgrid-dialog-ready');
});
