'use client';

import { useId, useState } from 'react';
import { formatDateTime, currentLocale } from '@/lib/i18n';
import {
  formatModuleValue,
  moduleChartSeries,
  type ModuleResult,
  type ModuleColumn,
} from '@/lib/modules';
import { tr, trText, useLocale } from './locale';

const W = 780,
  H = 230,
  LEFT = 58,
  RIGHT = 18,
  TOP = 12,
  BOTTOM = 30;

export function ModuleChart({
  history,
  dataKey,
  column,
}: {
  history: ModuleResult[];
  dataKey: string;
  column: ModuleColumn;
}) {
  useLocale();
  const gradient = useId();
  const [cursor, setCursor] = useState<number | null>(null);
  const [visible, setVisible] = useState(true);
  const [table, setTable] = useState(false);
  const series = moduleChartSeries(history, dataKey, column.key);
  const max = series.max * 1.1;
  const width = W - LEFT - RIGHT,
    height = H - TOP - BOTTOM;
  const x = (i: number) => LEFT + (width * i) / Math.max(1, series.values.length - 1);
  const y = (value: number) => TOP + height * (1 - value / max);
  const segments: { d: string; first: number; last: number }[] = [];
  series.values.forEach((value, i) => {
    if (value === null) return;
    if (i === 0 || series.values[i - 1] === null)
      segments.push({ d: `M${x(i)},${y(value)}`, first: i, last: i });
    else {
      const segment = segments[segments.length - 1];
      segment.d += ` L${x(i)},${y(value)}`;
      segment.last = i;
    }
  });
  const index = cursor === null ? null : Math.max(0, Math.min(history.length - 1, cursor));
  const time = (i: number) =>
    history[i]?.at
      ? formatDateTime(history[i].at, currentLocale(), {
          hour: '2-digit',
          minute: '2-digit',
          second: '2-digit',
        })
      : '—';
  return (
    <div className="module-resource-chart">
      <div className="panel-title chart-title">
        <div className="chart-legend">
          <button
            aria-pressed={visible}
            className={visible ? '' : 'series-hidden'}
            onClick={() => setVisible(!visible)}
          >
            <span className="legend-line" />
            {trText(column.label)}
          </button>
        </div>
        <strong className="mono">
          {formatModuleValue(series.latest, column.unit, currentLocale())}
        </strong>
      </div>
      <div className="chart-canvas">
        <svg
          viewBox={`0 0 ${W} ${H}`}
          role="img"
          aria-label={tr(column.label)}
          tabIndex={0}
          onFocus={() => setCursor(Math.max(0, history.length - 1))}
          onBlur={() => setCursor(null)}
          onPointerLeave={() => setCursor(null)}
          onPointerMove={(event) => {
            const box = event.currentTarget.getBoundingClientRect();
            const fraction = (((event.clientX - box.left) / box.width) * W - LEFT) / width;
            setCursor(
              Math.round(Math.max(0, Math.min(1, fraction)) * Math.max(0, history.length - 1)),
            );
          }}
          onKeyDown={(event) => {
            if (['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) {
              event.preventDefault();
              setCursor(
                event.key === 'Home'
                  ? 0
                  : event.key === 'End'
                    ? history.length - 1
                    : Math.max(
                        0,
                        Math.min(
                          history.length - 1,
                          (index ?? 0) + (event.key === 'ArrowLeft' ? -1 : 1),
                        ),
                      ),
              );
            }
          }}
        >
          <defs>
            <linearGradient id={gradient} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0" stopColor="var(--chart-primary)" stopOpacity=".18" />
              <stop offset="1" stopColor="var(--chart-primary)" stopOpacity=".01" />
            </linearGradient>
          </defs>
          {[0, 0.25, 0.5, 0.75, 1].map((fraction) => (
            <g key={fraction}>
              <line
                x1={LEFT}
                x2={W - RIGHT}
                y1={y(max * fraction)}
                y2={y(max * fraction)}
                className="chart-grid"
              />
              <text
                x={LEFT - 10}
                y={y(max * fraction) + 4}
                textAnchor="end"
                className="chart-label"
              >
                {formatModuleValue(max * fraction, undefined, currentLocale())}
              </text>
            </g>
          ))}
          {[0, 0.25, 0.5, 0.75, 1].map((fraction) => {
            const i = Math.round(fraction * Math.max(0, history.length - 1));
            const position = LEFT + fraction * width;
            return (
              <g key={fraction}>
                <line x1={position} x2={position} y1={TOP} y2={H - BOTTOM} className="chart-grid" />
                <text
                  x={position}
                  y={H - 7}
                  textAnchor={fraction === 0 ? 'start' : fraction === 1 ? 'end' : 'middle'}
                  className="chart-label"
                >
                  {time(i)}
                </text>
              </g>
            );
          })}
          {visible &&
            segments.map((segment, i) => (
              <g key={i}>
                <path
                  d={`${segment.d} L${x(segment.last)},${H - BOTTOM} L${x(segment.first)},${H - BOTTOM} Z`}
                  fill={`url(#${gradient})`}
                />
                <path
                  d={segment.d}
                  fill="none"
                  stroke="var(--chart-primary)"
                  strokeWidth="1.8"
                  vectorEffect="non-scaling-stroke"
                />
                {segment.first === segment.last && (
                  <circle
                    cx={x(segment.first)}
                    cy={y(series.values[segment.first]!)}
                    r="3"
                    fill="var(--chart-primary)"
                  />
                )}
              </g>
            ))}
          {index !== null && history.length > 0 && (
            <line
              x1={x(index)}
              x2={x(index)}
              y1={TOP}
              y2={H - BOTTOM}
              stroke="var(--muted)"
              strokeDasharray="3 4"
            />
          )}
        </svg>
        {index !== null && history.length > 0 && (
          <div
            className="chart-tooltip"
            style={{ left: `${Math.min(73, Math.max(8, (x(index) / W) * 100))}%` }}
          >
            <span className="mono">{time(index)}</span>
            <div>
              <span>{trText(column.label)}</span>
              <strong>
                {formatModuleValue(series.values[index], column.unit, currentLocale())}
              </strong>
            </div>
          </div>
        )}
      </div>
      <div className="chart-footer">
        <span className="muted">{tr('Последние 60 измерений')}</span>
        <button className="text-button" onClick={() => setTable(!table)}>
          {table ? tr('Скрыть значения') : tr('Таблица значений')}
        </button>
      </div>
      {table && (
        <div className="chart-data-table">
          <table>
            <thead>
              <tr>
                <th>{tr('Время')}</th>
                <th>{trText(column.label)}</th>
              </tr>
            </thead>
            <tbody>
              {history.map((sample, i) => (
                <tr key={`${sample.at}-${i}`}>
                  <td>{time(i)}</td>
                  <td>{formatModuleValue(series.values[i], column.unit, currentLocale())}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
