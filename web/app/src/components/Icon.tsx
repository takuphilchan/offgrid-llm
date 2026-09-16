import type { ReactNode } from 'react';

export type IconName = 'chat' | 'knowledge' | 'agents' | 'models' | 'activity' | 'settings' | 'send' | 'upload' | 'refresh' | 'plus' | 'trash' | 'check' | 'copy' | 'search' | 'menu' | 'close';

const paths: Record<IconName, string[]> = {
  chat: ['M21 15a4 4 0 0 1-4 4H8l-5 3V7a4 4 0 0 1 4-4h10a4 4 0 0 1 4 4z'],
  knowledge: ['M4 19.5A2.5 2.5 0 0 1 6.5 17H20', 'M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z'],
  agents: ['M12 3a3 3 0 1 0 0 6 3 3 0 0 0 0-6z', 'M19 13a3 3 0 1 0 0 6 3 3 0 0 0 0-6z', 'M5 13a3 3 0 1 0 0 6 3 3 0 0 0 0-6z', 'M12 9v4m-4 3h8'],
  models: ['M21 7 12 2 3 7l9 5 9-5z', 'm3 12 9 5 9-5', 'm3 17 9 5 9-5'],
  activity: ['M4 19V9m5 10V5m5 14v-7m5 7V3'],
  settings: ['M12 15.5a3.5 3.5 0 1 0 0-7 3.5 3.5 0 0 0 0 7z', 'M12 2v3m0 14v3M4.93 4.93l2.12 2.12m9.9 9.9 2.12 2.12M2 12h3m14 0h3M4.93 19.07l2.12-2.12m9.9-9.9 2.12-2.12'],
  send: ['m4 4 16 8-16 8 3-8-3-8z', 'M7 12h13'],
  upload: ['M12 16V4m-5 5 5-5 5 5', 'M5 20h14'],
  refresh: ['M20 6v5h-5', 'M4 18v-5h5', 'M18.5 9A7 7 0 0 0 6 6.5L4 11', 'M5.5 15A7 7 0 0 0 18 17.5l2-4.5'],
  plus: ['M12 5v14M5 12h14'],
  trash: ['M4 7h16M9 7V4h6v3m-8 0 1 13h8l1-13M10 11v5m4-5v5'],
  check: ['m5 12 4 4L19 6'],
  copy: ['M8 8h11v11H8z', 'M5 16H4V5h11v1'],
  search: ['m21 21-4.4-4.4', 'M19 11a8 8 0 1 1-16 0 8 8 0 0 1 16 0z'],
  menu: ['M4 6h16M4 12h16M4 18h16'],
  close: ['M18 6 6 18M6 6l12 12']
};

export function Icon({ name, size = 20 }: { name: IconName; size?: number }): ReactNode {
  return <svg aria-hidden="true" width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
    {paths[name].map((path, index) => <path d={path} key={index} />)}
  </svg>;
}
