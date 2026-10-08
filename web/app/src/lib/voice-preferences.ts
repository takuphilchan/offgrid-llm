import { useMemo } from 'react';
import type { AudioStatus } from '../api/client';
import { useDraft } from './drafts';
import { useWorkspace } from './workspace-context';
import type { LocaleCode } from '../i18n';
import { voiceSettingsText } from '../i18n/voice-settings';

export type VoicePreferences = { asr: string; tts: string; voice: string };
export type SpeechSelection = { model?: string; voice?: string };
export const defaultVoicePreferences: VoicePreferences = { asr: '', tts: '', voice: '' };

// Only model identifiers are persisted, scoped like existing composer preferences.
// Neither microphone consent nor automatic playback is persisted here.
export function useVoicePreferences() {
  const { scope } = useWorkspace();
  const draft = useDraft(scope, 'voice-preferences');
  const preferences = useMemo(() => {
    try {
      const value = JSON.parse(draft.value);
      if (['asr', 'tts', 'voice'].every(key => typeof value?.[key] === 'string')) return value as VoicePreferences;
    } catch { /* no saved selection */ }
    return defaultVoicePreferences;
  }, [draft.value]);
  return { preferences, setPreferences: (value: VoicePreferences) => draft.setValue(JSON.stringify(value)) };
}

// Resolve once per utterance/answer, not once per chunk. Explicit selections
// fail closed if removed or unavailable; they never silently become another model.
export function selectSpeech(status: AudioStatus, preferences: VoicePreferences, kind: 'asr' | 'tts', locale: LocaleCode = 'en'): SpeechSelection {
  const copy = voiceSettingsText(locale);
  const capability = kind === 'asr' ? 'transcription' : 'speech_synthesis';
  const requested = preferences[kind];
  const candidates = (status.profiles ?? []).filter(profile => profile.capabilities.includes(capability));
  const profile = requested ? candidates.find(profile => `${profile.id}@${profile.revision}` === requested) : candidates.find(profile => profile.available);
  if (requested && !profile) throw new Error(copy.removed);
  if (profile && !profile.available) throw new Error(profile.issue || copy.runtime);
  if (!profile && !status[kind]?.available) throw new Error(status[kind]?.issue || copy.notReady);
  const voice = kind === 'tts' ? preferences.voice : '';
  if (voice && profile && !profile.voices?.some(item => item.id === voice)) throw new Error(copy.voiceUnavailable);
  return { model: profile ? `${profile.id}@${profile.revision}` : undefined, ...(voice ? { voice } : {}) };
}
