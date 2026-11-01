import type { ChatMessage } from '../../api/client';
import { readPreference } from '../../lib/preferences';

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
  const title = prompt.replace(/\s+/g, ' ').slice(0, 42).trim();
  return `${title || 'Chat'} · ${stamp}`;
}

export function visibleSessionName(name: string): string {
  return name.replace(/ · \d{14}$/, '');
}
