// Positive admission, not "anything except embedding". Unspecified types remain
// compatible with old services; explicit speech/unknown types fail closed.
export function supportsChat(model: { type?: string; capabilities?: string[] }): boolean {
  return (!model.type || model.type === 'llm' || model.type === 'vlm' || model.type === 'chat') &&
    (model.capabilities === undefined || model.capabilities.includes('chat'));
}

// An absent readiness field is the legacy service contract, not a failed check.
export function chatAvailable(model: { type?: string; capabilities?: string[]; runtime_ready?: boolean }): boolean {
  return supportsChat(model) && model.runtime_ready !== false;
}

export function initialChatModel<T extends { id: string; loaded?: boolean; type?: string; capabilities?: string[]; runtime_ready?: boolean }>(models: T[]): string {
  const available = models.filter(chatAvailable);
  return (available.find(model => model.loaded) ?? available[0])?.id ?? '';
}
