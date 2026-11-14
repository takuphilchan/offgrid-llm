import {useEffect,useRef,useState} from 'react';
import {api} from '../../api/client';
import type {components} from '../../api/schema.generated';
import {useI18n} from '../../i18n';
import {workChangesText} from '../../i18n/work-changes';
import {workChangeRecoveryText} from '../../i18n/work-change-recovery';
import {formatBytes} from '../models/model-format';

type Schema = components['schemas'];
type Detail = Schema['WorkChangeDetail'];
type View = 'files' | 'validation' | 'integration' | 'recovery';

export function WorkChangeDetails({item}: {item: Schema['WorkChange']}) {
  const {locale,messages} = useI18n(), copy = {...workChangesText(locale),...workChangeRecoveryText(locale)};
  const [open,setOpen] = useState(false), [view,setView] = useState<View>('files'), [detail,setDetail] = useState<Detail | null>(null);
  const [busy,setBusy] = useState(false), [error,setError] = useState('');
  const pending = useRef<AbortController | null>(null);
  const status = (value: string) => ({integrated:copy.integrated,partial:copy.partial,conflict:copy.conflict,uncertain:copy.uncertain,passed:copy.passed,failed:copy.failed,abandoned:copy.failed}[value] ?? copy.unverified);
  const load = async (offset = 0) => {
    pending.current?.abort(); const controller = new AbortController(); pending.current = controller;
    setBusy(true); setError('');
    try {
      const page = await api.conversations.workChange(item.work_id,item.id,view,offset,'',controller.signal);
      if (controller.signal.aborted) return;
      if (page.change.id !== item.id || page.change.work_id !== item.work_id || page.view !== view || page.next_offset !== undefined && page.next_offset <= offset) throw new Error(messages.common.error);
      setDetail(old => offset && old ? {...page,files:[...(old.files ?? []),...(page.files ?? [])],checks:[...(old.checks ?? []),...(page.checks ?? [])],receipts:[...(old.receipts ?? []),...(page.receipts ?? [])],recovery:[...(old.recovery ?? []),...(page.recovery ?? [])]} : page);
    } catch (reason) {if (!controller.signal.aborted) {setDetail(null); setError(reason instanceof Error ? reason.message : messages.common.error);}}
    finally {if (!controller.signal.aborted) setBusy(false);}
  };
  useEffect(() => {setDetail(null); if (open && item.available) void load(); return () => pending.current?.abort();}, [open,view,item.id,item.state,item.available]);
  const download = async (path: string, hashExpected: string, size: number, artifact = '') => {
    if (!hashExpected || busy) return;
    pending.current?.abort(); const controller = new AbortController(); pending.current = controller;
    setBusy(true); setError('');
    try {
      const chunks: Uint8Array<ArrayBuffer>[] = []; let offset = 0;
      for (;;) {
        const contentView = artifact ? 'preserved' : 'content';
        const page = await api.conversations.workChange(item.work_id,item.id,contentView,offset,hashExpected,controller.signal,artifact);
        const content = page.content;
        if (page.change.id !== item.id || page.change.work_id !== item.work_id || page.view !== contentView || !content || content.sha256 !== hashExpected || content.offset !== offset || content.bytes !== size) throw new Error(messages.common.error);
        const bytes = Uint8Array.from(atob(content.data),char => char.charCodeAt(0)); chunks.push(bytes); offset += bytes.length;
        if (page.next_offset === undefined) {if (offset !== content.bytes) throw new Error(messages.common.error); break;}
        if (bytes.length === 0 || page.next_offset !== offset || offset > content.bytes) throw new Error(messages.common.error);
      }
      const blob = new Blob(chunks,{type:'application/octet-stream'});
      const hash = Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256',await blob.arrayBuffer())),byte => byte.toString(16).padStart(2,'0')).join('');
      if (hash !== hashExpected) throw new Error(messages.common.error);
      if (controller.signal.aborted) return;
      const url = URL.createObjectURL(blob), link = document.createElement('a'); link.href = url; link.download = (artifact ? 'retained-' : '') + (path.split('/').at(-1) || 'change'); link.click(); setTimeout(() => URL.revokeObjectURL(url),1000);
    } catch (reason) {if (!controller.signal.aborted) {setDetail(null); setError(reason instanceof Error ? reason.message : messages.common.error);}}
    finally {if (!controller.signal.aborted) setBusy(false);}
  };
  return <details className="work-change" onToggle={event => {event.stopPropagation(); setOpen(event.currentTarget.open);}}>
    <summary>{item.kind === 'candidate' ? copy.candidate : copy.proposal} · {status(item.state)} · {item.operation_count} {copy.files}</summary>
    {open && <div className="work-change-body" aria-busy={busy}>
      {!item.current && <p>{copy.historical}</p>}
      {!item.available ? <p role="status">{copy.unavailable}</p> : <>
        <div className="button-row">{(['files','validation','integration','recovery'] as const).filter(key => key !== 'integration' && key !== 'recovery' || item.integration_id).filter(key => key !== 'validation' || item.validation_state).map(key => <button type="button" className="secondary-button" aria-pressed={view === key} key={key} onClick={() => setView(key)}>{copy[key]}</button>)}<button type="button" className="secondary-button" disabled={busy} onClick={() => void load()}>{copy.refresh}</button></div>
        {error && <p role="alert">{error}</p>}
        {detail?.error && <p role="status"><code>{detail.error}</code></p>}
        {view === 'validation' && <p>{status(item.validation_state)} · {item.validation_count}</p>}
        {detail?.conflict_path && <p><strong>{copy.conflict}</strong>: <bdi>{detail.conflict_path}</bdi></p>}
        <ul>{detail?.files?.map(file => <li key={file.path}><bdi>{file.path}</bdi> · {({create:copy.created,update:copy.updated,delete:copy.deleted}[file.kind] ?? file.kind)} {file.result && <span>· {formatBytes(file.result.bytes,locale)}</span>} {file.result?.sha256 && <button type="button" className="utility-button" disabled={busy} onClick={() => void download(file.path,file.result!.sha256!,file.result!.bytes)}>{copy.download}</button>}</li>)}</ul>
        <ul>{detail?.checks?.map(check => <li key={check.id}>{status(check.state === 'exited' && check.exit_code === 0 ? 'passed' : check.state)} {check.exit_code !== undefined && <code>{check.exit_code}</code>}</li>)}</ul>
        <ul>{detail?.receipts?.map(receipt => <li key={receipt.ordinal}><bdi>{receipt.path}</bdi> · {status(receipt.state)} {receipt.original_retained && <span>· {copy.retained}</span>}</li>)}</ul>
        {view === 'recovery' && <p>{copy.recoveryHint}</p>}
        <ul>{detail?.recovery?.map(file => <li key={file.id}><bdi>{file.path}</bdi> · {file.role === 'displaced' ? copy.retained : copy.uncertain} · {formatBytes(file.bytes,locale)} {file.sha256 && !file.limitation ? <button type="button" className="utility-button" disabled={busy} onClick={() => void download(file.path,file.sha256!,file.bytes,file.id)}>{copy.downloadRetained}</button> : <span>{copy.unverified}</span>}</li>)}</ul>
        {detail?.next_offset !== undefined && <button type="button" className="secondary-button" disabled={busy} onClick={() => void load(detail.next_offset)}>{copy.more}</button>}
      </>}
      <a href={`#/agents/task/${encodeURIComponent(item.worker_run_id)}`}>{copy.worker}</a>
    </div>}
  </details>;
}
