import type { ChatMessage, ChatSession, Model } from '../../api/client';
import { readPreference } from '../../lib/preferences';
import { copyText } from '../../lib/clipboard';

export type ChatPageProps = {
  scope: string;
  models: Model[];
  model: string;
  setModel: (model: string) => void;
  onboardingPending: boolean;
  onFirstResponse: () => void;
  onOpenModels: () => void;
};

export async function copyChatMessage(key:string,content:string,copied:(key:string)=>void,failed:(message:string)=>void,fallback:string){
  try { await copyText(content); copied(key); }
  catch(reason){ failed(reason instanceof Error?reason.message:fallback); }
}

export function readPreferences(key: string): { profile: string; maxTokens: number } {
  try {
    const value = JSON.parse(readPreference(key) ?? '{}');
    return { profile: value.profile === 'extended' ? 'extended' : 'interactive', maxTokens: [256, 1024, 4096].includes(value.maxTokens) ? value.maxTokens : 1024 };
  } catch { return { profile: 'interactive', maxTokens: 1024 }; }
}

export function messageText(content: ChatMessage['content']): string {
  if (typeof content === 'string') return content;
  return content.filter(part => part.type === 'text').map(part => part.text).join('');
}

export function newSessionName(prompt: string): string {
  const stamp = new Date().toISOString().replace(/\D/g, '').slice(0, 14);
  // Session names are also file names on native hosts. Keep the original
  // prompt in the message; only the generated storage title is normalized.
  const cleaned = prompt.replace(/[<>:"/\\|?*\u0000-\u001f\u007f]/g, ' ').replace(/\s+/g, ' ').trim();
  let title = '';
  for (const character of Array.from(cleaned).slice(0, 42)) {
    if (new TextEncoder().encode(title + character).length > 96) break;
    title += character;
  }
  title = title.trim();
  return `${title || 'Chat'} · ${stamp}`;
}

export function visibleSessionName(name: string): string {
  return name.replace(/ · \d{14}$/, '');
}

export function visibleSessionTitle(session: ChatSession): string {
  if (/^agent-[a-f0-9-]{32,36}$/i.test(session.name)) {
    const first = session.messages?.find(message => message.role === 'user');
    if (first) {
      const title = messageText(first.content).replace(/\s+/g, ' ').trim().slice(0, 80);
      if (title) return title;
    }
  }
  return visibleSessionName(session.name);
}
// Metrics describe committed output, never an older answer or an in-flight
// preview. Chat clears the associated metrics when changing conversations.
export function conversationOutputState(messages: readonly { role: string; turn_id?: string }[], turnId?: string) {
  return {
    lastAssistant: messages.reduce((last, message, index) => message.role === 'assistant' ? index : last, -1),
    linkedOutputSaved: !!turnId && messages.some(message => message.turn_id === turnId && message.role === 'assistant'),
  };
}
