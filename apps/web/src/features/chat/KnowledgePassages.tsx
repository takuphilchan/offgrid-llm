import type { SessionMessage } from '../../api/client';
import { useI18n } from '../../i18n';
import { knowledgeScope, knowledgeSourceDetails } from '../../i18n/knowledge-scope';
import './knowledge-scope.css';

export function KnowledgePassages({ passages }: { passages?: SessionMessage['knowledge_sources'] }) {
  const { locale } = useI18n(), copy = knowledgeScope[locale];
  if (!passages?.length) return null;
  return <details className="knowledge-passages"><summary>{copy.passages} · {passages.length}</summary>
    <p>{copy.evidence}</p>
    {passages.map((passage, index) => <section key={`${passage.document_id}:${passage.chunk_id}:${index}`}>
      <h4><span>[{index + 1}]</span> <bdi>{passage.name}</bdi></h4>
      <blockquote dir="auto">{passage.content}</blockquote>
      <details className="knowledge-passage-details"><summary>{knowledgeSourceDetails[locale]}</summary><small dir="ltr">{passage.document_id} · {passage.chunk_id} · SHA-256: {passage.content_hash}</small></details>
    </section>)}
  </details>;
}
