import { Fragment, useCallback, useEffect, useRef, useState } from 'react';
import { api } from '../../api/client';
import { MarkdownMessage } from '../../components/MarkdownMessage';
import { useI18n } from '../../i18n';
import { workflow } from '../../i18n/workflow';
import { documentReader } from '../../i18n/document-reader';
import { useDocumentReader } from './DocumentWorkspace';
import { resolveAttachmentCitation, type PassageRef, type Selection } from './attachment-citations';

export function AttachmentAnswer({content, session, selections}: {content: string; session: string; selections: Selection[]}) {
  const openReader = useDocumentReader(), {locale, messages} = useI18n();
  const [error, setError] = useState(''), [pending, setPending] = useState('');
  const request = useRef<AbortController | null>(null);
  useEffect(() => () => request.current?.abort(), [session, selections]);
  const open = useCallback(async (ref: PassageRef) => {
    request.current?.abort(); const controller = new AbortController(); request.current = controller;
    setError(''); setPending(ref.label);
    try {
      const passage = await api.sessionAttachmentPassage(session, ref.source.id, ref.source.revision, ref.start, ref.end, controller.signal);
      if (controller.signal.aborted) return;
      if (passage.id !== ref.source.id || passage.revision !== ref.source.revision || passage.start_byte !== ref.start || passage.end_byte !== ref.end || new TextEncoder().encode(passage.text).length !== ref.end-ref.start || !/^[a-f0-9]{64}$/.test(passage.sha256)) throw Error(workflow[locale].sourceMissing);
      openReader?.({name: passage.name, sha256: passage.sha256, passage: passage.text, path: `/v1/sessions/${encodeURIComponent(session)}/attachments/${encodeURIComponent(passage.id)}/preview`, original: () => api.sessionAttachmentOriginal(session, passage.id)});
    } catch (reason) { if (!controller.signal.aborted) setError(reason instanceof Error ? reason.message : messages.common.error); }
    finally { if (!controller.signal.aborted) setPending(''); }
  }, [session, openReader, locale, messages.common.error]);
  const renderText = useCallback((text: string) => text.split(/(\[(?:S[1-9]\d*(?:\.[1-9]\d*)?|[^\]\n]{1,1024}:\s*\d+\s*[-–]\s*\d+)\])/g).map((part, index) => {
    const ref = resolveAttachmentCitation(part, selections);
    return <Fragment key={index}>{ref && openReader ? <button type="button" className="citation-chip" title={documentReader(locale).open} aria-label={`${documentReader(locale).open}: ${ref.label}`} onClick={() => void open(ref)}><bdi>{ref.label}</bdi></button> : part}</Fragment>;
  }), [selections, openReader, locale, open]);
  return <><MarkdownMessage content={content} renderText={renderText} />{pending && <span className="sr-only" role="status">{messages.common.loading}</span>}{error && <p role="alert" className="citation-error">{error}</p>}</>;
}
