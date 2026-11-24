import {useRef} from 'react';
import {writeDraft} from '../../lib/drafts';
import type {VoiceCaptureEvent} from '../../lib/voice-capture';
import type {useTalkMode} from './useTalkMode';

type Accepted = {owner: VoiceCaptureEvent; admission?: Promise<boolean>};
type Ownership = {sessionID: string; captureID: string; latest: string; accepted: Map<string,Accepted>};
const key = (event: VoiceCaptureEvent) => `${event.captureID}:${event.utteranceID}`;

// Bounded ownership for one ASR/admission transaction and one queued utterance.
// This is renderer capture accounting, never a second conversation or work store.
export function useTalkCapture(talk: ReturnType<typeof useTalkMode>, eligible: () => boolean,
  interrupt: () => void, draftKey: string, send: (text: string, audible: () => boolean) => Promise<boolean>) {
  const ownership = useRef<Ownership>({sessionID:'',captureID:'',latest:'',accepted:new Map()});
  const current = (event: VoiceCaptureEvent) => talk.active.current && !!talk.session.current && event.sessionID === talk.session.current;
  const onSpeechStart = (event: VoiceCaptureEvent) => {
    if (!current(event) || !eligible() || !event.captureID || !event.utteranceID) return false;
    if (ownership.current.sessionID !== event.sessionID) ownership.current = {sessionID:event.sessionID,captureID:event.captureID,latest:'',accepted:new Map()};
    const state = ownership.current, id = key(event);
    if (state.captureID !== event.captureID || state.latest === id || state.accepted.has(id) || state.accepted.size >= 2) return false;
    state.accepted.set(id,{owner:{...event}}); state.latest=id;
    // Silence playback before ASR or admission can await anything. Revision and
    // audio ownership are separate from the stable capture-session identity.
    talk.revision.current++; interrupt(); talk.setState('listening');
    return true;
  };
  const onTranscript = (value: string, event: VoiceCaptureEvent): Promise<boolean> => {
    const state = ownership.current, id = key(event), accepted = state.accepted.get(id);
    if (!current(event) || state.sessionID !== event.sessionID || !accepted) return Promise.resolve(false);
    if (accepted.admission) return accepted.admission;
    // The capture pipeline must preserve admission order even if a later onset
    // interrupted audio while an earlier utterance was still being recognized.
    if (state.accepted.keys().next().value !== id) return Promise.resolve(false);
    accepted.admission = Promise.resolve().then(async () => {
      if (!current(event)) return false;
      const text=value.trim(); if (!text) return true;
      writeDraft(draftKey,text);
      if (state.latest === id) talk.setState('thinking');
      const audible = () => current(event) && ownership.current === state && state.latest === id;
      try {
        const sent = await send(text,audible);
        if (!sent && current(event)) {talk.active.current=false;talk.setState('needs_attention');}
        return sent && current(event);
      } catch {
        if (current(event)) {talk.active.current=false;talk.setState('needs_attention');}
        return false;
      }
    });
    return accepted.admission;
  };
  const onSettled = (event: VoiceCaptureEvent) => {
    const state = ownership.current, id = key(event), accepted = state.accepted.get(id);
    if (state.sessionID === event.sessionID) {
      state.accepted.delete(id);
      if (current(event) && state.latest === id && accepted && !accepted.admission) talk.setState('listening');
    }
  };
  return {onSpeechStart,onTranscript,onSettled};
}
