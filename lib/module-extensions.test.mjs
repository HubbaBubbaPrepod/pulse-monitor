import test from 'node:test';
import assert from 'node:assert/strict';
import {
  componentEndpoint,
  operationMatches,
  actionAvailable,
  configurationUpdate,
} from './module-extensions.ts';

test('saving public configuration excludes read-only secret metadata', () => {
  const loaded = {
    enabled: true,
    version: '2.0.0',
    config: { socket: '' },
    grants: ['actions.execute'],
    secrets: ['token'],
    clearSecrets: ['old'],
  };
  assert.deepEqual(configurationUpdate(loaded), {
    enabled: true,
    config: { socket: '' },
    grants: ['actions.execute'],
    clearSecrets: ['old'],
  });
  assert.deepEqual(configurationUpdate({ enabled: false, config: {}, grants: [] }), {
    enabled: false,
    config: {},
    grants: [],
  });
});
test('component scopes use exact encoded module, component and server identities', () => {
  assert.equal(
    componentEndpoint('a/b', 'c d', 'node/a', 'data'),
    '/extensions/a%2Fb/components/c%20d/data?server=node%2Fa',
  );
  assert.equal(componentEndpoint('a', 'b', '', 'data'), '/extensions/a/components/b/data');
});
test('late results must match the operation and selected component scope', () => {
  const expected = {
    id: 'op',
    moduleId: 'docker',
    componentId: 'engine',
    serverId: 'node-a',
    action: 'logs',
  };
  assert.equal(operationMatches({ ...expected, status: 'succeeded' }, expected), true);
  for (const key of Object.keys(expected))
    assert.equal(operationMatches({ ...expected, [key]: 'other' }, expected), false);
});
test('controls respect grants, pending state, row identity and observed conditions', () => {
  const action = {
    type: 'manual',
    action: 'start',
    mutating: true,
    inputKey: 'containerId',
    when: { key: 'state', values: ['exited', 'created'] },
  };
  assert.equal(
    actionAvailable(
      action,
      { id: 'full', state: 'exited' },
      'id',
      true,
      ['actions.execute'],
      false,
    ),
    true,
  );
  assert.equal(
    actionAvailable(
      action,
      { id: 'full', state: 'running' },
      'id',
      true,
      ['actions.execute'],
      false,
    ),
    false,
  );
  assert.equal(
    actionAvailable(action, { id: 'full', state: 'exited' }, 'id', true, [], false),
    false,
  );
  assert.equal(
    actionAvailable(action, { id: 'full', state: 'exited' }, 'id', true, ['actions.execute'], true),
    false,
  );
  assert.equal(
    actionAvailable(action, { state: 'exited' }, 'id', true, ['actions.execute'], false),
    false,
  );
});

test('optional action grants are enforced before exposing controls', () => {
  const action = { type: 'manual', action: 'logs', requires: ['docker.logs'] };
  assert.equal(actionAvailable(action, { id: 'full' }, 'id', true, [], false), false);
  assert.equal(actionAvailable(action, { id: 'full' }, 'id', true, ['docker.logs'], false), true);
});
