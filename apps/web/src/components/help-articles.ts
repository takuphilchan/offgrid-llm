import type { LocaleCode } from '../i18n';
import type { Messages } from '../i18n/types';
import { workspaceHelp } from '../i18n/workspace-help';
import { helpTopics } from '../i18n/workspace-help-topics';
import { settingsNavigation } from '../i18n/settings-navigation';
import { voiceSettingsText } from '../i18n/voice-settings';
import { desktopCommand } from '../i18n/desktop-command';
import type { IconName } from './Icon';
import type { ModelCategory } from '../api/client';

export type HelpArticle={id:string;title:string;icon:IconName;sections:{title?:string;paragraphs:string[]}[];links:{href:string;label:string;category?:ModelCategory}[]};
export function helpArticles(locale:LocaleCode,text:Messages,desktop:boolean):HelpArticle[]{
  const copy=workspaceHelp[locale],topics=helpTopics[locale],settings=settingsNavigation[locale],voice=voiceSettingsText(locale),quick=desktopCommand(locale);
  return [
    {id:'chat',title:text.nav.chat,icon:'chat',sections:[{paragraphs:[copy.chat,copy.example]},{title:text.chat.selectModel,paragraphs:[text.guidance.coldStart,text.chatStreaming.profileHint]}],links:[{href:'#/chat',label:text.nav.chat},{href:'#/settings/models',label:settings.sections[1]}]},
    {id:'models',title:text.nav.models,icon:'models',sections:[{title:text.models.installed,paragraphs:[text.guidance.languageModels,text.guidance.embeddingModels,text.guidance.speechModels]},{title:text.models.discover,paragraphs:[text.guidance.discovery,text.modelSearch.compatibility]},{title:text.modelDownload.failed,paragraphs:[text.modelDownload.retained]}],links:[{href:'#/models',label:text.models.installed,category:'language'},{href:'#/models/discover',label:text.models.discover}]},
    {id:'agents',title:text.nav.agents,icon:'agents',sections:[{paragraphs:[copy.agents,text.guidance.profiles]},{title:text.agents.approvalTitle,paragraphs:[text.agents.approvalBody,copy.access]}],links:[{href:'#/agents/profiles',label:text.nav.agents},{href:'#/settings/access',label:settings.sections[4]}]},
    {id:'documents',title:text.nav.knowledge,icon:'knowledge',sections:[{paragraphs:[topics.documents]},{title:text.chat.knowledge,paragraphs:[text.guidance.knowledge,text.knowledgeSetup.body]}],links:[{href:'#/knowledge',label:text.nav.knowledge},{href:'#/chat',label:text.nav.chat}]},
    {id:'voice',title:voice.title,icon:'mic',sections:[{paragraphs:[topics.voice]},{title:voice.selectedUnavailable,paragraphs:[voice.removed,voice.runtime,voice.voiceUnavailable]},{title:voice.voice,paragraphs:[voice.retention]}],links:[{href:'#/settings/voice',label:voice.title},{href:'#/models',label:voice.manage,category:'speech_recognition'}]},
    {id:'activity',title:text.nav.activity,icon:'activity',sections:[{paragraphs:[copy.returning]},{title:text.recovery.uncertain,paragraphs:[text.guidance.activity,text.recovery.verifyOutcome]},{title:text.agentLive.reconnecting,paragraphs:[text.agentLive.reconnectHint,text.chatStreaming.cancelled]}],links:[{href:'#/activity',label:text.nav.activity},{href:'#/chat',label:text.nav.chat}]},
    {id:'access',title:settings.sections[4],icon:'shield',sections:[{paragraphs:[copy.access]},{title:text.onboarding.service,paragraphs:[text.guidance.connectionMeaning,text.chat.emptyBody]}],links:[{href:'#/settings/access',label:settings.sections[4]}]},
    {id:'connection',title:text.common.serviceDetails,icon:'info',sections:[{title:text.status.offline,paragraphs:[text.guidance.serviceOffline]},{title:text.status.ready,paragraphs:[text.guidance.connectionMeaning]},{title:settings.sections[5],paragraphs:[settings.descriptions[5]]}],links:[{href:'#/settings/advanced',label:text.common.serviceDetails}]},
    ...(desktop ? [
      {id:'quick-window',title:quick.title,icon:'quick-window' as const,sections:[{paragraphs:[quick.background]}],links:[{href:'#/settings/shortcuts',label:quick.shortcut}]},
      {id:'terminal',title:'OffGrid CLI',icon:'terminal' as const,sections:[{paragraphs:[topics.terminal]},{title:settings.sections[3],paragraphs:[settings.descriptions[3]]}],links:[{href:'#/settings/terminal',label:settings.sections[3]}]},
    ]:[])
  ];
}
// Literal terms only. Keep script marks such as Hindi vowel signs; accent folding
// applies only to Latin letters. Search never sends the query to a service.
const normalize=(value:string)=>value.normalize('NFKC').toLocaleLowerCase().normalize('NFD').replace(/(\p{Script=Latin})\p{M}+/gu,'$1').normalize('NFC');
export function findHelpArticles(articles:HelpArticle[],query:string){
  const terms=normalize(query.trim()).split(/\s+/u).filter(Boolean);
  return articles.filter(item=>{const body=normalize([item.title,...item.sections.flatMap(section=>[section.title??'',...section.paragraphs]),...item.links.map(link=>link.label)].join(' '));return terms.every(term=>body.includes(term));});
}
