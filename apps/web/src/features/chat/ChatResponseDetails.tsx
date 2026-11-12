import type {ChatMetrics} from '../../api/session-stream';
import {UtilityPopover} from '../../components/UtilityPopover';
import {useI18n} from '../../i18n';
import {workspaceExperience} from '../../i18n/workspace-experience';

export function ChatResponseDetails({metrics}:{metrics:ChatMetrics}){
 const {messages:text,locale, language}=useI18n();
 return <UtilityPopover label={workspaceExperience[locale].responseDetails} className="response-details" compact panelClassName="response-details-panel">
  <dl className="response-metrics">
   <div><dt>{text.chatStreaming.firstText}</dt><dd>{(metrics.first_text_ms/1000).toLocaleString(language,{maximumFractionDigits:2})} s</dd></div>
   {!!metrics.tokens_per_second&&<div><dt>{text.chatStreaming.tokensPerSecond}</dt><dd>{metrics.tokens_per_second.toLocaleString(language,{maximumFractionDigits:1})}</dd></div>}
   <div><dt>{text.chatStreaming.contextTokens}</dt><dd>{metrics.context_window.toLocaleString(language)}</dd></div>
  </dl>
 </UtilityPopover>;
}
