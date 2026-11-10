'use strict';
(function (root) {
 const defaults=Object.freeze({fontSize:14,lineHeight:1.25,cursorStyle:'block',cursorBlink:true,scrollback:5000});
 const colors=['black','red','green','yellow','blue','magenta','cyan','white','brightBlack','brightRed','brightGreen','brightYellow','brightBlue','brightMagenta','brightCyan','brightWhite'];
 const palettes={
  light:['#242424','#a52028','#286639','#826000','#2858a8','#813980','#006870','#555555','#666666','#b12424','#276039','#735c00','#2858a8','#813980','#006870','#242424'],
  dark:['#9b9b9b','#ff8585','#a3cc88','#e4c779','#82aaff','#d7a0e8','#7fd7d2','#d4d4d4','#aaaaaa','#ffa1a1','#b7df9c','#f0d88d','#aac6ff','#ebbcf7','#9ae7e2','#ffffff']
 };
 function normalize(value){
  value=value&&typeof value==='object'?value:{};
  const number=(key,min,max,step)=>typeof value[key]==='number'&&Number.isFinite(value[key])?Math.round(Math.max(min,Math.min(max,value[key]))/step)*step:defaults[key];
  return {fontSize:number('fontSize',11,24,1),lineHeight:number('lineHeight',1,1.8,.05),cursorStyle:['block','bar','underline'].includes(value.cursorStyle)?value.cursorStyle:defaults.cursorStyle,cursorBlink:typeof value.cursorBlink==='boolean'?value.cursorBlink:defaults.cursorBlink,scrollback:number('scrollback',1000,20000,1000)};
 }
 function load(storage){try{const saved=JSON.parse(storage.getItem('offgrid.terminal.display'));return normalize(saved?.version===1?saved.settings:null);}catch{return normalize(null);}}
 function save(storage,value){try{storage.setItem('offgrid.terminal.display',JSON.stringify({version:1,settings:normalize(value)}));return true;}catch{return false;}}
 function theme(dark,system){
  const background=dark?'#191919':'#ffffff',foreground=dark?'#e6e6e6':'#242424';
  if(system)return {background:system.background,foreground:system.foreground,cursor:system.foreground,cursorAccent:system.background,selectionBackground:system.foreground,selectionForeground:system.background,...Object.fromEntries(colors.map(key=>[key,system.foreground]))};
  return {background,foreground,cursor:foreground,cursorAccent:background,selectionBackground:dark?'#555555':'#cbd7ef',selectionForeground:foreground,...Object.fromEntries(colors.map((key,i)=>[key,palettes[dark?'dark':'light'][i]]))};
 }
 function options(value,reducedMotion){const settings=normalize(value);return {...settings,cursorBlink:settings.cursorBlink&&!reducedMotion};}
 const api={defaults,colors,normalize,load,save,theme,options};
 if(typeof module!=='undefined'&&module.exports)module.exports=api;
 else root.terminalPreferences=api;
})(globalThis);
