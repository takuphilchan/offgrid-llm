import { useEffect, useRef, useState } from 'react';
import { api } from '../../api/client';
import type { EndpointOwnerSnapshot } from '../../api/endpoint-owner';
import type { EndpointReceiptHistory, EndpointPolicyHistory } from '../../api/endpoint-history';
import { useI18n } from '../../i18n';
import { endpointOwner } from '../../i18n/endpoint-owner';
import { endpointMaintenance } from '../../i18n/endpoint-maintenance';

type View = { kind: 'receipts'; data: EndpointReceiptHistory; observed: string } | { kind: 'policies'; data: EndpointPolicyHistory | null; observed: string };

/** Historical evidence is read-only and never supplies an enable/replay action. */
export function EndpointOwnerHistory({ snapshot, busy }: { snapshot: EndpointOwnerSnapshot; busy: boolean }) {
 const { locale, messages } = useI18n(); const copy = endpointMaintenance[locale], owner = endpointOwner[locale];
 const [view, setView] = useState<View | null>(null), [kind, setKind] = useState<View['kind'] | null>(null);
 const [loading, setLoading] = useState(false), [error, setError] = useState('');
 const controller = useRef<AbortController | null>(null);
 useEffect(() => {
  controller.current?.abort(); setView(null); setKind(null); setLoading(false); setError('');
  return () => controller.current?.abort();
 }, [snapshot.journal_revision, snapshot.policy_maintenance?.policy_revision, snapshot.actor, busy]);
 const read = async (mode: View['kind'], next = false) => {
  if (busy || loading) return;
  const previous = view; controller.current?.abort(); const request = new AbortController(); controller.current = request;
  setKind(mode); setView(null); setError(''); setLoading(true);
  try {
   let result: View;
   if (mode === 'receipts') {
    const old = next && previous?.kind === 'receipts' ? previous.data : null;
    const data = await api.endpointOwner.history(old?.journal_revision, old?.next, request.signal);
    result = { kind: mode, data, observed: new Date().toISOString() };
   } else {
    const revision = next && previous?.kind === 'policies' ? previous.data?.history.snapshot.previous_snapshot : undefined;
    const data = await api.endpointOwner.policyHistory(revision, request.signal);
    result = { kind: mode, data, observed: new Date().toISOString() };
   }
   if (!request.signal.aborted) setView(result);
  } catch (reason) { if (!request.signal.aborted) setError(reason instanceof Error ? reason.message : messages.common.error); }
  finally { if (!request.signal.aborted) setLoading(false); }
 };
 const exportPage = () => {
  if (!view || !view.data || loading || busy) return;
  const value = { version: 1, kind: `endpoint_${view.kind}_history_page`, observed_at: view.observed, exported_by: snapshot.actor, evidence_only: true, page: view.data };
  const url = URL.createObjectURL(new Blob([JSON.stringify(value, null, 2)], { type: 'application/json;charset=utf-8' }));
  const link = document.createElement('a'); link.href = url; link.download = `offgrid-endpoint-${view.kind}-history.json`; link.click();
  setTimeout(() => URL.revokeObjectURL(url), 30_000);
 };
 const more = view?.kind === 'receipts' ? view.data.next : view?.data?.history.snapshot.previous_snapshot;
 return <section className="endpoint-owner-history stack" aria-label={owner.history} aria-busy={loading}>
  <div className="model-actions"><button disabled={busy || loading} onClick={() => void read('receipts')}>{copy.receipts}</button><button disabled={busy || loading} onClick={() => void read('policies')}>{copy.policies}</button></div>
  {kind && <><h4>{copy[kind]}</h4><p>{copy.evidenceOnly}</p>
   {loading && <p role="status">{messages.common.loading}</p>}{error && <p role="alert">{error}</p>}
   {view?.kind === 'receipts' && <>
    {view.data.receipts.length === 0 && <p role="status">{messages.history.noMatches}</p>}
    {view.data.receipts.map(receipt => <article className="endpoint-owner-record" key={receipt.record.request_id}>
     <h5><bdi>{receipt.record.endpoint}</bdi></h5><p>{owner[receipt.record.state]}</p>
     <p><bdi>{receipt.record.request_id}</bdi> · <bdi>{receipt.record.actor}</bdi> · <time dateTime={receipt.record.created_at}>{new Date(receipt.record.created_at).toLocaleString(locale)}</time></p>
     <p>{copy.archive}: <bdi>{receipt.archived_by}</bdi> · <time dateTime={receipt.archived_at}>{new Date(receipt.archived_at).toLocaleString(locale)}</time></p>
     <details><summary>{owner.review}</summary><p><bdi>{receipt.record.before}</bdi> → <bdi>{receipt.record.after}</bdi></p></details>
    </article>)}
   </>}
   {view?.kind === 'policies' && (view.data ? <>
    <p><bdi>{view.data.revision}</bdi></p><p><bdi>{view.data.history.actor}</bdi> · <time dateTime={view.data.history.at}>{new Date(view.data.history.at).toLocaleString(locale)}</time></p>
    {Object.entries(view.data.history.snapshot.decisions).map(([key, decision]) => <article className="endpoint-owner-record" key={key}>
     <h5><bdi>{key}</bdi></h5><p>{decision.enabled ? copy.allowedThen : copy.deniedThen}</p><p><bdi>{decision.actor}</bdi> · <time dateTime={decision.at}>{new Date(decision.at).toLocaleString(locale)}</time></p>
    </article>)}
   </> : <p role="status">{messages.history.noMatches}</p>)}
   <div className="model-actions"><button disabled={busy || loading} onClick={() => void read(kind)}>{messages.common.refresh}</button><button disabled={busy || loading || !more} onClick={() => void read(kind, true)}>{messages.history.showMore}</button><button disabled={busy || loading || !view?.data} onClick={exportPage}>{copy.exportPage}</button></div>
  </>}
 </section>;
}
