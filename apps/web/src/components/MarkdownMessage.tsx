import { isValidElement, memo, useState, type ReactNode } from 'react';
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
export const MarkdownMessage = memo(function MarkdownMessage({ content, streaming = false }: { content: string; streaming?: boolean }) {
  if (streaming || content.length > 4096) return <AsyncMarkdown content={content} components={components} />;
  return <Markdown remarkPlugins={[remarkGfm]} components={components} urlTransform={safeUrl}>{content}</Markdown>;
});
