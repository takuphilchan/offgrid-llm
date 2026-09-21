import { useEffect, useRef, useState } from 'react';
import { api, type ComputerModelCheck, type ComputerSession } from '../../api/client';
import { useI18n } from '../../i18n';
import { computerRecovery, computerModelCopy, computerActionReview } from '../../i18n/computer-recovery';
import { computerTaskText } from '../../i18n/computer-task';
import { copyText } from '../../lib/clipboard';
import { computerExperience } from '../../i18n/computer-experience';
import {stopBrowserAssistance} from './stopBrowser';

const copy = {
 en: ['Computer task · Preview','Pair browser','Stop browser','Select a paired browser','No paired browser','Run the host companion, approve one HTTPS site locally, then enter this single-use code. It expires in two minutes.','Native desktop and vision control are not available in this preview.'],
 fr: ['Tâche informatique · Aperçu','Associer le navigateur','Arrêter le navigateur','Sélectionner un navigateur associé','Aucun navigateur associé','Lancez le compagnon local, autorisez un site HTTPS, puis saisissez ce code à usage unique. Il expire dans deux minutes.','Le contrôle du bureau et la vision ne sont pas disponibles dans cet aperçu.'],
 de: ['Computeraufgabe · Vorschau','Browser verbinden','Browser stoppen','Verbundenen Browser auswählen','Kein Browser verbunden','Starten Sie den lokalen Begleiter, erlauben Sie eine HTTPS-Seite und geben Sie diesen einmaligen Code ein. Er läuft nach zwei Minuten ab.','Desktop- und Bildsteuerung sind in dieser Vorschau nicht verfügbar.'],
 es: ['Tarea informática · Vista previa','Vincular navegador','Detener navegador','Seleccionar navegador vinculado','Ningún navegador vinculado','Inicie el acompañante local, autorice un sitio HTTPS e introduzca este código de un solo uso. Caduca en dos minutos.','El control del escritorio y la visión no están disponibles en esta vista previa.'],
 ar: ['مهمة حاسوب · معاينة','ربط المتصفح','إيقاف المتصفح','اختر متصفحًا مرتبطًا','لا يوجد متصفح مرتبط','شغّل المرافق المحلي واسمح بموقع HTTPS ثم أدخل هذا الرمز لمرة واحدة. تنتهي صلاحيته خلال دقيقتين.','التحكم بسطح المكتب والرؤية غير متاحين في هذه المعاينة.'],
 sw: ['Kazi ya kompyuta · Hakiki','Unganisha kivinjari','Simamisha kivinjari','Chagua kivinjari kilichounganishwa','Hakuna kivinjari','Anzisha programu saidizi, ruhusu tovuti moja ya HTTPS, kisha ingiza msimbo huu wa matumizi moja. Unaisha baada ya dakika mbili.','Udhibiti wa eneo-kazi na kuona picha haupatikani katika hakiki hii.'],
 sn: ['Basa rekombiyuta · Ongororo','Batanidza bhurawuza','Misa bhurawuza','Sarudza bhurawuza rakabatana','Hapana bhurawuza rakabatana','Tanga mubatsiri pakombiyuta, bvumira saiti imwe yeHTTPS, woisa kodhi iyi kamwe chete. Inopera mumaminitsi maviri.','Kudzora desktop nekuona mifananidzo hazvisati zvawanikwa pano.'],
 nd: ['Umsebenzi wekhompyutha · Ukuhlola','Xhuma ibhrawuza','Misa ibhrawuza','Khetha ibhrawuza exhunyiweyo','Akulabhrawuza exhunyiweyo','Qalisa umsizi, uvumele iwebhusayithi eyodwa yeHTTPS, ufake ikhodi le kanye. Iphela ngemizuzu emibili.','Ukulawula ideskithophu lokubona imifanekiso akukatholakali lapha.'],
 zu: ['Umsebenzi wekhompyutha · Ukubuka kuqala','Xhuma isiphequluli','Misa isiphequluli','Khetha isiphequluli esixhunyiwe','Asikho isiphequluli esixhunyiwe','Qala umsizi, uvumele isayithi elilodwa leHTTPS bese ufaka le khodi kanye. Iphela emizuzwini emibili.','Ukulawula ideskithophu nokubona izithombe akutholakali lapha.']
};

export function ComputerSetup({ value, onChange, disabled, onAvailability, onReady, model }: { value: string; onChange: (id: string) => void; disabled: boolean; onAvailability: (available: boolean) => void; onReady: (ready: boolean) => void; model: string }) {
 const { locale, messages } = useI18n(); const text = copy[locale];
 const experience = computerExperience(locale);
 const managed = !!window.electron?.startComputerBrowser;
 const [website, setWebsite] = useState('');
 const [networkMode, setNetworkMode] = useState<'direct'|'trusted-vpn'>('direct');
 const [host, setHost] = useState<{state:string;installed:boolean;code?:string}>();
 const [sessions, setSessions] = useState<ComputerSession[]>([]);
 const taskText = computerTaskText(locale);
 const [copied, setCopied] = useState(false);
 const [now, setNow] = useState(Date.now());
 const [code, setCode] = useState(''); const [error, setError] = useState(''); const [busy, setBusy] = useState(false);
 const [pollError, setPollError] = useState('');
 const [check, setCheck] = useState<ComputerModelCheck | null>(null);
 const [checking, setChecking] = useState(false);
 const [checkError, setCheckError] = useState('');
 const checkController = useRef<AbortController | null>(null);
 const modelText = computerModelCopy[locale];
 useEffect(() => {
   if (!managed) return;
   let disposed=false; let timer: ReturnType<typeof setTimeout>;
   const refresh=async()=>{try {const state=await window.electron!.getComputerStatus!();if(!disposed)setHost(state);}catch{if(!disposed)setHost({state:'error',installed:false});}finally{if(!disposed)timer=setTimeout(()=>void refresh(),1000);}};
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
     const state=await window.electron!.startComputerBrowser!({origin,workspace:identity.workspace_id,networkMode:origin==='demo'?'direct':networkMode});
     if(state.target) onChange(state.target.id);
     else if(state.code==='consent_declined') setError(experience.declined);
     else setError(experience.setupStopped);
   }catch(e){const message=e instanceof Error?e.message:'';setError(message.includes('network_blocked')?experience.networkBlocked:message.includes('network_unavailable')?experience.networkUnavailable:experience.repair);}finally{setBusy(false);}
 }
 useEffect(() => {
   checkController.current?.abort(); setCheck(null); setCheckError(''); setChecking(false);
   return () => { checkController.current?.abort(); };
 }, [model]);
 async function testModel() {
   if (!model || checking) return;
   const controller = new AbortController(); checkController.current = controller;
   setChecking(true); setCheck(null); setCheckError('');
   try {
     const result = await api.checkComputerModel(model, controller.signal);
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
   if (sessions.length) setCode('');
 }, [sessions, value, pollError, now]);
 useEffect(() => { const timer = setInterval(() => setNow(Date.now()), 1000); return () => clearInterval(timer); }, []);
 useEffect(() => { if (!code) return; const timer = setTimeout(() => setCode(''), 120000); return () => clearTimeout(timer); }, [code]);
 async function pair() { setBusy(true); setError(''); setCopied(false); try { setCode((await api.computerPairing()).code); } catch (e) { setError(e instanceof Error ? e.message : messages.common.error); } finally { setBusy(false); } }
 async function stop() { setBusy(true);setError('');try { const result=await stopBrowserAssistance();if(result.revoked){onChange('');setSessions([]);setCode('');} if(result.error)setError(result.error==='lost'?taskText.lost:experience[result.error]); } finally {setBusy(false);} }
 return <fieldset className="computer-setup"><legend>{experience.browser}</legend>
   {!sessions.length && (managed ? <div className="browser-onboarding">
     <label className="field"><span>{experience.website}</span><input type="url" placeholder="https://example.com" value={website} disabled={busy || disabled} onChange={event=>setWebsite(event.target.value)} /></label>
     <details><summary>{experience.networkSettings}</summary><label className="field"><span>{experience.networkSettings}</span><select value={networkMode} disabled={busy || disabled} onChange={event=>setNetworkMode(event.target.value as 'direct'|'trusted-vpn')}><option value="direct">{experience.directNetwork}</option><option value="trusted-vpn">{experience.trustedNetwork}</option></select></label>{networkMode==='trusted-vpn' && <p>{experience.networkWarning}</p>}</details>
     <div className="button-row"><button type="button" className="primary-button" disabled={busy || disabled || !website.trim() || !host?.installed} onClick={()=>void openBrowser(website.trim())}>{host?.state==='consent'?experience.waiting:busy?experience.starting:experience.open}</button><button type="button" className="secondary-button" disabled={busy || disabled || !host?.installed} onClick={()=>void openBrowser('demo')}>{experience.practice}</button></div>
     {host && !host.installed && <p role="alert">{experience.repair}</p>}
     {busy && <div className="button-row"><p role="status">{host?.state==='consent'?experience.waiting:experience.starting}</p><button type="button" className="secondary-button" onClick={()=>void stop()}>{experience.stop}</button></div>}
   </div> : <div className="browser-onboarding"><strong>{experience.desktop}</strong><p>{experience.desktopHelp}</p><a className="secondary-button" href="offgrid://computer">{experience.openDesktop}</a></div>)}
   <details><summary>{taskText.advanced}</summary><section aria-label={modelText.check}>
     <p>{text[6]}</p>
     <p>{modelText.help}</p><strong>{model || '—'}</strong>
     <div className="button-row"><button type="button" className="secondary-button" disabled={!model || checking || disabled} onClick={() => void testModel()}>{checking ? modelText.checking : modelText.check}</button>
     {checking && <button type="button" onClick={() => { checkController.current?.abort(); setChecking(false); }}>{messages.models.cancel}</button>}</div>
     {check && <p role="status">{check.message}</p>}
     {check?.runtime && <small>{check.runtime.build} · {check.runtime.context} tokens · {check.runtime.template_sha256?.slice(0,12)}</small>}
     {checkError && <p role="alert">{checkError}</p>}
     {check && !check.passed && <a href="#/models">{messages.nav.models}</a>}
   </section></details>
   {sessions.length > 0 && <label className="field"><span>{text[3]}</span><select value={value} disabled={disabled} onChange={e => onChange(e.target.value)}><option value="">{text[4]}</option>{sessions.map(s => <option key={s.id} value={s.id} disabled={s.state !== 'ready'}>{s.origin} · {taskText[s.state ?? 'in_use']} · {s.remaining_actions}</option>)}</select></label>}
   {sessions.map(s => <p key={s.id}>{['finished','exhausted'].includes(s.state ?? '') ? (managed?experience.fresh:taskText.renew) : ''}{s.expires_at && <span className="browser-expiry">{taskText.remaining}: {Math.max(0, Math.ceil((Date.parse(s.expires_at) - now) / 1000))}</span>}</p>)}
   {!!sessions.length && <button type="button" className="secondary-button" disabled={busy} onClick={() => void stop()}>{text[2]}</button>}
   {!managed && <details className="developer-connection"><summary>{experience.manual}</summary><p>{computerRecovery[locale].setup}</p><button type="button" className="secondary-button" disabled={busy || disabled || sessions.length > 0} onClick={() => void pair()}>{text[1]}</button>
   {code && <div role="status"><p>{text[5]}</p><code className="computer-pair-code">{code}</code><button type="button" className="secondary-button" onClick={() => void copyText(code).then(() => setCopied(true)).catch(() => setError(messages.common.error))}>{copied ? taskText.copied : taskText.copy}</button></div>}
   </details>}
   {error && <p role="alert">{error}</p>}
   {!error && host?.state==='error' && <p role="alert">{host.code==='network_blocked'?experience.networkBlocked:host.code==='network_unavailable'?experience.networkUnavailable:host.code==='stop_unconfirmed'?experience.stopUnconfirmed:['action_conflict','action_uncertain','duplicate_action'].includes(host.code ?? '')?computerActionReview[locale]:experience.repair}</p>}
   {pollError && <p role="alert">{pollError}</p>}
 </fieldset>;
}
