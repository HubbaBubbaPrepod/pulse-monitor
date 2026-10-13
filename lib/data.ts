export type Server = {
	 source?: 'agent' | 'waiting';
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
export type LogEntry = { dataSource?: 'agent'; id: string; time: string; level: "INFO" | "WARN" | "ERROR"; source: string; message: string };

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
