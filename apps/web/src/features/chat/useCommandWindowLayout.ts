import { useLayoutEffect, useRef, useState } from 'react';
import {transitionPresentation} from '../../lib/presentation-transition';

/** Expand compact text mode for an open menu without changing its work state. */
export function useCommandWindowLayout(layout: 'input' | 'work' | 'voice') {
  const [voiceVisible, setVoiceVisible] = useState(layout === 'voice');
  const previous = useRef<string | undefined>(undefined);
  useLayoutEffect(() => {
    let mounted = true;
    const fit = () => {
      const effective = layout === 'input' && document.querySelector(':popover-open') ? 'work' : layout;
      if (effective === previous.current) return;
      const initial = previous.current === undefined;
      previous.current = effective;
      const apply = async (reduceMotion: boolean) => {
        if (!mounted) return;
        setVoiceVisible(effective === 'voice');
        await window.electron?.setCommandLayout?.(effective, {prepared:true, reduceMotion});
      };
      // Initial mount is not a user transition. Subsequent changes keep the old
      // surface mounted through fade-out and resize before revealing the new one.
      if (initial) void apply(true).catch(() => {});
      else void transitionPresentation(apply).catch(() => {});
    };
    fit();
    document.addEventListener('toggle', fit, true);
    return () => {mounted = false; document.removeEventListener('toggle', fit, true);};
  }, [layout]);
  return voiceVisible;
}
