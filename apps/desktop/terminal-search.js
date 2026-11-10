'use strict';
// Search owns its decorations and keyboard focus; it never sends shell input.
function mountTerminalSearch({current,tabs,theme,fit,changed}) {
 const field=id=>document.getElementById(id),bar=field('find'),query=field('query');
 const result=document.createElement('span');result.id='find-result';result.setAttribute('role','status');query.after(result);
 function clear(){for(const tab of tabs.values())tab.search.clearDecorations();result.textContent='';}
 function enabled(value){field('next').disabled=!value;field('previous').disabled=!value;}
 function find(previous=false,incremental=false){
  const tab=current();if(bar.hidden||!tab||!query.value){clear();enabled(false);return;}
  const palette=theme();
  tab.search[previous?'findPrevious':'findNext'](query.value,{incremental,decorations:{matchBackground:palette.selectionBackground,matchOverviewRuler:palette.foreground,activeMatchBackground:palette.selectionBackground,activeMatchBorder:palette.foreground,activeMatchColorOverviewRuler:palette.foreground}});
 }
 function attach(tab){tab.search.onDidChangeResults(value=>{
  if(tab!==current()||bar.hidden||!query.value)return;
  result.textContent=value.resultCount?(value.resultIndex<0?`${value.resultCount}+ matches`:`${value.resultIndex+1} of ${value.resultCount}`):'No matches';enabled(value.resultCount>0);
 });}
 function open(){bar.hidden=false;changed();fit(current());query.focus();find(false,true);}
 function close(){bar.hidden=true;clear();enabled(false);changed();fit(current());current()?.term.focus();}
 function select(){clear();find(false,true);}
 query.oninput=()=>find(false,true);query.onkeydown=event=>{if(event.key==='Enter'){event.preventDefault();find(event.shiftKey);}if(event.key==='Escape'){event.preventDefault();close();}};
 field('next').onclick=()=>find();field('previous').onclick=()=>find(true);field('find-close').onclick=close;field('search').onclick=open;enabled(false);
 return {open,attach,select,isOpen:()=>!bar.hidden};
}
