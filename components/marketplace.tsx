'use client';

import { useEffect, useState } from 'react';
import { Download, RefreshCw, ShieldCheck, Store } from 'lucide-react';
import { api } from '@/lib/monitor';
import type { ModuleManifest } from '@/lib/modules';
import { useLocale, tr } from './locale';
import { useMonitor } from './provider';
import { Empty, Modal, Panel, Skeleton } from './ui';

type Connection = { enabled: boolean; url: string };
type PublishedModule = {
  owner: string;
  id: string;
  name: string;
  description: string;
  version: string;
  sha256: string;
  size: number;
  platforms: string[];
  permissions: string[];
  sourceUrl: string;
};
type Catalog = { items: PublishedModule[]; nextCursor: string | null };

function validModule(value: unknown): value is PublishedModule {
  if (!value || typeof value !== 'object') return false;
  const item = value as Record<string, unknown>;
  const text = ['owner', 'id', 'name', 'description', 'version', 'sha256', 'sourceUrl'];
  return (
    text.every((key) => typeof item[key] === 'string') &&
    /^[a-f0-9]{64}$/.test(String(item.sha256)) &&
    typeof item.size === 'number' &&
    item.size > 0 &&
    item.size <= 68 * 1024 * 1024 &&
    Array.isArray(item.platforms) &&
    item.platforms.every((p) => typeof p === 'string') &&
    Array.isArray(item.permissions) &&
    item.permissions.every((p) => typeof p === 'string')
  );
}

export function Marketplace({
  installed,
}: {
  installed: Pick<ModuleManifest, 'id' | 'version'>[];
}) {
  useLocale();
  const { notify } = useMonitor();
  const [connection, setConnection] = useState<Connection | null>(null);
  const [address, setAddress] = useState('');
  const [items, setItems] = useState<PublishedModule[]>([]);
  const [cursor, setCursor] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [query, setQuery] = useState('');
  const [platform, setPlatform] = useState('');
  const [filters, setFilters] = useState({ query: '', platform: '' });
  const [revision, setRevision] = useState(0);
  const [versions, setVersions] = useState<PublishedModule[] | null>(null);
  const [choice, setChoice] = useState<PublishedModule | null>(null);
  const [accepted, setAccepted] = useState(false);
  useEffect(() => {
    const abort = new AbortController();
    api<Connection>('/marketplace', { signal: abort.signal })
      .then((value) => {
        setConnection(value);
        setAddress(value.url);
      })
      .catch((e) => {
        if (!abort.signal.aborted) setError(e.message);
      })
      .finally(() => {
        if (!abort.signal.aborted) setLoading(false);
      });
    return () => abort.abort();
  }, []);
  useEffect(() => {
    if (!connection?.enabled) return;
    const abort = new AbortController();
    api<Catalog>(`/marketplace/catalog?${new URLSearchParams(filters)}`, { signal: abort.signal })
      .then((page) => {
        if (!Array.isArray(page.items) || !page.items.every(validModule))
          throw new Error(tr('Некорректный ответ каталога'));
        setItems(page.items);
        setCursor(page.nextCursor);
        setError('');
      })
      .catch((e) => {
        if (!abort.signal.aborted) setError(e.message);
      })
      .finally(() => {
        if (!abort.signal.aborted) setLoading(false);
      });
    return () => abort.abort();
  }, [connection, filters, revision]);
  async function save(enabled: boolean) {
    setBusy(true);
    setError('');
    try {
      const saved = await api<Connection>('/marketplace', {
        method: 'PUT',
        body: JSON.stringify({ enabled, url: enabled ? address.trim() : (connection?.url ?? '') }),
      });
      setItems([]);
      setCursor(null);
      setLoading(enabled);
      setConnection(saved);
      setAddress(saved.url);
    } catch (e) {
      setError(e instanceof Error ? e.message : tr('Не удалось подключить каталог'));
    } finally {
      setBusy(false);
    }
  }
  async function more() {
    if (!cursor) return;
    setBusy(true);
    try {
      const page = await api<Catalog>(
        `/marketplace/catalog?${new URLSearchParams({ ...filters, cursor })}`,
      );
      if (!Array.isArray(page.items) || !page.items.every(validModule))
        throw new Error(tr('Некорректный ответ каталога'));
      setItems((current) => [...current, ...page.items]);
      setCursor(page.nextCursor);
      setError('');
    } catch (e) {
      setError(e instanceof Error ? e.message : tr('Каталог недоступен'));
    } finally {
      setBusy(false);
    }
  }
  async function inspect(item: PublishedModule) {
    setBusy(true);
    setError('');
    try {
      const data = await api<{ versions: PublishedModule[] }>(
        `/marketplace/versions?${new URLSearchParams({ owner: item.owner, id: item.id })}`,
      );
      if (
        !Array.isArray(data.versions) ||
        !data.versions.length ||
        !data.versions.every(validModule)
      )
        throw new Error(tr('Некорректный ответ каталога'));
      setVersions(data.versions);
      setChoice(data.versions[0]);
      setAccepted(false);
    } catch (e) {
      setError(e instanceof Error ? e.message : tr('Каталог недоступен'));
    } finally {
      setBusy(false);
    }
  }
  async function install() {
    if (!choice || !accepted) return;
    setBusy(true);
    setError('');
    try {
      await api('/marketplace/install', {
        method: 'POST',
        body: JSON.stringify({
          owner: choice.owner,
          id: choice.id,
          version: choice.version,
          sha256: choice.sha256,
        }),
      });
      window.dispatchEvent(new Event('pulse:modules'));
      notify(tr('Модуль установлен'));
      setChoice(null);
      setVersions(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : tr('Не удалось установить модуль'));
    } finally {
      setBusy(false);
    }
  }
  if (!connection && loading) return <Skeleton />;
  return (
    <div className="marketplace-workspace">
      <Panel>
        <div className="panel-title">
          <h2>
            <Store size={18} />
            {tr('Каталог модулей')}
          </h2>
          <span className="tag">{tr(connection?.enabled ? 'Подключён' : 'Отключён')}</span>
        </div>
        <div className="module-info">
          <p className="muted">
            {tr(
              'Подключение добровольное. Monitor запрашивает каталог и выбранные пакеты. Метрики, логи, список серверов и учётные данные не отправляются.',
            )}
          </p>
          <form
            className="marketplace-connection"
            onSubmit={(e) => {
              e.preventDefault();
              void save(true);
            }}
          >
            <label className="field">
              <span>{tr('Адрес marketplace')}</span>
              <input
                type="url"
                required
                value={address}
                disabled={busy}
                onChange={(e) => setAddress(e.target.value)}
                placeholder="https://marketplace.example.org"
              />
              <small>
                {tr(
                  'HTTPS-адрес вашего каталога. Для локальной проверки разрешён HTTP на 127.0.0.1.',
                )}
              </small>
            </label>
            <div className="button-row">
              <button className="button primary" disabled={busy || !address.trim()}>
                {tr(connection?.enabled ? 'Сохранить' : 'Подключить каталог')}
              </button>
              {connection?.enabled && (
                <button
                  className="button"
                  type="button"
                  disabled={busy}
                  onClick={() => void save(false)}
                >
                  {tr('Отключить каталог')}
                </button>
              )}
            </div>
          </form>
        </div>
      </Panel>
      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}
      {connection?.enabled ? (
        <>
          <form
            className="marketplace-filters"
            onSubmit={(e) => {
              e.preventDefault();
              setLoading(true);
              setFilters({ query, platform });
            }}
          >
            <label className="field">
              <span>{tr('Поиск модулей')}</span>
              <input
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder={tr('Название или описание')}
                maxLength={128}
              />
            </label>
            <label className="field">
              <span>{tr('Платформа')}</span>
              <select value={platform} onChange={(e) => setPlatform(e.target.value)}>
                <option value="">{tr('Все платформы')}</option>
                {['linux-amd64', 'linux-arm64', 'windows-amd64'].map((p) => (
                  <option key={p}>{p}</option>
                ))}
              </select>
            </label>
            <button className="button" disabled={busy}>
              {tr('Найти')}
            </button>
            <button
              type="button"
              className="icon-button"
              aria-label={tr('Обновить каталог')}
              disabled={busy || loading}
              onClick={() => {
                setLoading(true);
                setRevision((n) => n + 1);
              }}
            >
              <RefreshCw size={17} />
            </button>
          </form>
          {loading ? (
            <Skeleton />
          ) : !items.length ? (
            <Empty
              title={tr('Модули не найдены')}
              text={tr('Измените фильтры или повторите запрос позже.')}
            />
          ) : (
            <div className="module-list">
              {items.map((item) => {
                const existing = installed.find((m) => m.id === item.id);
                return (
                  <Panel key={`${item.owner}/${item.id}`}>
                    <div className="panel-title">
                      <h2>{item.name}</h2>
                      <span className="tag">{item.version}</span>
                    </div>
                    <div className="module-info">
                      <small>
                        {item.owner} / {item.id}
                      </small>
                      <p>{item.description}</p>
                      <small>
                        {item.platforms.join(' · ')} · {(item.size / 1024 / 1024).toFixed(1)} MB
                      </small>
                      <button
                        className="button"
                        disabled={busy || !!existing}
                        onClick={() => void inspect(item)}
                      >
                        <Download size={16} />
                        {existing ? tr('Установлен') : tr('Версии и установка')}
                      </button>
                      {existing && (
                        <small>
                          {tr(
                            'Уже установлен модуль с этим ID. Обновления доступны через ручной импорт JSON.',
                          )}
                        </small>
                      )}
                    </div>
                  </Panel>
                );
              })}
            </div>
          )}
          {cursor && !loading && (
            <button className="button" disabled={busy} onClick={() => void more()}>
              {tr('Показать ещё')}
            </button>
          )}
        </>
      ) : (
        !loading && (
          <Empty
            title={tr('Каталог отключён')}
            text={tr(
              'Включите подключение или установите JSON вручную во вкладке установленных модулей.',
            )}
          />
        )
      )}
      {choice && versions && (
        <Modal
          title={tr('Установка из каталога')}
          subtitle={`${choice.owner} / ${choice.id}`}
          onClose={() => {
            if (!busy) {
              setChoice(null);
              setVersions(null);
            }
          }}
        >
          <div className="drawer-body marketplace-review">
            <label className="field">
              <span>{tr('Версия')}</span>
              <select
                disabled={busy}
                value={choice.sha256}
                onChange={(e) => {
                  setChoice(versions.find((v) => v.sha256 === e.target.value) ?? null);
                  setAccepted(false);
                }}
              >
                {versions.map((v) => (
                  <option key={v.version} value={v.sha256}>
                    {v.version}
                  </option>
                ))}
              </select>
            </label>
            <p>
              {choice.name} · {choice.platforms.join(', ')}
            </p>
            <small>SHA-256</small>
            <code className="marketplace-digest">{choice.sha256}</code>
            <h3>{tr('Права модуля')}</h3>
            {choice.permissions.length ? (
              <ul>
                {choice.permissions.map((p) => (
                  <li key={p}>{p}</li>
                ))}
              </ul>
            ) : (
              <p className="muted">{tr('Дополнительные права не заявлены')}</p>
            )}
            {/^https:\/\/github\.com\/[^\s]+$/.test(choice.sourceUrl) && (
              <a className="button" href={choice.sourceUrl} target="_blank" rel="noreferrer">
                {tr('Исходный код')}
              </a>
            )}
            <p className="form-note">
              <ShieldCheck size={16} />
              {tr(
                'Модуль содержит исполняемый код. Одобрение каталога не гарантирует его безопасность. После установки включите его для нужного сервера отдельно.',
              )}
            </p>
            <label className="marketplace-consent">
              <input
                type="checkbox"
                checked={accepted}
                disabled={busy}
                onChange={(e) => setAccepted(e.target.checked)}
              />
              <span>{tr('Я доверяю автору и разрешаю установить эту версию.')}</span>
            </label>
            {error && (
              <p className="form-error" role="alert">
                {error}
              </p>
            )}
          </div>
          <div className="drawer-footer">
            <button
              className="button"
              disabled={busy}
              onClick={() => {
                setChoice(null);
                setVersions(null);
              }}
            >
              {tr('Отмена')}
            </button>
            <button
              className="button primary"
              disabled={busy || !accepted}
              onClick={() => void install()}
            >
              {tr(busy ? 'Устанавливаем…' : 'Установить')}
            </button>
          </div>
        </Modal>
      )}
    </div>
  );
}
