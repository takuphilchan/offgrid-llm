import {resolveMessages,featureTranslations} from './catalog';
import type {LocaleCode} from './index';
import source from './workspace-recovery.json' with {type:'json'};

export const workspaceRecovery=(locale:LocaleCode)=>resolveMessages(source,featureTranslations(locale,'i18n/workspace-recovery:copy'));
