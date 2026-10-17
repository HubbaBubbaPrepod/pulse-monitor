'use client';
import { useLocale, tr, trText } from './locale';
import { formatDateTime, currentLocale } from '@/lib/i18n';
import { useState, useEffect } from 'react';
import { useSearchParams } from 'next/navigation';
import { Puzzle, Upload, Trash2, Check, Settings2 } from 'lucide-react';
import { api } from '@/lib/monitor';
import {
  type ModuleManifest,
  type ModuleAssignment,
  type ModuleResult,
  formatModuleValue,
} from '@/lib/modules';
import { ModuleChart } from './module-chart';
import { useMonitor } from './provider';
import { Marketplace } from './marketplace';
import { PageHeading, PageMotion, Panel, Modal, Empty, Skeleton } from './ui';
function ConfigureModule({ module, onClose }: { module: ModuleManifest; onClose: () => void }) {
  useLocale();
  const { servers, server, notify } = useMonitor();
  const [node, setNode] = useState(server?.id ?? servers[0]?.id ?? ''),
    [config, setConfig] = useState<ModuleAssignment>({
      enabled: false,
      config: Object.fromEntries(
        module.fields.map((f) => [
          f.key,
          f.default ?? (f.type === 'boolean' ? false : f.type === 'number' ? 0 : ''),
        ]),
      ),
    }),
    [error, setError] = useState(''),
    [busy, setBusy] = useState(false),
    [loadedNode, setLoadedNode] = useState('');
  const ready = !!node && loadedNode === node;
  useEffect(() => {
    if (!node) return;
    const abort = new AbortController();
    api<Record<string, ModuleAssignment>>(`/servers/${encodeURIComponent(node)}/modules`, {
      signal: abort.signal,
    })
      .then((items) => {
        if (abort.signal.aborted) return;
        setConfig(
          items[module.id] ?? {
            enabled: false,
            config: Object.fromEntries(
              module.fields.map((f) => [
                f.key,
                f.default ?? (f.type === 'boolean' ? false : f.type === 'number' ? 0 : ''),
              ]),
            ),
          },
        );
        setLoadedNode(node);
      })
      .catch((e) => {
        if (!abort.signal.aborted) setError(e.message);
      });
    return () => abort.abort();
  }, [node, module]);
  async function save() {
    if (!ready || busy) return;
    setBusy(true);
    setError('');
    try {
      await api(`/servers/${encodeURIComponent(node)}/modules/${encodeURIComponent(module.id)}`, {
        method: 'PUT',
        body: JSON.stringify(config),
      });
      notify(tr('Настройки модуля сохранены'));
      window.dispatchEvent(new Event('pulse:modules'));
      onClose();
    } catch (e) {
      setError(e instanceof Error ? e.message : tr('Не удалось сохранить модуль'));
    } finally {
      setBusy(false);
    }
  }
  return (
    <Modal
      title={module.name}
      subtitle={tr('Настройка модуля на выбранном сервере')}
      onClose={onClose}
    >
      <div className="drawer-body" aria-busy={!ready}>
        <label className="field">
          {tr('Сервер')}
          <select
            value={node}
            disabled={busy}
            onChange={(e) => {
              setLoadedNode('');
              setError('');
              setConfig({
                enabled: false,
                config: Object.fromEntries(
                  module.fields.map((f) => [
                    f.key,
                    f.default ?? (f.type === 'boolean' ? false : f.type === 'number' ? 0 : ''),
                  ]),
                ),
              });
              setNode(e.target.value);
            }}
          >
            {trText(
              servers.map((s) => (
                <option key={s.id} value={s.id}>
                  {s.name}
                </option>
              )),
            )}
          </select>
        </label>
        {!ready && !error ? (
          <p className="muted" role="status">
            {tr('Загрузка данных…')}
          </p>
        ) : null}
        <div className="form-note">
          <p>
            {tr(
              'Этот модуль запускает код на сервере с правами агента. Устанавливайте модули авторов, которым доверяете.',
            )}
          </p>
        </div>
        {trText(
          module.permissions?.length ? (
            <div className="module-permissions">
              <strong>{tr('Необходимый доступ')}</strong>
              {trText(module.permissions.map((p) => <p key={p}>{trText(p)}</p>))}
            </div>
          ) : null,
        )}
        {trText(
          module.fields.map((field) => (
            <label className="field" key={field.key}>
              {trText(field.label)}
              {trText(
                field.type === 'boolean' ? (
                  <input
                    type="checkbox"
                    disabled={!ready || busy}
                    checked={!!config.config[field.key]}
                    onChange={(e) =>
                      setConfig({
                        ...config,
                        config: { ...config.config, [field.key]: e.target.checked },
                      })
                    }
                  />
                ) : (
                  <input
                    disabled={!ready || busy}
                    type={
                      field.type === 'password'
                        ? 'password'
                        : field.type === 'number'
                          ? 'number'
                          : 'text'
                    }
                    autoComplete="off"
                    required={field.required}
                    value={String(config.config[field.key] ?? '')}
                    onChange={(e) =>
                      setConfig({
                        ...config,
                        config: {
                          ...config.config,
                          [field.key]:
                            field.type === 'number' ? Number(e.target.value) : e.target.value,
                        },
                      })
                    }
                  />
                ),
              )}
            </label>
          )),
        )}
        <label className="module-enable">
          <input
            type="checkbox"
            disabled={!ready || busy}
            checked={config.enabled}
            onChange={(e) => setConfig({ ...config, enabled: e.target.checked })}
          />
          {tr('Включить модуль на этом сервере')}
        </label>
        {trText(
          error ? (
            <p role="alert" className="form-error">
              {trText(error)}
            </p>
          ) : null,
        )}
      </div>
      <div className="drawer-footer">
        <button className="button" onClick={onClose}>
          {tr('Отмена')}
        </button>
        <button className="button primary" disabled={busy || !ready} onClick={() => void save()}>
          <Check size={16} />
          {tr('Сохранить')}
        </button>
      </div>
    </Modal>
  );
}
function ModuleView({ module }: { module: ModuleManifest }) {
  useLocale();
  const { server, preferences, tick, paused, notify } = useMonitor();
  const serverId = server?.id;
  const liveTick = preferences.interval === 0 && !paused ? tick : 0;
  const [result, setResult] = useState<ModuleResult | null>(null),
    [error, setError] = useState(''),
    [history, setHistory] = useState<ModuleResult[]>([]);
  useEffect(() => {
    if (!serverId) return;
    const abort = new AbortController();
    let busy = false;
    async function load() {
      if (busy) return;
      busy = true;
      try {
        const v = await api<ModuleResult>(
          `/servers/${encodeURIComponent(serverId!)}/modules/${encodeURIComponent(module.id)}/data`,
          { signal: abort.signal },
        );
        if (!abort.signal.aborted) {
          setResult(v);
          setError('');
          setHistory((items) => (items.at(-1)?.at === v.at ? items : [...items, v].slice(-60)));
        }
      } catch (e) {
        if (!abort.signal.aborted)
          setError(e instanceof Error ? e.message : tr('Не удалось загрузить данные модуля'));
      } finally {
        busy = false;
      }
    }
    void load();
    const timer =
      !paused && preferences.interval > 0
        ? setInterval(() => void load(), preferences.interval * 1000)
        : undefined;
    return () => {
      clearInterval(timer);
      abort.abort();
    };
  }, [serverId, module.id, paused, preferences.interval, liveTick]);
  if (!server)
    return (
      <Empty
        title={tr('Подключите сервер')}
        text={tr('Модуль собирает данные на выбранном узле.')}
      />
    );
  if (error)
    return (
      <p className="form-error" role="alert">
        {trText(error)}
      </p>
    );
  if (!result) return <Skeleton />;
  if (result.status !== 'ok')
    return (
      <Empty
        title={result.status === 'waiting' ? tr('Ожидаем данные модуля') : tr('Модуль недоступен')}
        text={result.error || tr('Включите модуль на сервере и проверьте соединение агента.')}
      />
    );
  return (
    <div className="module-views">
      <p className="muted">
        {trText(
          formatDateTime(result.at, currentLocale(), { dateStyle: 'short', timeStyle: 'medium' }),
        )}{' '}
        · {server.name}
      </p>
      {trText(
        module.views.map((view, index) => {
          const raw = result.data[view.key];
          const rows = Array.isArray(raw) ? raw : [];
          if (view.kind === 'table')
            return (
              <Panel key={index}>
                <div className="panel-title">
                  <h2>{trText(view.title)}</h2>
                  <span className="tag">{trText(rows.length)}</span>
                </div>
                <div className="table-scroll">
                  <table>
                    <thead>
                      <tr>
                        {trText(view.columns.map((c) => <th key={c.key}>{trText(c.label)}</th>))}
                      </tr>
                    </thead>
                    <tbody>
                      {trText(
                        rows.map((row, i) => (
                          <tr key={i}>
                            {trText(
                              view.columns.map((c) => (
                                <td key={c.key}>
                                  {c.key === 'id' &&
                                  typeof (row as Record<string, unknown>)?.[c.key] === 'string' ? (
                                    <button
                                      className="text-button mono"
                                      title={String((row as Record<string, unknown>)[c.key])}
                                      aria-label={tr('Скопировать ID')}
                                      onClick={async () => {
                                        try {
                                          await navigator.clipboard.writeText(
                                            String((row as Record<string, unknown>)[c.key]),
                                          );
                                          notify(tr('ID скопирован'));
                                        } catch {
                                          notify(tr('Не удалось скопировать ID'));
                                        }
                                      }}
                                    >
                                      {String((row as Record<string, unknown>)[c.key]).slice(0, 12)}
                                    </button>
                                  ) : (
                                    formatModuleValue(
                                      (row as Record<string, unknown>)?.[c.key],
                                      c.unit,
                                      currentLocale(),
                                    )
                                  )}
                                </td>
                              )),
                            )}
                          </tr>
                        )),
                      )}
                    </tbody>
                  </table>
                </div>
              </Panel>
            );
          if (view.kind === 'cards') {
            const obj = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
            return (
              <Panel key={index}>
                <div className="panel-title">
                  <h2>{trText(view.title)}</h2>
                </div>
                <div className="module-card-grid">
                  {trText(
                    view.columns.map((c) => (
                      <div key={c.key}>
                        <span>{trText(c.label)}</span>
                        <strong>{formatModuleValue(obj[c.key], c.unit, currentLocale())}</strong>
                      </div>
                    )),
                  )}
                </div>
              </Panel>
            );
          }
          if (view.kind === 'logs')
            return (
              <Panel key={index}>
                <div className="panel-title">
                  <h2>{trText(view.title)}</h2>
                </div>
                <div className="module-log-feed mono">
                  {trText(
                    rows.map((row, i) => (
                      <p key={i}>{typeof row === 'string' ? row : formatModuleValue(row)}</p>
                    )),
                  )}
                </div>
              </Panel>
            );
          return (
            <Panel key={index}>
              <div className="panel-title">
                <h2>{trText(view.title)}</h2>
                <span className="muted">{tr('Последние 60 измерений')}</span>
              </div>
              {trText(
                view.columns.map((c) => (
                  <ModuleChart key={c.key} history={history} dataKey={view.key} column={c} />
                )),
              )}
            </Panel>
          );
        }),
      )}
    </div>
  );
}
export function Modules() {
  useLocale();
  const { servers, server, notify } = useMonitor();
  const params = useSearchParams();
  const [modules, setModules] = useState<ModuleManifest[]>([]),
    [revision, setRevision] = useState(0),
    [error, setError] = useState(''),
    [loading, setLoading] = useState(true),
    [configuring, setConfiguring] = useState<ModuleManifest | null>(null),
    [confirm, setConfirm] = useState<ModuleManifest | null>(null);
  useEffect(() => {
    const abort = new AbortController();
    api<ModuleManifest[]>('/modules', { signal: abort.signal })
      .then((items) => {
        setModules(items);
        setLoading(false);
      })
      .catch((e) => {
        if (!abort.signal.aborted) {
          setError(e.message);
          setLoading(false);
        }
      });
    const changed = () => setRevision((n) => n + 1);
    window.addEventListener('pulse:modules', changed);
    return () => {
      abort.abort();
      window.removeEventListener('pulse:modules', changed);
    };
  }, [revision]);
  async function upload(file: File) {
    try {
      if (file.size > 68 * 1024 * 1024) throw new Error(tr('Пакет модуля слишком большой'));
      await api('/modules', { method: 'POST', body: await file.text() });
      setRevision((n) => n + 1);
      notify(tr('Модуль установлен'));
      window.dispatchEvent(new Event('pulse:modules'));
      setError('');
    } catch (e) {
      setError(e instanceof Error ? e.message : tr('Не удалось установить модуль'));
    }
  }
  const [view, setView] = useState<'installed' | 'marketplace'>('installed');
  const selected = modules.find((m) => m.id === params.get('id'));
  return (
    <PageMotion>
      <PageHeading
        title={selected?.name ?? tr('Модули')}
        subtitle={selected?.description ?? tr('Расширяйте мониторинг без изменения ядра.')}
        actions={
          <label className="button primary module-upload">
            <Upload size={16} />
            {tr('Установить модуль')}
            <input
              type="file"
              accept=".json"
              aria-label={tr('Установить модуль')}
              onChange={(e) => {
                const file = e.target.files?.[0];
                if (file) void upload(file);
                e.target.value = '';
              }}
            />
          </label>
        }
      />
      {!selected && (
        <div className="button-row module-tabs" role="group" aria-label={tr('Разделы модулей')}>
          <button
            className={`button ${view === 'installed' ? 'primary' : ''}`}
            aria-pressed={view === 'installed'}
            onClick={() => setView('installed')}
          >
            {tr('Установленные')}
          </button>
          <button
            className={`button ${view === 'marketplace' ? 'primary' : ''}`}
            aria-pressed={view === 'marketplace'}
            onClick={() => setView('marketplace')}
          >
            {tr('Маркетплейс')}
          </button>
        </div>
      )}
      {view === 'marketplace' && !selected && <Marketplace installed={modules} />}
      {trText(
        error ? (
          <p className="form-error" role="alert">
            {trText(error)}
          </p>
        ) : null,
      )}
      {trText(
        view === 'marketplace' && !selected ? null : selected ? (
          <>
            <button
              className="button"
              disabled={!servers.length}
              onClick={() => setConfiguring(selected)}
            >
              <Settings2 size={16} />
              {tr('Настроить')}
            </button>
            <ModuleView key={`${selected.id}-${server?.id}`} module={selected} />
          </>
        ) : loading ? (
          <Skeleton />
        ) : !modules.length ? (
          <Empty
            title={tr('Подключите первый модуль')}
            text={tr(
              'Установите пакет разработчика, выберите сервер и включите сбор данных. Docker-модуль доступен в отдельном релизе pulse-module-docker.',
            )}
          />
        ) : (
          <div className="module-list">
            {trText(
              modules.map((m) => (
                <Panel key={m.id}>
                  <div className="panel-title">
                    <h2>
                      <Puzzle size={18} />
                      {m.name}
                    </h2>
                    <span className="tag">{trText(m.version)}</span>
                  </div>
                  <div className="module-info">
                    <p>{trText(m.description)}</p>
                    <small>
                      {trText(m.author)} · {trText(Object.keys(m.binaries).join(', '))}
                    </small>
                    <div className="button-row">
                      <a className="button" href={`/modules/?id=${encodeURIComponent(m.id)}`}>
                        {tr('Открыть')}
                      </a>
                      <button
                        className="button"
                        disabled={!servers.length}
                        onClick={() => setConfiguring(m)}
                      >
                        {tr('Настроить')}
                      </button>
                      <button
                        className="icon-button"
                        aria-label={tr('Удалить модуль')}
                        onClick={() => setConfirm(m)}
                      >
                        <Trash2 size={16} />
                      </button>
                    </div>
                  </div>
                </Panel>
              )),
            )}
          </div>
        ),
      )}
      {trText(
        configuring ? (
          <ConfigureModule module={configuring} onClose={() => setConfiguring(null)} />
        ) : null,
      )}
      {trText(
        confirm ? (
          <Modal
            title={tr('Удалить модуль?')}
            subtitle={confirm.name}
            onClose={() => setConfirm(null)}
          >
            <div className="drawer-body">
              <p>{tr('Сбор прекратится на всех узлах. Сохранённые данные останутся.')}</p>
            </div>
            <div className="drawer-footer">
              <button className="button" onClick={() => setConfirm(null)}>
                {tr('Отмена')}
              </button>
              <button
                className="button primary"
                onClick={async () => {
                  try {
                    await api(`/modules/${encodeURIComponent(confirm.id)}`, { method: 'DELETE' });
                    setConfirm(null);
                    setRevision((n) => n + 1);
                    window.dispatchEvent(new Event('pulse:modules'));
                  } catch (e) {
                    setError(e instanceof Error ? e.message : tr('Не удалось удалить модуль'));
                  }
                }}
              >
                {tr('Удалить')}
              </button>
            </div>
          </Modal>
        ) : null,
      )}
    </PageMotion>
  );
}
