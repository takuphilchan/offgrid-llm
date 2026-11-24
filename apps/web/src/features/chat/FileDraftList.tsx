import type { SessionAttachment } from '../../api/client';
import { Icon } from '../../components/Icon';
import { useI18n } from '../../i18n';
import { chatFiles } from '../../i18n/chat-files';
import { documentRecovery } from '../../i18n/document-recovery';
import { knowledgeImport } from '../../i18n/knowledge-import';
import { useDocumentReader } from './DocumentWorkspace';
import { api } from '../../api/client';
import { documentReader } from '../../i18n/document-reader';
import { documentCoverage } from '../../i18n/document-coverage';
import {DocumentEvidenceNotice} from '../../components/DocumentEvidenceNotice';
import {ocrProgress} from '../../i18n/ocr-progress';
import {documentNavigation} from '../../i18n/document-navigation';

export function FileDraftList({ files, disabled, retry, remove, session, textOnly }: {
  files: SessionAttachment[];
  disabled: boolean;
  retry: (file: SessionAttachment) => void;
  remove: (file: SessionAttachment) => void;
  session: string;
  textOnly: (file: SessionAttachment) => void;
}) {
  const { locale } = useI18n(), copy = chatFiles(locale);
  const open = useDocumentReader(), reader = documentReader(locale);
  const summary=(file:SessionAttachment)=><><span className="file-type-mark"><Icon name="file" size={28}/><span>{file.name.split('.').at(-1)?.slice(0,8).toUpperCase()}</span></span><span className="file-summary-copy"><strong title={file.name}><bdi>{file.name}</bdi></strong><small>{file.status === 'ready' ? copy.ready : file.status === 'uploaded' ? copy.preparing : knowledgeImport(locale).failed} · {new Intl.NumberFormat(locale,{maximumFractionDigits:1}).format(file.bytes>=1048576?file.bytes/1048576:Math.ceil(file.bytes/1024))} {file.bytes>=1048576?'MiB':'KiB'}</small></span></>;
  if (!files.length) return null;
  return <ul className="file-attachments" aria-label={copy.files}>
    {files.map(file => <li key={file.id} data-state={file.status}>
      {open?<button type="button" className="file-summary file-open" aria-label={reader.open} title={reader.open+': '+file.name} onClick={() => open({name:file.name,sha256:file.sha256,path:`/v1/sessions/${encodeURIComponent(session)}/attachments/${file.id}/preview`,original:()=>api.sessionAttachmentOriginal(session,file.id),extractionRevision:file.extraction_revision,attachment:file.revision?{session,id:file.id,revision:file.revision}:undefined})}>{summary(file)}</button>:<div className="file-summary">{summary(file)}</div>}
      <button type="button" className="icon-button file-remove" disabled={disabled} aria-label={`${copy.remove}: ${file.name}`} onClick={() => remove(file)}><Icon name="close" size={16} /></button>
      {file.status !== 'ready' && <p className="file-recovery">{documentRecovery(locale, file.error_code) ?? copy.blocked}</p>}
      {file.status !== 'ready' && !!file.ocr_completed_pages && <p className="workspace-secondary">{ocrProgress(locale,file.ocr_completed_pages)}</p>}
      {(file.coverage==='native_text_only'||file.evidence&&file.evidence.status!=='complete') && <details className="file-coverage-details"><summary><Icon name="info" size={13}/>{documentNavigation(locale).coverage}</summary>{file.coverage==='native_text_only'?<p className="document-coverage">{documentCoverage(locale)}</p>:<DocumentEvidenceNotice evidence={file.evidence}/>}</details>}
      <div className="file-actions">
        {file.status === 'failed' && ['document_ocr_required','document_ocr_language_missing','document_ocr_overlap','document_ocr_low_confidence'].includes(file.error_code ?? '') && <button type="button" className="text-button" disabled={disabled} onClick={() => textOnly(file)}>{reader.textOnly}</button>}
        {file.status !== 'ready' && <button type="button" className="text-button" disabled={disabled} onClick={() => retry(file)}>{copy.retry}</button>}
      </div>
    </li>)}
  </ul>;
}
