import { useMemo, useState } from 'react';
import type { SessionMessage } from '../../api/client';
import { DocumentSourceDialog } from '../knowledge/DocumentSourceDialog';
import { workflow } from '../../i18n/workflow';
import { useI18n } from '../../i18n';
import { knowledgeScope, knowledgeSourceDetails } from '../../i18n/knowledge-scope';
import { knowledgeUnitLabels } from '../../i18n/knowledge-locators';
import './knowledge-scope.css';

export function KnowledgePassages({ passages }: { passages?: SessionMessage['knowledge_sources'] }) {
  const { locale } = useI18n(), copy = knowledgeScope[locale];
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
      {passage.source && <button className="text-button" onClick={() => setSelected(passage)}>{workflow[locale].viewSource}</button>}
      <details className="knowledge-passage-details"><summary>{knowledgeSourceDetails[locale]}</summary><small dir="ltr">{passage.document_id} · {passage.chunk_id} · SHA-256: {passage.content_hash}</small></details>
    </section>)}
  </details>{selected && citation && <DocumentSourceDialog id={selected.document_id} citation={citation} close={() => setSelected(undefined)} />}</>;
}
