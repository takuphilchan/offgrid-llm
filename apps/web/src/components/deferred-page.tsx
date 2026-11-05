import { useEffect, useState, type ComponentType } from 'react';
import { useI18n } from '../i18n';
import { pageLoading } from '../i18n/page-loading';

// Cache code, never page state or actor data. Browsers cache failed module URLs:
// offer an explicit reload instead of a Retry button that repeats the failure.
export function deferredPage<P extends object>(load: () => Promise<ComponentType<P>>) {
  let loaded: ComponentType<P> | undefined;
  let pending: Promise<ComponentType<P>> | undefined;
  const request = () => pending ??= load().then(component => {
    loaded = component;
    return component;
  }).catch(reason => { pending = undefined; throw reason; });
  return function DeferredPage(props: P) {
    const { messages, locale } = useI18n();
    const [Page, setPage] = useState(() => loaded);
    const [failed, setFailed] = useState(false);
    useEffect(() => {
      if (Page) return;
      let current = true;
      void request().then(component => { if (current) setPage(() => component); })
        .catch(() => { if (current) setFailed(true); });
      return () => { current = false; };
    }, [Page]);
    if (Page) return <Page {...props} />;
    if (failed) return <div className="page-failure" role="alert"><p>{pageLoading(locale).failed}</p><button className="secondary-button" onClick={() => window.location.reload()}>{pageLoading(locale).reload}</button></div>;
    return <div className="deferred-page" role="status" aria-live="polite"><span className="deferred-page-indicator" aria-hidden="true" />{messages.common.loading}</div>;
  };
}
