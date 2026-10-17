import test from 'node:test';
import assert from 'node:assert/strict';
import { parseRetentionDays, retentionNeedsConfirmation } from './retention.ts';
test('retention accepts bounded whole days and explicit disabled value', () => {
  for (const value of ['0', '1', '7', '30', '3650'])
    assert.equal(parseRetentionDays(value), Number(value));
  for (const value of ['', ' ', '-1', '1.5', '3651', 'Infinity', '1e2'])
    assert.equal(parseRetentionDays(value), null);
});
test('enabling deletion or shortening retention needs explicit warning', () => {
  assert.equal(retentionNeedsConfirmation(0, 30), true);
  assert.equal(retentionNeedsConfirmation(30, 7), true);
  assert.equal(retentionNeedsConfirmation(30, 90), false);
  assert.equal(retentionNeedsConfirmation(30, 0), false);
  assert.equal(retentionNeedsConfirmation(30, 30), false);
});
