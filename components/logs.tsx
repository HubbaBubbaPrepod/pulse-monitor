'use client';
import { useLocale, tr, trText } from './locale';
import { useState, useEffect } from 'react';
import { Pause, Play, Download, TriangleAlert, Copy } from 'lucide-react';
import { download, type LogEntry } from '@/lib/data';
import { useNodeMonitor } from './provider';
import { Empty, PageHeading, PageMotion, Panel, SearchInput, Skeleton } from './ui';
import { OfflineBanner } from './connection';
import { api, groupLogs, logTime, type LogPage } from '@/lib/monitor';
function GroupedLogs({ entries }: { entries: LogEntry[] }) {
  return (
    <div className="log-groups">
      {groupLogs(entries).map((group) =>
        group.entries.length === 1 ? (
          <LogFeed key={group.entries[0].id} entries={group.entries} />
        ) : (
          <details
            className="log-group"
            key={JSON.stringify([group.entries[0].source, group.entries[0].level, group.summary])}
          >
            <summary className={`log-line level-${group.entries[0].level.toLowerCase()}`}>
              <time className="mono">{logTime(group.entries[0].time)}</time>
              <span className="log-level">{group.entries[0].level}</span>
              <span className="log-source mono" title={group.entries[0].source}>
                {group.entries[0].source || tr('Система')}
              </span>
              <span className="log-message mono">
                {group.summary === group.entries[0].message ? group.summary : tr(group.summary)}{' '}
                <span className="tag">×{group.entries.length}</span>
                <small>{tr('Раскрыть исходные записи')}</small>
              </span>
            </summary>
            <LogFeed entries={group.entries} />
          </details>
        ),
      )}
    </div>
  );
}
export function LogFeed({ preview = false, entries }: { preview?: boolean; entries?: LogEntry[] }) {
  useLocale();
  const { recentLogs, notify } = useNodeMonitor();
  const rows = entries ?? recentLogs.slice(0, preview ? 7 : 100);
  if (!rows.length)
    return (
      <Empty title={tr('Логи не найдены')} text={tr('Проверьте фильтры и доступность сервера.')} />
    );
  return (
    <div className={`log-feed ${preview ? 'log-feed-preview' : ''}`}>
      {trText(
        rows.map((entry) => (
          <div className={`log-line level-${entry.level.toLowerCase()}`} key={entry.id}>
            <time className="mono">{trText(logTime(entry.time))}</time>
            <span className={`log-level ${entry.level === 'INFO' ? '' : 'log-level-warning'}`}>
              {trText(entry.level !== 'INFO' ? <TriangleAlert size={10} /> : null)}
              {trText(entry.level)}
            </span>
            <span className="log-source mono" title={entry.source}>
              {entry.source || tr('Система')}
            </span>
            <span className="log-message mono" title={entry.message}>
              {entry.message}
            </span>
            {trText(
              !preview ? (
                <button
                  className="icon-button row-action"
                  aria-label={tr('Копировать лог {p0}', { p0: entry.time })}
                  onClick={async () => {
                    try {
                      await navigator.clipboard.writeText(
                        `${entry.time} ${entry.level} ${entry.source} ${entry.message}`,
                      );
                      notify(tr('Строка лога скопирована'));
                    } catch {
                      notify(tr('Копирование недоступно. Выделите текст вручную.'));
                    }
                  }}
                >
                  <Copy size={14} />
                </button>
              ) : null,
            )}
          </div>
        )),
      )}
    </div>
  );
}
export function Logs() {
  useLocale();
  const { server, loading, notify, recentLogs, backend, startedAt, preferences, tick, paused } =
    useNodeMonitor();
  const [query, setQuery] = useState('');
  const [level, setLevel] = useState('ALL');
  const [source, setSource] = useState('ALL');
  const [category, setCategory] = useState('ALL');
  const [grouped, setGrouped] = useState(true);
  const [offset, setOffset] = useState(0);
  const [before, setBefore] = useState('');
  const [revision, setRevision] = useState(0);
  const [result, setResult] = useState<{
    key: string;
    page: LogPage;
    error: string;
  } | null>(null);
  const limit = 100;
  const key = JSON.stringify([server.id, query, level, source, category, offset, before]);
  const liveTick = !before && !paused && preferences.interval === 0 ? tick : 0;
  useEffect(() => {
    if (backend !== 'connected' || loading) return;
    const controller = new AbortController();
    const params = new URLSearchParams({ limit: String(limit), offset: String(offset) });
    if (query) params.set('q', query);
    if (level !== 'ALL') params.set('level', level);
    if (source !== 'ALL') params.set('source', source);
    if (category !== 'ALL') params.set('category', category);
    if (before) params.set('before', before);
    let busy = false;
    const load = async () => {
      if (busy) return;
      busy = true;
      try {
        const page = await api<LogPage>(
          `/servers/${encodeURIComponent(server.id)}/logs?${params}`,
          { signal: controller.signal },
        );
        if (!controller.signal.aborted) setResult({ key, page, error: '' });
      } catch (e) {
        if (!controller.signal.aborted)
          setResult({
            key,
            page: { items: [], total: 0 },
            error: e instanceof Error ? e.message : tr('Ошибка архива'),
          });
      } finally {
        busy = false;
      }
    };
    const delay = setTimeout(() => void load(), 200);
    const timer =
      !before && !paused && preferences.interval > 0
        ? setInterval(() => void load(), preferences.interval * 1000)
        : undefined;
    return () => {
      controller.abort();
      clearTimeout(delay);
      clearInterval(timer);
    };
  }, [
    backend,
    loading,
    server.id,
    query,
    level,
    source,
    category,
    offset,
    before,
    key,
    revision,
    paused,
    preferences.interval,
    liveTick,
  ]);
  const page = result?.key === key ? result.page : null;
  const fallback = recentLogs.filter(
    (l) =>
      (level === 'ALL' || l.level === level) &&
      (source === 'ALL' || l.source === source) &&
      `${l.source} ${l.message} ${l.time}`.toLowerCase().includes(query.toLowerCase()),
  );
  const rows = backend === 'connected' ? (page?.items ?? []) : fallback;
  const total = backend === 'connected' ? (page?.total ?? 0) : fallback.length;
  const sources = [...new Set([...recentLogs, ...rows].map((l) => l.source))];
  const reset = () => {
    setOffset(0);
  };
  function navigate(next: number) {
    if (!before) setBefore(new Date().toISOString());
    setOffset(next);
  }
  return (
    <PageMotion>
      <PageHeading
        title={tr('Весь архив логов.')}
        subtitle={tr('История с первого запуска. Новые записи появляются сверху.')}
        actions={
          <>
            <button
              className="button"
              onClick={() => {
                setBefore(before ? '' : new Date().toISOString());
                setOffset(0);
              }}
            >
              {trText(before ? <Play size={16} /> : <Pause size={16} />)}{' '}
              {trText(before ? tr('К живому потоку') : tr('Пауза просмотра'))}
            </button>
            <button
              className="button"
              disabled={!rows.length}
              onClick={() => {
                download(
                  `${server.name}-logs-${offset}.txt`,
                  rows.map((l) => `${l.time} [${l.level}] ${l.source}: ${l.message}`).join('\n'),
                  'text/plain;charset=utf-8',
                );
                notify(tr('Страница архива экспортирована'));
              }}
            >
              <Download size={16} />
              {tr('Экспорт страницы')}
            </button>
          </>
        }
      />
      <OfflineBanner />
      {trText(
        loading ? (
          <Skeleton />
        ) : (
          <Panel className="logs-panel">
            <div className="logs-toolbar">
              <SearchInput
                label={tr('Поиск по всему архиву\u2026')}
                value={query}
                onChange={(value) => {
                  setQuery(value);
                  reset();
                }}
              />
              <select
                aria-label={tr('Уровень логов')}
                value={level}
                onChange={(e) => {
                  setLevel(e.target.value);
                  reset();
                }}
              >
                <option value="ALL">{tr('Все уровни')}</option>
                <option>INFO</option>
                <option>WARN</option>
                <option>ERROR</option>
              </select>
              <select
                aria-label={tr('Источник логов')}
                value={source}
                onChange={(e) => {
                  setSource(e.target.value);
                  reset();
                }}
              >
                <option value="ALL">{tr('Все источники')}</option>
                {trText(
                  [...new Set([...sources, ...(source === 'ALL' ? [] : [source])])].map((s) => (
                    <option key={s}>{s}</option>
                  )),
                )}
              </select>
              <select
                aria-label={tr('Категория логов')}
                value={category}
                onChange={(e) => {
                  setCategory(e.target.value);
                  reset();
                }}
              >
                <option value="ALL">{tr('Все категории')}</option>
                <option value="system">{tr('Система и приложения')}</option>
                <option value="security">{tr('Безопасность')}</option>
                <option value="telemetry">{tr('Телеметрия')}</option>
                <option value="agent">{tr('Агент')}</option>
              </select>
              <button
                className={`button ${grouped ? 'active' : ''}`}
                aria-pressed={grouped}
                onClick={() => setGrouped((value) => !value)}
              >
                {tr(grouped ? 'Повторы свёрнуты' : 'Все строки')}
              </button>
            </div>
            <div className="terminal-header">
              <span>
                <span className="tiny-dot" />
                {trText(
                  backend === 'connected'
                    ? before
                      ? tr('Архив зафиксирован')
                      : tr('Сохраняется на диске')
                    : tr('Нет соединения с архивом'),
                )}
              </span>
              <span className="mono">
                {trText(total)}
                {' ' + tr('записей')}
              </span>
            </div>
            {trText(
              result?.key === key && result.error ? (
                <div className="empty">
                  <h3>{tr('Не удалось прочитать архив')}</h3>
                  <p role="alert">{trText(result.error)}</p>
                  <button className="button" onClick={() => setRevision((n) => n + 1)}>
                    {tr('Повторить')}
                  </button>
                </div>
              ) : backend === 'connected' && !page ? (
                <div className="archive-loading" role="status">
                  {tr('Загрузка архива\u2026')}
                </div>
              ) : rows.length && grouped ? (
                <GroupedLogs entries={rows} />
              ) : (
                <LogFeed entries={rows} />
              ),
            )}
            <div className="archive-pagination">
              <span className="mono">
                {trText(total ? offset + 1 : 0)}–{trText(Math.min(offset + limit, total))}
                {' ' + tr('из') + ' '}
                {trText(total)}
              </span>
              <div className="button-row">
                <button
                  className="button"
                  disabled={!offset}
                  onClick={() => setOffset(Math.max(0, offset - limit))}
                >
                  {tr('Новее')}
                </button>
                <button
                  className="button"
                  disabled={offset + limit >= total}
                  onClick={() => navigate(offset + limit)}
                >
                  {tr('Раньше')}
                </button>
                <button
                  className="button"
                  disabled={offset + limit >= total}
                  onClick={() => navigate(Math.floor((total - 1) / limit) * limit)}
                >
                  {tr('К первым записям')}
                </button>
              </div>
            </div>
            <div className="table-foot">
              {trText(
                backend === 'connected'
                  ? tr(
                      'Сбор продолжается при паузе и закрытой вкладке. Текущий запуск: {p0}. Архив прошлых запусков сохраняется.',
                      { p0: logTime(startedAt) },
                    )
                  : tr('Для постоянного хранения логов запустите Go-бэкенд.'),
              )}
            </div>
          </Panel>
        ),
      )}
    </PageMotion>
  );
}
