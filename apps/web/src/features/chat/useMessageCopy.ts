import { useState } from 'react';
import { copyChatMessage } from './chat-presentation';

export function useMessageCopy(beforeCopy: () => void, failed: (message: string) => void, fallback: string) {
  const [copiedMessage, setCopiedMessage] = useState('');
  const copyMessage = async (key: string, content: string) => {
    beforeCopy();
    await copyChatMessage(key, content, setCopiedMessage, failed, fallback);
  };
  return { copiedMessage, setCopiedMessage, copyMessage };
}
