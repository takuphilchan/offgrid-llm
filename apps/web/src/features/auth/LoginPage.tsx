import { useEffect, useId, useRef, useState, type FormEvent } from 'react';
import { api, type PublicUser } from '../../api/client';
import { useI18n } from '../../i18n';
import { loginContext } from '../../i18n/login-context';
import './login.css';

export function LoginPage({ onAuthenticated }: { onAuthenticated: (user: PublicUser) => void }) {
  const { messages: text, locale } = useI18n(), copy = loginContext[locale], id = useId();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [visible, setVisible] = useState(false);
  const [service, setService] = useState(window.location.origin);
  const pending = useRef(false);
  useEffect(() => {
    let disposed = false;
    void window.electron?.getBackendInfo?.().then(backend => {
      if (disposed) return;
      try {
        const url = new URL(backend.url);
        // Display identity only: never expose URL credentials, queries or paths.
        if (url.protocol === 'http:' || url.protocol === 'https:') setService(url.origin);
      } catch { /* The authenticated transport still uses the current origin. */ }
    }).catch(() => { /* Metadata failure does not block sign-in. */ });
    return () => { disposed = true; };
  }, []);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (!username.trim() || !password || pending.current) return;
    pending.current = true;
    setBusy(true);
    setError('');
    try {
      const result = await api.login(username.trim(), password);
      onAuthenticated(result.user);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : text.common.error);
    } finally { pending.current = false; setBusy(false); }
  };

  return <div className="login-screen"><form className="login-card" onSubmit={submit}>
    <div className="brand-mark large"><span /></div><span className="eyebrow">{text.product}</span><h1>{text.auth.title}</h1><p>{text.auth.body}</p>
    <div className="login-service"><span>{copy.service}</span><code dir="ltr">{service}</code></div>
    <p className="login-account-hint" id={`${id}-account`}>{copy.account}</p>
    {error && <div className="inline-error login-recovery" role="alert"><p>{error}</p><p>{copy.recovery}</p></div>}
    <label><span>{text.auth.username}</span><input autoComplete="username" autoFocus autoCapitalize="none" spellCheck={false} aria-describedby={`${id}-account`} disabled={busy} value={username} onChange={event => setUsername(event.target.value)} /></label>
    <div className="login-password"><label htmlFor={`${id}-password`}>{text.auth.password}</label><input id={`${id}-password`} type={visible ? 'text' : 'password'} autoComplete="current-password" disabled={busy} value={password} onChange={event => setPassword(event.target.value)} />
      <button type="button" className="utility-button" aria-controls={`${id}-password`} aria-pressed={visible} onClick={() => setVisible(!visible)}>{visible ? copy.hide : copy.show}</button></div>
    <button className="primary-button" disabled={busy || !username.trim() || !password}>{busy ? text.common.loading : text.auth.signIn}</button>
    <small>{text.auth.localNote}</small>
  </form></div>;
}
