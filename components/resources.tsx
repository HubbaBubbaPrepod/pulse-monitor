'use client';
import { formatNumber } from '@/lib/i18n';
import { useLocale, tr, trText } from './locale';
import { useNodeMonitor } from './provider';
import { PageHeading, PageMotion, Panel, Skeleton, AnimatedNumber, Empty } from './ui';
import { Metrics, ResourceCard } from './overview';
import { OfflineBanner } from './connection';
import { Chart, Sparkline } from './chart';
export function Resources() {
  useLocale();
  const { loading, server, snapshot, backend } = useNodeMonitor();
  return (
    <PageMotion>
      <PageHeading
        title={tr('Каждый ресурс.')}
        subtitle={tr('Нагрузка, память, диски и сетевой трафик')}
      />
      <OfflineBanner />
      {trText(
        loading ? (
          <Skeleton dashboard />
        ) : server.online ? (
          <>
            <Metrics />
            <div className="chart-layout">
              <Chart mode="network" />
              <ResourceCard />
            </div>
            <div className="chart-layout">
              <Chart mode="disk" />
              <Panel className="settings-panel">
                <div className="panel-title">
                  <h2>{tr('Конфигурация узла')}</h2>
                  <span className="tag">
                    {trText(snapshot?.source === 'agent' ? tr('Агент') : tr('Ожидание'))}
                  </span>
                </div>
                <dl className="detail-list">
                  <div>
                    <dt>{tr('Система')}</dt>
                    <dd>{server.os}</dd>
                  </div>
                  <div>
                    <dt>{tr('Процессор')}</dt>
                    <dd>
                      {trText(
                        snapshot?.source === 'agent'
                          ? tr('{p0} ядер \u00B7 {p1}', {
                              p0: snapshot.hardware?.cpuCores,
                              p1: snapshot.hardware?.cpuModel,
                            })
                          : '—',
                      )}
                    </dd>
                  </div>
                  <div>
                    <dt>{tr('Память')}</dt>
                    <dd>
                      {trText(
                        snapshot?.source === 'agent'
                          ? `${formatNumber(snapshot.hardware?.memoryGB ?? NaN, undefined, { minimumFractionDigits: 1, maximumFractionDigits: 1 })} GB`
                          : '—',
                      )}
                    </dd>
                  </div>
                  <div>
                    <dt>{tr('Видеокарта')}</dt>
                    <dd>
                      {trText(
                        snapshot
                          ? snapshot.gpus.length
                            ? snapshot.gpus
                                .map(
                                  (g) =>
                                    `${g.name} · ${g.supported?.memoryTotal === false ? '—' : g.memoryTotal} GB VRAM`,
                                )
                                .join(', ')
                            : tr('Не обнаружена')
                          : tr('Ожидаем данные'),
                      )}
                    </dd>
                  </div>
                  <div>
                    <dt>{tr('Накопители')}</dt>
                    <dd>
                      {trText(
                        snapshot?.disks
                          .map(
                            (d) =>
                              `${d.mount} · ${formatNumber(d.totalGB, undefined, { minimumFractionDigits: 1, maximumFractionDigits: 1 })} GB`,
                          )
                          .join(', ') || '—',
                      )}
                    </dd>
                  </div>
                  <div>
                    <dt>{tr('Сетевой интерфейс')}</dt>
                    <dd>{trText(snapshot?.source === 'agent' ? tr('Все интерфейсы') : '—')}</dd>
                  </div>
                </dl>
              </Panel>
            </div>
            <Panel className="disk-panel">
              <div className="panel-title">
                <h2>{tr('Накопители')}</h2>
                <span className="tag">
                  {trText(snapshot?.source === 'agent' ? tr('Агент') : tr('Ожидание'))} ·{' '}
                  {trText((snapshot?.disks ?? []).length)}
                  {' ' + tr('диска')}
                </span>
              </div>
              {trText(
                snapshot?.disks.length ? (
                  <div className="table-scroll">
                    <table>
                      <thead>
                        <tr>
                          <th>{tr('Диск / том')}</th>
                          <th>{tr('Заполнено')}</th>
                          <th>{tr('Свободно')}</th>
                          <th>{tr('\u2193 Чтение')}</th>
                          <th>{tr('\u2191 Запись')}</th>
                          <th>IOPS</th>
                          <th>{tr('Занятость')}</th>
                          <th>{tr('Задержка')}</th>
                          <th>{tr('Температура')}</th>
                        </tr>
                      </thead>
                      <tbody>
                        {trText(
                          snapshot.disks.map((d) => (
                            <tr key={d.id}>
                              <td>
                                <strong>{d.name}</strong>
                                <small className="cell-sub mono">
                                  {trText(d.mount)} · {trText(d.filesystem)}
                                </small>
                              </td>
                              <td>
                                <span className="mono">
                                  {trText(
                                    formatNumber(d.usedGB, undefined, {
                                      minimumFractionDigits: 1,
                                      maximumFractionDigits: 1,
                                    }),
                                  )}{' '}
                                  /{' '}
                                  {trText(
                                    formatNumber(d.totalGB, undefined, {
                                      minimumFractionDigits: 1,
                                      maximumFractionDigits: 1,
                                    }),
                                  )}{' '}
                                  GB
                                </span>
                                <div className="usage-track disk-usage">
                                  <span
                                    style={{
                                      transform: `scaleX(${d.totalGB > 0 ? d.usedGB / d.totalGB : 0})`,
                                    }}
                                  />
                                </div>
                              </td>
                              <td className="mono">
                                {trText(
                                  formatNumber(d.totalGB - d.usedGB, undefined, {
                                    minimumFractionDigits: 1,
                                    maximumFractionDigits: 1,
                                  }),
                                )}{' '}
                                GB
                              </td>
                              <td className="mono">
                                {trText(
                                  d.supported?.read === false
                                    ? '—'
                                    : `${formatNumber(d.read, undefined, { minimumFractionDigits: 1, maximumFractionDigits: 1 })} MB/s`,
                                )}
                              </td>
                              <td className="mono">
                                {trText(
                                  d.supported?.write === false
                                    ? '—'
                                    : `${formatNumber(d.write, undefined, { minimumFractionDigits: 1, maximumFractionDigits: 1 })} MB/s`,
                                )}
                              </td>
                              <td className="mono">
                                {trText(
                                  d.supported?.iops === false
                                    ? '—'
                                    : formatNumber(d.iops, undefined, {
                                        minimumFractionDigits: 0,
                                        maximumFractionDigits: 0,
                                      }),
                                )}
                              </td>
                              <td className="mono">
                                {trText(
                                  d.supported?.busy === false
                                    ? '—'
                                    : `${formatNumber(d.busy, undefined, { minimumFractionDigits: 1, maximumFractionDigits: 1 })}%`,
                                )}
                              </td>
                              <td className="mono">
                                {trText(
                                  d.supported?.latency === false
                                    ? '—'
                                    : `${formatNumber(d.latency, undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })} ms`,
                                )}
                              </td>
                              <td className="mono">
                                {trText(d.temperature > 0 ? `${d.temperature}°C` : '—')}
                              </td>
                            </tr>
                          )),
                        )}
                      </tbody>
                    </table>
                  </div>
                ) : (
                  <Empty
                    title={tr('Ожидаем данные дисков')}
                    text={tr('Подключите агент для получения метрик накопителей.')}
                  />
                ),
              )}
            </Panel>
            <Panel className="gpu-panel">
              <div className="panel-title">
                <h2>{tr('Видеокарта')}</h2>
                <span className="tag">
                  {trText(
                    snapshot?.gpus.length
                      ? snapshot.source === 'agent'
                        ? tr('GPU \u00B7 агент')
                        : tr('GPU \u00B7 ожидание')
                      : tr('Обнаружение оборудования'),
                  )}
                </span>
              </div>
              {trText(
                snapshot?.gpus.length ? (
                  snapshot.gpus.map((g) => (
                    <div key={g.id} className="gpu-content">
                      <div className="gpu-heading">
                        <strong>{g.name}</strong>
                        <span className="muted">
                          {trText(g.supported?.memoryTotal === false ? '—' : g.memoryTotal)} GB VRAM
                        </span>
                      </div>
                      <div className="gpu-metrics">
                        <div>
                          <span>{tr('Нагрузка GPU')}</span>
                          <strong>
                            {trText(
                              g.supported?.utilization === false ? (
                                '—'
                              ) : (
                                <AnimatedNumber value={g.utilization} />
                              ),
                            )}
                            %
                          </strong>
                          <Sparkline values={g.history} />
                        </div>
                        <div>
                          <span>{tr('Видеопамять')}</span>
                          <strong>
                            {trText(
                              g.supported?.memoryUsed === false ? (
                                '—'
                              ) : (
                                <AnimatedNumber value={g.memoryUsed} />
                              ),
                            )}{' '}
                            <small>
                              / {trText(g.supported?.memoryTotal === false ? '—' : g.memoryTotal)}{' '}
                              GB
                            </small>
                          </strong>
                          <div className="usage-track">
                            <span
                              style={{
                                transform: `scaleX(${g.memoryTotal > 0 ? g.memoryUsed / g.memoryTotal : 0})`,
                              }}
                            />
                          </div>
                        </div>
                        <div>
                          <span>{tr('Температура')}</span>
                          <strong>
                            {trText(
                              g.supported?.temperature === false
                                ? '—'
                                : formatNumber(g.temperature, undefined, {
                                    minimumFractionDigits: 0,
                                    maximumFractionDigits: 0,
                                  }),
                            )}
                            <small>°C</small>
                          </strong>
                        </div>
                        <div>
                          <span>{tr('Потребление')}</span>
                          <strong>
                            {trText(
                              g.supported?.power === false
                                ? '—'
                                : formatNumber(g.power, undefined, {
                                    minimumFractionDigits: 0,
                                    maximumFractionDigits: 0,
                                  }),
                            )}
                            <small> W</small>
                          </strong>
                          <span>
                            {tr('Вентилятор') + ' '}
                            {trText(
                              g.supported?.fan === false
                                ? '—'
                                : formatNumber(g.fan, undefined, {
                                    minimumFractionDigits: 0,
                                    maximumFractionDigits: 0,
                                  }),
                            )}
                            %
                          </span>
                        </div>
                      </div>
                    </div>
                  ))
                ) : (
                  <Empty
                    title={snapshot ? tr('Видеокарта не обнаружена') : tr('Ожидаем сведения о GPU')}
                    text={
                      snapshot
                        ? tr('GPU не обнаружен или его драйвер не предоставляет метрики.')
                        : backend === 'unavailable'
                          ? tr('Запустите Go-бэкенд для получения сведений об оборудовании.')
                          : tr('Проверяем конфигурацию узла\u2026')
                    }
                  />
                ),
              )}
            </Panel>
          </>
        ) : (
          <Panel className="empty-state">
            <h3>{tr('Метрики недоступны')}</h3>
            <p>{tr('Выберите сервер в сети.')}</p>
          </Panel>
        ),
      )}
    </PageMotion>
  );
}
