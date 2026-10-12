"use client";

import { useState, useEffect } from "react";
import { Pause, Play, Download, TriangleAlert, Copy } from "lucide-react";
import { download, type LogEntry } from "@/lib/demo";
import { useMonitor } from "./provider";
import { Empty, PageHeading, PageMotion, Panel, SearchInput, Skeleton } from "./ui";
import { OfflineBanner } from "./connection";

import { api, logTime, type LogPage } from "@/lib/monitor";

export function LogFeed({ preview = false, entries }: { preview?: boolean; entries?: LogEntry[] }) {
  const { recentLogs, notify } = useMonitor();
  const rows = entries ?? recentLogs.slice(0, preview ? 7 : 100);
  if (!rows.length) return <Empty title="Логи не найдены" text="Проверьте фильтры и доступность сервера." />;
  return <div className={`log-feed ${preview ? "log-feed-preview" : ""}`}>{rows.map(entry => <div className={`log-line level-${entry.level.toLowerCase()}`} key={entry.id}><time className="mono">{logTime(entry.time)}</time><span className={`log-level ${entry.level === "INFO" ? "" : "log-level-warning"}`}>{entry.level !== "INFO" ? <TriangleAlert size={10} /> : null}{entry.level}</span><span className="log-source mono">{entry.dataSource === "demo" ? "демо / " : ""}{entry.source}</span><span className="log-message mono" title={entry.message}>{entry.message}</span>{!preview ? <button className="icon-button row-action" aria-label={`Копировать лог ${entry.time}`} onClick={async () => { try { await navigator.clipboard.writeText(`${entry.time} ${entry.level} ${entry.source} ${entry.message}`); notify("Строка лога скопирована"); } catch { notify("Копирование недоступно. Выделите текст вручную."); } }}><Copy size={14} /></button> : null}</div>)}</div>;
}

export function Logs() {
 const { server, loading, notify, recentLogs, backend, startedAt } = useMonitor();
 const [query,setQuery] = useState('');
 const [level,setLevel] = useState('ALL');
 const [source,setSource] = useState('ALL');
 const [offset,setOffset] = useState(0);
 const [before,setBefore] = useState('');
 const [revision,setRevision] = useState(0);
 const [result,setResult] = useState<{key:string; page:LogPage; error:string}|null>(null);
 const limit = 100;
 const key = JSON.stringify([server.id,query,level,source,offset,before]);
 useEffect(()=>{
  if (backend !== 'connected' || loading) return;
  const controller = new AbortController();
  const params = new URLSearchParams({limit:String(limit),offset:String(offset)});
  if(query)params.set('q',query); if(level!=='ALL')params.set('level',level); if(source!=='ALL')params.set('source',source); if(before)params.set('before',before);
  const load = async()=>{try {const page = await api<LogPage>(`/servers/${encodeURIComponent(server.id)}/logs?${params}`,{signal:controller.signal});if(!controller.signal.aborted)setResult({key,page,error:''});}catch(e){if(!controller.signal.aborted)setResult({key,page:{items:[],total:0},error:e instanceof Error?e.message:'Ошибка архива'});}};
  const delay = setTimeout(()=>void load(),200);
  const timer = !before ? setInterval(()=>void load(),5000) : undefined;
  return ()=>{controller.abort();clearTimeout(delay);clearInterval(timer);};
 },[backend,loading,server.id,query,level,source,offset,before,key,revision]);
 const page = result?.key===key ? result.page : null;
 const fallback = recentLogs.filter(l=>(level==='ALL'||l.level===level)&&(source==='ALL'||l.source===source)&&`${l.source} ${l.message} ${l.time}`.toLowerCase().includes(query.toLowerCase()));
 const rows = backend==='connected' ? page?.items ?? [] : fallback;
 const total = backend==='connected' ? page?.total ?? 0 : fallback.length;
 const sources = [...new Set([...recentLogs,...rows].map(l=>l.source))];
 const reset = ()=>{setOffset(0);};
 function navigate(next:number) { if(!before)setBefore(new Date().toISOString());setOffset(next); }
 return <PageMotion><PageHeading title="Весь архив логов." subtitle="История с первого запуска. Новые записи появляются сверху." actions={<><button className="button" onClick={()=>{setBefore(before?'':new Date().toISOString());setOffset(0);}}>{before?<Play size={16}/>:<Pause size={16}/>} {before?'К живому потоку':'Пауза просмотра'}</button><button className="button" disabled={!rows.length} onClick={()=>{download(`${server.name}-logs-${offset}.txt`,rows.map(l=>`${l.time} [${l.level}] ${l.source}: ${l.message}`).join('\n'),'text/plain;charset=utf-8');notify('Страница архива экспортирована');}}><Download size={16}/>Экспорт страницы</button></>}/><OfflineBanner/>{loading?<Skeleton/>:<Panel className="logs-panel"><div className="logs-toolbar"><SearchInput label="Поиск по всему архиву…" value={query} onChange={value=>{setQuery(value);reset();}}/><select aria-label="Уровень логов" value={level} onChange={e=>{setLevel(e.target.value);reset();}}><option value="ALL">Все уровни</option><option>INFO</option><option>WARN</option><option>ERROR</option></select><select aria-label="Источник логов" value={source} onChange={e=>{setSource(e.target.value);reset();}}><option value="ALL">Все источники</option>{[...new Set([...sources,...(source==='ALL'?[]:[source])])].map(s=><option key={s}>{s}</option>)}</select></div><div className="terminal-header"><span><span className="tiny-dot"/>{backend==='connected'?before?'Архив зафиксирован':'Сохраняется на диске':'Локальное демо · без архивного хранения'}</span><span className="mono">{total} записей</span></div>{result?.key===key && result.error?<div className="empty"><h3>Не удалось прочитать архив</h3><p role="alert">{result.error}</p><button className="button" onClick={()=>setRevision(n=>n+1)}>Повторить</button></div>:backend==='connected'&&!page?<div className="archive-loading" role="status">Загрузка архива…</div>:<LogFeed entries={rows}/>}<div className="archive-pagination"><span className="mono">{total?offset+1:0}–{Math.min(offset+limit,total)} из {total}</span><div className="button-row"><button className="button" disabled={!offset} onClick={()=>setOffset(Math.max(0,offset-limit))}>Новее</button><button className="button" disabled={offset+limit>=total} onClick={()=>navigate(offset+limit)}>Раньше</button><button className="button" disabled={offset+limit>=total} onClick={()=>navigate(Math.floor((total-1)/limit)*limit)}>К первым записям</button></div></div><div className="table-foot">{backend==='connected'?`Сбор продолжается при паузе и закрытой вкладке. Текущий запуск: ${logTime(startedAt)}. Архив прошлых запусков сохраняется.`:'Для постоянного хранения логов запустите Go-бэкенд.'}</div></Panel>}</PageMotion>;
}
