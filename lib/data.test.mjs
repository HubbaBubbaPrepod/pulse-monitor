import test from 'node:test';
import assert from 'node:assert/strict';
import { validAddress, csv, memoryLabel } from './data.ts';

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
