export type Server = {
	 source?: 'demo' | 'agent' | 'waiting';
  id: string;
  name: string;
  address: string;
  os: string;
  group: string;
  online: boolean;
  cpu: number;
  memory: number;
  disk: number;
  interval: number;
  threshold: number;
};

export type Sample = { at?: string; cpu: number; memory: number; incoming: number; outgoing: number; read: number; write: number };
export type Process = { pid: number; name: string; user: string; cpu: number; memory: number; gpu: number | null; gpuMemory: number | null; command: string; status: string };
export type LogEntry = { dataSource?: 'demo' | 'agent'; id: string; time: string; level: "INFO" | "WARN" | "ERROR"; source: string; message: string };

export const initialServers: Server[] = [
  { id: "production-01", name: "production-01", address: "https://192.168.1.10:9443", os: "Ubuntu 24.04 LTS", group: "Production", online: true, cpu: 24.8, memory: 62.4, disk: 38.2, interval: 5, threshold: 85 },
  { id: "windows-02", name: "windows-02", address: "https://192.168.1.20:9443", os: "Windows Server 2022", group: "Production", online: true, cpu: 18.6, memory: 44.2, disk: 56.7, interval: 5, threshold: 85 },
  { id: "backup-03", name: "backup-03", address: "https://192.168.1.30:9443", os: "Debian 12", group: "Backup", online: false, cpu: 0, memory: 0, disk: 71.2, interval: 10, threshold: 90 },
];

export function history(server: Server, tick: number, period = 60): Sample[] {
	if (server.source === 'agent' || server.source === 'waiting') return [];
  const seed = server.name.split("").reduce((n, c) => n + c.charCodeAt(0), 0);
  return Array.from({ length: 61 }, (_, i) => {
    const x = i * period / 60 + tick;
    const wave = Math.sin(x * .46 + seed) * 6 + Math.sin(x * 1.8) * 3;
    const spike = Math.pow(Math.max(0, Math.sin(x * .39)), 18) * 23;
    return {
      cpu: server.online ? Math.max(3, Math.min(96, server.cpu + wave + spike)) : 0,
      memory: server.online ? server.memory + Math.sin(x * .16) * 2 : 0,
      incoming: server.online ? 48 + wave * 1.7 + spike : 0,
      outgoing: server.online ? 25 + Math.sin(x * .2) * 9 : 0,
      read: server.online ? 16 + Math.abs(wave) * 3 : 0,
      write: server.online ? 8 + Math.abs(Math.sin(x * .8)) * 16 : 0,
    };
  });
}

export function processes(server: Server, tick: number): Process[] {
	if (server.source === 'agent' || server.source === 'waiting') return [];
  if (!server.online) return [];
  const windows = server.os.startsWith("Windows");
  const rows: [string, number, string, number, number, string][] = windows ? [
    ["sqlservr.exe", 1840, "NT SERVICE", 12.3, 1229, "C:\\Program Files\\Microsoft SQL Server\\sqlservr.exe"],
    ["w3wp.exe", 2368, "IIS APPPOOL", 8.1, 512, "C:\\Windows\\System32\\inetsrv\\w3wp.exe"],
    ["node.exe", 4210, "Administrator", 4.7, 420, "node.exe server.js"],
    ["svchost.exe", 1024, "SYSTEM", 1.2, 96, "svchost.exe -k netsvcs"],
    ["MsMpEng.exe", 3650, "SYSTEM", .8, 180, "Windows Defender Service"],
    ["pulse-agent.exe", 9012, "SYSTEM", .3, 28, "pulse-agent.exe serve"],
  ] : [
    ["nginx", 1142, "www-data", 12.3, 512, "nginx: worker process"],
    ["postgres", 2876, "postgres", 8.1, 1229, "postgres: checkpointer"],
    ["node", 4210, "node", 4.7, 420, "/usr/bin/node /srv/app/server.js"],
    ["systemd", 1, "root", 1.2, 96, "/sbin/init"],
    ["redis-server", 1854, "redis", .8, 148, "redis-server 127.0.0.1:6379"],
    ["docker", 2201, "root", .6, 240, "/usr/bin/dockerd"],
    ["sshd", 931, "root", .1, 12, "sshd: /usr/sbin/sshd -D"],
    ["pulse-agent", 9012, "pulse", .3, 28, "pulse-agent serve"],
  ];
  if(server.id==='production-01') rows.push(['python3',6701,'inference',14,3200,'/usr/bin/python3 /srv/inference.py']);
  return rows.map(([name, pid, user, cpu, memory, command], i) => ({ name, pid, user, cpu: Math.max(.1, cpu + Math.sin(tick + i) * .3), memory, gpu:server.id==='production-01'?(35+Math.sin((29+tick)*.14)*18)*(name==='python3'?.8:name==='node'?.15:name==='docker'?.05:0):null, gpuMemory:server.id==='production-01'?8.4*1024*(name==='python3'?.8:name==='node'?.15:name==='docker'?.05:0):null, command, status: "Работает" }));
}

const messages: [LogEntry["level"], string, string][] = [
  ["INFO", "nginx", '192.168.1.50 — GET /api/status 200 · 12 ms'],
  ["INFO", "systemd", "Started Backup Service (backup.service)"],
  ["WARN", "kernel", "Disk usage exceeded threshold (85%) on /archive"],
  ["INFO", "postgres", "checkpoint complete: wrote 245 buffers (1.2 s)"],
  ["INFO", "node", "Application ready on port 3000"],
  ["INFO", "nginx", '192.168.1.23 — POST /api/login 200 · 8 ms'],
  ["WARN", "systemd", "High memory usage detected in backup.service"],
  ["INFO", "cron", "Completed daily backup (12.4 GB, 3 m 12 s)"],
  ["ERROR", "postgres", "Connection timeout from 192.168.1.42; retry scheduled"],
  ["INFO", "pulse", "Metrics collected successfully"],
];

export function logs(server: Server, tick: number): LogEntry[] {
	if (server.source === 'agent' || server.source === 'waiting') return [];
  if (!server.online) return [];
  return Array.from({ length: 24 }, (_, i) => {
    const [level, source, message] = messages[(i + tick) % messages.length];
    const minute = 59 - Math.floor(i / 2);
    return { id: `${server.id}-${tick}-${i}`, time: `14:${String(minute).padStart(2, "0")}:${String((52 - i * 2 + 60) % 60).padStart(2, "0")}`, level, source: server.os.startsWith("Windows") && source === "systemd" ? "Service Control" : source, message };
  });
}

export function validAddress(value: string): boolean {
  try {
    const url = new URL(value);
    return ["http:", "https:"].includes(url.protocol) && !!url.hostname && !url.username && !url.password && !url.hash && !url.search;
  } catch { return false; }
}

export function csv(rows: (string | number)[][]): string {
  return "\uFEFF" + rows.map(row => row.map(value => {
    const text = String(value);
    const safe = /^[=+@\-\t\r]/.test(text) ? `'${text}` : text;
    return `"${safe.replaceAll('"', '""')}"`;
  }).join(",")).join("\r\n");
}

export function download(name: string, content: string, type = "text/csv;charset=utf-8") {
  const url = URL.createObjectURL(new Blob([content], { type }));
  const link = document.createElement("a");
  link.href = url; link.download = name; link.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

export function memoryLabel(mb: number) { return mb >= 1024 ? `${(mb / 1024).toFixed(1)} GB` : `${Number(mb.toFixed(1))} MB`; }

export function addressLabel(address: string) {
  try { return new URL(address).hostname; } catch { return address; }
}
