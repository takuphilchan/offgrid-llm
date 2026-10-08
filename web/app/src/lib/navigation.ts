export const pages = ['chat', 'knowledge', 'agents', 'models', 'activity', 'settings'] as const;
export type Page = typeof pages[number];
export type ServiceHealth = 'checking' | 'ready' | 'offline';

export const navigationGroups: { label: 'work' | 'library' | 'system'; items: Page[] }[] = [
  { label: 'work', items: ['chat', 'agents'] },
  { label: 'library', items: ['knowledge', 'models'] },
  { label: 'system', items: ['activity', 'settings'] },
];

export function pageFromLocation(): Page {
  const candidate = window.location.hash.replace(/^#\/?/, '').split('/')[0];
  return pages.includes(candidate as Page) ? candidate as Page : 'chat';
}
