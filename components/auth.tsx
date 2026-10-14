'use client';
import { useLocale, tr, trText } from './locale';
import {
  createContext,
  useContext,
  useEffect,
  useRef,
  useState,
  useSyncExternalStore,
  type ReactNode,
} from 'react';
import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import {
  Activity,
  ArrowUpRight,
  Eye,
  EyeOff,
  LockKeyhole,
  Sun,
  Moon,
  LoaderCircle,
} from 'lucide-react';
import gsap from 'gsap';
import { useGSAP } from '@gsap/react';
import { api, safeReturnPath, setCSRFToken } from '@/lib/monitor';
gsap.registerPlugin(useGSAP);
type AuthStatus = {
  authenticated: boolean;
  setupRequired: boolean;
  csrfToken?: string;
  user?: {
    id: string;
    name: string;
    email: string;
  };
};
type AuthContext = {
  status: AuthStatus | null;
  error: string;
  retry: () => void;
  accept: (s: AuthStatus) => void;
  logout: () => Promise<void>;
};
const Context = createContext<AuthContext | null>(null);
function subscribeTheme(update: () => void) {
  const observer = new MutationObserver(update);
  observer.observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] });
  return () => observer.disconnect();
}
export function useAuth() {
  const value = useContext(Context);
  if (!value) throw new Error('Auth provider required');
  return value;
}
export function AuthProvider({ children }: { children: ReactNode }) {
  useLocale();
  const [status, setStatus] = useState<AuthStatus | null>(null),
    [error, setError] = useState(''),
    [revision, setRevision] = useState(0);
  function accept(s: AuthStatus) {
    setCSRFToken(s.csrfToken ?? '');
    setStatus(s);
    setError('');
  }
  useEffect(() => {
    const abort = new AbortController();
    api<AuthStatus>('/auth/status', { signal: abort.signal })
      .then(accept)
      .catch((e) => {
        if (!abort.signal.aborted)
          setError(e instanceof Error ? e.message : tr('Бэкенд недоступен'));
      });
    return () => abort.abort();
  }, [revision]);
  useEffect(() => {
    function expired() {
      setCSRFToken('');
      setStatus((s) => (s ? { authenticated: false, setupRequired: s.setupRequired } : null));
    }
    window.addEventListener('pulse:unauthorized', expired);
    return () => window.removeEventListener('pulse:unauthorized', expired);
  }, []);
  async function logout() {
    await api('/auth/logout', { method: 'POST', body: '{}' });
    accept({ authenticated: false, setupRequired: false });
  }
  return (
    <Context.Provider
      value={{ status, error, retry: () => setRevision((n) => n + 1), accept, logout }}
    >
      {trText(children)}
    </Context.Provider>
  );
}
export function SessionGate({ children }: { children: ReactNode }) {
  useLocale();
  const { status, error, retry } = useAuth(),
    router = useRouter(),
    pathname = usePathname();
  useEffect(() => {
    if (status && !status.authenticated)
      router.replace(
        status.setupRequired
          ? '/register/'
          : `/login/?next=${encodeURIComponent(safeReturnPath(pathname))}`,
      );
  }, [status, router, pathname]);
  if (!status?.authenticated)
    return (
      <div className="auth-wait">
        <Activity size={36} />
        <strong>PULSE</strong>
        {trText(
          error ? (
            <>
              <p role="alert">
                {tr('Не удалось подключиться к серверу:') + ' '}
                {trText(error)}
              </p>
              <button className="button" onClick={retry}>
                {tr('Повторить')}
              </button>
            </>
          ) : (
            <>
              <div className="auth-skeleton" />
              <p>{tr('Проверяем доступ\u2026')}</p>
            </>
          ),
        )}
      </div>
    );
  return children;
}
export function AuthPage({ register = false }: { register?: boolean }) {
  useLocale();
  const { status, error: connectionError, retry, accept } = useAuth(),
    router = useRouter(),
    root = useRef<HTMLDivElement>(null);
  const [name, setName] = useState(''),
    [email, setEmail] = useState(''),
    [password, setPassword] = useState(''),
    [confirm, setConfirm] = useState(''),
    [visible, setVisible] = useState(false),
    [busy, setBusy] = useState(false),
    [error, setError] = useState('');
  const light = useSyncExternalStore(
    subscribeTheme,
    () => document.documentElement.dataset.theme === 'light',
    () => false,
  );
  useGSAP(
    () => {
      const mm = gsap.matchMedia();
      mm.add('(prefers-reduced-motion: no-preference)', () => {
        gsap.from('.auth-reveal', {
          y: 22,
          opacity: 0,
          duration: 0.8,
          stagger: 0.09,
          ease: 'power3.out',
        });
        gsap.from('.auth-wave', { strokeDashoffset: 1200, duration: 2.4, ease: 'power2.out' });
      });
      return () => mm.revert();
    },
    { scope: root },
  );
  useEffect(() => {
    if (status?.authenticated)
      router.replace(safeReturnPath(new URLSearchParams(window.location.search).get('next')));
  }, [status, router]);
  function theme() {
    const next = !light;
    document.documentElement.dataset.theme = next ? 'light' : 'dark';
    try {
      localStorage.setItem('pulse.theme.v1', next ? 'light' : 'dark');
    } catch {}
  }
  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError('');
    if (
      register &&
      (Array.from(password).length < 12 || new TextEncoder().encode(password).length > 72)
    ) {
      setError(tr('Пароль: от 12 символов, не более 72 байт.'));
      return;
    }
    if (register && password !== confirm) {
      setError(tr('Пароли не совпадают.'));
      return;
    }
    setBusy(true);
    try {
      const result = await api<AuthStatus>(register ? '/auth/register' : '/auth/login', {
        method: 'POST',
        body: JSON.stringify(register ? { name, email, password } : { email, password }),
      });
      accept(result);
      router.replace(safeReturnPath(new URLSearchParams(window.location.search).get('next')));
    } catch (e) {
      setError(e instanceof Error ? e.message : tr('Не удалось войти'));
    } finally {
      setBusy(false);
    }
  }
  const closed = register && status && !status.setupRequired;
  return (
    <div className="auth-page" ref={root}>
      <header className="auth-header">
        <Link className="brand" href="/login/">
          <Activity size={32} />
          <span>PULSE</span>
          <span className="brand-version">/ MONITOR</span>
        </Link>
        <button
          className="icon-button"
          aria-label={light ? tr('Включить тёмную тему') : tr('Включить светлую тему')}
          onClick={theme}
        >
          {trText(light ? <Moon size={20} /> : <Sun size={20} />)}
        </button>
      </header>
      <main className="auth-layout">
        <section className="auth-story">
          <span className="section-eyebrow auth-reveal">
            {tr('ВАША ИНФРАСТРУКТУРА / ПОД КОНТРОЛЕМ')}
          </span>
          <h1 className="auth-reveal">
            {tr('Всё видно.')}
            <br />
            <span>{tr('Всё спокойно.')}</span>
          </h1>
          <p className="auth-reveal">
            {tr('Процессы, ресурсы и события.')}
            <br />
            {tr('Единое пространство для ваших серверов.')}
          </p>
          <div className="auth-visual auth-reveal" aria-hidden="true">
            <div className="auth-signal-label">
              <span>
                <i className="tiny-dot" /> SYSTEM PULSE
              </span>
              <span className="mono">LIVE / 001</span>
            </div>
            <svg viewBox="0 0 600 180" fill="none">
              <path
                className="auth-grid"
                d="M0 45H600M0 90H600M0 135H600M100 0V180M200 0V180M300 0V180M400 0V180M500 0V180"
              />
              <path
                className="auth-wave"
                strokeDasharray="1200"
                d="M0 100H82L100 90L114 107L132 96H190L210 64L228 131L252 23L275 154L299 86L320 100H382L402 85L420 113L440 100H600"
              />
            </svg>
            <div className="auth-visual-bottom">
              <span>LINUX + WINDOWS</span>
              <span>ONE CONTROL POINT</span>
            </div>
          </div>
          <div className="auth-story-footer auth-reveal">
            <LockKeyhole size={15} />
            {' ' + tr('Ваш сервер. Ваши данные. Ваш контроль.')}
          </div>
        </section>
        <section className="auth-card auth-reveal" aria-labelledby="auth-title">
          <span className="section-eyebrow">
            {trText(register ? tr('01 / НАЧАЛО РАБОТЫ') : tr('01 / ДОСТУП К ПАНЕЛИ'))}
          </span>
          <h2 id="auth-title">
            {trText(register ? tr('Создать аккаунт.') : tr('С возвращением.'))}
          </h2>
          <p className="auth-description">
            {trText(
              register
                ? tr('Создайте учётную запись владельца панели.')
                : tr('Войдите, чтобы увидеть состояние серверов.'),
            )}
          </p>
          {trText(
            connectionError ? (
              <div className="form-note">
                <p role="alert">
                  {tr('Бэкенд недоступен.') + ' '}
                  {trText(connectionError)}
                </p>
                <button className="text-button" onClick={retry}>
                  {tr('Повторить')}
                </button>
              </div>
            ) : !status ? (
              <div className="auth-skeleton" />
            ) : closed ? (
              <div className="form-note">
                <p>{tr('Владелец уже зарегистрирован. Используйте страницу входа.')}</p>
              </div>
            ) : (
              <form onSubmit={submit} className="auth-form">
                {trText(
                  register ? (
                    <label className="field" htmlFor="auth-name">
                      {tr('Ваше имя')}
                      <input
                        id="auth-name"
                        name="name"
                        autoComplete="name"
                        required
                        maxLength={64}
                        value={name}
                        onChange={(e) => setName(e.target.value)}
                        placeholder={tr('Как к вам обращаться')}
                        disabled={busy}
                      />
                    </label>
                  ) : null,
                )}
                <label className="field" htmlFor="auth-email">
                  Email
                  <input
                    id="auth-email"
                    name="email"
                    type="email"
                    autoComplete="username"
                    required
                    maxLength={254}
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    placeholder="you@example.com"
                    disabled={busy}
                  />
                </label>
                <label className="field" htmlFor="auth-password">
                  {tr('Пароль')}
                  <div className="auth-password">
                    <input
                      id="auth-password"
                      name="password"
                      type={visible ? 'text' : 'password'}
                      autoComplete={register ? 'new-password' : 'current-password'}
                      required
                      value={password}
                      onChange={(e) => setPassword(e.target.value)}
                      aria-describedby={register ? 'password-hint' : undefined}
                      disabled={busy}
                    />
                    <button
                      type="button"
                      className="icon-button"
                      aria-label={visible ? tr('Скрыть пароль') : tr('Показать пароль')}
                      aria-pressed={visible}
                      onClick={() => setVisible((v) => !v)}
                    >
                      {trText(visible ? <EyeOff size={18} /> : <Eye size={18} />)}
                    </button>
                  </div>
                  {trText(
                    register ? (
                      <small id="password-hint">
                        {tr('От 12 символов \u00B7 не более 72 байт')}
                      </small>
                    ) : null,
                  )}
                </label>
                {trText(
                  register ? (
                    <label className="field" htmlFor="auth-confirm">
                      {tr('Повторите пароль')}
                      <input
                        id="auth-confirm"
                        name="confirm"
                        type={visible ? 'text' : 'password'}
                        autoComplete="new-password"
                        required
                        value={confirm}
                        onChange={(e) => setConfirm(e.target.value)}
                        disabled={busy}
                      />
                    </label>
                  ) : null,
                )}
                {trText(
                  error ? (
                    <p className="form-error" role="alert">
                      {trText(error)}
                    </p>
                  ) : null,
                )}
                <button type="submit" className="button primary auth-submit" disabled={busy}>
                  {trText(
                    busy ? (
                      <>
                        <LoaderCircle size={17} className="spin" />
                        {tr('Подождите\u2026')}
                      </>
                    ) : (
                      <>
                        {trText(register ? tr('Создать аккаунт') : tr('Войти в панель'))}
                        <ArrowUpRight size={18} />
                      </>
                    ),
                  )}
                </button>
                {trText(
                  register ? (
                    <p className="auth-security">
                      <LockKeyhole size={13} />
                      {' ' + tr('Регистрация закроется после создания владельца.')}
                    </p>
                  ) : null,
                )}
              </form>
            ),
          )}
          <div className="auth-switch">
            {trText(
              register ? (
                <>
                  {tr('Уже есть аккаунт?') + ' '}
                  <Link href="/login/">
                    {tr('Войти') + ' '}
                    <ArrowUpRight size={13} />
                  </Link>
                </>
              ) : status?.setupRequired ? (
                <>
                  {tr('Первый запуск?') + ' '}
                  <Link href="/register/">
                    {tr('Создать аккаунт') + ' '}
                    <ArrowUpRight size={13} />
                  </Link>
                </>
              ) : (
                <span>{tr('Доступ только для владельца панели')}</span>
              ),
            )}
          </div>
        </section>
      </main>
      <footer className="auth-footer">
        <span>PULSE MONITOR</span>
        <span>PRIVATE INFRASTRUCTURE / CLEAR VISION</span>
      </footer>
    </div>
  );
}
