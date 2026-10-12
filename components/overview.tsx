"use client";

import Link from "next/link";
import { Cpu, MemoryStick, HardDrive, ArrowDownUp, Server, Download, ArrowUpRight, Pause, Play, RefreshCw } from "lucide-react";
import { OfflineBanner } from "./connection";
import { useMonitor } from "./provider";
import { AnimatedNumber, PageHeading, PageMotion, Panel, Skeleton, Status } from "./ui";
import { Chart, Sparkline, Telemetry } from "./chart";
import { ProcessTable } from "./processes";
import { LogFeed } from "./logs";
import { addressLabel, csv, download, history } from "@/lib/demo";

import { OSIcon } from "./icons";

export function Metrics() {
  const { server, tick, snapshot } = useMonitor();
  const samples = snapshot?.samples ?? history(server, tick);
  const last = samples.at(-1) ?? {cpu:0,memory:0,incoming:0,outgoing:0,read:0,write:0};
  const real = server.source==='agent';
  const ram = real ? snapshot?.hardware?.memoryGB ?? 0 : 16;
  const totalDisk = snapshot?.disks.reduce((sum,d)=>sum+d.totalGB,0) ?? 500;
  const usedDisk = snapshot?.disks.reduce((sum,d)=>sum+d.usedGB,0) ?? server.disk*5;
  const cards = [
    { label: "CPU", value: last.cpu, unit: "%", detail: real ? `${snapshot?.hardware?.cpuCores ?? '—'} ядер` : "8 ядер · 3.40 GHz", icon: Cpu, values: samples.map(s => s.cpu), delta: real?'CPU':'−12.4%' },
    { label: "Память", value: last.memory, unit: "%", detail: `${(last.memory / 100 * ram).toFixed(1)} / ${ram.toFixed(1)} GB`, icon: MemoryStick, values: samples.map(s => s.memory), delta: real?'RAM':'+2.1%' },
    { label: "Диск", value: totalDisk>0?usedDisk/totalDisk*100:0, unit: "%", detail: `${usedDisk.toFixed(1)} / ${totalDisk.toFixed(1)} GB`, icon: HardDrive, values: samples.map(s=>s.read+s.write), delta: real?'I/O':'NVMe SSD' },
    { label: "Сеть", value: last.incoming + last.outgoing, unit: "MB/s", detail: `↓ ${last.incoming.toFixed(1)} · ↑ ${last.outgoing.toFixed(1)} MB/s`, icon: ArrowDownUp, values: samples.map(s => s.incoming), delta: real?'Σ интерфейсов':'eth0' },
  ];
  return <div className="metrics-grid">{cards.map(({ label, value, unit, detail, icon: Icon, values, delta }) => <Panel key={label} className="metric-card" spotlight><div className="metric-top"><span><Icon size={19} strokeWidth={1.5} />{label}</span><span className="metric-delta">{delta}</span></div><div className="metric-main"><strong><AnimatedNumber value={value} /><span className={`metric-unit ${unit === "%" ? "percent" : ""}`}>{unit}</span></strong><Sparkline values={values.slice(-30)} bars={label === "Диск"} /></div><div className="metric-detail mono">{detail}</div></Panel>)}</div>;
}

export function ResourceCard() {
  const { server, tick, snapshot } = useMonitor();
  const memory = (snapshot?.samples ?? history(server, tick)).at(-1)?.memory ?? 0;
  const ram=server.source==='agent'?snapshot?.hardware?.memoryGB ?? 0:16;
  const cache = server.source==='agent'?0:Math.min(19.4,100-memory);
  const free = 100 - memory - cache;
  const radius = 55, circumference = 2 * Math.PI * radius;
  return <Panel className="resource-card"><div className="panel-title"><h2>Ресурсы</h2><Link href="/resources/" className="icon-button small-icon" aria-label="Подробнее о ресурсах"><ArrowUpRight size={17} /></Link></div><span className="resource-section-label">Память</span><div className="memory-overview"><div className="memory-ring"><svg viewBox="0 0 140 140" aria-label={`Использовано ${memory.toFixed(1)}% памяти`} role="img"><circle cx="70" cy="70" r={radius} fill="none" stroke="var(--track)" strokeWidth="11" /><circle className="gauge-arc" cx="70" cy="70" r={radius} fill="none" stroke="var(--chart-primary)" strokeWidth="11" strokeLinecap="round" strokeDasharray={`${circumference * memory / 100} ${circumference}`} transform="rotate(-90 70 70)" /><circle cx="70" cy="70" r={radius} fill="none" stroke="var(--chart-secondary)" strokeWidth="11" strokeDasharray={`${Math.max(0,circumference * cache / 100 - 4)} ${circumference}`} strokeDashoffset={-circumference * memory / 100 - 4} transform="rotate(-90 70 70)" /></svg><div className="ring-value"><strong><AnimatedNumber value={memory} />%</strong><span>{(memory / 100 * ram).toFixed(1)} / {ram.toFixed(1)} GB</span></div></div><div className="memory-breakdown">{[{ label: "Используется", amount: memory, tone: "primary" }, { label: "Кэш", amount: cache, tone: "secondary" }, { label: "Доступно", amount: free, tone: "tertiary" }].filter(item=>item.label!=="Кэш"||server.source!=="agent").map(item => <div key={item.label}><span><i className={`legend-dot ${item.tone}`} />{item.label}</span><strong className="mono">{(item.amount / 100 * ram).toFixed(1)} GB</strong></div>)}</div></div><div className="disk-breakdown"><span className="resource-section-label">Диск</span>{(snapshot?.disks.map(d=>({name:d.mount,percent:d.totalGB>0?d.usedGB/d.totalGB*100:0,size:`${d.totalGB.toFixed(1)} GB`})) ?? [{ name: server.os.startsWith("Windows") ? "C:" : "/", percent: server.disk, size: "500 GB" }, { name: server.os.startsWith("Windows") ? "D:" : "/data", percent: 27.6, size: "1 TB" }]).map(item => <div className="disk-row" key={item.name}><span className="mono">{item.name}</span><div className="usage-track"><span style={{ transform: `scaleX(${item.percent / 100})` }} /></div><strong className="mono">{item.percent.toFixed(1)}%</strong><small>{item.size}</small></div>)}</div></Panel>;
}

export function Overview() {
  const { server, snapshot, loading, tick, paused, setPaused, refresh, notify } = useMonitor();
  function exportMetrics() {
    const samples = snapshot?.samples ?? history(server, tick);
    download(`${server.name}-metrics.csv`, csv([["Sample", "CPU %", "Memory %", "Incoming MB/s", "Outgoing MB/s"], ...samples.map((s, i) => [i, s.cpu.toFixed(2), s.memory.toFixed(2), s.incoming.toFixed(2), s.outgoing.toFixed(2)])]));
    notify("Метрики экспортированы в CSV");
  }
  return <PageMotion><PageHeading title="Всё под контролем." subtitle="Состояние сервера в одном месте" actions={<><button className="icon-button bordered" onClick={() => setPaused(!paused)} aria-label={paused ? "Возобновить обновление" : "Приостановить обновление"}>{paused ? <Play size={17} /> : <Pause size={17} />}</button><button className="icon-button bordered" onClick={refresh} aria-label="Обновить данные"><RefreshCw size={17} /></button><button className="button" onClick={exportMetrics} disabled={loading || !server.online}><Download size={16} />Экспорт<ArrowUpRight size={13} /></button></>} />
    {loading ? <Skeleton dashboard /> : <><Panel className="server-banner"><div className="server-symbol"><OSIcon os={server.os} /></div><div className="server-identity"><strong>{server.name}</strong><span>{server.os}<i />{addressLabel(server.address)}</span></div><Status online={server.online} /><div className="uptime"><span>Аптайм</span><strong className="mono">{server.online ? server.source === "agent" ? `${Math.floor((snapshot?.hardware?.uptime ?? 0)/86400)} д. ${Math.floor((snapshot?.hardware?.uptime ?? 0)%86400/3600)} ч.` : "14 д. 8 ч." : "—"}</strong></div><Telemetry /></Panel><OfflineBanner />{server.online ? <><Metrics /><div className="chart-layout"><Chart /><ResourceCard /></div><div className="bottom-grid"><Panel className="process-preview"><div className="panel-title"><h2>Активные процессы</h2><Link href="/processes/" className="text-button">Все процессы<ArrowUpRight size={14} /></Link></div><ProcessTable preview /></Panel><Panel className="log-preview"><div className="panel-title"><h2>Последние логи</h2><Link href="/logs/" className="text-button">Все логи<ArrowUpRight size={14} /></Link></div><LogFeed preview /></Panel></div></> : <div className="offline-placeholder"><Server size={44} strokeWidth={1} /><h2>Ожидаем соединение</h2><p>Выберите доступный сервер, чтобы посмотреть графики и процессы.</p></div>}</>}
  </PageMotion>;
}
