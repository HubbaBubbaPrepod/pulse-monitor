import test from 'node:test';
import assert from 'node:assert/strict';
import * as modules from './modules.ts';

test('module charts preserve unavailable samples as gaps instead of zero', () => {
  assert.equal(typeof modules.moduleChartSeries, 'function');
  const data = [
    { usage: { cpu: 2 } },
    { usage: { cpu: null } },
    { usage: {} },
    { usage: { cpu: 0 } },
    { usage: { cpu: '3' } },
    { usage: { cpu: 4 } },
  ];
  const series = modules.moduleChartSeries(
    data.map((data) => ({ data })),
    'usage',
    'cpu',
  );
  assert.deepEqual(series.values, [2, null, null, 0, null, 4]);
  assert.equal(series.segments.length, 3);
  assert.equal(series.latest, 4);
});

test('module values localize numbers without altering arbitrary remote strings', () => {
  assert.equal(modules.formatModuleValue(null), '—');
  assert.equal(modules.formatModuleValue(1.25, 'MiB', 'ru'), '1,25 MiB');
  assert.equal(modules.formatModuleValue('Сервер', undefined, 'en'), 'Сервер');
});
