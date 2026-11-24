import { useEffect, useRef, useState, type ReactNode } from 'react';
import { api, type ModelOperation } from '../../api/client';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { useI18n } from '../../i18n';
import { modelAcquisition } from '../../i18n/model-acquisition';
import { operationFeedback } from '../../i18n/operation-feedback';
import { useWorkspace } from '../../lib/workspace-context';
import { PackageOperation } from './PackageOperation';

export function PackageTransfers({ operations, error, refresh, legacyCount, legacyActive, children }: { operations: ModelOperation[]; error: string; refresh: () => Promise<void>; legacyCount: number; legacyActive: boolean; children: ReactNode }) {
  const { admin } = useWorkspace(), { locale, messages: text } = useI18n();
  const copy = modelAcquisition[locale], feedback = operationFeedback(locale);
  const [busy, setBusy] = useState(false), [failure, setFailure] = useState('');
  const [expanded, setExpanded] = useState<boolean>();
  const [discard, setDiscard] = useState<ModelOperation | null>(null);
  const lock = useRef(false), mounted = useRef(true);
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; }; }, []);
  const control = async (operation: ModelOperation, action: 'cancel' | 'resume' | 'discard') => {
    if (!admin || lock.current) return;
    lock.current = true; setBusy(true); setFailure('');
    try { await api.controlModelOperation(operation.id, action); }
    catch (reason) { if (mounted.current) setFailure(reason instanceof Error ? reason.message : text.common.error); throw reason; }
    finally {
      // A missing acknowledgment is reconciled by a read, never a replay.
      await refresh(); lock.current = false;
      if (mounted.current) setBusy(false);
    }
  };
  const available = operations.filter(item => item.target.kind === 'package' && !item.discarded && (item.state !== 'complete' || item.retained_bytes > 0));
  if (!admin || (!available.length && !legacyCount && !error && !failure)) return null;
  const needsAttention = available.some(item => !['complete', 'cancelled'].includes(item.state));
  return <details className="model-transfers" open={expanded ?? (legacyActive || needsAttention || !!error || !!failure)} onToggle={event => setExpanded(event.currentTarget.open)}>
    <summary>{copy.operations} · {legacyCount + available.length}</summary>
    <div className="stack">
    {(error || failure) && <div className="inline-error" role="alert">{error || failure}<button className="secondary-button" disabled={busy} onClick={() => void refresh()}>{feedback.checkStatus}</button></div>}
    {children}
    {available.map(operation => <PackageOperation key={operation.id} operation={operation} disabled={busy} control={action => void control(operation, action).catch(() => {})} discard={() => setDiscard(operation)} />)}
    </div>
    {discard && <ConfirmDialog title={copy.discard} body={copy.discardConfirm} confirmLabel={copy.discard} close={() => setDiscard(null)} confirm={async () => { await control(discard, 'discard'); setDiscard(null); }} />}
  </details>;
}
