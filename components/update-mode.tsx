'use client';
import { useLocale, tr, trText } from './locale';
import { Radio } from 'lucide-react';
import { useMonitor } from './provider';
import { intervalLabel } from '@/lib/monitor';
export function UpdateMode() {
  useLocale();
  const { preferences, setPreferences, paused } = useMonitor();
  return (
    <label className={`update-mode ${preferences.interval === 0 && !paused ? 'is-live' : ''}`}>
      <Radio size={14} />
      <span className="sr-only">{tr('Режим обновления')}</span>
      <select
        aria-label={tr('Режим обновления')}
        value={preferences.interval}
        onChange={(e) => setPreferences({ ...preferences, interval: Number(e.target.value) })}
      >
        {trText(
          [0, 2, 5, 10, 30].map((n) => (
            <option key={n} value={n}>
              {trText(intervalLabel(n))}
            </option>
          )),
        )}
      </select>
    </label>
  );
}
