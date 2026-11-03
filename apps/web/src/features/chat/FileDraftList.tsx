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
  if (!files.length) return null;
  return <ul className="file-attachments" aria-label={copy.files}>
    {files.map(file => <li key={file.id} data-state={file.status}>
      <div className="file-summary">
        <strong title={file.name}><bdi>{file.name}</bdi></strong>
        <small>{file.status === 'ready' ? copy.ready : file.status === 'uploaded' ? copy.preparing : knowledgeImport(locale).failed} · {Math.ceil(file.bytes / 1024)} KiB</small>
      </div>
      {file.status !== 'ready' && <p className="file-recovery">{documentRecovery(locale, file.error_code) ?? copy.blocked}</p>}
      {file.status === 'ready' && file.coverage === 'native_text_only' && <p className="document-coverage">{documentCoverage(locale)}</p>}
      <div className="file-actions">
        {open && <button type="button" className="text-button" onClick={() => open({name:file.name,sha256:file.sha256,path:`/v1/sessions/${encodeURIComponent(session)}/attachments/${file.id}/preview`,original:()=>api.sessionAttachmentOriginal(session,file.id)})}>{reader.open}</button>}
        {file.status === 'failed' && ['document_ocr_required','document_ocr_language_missing','document_ocr_overlap','document_ocr_low_confidence'].includes(file.error_code ?? '') && <button type="button" className="text-button" disabled={disabled} onClick={() => textOnly(file)}>{reader.textOnly}</button>}
        {file.status !== 'ready' && <button type="button" className="text-button" disabled={disabled} onClick={() => retry(file)}>{copy.retry}</button>}
        <button type="button" className="icon-button" disabled={disabled} aria-label={`${copy.remove}: ${file.name}`} onClick={() => remove(file)}><Icon name="close" size={16} /></button>
      </div>
    </li>)}
  </ul>;
}
