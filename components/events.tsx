'use client';
import { useLocale, tr, trText } from './locale';
import { useEffect, useState } from 'react';
import { Bell, Check, CircleAlert, Info } from 'lucide-react';
import { useMonitor } from './provider';
import { PageHeading, PageMotion, Panel } from './ui';
import { api, logTime, type MonitorEvent } from '@/lib/monitor';
export function Events() {
  useLocale();
  const { servers, notify, backend, preferences, tick, paused } = useMonitor();
  const liveTick = preferences.interval === 0 && !paused ? tick : 0;
  const [rows, setRows] = useState<MonitorEvent[]>([]),
    [filter, setFilter] = useState('all'),
    [error, setError] = useState('');
  useEffect(() => {
    if (backend !== 'connected') return;
    const controller = new AbortController();
    let busy = false;
    const load = async () => {
      if (busy) return;
      busy = true;
      try {
        const items = await api<MonitorEvent[]>('/events', { signal: controller.signal });
        if (!controller.signal.aborted) {
          setRows(items);
          setError('');
        }
      } catch (e) {
        if (!controller.signal.aborted)
          setError(e instanceof Error ? e.message : tr('Не удалось загрузить события'));
      } finally {
        busy = false;
      }
    };
    void load();
    const timer =
      !paused && preferences.interval > 0
        ? setInterval(load, preferences.interval * 1000)
        : undefined;
    return () => {
      controller.abort();
      clearInterval(timer);
    };
  }, [backend, paused, preferences.interval, liveTick]);
  async function acknowledge(id: string) {
    try {
      await api(`/events/${id}/ack`, { method: 'POST', body: '{}' });
      setRows((current) =>
        current.map((row) =>
          id === 'all' || row.id === id ? { ...row, acknowledged: true } : row,
        ),
      );
      notify(tr('События отмечены просмотренными'));
    } catch (e) {
      setError(e instanceof Error ? e.message : tr('Не удалось сохранить'));
    }
  }
  const visible = rows.filter((r) => filter === 'all' || !r.acknowledged);
  return (
    <PageMotion>
      <PageHeading
        title={tr('Ничего не упустить.')}
        subtitle={tr('События агентов и изменения состояния \u00B7 сохраняются на диске')}
        actions={
          <button
            className="button"
            disabled={!rows.some((r) => !r.acknowledged)}
            onClick={() => void acknowledge('all')}
          >
            <Check size={16} />
            {tr('Отметить всё')}
          </button>
        }
      />
      <Panel>
        <div className="table-toolbar">
          <div className="segmented">
            {trText(
              [
                { value: 'all', label: tr('Все события') },
                { value: 'new', label: tr('Непросмотренные') },
              ].map((item) => (
                <button
                  className={filter === item.value ? 'active' : ''}
                  key={item.value}
                  onClick={() => setFilter(item.value)}
                >
                  {trText(item.label)}
                </button>
              )),
            )}
          </div>
          <span className="muted">{tr('Последние 500 событий')}</span>
        </div>
        {trText(
          error ? (
            <p className="form-error" role="alert">
              {trText(error)}
            </p>
          ) : null,
        )}
        <div className="event-list">
          {trText(
            visible.map((row) => (
              <div className={`event-row ${row.acknowledged ? 'acknowledged' : ''}`} key={row.id}>
                <div className="event-icon">
                  {trText(row.level === 'INFO' ? <Info size={21} /> : <CircleAlert size={21} />)}
                </div>
                <div>
                  <div className="event-heading">
                    <h3>{trText(row.title)}</h3>
                    <span className="tag mono">{trText(row.level)}</span>
                  </div>
                  <p>{trText(row.text)}</p>
                  <small className="mono">
                    {servers.find((s) => s.id === row.serverId)?.name ?? row.serverId} ·{' '}
                    {trText(logTime(row.time))}
                  </small>
                </div>
                <button
                  className="icon-button bordered"
                  disabled={row.acknowledged}
                  aria-label={tr('Отметить: {p0}', { p0: row.title })}
                  onClick={() => void acknowledge(row.id)}
                >
                  <Check size={17} />
                </button>
              </div>
            )),
          )}
        </div>
        {trText(
          !visible.length ? (
            <div className="empty-state">
              <Bell size={28} />
              <h3>{tr('Новых событий нет')}</h3>
              <p>
                {tr('Подключения, превышения порога и потеря связи с агентами появятся здесь.')}
              </p>
            </div>
          ) : null,
        )}
      </Panel>
    </PageMotion>
  );
}
