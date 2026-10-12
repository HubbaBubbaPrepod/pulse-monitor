"use client";
import { useEffect, useState } from "react";
import { Bell, Check, CircleAlert, Info } from "lucide-react";
import { useMonitor } from "./provider";
import { PageHeading, PageMotion, Panel } from "./ui";
import { api, logTime, type MonitorEvent } from "@/lib/monitor";
export function Events() {
 const {servers,notify,backend}=useMonitor();
 const [rows,setRows]=useState<MonitorEvent[]>([]),[filter,setFilter]=useState('all'),[error,setError]=useState('');
 useEffect(()=>{if(backend!=='connected')return;const controller=new AbortController();let busy=false;
 const load=async()=>{if(busy)return;busy=true;try{const items=await api<MonitorEvent[]>('/events',{signal:controller.signal});if(!controller.signal.aborted){setRows(items);setError('');}}catch(e){if(!controller.signal.aborted)setError(e instanceof Error?e.message:'Не удалось загрузить события');}finally{busy=false;}};
 void load();const timer=setInterval(load,5000);return()=>{controller.abort();clearInterval(timer);};},[backend]);
 async function acknowledge(id:string){try{await api(`/events/${id}/ack`,{method:'POST',body:'{}'});setRows(current=>current.map(row=>id==='all'||row.id===id?{...row,acknowledged:true}:row));notify('События отмечены просмотренными');}catch(e){setError(e instanceof Error?e.message:'Не удалось сохранить');}}
 const visible=rows.filter(r=>filter==='all'||!r.acknowledged);
 return <PageMotion><PageHeading title="Ничего не упустить." subtitle="События агентов и изменения состояния · сохраняются на диске" actions={<button className="button" disabled={!rows.some(r=>!r.acknowledged)} onClick={()=>void acknowledge('all')}><Check size={16}/>Отметить всё</button>}/><Panel><div className="table-toolbar"><div className="segmented">{[{value:'all',label:'Все события'},{value:'new',label:'Непросмотренные'}].map(item=><button className={filter===item.value?'active':''} key={item.value} onClick={()=>setFilter(item.value)}>{item.label}</button>)}</div><span className="muted">Последние 500 событий</span></div>{error?<p className="form-error" role="alert">{error}</p>:null}<div className="event-list">{visible.map(row=><div className={`event-row ${row.acknowledged?'acknowledged':''}`} key={row.id}><div className="event-icon">{row.level==='INFO'?<Info size={21}/>:<CircleAlert size={21}/>}</div><div><div className="event-heading"><h3>{row.title}</h3><span className="tag mono">{row.level}</span></div><p>{row.text}</p><small className="mono">{servers.find(s=>s.id===row.serverId)?.name??row.serverId} · {logTime(row.time)}</small></div><button className="icon-button bordered" disabled={row.acknowledged} aria-label={`Отметить: ${row.title}`} onClick={()=>void acknowledge(row.id)}><Check size={17}/></button></div>)}</div>{!visible.length?<div className="empty-state"><Bell size={28}/><h3>Новых событий нет</h3><p>Подключения, превышения порога и потеря связи с агентами появятся здесь.</p></div>:null}</Panel></PageMotion>;
}
