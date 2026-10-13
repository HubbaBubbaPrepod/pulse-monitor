import type { Metadata } from "next";
import { builtinThemes, colorKeys, defaultOptions } from "@/lib/themes";
import { Shell } from "@/components/shell";
import "./globals.css";

export const metadata: Metadata = {
  title: "Pulse — мониторинг серверов",
  description: "Состояние, процессы и логи ваших серверов в одном интерфейсе.",
};

const themeScript = `try{const p=JSON.parse(localStorage.getItem('pulse.preferences.v2')||'null');const a=p?.appearance;const id=a?.mode==='system'?(matchMedia('(prefers-color-scheme: dark)').matches?a.darkThemeId:a.lightThemeId):p?.themeId;const t=[...${JSON.stringify(builtinThemes)},...(p?.themes||[])].find(t=>t.id===id);const r=document.documentElement;r.dataset.theme=t?.mode||(localStorage.getItem('pulse.theme.v1')==='light'?'light':'dark');if(t){for(const k of ${JSON.stringify(colorKeys)}){const v=t.colors?.[k];if(/^#[a-fA-F0-9]{6}$/.test(v))r.style.setProperty('--'+k,v)}if(Number.isFinite(t.radius)&&t.radius>=0&&t.radius<=24)r.style.setProperty('--radius',t.radius+'px');if(/^#[a-fA-F0-9]{6}$/.test(t.accent))r.style.setProperty('--accent',t.accent);const o={...${JSON.stringify(defaultOptions)},...t.options};const f={system:'Arial, sans-serif',arial:'Arial, sans-serif',segoe:'Segoe UI, sans-serif'},c={system:'Consolas, monospace',consolas:'Consolas, monospace',courier:'Courier New, monospace'};if(f[o.font])r.style.setProperty('--ui-font',f[o.font]);if(o.contentFont==='inherit'||f[o.contentFont])r.style.setProperty('--content-font',f[o.contentFont]||f[o.font]);if(c[o.codeFont])r.style.setProperty('--code-font',c[o.codeFont]);if(o.fontSize>=12&&o.fontSize<=20)r.style.setProperty('--ui-size',o.fontSize+'px');if(o.codeSize>=10&&o.codeSize<=20)r.style.setProperty('--code-size',o.codeSize+'px');r.style.setProperty('--ui-style',o.italic?'italic':'normal');if(o.contrast>=0&&o.contrast<=100)r.style.setProperty('--contrast',(30+o.contrast*.7)+'%');r.dataset.sidebar=o.transparent?'transparent':'solid';r.dataset.pointer=o.pointer?'pointer':'default'}r.dataset.reducedMotion=a?.motionMode||(p?.motion===false?'on':'system');r.lang=p?.locale==='en'?'en':'ru'}catch{document.documentElement.dataset.theme='dark'}`;

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="ru" data-theme="dark" suppressHydrationWarning>
      <head><script dangerouslySetInnerHTML={{ __html: themeScript }} /></head>
      <body><Shell>{children}</Shell></body>
    </html>
  );
}
