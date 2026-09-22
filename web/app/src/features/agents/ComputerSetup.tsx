import { useEffect, useRef, useState } from 'react';
import { api, type ComputerModelCheck, type ComputerSession } from '../../api/client';
import { useI18n } from '../../i18n';
import { computerModelCopy, computerActionReview } from '../../i18n/computer-recovery';
import { computerTaskText } from '../../i18n/computer-task';
import { computerExperience } from '../../i18n/computer-experience';
import {stopBrowserAssistance} from './stopBrowser';
import {nativeAppText,nativeAppFeedback,nativeAppError,approvalPolicyText} from '../../i18n/native-app';

const visionCopy = {
 en:['Local vision ready for browser views · Preview','Local vision is not installed for this model. Structured controls still work.'],
 fr:['Vision locale prête pour les vues du navigateur · Aperçu','La vision locale n’est pas installée pour ce modèle. Les contrôles structurés fonctionnent toujours.'],
 de:['Lokale Bilderkennung für Browseransichten bereit · Vorschau','Für dieses Modell ist keine lokale Bilderkennung installiert. Strukturierte Steuerelemente funktionieren weiterhin.'],
 es:['Visión local lista para vistas del navegador · Vista previa','La visión local no está instalada para este modelo. Los controles estructurados siguen funcionando.'],
 ar:['الرؤية المحلية جاهزة لعرض المتصفح · معاينة','الرؤية المحلية غير مثبتة لهذا النموذج. تظل عناصر التحكم المنظمة متاحة.'],
 sw:['Uonaji wa ndani uko tayari kwa mwonekano wa kivinjari · Hakiki','Uonaji wa ndani haujasakinishwa kwa modeli hii. Vidhibiti vilivyopangwa bado vinafanya kazi.'],
 sn:['Kuona kwemuno kwagadzirira bhurawuza · Ongororo','Kuona kwemuno hakuna kuiswa pamuenzaniso uyu. Zvidzoro zvakarongwa zvichiri kushanda.'],
 nd:['Ukubona kwasendaweni sekulungele ibhrawuza · Ukuhlola','Ukubona kwasendaweni akufakwanga kule modeli. Izilawuli ezihlelekileyo zisasebenza.'],
 zu:['Ukubona kwasendaweni kulungele isiphequluli · Ukubuka kuqala','Ukubona kwasendaweni akufakiwe kule modeli. Izilawuli ezihlelekile zisasebenza.']
} as const;
const visionInstallCopy = {
 en: 'Local vision files are installed. OffGrid verifies the current model and runtime before enabling browser screenshots.',
 fr: 'Les fichiers de vision locale sont installés. OffGrid vérifie le modèle et le moteur actuels avant d’activer les captures du navigateur.',
 de: 'Lokale Bilddateien sind installiert. OffGrid prüft das aktuelle Modell und die Laufzeit, bevor Browseraufnahmen aktiviert werden.',
 es: 'Los archivos de visión local están instalados. OffGrid verifica el modelo y el entorno actuales antes de habilitar capturas del navegador.',
 ar: 'ملفات الرؤية المحلية مثبتة. يتحقق OffGrid من النموذج وبيئة التشغيل الحالية قبل تفعيل لقطات المتصفح.',
 sw: 'Faili za kuona za ndani zimesakinishwa. OffGrid hukagua modeli na mazingira ya sasa kabla ya kuwezesha picha za kivinjari.',
 sn: 'Mafaira ekuona emuno akaiswa. OffGrid inoongorora modhi neruntime yazvino isati yagonesa mifananidzo yebhurawuza.',
 nd: 'Amafayela okubona asekhaya afakiwe. I-OffGrid ihlola imodeli le-runtime yamanje ingakavumeli izithombe zebhrawuza.',
 zu: 'Amafayela okubona endawo afakiwe. I-OffGrid ihlola imodeli nesikhathi sokusebenza samanje ngaphambi kokuvumela izithombe zesiphequluli.'
} as const;

export function ComputerSetup({ value, onChange, disabled, onAvailability, onReady, model, visionStatus, mode }: { value: string; onChange: (id: string) => void; disabled: boolean; onAvailability: (available: boolean) => void; onReady: (ready: boolean) => void; model: string; visionStatus?: string; mode: 'app'|'browser' }) {
 const { locale, messages } = useI18n();
 const experience = computerExperience(locale);
 const managed = !!window.electron?.startComputerBrowser;
 const nativeManaged = !!window.electron?.discoverComputerApps;
 const nativeText = nativeAppText(locale);
 const approvalText = approvalPolicyText(locale);
 const [approvalMode,setApprovalMode]=useState<'ask_every_time'|'scoped_changes'|'full_task'>('scoped_changes');
 const [targets,setTargets]=useState<{id:string;title:string;driver:string}[]>([]);
 const [appTarget,setAppTarget]=useState('');
 const [launchTargets,setLaunchTargets]=useState<{id:string;title:string;driver:string}[]>([]);
 async function discoverApps() {
   setBusy(true);setError('');
   try {const identity=await api.systemIdentity();if(!identity.workspace_id)throw Error('workspace_unavailable');const state=await window.electron!.discoverComputerApps!({workspace:identity.workspace_id});setTargets(state.targets??[]);setAppTarget('');if(state.targets?.length===0)setError(nativeText.empty);}
   catch(e){setError(nativeAppError(locale,e instanceof Error?e.message:'',experience.repair));}finally{setBusy(false);}
 }
 async function discoverLaunchable() {
   setBusy(true); setError('');
   try { const state=await window.electron!.listComputerApplications!(); setLaunchTargets(state.targets??[]); if(!state.targets?.length)setError(nativeText.empty); }
   catch(e){setError(nativeAppError(locale,e instanceof Error?e.message:'',experience.repair));} finally {setBusy(false);}
 }
 async function launchApp() {
   if(!appTarget) return;
   setBusy(true); setError('');
   try { await window.electron!.launchComputerApplication!({id:appTarget}); setError(''); setLaunchTargets([]); setAppTarget(''); window.setTimeout(()=>void discoverApps(),700); }
   catch(e){setError(nativeAppError(locale,e instanceof Error?e.message:'',experience.repair));} finally {setBusy(false);}
 }
 async function connectApp() {
   setBusy(true);setError('');
   try {const identity=await api.systemIdentity();if(!identity.workspace_id)throw Error('workspace_unavailable');const state=await window.electron!.startComputerApp!({workspace:identity.workspace_id,target:appTarget,approvalMode});if(state.target)onChange(state.target.id);else throw Error(state.code??'computer_session_unavailable');}
   catch(e){setError(nativeAppError(locale,e instanceof Error?e.message:'',experience.repair));}finally{setBusy(false);}
 }
 const [website, setWebsite] = useState('');
 const [upload,setUpload]=useState<{id:string;name:string;size:number;sha256:string}|null>(null);
 const [networkMode, setNetworkMode] = useState<'direct'|'trusted-vpn'>('direct');
 const [host, setHost] = useState<{state:string;installed:boolean;code?:string}>();
 const [sessions, setSessions] = useState<ComputerSession[]>([]);
 const taskText = computerTaskText(locale);
 const [now, setNow] = useState(Date.now());
 const [error, setError] = useState(''); const [busy, setBusy] = useState(false);
 const [pollError, setPollError] = useState('');
 const [check, setCheck] = useState<ComputerModelCheck | null>(null);
 const [checking, setChecking] = useState(false);
 const [checkError, setCheckError] = useState('');
 const checkController = useRef<AbortController | null>(null);
 const modelText = computerModelCopy[locale];
 useEffect(() => {
   if (!managed) return;
   let disposed=false; let timer: ReturnType<typeof setTimeout>;
   const refresh=async()=>{try {const state=await window.electron!.getComputerStatus!();if(!disposed){setHost(state);if(state.state==='selecting'&&state.targets)setTargets(state.targets);}}catch{if(!disposed)setHost({state:'error',installed:false});}finally{if(!disposed)timer=setTimeout(()=>void refresh(),1000);}};
   void refresh(); return()=>{disposed=true;clearTimeout(timer);};
 }, [managed]);
 async function openBrowser(origin:string) {
   if(origin!=='demo') {
     try { const url=new URL(origin); if(url.protocol!=='https:' || url.username || url.password) throw Error(); }
     catch {setError(experience.invalidWebsite);return;}
   }
   setBusy(true);setError('');
   try {
     const identity=await api.systemIdentity();
     if (!identity.workspace_id) throw Error('workspace_unavailable');
     const state=await window.electron!.startComputerBrowser!({origin,workspace:identity.workspace_id,networkMode:origin==='demo'?'direct':networkMode,approvalMode,...(upload?{uploadGrant:upload.id}:{})});
     if(state.target) onChange(state.target.id);
     else if(state.code==='consent_declined') setError(experience.declined);
     else setError(experience.setupStopped);
   }catch(e){const message=e instanceof Error?e.message:'';setError(message.includes('network_blocked')?experience.networkBlocked:message.includes('network_unavailable')?experience.networkUnavailable:experience.repair);}finally{setBusy(false);}
 }
 useEffect(() => {
   checkController.current?.abort(); setCheck(null); setCheckError(''); setChecking(false);
   return () => { checkController.current?.abort(); };
 }, [model,value,appTarget,mode]);
 async function testModel() {
   if (!model || checking) return;
   const controller = new AbortController(); checkController.current = controller;
   setChecking(true); setCheck(null); setCheckError('');
   try {
     const driver=sessions.find(s=>s.id===value)?.driver??targets.find(t=>t.id===appTarget)?.driver??'browser';
     const result = await api.checkComputerModel(model, controller.signal, driver);
     if (!controller.signal.aborted) setCheck(result);
   } catch (error) { if (!controller.signal.aborted) setCheckError(error instanceof Error ? error.message : messages.common.error); }
   finally { if (!controller.signal.aborted) setChecking(false); }
 }
 useEffect(() => { let disposed = false;
   let timer: ReturnType<typeof setTimeout>;
   const refresh = async () => { try { const data = await api.computerSessions(); if (!disposed) { setSessions(data.sessions); setPollError(''); } } catch (e) { if (!disposed) setPollError(e instanceof Error ? e.message : messages.common.error); } finally { if (!disposed) timer = setTimeout(() => void refresh(), 2500); } };
   void refresh(); return () => { disposed = true; clearTimeout(timer); };
 }, [messages.common.error]);
 useEffect(() => {
   onAvailability(sessions.length > 0);
   if (value && !sessions.some(session => session.id === value)) onChange('');
   if (!value && sessions.length === 1 && sessions[0].state === 'ready') onChange(sessions[0].id);
   const selected = sessions.find(s => s.id === value);
   onReady(!pollError && !!selected && selected.state === 'ready' && selected.remaining_actions > 0 && (!selected.expires_at || Date.parse(selected.expires_at) > now));
 }, [sessions, value, pollError, now]);
 useEffect(() => { const timer = setInterval(() => setNow(Date.now()), 1000); return () => clearInterval(timer); }, []);
 async function stop() { setBusy(true);setError('');try { const result=await stopBrowserAssistance();if(result.revoked){onChange('');setSessions([]);setTargets([]);setLaunchTargets([]);setAppTarget('');} if(result.error)setError(result.error==='lost'?taskText.lost:experience[result.error]); } finally {setBusy(false);} }
 return <fieldset className="computer-setup" aria-label={nativeText.title}><legend>{mode==='app'?nativeText.window:nativeText.browser}</legend>
   {!sessions.length && <div className="field"><label htmlFor="computer-approval-mode">{approvalText.title}</label><select id="computer-approval-mode" aria-describedby="computer-approval-help" value={approvalMode} disabled={busy||disabled} onChange={event=>setApprovalMode(event.target.value as typeof approvalMode)}><option value="ask_every_time">{approvalText.ask}</option><option value="scoped_changes">{approvalText.scoped}</option><option value="full_task">{approvalText.full}</option></select><small id="computer-approval-help">{approvalMode==='ask_every_time'?approvalText.askHelp:approvalMode==='scoped_changes'?approvalText.scopedHelp:approvalText.fullHelp}</small></div>}
   {!sessions.length && nativeManaged && mode==='app' && <div className="browser-onboarding">
     <p>{nativeText.scope}</p>
     {host && !host.installed && <p role="alert">{experience.repair}</p>}
     {host?.state!=='selecting' && <div className="button-row"><button type="button" className="secondary-button" disabled={busy||disabled||!host?.installed} onClick={()=>void discoverApps()}>{busy?experience.starting:nativeText.choose}</button><button type="button" className="secondary-button" disabled={busy||disabled||!host?.installed} onClick={()=>void discoverLaunchable()}>{nativeText.open}</button></div>}
     {launchTargets.length>0 && <><p>{nativeText.opened}</p><label className="field"><span>{nativeText.open}</span><select value={appTarget} disabled={busy||disabled} onChange={event=>setAppTarget(event.target.value)}><option value="">{nativeText.open}</option>{launchTargets.map(target=><option key={target.id} value={target.id}>{target.title}</option>)}</select></label><button type="button" className="secondary-button" disabled={busy||disabled||!appTarget} onClick={()=>void launchApp()}>{nativeText.launch}</button></>}
     {targets.length>0 && <><label className="field"><span>{nativeText.select}</span><select value={appTarget} disabled={busy||disabled} onChange={event=>setAppTarget(event.target.value)}><option value="">{nativeText.select}</option>{targets.map(target=><option key={target.id} value={target.id}>{target.title}</option>)}</select></label><button type="button" className="primary-button" disabled={busy||disabled||!appTarget} onClick={()=>void connectApp()}>{busy?experience.starting:nativeText.connect}</button></>}
     {(busy||host?.state==='selecting'||host?.state==='error') && <button type="button" className="secondary-button" onClick={()=>void stop()}>{messages.models.cancel}</button>}
   </div>}
   {!sessions.length && (!nativeManaged||mode==='browser') && (managed ? <div className="browser-onboarding">
     <label className="field"><span>{experience.website}</span><input type="url" placeholder="https://example.com" value={website} disabled={busy || disabled} onChange={event=>setWebsite(event.target.value)} /></label>
     {window.electron?.selectComputerUpload && <div className="button-row"><button type="button" className="secondary-button" disabled={busy||disabled} onClick={()=>void window.electron!.selectComputerUpload!().then(setUpload).catch(()=>setError(experience.repair))}>{nativeText.chooseUpload}</button>{upload&&<><span>{nativeText.selectedUpload}: {upload.name} · {(upload.size/1024/1024).toFixed(1)} MB</span><button type="button" className="secondary-button" disabled={busy||disabled} onClick={()=>setUpload(null)}>{nativeText.removeUpload}</button></>}</div>}
     <details><summary>{experience.networkSettings}</summary><label className="field"><span>{experience.networkSettings}</span><select value={networkMode} disabled={busy || disabled} onChange={event=>setNetworkMode(event.target.value as 'direct'|'trusted-vpn')}><option value="direct">{experience.directNetwork}</option><option value="trusted-vpn">{experience.trustedNetwork}</option></select></label>{networkMode==='trusted-vpn' && <p>{experience.networkWarning}</p>}</details>
     <div className="button-row"><button type="button" className="primary-button" disabled={busy || disabled || !website.trim() || !host?.installed} onClick={()=>void openBrowser(website.trim())}>{host?.state==='consent'?experience.waiting:busy?experience.starting:experience.open}</button><button type="button" className="secondary-button" disabled={busy || disabled || !host?.installed} onClick={()=>void openBrowser('demo')}>{experience.practice}</button></div>
     {host && !host.installed && <p role="alert">{experience.repair}</p>}
     {busy && <div className="button-row"><p role="status">{host?.state==='consent'?experience.waiting:experience.starting}</p><button type="button" className="secondary-button" onClick={()=>void stop()}>{experience.stop}</button></div>}
   </div> : <div className="browser-onboarding"><strong>{experience.desktop}</strong><p>{nativeText.desktop}</p><a className="secondary-button" href="offgrid://computer">{experience.openDesktop}</a></div>)}
   <details><summary>{taskText.advanced}</summary><section aria-label={modelText.check}>
     <p>{nativeText.scope}</p>
     <p role="status">{check?.vision?.passed || visionStatus === 'tested' ? visionCopy[locale][0] : visionStatus ? visionInstallCopy[locale] : visionCopy[locale][1]}</p>
     <p>{modelText.help}</p><strong>{model || '—'}</strong>
     <div className="button-row"><button type="button" className="secondary-button" disabled={!model || checking || disabled} onClick={() => void testModel()}>{checking ? modelText.checking : modelText.check}</button>
     {checking && <button type="button" onClick={() => { checkController.current?.abort(); setChecking(false); }}>{messages.models.cancel}</button>}</div>
     {check && <p role="status">{check.message}</p>}
     {check?.vision && <p role={check.vision.passed ? 'status' : 'alert'}>{check.vision.message}</p>}
     {check?.runtime && <small>{check.runtime.build} · {check.runtime.context} tokens · {check.runtime.template_sha256?.slice(0,12)}</small>}
     {checkError && <p role="alert">{checkError}</p>}
     {check && !check.passed && <a href="#/models">{messages.nav.models}</a>}
   </section></details>
   {sessions.length > 0 && <label className="field"><span>{experience.target}</span><select value={value} disabled={disabled} onChange={e => onChange(e.target.value)}><option value="">{nativeText.select}</option>{sessions.map(s => <option key={s.id} value={s.id} disabled={s.state !== 'ready'}>{s.origin} · {s.approval_mode==='full_task'?approvalText.full:s.approval_mode==='scoped_changes'?approvalText.scoped:approvalText.ask} · {taskText[s.state ?? 'in_use']} · {s.remaining_actions}</option>)}</select></label>}
   {sessions.map(s => <p key={s.id}>{['finished','exhausted'].includes(s.state ?? '') ? (managed?experience.fresh:taskText.renew) : ''}{s.expires_at && <span className="browser-expiry">{taskText.remaining}: {Math.max(0, Math.ceil((Date.parse(s.expires_at) - now) / 1000))}</span>}</p>)}
   {!!sessions.length && <button type="button" className="secondary-button" disabled={busy} onClick={() => void stop()}>{sessions.some(s=>s.driver&&s.driver!=='browser')?nativeAppFeedback(locale).stop:experience.stop}</button>}
   {error && <p role="alert">{error}</p>}
   {!error && host?.state==='error' && <p role="alert">{host.code==='network_blocked'?experience.networkBlocked:host.code==='network_unavailable'?experience.networkUnavailable:host.code==='stop_unconfirmed'?experience.stopUnconfirmed:['action_conflict','action_uncertain','duplicate_action'].includes(host.code ?? '')?computerActionReview[locale]:nativeAppError(locale,host.code??'',experience.repair)}</p>}
   {pollError && <p role="alert">{pollError}</p>}
 </fieldset>;
}
