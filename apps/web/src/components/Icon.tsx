import type { ReactNode } from 'react';

export type IconName = 'browser' | 'calculator' | 'archive' | 'edit' | 'expand' | 'contract' | 'plug' | 'external' | 'tool' | 'palette' | 'quick-window' | 'shield' | 'drive' | 'info' | 'compare' | 'file-search' | 'collections' | 'home' | 'power' | 'warning' | 'image' | 'folder' | 'terminal' | 'arrow-up' | 'arrow-down' | 'download' | 'chevron-left' | 'chevron-right' | 'chevron-down' | 'file' | 'sliders' | 'help' | 'chat' | 'knowledge' | 'agents' | 'models' | 'activity' | 'settings' | 'send' | 'upload' | 'refresh' | 'plus' | 'trash' | 'check' | 'copy' | 'search' | 'menu' | 'close' | 'mic' | 'waveform' | 'stop' | 'speaker' | 'more' | 'sidebar';

const paths: Record<IconName, string[]> = {
  browser: ['M5 3h14a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2z', 'M3 8h18M7 5.5h.01M10 5.5h.01', 'm8 13 3 2-3 2M14 17h3'],
  calculator: ['M6 2h12a2 2 0 0 1 2 2v16a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2z', 'M8 6h8M8 11h.01M12 11h.01M16 11h.01M8 15h.01M12 15h.01M16 15v4M8 19h.01M12 19h.01'],
  archive: ['M3 3h18v5H3zM5 8v13h14V8M10 12h4'],
  edit: ['m16 3 5 5-12 12-6 1 1-6L16 3z', 'm13 6 5 5'],
  expand: ['M8 3H3v5M16 21h5v-5'],
  contract: ['M3 8h5V3M21 16h-5v5'],
  plug: ['M8 2v5M16 2v5M5 7h14v3a7 7 0 0 1-14 0V7z','M12 17v5'],
  external: ['M14 3h7v7M21 3l-9 9','M10 3H5a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-5'],
  tool: ['M14 6a5 5 0 0 0-6 6l-5 5a2.8 2.8 0 0 0 4 4l5-5a5 5 0 0 0 6-6l-3 3-4-4 3-3z'],
  palette: ['M12 3a9 9 0 1 0 0 18h1a2 2 0 0 0 1.4-3.4 1.5 1.5 0 0 1 1.1-2.6H17a4 4 0 0 0 4-4c0-4.4-4-8-9-8z', 'M7.5 10h.01M10 6.5h.01M15 7h.01M17.5 10.5h.01'],
  'quick-window': ['M9 8V5a2 2 0 0 1 2-2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2h-3', 'M5 9h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-8a2 2 0 0 1 2-2z', 'M6 13h6M6 17h3'],
  shield: ['M12 3 3 7v5c0 5 9 9 9 9s9-4 9-9V7l-9-4z', 'm8 12 3 3 5-6'],
  drive: ['M6 4h12l3 10v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4L6 4z', 'M3 14h18M7 17h.01M11 17h.01'],
  info: ['M12 22a10 10 0 1 0 0-20 10 10 0 0 0 0 20', 'M12 11v6M12 7h.01'],
  compare: ['M8 3H4v18h4M16 3h4v18h-4', 'M7 8h10m-3-3 3 3-3 3M17 16H7m3-3-3 3 3 3'],
  'file-search': ['M13 2H5v20h14v-3M13 2v5h6l-6-5z', 'M19 12a4 4 0 1 1-8 0 4 4 0 0 1 8 0z', 'm18 15 3 3'],
  collections: ['M2 8h7l2 2h9v9a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V8z', 'M6 5h7l2 2h7v10', 'M10 2h7l2 2h3'],
  home: ['M3 10l9-7 9 7','M5 9v12h5v-7h4v7h5V9'],
  power: ['M12 2v10', 'M6 5a9 9 0 1 0 12 0'],
  warning: ['M12 3 2 21h20L12 3z', 'M12 9v5', 'M12 17h.01'],
  image: ['M3 3h18v18H3z', 'M3 16l6-6 5 5 3-3 4 4', 'M16 7h.01'],
  folder: ['M3 6h7l2 3h9v11H3z', 'M3 6V4h7l2 2h9v3'],
  terminal: ['M4 5l6 7-6 7', 'M13 19h7'],
  waveform: ['M4 10v4m4-8v12m4-15v18m4-14v10m4-7v4'],
  'arrow-up': ['M12 20V4m-6 6 6-6 6 6'],
  'arrow-down': ['M12 4v16m-6-6 6 6 6-6'],
  download: ['M12 3v12m-5-5 5 5 5-5', 'M5 16v5h14v-5'],
  'chevron-left': ['M15 5 8 12l7 7'],
  'chevron-right': ['M9 5 16 12l-7 7'],
  'chevron-down': ['M5 9l7 7 7-7'],
  file: ['M14 2H5v20h14V7l-5-5z', 'M14 2v5h5', 'M8 12h8M8 16h6'],
  sliders: ['M4 7h3m6 0h7M4 17h9m6 0h1', 'M7 4h6v6H7zM13 14h6v6h-6z'],
  help: ['M12 22a10 10 0 1 0 0-20 10 10 0 0 0 0 20', 'M9 9a3 3 0 0 1 6 0c0 2-3 2-3 5', 'M12 18h.01'],
  more: ['M5 12h.01M12 12h.01M19 12h.01'],
  sidebar: ['M4 3h16v18H4z', 'M9 3v18'],
  speaker: ['M11 4 6 8H3v8h3l5 4V4z', 'M15 8a6 6 0 0 1 0 8', 'M18 5a10 10 0 0 1 0 14'],
  chat: ['M21 15a4 4 0 0 1-4 4H8l-5 3V7a4 4 0 0 1 4-4h10a4 4 0 0 1 4 4z'],
  knowledge: ['M4 19.5A2.5 2.5 0 0 1 6.5 17H20', 'M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z'],
  agents: ['M12 3a3 3 0 1 0 0 6 3 3 0 0 0 0-6z', 'M19 13a3 3 0 1 0 0 6 3 3 0 0 0 0-6z', 'M5 13a3 3 0 1 0 0 6 3 3 0 0 0 0-6z', 'M12 9v7m-4 0h8'],
  models: ['M21 7 12 2 3 7l9 5 9-5z', 'm3 12 9 5 9-5', 'm3 17 9 5 9-5'],
  activity: ['M3 11a9 9 0 1 1 2.6 7.4', 'M3 4v7h7', 'M12 7v5l3 2'],
  settings: ['M9.5 2h5l.5 2.5 2 1.2 2.4-.8 2.5 4.2-1.9 1.7v2.4l1.9 1.7-2.5 4.2-2.4-.8-2 1.2-.5 2.5h-5L9 19.5l-2-1.2-2.4.8-2.5-4.2L4 13.2v-2.4l-1.9-1.7 2.5-4.2 2.4.8 2-1.2.5-2.5z', 'M15.5 12a3.5 3.5 0 1 1-7 0 3.5 3.5 0 0 1 7 0'],
  send: ['m4 4 16 8-16 8 3-8-3-8z', 'M7 12h13'],
  upload: ['M12 16V4m-5 5 5-5 5 5', 'M5 20h14'],
  refresh: ['M20 6v5h-5', 'M4 18v-5h5', 'M18.5 9A7 7 0 0 0 6 6.5L4 11', 'M5.5 15A7 7 0 0 0 18 17.5l2-4.5'],
  plus: ['M12 5v14M5 12h14'],
  trash: ['M4 7h16M9 7V4h6v3m-8 0 1 13h8l1-13M10 11v5m4-5v5'],
  check: ['m5 12 4 4L19 6'],
  copy: ['M8 8h11v11H8z', 'M5 16H4V5h11v1'],
  search: ['m21 21-4.4-4.4', 'M19 11a8 8 0 1 1-16 0 8 8 0 0 1 16 0z'],
  menu: ['M4 6h16M4 12h16M4 18h16'],
  close: ['M18 6 6 18M6 6l12 12'],
  mic: ['M12 2a3 3 0 0 0-3 3v7a3 3 0 0 0 6 0V5a3 3 0 0 0-3-3z', 'M19 10v2a7 7 0 0 1-14 0v-2', 'M12 19v3', 'M8 22h8'],
  stop: ['M6 6h12v12H6z']
};

export function Icon({ name, size = 20 }: { name: IconName; size?: number }): ReactNode {
  return <svg aria-hidden="true" width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
    {paths[name].map((path, index) => <path d={path} key={index} />)}
  </svg>;
}
