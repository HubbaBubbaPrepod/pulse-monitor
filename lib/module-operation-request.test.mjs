import test from 'node:test';
import assert from 'node:assert/strict';
import {
  operationRequest,
  clearOperationRequest,
  readOperationRequest,
} from './module-operation-request.ts';

const storage = () => {
  const values = new Map();
  return {
    getItem: (key) => values.get(key) ?? null,
    setItem: (key, value) => values.set(key, value),
    removeItem: (key) => values.delete(key),
  };
};

test('a lost response retries with the persisted key and never creates another command', async () => {
  const store = storage();
  const commands = new Map();
  const keys = [];
  const send = async (key) => {
    assert.equal(readOperationRequest(store, '/scope').key, key);
    keys.push(key);
    if (!commands.has(key)) commands.set(key, { id: 'original', status: 'pending' });
    if (keys.length === 1) throw new TypeError('response lost');
    return commands.get(key);
  };
  const result = await operationRequest(store, '/scope', async () => null, send);
  assert.equal(result.id, 'original');
  assert.equal(commands.size, 1);
  assert.equal(keys.length, 2);
  assert.equal(keys[0], keys[1]);
});

test('a reload recovers a completed request without sending a new mutation', async () => {
  const store = storage();
  await operationRequest(
    store,
    '/scope',
    async () => null,
    async () => {
      throw new TypeError('lost');
    },
  ).catch(() => {});
  let sends = 0;
  const oldKey = readOperationRequest(store, '/scope').key;
  const result = await operationRequest(
    store,
    '/scope',
    async (key) => {
      assert.equal(key, oldKey);
      return { id: 'completed-before-reload', status: 'succeeded' };
    },
    async () => {
      sends++;
    },
  );
  assert.equal(result.id, 'completed-before-reload');
  assert.equal(sends, 0);
  clearOperationRequest(store, '/scope');
  assert.equal(readOperationRequest(store, '/scope'), null);
});

test('HTTP rejection is not retried and an expired unresolved key cannot create a new command', async () => {
  const store = storage();
  let sends = 0;
  await assert.rejects(
    operationRequest(
      store,
      '/scope',
      async () => null,
      async () => {
        sends++;
        throw new Error('403');
      },
    ),
    /403/,
  );
  assert.equal(sends, 1);
  await assert.rejects(
    operationRequest(
      store,
      '/scope',
      async () => null,
      async () => {
        sends++;
      },
      undefined,
      Date.now() + 25 * 3600000,
    ),
    /requestExpired/,
  );
  assert.equal(sends, 1);
});
