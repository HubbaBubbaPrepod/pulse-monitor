import assert from 'node:assert/strict';
import test from 'node:test';
import { createRequire } from 'node:module';
const load = createRequire(import.meta.url);
const { newerModuleVersion } = load('../lib/module-version.ts');

test('catalog update labels never recommend downgrades or identical versions', () => {
  assert.equal(newerModuleVersion('1.0.3', '2.0.0-alpha1'), false);
  assert.equal(newerModuleVersion('1.0.3', '1.0.3'), false);
  assert.equal(newerModuleVersion('1.10.0', '1.9.0'), true);
  assert.equal(newerModuleVersion('2.0.0', '2.0.0-alpha1'), true);
  assert.equal(newerModuleVersion('2.0.0-alpha1', '2.0.0'), false);
});
