import { useEffect, type RefObject } from 'react';

/** Prefer native reflow; retain bounded sizing for older browser engines. */
export function useComposerSizing(input: RefObject<HTMLTextAreaElement | null>, draft: string) {
 useEffect(() => {
  const field = input.current;
  if (!field || CSS.supports('field-sizing', 'content')) return;
  const resize = () => {
   field.style.height = '0';
   field.style.height = `${Math.min(field.scrollHeight, 160)}px`;
  };
  resize();
  window.addEventListener('resize', resize);
  return () => window.removeEventListener('resize', resize);
 }, [input, draft]);
}
