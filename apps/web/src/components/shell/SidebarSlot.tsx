import { createContext, useContext, type ReactNode } from 'react';
import { createPortal } from 'react-dom';

export const SidebarContext = createContext<{ target: HTMLElement | null; actionTarget: HTMLElement | null; close: () => void }>({ target: null, actionTarget: null, close: () => {} });

/** Move presentation, not the controller: Chat still owns its scoped history. */
export function SidebarSlot({ children, action = false }: { children: ReactNode; action?: boolean }) {
  const context = useContext(SidebarContext);
  const target = action ? context.actionTarget : context.target;
  return target ? createPortal(children, target) : null;
}
