import test from 'node:test';
import assert from 'node:assert/strict';
import { mergeLogs, sortProcesses, safeReturnPath } from './monitor.ts';
import * as monitor from './monitor.ts';

test('avatars use account initials instead of a fixed administrator letter', () => {
 assert.equal(typeof monitor.userInitials,'function','account initials must be derived from the registered name');
 assert.equal(monitor.userInitials('Danil Kolbasenko'),'DK');
 assert.equal(monitor.userInitials('  Данил   Колбасенко  '),'ДК');
 assert.equal(monitor.userInitials('Danil'),'D');
 assert.equal(monitor.userInitials(''),'?');
});

test('login redirects only to known local monitoring pages', () => {
 for (const path of ['https://evil.test','//evil.test','/api/auth/logout','/login/']) assert.equal(safeReturnPath(path),'/');
 assert.equal(safeReturnPath('/processes/'),'/processes/');
});
test('missing GPU readings remain last in both sorting directions', () => {
 const rows=[{pid:1,gpu:null},{pid:2,gpu:0},{pid:3,gpu:45}];
 assert.deepEqual(sortProcesses(rows,'gpu',true).map(p=>p.pid),[3,2,1]);
 assert.deepEqual(sortProcesses(rows,'gpu',false).map(p=>p.pid),[2,3,1]);
});

test('appending live logs preserves older entries and deduplicates reconnects', () => {
 const old = [{id:'old',time:'2026-10-12T10:00:00Z',level:'INFO',source:'test',message:'first'}];
 const fresh = {id:'new',time:'2026-10-12T10:00:01Z',level:'WARN',source:'test',message:'second'};
 const result = mergeLogs(old,[fresh]);
 assert.deepEqual(result.map(r=>r.id),['new','old']);
 assert.equal(mergeLogs(result,[fresh]).length,2);
});
test('processes sort by numeric memory descending without mutating data', () => {
 const rows = [{name:'small',pid:1,memory:96,cpu:20},{name:'large',pid:2,memory:1229,cpu:1},{name:'medium',pid:3,memory:512,cpu:5}];
 assert.deepEqual(sortProcesses(rows,'memory',true).map(p=>p.pid),[2,3,1]);
 assert.deepEqual(rows.map(p=>p.pid),[1,2,3]);
 assert.deepEqual(sortProcesses(rows,'memory',false).map(p=>p.pid),[1,3,2]);
});
