// Positive admission, not "anything except embedding". Unspecified types remain
// compatible with old services; explicit speech/unknown types fail closed.
export function supportsChat(model: { type?: string; capabilities?: string[] }): boolean {
  return (!model.type || model.type === 'llm' || model.type === 'vlm' || model.type === 'chat') &&
    (model.capabilities === undefined || model.capabilities.includes('chat'));
}
