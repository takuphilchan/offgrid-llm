import {useEffect,useState} from 'react';
import type {CommandWindowState} from '../../platform';
import {useI18n} from '../../i18n';
import {desktopCommand} from '../../i18n/desktop-command';
import {ActionCard} from '../../components/ActionCard';
export function DesktopCommandSettings() {
 const {locale,messages} = useI18n(), copy = desktopCommand(locale);
 const [state,setState] = useState<CommandWindowState|null>(null), [error,setError] = useState(''), [busy,setBusy] = useState(false);
 useEffect(()=>{
  let mounted = true, received = false;
  const unsubscribe=window.electron?.onCommandWindow?.(value=>{received=true;if(mounted)setState(value);});
  void window.electron?.getCommandWindow?.().then(value=>{if(mounted&&!received)setState(value);}).catch(()=>{if(mounted)setError(messages.common.error);});
  return ()=>{mounted=false;unsubscribe?.();};
 },[]);
 if (!window.electron?.setCommandShortcut) return null;
 return <><p className="settings-scope">{copy.background}</p><div className="settings-panel desktop-command-settings">
  <label className="field" htmlFor="desktop-command-shortcut">{copy.shortcut}</label><select id="desktop-command-shortcut" disabled={!state||busy} value={state?.shortcut??''} onChange={async event=>{
   setBusy(true);setError('');try{setState(await window.electron!.setCommandShortcut!(event.target.value));}catch{setError(messages.common.error);}finally{setBusy(false);}
  }}><option value="">{copy.off}</option>{['CommandOrControl+Shift+Space','Alt+Space','CommandOrControl+Alt+Space'].map(value=><option key={value} value={value}>{value.replace('CommandOrControl',window.electron?.platform==='darwin'?'⌘':'Ctrl').replaceAll('+',' + ')}</option>)}</select>
  {state?.shortcut&&!state.registered&&<p role="status">{copy.conflict}</p>}{error&&<p role="alert">{error}</p>}
 </div><ActionCard icon="quick-window" title={copy.open} description={state?.shortcut ? state.shortcut.replace('CommandOrControl',window.electron?.platform==='darwin'?'⌘':'Ctrl').replaceAll('+',' + ') : copy.title} onClick={()=>void window.electron?.openCommandWindow?.().catch(()=>setError(messages.common.error))}/></>;
}
