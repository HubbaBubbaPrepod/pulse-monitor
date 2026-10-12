import test from 'node:test';
import assert from 'node:assert/strict';
import { validAddress, csv, history, processes, logs, initialServers, memoryLabel } from './demo.ts';

test('fractional VRAM values display without floating point artifacts',()=>{
 assert.equal(memoryLabel(430.08000000000004),'430.1 MB');
 assert.equal(memoryLabel(512),'512 MB');
});

test('agent addresses reject credentials, scripts and malformed URLs', () => {
  assert.equal(validAddress('https://192.168.1.10:9443'), true);
  for (const value of ['not a url', 'javascript:alert(1)', 'ftp://host', 'https://user:secret@host', 'https://host?token=secret', 'https://host#token']) assert.equal(validAddress(value), false);
});
test('CSV escapes quotes and neutralizes spreadsheet formulas', () => {
  const result = csv([['=HYPERLINK("bad")', 'a,b', 'a"b']]);
  assert.equal(result, '\uFEFF"\'=HYPERLINK(""bad"")","a,b","a""b"');
});
test('offline servers never expose current metrics, processes or logs', () => {
  const server = initialServers.find(s => !s.online);
  assert.deepEqual(processes(server, 5), []);
  assert.deepEqual(logs(server, 5), []);
  assert.ok(history(server, 5).every(sample => Object.values(sample).every(v => v === 0)));
});
test('demo process data matches the selected server platform', () => {
  const windows = initialServers.find(s => s.os.startsWith('Windows'));
  assert.ok(processes(windows, 0).every(p => p.name.endsWith('.exe')));
  assert.ok(history(windows, 0, 1440).every(s => Object.values(s).every(Number.isFinite)));
});

test('agent and waiting nodes never fall back to synthetic data',()=>{
 for(const source of ['agent','waiting']){
  const server={...initialServers[0],source};
  assert.deepEqual(history(server,1),[]);
  assert.deepEqual(processes(server,1),[]);
  assert.deepEqual(logs(server,1),[]);
 }
});
