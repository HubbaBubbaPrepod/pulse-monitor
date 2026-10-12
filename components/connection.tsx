"use client";
import { CircleAlert, RefreshCw } from "lucide-react";
import { useMonitor } from "./provider";
export function OfflineBanner() {
  const { server, refresh, notify } = useMonitor();
  if (server.online) return null;
  return <div className="offline-banner"><CircleAlert size={20} /><div><strong>Сервер недоступен</strong><p>Демонстрация потери соединения. Текущие метрики отсутствуют.</p></div><button className="button" onClick={() => { refresh(); notify(`Демо: сервер ${server.name} остаётся недоступным`); }}><RefreshCw size={15} />Повторить</button></div>;
}
