'use client';
import { useState, useEffect, type ReactNode } from 'react';
import { useLocale, tr, trText } from './locale';
import { useMonitor } from './provider';
import { api } from '@/lib/monitor';
import { type ModuleManifest, type ModuleResult, formatModuleValue } from '@/lib/modules';
import { formatDateTime, currentLocale } from '@/lib/i18n';
import { Panel, Empty, Skeleton } from './ui';
import { ModuleChart } from './module-chart';
export function ModuleDataView({
  module,
  endpoint,
  monitorScope = false,
  rowControls,
  onRowSelect,
  revision = 0,
}: {
  module: Pick<ModuleManifest, 'id' | 'views'>;
  endpoint: string;
  monitorScope?: boolean;
  rowControls?: (view: ModuleManifest['views'][number], row: Record<string, unknown>) => ReactNode;
  onRowSelect?: (view: ModuleManifest['views'][number], row: Record<string, unknown>) => void;
  revision?: number;
}) {
  useLocale();
  const { server, preferences, tick, paused, notify } = useMonitor();
  const serverId = server?.id;
  const liveTick = preferences.interval === 0 && !paused ? tick : 0;
  const [result, setResult] = useState<ModuleResult | null>(null),
    [error, setError] = useState(''),
    [history, setHistory] = useState<ModuleResult[]>([]);
  useEffect(() => {
    if (!serverId && !monitorScope) return;
    const abort = new AbortController();
    let busy = false;
    async function load() {
      if (busy) return;
      busy = true;
      try {
        const v = await api<ModuleResult>(endpoint, { signal: abort.signal });
        if (!abort.signal.aborted) {
          const normalized = {
            ...v,
            status: String(v.status) === 'succeeded' ? 'ok' : v.status,
          } as ModuleResult;
          setResult(normalized);
          setError('');
          setHistory((items) =>
            items.at(-1)?.at === v.at ? items : [...items, normalized].slice(-60),
          );
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
  }, [
    serverId,
    module.id,
    endpoint,
    monitorScope,
    paused,
    preferences.interval,
    liveTick,
    revision,
  ]);
  if (!server && !monitorScope)
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
        · {monitorScope ? tr('Монитор') : server?.name}
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
                        {rowControls && <th>{tr('Действия')}</th>}
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
                                  ) : onRowSelect &&
                                    c.key === view.columns.find((col) => col.key !== 'id')?.key ? (
                                    <button
                                      className="text-button"
                                      onClick={() =>
                                        onRowSelect(view, row as Record<string, unknown>)
                                      }
                                    >
                                      {formatModuleValue(
                                        (row as Record<string, unknown>)?.[c.key],
                                        c.unit,
                                        currentLocale(),
                                      )}
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
                            {rowControls && (
                              <td>{rowControls(view, row as Record<string, unknown>)}</td>
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
