import { useLayoutEffect, useRef, useState } from 'react';
import {transitionPresentation} from '../../lib/presentation-transition';

/** Expand compact text mode for an open menu without changing its work state. */
export function useCommandWindowLayout(layout: 'input' | 'work' | 'voice') {
  const [voiceVisible, setVoiceVisible] = useState(layout === 'voice');
  const previous = useRef<string | undefined>(undefined);
  const clippedInputWidth = useRef<number | undefined>(undefined);
  useLayoutEffect(() => {
    let mounted = true;
    let frame = 0;
    const fit = () => {
      const compact = document.querySelector('.command-window');
      const composer = compact?.querySelector<HTMLElement>('.composer');
      if (clippedInputWidth.current !== undefined && innerWidth > clippedInputWidth.current + 1) clippedInputWidth.current = undefined;
      const settled = !document.documentElement.hasAttribute('data-window-transition');
      // Native resize can report innerHeight before CSS viewport units have
      // caught up. Measure the settled workspace, not that intermediate frame.
      const viewportReady = Math.abs((compact?.querySelector('.workspace')?.getBoundingClientRect().height ?? innerHeight) - innerHeight) < 2;
      if (settled && layout === 'input' && composer?.getClientRects().length && !viewportReady) scheduleFit();
      if (settled && viewportReady && layout === 'input' && composer && composer.getBoundingClientRect().top + composer.scrollHeight > innerHeight + 1) clippedInputWidth.current = innerWidth;
      // Zoom can leave the native input window shorter than its controls. Use
      // the existing work size and retain it at this width, so its resize does
      // not immediately shrink the window again. Widening permits input size.
      // A menu can already be using work height when zoom changes. Keep that
      // height before its closing callback can request a now-too-short input
      // size. Native window width stays unzoomed; small frame borders fit
      // within the tolerance. No new desktop capability is needed.
      const zoomNeedsRoom = window.outerWidth > innerWidth * 1.2;
      const needsRoom = compact && (clippedInputWidth.current !== undefined || zoomNeedsRoom);
      const effective = layout === 'input' && (needsRoom || document.querySelector(':popover-open')) ? 'work' : layout;
      if (effective === previous.current) return;
      const initial = previous.current === undefined;
      const voiceChanged = (effective === 'voice') !== (previous.current === 'voice');
      previous.current = effective;
      const apply = async (reduceMotion: boolean) => {
        if (!mounted) return;
        setVoiceVisible(effective === 'voice');
        await window.electron?.setCommandLayout?.(effective, {prepared:true, reduceMotion});
      };
      // Only voice swaps and compact native resizing change the surface. A
      // normal menu or longer draft must not fade a browser/full workspace.
      if (initial || !voiceChanged && !document.querySelector('.command-window')) void apply(true).catch(() => {});
      else void transitionPresentation(apply).catch(() => {});
    };
    const scheduleFit = () => { cancelAnimationFrame(frame); frame = requestAnimationFrame(fit); };
    fit();
    const observer = new ResizeObserver(scheduleFit);
    document.querySelectorAll('.composer, .composer textarea, .command-window-header').forEach(node => observer.observe(node));
    const transition = new MutationObserver(scheduleFit);
    transition.observe(document.documentElement, { attributes: true, attributeFilter: ['data-window-transition'] });
    document.addEventListener('toggle', scheduleFit, true);
    window.addEventListener('resize', scheduleFit);
    return () => {mounted = false; cancelAnimationFrame(frame); observer.disconnect(); transition.disconnect(); document.removeEventListener('toggle', scheduleFit, true); window.removeEventListener('resize', scheduleFit);};
  }, [layout]);
  return voiceVisible;
}
