import { Fragment, type ReactNode } from 'react';
import type { ChatMessage, SessionMessage } from '../../api/client';
import type { ChatMetrics } from '../../api/session-stream';
import { useI18n } from '../../i18n';
import { Icon } from '../../components/Icon';
import { MarkdownMessage } from '../../components/MarkdownMessage';
import { ReadAloudButton } from '../../components/VoiceInputButton';
import { documentExcerptCoverage } from '../../i18n/document-coverage';
import { conversationWorkText } from '../../i18n/conversation-work';
import { AttachmentAnswer } from './AttachmentAnswer';
import { KnowledgePassages } from './KnowledgePassages';
import { ImageAttachments } from './ComposerImages';
import { FileAttachments } from './ComposerFiles';
import { ChatResponseDetails } from './ChatResponseDetails';
import { messageText } from './chat-presentation';

export type ConversationMessage = ChatMessage & Pick<Partial<SessionMessage>, 'knowledge_sources' | 'images' | 'attachments'>;

type Props = {
  conversation: ConversationMessage[];
  activeName: string;
  busy: boolean;
  linkedAnswerIndex: number;
  linkedWork: ReactNode;
  linkedJob?: string;
  lastAssistant: number;
  metrics?: ChatMetrics;
  copiedMessage: string;
  copyMessage: (key: string, content: string) => Promise<void>;
};

export function ConversationMessages({ conversation, activeName, busy, linkedAnswerIndex, linkedWork, linkedJob, lastAssistant, metrics, copiedMessage, copyMessage }: Props) {
  const { messages: text, locale } = useI18n();
  const jobCopy = conversationWorkText[locale];
  return <>{conversation.map((message, index) => {
          const content = messageText(message.content);
          const messageKey = `${index}-${'timestamp' in message ? message.timestamp : content.slice(0, 24)}`;
          return <Fragment key={messageKey}>
          {index === linkedAnswerIndex && linkedWork}
          <article className={`message ${message.role}`}>
            <div className="message-heading"><div className="message-label">{message.role === 'user' ? text.chat.you : message.role === 'assistant' ? text.chat.assistant : message.role}</div></div>
            <div className="message-body">{message.role === 'assistant' ? 'attachment_selections' in message && Array.isArray(message.attachment_selections) && message.attachment_selections.length ? <AttachmentAnswer content={content} session={activeName} selections={message.attachment_selections} /> : <MarkdownMessage content={content} /> : content}</div>
            <ImageAttachments images={message.images}/>
            <FileAttachments name={activeName} references={message.attachments} active={busy && index === conversation.length-1}/>
            {message.role === 'assistant' && <KnowledgePassages passages={message.knowledge_sources} />}
            {message.role === 'assistant' && 'attachment_selections' in message && Array.isArray(message.attachment_selections) && message.attachment_selections.some(source => source.mode === 'excerpts') && <p className="document-answer-coverage">{documentExcerptCoverage(locale)}</p>}
            {message.role === 'assistant' && <footer className="message-actions">
              <button type="button" className="message-copy" onClick={() => void copyMessage(messageKey, content)} aria-label={copiedMessage === messageKey ? text.chat.copied : text.chat.copyResponse}><Icon name={copiedMessage === messageKey ? 'check' : 'copy'} size={14} />{copiedMessage === messageKey ? text.chat.copied : text.chat.copy}</button>
              <ReadAloudButton text={content} />
              {'job_id' in message && typeof message.job_id === 'string' && message.job_id !== linkedJob && <a className="secondary-button message-job-link" href={`#/agents/task/${encodeURIComponent(message.job_id)}`}>{jobCopy.inspect}</a>}
              {metrics && index === lastAssistant && <ChatResponseDetails metrics={metrics}/>}
            </footer>}
          </article></Fragment>;
        })}</>;
}
