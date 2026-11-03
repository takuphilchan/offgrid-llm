import { useEffect, useMemo, useRef, useState } from 'react';
import type { SessionMessage } from '../../api/client';
import { DocumentSourceDialog } from '../knowledge/DocumentSourceDialog';
import { workflow } from '../../i18n/workflow';
import { useI18n } from '../../i18n';
import { knowledgeScope, knowledgeSourceDetails } from '../../i18n/knowledge-scope';
import { knowledgeUnitLabels } from '../../i18n/knowledge-locators';
import './knowledge-scope.css';
import { api } from '../../api/client';
import { useDocumentReader } from './DocumentWorkspace';

export function KnowledgePassages({ passages }: { passages?: SessionMessage['knowledge_sources'] }) {
  const { locale } = useI18n(), copy = knowledgeScope[locale];
  const openReader = useDocumentReader();
  const [error, setError] = useState('');
  const pending = useRef<AbortController | null>(null);
  useEffect(() => () => pending.current?.abort(), [passages]);
  const openSource = async (passage: NonNullable<SessionMessage['knowledge_sources']>[number]) => {
    if (!openReader || !passage.source) { setSelected(passage); return; }
    const citation = {...passage.source, content_hash:passage.content_hash};
    pending.current?.abort();
    const controller = new AbortController(); pending.current = controller;
    setError('');
    try {
      const verified = await api.documentSource(passage.document_id,citation,controller.signal);
      if (controller.signal.aborted) return;
      if (verified.citation_verified !== true) throw Error(workflow[locale].sourceMissing);
      if (verified.document.original_retained !== true) { setSelected(passage); return; }
      openReader({name:passage.name, path:`/v1/documents/source?id=${encodeURIComponent(passage.document_id)}&format=preview`, sha256:citation.source_sha256, citation, verifiedText:verified.content, original:()=>api.documentOriginal(passage.document_id)});
    } catch(reason) { if (!controller.signal.aborted) setError(reason instanceof Error ? reason.message : workflow[locale].sourceMissing); }
  };
  const [selected, setSelected] = useState<NonNullable<SessionMessage['knowledge_sources']>[number]>();
  const citation = useMemo(() => selected?.source ? { ...selected.source, content_hash: selected.content_hash } : undefined, [selected]);
  if (!passages?.length) return null;
  return <><details className="knowledge-passages"><summary>{copy.passages} · {passages.length}</summary>
    <p>{copy.evidence}</p>
    {passages.map((passage, index) => <section key={`${passage.document_id}:${passage.chunk_id}:${index}`}>
      <h4><span>[{index + 1}]</span> <bdi>{passage.name}</bdi></h4>
      {passage.source && Object.hasOwn(knowledgeUnitLabels[locale], passage.source.kind) && Number.isSafeInteger(passage.source.index) && passage.source.index > 0 && <p><bdi>{knowledgeUnitLabels[locale][passage.source.kind]} {new Intl.NumberFormat(locale).format(passage.source.index)}{passage.source.name ? ` · ${passage.source.name}` : ''}</bdi></p>}
      <blockquote dir="auto">{passage.content}</blockquote>
      {passage.source?.kind === 'sheet' && !!passage.source.cells?.length && <p className="workspace-secondary"><bdi>{passage.source.cells.join(', ')}</bdi></p>}
      {passage.source && <button className="text-button" onClick={() => void openSource(passage)}>{workflow[locale].viewSource}</button>}
      <details className="knowledge-passage-details"><summary>{knowledgeSourceDetails[locale]}</summary><small dir="ltr">{passage.document_id} · {passage.chunk_id} · SHA-256: {passage.content_hash}</small></details>
    </section>)}
  </details>{error && <p role="alert">{error}</p>}{selected && citation && <DocumentSourceDialog id={selected.document_id} citation={citation} close={() => setSelected(undefined)} />}</>;
}
