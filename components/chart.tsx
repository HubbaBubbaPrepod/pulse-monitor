"use client";
import {useLocale,tr,trText} from "./locale";
import { useId, useRef, useState, useEffect, type KeyboardEvent } from "react";
import gsap from "gsap";
import { useGSAP } from "@gsap/react";
import { ArrowLeftRight } from "lucide-react";
import { formatDateTime, formatNumber } from '@/lib/i18n';
import { type Sample } from "@/lib/data";
import { useNodeMonitor } from "./provider";
import { Panel } from "./ui";
import { api, type Snapshot } from "@/lib/monitor";
const W = 780, H = 230, LEFT = 42, RIGHT = 14, TOP = 12, BOTTOM = 30;
const graphWidth = W - LEFT - RIGHT, graphHeight = H - TOP - BOTTOM;
type Field = Exclude<keyof Sample, 'at'>;
const ranges = [{ label: "15 мин", minutes: 15 }, { label: "1 ч", minutes: 60 }, { label: "6 ч", minutes: 360 }, { label: "24 ч", minutes: 1440 }];
function points(values: number[], max = 100) {
    return values.map((value, i) => [LEFT + graphWidth * i / Math.max(1, values.length - 1), TOP + graphHeight * (1 - Math.min(max, value) / max)]);
}
function path(values: number[], max = 100) {
    return points(values, max).map(([x, y], i) => `${i ? "L" : "M"}${x.toFixed(2)},${y.toFixed(2)}`).join(" ");
}
export function Sparkline({ values, bars = false }: {
    values: number[];
    bars?: boolean;
}) {
    useLocale();
    const id = useId();
    const max = Math.max(...values, 1) * 1.1;
    const d = values.map((v, i) => `${i ? "L" : "M"}${(i * 150 / Math.max(1, values.length - 1)).toFixed(1)},${(45 - v / max * 38).toFixed(1)}`).join(" ");
    return <svg className="sparkline" viewBox="0 0 150 50" aria-hidden="true"><defs><linearGradient id={id} x1="0" y1="0" x2="0" y2="1"><stop offset="0%" stopColor="currentColor" stopOpacity=".22"/><stop offset="100%" stopColor="currentColor" stopOpacity="0"/></linearGradient></defs>{trText(bars ? values.filter((_, i) => i % 3 === 0).map((v, i) => <rect key={i} x={i * 8} y={48 - v / max * 38} width={3} height={v / max * 38} fill="currentColor" opacity={.35 + i / 40}/>) : <><path d={`${d} L150,50 L0,50 Z`} fill={`url(#${id})`}/><path d={d} fill="none" stroke="currentColor" strokeWidth="1.6" vectorEffect="non-scaling-stroke"/></>)}</svg>;
}
export function Telemetry() {
    useLocale();
    const { snapshot } = useNodeMonitor();
    const samples = snapshot?.samples ?? [];
    return <div className="telemetry"><div><span>{tr("История нагрузки")}</span><span className="mono">{tr("1 ч")}</span></div><div className="telemetry-bars" aria-label={tr("История нагрузки CPU")}>{trText(samples.slice(0, 48).map((s, i) => <span key={i} style={{ transform: `scaleY(${Math.max(.1, s.cpu / 60)})`, opacity: .25 + s.cpu / 100 }}/>))}</div></div>;
}
export function Chart({ mode = "system", frozenSnapshot }: {
    mode?: "system" | "network" | "disk";
    frozenSnapshot?: Snapshot | null;
}) {
    useLocale();
    const { server, tick, preferences, paused: globallyPaused, snapshot: liveSnapshot, backend, streamState } = useNodeMonitor();
    const snapshot = frozenSnapshot ?? liveSnapshot;
    const paused = globallyPaused || !!frozenSnapshot;
    const displayTick = snapshot?.tick ?? tick;
    const [period, setPeriod] = useState(60);
    const [cursor, setCursor] = useState<number | null>(null);
    const [visible, setVisible] = useState([true, true]);
    const [showTable, setShowTable] = useState(false);
    const container = useRef<HTMLDivElement>(null);
    const primary = useRef<SVGPathElement>(null);
    const secondary = useRef<SVGPathElement>(null);
    const area = useRef<SVGPathElement>(null);
    const prior = useRef<[
        string,
        string
    ] | null>(null);
    const id = useId();
    const fields: [
        Field,
        Field
    ] = mode === "network" ? ["incoming", "outgoing"] : mode === "disk" ? ["read", "write"] : ["cpu", "memory"];
    const names = mode === "network" ? [tr("Приём"), tr("Отправка")] : mode === "disk" ? [tr("Чтение"), tr("Запись")] : ["CPU", tr("Память")];
    const title = mode === "network" ? tr("Сетевой трафик") : mode === "disk" ? tr("Дисковые операции") : tr("Нагрузка системы");
    const unit = mode === "system" ? "%" : " MB/s";
    const [periodData, setPeriodData] = useState<{
        serverId: string;
        period: number;
        samples: Sample[];
    } | null>(null);
    useEffect(() => {
        if (backend !== 'connected' || period === 60 || paused)
            return;
        const controller = new AbortController();
        api<Snapshot>(`/servers/${encodeURIComponent(server.id)}/snapshot?period=${period}`, { signal: controller.signal }).then(value => { if (!controller.signal.aborted)
            setPeriodData({ serverId: server.id, period, samples: value.samples }); }).catch(() => { });
        return () => controller.abort();
    }, [backend, period, server.id, displayTick, paused]);
    const samples = period === 60 ? snapshot?.samples ?? [] : periodData?.serverId === server.id && periodData.period === period ? periodData.samples : [];
    const first = samples.map(s => s[fields[0]]), second = samples.map(s => s[fields[1]]);
    const chartMax = mode === 'system' ? 100 : Math.max(1, Math.ceil(Math.max(...first, ...second, 1) / 25) * 25);
    const d1 = path(first, chartMax) || `M${LEFT},${H - BOTTOM}`, d2 = path(second, chartMax) || `M${LEFT},${H - BOTTOM}`, closing = ` L${W - RIGHT},${H - BOTTOM} L${LEFT},${H - BOTTOM} Z`;
    useGSAP(() => {
        const mm = gsap.matchMedia();
        mm.add(preferences.appearance?.motionMode === "off" ? "all" : "(prefers-reduced-motion: no-preference)", () => {
            if (!preferences.motion)
                return;
            if (prior.current) {
                gsap.fromTo(primary.current, { attr: { d: prior.current[0] } }, { attr: { d: d1 }, duration: preferences.interval === 0 ? .3 : .65, ease: "power2.out" });
                gsap.fromTo(secondary.current, { attr: { d: prior.current[1] } }, { attr: { d: d2 }, duration: preferences.interval === 0 ? .3 : .65, ease: "power2.out" });
                gsap.fromTo(area.current, { attr: { d: prior.current[0] + closing } }, { attr: { d: d1 + closing }, duration: preferences.interval === 0 ? .3 : .65, ease: "power2.out" });
            }
            else {
                const length = primary.current?.getTotalLength() ?? 1000;
                gsap.fromTo(primary.current, { strokeDasharray: length, strokeDashoffset: length }, { strokeDashoffset: 0, duration: 1.2, ease: "power2.inOut", clearProps: "strokeDasharray,strokeDashoffset" });
                gsap.from(area.current, { opacity: 0, duration: 1 });
            }
        });
        prior.current = [d1, d2];
        return () => mm.revert();
    }, { scope: container, dependencies: [d1, d2, preferences.motion, preferences.appearance?.motionMode, preferences.interval], revertOnUpdate: true });
    function timeAt(index: number) {
        if (samples[0]?.at && samples.at(-1)?.at) {
            const start = new Date(samples[0].at).getTime(), end = new Date(samples.at(-1)!.at!).getTime();
            return formatDateTime(start + (end - start) * index / 60,undefined, { hour: '2-digit', minute: '2-digit', second: '2-digit' });
        }
        const end = snapshot ? new Date(snapshot.at) : new Date();
        return formatDateTime(end.getTime() - (period - period * index / 60) * 60000,undefined, { hour: "2-digit", minute: "2-digit" });
    }
    function keyboard(e: KeyboardEvent<SVGSVGElement>) {
        if (["ArrowLeft", "ArrowRight", "Home", "End"].includes(e.key)) {
            e.preventDefault();
            setCursor(current => e.key === "Home" ? 0 : e.key === "End" ? 60 : Math.max(0, Math.min(60, (current ?? 30) + (e.key === "ArrowLeft" ? -1 : 1))));
        }
    }
    const active = cursor === null ? null : samples[Math.round(cursor / 60 * Math.max(0, samples.length - 1))];
    const cx = cursor === null ? 0 : LEFT + cursor / 60 * graphWidth;
    return <Panel className="chart-panel"><div ref={container}><div className="panel-title chart-title"><h2>{trText(title)}</h2><div className="chart-legend">{trText(names.map((name, i) => <button key={name} className={visible[i] ? "" : "series-hidden"} aria-pressed={visible[i]} onClick={() => setVisible(v => v.map((value, j) => i === j ? !value : value))}><span className={i ? "legend-dash" : "legend-line"}/>{trText(name)}</button>))}</div><div className="time-switch" aria-label={tr("Период графика")}>{trText(ranges.map(range => <button key={range.minutes} aria-pressed={period === range.minutes} className={period === range.minutes ? "selected" : ""} onClick={() => setPeriod(range.minutes)}>{trText(range.label)}</button>))}</div></div><div className="chart-live"><span className="tiny-dot"/>{trText(paused ? tr("Обновление приостановлено") : `${server.source === "agent" ? tr("Агент") : server.source === "waiting" ? tr("Ожидаем агента") : tr("Ожидание")} · ${streamState}`)}</div>
    <div className="chart-canvas"><svg viewBox={`0 0 ${W} ${H}`} role="img" aria-label={tr("{p0}. Используйте стрелки для просмотра значений. {p1} {p2}{p3}, {p4} {p5}{p6}", { p0: title, p1: names[0], p2: formatNumber(first.at(-1) ?? NaN), p3: unit, p4: names[1], p5: formatNumber(second.at(-1) ?? NaN), p6: unit })} tabIndex={0} onKeyDown={keyboard} onFocus={() => setCursor(40)} onBlur={() => setCursor(null)} onPointerLeave={() => setCursor(null)} onPointerMove={e => { const bounds = e.currentTarget.getBoundingClientRect(); const x = (e.clientX - bounds.left) / bounds.width * W; setCursor(Math.max(0, Math.min(60, Math.round((x - LEFT) / graphWidth * 60)))); }} onPointerDown={e => { const bounds = e.currentTarget.getBoundingClientRect(); setCursor(Math.max(0, Math.min(60, Math.round(((e.clientX - bounds.left) / bounds.width * W - LEFT) / graphWidth * 60)))); }}>
      <defs><linearGradient id={id} x1="0" y1="0" x2="0" y2="1"><stop offset="0" stopColor="var(--chart-primary)" stopOpacity=".18"/><stop offset="1" stopColor="var(--chart-primary)" stopOpacity=".01"/></linearGradient></defs>
      {trText([0, 25, 50, 75, 100].map(value => { const y = TOP + graphHeight * (1 - value / 100); return <g key={value}><line x1={LEFT} x2={W - RIGHT} y1={y} y2={y} className="chart-grid"/><text x={LEFT - 10} y={y + 4} textAnchor="end" className="chart-label">{trText(Math.round(value * chartMax / 100))}{trText(mode === "system" ? "%" : "")}</text></g>; }))}
      {trText([0, 10, 20, 30, 40, 50, 60].map(i => { const x = LEFT + i / 60 * graphWidth; return <g key={i}><line x1={x} x2={x} y1={TOP} y2={H - BOTTOM} className="chart-grid"/><text x={x} y={H - 7} textAnchor={i === 0 ? "start" : i === 60 ? "end" : "middle"} className="chart-label">{trText(timeAt(i))}</text></g>; }))}
      <path ref={area} d={d1 + closing} fill={`url(#${id})`} style={{ visibility: visible[0] ? "visible" : "hidden" }}/>
      <path ref={primary} d={d1} fill="none" stroke="var(--chart-primary)" strokeWidth="1.8" vectorEffect="non-scaling-stroke" style={{ visibility: visible[0] ? "visible" : "hidden" }}/>
      <path ref={secondary} d={d2} fill="none" stroke="var(--chart-secondary)" strokeDasharray="5 5" strokeWidth="1.6" vectorEffect="non-scaling-stroke" style={{ visibility: visible[1] ? "visible" : "hidden" }}/>
      {trText(active ? <g><line x1={cx} x2={cx} y1={TOP} y2={H - BOTTOM} stroke="var(--muted)" strokeDasharray="3 4"/>{trText(fields.map((field, i) => visible[i] ? <circle key={field} cx={cx} cy={TOP + graphHeight * (1 - active[field] / chartMax)} r={4} fill={i ? "var(--chart-secondary)" : "var(--chart-primary)"} stroke="var(--surface)" strokeWidth={2}/> : null))}</g> : null)}
    </svg>{trText(active ? <div className="chart-tooltip" style={{ left: `${Math.min(73, Math.max(8, cx / W * 100))}%` }}><span className="mono">{trText(timeAt(cursor!))}</span>{trText(fields.map((field, i) => <div key={field}><span>{trText(names[i])}</span><strong>{trText(formatNumber(active[field],undefined,{minimumFractionDigits:1,maximumFractionDigits:1}))}{trText(unit)}</strong></div>))}</div> : null)}</div><div className="chart-foot"><span>{trText(mode === "system" ? tr("Использование ресурсов, %") : tr("Скорость передачи, MB/s"))}</span><button className="text-button" onClick={() => setShowTable(s => !s)}><ArrowLeftRight size={12}/>{trText(showTable ? tr("Скрыть значения") : tr("Таблица значений"))}</button></div>
    {trText(showTable ? <div className="chart-data-table"><table><caption className="sr-only">{tr("Значения графика")}</caption><thead><tr><th>{tr("Время")}</th><th>{trText(names[0])}</th><th>{trText(names[1])}</th></tr></thead><tbody>{trText(samples.filter((_, i) => i % 10 === 0).map((sample, i) => <tr key={i}><td>{trText(sample.at ? formatDateTime(sample.at,undefined,{hour:'2-digit',minute:'2-digit',second:'2-digit'}) : timeAt(i * 10))}</td><td>{trText(formatNumber(sample[fields[0]],undefined,{minimumFractionDigits:1,maximumFractionDigits:1}))}{trText(unit)}</td><td>{trText(formatNumber(sample[fields[1]],undefined,{minimumFractionDigits:1,maximumFractionDigits:1}))}{trText(unit)}</td></tr>))}</tbody></table></div> : null)}</div></Panel>;
}
