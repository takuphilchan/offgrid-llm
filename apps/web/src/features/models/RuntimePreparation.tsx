import { useCallback, useEffect, useRef, useState } from 'react';
import { api, APIError } from '../../api/client';
import type { components } from '../../api/schema.generated';
import { useI18n } from '../../i18n';
import { runtimeSetup } from '../../i18n/runtime-setup';

type Status = components['schemas']['InferenceRuntimeReadiness'];
function validStatus(value: unknown): value is Status {
 if (!value || typeof value !== 'object') return false;
 const status = value as Status;
 return ['unchecked', 'missing', 'checking', 'downloading', 'installing', 'ready', 'failed', 'cancelled'].includes(status.state)
  && ['cpu', 'metal', 'nvidia', 'amd'].includes(status.mode)
  && typeof status.gpu_detected === 'boolean' && typeof status.download_required === 'boolean'
  && Number.isSafeInteger(status.download_bytes) && status.download_bytes >= 0
  && Number.isSafeInteger(status.downloaded_bytes) && status.downloaded_bytes >= 0
  && typeof status.revision === 'string';
}
export function RuntimePreparation({ canPrepare }: { canPrepare: boolean }) {
 const { locale, messages: text } = useI18n(), copy = runtimeSetup(locale);
 const [status, setStatus] = useState<Status | null>(null);
 const [availability, setAvailability] = useState<'checking' | 'known' | 'unavailable' | 'unsupported'>('checking');
 const [reading, setReading] = useState(false);
 const [working, setWorking] = useState(false);
 const lifetime = useRef<AbortController | null>(null);
 const operation = useRef<AbortController | null>(null), read = useRef<AbortController | null>(null);
 const busy = working || !!status && ['checking', 'downloading', 'installing'].includes(status.state);
 const invalidateRead = () => { read.current?.abort(); read.current = null; };
 const refresh = useCallback(async () => {
  const owner = lifetime.current;
  if (!owner || owner.signal.aborted || read.current) return;
  const controller = new AbortController(); read.current = controller;
  const signal = AbortSignal.any([owner.signal, controller.signal]);
  setReading(true);
  try {
   const next = await api.runtimeReadiness(signal);
   if (!validStatus(next)) throw Error('Unavailable runtime status');
   if (!signal.aborted) { setStatus(next); setAvailability('known'); }
  } catch (reason) {
   if (!signal.aborted) {
    setStatus(null);
    setAvailability(reason instanceof APIError && reason.status === 404 ? 'unsupported' : 'unavailable');
   }
  } finally {
   if (read.current === controller) { read.current = null; if (!owner.signal.aborted) setReading(false); }
  }
 }, []);
 useEffect(() => {
  const owner = new AbortController(); lifetime.current = owner;
  void refresh();
  return () => { owner.abort(); invalidateRead(); operation.current?.abort(); };
 }, [refresh]);
 useEffect(() => {
  if (!busy) return;
  const timer = setInterval(() => void refresh(), 1000);
  return () => clearInterval(timer);
 }, [busy, refresh]);
 const prepare = async () => {
  const owner = lifetime.current;
  if (!canPrepare || !owner || owner.signal.aborted || operation.current || availability !== 'known' || !status || status.state === 'ready' || busy) return;
  const controller = new AbortController(); operation.current = controller;
  invalidateRead(); setReading(false); setWorking(true);
  try {
   const next = await api.prepareRuntime(AbortSignal.any([owner.signal, controller.signal]));
   if (!validStatus(next)) throw Error('Unavailable runtime status');
   if (!owner.signal.aborted && !controller.signal.aborted) {
    invalidateRead(); setReading(false); setStatus(next); setAvailability('known');
   }
  } catch {
   if (!owner.signal.aborted) {
    // A failed response or cancellation is not proof the service is idle. Only
    // a fresh read can enable another explicit preparation attempt.
    invalidateRead(); setStatus(null); setAvailability('checking');
    await refresh();
   }
  } finally {
   if (operation.current === controller) operation.current = null;
   if (!owner.signal.aborted) setWorking(false);
  }
 };
 const amount = (bytes: number) => `${(bytes / 1048576).toLocaleString(locale, { maximumFractionDigits: 1 })} MiB`;
 return <section className="setup-guidance runtime-preparation" aria-label={copy.title}>
  <strong>{copy.title}{status ? ` · ${status.mode.toUpperCase()}` : ''}</strong>
  <p role="status">{availability === 'unsupported' ? copy.unsupported : availability === 'unavailable' ? copy.unavailable : availability === 'checking' ? text.status.checking : status?.state === 'ready' ? copy.ready : status && ['failed', 'cancelled'].includes(status.state) ? copy.failed : busy ? text.status.checking : status?.download_required ? copy.notice : copy.check}</p>
  {status?.gpu_detected && <p>{copy.gpu}</p>}
  {!!status?.download_bytes && status.state !== 'ready' && <p>{amount(status.downloaded_bytes)} / {amount(status.download_bytes)}</p>}
  {busy && status && <progress aria-label={copy.title} max={status.download_bytes || 1} value={status.state === 'downloading' && status.download_bytes ? status.downloaded_bytes : undefined} />}
  {canPrepare && availability === 'known' && status && status.state !== 'ready' && !busy && <button className="secondary-button" onClick={() => void prepare()}>{copy.prepare}</button>}
  {!canPrepare && availability === 'known' && status && status.state !== 'ready' && <p>{copy.admin}</p>}
  {working && <button className="text-button" onClick={() => operation.current?.abort()}>{text.models.cancel}</button>}
  {availability === 'unavailable' && <button className="text-button" disabled={reading} aria-label={`${copy.title}: ${text.common.retry}`} onClick={() => void refresh()}>{text.common.retry}</button>}
 </section>;
}
