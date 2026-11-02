import { useEffect, useRef, useState } from 'react';
import { api, type Document } from '../../api/client';

// Optional resource discovery must not hold the ordinary profile form hostage.
// Refresh only after the user opens resources (or reviews attached resources).
export function useProfileResources(knowledgePermitted: boolean, fallbackError: string) {
  const [resources, setResources] = useState<Awaited<ReturnType<typeof api.agentProfileResources>>>();
  const [documents, setDocuments] = useState<Document[]>([]);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const requested = useRef(false), generation = useRef(0);
  const load = async () => {
    requested.current = true;
    const current = ++generation.current;
    setLoading(true); setError('');
    if (!knowledgePermitted) setDocuments([]);
    const results = await Promise.allSettled([api.agentProfileResources(), knowledgePermitted ? api.documents() : Promise.resolve({ documents: [] })]);
    if (current !== generation.current) return;
    setResources(results[0].status === 'fulfilled' ? results[0].value : undefined);
    setDocuments(results[1].status === 'fulfilled' ? results[1].value.documents : []);
    const failed = results.find(result => result.status === 'rejected');
    setError(failed?.status === 'rejected' ? (failed.reason instanceof Error ? failed.reason.message : fallbackError) : '');
    setLoading(false);
  };
  useEffect(() => () => { generation.current++; }, []);
  return { resources, documents, error, loading, load, refresh: () => requested.current ? load() : Promise.resolve() };
}
