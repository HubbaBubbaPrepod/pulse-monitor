"use client";
import {useLocale,tr,trText} from "./locale";
import {restoreServerSelection,persistServerSelection} from "@/lib/selection";
import {type Locale,resolveLocale} from '@/lib/i18n';
import { createContext, useContext, useState, useEffect, useRef, useSyncExternalStore, type ReactNode } from "react";
import { type LogEntry, type Server } from "@/lib/data";
import { applyTheme, builtinThemes, validateTheme, type ThemeDefinition } from "@/lib/themes";
import { api, apiBase, mergeLogs, type Bootstrap, type Snapshot, type LogPage } from "@/lib/monitor";
export type Preferences = {
    interval: number;
    motion: boolean;
    compact: boolean;
    locale: Locale;
    themeId: string;
    themes: ThemeDefinition[];
    appearance?: {
        mode: "system" | "dark" | "light";
        separate: boolean;
        lightThemeId: string;
        darkThemeId: string;
        motionMode: "system" | "on" | "off";
    };
};
type Store = {
    servers: Server[];
    server: Server | null;
    loading: boolean;
    tick: number;
    paused: boolean;
    preferences: Preferences;
    theme: "dark" | "light";
    notice: string;
    snapshot: Snapshot | null;
    recentLogs: LogEntry[];
    backend: 'connecting' | 'connected' | 'unavailable';
    streamState: string;
    startedAt: string;
    selectServer: (id: string) => void;
    saveServer: (server: Server) => Promise<void>;
    removeServer: (id: string) => Promise<void>;
    setPreferences: (p: Preferences) => void;
    toggleTheme: () => void;
    setPaused: (p: boolean) => void;
    refresh: () => void;
    notify: (text: string) => void;
    retryBackend: () => void;
};
const Context = createContext<Store | null>(null);
const defaults: Preferences = { interval: 5, motion: true, compact: false, locale: "ru", themeId: "dark", themes: [] };
function subscribeSystemTheme(notify: () => void) { const media = matchMedia('(prefers-color-scheme: dark)'); media.addEventListener('change', notify); return () => media.removeEventListener('change', notify); }
export function Provider({ children }: {
    children: ReactNode;
}) {
    useLocale();
    const [servers, setServers] = useState<Server[]>([]);
    const [selectedId, setSelectedId] = useState('');
    const [selectionReady, setSelectionReady] = useState(false);
    const [loading, setLoading] = useState(true);
    const [tick, setTick] = useState(0);
    const [paused, setPaused] = useState(false);
    const [preferences, setPreferences] = useState(defaults);
    const [settingsReady, setSettingsReady] = useState(false);
    const systemDark = useSyncExternalStore(subscribeSystemTheme, () => matchMedia('(prefers-color-scheme: dark)').matches, () => true);
    const automatic = preferences.appearance?.mode === 'system';
    const chosenId = automatic ? (systemDark ? preferences.appearance!.darkThemeId : preferences.appearance!.lightThemeId) : preferences.themeId;
    const activeTheme = [...builtinThemes, ...preferences.themes].find(t => t.id === chosenId) ?? builtinThemes[0];
    const theme = activeTheme.mode;
    const [notice, setNotice] = useState("");
    const [snapshot, setSnapshot] = useState<Snapshot | null>(null);
    const [recentLogs, setRecentLogs] = useState<LogEntry[]>([]);
    const [backend, setBackend] = useState<Store['backend']>('connecting');
    const [streamState, setStreamState] = useState(tr("Подключение"));
    const [startedAt, setStartedAt] = useState('');
    const [revision, setRevision] = useState(0);
    const [bootRevision, setBootRevision] = useState(0);
    const server = servers.find(s => s.id === selectedId) ?? servers[0] ?? null;
    const serverId = server?.id;
    const serverOnline = server?.online;
    const liveTick = preferences.interval === 0 && !paused ? tick : 0;
    useEffect(() => {
        const controller = new AbortController();
        const boot = setTimeout(async () => {
            try {
                const saved = JSON.parse(localStorage.getItem('pulse.preferences.v1') ?? 'null');
                if (saved && [0, 2, 5, 10, 30].includes(saved.interval))
                    setPreferences({ ...defaults, interval: saved.interval, motion: saved.motion !== false, compact: saved.compact === true });
            }
            catch { /* Browser preferences are optional. */ }
            try {
                const result = await api<Bootstrap>('/bootstrap', { signal: controller.signal });
                if (controller.signal.aborted)
                    return;
                setSelectedId(restoreServerSelection(result.servers));
                setServers(result.servers);
                setSelectionReady(true);
                setStartedAt(result.startedAt);
                setBackend('connected');
                const saved = await api<Preferences>('/preferences', { signal: controller.signal });
                if (!controller.signal.aborted) {
                    setPreferences({ ...saved, locale:resolveLocale(saved.locale), themes: saved.themes.map(validateTheme) });
                    setSettingsReady(true);
                }
            }
            catch {
                if (controller.signal.aborted)
                    return;
                setBackend('unavailable');
                setStreamState(tr("Нет соединения"));
            }
            setLoading(false);
        }, 250);
        return () => { clearTimeout(boot); controller.abort(); };
    }, [bootRevision]);
    useEffect(() => { if (selectionReady) persistServerSelection(serverId ?? ''); }, [selectionReady, serverId]);
    useEffect(() => { if (!settingsReady)
        return; applyTheme(activeTheme); document.documentElement.dataset.reducedMotion = preferences.appearance?.motionMode ?? (preferences.motion ? 'system' : 'on'); document.documentElement.lang = preferences.locale; window.dispatchEvent(new Event('pulse:locale')); try {
        localStorage.setItem('pulse.preferences.v2', JSON.stringify(preferences));
        localStorage.setItem('pulse.theme.v1', activeTheme.mode);
    }
    catch { } }, [preferences, activeTheme, settingsReady]);
    const savedSettings = useRef('');
    useEffect(() => { if (!settingsReady)
        return; const raw = JSON.stringify(preferences); if (!savedSettings.current) {
        savedSettings.current = raw;
        return;
    } if (raw === savedSettings.current)
        return; const abort = new AbortController(); const timer = setTimeout(() => { api('/preferences', { method: 'PUT', body: raw, signal: abort.signal }).then(() => { savedSettings.current = raw; }).catch(e => { if (!abort.signal.aborted)
        setNotice(e instanceof Error ? e.message : tr("Не удалось сохранить настройки")); }); }, 600); return () => { clearTimeout(timer); abort.abort(); }; }, [preferences, settingsReady]);
    useEffect(() => {
        if (backend !== 'connected')
            return;
        const controller = new AbortController();
        let busy = false;
        const timer = setInterval(async () => { if (busy || document.hidden)
            return; busy = true; try {
            const value = await api<Bootstrap>('/bootstrap', { signal: controller.signal });
            if (!controller.signal.aborted)
                setServers(value.servers);
        }
        catch { /* Retain last known status. */ }
        finally {
            busy = false;
        } }, (preferences.interval || 2) * 1000);
        return () => { clearInterval(timer); controller.abort(); };
    }, [backend, preferences.interval]);
    useEffect(() => {
        if (backend !== 'connected' || loading || !serverId)
            return;
        const controller = new AbortController();
        let stream: EventSource | undefined;
        let timer: ReturnType<typeof setTimeout> | undefined;
        let busy = false;
        async function fetchSnapshot() {
            if (busy)
                return;
            busy = true;
            try {
                if (!serverId)
                    return;
                const value = await api<Snapshot>(`/servers/${encodeURIComponent(serverId)}/snapshot`, { signal: controller.signal });
                if (!controller.signal.aborted) {
                    setSnapshot(value);
                    setTick(value.tick);
                    setStreamState(paused ? tr("Пауза просмотра") : preferences.interval === 0 ? tr("Реалтайм \u00B7 SSE") : tr("Обновление {p0} с", { p0: preferences.interval }));
                }
            }
            catch {
                if (!controller.signal.aborted)
                    setStreamState(tr("Нет связи с бэкендом \u00B7 данные устарели"));
            }
            finally {
                busy = false;
            }
        }
        void fetchSnapshot();
        if (!paused && serverOnline) {
            if (preferences.interval === 0) {
                stream = new EventSource(`${apiBase()}/api/servers/${encodeURIComponent(serverId)}/stream`, { withCredentials: true });
                stream.addEventListener('snapshot', event => {
                    if (document.hidden || controller.signal.aborted)
                        return;
                    try {
                        const value = JSON.parse((event as MessageEvent).data) as Snapshot;
                        setSnapshot(value);
                        setTick(value.tick);
                        setStreamState(tr("Реалтайм \u00B7 SSE"));
                    }
                    catch {
                        setStreamState(tr("Ошибка данных потока"));
                    }
                });
                stream.onerror = () => { if (!controller.signal.aborted)
                    setStreamState(tr("Поток переподключается \u00B7 данные устарели")); };
            }
            else {
                const poll = async () => { if (!document.hidden)
                    await fetchSnapshot(); if (!controller.signal.aborted)
                    timer = setTimeout(poll, preferences.interval * 1000); };
                timer = setTimeout(poll, preferences.interval * 1000);
            }
        }
        return () => { controller.abort(); stream?.close(); clearTimeout(timer); };
    }, [serverId, serverOnline, backend, loading, paused, preferences.interval, revision]);
    useEffect(() => {
        if (loading || !serverId)
            return;
        let cancelled = false;
        const controller = new AbortController();
        async function collectLogs() {
            if (!serverId)
                return;
            if (backend === 'connected') {
                try {
                    const page = await api<LogPage>(`/servers/${encodeURIComponent(serverId)}/logs?limit=100`, { signal: controller.signal });
                    if (!cancelled)
                        setRecentLogs(current => mergeLogs(current.filter(l => l.id.startsWith('local-') === false), page.items).slice(0, 200));
                }
                catch { /* The archive page surfaces request failures with a retry control. */ }
            }
        }
        void collectLogs();
        const timer = !paused && preferences.interval > 0 ? setInterval(collectLogs, preferences.interval * 1000) : undefined;
        return () => { cancelled = true; controller.abort(); clearInterval(timer); };
    }, [serverId, backend, loading, paused, preferences.interval, liveTick, revision]);
    useEffect(() => { if (!notice)
        return; const timer = setTimeout(() => setNotice(''), 4000); return () => clearTimeout(timer); }, [notice]);
    function selectServer(id: string) { if (id === server?.id)
        return; setSelectedId(id); setTick(0); setSnapshot(null); setRecentLogs([]); setRevision(n => n + 1); }
    function toggleTheme() { setPreferences(p => ({ ...p, themeId: theme === 'dark' ? (p.appearance?.separate ? p.appearance.lightThemeId : 'light') : (p.appearance?.separate ? p.appearance.darkThemeId : 'dark'), appearance: p.appearance ? { ...p.appearance, mode: theme === 'dark' ? 'light' : 'dark' } : undefined })); }
    async function saveServer(updated: Server) {
        if (backend !== 'connected')
            throw new Error(tr("Запустите Go-бэкенд для сохранения серверов"));
        const saved = await api<Server>(`/servers/${encodeURIComponent(updated.id)}`, { method: 'PUT', body: JSON.stringify(updated) });
        setServers(current => current.some(s => s.id === saved.id) ? current.map(s => s.id === saved.id ? saved : s) : [...current, saved]);
        setSelectedId(saved.id);
        setRevision(n => n + 1);
    }
    async function removeServer(id: string) {
        if (backend !== 'connected')
            throw new Error(tr("Запустите Go-бэкенд для удаления сервера"));
        await api(`/servers/${encodeURIComponent(id)}`, { method: 'DELETE' });
        setServers(current => current.filter(s => s.id !== id));
        if (server?.id === id)
            selectServer(servers.find(s => s.id !== id)?.id ?? '');
    }
    return <Context.Provider value={{ servers, server, loading, tick, paused, preferences, theme, notice, snapshot: snapshot?.serverId === server?.id ? snapshot : null, recentLogs, backend, streamState, startedAt, selectServer, saveServer, removeServer, setPreferences, toggleTheme, setPaused, refresh: () => { setRevision(n => n + 1); if (backend === 'unavailable')
            setTick(t => t + 1); }, notify: setNotice, retryBackend: () => setBootRevision(n => n + 1) }}>{trText(children)}</Context.Provider>;
}
export function useMonitor() { const context = useContext(Context); if (!context)
    throw new Error('Monitor components require Provider'); return context; }
export function useNodeMonitor() { const store = useMonitor(); if (!store.server)
    throw new Error("Select a server"); return { ...store, server: store.server }; }
