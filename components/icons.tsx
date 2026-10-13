"use client";
import {useLocale,tr,trText} from "./locale";
import { Activity, Cpu, Database, Globe, Grid2X2, Settings, ShieldCheck, Terminal, Server } from 'lucide-react';
function BrandIcon({ slug, label, size = 20 }: {
    slug: string;
    label: string;
    size?: number;
}) {
    useLocale();
    return <span className="brand-icon" role="img" aria-label={label} title={label} style={{ width: size, height: size, maskImage: `url(/icons/${slug}.svg)`, WebkitMaskImage: `url(/icons/${slug}.svg)` }}/>;
}
export function OSIcon({ os, size = 22 }: {
    os: string;
    size?: number;
}) {
    useLocale();
    if (!os)
        return <Server size={size} aria-label={tr("ОС ещё не определена")} role="img"/>;
    const name = os.toLowerCase();
    if (name.includes('ubuntu'))
        return <BrandIcon slug="ubuntu" label={os} size={size}/>;
    if (name.includes('debian'))
        return <BrandIcon slug="debian" label={os} size={size}/>;
    if (name.includes('windows'))
        return <Grid2X2 size={size} aria-label={os} role="img"/>;
    return <BrandIcon slug="linux" label={os} size={size}/>;
}
export function ProcessIcon({ name }: {
    name: string;
}) {
    useLocale();
    const value = name.toLowerCase();
    const slug = value.includes('nginx') ? 'nginx' : value.includes('postgres') ? 'postgresql' : value.includes('node') ? 'nodedotjs' : value.includes('redis') ? 'redis' : value.includes('docker') ? 'docker' : null;
    const Icon = value.includes('pulse') ? Activity : value.includes('sql') ? Database : value.includes('w3wp') ? Globe : value.includes('msmp') ? ShieldCheck : value.includes('svchost') ? Grid2X2 : value.includes('system') ? Settings : value.includes('sshd') ? Terminal : Cpu;
    return <span className="process-icon" title={name}>{trText(slug ? <BrandIcon slug={slug} label={name} size={15}/> : <Icon size={16} strokeWidth={1.5} aria-label={name} role="img"/>)}</span>;
}
