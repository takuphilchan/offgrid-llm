'use strict';
// Search owns its decorations and keyboard focus; it never sends shell input.
function mountTerminalSearch({current,tabs,theme,fit,changed}) {
 const field=id=>document.getElementById(id),bar=field('find'),query=field('query');
 const result=document.createElement('span');result.id='find-result';result.setAttribute('role','status');query.after(result);
 let results=null;
 function clear(){for(const tab of tabs.values())tab.search.clearDecorations();results=null;result.textContent='';}
 function enabled(value){field('next').disabled=!value;field('previous').disabled=!value;}
 function find(previous=false,incremental=false){
  const tab=current();if(bar.hidden||!tab||!query.value){clear();enabled(false);return;}
  const palette=theme();
  tab.search[previous?'findPrevious':'findNext'](query.value,{incremental,decorations:{matchBackground:palette.selectionBackground,matchOverviewRuler:palette.foreground,activeMatchBackground:palette.selectionBackground,activeMatchBorder:palette.foreground,activeMatchColorOverviewRuler:palette.foreground}});
 }
 function attach(tab){tab.search.onDidChangeResults(value=>{
  if(tab!==current()||bar.hidden||!query.value)return;
  results=value;localize();enabled(value.resultCount>0);
 });}
 function localize(){if(!results||bar.hidden||!query.value)return;
  const {resultCount,resultIndex}=results,count=resultCount.toLocaleString(document.documentElement.lang),index=(resultIndex+1).toLocaleString(document.documentElement.lang);
  result.textContent=resultCount?(resultIndex<0?terminalText('{count}+ matches',{count}):terminalText('{index} of {count}',{index,count})):terminalText('No matches');
 }
 function open(){bar.hidden=false;changed();fit(current());query.focus();find(false,true);}
 function close(){bar.hidden=true;clear();enabled(false);changed();fit(current());current()?.term.focus();}
 function select(){clear();find(false,true);}
 query.oninput=()=>find(false,true);query.onkeydown=event=>{if(event.key==='Enter'){event.preventDefault();find(event.shiftKey);}if(event.key==='Escape'){event.preventDefault();close();}};
 field('next').onclick=()=>find();field('previous').onclick=()=>find(true);field('find-close').onclick=close;field('search').onclick=open;enabled(false);
 return {open,attach,select,localize,isOpen:()=>!bar.hidden};
}
