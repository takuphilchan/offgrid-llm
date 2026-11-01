import type { ReactNode } from 'react';
import { ScopedNotice } from '../../components/WorkspacePresentation';
import { useI18n } from '../../i18n';
import { chatRecovery, chatBusyRecovery } from '../../i18n/chat-recovery';
import { presentation } from '../../i18n/presentation';

export function ChatFailureNotice({ error, code, action, chooseModel }: { error: string; code: string; action: ReactNode; chooseModel: () => void }) {
  const { messages: text, locale } = useI18n();
  const capacity = code === 'insufficient_runtime_capacity';
  const busy = code === 'inference_busy';
  return <ScopedNotice kind="error" action={capacity ? <button className="secondary-button" onClick={chooseModel}>{text.onboarding.chooseModel}</button> : busy ? undefined : action}>
    {capacity || busy ? <>{capacity ? chatRecovery[locale] : chatBusyRecovery[locale]}<details><summary>{presentation[locale].details}</summary><p>{error}</p></details></> : error}
  </ScopedNotice>;
}
