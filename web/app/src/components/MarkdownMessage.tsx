import { isValidElement, useState, type ReactNode } from 'react';
import Markdown, { type Components } from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { copyText } from '../lib/clipboard';
import { useI18n } from '../i18n';
import { Icon } from './Icon';

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

  const copy = async () => {
    try {
      await copyText(source);
      setCopied(true);
    } catch { setCopied(false); }
  };

  return <div className="code-block">
    <button type="button" onClick={() => void copy()} aria-label={copied ? text.chat.copied : text.chat.copyCode}>
      <Icon name={copied ? 'check' : 'copy'} size={14} />{copied ? text.chat.copied : text.chat.copy}
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

export function MarkdownMessage({ content }: { content: string }) {
  return <Markdown remarkPlugins={[remarkGfm]} components={components} urlTransform={safeUrl}>{content}</Markdown>;
}
