import test from 'node:test';
import assert from 'node:assert/strict';
import * as selection from './selection.ts';
const servers = [{ id: 'first' }, { id: 'chosen' }];
function memoryStorage() {
  const values = new Map();
  return {
    getItem: (key) => values.get(key) ?? null,
    setItem: (key, value) => values.set(key, value),
    removeItem: (key) => values.delete(key),
  };
}
test('selected server survives a new bootstrap without changing server order', () => {
  assert.equal(typeof selection.persistServerSelection, 'function');
  const storage = memoryStorage();
  selection.persistServerSelection('chosen', storage);
  assert.equal(selection.restoreServerSelection(servers, storage), 'chosen');
});
test('removed or missing saved servers fall back to the first available server', () => {
  assert.equal(typeof selection.restoreServerSelection, 'function');
  const storage = memoryStorage();
  selection.persistServerSelection('removed', storage);
  assert.equal(selection.restoreServerSelection(servers, storage), 'first');
  assert.equal(selection.restoreServerSelection([], storage), '');
});
test('initial unresolved selection does not overwrite the saved server', () => {
  assert.equal(typeof selection.persistServerSelection, 'function');
  const storage = memoryStorage();
  selection.persistServerSelection('chosen', storage);
  selection.persistServerSelection(undefined, storage);
  assert.equal(selection.restoreServerSelection(servers, storage), 'chosen');
  selection.persistServerSelection('', storage);
  assert.equal(selection.restoreServerSelection(servers, storage), 'first');
});
test('blocked browser storage cannot prevent server selection', () => {
  assert.equal(typeof selection.restoreServerSelection, 'function');
  const storage = {
    getItem: () => {
      throw new Error('blocked');
    },
    setItem: () => {
      throw new Error('blocked');
    },
    removeItem: () => {
      throw new Error('blocked');
    },
  };
  assert.equal(selection.restoreServerSelection(servers, storage), 'first');
  assert.doesNotThrow(() => selection.persistServerSelection('chosen', storage));
});
