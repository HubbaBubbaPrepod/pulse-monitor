import test from 'node:test';
import assert from 'node:assert/strict';
import * as data from './data.ts';

test('history displays the observation average while retaining independent peaks', () => {
  assert.equal(typeof data.sampleMetric, 'function', 'history needs summary-aware chart values');
  const sample = {
    at: '2026-11-20T10:00:00Z',
    cpu: 90,
    memory: 20,
    summary: {
      count: 2,
      start: '2026-11-20T10:00:00Z',
      end: '2026-11-20T10:00:05Z',
      avg: { cpu: 50, memory: 50 },
      min: { cpu: 10, memory: 20 },
      max: { cpu: 90, memory: 80 },
    },
  };
  assert.deepEqual(data.sampleMetric(sample, 'cpu'), { value: 50, min: 10, max: 90, count: 2 });
  assert.deepEqual(data.sampleMetric(sample, 'memory'), { value: 50, min: 20, max: 80, count: 2 });
});

test('legacy and live points retain their scalar value without invented extrema', () => {
  assert.equal(typeof data.sampleMetric, 'function', 'legacy chart values remain supported');
  assert.deepEqual(data.sampleMetric({ cpu: 23 }, 'cpu'), {
    value: 23,
    min: 23,
    max: 23,
    count: 1,
  });
});

test('invalid or incomplete summaries cannot distort the chart scale', () => {
  assert.equal(
    typeof data.sampleMetric,
    'function',
    'invalid summaries fall back to their live value',
  );
  for (const summary of [
    { count: 0, avg: { cpu: 50 }, min: { cpu: 10 }, max: { cpu: 90 } },
    { count: 2, avg: { cpu: 150 }, min: { cpu: 10 }, max: { cpu: 90 } },
    { count: 2, avg: { cpu: 50 }, min: { cpu: 10 }, max: {} },
  ]) {
    assert.deepEqual(data.sampleMetric({ cpu: 23, summary }, 'cpu'), {
      value: 23,
      min: 23,
      max: 23,
      count: 1,
    });
  }
});
