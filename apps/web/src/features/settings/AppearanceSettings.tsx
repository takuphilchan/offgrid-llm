import {useEffect,useRef,useState,type CSSProperties} from 'react';
import {useI18n} from '../../i18n';
import {personalization} from '../../i18n/personalization';
import {themeChooser} from '../../i18n/theme-chooser';
import {Icon} from '../../components/Icon';
import {restoreFocus} from '../../lib/restore-focus';
import {reportPreferenceSave} from '../../lib/preferences';
import {usePreferenceSaveStates} from '../../lib/usePreferenceSaveStates';
import {appearanceDefaults,applyAppearance,availablePalettes,paletteTokens,paletteLabel,readAppearance,saveAppearance,type Appearance,type Palette,type PaletteMode} from '../../appearance';

export function ThemeSample({id,mode}:{id:Palette;mode:PaletteMode}) {
 const t=paletteTokens(id,mode);
 const style={'--preview-bg':t['surface-0'],'--preview-panel':t['surface-1'],'--preview-fill':t['surface-2'],'--preview-text':t.text,'--preview-muted':t.muted,'--preview-accent':t.accent} as CSSProperties;
 return <span className="theme-workspace-preview" style={style} aria-hidden="true">
  <span className="theme-preview-sidebar"><i/><i/><i/><i/></span>
  <span className="theme-preview-main"><span className="theme-preview-title"/><span className="theme-preview-bubble"/><span className="theme-preview-lines"><i/><i/></span><span className="theme-preview-composer"><i/></span></span>
 </span>;
}

function ThemeChooser({initial,initialMode,onApply,onClose}:{initial:Appearance;initialMode:PaletteMode;onApply:(value:Appearance)=>void;onClose:()=>void}) {
 const {locale,language}=useI18n(),c=personalization[locale],copy=themeChooser[locale];
 const [draft,setDraft]=useState(initial),[mode,setMode]=useState(initialMode),[query,setQuery]=useState('');
 const dialog=useRef<HTMLDialogElement>(null);
 useEffect(()=>{
  const node=dialog.current!,previous=document.activeElement as HTMLElement|null;
  node.showModal();
  return()=>{node.close();applyAppearance();restoreFocus(previous);};
 },[]);
 useEffect(()=>{applyAppearance(draft);},[draft]);
 const name=(id:Palette)=>id==='contrast'?c[12]:paletteLabel(id,mode);
 const matches=availablePalettes(mode).filter(id=>name(id).toLocaleLowerCase(language).includes(query.trim().toLocaleLowerCase(language)));
 function switchMode(next:PaletteMode){setMode(next);setQuery('');}
 return <dialog ref={dialog} className="theme-chooser" aria-labelledby="theme-chooser-title" aria-describedby="theme-chooser-hint" onCancel={event=>{event.preventDefault();onClose();}}>
  <header className="theme-chooser-heading"><div><h2 id="theme-chooser-title">{copy.title}</h2><p id="theme-chooser-hint">{copy.hint}</p></div></header>
  <div className="theme-chooser-toolbar">
   <div className="theme-mode-tabs" role="tablist" aria-label={c[0]}>{(['light','dark'] as const).map(item=><button type="button" key={item} id={'theme-tab-'+item} role="tab" aria-selected={mode===item} aria-controls="theme-chooser-panel" tabIndex={mode===item?0:-1} onClick={()=>switchMode(item)} onKeyDown={event=>{if(['ArrowLeft','ArrowRight','Home','End'].includes(event.key)){event.preventDefault();const next=event.key==='Home'?'light':event.key==='End'?'dark':mode==='light'?'dark':'light';switchMode(next);document.getElementById('theme-tab-'+next)?.focus();}}}>{c[item==='light'?1:2]}</button>)}</div>
   <input type="search" aria-label={copy.search} placeholder={copy.search} value={query} onChange={event=>setQuery(event.target.value)} />
  </div>
  <div id="theme-chooser-panel" className="theme-chooser-body" role="tabpanel" aria-labelledby={'theme-tab-'+mode}>
   <fieldset aria-label={c[mode==='light'?1:2]}><div className="theme-gallery">{matches.map(id=><button type="button" className="theme-option" key={id} aria-label={name(id)} aria-pressed={draft[mode]===id} onClick={()=>setDraft({...draft,[mode]:id})}>
    <ThemeSample id={id} mode={mode}/><span className="theme-option-label"><strong>{name(id)}</strong><span className="theme-selection-mark" aria-hidden="true">{draft[mode]===id?'✓':''}</span></span>
   </button>)}</div></fieldset>
   {!matches.length&&<p className="theme-empty" role="status">{copy.empty}</p>}
  </div>
  <footer className="theme-chooser-footer"><p role="status"><span>{c[5]}</span><strong>{name(draft[mode])}</strong></p><div><button type="button" className="secondary-button" onClick={onClose}>{c[7]}</button><button type="button" className="primary-button" onClick={()=>onApply(draft)}>{c[6]}</button></div></footer>
 </dialog>;
}

export function AppearanceSettings() {
 const {locale,messages}=useI18n(),c=personalization[locale];
 const [value,setValue]=useState(readAppearance),[picker,setPicker]=useState<PaletteMode|null>(null),[attempted,setAttempted]=useState(false);
 const states=usePreferenceSaveStates(), outcomes=[states['offgrid.appearance.v1'],states['desktop-presentation']];
 const failed=outcomes.includes('unsaved'),pending=outcomes.includes('pending');
 useEffect(()=>{const sync=()=>{setValue(readAppearance());setPicker(null);};sync();window.addEventListener('offgrid-appearance',sync);return()=>{window.removeEventListener('offgrid-appearance',sync);applyAppearance();};},[]);
 function commit(next:Appearance){
  if(window.electron?.setPresentation)reportPreferenceSave('desktop-presentation','pending');
  saveAppearance(next);setAttempted(true);setValue(next);setPicker(null);
 }
 return <div className="appearance-settings" id="setting-appearance" tabIndex={-1}>
  <p className="appearance-scope">{c[9]}</p>
  <div className="settings-card appearance-values">
  <div className="theme-summaries">{(['light','dark'] as const).map(mode=><div className="appearance-preference-row" key={mode}><span>{c[mode==='light'?1:2]}</span><button type="button" className="theme-summary" data-setting={mode+'-theme'} aria-haspopup="dialog" aria-label={c[mode==='light'?1:2]+': '+(value[mode]==='contrast'?c[12]:paletteLabel(value[mode],mode))} onClick={()=>setPicker(mode)}>
   <ThemeSample id={value[mode]} mode={mode}/><strong>{value[mode]==='contrast'?c[12]:paletteLabel(value[mode],mode)}</strong><Icon name="chevron-down" size={16}/>
  </button></div>)}</div>
  <div className="appearance-preference-row"><label htmlFor="appearance-text-size">{c[3]}</label><select id="appearance-text-size" value={value.textSize} onChange={event=>commit({...value,textSize:Number(event.target.value)})}>{[100,110,125,150].map(n=><option key={n} value={n}>{n}%</option>)}</select></div>
  <div className="appearance-preference-row"><label htmlFor="appearance-motion">{c[4]}</label><input id="appearance-motion" type="checkbox" checked={value.reduceMotion} onChange={event=>commit({...value,reduceMotion:event.target.checked})}/></div>
  </div>
  <div className="appearance-footer"><p role="status">{failed?c[10]:attempted?(pending?messages.common.loading:c[11]):''}</p><button type="button" className="text-button" onClick={()=>commit({...appearanceDefaults})}>{c[8]}</button></div>
  {picker&&<ThemeChooser initial={value} initialMode={picker} onApply={commit} onClose={()=>setPicker(null)}/>}
 </div>;
}
