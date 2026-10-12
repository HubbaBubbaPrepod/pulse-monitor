"use client";
import { Radio } from 'lucide-react';
import { useMonitor } from './provider';
import { intervalLabel } from '@/lib/monitor';

export function UpdateMode() {
 const { preferences,setPreferences,paused }=useMonitor();
 return <label className={`update-mode ${preferences.interval===0&&!paused?'is-live':''}`}><Radio size={14}/><span className="sr-only">Режим обновления</span><select aria-label="Режим обновления" value={preferences.interval} onChange={e=>setPreferences({...preferences,interval:Number(e.target.value)})}>{[0,2,5,10,30].map(n=><option key={n} value={n}>{intervalLabel(n)}</option>)}</select></label>;
}
