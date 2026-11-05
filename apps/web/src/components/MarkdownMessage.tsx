import { cloneElement, isValidElement, memo, useMemo, useState, type ReactNode } from 'react';
import Markdown, { type Components } from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { copyText } from '../lib/clipboard';
import { useI18n } from '../i18n';
import { Icon } from './Icon';
import { AsyncMarkdown } from './AsyncMarkdown';

function nodeText(node: ReactNode): string {
  if (typeof node === 'string' || typeof node === 'number') return String(node);
  if (Array.isArray(node)) return node.map(nodeText).join('');
  if (isValidElement<{ children?: ReactNode }>(node)) return nodeText(node.props.children);
  return '';
}

function CodeBlock({ children }: { children?: ReactNode }) {
  const { messages: text } = useI18n();
  const [copied, setCopied] = useState(false);
  const source = nodeText(children).replace(/\n$/, '');
  const className = isValidElement<{ className?: string }>(children) ? children.props.className ?? '' : '';
  const language = /\blanguage-([a-z0-9_+-]{1,40})\b/i.exec(className)?.[1];

  const copy = async () => {
    try {
      await copyText(source);
      setCopied(true);
    } catch { setCopied(false); }
  };

  return <div className="code-block">
    {language && <span className="code-language">{language}</span>}
    <button type="button" onClick={() => void copy()} aria-label={copied ? text.chat.copied : text.chat.copyCode}>
      <Icon name={copied ? 'check' : 'copy'} size={16} />{copied ? text.chat.copied : text.chat.copy}
    </button>
    <pre>{children}</pre>
  </div>;
}

const components: Components = {
  a: ({ href, children }) => <a href={href} target="_blank" rel="noreferrer">{children}</a>,
  img: ({ alt }) => <span className="blocked-image">[{alt || 'Image'}]</span>,
  pre: ({ children }) => <CodeBlock>{children}</CodeBlock>
};

function safeUrl(url: string): string {
  return /^(https?:|mailto:)/i.test(url.trim()) ? url : '';
}

// Input, lifecycle and unrelated progress updates must not reparse saved prose.
export const MarkdownMessage = memo(function MarkdownMessage({ content, streaming = false, renderText }: { content: string; streaming?: boolean; renderText?: (text: string) => ReactNode }) {
  const renderers = useMemo<Components>(() => {
    if (!renderText) return components;
    const transform = (node: ReactNode): ReactNode => {
      if (typeof node === 'string') return renderText(node);
      if (Array.isArray(node)) return node.map(transform);
      // Never interpret citations inside code, links or already rendered controls.
      if (isValidElement<{children?: ReactNode}>(node) && ['strong','em','del','span'].includes(String(node.type))) return cloneElement(node, {}, transform(node.props.children));
      return node;
    };
    return {...components, p: ({children}) => <p>{transform(children)}</p>, li: ({children}) => <li>{transform(children)}</li>, td: ({children}) => <td>{transform(children)}</td>, th: ({children}) => <th>{transform(children)}</th>};
  }, [renderText]);
  if (streaming || content.length > 4096) return <AsyncMarkdown content={content} components={renderers} />;
  return <Markdown remarkPlugins={[remarkGfm]} components={renderers} urlTransform={safeUrl}>{content}</Markdown>;
});
