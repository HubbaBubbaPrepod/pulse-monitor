'use client';
import { formatNumber } from '@/lib/i18n';
import { useLocale, tr, trText } from './locale';
import { useState } from 'react';
import {
  ArrowDownLeft,
  ArrowUpRight,
  ArrowRight,
  Pause,
  Play,
  Download,
  Network as NetworkIcon,
} from 'lucide-react';
import { useNodeMonitor } from './provider';
import {
  PageMotion,
  PageHeading,
  Panel,
  Skeleton,
  Empty,
  SearchInput,
  Modal,
  AnimatedNumber,
} from './ui';
import { ProcessIcon, OSIcon } from './icons';
import { Chart } from './chart';
import { OfflineBanner } from './connection';
import { csv, download } from '@/lib/data';
import { logTime, type Packet, type Snapshot } from '@/lib/monitor';
export function Network() {
  useLocale();
  const { server, snapshot, loading, backend } = useNodeMonitor();
  const [tab, setTab] = useState('packets');
  const [query, setQuery] = useState('');
  const [direction, setDirection] = useState('all');
  const [protocol, setProtocol] = useState('all');
  const [frozen, setFrozen] = useState<Snapshot | null>(null);
  const [detail, setDetail] = useState<Packet | null>(null);
  const data = frozen?.serverId === server.id ? frozen : snapshot;
  const paused = frozen?.serverId === server.id;
  const packets = (data?.packets ?? []).filter(
    (p) =>
      (direction === 'all' || p.direction === direction) &&
      (protocol === 'all' || p.protocol === protocol) &&
      `${p.source} ${p.destination} ${p.process} ${p.flags} ${p.summary}`
        .toLowerCase()
        .includes(query.toLowerCase()),
  );
  const connections = (data?.connections ?? []).filter(
    (c) =>
      (protocol === 'all' || c.protocol === protocol) &&
      `${c.local} ${c.remote} ${c.process} ${c.state} ${c.pid}`
        .toLowerCase()
        .includes(query.toLowerCase()),
  );
  const last = data?.samples.at(-1);
  return (
    <PageMotion>
      <PageHeading
        title={tr('Весь трафик.')}
        subtitle={tr('Соединения, направления и каждый пакет в выборке.')}
        actions={
          <>
            <button
              className="button"
              disabled={!snapshot}
              onClick={() => setFrozen(paused ? null : snapshot)}
            >
              {trText(paused ? <Play size={16} /> : <Pause size={16} />)}{' '}
              {trText(paused ? tr('Продолжить') : tr('Пауза просмотра'))}
            </button>
            <button
              className="button"
              disabled={!(tab === 'packets' ? packets.length : connections.length)}
              onClick={() =>
                download(
                  `${server.name}-${tab}.csv`,
                  csv(
                    tab === 'packets'
                      ? [
                          [
                            'Time',
                            'Direction',
                            'Source',
                            'Destination',
                            'Protocol',
                            'Bytes',
                            'Process',
                          ],
                          ...packets.map((p) => [
                            p.time,
                            p.direction,
                            p.source,
                            p.destination,
                            p.protocol,
                            p.length,
                            p.process,
                          ]),
                        ]
                      : [
                          [
                            'Local',
                            'Remote',
                            'Protocol',
                            'State',
                            'PID',
                            'Process',
                            'In MB/s',
                            'Out MB/s',
                          ],
                          ...connections.map((c) => [
                            c.local,
                            c.remote,
                            c.protocol,
                            c.state,
                            c.pid,
                            c.process,
                            c.incoming,
                            c.outgoing,
                          ]),
                        ],
                  ),
                )
              }
            >
              <Download size={16} />
              {tr('Экспорт')}
            </button>
          </>
        }
      />
      <OfflineBanner />
      {trText(
        loading ? (
          <Skeleton dashboard />
        ) : (
          <>
            <Panel className="network-flow">
              <div className="flow-end">
                <ArrowDownLeft size={22} />
                <span>{tr('Входящий трафик')}</span>
                <strong>
                  <AnimatedNumber value={last?.incoming ?? 0} /> <small>MB/s</small>
                </strong>
              </div>
              <div className="flow-path">
                <ArrowRight size={16} />
              </div>
              <div className="flow-node">
                <OSIcon os={server.os} />
                <strong>{server.name}</strong>
                <span>
                  {trText(data?.connections.length ?? 0)}
                  {' ' + tr('соединений \u00B7') + ' '}
                  {trText(data?.source === 'agent' ? tr('агент') : tr('ожидание'))}
                </span>
              </div>
              <div className="flow-path">
                <ArrowRight size={16} />
              </div>
              <div className="flow-end">
                <ArrowUpRight size={22} />
                <span>{tr('Исходящий трафик')}</span>
                <strong>
                  <AnimatedNumber value={last?.outgoing ?? 0} /> <small>MB/s</small>
                </strong>
              </div>
            </Panel>
            <Chart mode="network" frozenSnapshot={paused ? frozen : null} />
            <Panel className="traffic-panel">
              <div className="traffic-toolbar">
                <div className="traffic-tabs">
                  <button
                    className={tab === 'packets' ? 'selected' : ''}
                    aria-pressed={tab === 'packets'}
                    onClick={() => setTab('packets')}
                  >
                    {tr('Пакеты') + ' '}
                    <span>{trText(data?.packets.length ?? 0)}</span>
                  </button>
                  <button
                    className={tab === 'connections' ? 'selected' : ''}
                    aria-pressed={tab === 'connections'}
                    onClick={() => setTab('connections')}
                  >
                    {tr('Соединения') + ' '}
                    <span>{trText(data?.connections.length ?? 0)}</span>
                  </button>
                </div>
                <span className="tag">
                  {trText(
                    paused
                      ? tr('Просмотр зафиксирован')
                      : data?.source === 'agent'
                        ? tr('Последняя выборка \u00B7 агент')
                        : tr('Ожидаем данные агента'),
                  )}
                </span>
              </div>
              <div className="logs-toolbar">
                <SearchInput
                  label={tr('Адрес, процесс или флаг\u2026')}
                  value={query}
                  onChange={setQuery}
                />
                {trText(
                  tab === 'packets' ? (
                    <select
                      aria-label={tr('Направление пакетов')}
                      value={direction}
                      onChange={(e) => setDirection(e.target.value)}
                    >
                      <option value="all">{tr('Все направления')}</option>
                      <option value="in">{tr('Входящие')}</option>
                      <option value="out">{tr('Исходящие')}</option>
                    </select>
                  ) : null,
                )}
                <select
                  aria-label={tr('Протокол')}
                  value={protocol}
                  onChange={(e) => setProtocol(e.target.value)}
                >
                  <option value="all">{tr('Все протоколы')}</option>
                  <option>TCP</option>
                  <option>UDP</option>
                  <option>DNS</option>
                </select>
              </div>
              {trText(
                !(tab === 'packets' ? packets.length : connections.length) ? (
                  <Empty
                    title={
                      backend === 'unavailable' ? tr('Бэкенд недоступен') : tr('Трафик не найден')
                    }
                    text={
                      backend === 'unavailable'
                        ? tr('Подключите агент и проверьте доступ к сетевым метрикам.')
                        : data?.source === 'agent' && tab === 'packets'
                          ? (data.capabilities?.packets ?? tr('Захват пакетов выключен'))
                          : tr('Измените фильтры или выберите доступный сервер.')
                    }
                  />
                ) : (
                  <div className="table-scroll">
                    <table className="traffic-table">
                      <thead>
                        {trText(
                          tab === 'packets' ? (
                            <tr>
                              <th>{tr('Время')}</th>
                              <th>{tr('Направление')}</th>
                              <th>{tr('Откуда')}</th>
                              <th>{tr('Куда')}</th>
                              <th>{tr('Протокол / флаги')}</th>
                              <th>{tr('Размер')}</th>
                              <th>{tr('Процесс')}</th>
                              <th />
                            </tr>
                          ) : (
                            <tr>
                              <th>{tr('Локальный адрес')}</th>
                              <th>{tr('Удалённый адрес')}</th>
                              <th>{tr('Протокол')}</th>
                              <th>{tr('Состояние')}</th>
                              <th>{tr('Процесс / PID')}</th>
                              <th>↓ MB/s</th>
                              <th>↑ MB/s</th>
                            </tr>
                          ),
                        )}
                      </thead>
                      <tbody>
                        {trText(
                          tab === 'packets'
                            ? packets.map((p) => (
                                <tr key={p.id}>
                                  <td className="mono">{trText(logTime(p.time))}</td>
                                  <td>
                                    <span className="packet-direction">
                                      {trText(
                                        p.direction === 'in' ? (
                                          <ArrowDownLeft size={14} />
                                        ) : (
                                          <ArrowUpRight size={14} />
                                        ),
                                      )}{' '}
                                      {trText(
                                        p.direction === 'in' ? tr('Входящий') : tr('Исходящий'),
                                      )}
                                    </span>
                                  </td>
                                  <td className="mono">{p.source}</td>
                                  <td className="mono">{trText(p.destination)}</td>
                                  <td>
                                    <span className="tag">{trText(p.protocol)}</span>
                                    <small className="cell-sub mono">{trText(p.flags)}</small>
                                  </td>
                                  <td className="mono">{trText(p.length)} B</td>
                                  <td>
                                    <span className="process-cell">
                                      <ProcessIcon name={p.process} />
                                      {p.process}
                                    </span>
                                  </td>
                                  <td>
                                    <button
                                      className="icon-button"
                                      aria-label={tr('Пакет {p0}', { p0: p.id })}
                                      onClick={() => setDetail(p)}
                                    >
                                      <ArrowUpRight size={16} />
                                    </button>
                                  </td>
                                </tr>
                              ))
                            : connections.map((c) => (
                                <tr key={c.id}>
                                  <td className="mono">{trText(c.local)}</td>
                                  <td className="mono">{trText(c.remote)}</td>
                                  <td>
                                    <span className="tag">{trText(c.protocol)}</span>
                                  </td>
                                  <td className="mono">{trText(c.state)}</td>
                                  <td>
                                    <span className="process-cell">
                                      <ProcessIcon name={c.process} />
                                      {c.process}
                                    </span>
                                    <small className="cell-sub mono">PID {trText(c.pid)}</small>
                                  </td>
                                  <td className="mono">
                                    {trText(
                                      data?.source === 'agent'
                                        ? '—'
                                        : formatNumber(c.incoming, undefined, {
                                            minimumFractionDigits: 2,
                                            maximumFractionDigits: 2,
                                          }),
                                    )}
                                  </td>
                                  <td className="mono">
                                    {trText(
                                      data?.source === 'agent'
                                        ? '—'
                                        : formatNumber(c.outgoing, undefined, {
                                            minimumFractionDigits: 2,
                                            maximumFractionDigits: 2,
                                          }),
                                    )}
                                  </td>
                                </tr>
                              )),
                        )}
                      </tbody>
                    </table>
                  </div>
                ),
              )}
              <div className="table-footer">
                {trText(tab === 'packets' ? packets.length : connections.length)}
                {' ' + tr('в выборке')}
                <span>
                  {trText(
                    data?.source === 'agent'
                      ? (data.capabilities?.packets ?? tr('Метаданные пакетов'))
                      : tr('Нет захвата пакетов'),
                  )}
                </span>
              </div>
            </Panel>
          </>
        ),
      )}
      {trText(
        detail ? (
          <Modal
            title={tr('Пакет в деталях')}
            subtitle={logTime(detail.time)}
            onClose={() => setDetail(null)}
          >
            <div className="drawer-body">
              <div className="form-note">
                <NetworkIcon size={20} />
                <p>{detail.summary}</p>
              </div>
              <dl className="detail-list">
                {trText(
                  Object.entries({
                    Откуда: detail.source,
                    Куда: detail.destination,
                    Направление: detail.direction === 'in' ? tr('Входящий') : tr('Исходящий'),
                    Протокол: detail.protocol,
                    Флаги: detail.flags || '—',
                    Размер: tr('{p0} байт', { p0: detail.length }),
                    Процесс: detail.process,
                  }).map(([label, value]) => (
                    <div key={label}>
                      <dt>{trText(label)}</dt>
                      <dd className="mono">{value}</dd>
                    </div>
                  )),
                )}
              </dl>
              <p className="muted">
                {tr(
                  'Показаны метаданные пакета. Содержимое зашифрованного трафика здесь не расшифровывается.',
                )}
              </p>
            </div>
          </Modal>
        ) : null,
      )}
    </PageMotion>
  );
}
