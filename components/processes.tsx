'use client';
import { useLocale, tr, trText } from './locale';
import { useState } from 'react';
import { ArrowDown, ArrowUp, Copy, Download, ArrowUpRight } from 'lucide-react';
import { formatNumber } from '@/lib/i18n';
import { csv, download, type Process } from '@/lib/data';
import { useNodeMonitor } from './provider';
import { Empty, Modal, PageHeading, PageMotion, Panel, SearchInput, Skeleton } from './ui';
import { OfflineBanner } from './connection';
import { ProcessIcon } from './icons';
import { sortProcesses, intervalLabel, type SortField } from '@/lib/monitor';
function memoryLabel(value: number) {
  return `${formatNumber(value >= 1024 ? value / 1024 : value, undefined, { maximumFractionDigits: 1 })} ${value >= 1024 ? 'GB' : 'MB'}`;
}
export function ProcessTable({
  preview = false,
  query = '',
  sortBy,
}: {
  preview?: boolean;
  query?: string;
  sortBy?: SortField;
}) {
  useLocale();
  const { server, notify, snapshot } = useNodeMonitor();
  const [sort, setSort] = useState<SortField>(sortBy ?? 'cpu');
  const [descending, setDescending] = useState(sortBy !== 'name');
  const [detail, setDetail] = useState<Process | null>(null);
  const values = snapshot?.processes ?? [];
  const filtered = sortProcesses(
    values.filter((p) =>
      `${p.name} ${p.pid} ${p.user}`.toLowerCase().includes(query.toLowerCase()),
    ),
    sort,
    descending,
  );
  const rows = preview ? filtered.slice(0, 4) : filtered;
  function changeSort(field: typeof sort) {
    if (field === sort) setDescending((v) => !v);
    else {
      setSort(field);
      setDescending(field !== 'name');
    }
  }
  return (
    <>
      {trText(
        rows.length ? (
          <div className="table-scroll">
            <table className="process-table">
              <thead>
                <tr>
                  {trText(
                    [
                      { label: tr('Процесс'), key: 'name' },
                      { label: 'PID', key: 'pid' },
                      { label: 'CPU', key: 'cpu' },
                      { label: tr('Память'), key: 'memory' },
                      { label: 'GPU', key: 'gpu' },
                      { label: 'VRAM', key: 'gpuMemory' },
                    ].map((item) => (
                      <th
                        key={item.key}
                        aria-sort={
                          sort === item.key ? (descending ? 'descending' : 'ascending') : 'none'
                        }
                      >
                        <button onClick={() => changeSort(item.key as typeof sort)}>
                          {trText(item.label)}
                          {trText(
                            sort === item.key ? (
                              descending ? (
                                <ArrowDown size={12} />
                              ) : (
                                <ArrowUp size={12} />
                              )
                            ) : null,
                          )}
                        </button>
                      </th>
                    )),
                  )}
                  <th>{tr('Пользователь')}</th>
                  <th>
                    <span className="sr-only">{tr('Подробности')}</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {trText(
                  rows.map((process) => (
                    <tr key={process.pid}>
                      <td>
                        <button className="process-name" onClick={() => setDetail(process)}>
                          <ProcessIcon name={process.name} />
                          <span>{process.name}</span>
                        </button>
                      </td>
                      <td className="mono muted">{trText(process.pid)}</td>
                      <td>
                        <div className="process-cpu">
                          <span className="mono">
                            {trText(
                              formatNumber(process.cpu, undefined, {
                                minimumFractionDigits: 1,
                                maximumFractionDigits: 1,
                              }),
                            )}
                            %
                          </span>
                          <div className="usage-track">
                            <span
                              style={{ transform: `scaleX(${Math.min(1, process.cpu / 35)})` }}
                            />
                          </div>
                        </div>
                      </td>
                      <td className="mono muted">{trText(memoryLabel(process.memory))}</td>
                      <td className="mono muted">
                        {trText(
                          process.gpu == null
                            ? '—'
                            : `${formatNumber(process.gpu, undefined, { minimumFractionDigits: 1, maximumFractionDigits: 1 })}%`,
                        )}
                      </td>
                      <td className="mono muted">
                        {trText(process.gpuMemory == null ? '—' : memoryLabel(process.gpuMemory))}
                      </td>
                      <td className="muted">{process.user}</td>
                      <td>
                        <button
                          className="icon-button row-action"
                          onClick={() => setDetail(process)}
                          aria-label={tr('Подробности процесса {p0}', { p0: process.name })}
                        >
                          <ArrowUpRight size={15} />
                        </button>
                      </td>
                    </tr>
                  )),
                )}
              </tbody>
            </table>
          </div>
        ) : (
          <Empty
            title={server.online ? tr('Процессы не найдены') : tr('Нет данных о процессах')}
            text={
              server.online
                ? tr('Измените имя, PID или пользователя в поиске.')
                : tr('Сервер недоступен. Выберите другой сервер.')
            }
          />
        ),
      )}
      {trText(
        detail ? (
          <Modal
            title={detail.name}
            subtitle={tr('Процесс {p0} \u00B7 {p1}', { p0: detail.pid, p1: server.name })}
            onClose={() => setDetail(null)}
          >
            <div className="drawer-body">
              <div className="detail-icon">
                <ProcessIcon name={detail.name} />
              </div>
              <dl className="detail-grid">
                <div>
                  <dt>PID</dt>
                  <dd className="mono">{trText(detail.pid)}</dd>
                </div>
                <div>
                  <dt>{tr('Пользователь')}</dt>
                  <dd>{detail.user}</dd>
                </div>
                <div>
                  <dt>CPU</dt>
                  <dd>
                    {trText(
                      formatNumber(detail.cpu, undefined, {
                        minimumFractionDigits: 1,
                        maximumFractionDigits: 1,
                      }),
                    )}
                    %
                  </dd>
                </div>
                <div>
                  <dt>{tr('Память')}</dt>
                  <dd>{trText(memoryLabel(detail.memory))}</dd>
                </div>
                <div>
                  <dt>GPU</dt>
                  <dd>
                    {trText(
                      detail.gpu == null
                        ? '—'
                        : `${formatNumber(detail.gpu, undefined, { minimumFractionDigits: 1, maximumFractionDigits: 1 })}%`,
                    )}
                  </dd>
                </div>
                <div>
                  <dt>{tr('Видеопамять')}</dt>
                  <dd>{trText(detail.gpuMemory == null ? '—' : memoryLabel(detail.gpuMemory))}</dd>
                </div>
                <div>
                  <dt>{tr('Состояние')}</dt>
                  <dd>{trText(detail.status)}</dd>
                </div>
                <div>
                  <dt>{tr('Источник')}</dt>
                  <dd>{tr('Ожидание')}</dd>
                </div>
              </dl>
              <div className="form-section">
                <h3>{tr('Команда запуска')}</h3>
                <code className="command-code">{detail.command}</code>
                <button
                  className="button"
                  onClick={async () => {
                    try {
                      await navigator.clipboard.writeText(detail.command);
                      notify(tr('Команда скопирована'));
                    } catch {
                      notify(tr('Копирование недоступно. Выделите команду вручную.'));
                    }
                  }}
                >
                  <Copy size={15} />
                  {tr('Копировать')}
                </button>
              </div>
            </div>
            <div className="modal-footer">
              <button className="button button-primary" onClick={() => setDetail(null)}>
                {tr('Закрыть')}
              </button>
            </div>
          </Modal>
        ) : null,
      )}
    </>
  );
}
export function Processes() {
  useLocale();
  const { server, loading, notify, snapshot, preferences } = useNodeMonitor();
  const [query, setQuery] = useState('');
  const rows = snapshot?.processes ?? [];
  return (
    <PageMotion>
      <PageHeading
        title={tr('Процессы')}
        subtitle={tr('Каждый процесс. Каждая доля ресурса.')}
        actions={
          <button
            className="button"
            disabled={loading || !server.online}
            onClick={() => {
              download(
                `${server.name}-processes.csv`,
                csv([
                  ['Process', 'PID', 'CPU %', 'Memory MB', 'GPU %', 'VRAM MB', 'User'],
                  ...rows.map((p) => [
                    p.name,
                    p.pid,
                    p.cpu.toFixed(1),
                    p.memory,
                    p.gpu ?? '',
                    p.gpuMemory ?? '',
                    p.user,
                  ]),
                ]),
              );
              notify(tr('Процессы экспортированы'));
            }}
          >
            <Download size={16} />
            {tr('Экспорт')}
          </button>
        }
      />
      <OfflineBanner />
      {trText(
        loading ? (
          <Skeleton />
        ) : (
          <>
            <div className="summary-strip reveal">
              <div>
                <span>{tr('Всего процессов')}</span>
                <strong>{trText(rows.length)}</strong>
              </div>
              <div>
                <span>{tr('CPU процессов')}</span>
                <strong>
                  {trText(
                    formatNumber(
                      rows.reduce((n, p) => n + p.cpu, 0),
                      undefined,
                      { minimumFractionDigits: 1, maximumFractionDigits: 1 },
                    ),
                  )}
                  %
                </strong>
              </div>
              <div>
                <span>{tr('Память процессов')}</span>
                <strong>{trText(memoryLabel(rows.reduce((n, p) => n + p.memory, 0)))}</strong>
              </div>
              <div>
                <span>{tr('Обновление')}</span>
                <strong>
                  {trText(intervalLabel(preferences.interval))}{' '}
                  <small>
                    · {trText(server.source === 'agent' ? tr('агент') : tr('ожидание'))}
                  </small>
                </strong>
              </div>
            </div>
            <Panel className="full-table-panel">
              <div className="table-toolbar">
                <h2>{tr('Активные процессы')}</h2>
                <SearchInput
                  label={tr('Имя, PID или пользователь')}
                  value={query}
                  onChange={setQuery}
                />
              </div>
              <ProcessTable query={query} />
              <div className="table-foot">
                {tr(
                  'Сортировка по текущим значениям. Нажмите заголовок столбца, чтобы изменить направление.',
                )}
              </div>
            </Panel>
          </>
        ),
      )}
    </PageMotion>
  );
}
