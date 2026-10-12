"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useRef, useState, type ReactNode } from "react";
import { Activity, LayoutDashboard, Server, ListTree, ChartPie, ScrollText, Bell, Settings2, Sun, Moon, ChevronDown, ChevronsLeftRight, Menu, Network, Search, ArrowRight, Check, LogOut } from "lucide-react";
import { Provider, useMonitor } from "./provider";
import { Modal, SearchInput, Toast } from "./ui";
import { OSIcon } from "./icons";
import { UpdateMode } from "./update-mode";
import { addressLabel } from "@/lib/demo";
import { userInitials } from "@/lib/monitor";

import { AuthProvider, SessionGate, useAuth } from "./auth";

const navigation = [
  { href: "/", label: "Обзор", icon: LayoutDashboard },
  { href: "/servers/", label: "Серверы", icon: Server },
  { href: "/processes/", label: "Процессы", icon: ListTree },
  { href: "/network/", label: "Сеть", icon: Network },
  { href: "/resources/", label: "Ресурсы", icon: ChartPie },
  { href: "/logs/", label: "Логи", icon: ScrollText },
  { href: "/events/", label: "События", icon: Bell },
];

export function Shell({ children }: { children: ReactNode }) {
  return <AuthProvider><SessionShell>{children}</SessionShell></AuthProvider>;
}

function SessionShell({children}:{children:ReactNode}) {
 const pathname=usePathname();
 if(['/login','/login/','/register','/register/'].includes(pathname)) return children;
 return <SessionGate><Provider><Chrome>{children}</Chrome></Provider></SessionGate>;
}

function Chrome({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const {status,logout}=useAuth();
  const [logoutError,setLogoutError]=useState("");
  const initials=userInitials(status?.user?.name);
  const { server, servers, theme, toggleTheme, selectServer, preferences, backend, streamState, retryBackend } = useMonitor();
  const [mobileOpen, setMobileOpen] = useState(false);
  const [searchOpen, setSearchOpen] = useState(false);
  const [query, setQuery] = useState("");
  const picker = useRef<HTMLDetailsElement>(null);
  const activePath = pathname === "/" ? "/" : `${pathname.replace(/\/$/, "")}/`;
  const section = navigation.find(n => n.href === activePath)?.label ?? "Настройки";

  useEffect(() => {
    function shortcut(e: KeyboardEvent) {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "k") { e.preventDefault(); setSearchOpen(value => !value); }
      if (e.key === "Escape") { setMobileOpen(false); if (picker.current) picker.current.open = false; }
    }
    function outside(e: MouseEvent) { if (picker.current && !picker.current.contains(e.target as Node)) picker.current.open = false; }
    document.addEventListener("keydown", shortcut);
    document.addEventListener("click", outside);
    return () => { document.removeEventListener("keydown", shortcut); document.removeEventListener("click", outside); };
  }, []);

  return <div className={`app-shell ${preferences.compact ? "density-compact" : ""} ${preferences.motion ? "" : "motion-off"}`}>
    <a href="#main" className="skip-link">Перейти к содержимому</a>
    {mobileOpen ? <button className="nav-scrim" aria-label="Закрыть навигацию" onClick={() => setMobileOpen(false)} /> : null}
    <aside className={`sidebar ${mobileOpen ? "sidebar-open" : ""}`}>
      <Link href="/" className="brand" aria-label="Pulse — обзор"><Activity size={33} strokeWidth={1.7} /><span>PULSE</span><span className="brand-version">/ 01</span></Link>
      <div className="nav-caption">РАБОЧЕЕ ПРОСТРАНСТВО</div>
      <nav aria-label="Основная навигация">{navigation.map(({ href, label, icon: Icon }) => <Link key={href} href={href} className={`nav-link ${activePath === href ? "nav-active" : ""}`} aria-current={activePath === href ? "page" : undefined} onClick={() => setMobileOpen(false)}><Icon size={19} strokeWidth={1.6} /><span>{label}</span>{href === "/servers/" ? <span className="nav-count">{servers.length}</span> : null}</Link>)}</nav>
      <div className="sidebar-bottom"><div className="workspace-note"><span className="tiny-dot" /><span>{server.source === "agent" ? "Мониторинг узла" : server.source === "waiting" ? "Ожидаем агента" : "Демонстрационный режим"}</span><span className="mono">v0.4</span></div><Link href="/settings/" onClick={() => setMobileOpen(false)} className={`nav-link ${activePath === "/settings/" ? "nav-active" : ""}`}><Settings2 size={19} strokeWidth={1.6} /><span>Настройки</span></Link><div className="profile"><div className="avatar" title={status?.user?.name}>{initials}</div><div><strong>{status?.user?.name}</strong><span>Владелец панели</span></div><button className="icon-button" aria-label="Выйти из аккаунта" onClick={()=>logout().catch(e=>setLogoutError(e instanceof Error?e.message:"Не удалось выйти"))}><LogOut size={17}/></button></div></div>
    </aside>
    <div className="workspace">
      <header className="topbar"><div className="breadcrumbs"><button className="icon-button mobile-menu" onClick={() => setMobileOpen(true)} aria-label="Открыть навигацию"><Menu size={20} /></button><ChevronsLeftRight className="breadcrumb-icon" size={18} /><span>{section}</span><span className="breadcrumb-slash">/</span><strong>{server.name}</strong><span className="demo-pill" title="Источник показаний">{server.source === "agent" ? "Агент" : server.source === "waiting" ? "Ожидание" : "Демо"}</span></div><div className="topbar-actions"><UpdateMode /><button className="global-search" onClick={() => { setQuery(""); setSearchOpen(true); }}><Search size={16} /><span>Найти сервер или раздел</span><kbd>Ctrl K</kbd></button><button className="icon-button theme-button" onClick={toggleTheme} aria-label={theme === "dark" ? "Включить светлую тему" : "Включить тёмную тему"} title={theme === "dark" ? "Светлая тема" : "Тёмная тема"}>{theme === "dark" ? <Sun size={19} /> : <Moon size={19} />}</button><Link className="icon-button notification-button" href="/events/" aria-label="Открыть события"><Bell size={19} /><span className="notification-dot" /></Link><div className="avatar avatar-small" aria-hidden="true" title={status?.user?.name}>{initials}</div></div></header>
      <main id="main" tabIndex={-1} className="main-content" key={activePath}>
        <div className="context-bar"><span className="section-eyebrow">МОНИТОРИНГ / {section.toLocaleUpperCase("ru")}</span><details ref={picker} className="server-picker"><summary><OSIcon os={server.os} size={15} /><span>{server.name}</span><ChevronDown size={14} /></summary><div className="server-menu">{servers.map(item => <button key={item.id} onClick={() => { selectServer(item.id); if (picker.current) picker.current.open = false; }}><span><strong>{item.name}</strong><small>{addressLabel(item.address)}</small></span>{item.id === server.id ? <Check size={17} /> : <span className={`tiny-dot ${item.online ? "" : "offline-dot"}`} />}</button>)}<Link href="/servers/">Управление серверами<ArrowRight size={15} /></Link></div></details></div>
        {backend === "unavailable" ? <div className="backend-notice"><span>Go-бэкенд недоступен. Последние данные могут быть устаревшими; обновление и архив недоступны.</span><button className="text-button" onClick={retryBackend}>Повторить подключение</button></div> : null}
        {logoutError?<p className="form-error" role="alert">{logoutError}</p>:null}
        {children}
        <footer className="page-footer"><span><Activity size={12} /> PULSE MONITOR</span><span>{backend === "connected" ? `Go-бэкенд · ${streamState} · ${server.source === "agent" ? "агент" : "демо"}` : "Локальное демо · агент не подключён"}</span></footer>
      </main>
    </div>
    <Toast />
    {searchOpen ? <Modal title="Быстрый переход" subtitle="Поиск серверов и разделов · Ctrl K" onClose={() => setSearchOpen(false)}><div className="drawer-body"><SearchInput label="Введите название…" value={query} onChange={setQuery} /><div className="command-list"><span className="small-label">РАЗДЕЛЫ</span>{navigation.filter(n => n.label.toLowerCase().includes(query.toLowerCase())).map(({ href, label, icon: Icon }) => <Link key={href} href={href} onClick={() => setSearchOpen(false)}><Icon size={18} />{label}<ArrowRight size={15} /></Link>)}<span className="small-label">СЕРВЕРЫ</span>{servers.filter(s => `${s.name} ${s.address}`.toLowerCase().includes(query.toLowerCase())).map(item => <button key={item.id} onClick={() => { selectServer(item.id); setSearchOpen(false); }}><Server size={18} /><span>{item.name}<small>{addressLabel(item.address)}</small></span><ArrowRight size={15} /></button>)}</div></div></Modal> : null}
  </div>;
}

