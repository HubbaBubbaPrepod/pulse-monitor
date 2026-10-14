'use client';
import { useLocale, tr } from './locale';
import { CircleAlert, RefreshCw } from 'lucide-react';
import { useNodeMonitor } from './provider';
export function OfflineBanner() {
  useLocale();
  const { server, refresh, notify } = useNodeMonitor();
  if (server.online) return null;
  return (
    <div className="offline-banner">
      <CircleAlert size={20} />
      <div>
        <strong>{tr('Сервер недоступен')}</strong>
        <p>{tr('Агент не выходит на связь. Проверьте его службу и соединение с монитором.')}</p>
      </div>
      <button
        className="button"
        onClick={() => {
          refresh();
          notify(tr('Ожидаем подключение сервера {p0}', { p0: server.name }));
        }}
      >
        <RefreshCw size={15} />
        {tr('Повторить')}
      </button>
    </div>
  );
}
