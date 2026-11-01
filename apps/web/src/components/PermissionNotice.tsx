import { useI18n } from '../i18n';
import { interaction } from '../i18n/interaction';
import { accessGuidance } from '../i18n/access-guidance';
import { ScopedNotice } from './WorkspacePresentation';

/** Explains an existing denied area; never changes authority or the current task. */
export function PermissionNotice() {
 const { locale, messages } = useI18n();
 return <ScopedNotice className="permission-notice" title={interaction[locale].adminOnly}
   action={<a className="secondary-button" href="#/chat">{messages.nav.chat}</a>}>
   <p>{accessGuidance[locale].body}</p>
 </ScopedNotice>;
}
