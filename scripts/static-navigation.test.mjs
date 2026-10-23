import assert from 'node:assert/strict';
import { existsSync } from 'node:fs';
import { join } from 'node:path';
import test from 'node:test';

test('export includes the page segments requested by sidebar prefetches', () => {
  const root = new URL('../out/', import.meta.url);
  for (const route of [
    'servers',
    'processes',
    'network',
    'resources',
    'logs',
    'modules',
    'events',
    'settings',
    'login',
    'register',
  ]) {
    const segment = join(route, `__next.${route}.__PAGE__.txt`);
    assert.ok(
      existsSync(new URL(segment.replaceAll('\\', '/'), root)),
      `Missing prefetch response: ${segment}`,
    );
  }
});
