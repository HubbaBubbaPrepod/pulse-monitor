"use client";

import { createContext, useContext, useState, useEffect, useRef, useEffectEvent, type ReactNode } from "react";
import { initialServers, logs, type LogEntry, type Server } from "@/lib/demo";
import { api, apiBase, mergeLogs, type Bootstrap, type Snapshot, type LogPage } from "@/lib/monitor";

type Preferences = { interval: number; motion: boolean; compact: boolean };
type Store = {
 servers: Server[]; server: Server; loading: boolean; tick: number; paused: boolean;
 preferences: Preferences; theme: "dark" | "light"; notice: string;
 snapshot: Snapshot | null; recentLogs: LogEntry[]; backend: 'connecting' | 'connected' | 'unavailable'; streamState: string; startedAt: string;
 selectServer: (id: string) => void; saveServer: (server: Server) => Promise<void>; removeServer: (id: string) => Promise<void>;
 setPreferences: (p: Preferences) => void; toggleTheme: () => void; setPaused: (p: boolean) => void;
 refresh: () => void; notify: (text: string) => void; retryBackend: () => void;
};
const Context = createContext<Store | null>(null);
const defaults: Preferences = { interval: 5, motion: true, compact: false };

export function Provider({ children }: { children: ReactNode }) {
 const [servers, setServers] = useState(initialServers);
 const [selectedId, setSelectedId] = useState(initialServers[0].id);
 const [loading, setLoading] = useState(true);
 const [tick, setTick] = useState(0);
 const [paused, setPaused] = useState(false);
 const [preferences, setPreferences] = useState(defaults);
 const [theme, setTheme] = useState<"dark" | "light">("dark");
 const [notice, setNotice] = useState("");
 const [snapshot, setSnapshot] = useState<Snapshot | null>(null);
 const [recentLogs, setRecentLogs] = useState<LogEntry[]>([]);
 const [backend, setBackend] = useState<Store['backend']>('connecting');
 const [streamState, setStreamState] = useState('Подключение');
 const [startedAt, setStartedAt] = useState('');
 const [revision, setRevision] = useState(0);
 const [bootRevision, setBootRevision] = useState(0);
 const ready = useRef(false);
 const server = servers.find(s => s.id === selectedId) ?? servers[0];

 useEffect(() => {
  const controller = new AbortController();
  const boot = setTimeout(async () => {
   try {
    const saved = JSON.parse(localStorage.getItem('pulse.preferences.v1') ?? 'null');
    if (saved && [0,2,5,10,30].includes(saved.interval)) setPreferences({ interval:saved.interval, motion:saved.motion !== false, compact:saved.compact === true });
   } catch { /* Browser preferences are optional. */ }
   setTheme(document.documentElement.dataset.theme === 'light' ? 'light' : 'dark');
   try {
    const result = await api<Bootstrap>('/bootstrap', { signal:controller.signal });
    if (controller.signal.aborted) return;
    if (!result.servers.length) throw new Error('Empty server registry');
    setServers(result.servers); setStartedAt(result.startedAt); setBackend('connected');
   } catch {
    if (controller.signal.aborted) return;
    setBackend('unavailable'); setStreamState('Локальная демонстрация');
   }
   ready.current = true; setLoading(false);
  }, 250);
  return () => { clearTimeout(boot); controller.abort(); };
 }, [bootRevision]);

 useEffect(() => {
  if (!ready.current) return;
  try { localStorage.setItem('pulse.preferences.v1', JSON.stringify(preferences)); } catch { /* Optional storage. */ }
 }, [preferences, loading]);

 useEffect(() => {
  if (backend !== 'connected') return;
  const controller=new AbortController();let busy=false;
  const timer=setInterval(async()=>{if(busy||document.hidden)return;busy=true;try {const value=await api<Bootstrap>('/bootstrap',{signal:controller.signal});if(!controller.signal.aborted)setServers(value.servers);}catch{/* Retain last known status. */}finally{busy=false;}},2000);
  return()=>{clearInterval(timer);controller.abort();};
 },[backend]);

 useEffect(() => {
  if (backend !== 'connected' || loading) return;
  const controller = new AbortController(); let stream: EventSource | undefined;
  let timer: ReturnType<typeof setTimeout> | undefined;
  let busy = false;
  async function fetchSnapshot() {
   if (busy) return; busy = true;
   try {
    const value = await api<Snapshot>(`/servers/${encodeURIComponent(server.id)}/snapshot`, { signal:controller.signal });
    if (!controller.signal.aborted) { setSnapshot(value); setTick(value.tick); setStreamState(paused ? 'Пауза просмотра' : preferences.interval === 0 ? 'Реалтайм · SSE' : `Обновление ${preferences.interval} с`); }
   } catch { if (!controller.signal.aborted) setStreamState('Нет связи с бэкендом · данные устарели'); }
   finally { busy = false; }
  }
  void fetchSnapshot();
  if (!paused && server.online) {
   if (preferences.interval === 0) {
    stream = new EventSource(`${apiBase()}/api/servers/${encodeURIComponent(server.id)}/stream`, {withCredentials:true});
    stream.addEventListener('snapshot', event => {
     if (document.hidden || controller.signal.aborted) return;
     try { const value = JSON.parse((event as MessageEvent).data) as Snapshot; setSnapshot(value); setTick(value.tick); setStreamState('Реалтайм · SSE'); }
     catch { setStreamState('Ошибка данных потока'); }
    });
    stream.onerror = () => { if (!controller.signal.aborted) setStreamState('Поток переподключается · данные устарели'); };
   } else {
    const poll = async () => { if (!document.hidden) await fetchSnapshot(); if (!controller.signal.aborted) timer = setTimeout(poll, preferences.interval*1000); };
    timer = setTimeout(poll, preferences.interval*1000);
   }
  }
  return () => { controller.abort(); stream?.close(); clearTimeout(timer); };
 }, [server.id, server.online, backend, loading, paused, preferences.interval, revision]);

 useEffect(() => {
  if (backend !== 'unavailable' || loading || paused || !server.online || server.source==='agent' || server.source==='waiting') return;
  const timer = setInterval(() => { if (!document.hidden) setTick(t=>t+1); }, preferences.interval === 0 ? 500 : preferences.interval*1000);
  return () => clearInterval(timer);
 }, [backend, loading, paused, server.online, server.source, preferences.interval]);

 const localLog = useEffectEvent(() => logs(server,tick).slice(0,1).map(l=>({ ...l,id:`local-${server.id}-${Date.now()}`,time:new Date().toISOString() })));
 useEffect(() => {
  if (loading) return;
  let cancelled = false; const controller = new AbortController();
  async function collectLogs() {
   if (backend === 'connected') {
    try { const page = await api<LogPage>(`/servers/${encodeURIComponent(server.id)}/logs?limit=100`, { signal:controller.signal }); if (!cancelled) setRecentLogs(current=>mergeLogs(current.filter(l=>l.id.startsWith('local-') === false),page.items).slice(0,200)); }
    catch { /* The archive page surfaces request failures with a retry control. */ }
   } else if (backend === 'unavailable' && server.source!=='agent' && server.source!=='waiting') {
    const entries = localLog();
    if (!cancelled) setRecentLogs(current=>mergeLogs(current,entries).slice(0,200));
   }
  }
  void collectLogs(); const timer = setInterval(collectLogs,5000);
  return () => { cancelled = true; controller.abort(); clearInterval(timer); };
 }, [server.id, server.source, backend, loading]);

 useEffect(() => { if (!notice) return; const timer=setTimeout(()=>setNotice(''),4000); return ()=>clearTimeout(timer); }, [notice]);

 function selectServer(id: string) { if (id === server.id) return; setSelectedId(id); setTick(0); setSnapshot(null); setRecentLogs([]); setRevision(n=>n+1); }
 function toggleTheme() { const next=theme==='dark'?'light':'dark'; document.documentElement.dataset.theme=next; setTheme(next); try {localStorage.setItem('pulse.theme.v1',next);} catch {} }
 async function saveServer(updated: Server) {
  if (backend !== 'connected') throw new Error('Запустите Go-бэкенд для сохранения серверов');
  const saved=await api<Server>(`/servers/${encodeURIComponent(updated.id)}`,{method:'PUT',body:JSON.stringify(updated)});
  setServers(current=>current.some(s=>s.id===saved.id)?current.map(s=>s.id===saved.id?saved:s):[...current,saved]); setRevision(n=>n+1);
 }
 async function removeServer(id: string) {
  if (servers.length<=1) return;
  if (backend !== 'connected') throw new Error('Запустите Go-бэкенд для удаления сервера');
  await api(`/servers/${encodeURIComponent(id)}`,{method:'DELETE'});
  setServers(current=>current.filter(s=>s.id!==id)); if (server.id===id) selectServer(servers.find(s=>s.id!==id)!.id);
 }
 return <Context.Provider value={{ servers,server,loading,tick,paused,preferences,theme,notice,snapshot:snapshot?.serverId===server.id?snapshot:null,recentLogs,backend,streamState,startedAt,selectServer,saveServer,removeServer,setPreferences,toggleTheme,setPaused,refresh:()=>{setRevision(n=>n+1);if(backend==='unavailable')setTick(t=>t+1);},notify:setNotice,retryBackend:()=>setBootRevision(n=>n+1) }}>{children}</Context.Provider>;
}
export function useMonitor() { const context=useContext(Context);if(!context)throw new Error('Monitor components require Provider');return context; }
