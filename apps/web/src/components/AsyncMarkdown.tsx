import { Fragment, startTransition, useEffect, useMemo, useRef, useState } from 'react';
import { jsx, jsxs } from 'react/jsx-runtime';
import { toJsxRuntime } from 'hast-util-to-jsx-runtime';
import type { Root } from 'hast';
import type { Components } from 'react-markdown';
import { forgetMarkdown, parseMarkdown } from '../lib/markdown-renderer';

export function AsyncMarkdown({ content, components }: { content: string; components: Components }) {
  const owner = useRef(Symbol('markdown'));
  const latest = useRef(content); latest.current = content;
  const [parsed, setParsed] = useState<{ source: string; tree?: Root }>();
  useEffect(() => {
    parseMarkdown(owner.current, content, tree => {
      // An append-only stream may render an intermediate result. A replacement,
      // different turn, or authoritative final answer must never show old text.
      if (latest.current.startsWith(content)) startTransition(() => setParsed({ source: content, tree }));
    });
  }, [content]);
  useEffect(() => { const key = owner.current; return () => forgetMarkdown(key); }, []);
  const rendered = useMemo(() => parsed?.tree ? toJsxRuntime(parsed.tree, { Fragment, jsx, jsxs, components, passKeys: true, passNode: true, ignoreInvalidStyle: true }) : null, [parsed?.tree, components]);
  // Source remains readable during cold parser startup/failure. No response is
  // discarded or replaced with a spinner, and final copy uses canonical text.
  if (!parsed?.tree || !content.startsWith(parsed.source)) return <div className="markdown-pending" style={{ whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }}>{content}</div>;
  return <>{rendered}</>;
}
