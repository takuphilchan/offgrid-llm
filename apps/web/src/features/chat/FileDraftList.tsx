import type { SessionAttachment } from '../../api/client';
import { Icon } from '../../components/Icon';
import { useI18n } from '../../i18n';
import { chatFiles } from '../../i18n/chat-files';
import { documentRecovery } from '../../i18n/document-recovery';
import { knowledgeImport } from '../../i18n/knowledge-import';

export function FileDraftList({ files, disabled, retry, remove }: {
  files: SessionAttachment[];
  disabled: boolean;
  retry: (file: SessionAttachment) => void;
  remove: (file: SessionAttachment) => void;
}) {
  const { locale } = useI18n(), copy = chatFiles(locale);
  if (!files.length) return null;
  return <ul className="file-attachments" aria-label={copy.files}>
    {files.map(file => <li key={file.id} data-state={file.status}>
      <div className="file-summary">
        <strong title={file.name}><bdi>{file.name}</bdi></strong>
        <small>{file.status === 'ready' ? copy.ready : knowledgeImport(locale).failed} · {Math.ceil(file.bytes / 1024)} KiB</small>
      </div>
      {file.status !== 'ready' && <p className="file-recovery">{documentRecovery(locale, file.error_code) ?? copy.blocked}</p>}
      <div className="file-actions">
        {file.status !== 'ready' && <button type="button" className="text-button" disabled={disabled} onClick={() => retry(file)}>{copy.retry}</button>}
        <button type="button" className="icon-button" disabled={disabled} aria-label={`${copy.remove}: ${file.name}`} onClick={() => remove(file)}><Icon name="close" size={16} /></button>
      </div>
    </li>)}
  </ul>;
}
