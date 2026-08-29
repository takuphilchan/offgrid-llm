export type Messages = {
  product: string;
  privateWorkspace: string;
  nav: { chat: string; knowledge: string; agents: string; models: string; activity: string; settings: string };
  status: { ready: string; offline: string; checking: string; noModel: string };
  auth: { title: string; body: string; username: string; password: string; signIn: string; signOut: string; localNote: string };
  chat: { title: string; subtitle: string; emptyTitle: string; emptyBody: string; placeholder: string; send: string; stop: string; knowledge: string; model: string; selectModel: string; you: string; assistant: string; newChat: string; conversations: string; noConversations: string; deleteConversation: string };
  knowledge: { title: string; subtitle: string; add: string; empty: string; chunks: string; disabled: string; retained: string; legacy: string; reindex: string; reindexing: string };
  agents: { title: string; subtitle: string; task: string; placeholder: string; run: string; running: string; result: string; approvalTitle: string; approvalBody: string; approve: string; deny: string; denied: string };
  models: { title: string; subtitle: string; available: string; empty: string; local: string; embedding: string; installed: string; catalog: string; discover: string; recommended: string; download: string; resume: string; cancel: string; delete: string; confirmDelete: string; verify: string };
  activity: { title: string; subtitle: string; uptime: string; requests: string; currentModel: string; version: string; runs: string; noRuns: string; selectRun: string };
  settings: { title: string; subtitle: string; service: string; version: string; inferenceSlots: string; knowledge: string; enabled: string; disabled: string; safety: string; computerUse: string; available: string; unavailable: string; stopped: string; sessions: string; emergencyStop: string; setup: string; showGuide: string };
  onboarding: { title: string; body: string; service: string; models: string; privacy: string; local: string; continue: string };
  common: { refresh: string; retry: string; loading: string; error: string; language: string; localProcessing: string; runtime: string; document: string };
};

export type LocaleDefinition = { code: string; label: string; direction: 'ltr' | 'rtl'; messages: Messages };
