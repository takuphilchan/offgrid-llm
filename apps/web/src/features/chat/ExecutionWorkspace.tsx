import { useEffect, useRef, useState } from 'react';
import { api, type ExecutionWorkspace as Workspace } from '../../api/client';
import { UtilityPopover } from '../../components/UtilityPopover';
import { useI18n } from '../../i18n';
import { executionWorkspaceText, executionNativeText } from '../../i18n/execution-workspace';
import { useWorkspaceRefresh } from '../../lib/workspace-refresh';

export function sameExecutionWorkspace(a: Workspace, b: Workspace) {
  return a.reference.host === b.reference.host && a.reference.session === b.reference.session && a.reference.grant === b.reference.grant && a.reference.revision === b.reference.revision;
}

/** Discovery selects existing authority; never requests a grant or submits work. */
export function ExecutionWorkspace({value, onChange, disabled, onReady,ceiling,initialAccess='edit'}: {value: Workspace | null; onChange: (value: Workspace | null) => void; disabled: boolean; onReady: (ready: boolean) => void;ceiling?:Workspace['permissions'];initialAccess?:'read'|'edit'|'trusted'}) {
  const {locale, messages} = useI18n(), copy = executionWorkspaceText[locale];
  const [hosts, setHosts] = useState<Workspace[]>([]), [loading, setLoading] = useState(true), [error, setError] = useState('');
  const [supported, setSupported] = useState(false);
  const nativeCopy=executionNativeText[locale];
  const [access,setAccess]=useState<'read'|'edit'|'trusted'>(initialAccess),[nativeBusy,setNativeBusy]=useState(false),[nativeState,setNativeState]=useState('idle');
  const [nativeError,setNativeError]=useState('');
  const alive=useRef(true);
  const check = useRef<AbortController | null>(null);
  const ready = !value || !loading && !error && hosts.some(host => sameExecutionWorkspace(value, host));
  useEffect(() => { onReady(ready); }, [ready, onReady]);
  const refresh = async () => {
    check.current?.abort(); const controller = new AbortController(); check.current = controller; setLoading(true); setError('');
    try {
      const identity = await api.systemIdentity();
      if (controller.signal.aborted) return;
      const available = identity.capabilities?.includes('native-execution-preview-v1') === true; setSupported(available);
      if (!available) { setHosts([]); return; }
      const result = await api.executionWorkspaces(controller.signal);
      if (result.protocol !== 1 || !Array.isArray(result.workspaces)) throw new Error(copy.unsupported);
      if (!controller.signal.aborted) setHosts(result.workspaces);
      if (window.electron?.getExecutionWorkspace) {
        try{const local=await window.electron.getExecutionWorkspace();if(!controller.signal.aborted){setNativeState(local.state);setNativeError(local.state==='uncertain'?copy.unavailable:'');}}
        catch{if(!controller.signal.aborted){setNativeState('uncertain');setNativeError(copy.unavailable);}}
      }
    } catch (reason) { if (!controller.signal.aborted) { setHosts([]); setError(reason instanceof Error ? reason.message : messages.common.error); } }
    finally { if (!controller.signal.aborted) setLoading(false); }
  };
  useEffect(() => { alive.current=true;void refresh(); return () => {alive.current=false;check.current?.abort();}; }, []);
  useWorkspaceRefresh(refresh);
  const nativeAction=async(stop:boolean)=>{
    if(nativeBusy)return;setNativeBusy(true);setNativeError('');
    try{
      const result:{state:string;resource?:Workspace}=stop?await window.electron!.stopExecutionWorkspace!():await window.electron!.startExecutionWorkspace!({access});
      if(!alive.current)return;setNativeState(result.state);
      if(!stop&&result.resource)onChange(result.resource);
      if(result.state==='uncertain')throw new Error(copy.unavailable);
      await refresh();
    }catch{if(alive.current)setNativeError(copy.unavailable);}
    finally{if(alive.current)setNativeBusy(false);}
  };
  return <UtilityPopover label={copy.label} trigger={<>{copy.label}<span className="composer-choice">{value ? value.reference.host : copy.none}</span></>} triggerClassName="secondary-button" className="chat-execution-workspace" onOpenChange={open => {if (open) void refresh();}}>
    <p>{copy.help}</p>
    <fieldset disabled={disabled}><legend>{copy.label}</legend>
      <label className="profile-option"><input type="radio" name="chat-execution" checked={!value} onChange={() => onChange(null)} /><span>{copy.none}</span></label>
      {hosts.map(host => <label className="profile-option" key={JSON.stringify(host.reference)}><input type="radio" name="chat-execution" checked={!!value && sameExecutionWorkspace(value, host)} onChange={() => onChange(host)} /><span>{host.reference.host}<small dir="auto" style={{overflowWrap:'anywhere'}}>{host.root}</small><small>{host.backend === 'isolated_linux' ? copy.isolated : copy.local}</small><small>{[host.permissions.read && ceiling?.read!==false && copy.read, host.permissions.write && ceiling?.write!==false && copy.edit, host.permissions.processes && ceiling?.processes!==false && copy.commands].filter(Boolean).join(' · ')}</small></span></label>)}
    </fieldset>
    {window.electron?.startExecutionWorkspace && supported && <fieldset disabled={disabled||nativeBusy}>
      <legend>{nativeCopy.access}</legend><label>{nativeCopy.access}<select value={access} onChange={event=>setAccess(event.target.value as typeof access)}><option value="read">{copy.read}</option><option value="edit">{copy.read} · {copy.edit}</option><option value="trusted">{copy.read} · {copy.edit} · {copy.commands}</option></select></label>
      <p>{copy.local}</p><button type="button" className="secondary-button" disabled={!['idle','stopped','error'].includes(nativeState)} onClick={()=>void nativeAction(false)}>{nativeCopy.choose}</button>
    </fieldset>}
    {window.electron?.stopExecutionWorkspace && !['idle','stopped','error'].includes(nativeState) && <button type="button" className="secondary-button" disabled={nativeBusy} onClick={()=>void nativeAction(true)}>{nativeCopy.stop}</button>}
    {nativeBusy && <p role="status">{nativeCopy.preparing}</p>}
    {nativeError && <p role="alert">{nativeError}</p>}
    {loading && <p role="status">{messages.common.loading}</p>}
    {!loading && !supported && <p role="status">{copy.unsupported}</p>}
    {!loading && supported && !hosts.length && !error && <p role="status">{copy.empty}</p>}
    {!ready && value && <p role="alert">{copy.unavailable}</p>}
    {error && <p role="alert">{error}</p>}
    <button type="button" className="secondary-button" disabled={loading} onClick={() => void refresh()}>{copy.refresh}</button>
  </UtilityPopover>;
}
