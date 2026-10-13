import { backendErrorMessage, formatDateTime } from './i18n.ts';
import type { LogEntry, Process, Sample, Server } from './data';

export type Disk = { supported?: Record<string,boolean>; id: string; name: string; mount: string; filesystem: string; totalGB: number; usedGB: number; read: number; write: number; iops: number; busy: number; latency: number; temperature: number };
export type GPU = { supported?: Record<string,boolean>; id: string; name: string; utilization: number; memoryUsed: number; memoryTotal: number; temperature: number; power: number; fan: number; history: number[] };
export type Connection = { id: string; local: string; remote: string; protocol: string; state: string; process: string; pid: number; incoming: number; outgoing: number };
export type Packet = { id: string; time: string; direction: 'in' | 'out'; source: string; destination: string; protocol: string; length: number; flags: string; process: string; summary: string };
export type Snapshot = { serverId: string; source: 'agent' | 'waiting'; tick: number; at: string; hardware?: {cpuCores:number; cpuModel:string; memoryGB:number; uptime:number; hostname:string}; capabilities?: Record<string,string>; samples: Sample[]; processes: Process[]; disks: Disk[]; gpus: GPU[]; connections: Connection[]; packets: Packet[] };
export type MonitorEvent = {id:string;serverId:string;title:string;text:string;time:string;level:string;acknowledged:boolean};
export type LogPage = { items: LogEntry[]; total: number };
export type Bootstrap = { servers: Server[]; source: 'agent' | 'waiting' | 'mixed'; startedAt: string };
export type SortField = 'name' | 'pid' | 'cpu' | 'memory' | 'gpu' | 'gpuMemory';

export function mergeLogs<T extends { id: string; time: string }>(older: T[], newer: T[]): T[] {
 const byID = new Map(older.map(entry => [entry.id, entry]));
 for (const entry of newer) byID.set(entry.id, entry);
 return [...byID.values()].sort((a,b) => b.time.localeCompare(a.time));
}
export function groupLogs<T extends Pick<LogEntry, 'id' | 'time' | 'level' | 'source' | 'message'>>(rows: T[]): { entries: T[]; summary: string }[] {
 const groups = new Map<string, { entries: T[]; summary: string }>();
 for (const entry of rows) {
  let summary = entry.message;
  // Only recognized SSH failure types are grouped across users and IPs.
  if (/^(ssh|sshd)(\.service)?$/.test(entry.source)) {
   if (/^Failed password for /.test(summary)) summary = 'Неудачные попытки входа по SSH';
   else if (/^Invalid user .* from /.test(summary)) summary = 'Попытки SSH-входа с неизвестным пользователем';
   else if (/^Connection closed by (invalid|authenticating) user /.test(summary) && summary.includes('[preauth]')) summary = 'SSH-соединения закрыты до авторизации';
   else if (/^pam_unix\(sshd:auth\): authentication failure;/.test(summary)) summary = 'Отказы аутентификации SSH';
  }
  const key = JSON.stringify([entry.source, entry.level, summary]);
  const group = groups.get(key);
  if (group) group.entries.push(entry);
  else groups.set(key, { entries: [entry], summary });
 }
 return [...groups.values()];
}
export function sortProcesses<T extends Pick<Process, SortField>>(rows: T[], field: SortField, descending: boolean): T[] {
 return [...rows].sort((a,b) => { const av=a[field],bv=b[field]; if(av==null) return bv==null?a.pid-b.pid:1; if(bv==null) return -1; return (field==='name'?a.name.localeCompare(b.name):Number(av)-Number(bv))*(descending?-1:1)||a.pid-b.pid; });
}

export function apiBase() {
 if (typeof window === 'undefined') return '';
 return ['localhost','127.0.0.1'].includes(window.location.hostname) && window.location.port === '3000' ? `http://${window.location.hostname}:8080` : '';
}
export async function api<T>(path: string, options?: RequestInit): Promise<T> {
 const response = await fetch(`${apiBase()}/api${path}`, { ...options, credentials:'include', headers: { 'Content-Type': 'application/json', ...(csrfToken?{'X-CSRF-Token':csrfToken}:{}), ...options?.headers }, cache: 'no-store' });
 if (!response.ok) { const detail = await response.json().catch(() => null); if(response.status===401&&!path.startsWith('/auth/')&&typeof window!=='undefined')window.dispatchEvent(new Event('pulse:unauthorized')); throw new Error(backendErrorMessage(detail?.code,response.status),{cause:detail}); }
 return response.json() as Promise<T>;
}
export function intervalLabel(interval: number) { return interval === 0 ? 'Реалтайм' : `${interval} с`; }
export function logTime(value: string) { return value.includes('T') ? formatDateTime(value,undefined,{ day:'2-digit', month:'2-digit', hour:'2-digit', minute:'2-digit', second:'2-digit' }) : value; }

let csrfToken='';
export function setCSRFToken(value:string){csrfToken=value;}
export function userInitials(name:string=''){
 const parts=name.trim().split(/\s+/).filter(Boolean);
 return (parts.length>1 ? `${Array.from(parts[0])[0]}${Array.from(parts[parts.length-1])[0]}` : Array.from(parts[0]??'')[0]??'?').toLocaleUpperCase('ru-RU');
}
export function safeReturnPath(value:string|null){return ['/', '/servers/', '/processes/', '/resources/', '/network/', '/logs/', '/events/', '/settings/', '/modules/'].includes(value??'')?value!:'/';}
