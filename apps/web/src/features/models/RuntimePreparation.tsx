import { useEffect, useRef, useState } from 'react';
import { api } from '../../api/client';
import type { components } from '../../api/schema.generated';
import { useI18n } from '../../i18n';
import { runtimeSetup } from '../../i18n/runtime-setup';

type Status = components['schemas']['InferenceRuntimeReadiness'];
export function RuntimePreparation({ canPrepare }: { canPrepare: boolean }) {
 const { locale, messages: text } = useI18n(), copy = runtimeSetup(locale);
 const [status, setStatus] = useState<Status | null>(null);
 const [unavailable, setUnavailable] = useState(false);
 const [working, setWorking] = useState(false);
 const operation = useRef<AbortController | null>(null), alive = useRef(true);
 const sequence = useRef(0);
 const busy = working || !!status && ['checking', 'downloading', 'installing'].includes(status.state);
 const refresh = async (signal?: AbortSignal) => {
  const request = ++sequence.current;
  try {
   const next = await api.runtimeReadiness(signal);
   if (!next || typeof next.mode !== 'string' || typeof next.state !== 'string') throw Error('Unavailable runtime status');
   if (alive.current && !signal?.aborted && sequence.current === request) { setStatus(next); setUnavailable(false); }
  }
  catch { if (alive.current && !signal?.aborted && sequence.current === request) setUnavailable(true); }
 };
 useEffect(() => {
  alive.current = true;
  const controller = new AbortController();
  void refresh(controller.signal);
  return () => { alive.current = false; controller.abort(); operation.current?.abort(); };
 }, []);
 useEffect(() => {
  if (!busy) return;
  const controller = new AbortController();
  const timer = setInterval(() => void refresh(controller.signal), 1000);
  return () => { clearInterval(timer); controller.abort(); };
 }, [busy]);
 const prepare = async () => {
  if (operation.current) return;
  const controller = new AbortController(); operation.current = controller;
  setWorking(true);
  try { const next = await api.prepareRuntime(controller.signal); if (alive.current && !controller.signal.aborted) { sequence.current++; setStatus(next); } }
  catch { if (alive.current) await refresh(); }
  finally { operation.current = null; if (alive.current) setWorking(false); }
 };
 const amount = (bytes: number) => `${(bytes / 1048576).toLocaleString(locale, { maximumFractionDigits: 1 })} MiB`;
 return <section className="setup-guidance runtime-preparation" aria-label={copy.title}>
  <strong>{copy.title}{status ? ` · ${status.mode.toUpperCase()}` : ''}</strong>
  <p role="status">{unavailable ? copy.unavailable : status?.state === 'ready' ? copy.ready : status && ['failed', 'cancelled'].includes(status.state) ? copy.failed : busy ? text.status.checking : copy.notice}</p>
  {status?.gpu_detected && <p>{copy.gpu}</p>}
  {!!status?.download_bytes && status.state !== 'ready' && <p>{amount(status.downloaded_bytes)} / {amount(status.download_bytes)}</p>}
  {busy && status && <progress aria-label={copy.title} max={status.download_bytes || 1} value={status.state === 'downloading' && status.download_bytes ? status.downloaded_bytes : undefined} />}
  {canPrepare && status && status.state !== 'ready' && !busy && <button className="secondary-button" onClick={() => void prepare()}>{copy.prepare}</button>}
  {!canPrepare && status?.state !== 'ready' && <p>{copy.admin}</p>}
  {working && <button className="text-button" onClick={() => operation.current?.abort()}>{text.models.cancel}</button>}
  {unavailable && <button className="text-button" aria-label={`${copy.title}: ${text.common.retry}`} onClick={() => void refresh()}>{text.common.retry}</button>}
 </section>;
}
