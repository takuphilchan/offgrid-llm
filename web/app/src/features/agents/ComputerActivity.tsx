import { useEffect, useState } from 'react';
import { api, type ComputerSession } from '../../api/client';
import { useI18n } from '../../i18n';
import { computerTaskText } from '../../i18n/computer-task';
import {computerExperience} from '../../i18n/computer-experience';
import {stopBrowserAssistance} from './stopBrowser';
import {nativeAppFeedback,approvalPolicyText} from '../../i18n/native-app';

// Mounted in the workspace shell, not a page. Stopping the local browser must
// remain possible while navigating, checking a model or waiting for approval.
export function ComputerActivity() {
 const { locale } = useI18n(); const text = computerTaskText(locale);
 const [sessions, setSessions] = useState<ComputerSession[]>([]);
 const [lost, setLost] = useState(false), [busy, setBusy] = useState(false), [stopping, setStopping] = useState(false);
 const [error, setError] = useState('');
 const [native,setNative]=useState(false);
 const nativeText=nativeAppFeedback(locale);
 const approvalText=approvalPolicyText(locale);
 useEffect(() => {
  let disposed = false; let timer: ReturnType<typeof setTimeout>;
  async function refresh() {
   try { const data = await api.computerSessions(); if (!disposed) { setSessions(data.sessions);if(data.sessions.length)setNative(data.sessions.some(s=>!!s.driver&&s.driver!=='browser')); setLost(false); } }
   catch { if (!disposed) setLost(true); }
   finally { if (!disposed) timer = setTimeout(() => void refresh(), 2500); }
  }
  void refresh(); return () => { disposed = true; clearTimeout(timer); };
 }, []);
 async function stop() {
  setBusy(true); setError('');
  try { const result=await stopBrowserAssistance();setStopping(result.revoked);setError(result.error?(result.error==='lost'?text.lost:computerExperience(locale)[result.error]):''); }
  finally { setBusy(false); }
 }
 useEffect(() => { if (sessions.some(s => s.state === 'ready')) setStopping(false); }, [sessions]);
 if (!sessions.length && !stopping && !error) return null;
 return <aside className="computer-activity" aria-label={native?nativeText.connected:text.connected}>
  <div><strong>{native?nativeText.connected:text.connected}</strong>{sessions.map(s => <p key={s.id}>{s.origin} · {s.approval_mode==='full_task'?approvalText.full:s.approval_mode==='scoped_changes'?approvalText.scoped:approvalText.ask} · {text[s.state ?? 'in_use']}</p>)}
  {stopping && <p role="status">{native?nativeText.stopping:text.stopping}</p>}{(lost || error) && <p role="alert">{error || text.lost}</p>}</div>
  {stopping && !sessions.length && !lost && !error ? <button type="button" className="secondary-button" onClick={() => setStopping(false)}>{text.close}</button> : <button type="button" className="secondary-button" disabled={busy} onClick={() => void stop()}>{native?nativeText.stop:text.stop}</button>}
 </aside>;
}
