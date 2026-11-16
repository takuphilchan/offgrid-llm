'use strict';
const {ipcRenderer}=require('electron');
window.addEventListener('DOMContentLoaded',()=>{
  let content;
  const $=id=>document.getElementById(id);
  const answer=response=>{if(content)ipcRenderer.send('offgrid-dialog-answer',{response,checkboxChecked:$('remember').checked});};
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
    $('close').setAttribute('aria-label',value.buttons[value.cancelId]);$('close').onclick=()=>answer(value.cancelId);
    value.buttons.forEach((label,index)=>{
      const button=document.createElement('button');button.type='button';button.textContent=label;
      if(index!==value.cancelId)button.className='confirm';
      button.onclick=()=>answer(index);$('actions').append(button);
    });
    $('actions').children[value.defaultId]?.focus();
  });
  document.addEventListener('keydown',event=>{
    if(event.key==='Escape' && content){event.preventDefault();answer(content.cancelId);}
    if(event.key==='Tab'){
      const controls=[...document.querySelectorAll('button,input:not(:disabled)')].filter(el=>el.getClientRects().length);
      const index=controls.indexOf(document.activeElement),next=(index+(event.shiftKey?-1:1)+controls.length)%controls.length;
      event.preventDefault();controls[next]?.focus();
    }
  });
  ipcRenderer.send('offgrid-dialog-ready');
});
