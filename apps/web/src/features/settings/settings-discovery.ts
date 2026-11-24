import type { useI18n, LocaleCode } from '../../i18n';
import { settingsNavigation, type SettingsSection as BaseSettingsSection } from '../../i18n/settings-navigation';
import { personalization } from '../../i18n/personalization';
import { responseDisplay } from '../../i18n/response-display';
import { voiceSettingsText } from '../../i18n/voice-settings';
import { desktopCommand } from '../../i18n/desktop-command';
import { workspaceRecovery } from '../../i18n/workspace-recovery';
import { workspaceManagement } from '../../i18n/workspace-management';

export type SettingsSection = BaseSettingsSection | 'appearance' | 'shortcuts' | 'storage';
export type SettingTarget = { id: string; section: SettingsSection; label: string; selector: string; fallback?: string; keywords?: string; searchable?: boolean };

// Routes point to the existing owner. Finding a preference never activates it.
export function settingsTargets(text: ReturnType<typeof useI18n>['messages'], locale: LocaleCode): SettingTarget[] {
  const copy = settingsNavigation[locale], appearance = personalization[locale], voice = voiceSettingsText(locale), quick = desktopCommand(locale);
  return [
    { id: 'language', section: 'general', label: text.common.language, selector: '#settings-language' },
    { id: 'setup-guide', section: 'general', label: text.settings.showGuide, selector: 'section[aria-labelledby="preferences-title"] > .action-card' },
    { id: 'theme', section: 'appearance', label: text.shell.theme, selector: '#setting-theme button[aria-pressed="true"]' },
    { id: 'appearance', section: 'appearance', label: appearance[0], selector: '#setting-appearance' },
    { id: 'light-theme', section: 'appearance', label: appearance[1], selector: '[data-setting="light-theme"]' },
    { id: 'dark-theme', section: 'appearance', label: appearance[2], selector: '[data-setting="dark-theme"]' },
    { id: 'text-size', section: 'appearance', label: appearance[3], selector: '#appearance-text-size' },
    { id: 'motion', section: 'appearance', label: appearance[4], selector: '#appearance-motion' },
    { id: 'response-display', section: 'models', label: responseDisplay(locale).label, selector: '.response-settings select[id$="-display"]', keywords: responseDisplay(locale).hint },
    { id: 'response-profile', section: 'models', label: text.chatStreaming.profile, selector: '.response-settings select[id$="-profile"]', keywords: text.chatStreaming.profileHint },
    { id: 'response-limit', section: 'models', label: text.chatStreaming.responseLimit, selector: '.response-settings select[id$="-limit"]' },
    { id: 'responses', section: 'models', label: copy.sections[1], selector: '#model-settings-title', searchable: false },
    { id: 'voice', section: 'voice', label: copy.sections[2], selector: '#voice-settings-title' },
    { id: 'recognition-model', section: 'voice', label: voice.recognition, selector: '[data-setting="recognition-model"] button:not(:disabled)', fallback: '[data-setting="recognition-model"]' },
    { id: 'speech-model', section: 'voice', label: voice.synthesis, selector: '[data-setting="speech-model"] button:not(:disabled)', fallback: '[data-setting="speech-model"]' },
    { id: 'speech-voice', section: 'voice', label: voice.voice, selector: '[data-setting="speech-voice"] select:not(:disabled)', fallback: '.voice-settings-inline' },
    { id: 'terminal', section: 'terminal', label: copy.sections[3], selector: '#terminal-settings-title', keywords: copy.terminal + ' ' + copy.descriptions[3] },
    { id: 'quick-window', section: 'shortcuts', label: quick.title, selector: '#desktop-command-shortcut:not(:disabled)', fallback: '#shortcuts-settings-title' },
    { id: 'shortcut', section: 'shortcuts', label: quick.shortcut, selector: '#desktop-command-shortcut:not(:disabled)', fallback: '#shortcuts-settings-title' },
    { id: 'access', section: 'access', label: copy.sections[4], selector: '#access-settings-title', keywords: text.settings.computerUse },
    { id: 'recovery', section: 'storage', label: workspaceRecovery(locale).title, selector: '#workspace-recovery-title' },
    { id: 'service', section: 'advanced', label: text.settings.service, selector: '#workspace-title' },
    { id: 'diagnostics', section: 'advanced', label: workspaceManagement(locale).diagnostics, selector: '.settings-diagnostics > summary' }
  ];
}
