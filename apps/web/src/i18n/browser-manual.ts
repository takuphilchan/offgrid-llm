import {resolveMessages,featureTranslations} from './catalog';
import type {LocaleCode} from './index';
import source from './browser-manual.json' with {type:'json'};

export const browserManualText=(locale:LocaleCode)=>resolveMessages(source,featureTranslations(locale,'i18n/browser-manual:copy'));
