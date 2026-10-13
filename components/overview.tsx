"use client";
import {useLocale,tr,trText} from "./locale";
import {formatNumber} from '@/lib/i18n';
import Link from "next/link";
import { Cpu, MemoryStick, HardDrive, ArrowDownUp, Server, Download, ArrowUpRight, Pause, Play, RefreshCw } from "lucide-react";
import { OfflineBanner } from "./connection";
import { useNodeMonitor } from "./provider";
import { AnimatedNumber, PageHeading, PageMotion, Panel, Skeleton, Status } from "./ui";
import { Chart, Sparkline, Telemetry } from "./chart";
import { ProcessTable } from "./processes";
import { LogFeed } from "./logs";
import { addressLabel, csv, download } from "@/lib/data";
import { OSIcon } from "./icons";
export function Metrics() {
    useLocale();
    const { server, snapshot } = useNodeMonitor();
    const samples = snapshot?.samples ?? [];
    const last = samples.at(-1) ?? { cpu: 0, memory: 0, incoming: 0, outgoing: 0, read: 0, write: 0 };
    const real = server.source === 'agent';
    const ram = snapshot?.hardware?.memoryGB ?? 0;
    const totalDisk = snapshot?.disks.reduce((sum, d) => sum + d.totalGB, 0) ?? 0;
    const usedDisk = snapshot?.disks.reduce((sum, d) => sum + d.usedGB, 0) ?? 0;
    const cards = [
        { label: "CPU", value: last.cpu, unit: "%", detail: real ? tr("{p0} ядер", { p0: snapshot?.hardware?.cpuCores ?? '—' }) : "—", icon: Cpu, values: samples.map(s => s.cpu), delta: 'CPU' },
        { label: tr("Память"), value: last.memory, unit: "%", detail: `${formatNumber((last.memory / 100 * ram),undefined,{minimumFractionDigits:1,maximumFractionDigits:1})} / ${formatNumber(ram,undefined,{minimumFractionDigits:1,maximumFractionDigits:1})} GB`, icon: MemoryStick, values: samples.map(s => s.memory), delta: 'RAM' },
        { label: tr("Диск"), value: totalDisk > 0 ? usedDisk / totalDisk * 100 : 0, unit: "%", detail: `${formatNumber(usedDisk,undefined,{minimumFractionDigits:1,maximumFractionDigits:1})} / ${formatNumber(totalDisk,undefined,{minimumFractionDigits:1,maximumFractionDigits:1})} GB`, icon: HardDrive, values: samples.map(s => s.read + s.write), delta: 'I/O' },
        { label: tr("Сеть"), value: last.incoming + last.outgoing, unit: "MB/s", detail: `↓ ${formatNumber(last.incoming,undefined,{minimumFractionDigits:1,maximumFractionDigits:1})} · ↑ ${formatNumber(last.outgoing,undefined,{minimumFractionDigits:1,maximumFractionDigits:1})} MB/s`, icon: ArrowDownUp, values: samples.map(s => s.incoming), delta: tr("\u03A3 интерфейсов") },
    ];
    if (!snapshot?.samples.length) return <Panel><p className="muted">{tr("Ожидаем данные агента")}</p></Panel>;
    return <div className="metrics-grid">{trText(cards.map(({ label, value, unit, detail, icon: Icon, values, delta }) => <Panel key={label} className="metric-card" spotlight><div className="metric-top"><span><Icon size={19} strokeWidth={1.5}/>{trText(label)}</span><span className="metric-delta">{trText(delta)}</span></div><div className="metric-main"><strong><AnimatedNumber value={value}/><span className={`metric-unit ${unit === "%" ? "percent" : ""}`}>{trText(unit)}</span></strong><Sparkline values={values.slice(-30)} bars={label === tr("Диск")}/></div><div className="metric-detail mono">{trText(detail)}</div></Panel>))}</div>;
}
export function ResourceCard() {
    useLocale();
    const { server, snapshot } = useNodeMonitor();
    const memory = (snapshot?.samples ?? []).at(-1)?.memory ?? 0;
    const ram = snapshot?.hardware?.memoryGB ?? 0;
    const cache = 0;
    const free = 100 - memory - cache;
    const radius = 55, circumference = 2 * Math.PI * radius;
    if (!snapshot?.samples.length || !snapshot.hardware) return <Panel className="resource-card"><h2>{tr("Ресурсы")}</h2><p className="muted">{tr("Ожидаем данные агента")}</p></Panel>;
    return <Panel className="resource-card"><div className="panel-title"><h2>{tr("Ресурсы")}</h2><Link href="/resources/" className="icon-button small-icon" aria-label={tr("Подробнее о ресурсах")}><ArrowUpRight size={17}/></Link></div><span className="resource-section-label">{tr("Память")}</span><div className="memory-overview"><div className="memory-ring"><svg viewBox="0 0 140 140" aria-label={tr("Использовано {p0}% памяти", { p0: formatNumber(memory,undefined,{minimumFractionDigits:1,maximumFractionDigits:1}) })} role="img"><circle cx="70" cy="70" r={radius} fill="none" stroke="var(--track)" strokeWidth="11"/><circle className="gauge-arc" cx="70" cy="70" r={radius} fill="none" stroke="var(--chart-primary)" strokeWidth="11" strokeLinecap="round" strokeDasharray={`${circumference * memory / 100} ${circumference}`} transform="rotate(-90 70 70)"/><circle cx="70" cy="70" r={radius} fill="none" stroke="var(--chart-secondary)" strokeWidth="11" strokeDasharray={`${Math.max(0, circumference * cache / 100 - 4)} ${circumference}`} strokeDashoffset={-circumference * memory / 100 - 4} transform="rotate(-90 70 70)"/></svg><div className="ring-value"><strong><AnimatedNumber value={memory}/>%</strong><span>{trText(formatNumber((memory / 100 * ram),undefined,{minimumFractionDigits:1,maximumFractionDigits:1}))} / {trText(formatNumber(ram,undefined,{minimumFractionDigits:1,maximumFractionDigits:1}))} GB</span></div></div><div className="memory-breakdown">{trText([{ label: tr("Используется"), amount: memory, tone: "primary" }, { label: tr("Кэш"), amount: cache, tone: "secondary" }, { label: tr("Доступно"), amount: free, tone: "tertiary" }].filter(item => item.label !== tr("Кэш") || server.source !== "agent").map(item => <div key={item.label}><span><i className={`legend-dot ${item.tone}`}/>{trText(item.label)}</span><strong className="mono">{trText(formatNumber((item.amount / 100 * ram),undefined,{minimumFractionDigits:1,maximumFractionDigits:1}))} GB</strong></div>))}</div></div><div className="disk-breakdown"><span className="resource-section-label">{tr("Диск")}</span>{trText((snapshot?.disks.map(d => ({ name: d.mount, percent: d.totalGB > 0 ? d.usedGB / d.totalGB * 100 : 0, size: `${formatNumber(d.totalGB,undefined,{minimumFractionDigits:1,maximumFractionDigits:1})} GB` })) ?? []).map(item => <div className="disk-row" key={item.name}><span className="mono">{item.name}</span><div className="usage-track"><span style={{ transform: `scaleX(${item.percent / 100})` }}/></div><strong className="mono">{trText(formatNumber(item.percent,undefined,{minimumFractionDigits:1,maximumFractionDigits:1}))}%</strong><small>{trText(item.size)}</small></div>))}</div></Panel>;
}
export function Overview() {
    useLocale();
    const { server, snapshot, loading, paused, setPaused, refresh, notify } = useNodeMonitor();
    function exportMetrics() {
        const samples = snapshot?.samples ?? [];
        download(`${server.name}-metrics.csv`, csv([["Sample", "CPU %", "Memory %", "Incoming MB/s", "Outgoing MB/s"], ...samples.map((s, i) => [i, s.cpu.toFixed(2), s.memory.toFixed(2), s.incoming.toFixed(2), s.outgoing.toFixed(2)])]));
        notify(tr("Метрики экспортированы в CSV"));
    }
    return <PageMotion><PageHeading title={tr("Всё под контролем.")} subtitle={tr("Состояние сервера в одном месте")} actions={<><button className="icon-button bordered" onClick={() => setPaused(!paused)} aria-label={paused ? tr("Возобновить обновление") : tr("Приостановить обновление")}>{trText(paused ? <Play size={17}/> : <Pause size={17}/>)}</button><button className="icon-button bordered" onClick={refresh} aria-label={tr("Обновить данные")}><RefreshCw size={17}/></button><button className="button" onClick={exportMetrics} disabled={loading || !server.online}><Download size={16}/>{tr("Экспорт")}<ArrowUpRight size={13}/></button></>}/>
    {trText(loading ? <Skeleton dashboard/> : <><Panel className="server-banner"><div className="server-symbol"><OSIcon os={server.os}/></div><div className="server-identity"><strong>{server.name}</strong><span>{server.os}<i />{trText(addressLabel(server.address))}</span></div><Status online={server.online}/><div className="uptime"><span>{tr("Аптайм")}</span><strong className="mono">{trText(server.online ? server.source === "agent" ? tr("{p0} д. {p1} ч.", { p0: Math.floor((snapshot?.hardware?.uptime ?? 0) / 86400), p1: Math.floor((snapshot?.hardware?.uptime ?? 0) % 86400 / 3600) }) : "—" : "—")}</strong></div><Telemetry /></Panel><OfflineBanner />{trText(server.online ? <><Metrics /><div className="chart-layout"><Chart /><ResourceCard /></div><div className="bottom-grid"><Panel className="process-preview"><div className="panel-title"><h2>{tr("Активные процессы")}</h2><Link href="/processes/" className="text-button">{tr("Все процессы")}<ArrowUpRight size={14}/></Link></div><ProcessTable preview/></Panel><Panel className="log-preview"><div className="panel-title"><h2>{tr("Последние логи")}</h2><Link href="/logs/" className="text-button">{tr("Все логи")}<ArrowUpRight size={14}/></Link></div><LogFeed preview/></Panel></div></> : <div className="offline-placeholder"><Server size={44} strokeWidth={1}/><h2>{tr("Ожидаем соединение")}</h2><p>{tr("Выберите доступный сервер, чтобы посмотреть графики и процессы.")}</p></div>)}</>)}
  </PageMotion>;
}
