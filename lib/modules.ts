import { formatNumber, type Locale } from './i18n.ts';
export type ModuleField = {
  key: string;
  label: string;
  type: 'text' | 'password' | 'number' | 'boolean';
  required?: boolean;
  default?: string | number | boolean;
};
export type ModuleColumn = { key: string; label: string; unit?: string };
export type ModuleManifest = {
  schemaVersion: 1;
  protocol: 1;
  id: string;
  version: string;
  name: string;
  author: string;
  description: string;
  permissions: string[];
  fields: ModuleField[];
  views: {
    kind: 'table' | 'cards' | 'chart' | 'logs';
    title: string;
    key: string;
    columns: ModuleColumn[];
  }[];
  binaries: Record<string, { sha256: string }>;
};
export type ModuleAssignment = {
  enabled: boolean;
  config: Record<string, string | number | boolean>;
};
export type ModuleResult = {
  at: string;
  status: 'waiting' | 'ok' | 'error' | 'unsupported';
  error?: string;
  data: Record<string, unknown>;
};
export function moduleChartSeries(
  history: Pick<ModuleResult, 'data'>[],
  key: string,
  column: string,
) {
  const values = history.map((item) => {
    const object = item.data[key];
    const value =
      object && typeof object === 'object' ? (object as Record<string, unknown>)[column] : null;
    return typeof value === 'number' && Number.isFinite(value) ? value : null;
  });
  const max = Math.max(1, ...values.filter((value): value is number => value !== null));
  const segments: string[] = [];
  let points: string[] = [];
  values.forEach((value, index) => {
    if (value === null) {
      if (points.length) segments.push(points.join(' '));
      points = [];
      return;
    }
    points.push(`${(index * 600) / Math.max(1, values.length - 1)},${130 - (value / max) * 110}`);
  });
  if (points.length) segments.push(points.join(' '));
  return { values, segments, latest: values.at(-1) ?? null, max };
}
export function formatModuleValue(value: unknown, unit?: string, locale: Locale = 'en') {
  if (value === null || value === undefined) return '—';
  if (typeof value === 'boolean')
    return locale === 'ru' ? (value ? 'Да' : 'Нет') : value ? 'Yes' : 'No';
  if (typeof value === 'number')
    return `${formatNumber(value, locale, { useGrouping: false })}${unit ? ' ' + unit : ''}`;
  return typeof value === 'object' ? JSON.stringify(value) : String(value);
}
