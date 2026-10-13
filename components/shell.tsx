"use client";
import {useLocale,tr,trText} from "./locale";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useRef, useState, type ReactNode } from "react";
import { Activity, LayoutDashboard, Server, ListTree, ChartPie, ScrollText, Bell, Settings2, Sun, Moon, ChevronDown, ChevronsLeftRight, Menu, Network, Search, ArrowRight, Check, LogOut, Puzzle } from "lucide-react";
import { Provider, useMonitor } from "./provider";
import { Modal, SearchInput, Toast } from "./ui";
import { OSIcon } from "./icons";
import { UpdateMode } from "./update-mode";
import { addressLabel } from "@/lib/data";
import { userInitials } from "@/lib/monitor";
import { AuthProvider, SessionGate, useAuth } from "./auth";
const navigation = [
    { href: "/", label: tr("Обзор"), icon: LayoutDashboard },
    { href: "/servers/", label: tr("Серверы"), icon: Server },
    { href: "/processes/", label: tr("Процессы"), icon: ListTree },
    { href: "/network/", label: tr("Сеть"), icon: Network },
    { href: "/resources/", label: tr("Ресурсы"), icon: ChartPie },
    { href: "/logs/", label: tr("Логи"), icon: ScrollText },
    { href: "/modules/", label: tr("Модули"), icon: Puzzle },
    { href: "/events/", label: tr("События"), icon: Bell },
];
export function Shell({ children }: {
    children: ReactNode;
}) {
    useLocale();
    return <AuthProvider><SessionShell>{trText(children)}</SessionShell></AuthProvider>;
}
function SessionShell({ children }: {
    children: ReactNode;
}) {
    useLocale();
    const pathname = usePathname();
    if (['/login', '/login/', '/register', '/register/'].includes(pathname))
        return children;
    return <SessionGate><Provider><Chrome>{trText(children)}</Chrome></Provider></SessionGate>;
}
function Chrome({ children }: {
    children: ReactNode;
}) {
    useLocale();
    const pathname = usePathname();
    const { status, logout } = useAuth();
    const [logoutError, setLogoutError] = useState("");
    const initials = userInitials(status?.user?.name);
    const { server, servers, theme, toggleTheme, selectServer, preferences, backend, streamState, retryBackend } = useMonitor();
    const [mobileOpen, setMobileOpen] = useState(false);
    const [searchOpen, setSearchOpen] = useState(false);
    const [query, setQuery] = useState("");
    const picker = useRef<HTMLDetailsElement>(null);
    const activePath = pathname === "/" ? "/" : `${pathname.replace(/\/$/, "")}/`;
    const section = navigation.find(n => n.href === activePath)?.label ?? tr("Настройки");
    useEffect(() => {
        function shortcut(e: KeyboardEvent) {
            if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "k") {
                e.preventDefault();
                setSearchOpen(value => !value);
            }
            if (e.key === "Escape") {
                setMobileOpen(false);
                if (picker.current)
                    picker.current.open = false;
            }
        }
        function outside(e: MouseEvent) { if (picker.current && !picker.current.contains(e.target as Node))
            picker.current.open = false; }
        document.addEventListener("keydown", shortcut);
        document.addEventListener("click", outside);
        return () => { document.removeEventListener("keydown", shortcut); document.removeEventListener("click", outside); };
    }, []);
    return <div className={`app-shell ${preferences.compact ? "density-compact" : ""} ${preferences.motion ? "" : "motion-off"}`}>
    <a href="#main" className="skip-link">{tr("Перейти к содержимому")}</a>
    {trText(mobileOpen ? <button className="nav-scrim" aria-label={tr("Закрыть навигацию")} onClick={() => setMobileOpen(false)}/> : null)}
    <aside className={`sidebar ${mobileOpen ? "sidebar-open" : ""}`}>
      <Link href="/" className="brand" aria-label={tr("Pulse \u2014 обзор")}><Activity size={33} strokeWidth={1.7}/><span>PULSE</span><span className="brand-version">/ 01</span></Link>
      <div className="nav-caption">{tr("РАБОЧЕЕ ПРОСТРАНСТВО")}</div>
      <nav aria-label={tr("Основная навигация")}>{trText(navigation.map(({ href, label, icon: Icon }) => <Link key={href} href={href} className={`nav-link ${activePath === href ? "nav-active" : ""}`} aria-current={activePath === href ? "page" : undefined} onClick={() => setMobileOpen(false)}><Icon size={19} strokeWidth={1.6}/><span>{trText(label)}</span>{trText(href === "/servers/" ? <span className="nav-count">{trText(servers.length)}</span> : null)}</Link>))}</nav>
      <div className="sidebar-bottom"><div className="workspace-note"><span className="tiny-dot"/><span>{trText(server?.source === "agent" ? tr("Мониторинг узла") : server?.source === "waiting" ? tr("Ожидаем агента") : tr("Подключите сервер"))}</span><span className="mono">v0.4</span></div><Link href="/settings/" onClick={() => setMobileOpen(false)} className={`nav-link ${activePath === "/settings/" ? "nav-active" : ""}`}><Settings2 size={19} strokeWidth={1.6}/><span>{tr("Настройки")}</span></Link><div className="profile"><div className="avatar" title={status?.user?.name}>{trText(initials)}</div><div><strong>{status?.user?.name}</strong><span>{tr("Владелец панели")}</span></div><button className="icon-button" aria-label={tr("Выйти из аккаунта")} onClick={() => logout().catch(e => setLogoutError(e instanceof Error ? e.message : tr("Не удалось выйти")))}><LogOut size={17}/></button></div></div>
    </aside>
    <div className="workspace">
      <header className="topbar"><div className="breadcrumbs"><button className="icon-button mobile-menu" onClick={() => setMobileOpen(true)} aria-label={tr("Открыть навигацию")}><Menu size={20}/></button><ChevronsLeftRight className="breadcrumb-icon" size={18}/><span>{trText(section)}</span><span className="breadcrumb-slash">/</span><strong>{trText((server?.name ?? tr("Нет серверов")))}</strong><span className="demo-pill" title={tr("Источник показаний")}>{trText(server?.source === "agent" ? tr("Агент") : server?.source === "waiting" ? tr("Ожидание") : tr("Нет узлов"))}</span></div><div className="topbar-actions"><UpdateMode /><button className="global-search" onClick={() => { setQuery(""); setSearchOpen(true); }}><Search size={16}/><span>{tr("Найти сервер или раздел")}</span><kbd>Ctrl K</kbd></button><button className="icon-button theme-button" onClick={toggleTheme} aria-label={theme === "dark" ? tr("Включить светлую тему") : tr("Включить тёмную тему")} title={theme === "dark" ? tr("Светлая тема") : tr("Тёмная тема")}>{trText(theme === "dark" ? <Sun size={19}/> : <Moon size={19}/>)}</button><Link className="icon-button notification-button" href="/events/" aria-label={tr("Открыть события")}><Bell size={19}/><span className="notification-dot"/></Link><div className="avatar avatar-small" aria-hidden="true" title={status?.user?.name}>{trText(initials)}</div></div></header>
      <main id="main" tabIndex={-1} className="main-content" key={activePath}>
        <div className="context-bar"><span className="section-eyebrow">{tr("МОНИТОРИНГ /") + " "}{trText(section.toLocaleUpperCase("ru"))}</span><details ref={picker} className="server-picker"><summary><OSIcon os={(server?.os ?? "")} size={15}/><span>{trText((server?.name ?? tr("Нет серверов")))}</span><ChevronDown size={14}/></summary><div className="server-menu">{trText(servers.map(item => <button key={item.id} onClick={() => { selectServer(item.id); if (picker.current)
        picker.current.open = false; }}><span><strong>{item.name}</strong><small>{trText(addressLabel(item.address))}</small></span>{trText(item.id === server?.id ? <Check size={17}/> : <span className={`tiny-dot ${item.online ? "" : "offline-dot"}`}/>)}</button>))}<Link href="/servers/">{tr("Управление серверами")}<ArrowRight size={15}/></Link></div></details></div>
        {trText(backend === "unavailable" ? <div className="backend-notice"><span>{tr("Go-бэкенд недоступен. Последние данные могут быть устаревшими; обновление и архив недоступны.")}</span><button className="text-button" onClick={retryBackend}>{tr("Повторить подключение")}</button></div> : null)}
        {trText(logoutError ? <p className="form-error" role="alert">{trText(logoutError)}</p> : null)}
        {trText(!server && !["/servers/", "/settings/", "/events/", "/modules/"].includes(activePath) ? <div className="panel empty-state"><h2>{tr("Подключите первый сервер")}</h2><p>{tr("Монитор уже работает. Добавьте сервер, установите на нём агент и смотрите реальные показатели здесь.")}</p><Link className="button primary" href="/servers/">{tr("Добавить сервер")}</Link></div> : children)}
        <footer className="page-footer"><span><Activity size={12}/> PULSE MONITOR</span><span>{trText(backend === "connected" ? tr("Go-бэкенд \u00B7 {p0} \u00B7 {p1}", { p0: streamState, p1: server?.source === "agent" ? tr("агент") : tr("ожидание") }) : tr("Монитор недоступен"))}</span></footer>
      </main>
    </div>
    <Toast />
    {trText(searchOpen ? <Modal title={tr("Быстрый переход")} subtitle={tr("Поиск серверов и разделов \u00B7 Ctrl K")} onClose={() => setSearchOpen(false)}><div className="drawer-body"><SearchInput label={tr("Введите название\u2026")} value={query} onChange={setQuery}/><div className="command-list"><span className="small-label">{tr("РАЗДЕЛЫ")}</span>{trText(navigation.filter(n => n.label.toLowerCase().includes(query.toLowerCase())).map(({ href, label, icon: Icon }) => <Link key={href} href={href} onClick={() => setSearchOpen(false)}><Icon size={18}/>{trText(label)}<ArrowRight size={15}/></Link>))}<span className="small-label">{tr("СЕРВЕРЫ")}</span>{trText(servers.filter(s => `${s.name} ${s.address}`.toLowerCase().includes(query.toLowerCase())).map(item => <button key={item.id} onClick={() => { selectServer(item.id); setSearchOpen(false); }}><Server size={18}/><span>{item.name}<small>{trText(addressLabel(item.address))}</small></span><ArrowRight size={15}/></button>))}</div></div></Modal> : null)}
  </div>;
}
