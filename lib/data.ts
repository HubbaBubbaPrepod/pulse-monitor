export type Server = {
  logRetentionDays?: number;
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

export type SampleField = 'cpu' | 'memory' | 'incoming' | 'outgoing' | 'read' | 'write';
export type SampleSummary = {
  count: number;
  start: string;
  end: string;
  min: Record<SampleField, number>;
  max: Record<SampleField, number>;
  avg: Record<SampleField, number>;
};
export type Sample = {
  at?: string;
  cpu: number;
  memory: number;
  incoming: number;
  outgoing: number;
  read: number;
  write: number;
  summary?: SampleSummary;
};

export function sampleMetric(sample: Sample, field: SampleField) {
  const summary = sample.summary;
  const min = summary?.min?.[field],
    max = summary?.max?.[field],
    avg = summary?.avg?.[field];
  if (
    summary &&
    Number.isSafeInteger(summary.count) &&
    summary.count > 0 &&
    typeof min === 'number' &&
    Number.isFinite(min) &&
    typeof max === 'number' &&
    Number.isFinite(max) &&
    typeof avg === 'number' &&
    Number.isFinite(avg) &&
    min <= avg &&
    avg <= max
  ) {
    return { value: avg, min, max, count: summary.count };
  }
  return { value: sample[field], min: sample[field], max: sample[field], count: 1 };
}
export type Process = {
  pid: number;
  name: string;
  user: string;
  cpu: number;
  memory: number;
  gpu: number | null;
  gpuMemory: number | null;
  command: string;
  status: string;
};
export type LogEntry = {
  dataSource?: 'agent';
  id: string;
  time: string;
  level: 'INFO' | 'WARN' | 'ERROR';
  source: string;
  message: string;
};

export function validAddress(value: string): boolean {
  if (!value.trim()) return true;
  try {
    const address = value.trim();
    if (address.length > 512 || /\s/.test(address)) return false;
    const url = new URL(address.includes('://') ? address : `https://${address}`);
    return (
      ['http:', 'https:'].includes(url.protocol) &&
      !!url.hostname &&
      !url.username &&
      !url.password &&
      !url.hash &&
      !url.search
    );
  } catch {
    return false;
  }
}

export function csv(rows: (string | number)[][]): string {
  return (
    '\uFEFF' +
    rows
      .map((row) =>
        row
          .map((value) => {
            const text = String(value);
            const safe = /^[=+@\-\t\r]/.test(text) ? `'${text}` : text;
            return `"${safe.replaceAll('"', '""')}"`;
          })
          .join(','),
      )
      .join('\r\n')
  );
}

export function download(name: string, content: string, type = 'text/csv;charset=utf-8') {
  const url = URL.createObjectURL(new Blob([content], { type }));
  const link = document.createElement('a');
  link.href = url;
  link.download = name;
  link.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

export function memoryLabel(mb: number) {
  return mb >= 1024 ? `${(mb / 1024).toFixed(1)} GB` : `${Number(mb.toFixed(1))} MB`;
}

export function addressLabel(address: string) {
  try {
    return new URL(address).hostname;
  } catch {
    return address;
  }
}
