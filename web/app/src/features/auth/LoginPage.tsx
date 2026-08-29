import { useState, type FormEvent } from 'react';
import { api, type PublicUser } from '../../api/client';
import { useI18n } from '../../i18n';

export function LoginPage({ onAuthenticated }: { onAuthenticated: (user: PublicUser) => void }) {
  const { messages: text } = useI18n();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (!username.trim() || !password || busy) return;
    setBusy(true);
    setError('');
    try {
      const result = await api.login(username.trim(), password);
      onAuthenticated(result.user);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : text.common.error);
    } finally { setBusy(false); }
  };

  return <div className="login-screen"><form className="login-card" onSubmit={submit}>
    <div className="brand-mark large"><span /></div><span className="eyebrow">{text.product}</span><h1>{text.auth.title}</h1><p>{text.auth.body}</p>
    {error && <div className="inline-error" role="alert">{error}</div>}
    <label><span>{text.auth.username}</span><input autoComplete="username" autoFocus value={username} onChange={event => setUsername(event.target.value)} /></label>
    <label><span>{text.auth.password}</span><input type="password" autoComplete="current-password" value={password} onChange={event => setPassword(event.target.value)} /></label>
    <button className="primary-button" disabled={busy || !username.trim() || !password}>{busy ? text.common.loading : text.auth.signIn}</button>
    <small>{text.auth.localNote}</small>
  </form></div>;
}
