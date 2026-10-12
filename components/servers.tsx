"use client";

import { useState } from "react";
import Link from "next/link";
import { Plus, Server as ServerIcon, ArrowUpRight, Settings2, Trash2 } from "lucide-react";
import { useMonitor } from "./provider";
import { Modal, PageHeading, PageMotion, Panel, SearchInput, Skeleton, Status } from "./ui";
import { addressLabel, validAddress, type Server } from "@/lib/demo";

import { OSIcon } from "./icons";
import { api } from "@/lib/monitor";

function AgentSetup({id}:{id:string}) {
 const {notify}=useMonitor();
 const [token,setToken]=useState(''),[error,setError]=useState(''),[busy,setBusy]=useState(false);
 async function issue(){setBusy(true);setError('');try{const result=await api<{token:string}>(`/servers/${encodeURIComponent(id)}/token`,{method:'POST',body:'{}'});setToken(result.token);notify('Новый токен создан. Предыдущий токен отозван.');}catch(e){setError(e instanceof Error?e.message:'Не удалось создать токен');}finally{setBusy(false);}}
 async function revoke(){setBusy(true);setError('');try{await api(`/servers/${encodeURIComponent(id)}/token`,{method:'DELETE'});setToken('');notify('Токен отозван');}catch(e){setError(e instanceof Error?e.message:'Не удалось отозвать токен');}finally{setBusy(false);}}
 return <div className="agent-setup"><h3>Подключение Go-агента</h3><p className="muted">Агент сам отправляет данные в панель. Сохраните токен в файл на узле. Токен показывается только при создании.</p><div className="button-row"><button type="button" className="button" disabled={busy} onClick={()=>void issue()}>Создать / заменить токен</button><button type="button" className="button" disabled={busy} onClick={()=>void revoke()}>Отозвать</button></div>{token?<><label className="field">Токен<input readOnly value={token} onFocus={e=>e.target.select()}/></label><label className="field">Команда запуска<textarea readOnly rows={4} value={`pulse-agent -url ${window.location.origin} -server ${id} -token-file agent.token`}/></label><small>Бинарники: artifacts/releases. После первого отчёта ОС определится автоматически.</small></>:null}{error?<p className="form-error" role="alert">{error}</p>:null}</div>;
}

function ServerEditor({ item, onClose }: { item?: Server; onClose: () => void }) {
  const { servers, saveServer, removeServer, notify } = useMonitor();
  const [name, setName] = useState(item?.name ?? "");
  const [address, setAddress] = useState(item?.address ?? "");
  const [group, setGroup] = useState(item?.group ?? "Production");
  const [threshold, setThreshold] = useState(item?.threshold ?? 85);
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  function validate() {
    if (!name.trim()) { setError("Укажите название сервера"); return false; }
    if (!validAddress(address)) { setError("Укажите URL узла: https://host без пароля и параметров"); return false; }
    if (servers.some(s => s.id !== item?.id && s.name.toLowerCase() === name.trim().toLowerCase())) { setError("Сервер с таким названием уже существует"); return false; }
    setError(""); return true;
  }
  async function save(e: React.FormEvent) {
    e.preventDefault(); if (!validate()) return;
    setSaving(true);
    try { await saveServer({ id: item?.id ?? crypto.randomUUID(), name: name.trim(), address: address.trim(), os: item?.os ?? "", group: group.trim() || "Без группы", online: item?.online ?? false, cpu: item?.cpu ?? 0, memory: item?.memory ?? 0, disk: item?.disk ?? 0, interval: item?.interval ?? 5, threshold });
    notify(item ? "Настройки сервера сохранены" : "Сервер добавлен"); onClose();
    } catch (e) { setError(e instanceof Error ? e.message : "Не удалось сохранить сервер"); } finally { setSaving(false); }
  }
  return <Modal title={item ? "Настройки сервера" : "Добавить сервер"} subtitle="Настройте узел вашего рабочего пространства" onClose={onClose}><form onSubmit={save} className="drawer-form"><div className="drawer-body"><div className="setup-steps"><span className="step-active">01 / Подключение</span><span>02 / Мониторинг</span></div><div className="form-note"><ServerIcon size={20} /><p>ОС и конфигурация определятся автоматически при подключении агента. Сейчас сохраняется адрес узла.</p></div><label className="field">Название<input autoFocus value={name} onChange={e => setName(e.target.value)} placeholder="production-04" maxLength={48} /></label><label className="field">Адрес узла<input value={address} onChange={e => { setAddress(e.target.value); }} placeholder="https://192.168.1.10:9443" /><small>Полный HTTP(S) адрес сервера</small></label><div className="form-note"><OSIcon os={item?.os ?? ''} /><p>Операционная система<br /><strong>{item?.os || 'Определится при подключении агента'}</strong></p></div><label className="field">Группа<input value={group} onChange={e => setGroup(e.target.value)} maxLength={32} /></label><div className="form-divider" /><h3>Мониторинг</h3><label className="field">Порог предупреждений · {threshold}%<input type="range" min="50" max="99" value={threshold} onChange={e => setThreshold(Number(e.target.value))} /><small>Порог для CPU, памяти, GPU и заполнения дисков.</small></label>{error ? <p className="form-error" role="alert">{error}</p> : null}{item ? <><AgentSetup id={item.id}/><div className="danger-zone"><h3>Удаление сервера</h3><p>Сервер будет удалён из реестра. Архив логов сохранится.</p>{confirmDelete ? <div className="button-row"><button type="button" className="button primary" disabled={saving} onClick={async () => { setSaving(true); try { await removeServer(item.id); notify("Сервер удалён"); onClose(); } catch(e) { setError(e instanceof Error ? e.message : "Не удалось удалить сервер"); } finally { setSaving(false); } }}>Подтвердить удаление</button><button type="button" className="button" onClick={() => setConfirmDelete(false)}>Отмена</button></div> : <button type="button" className="button" disabled={servers.length === 1} onClick={() => setConfirmDelete(true)}><Trash2 size={15} />Удалить сервер</button>}{servers.length === 1 ? <small>В рабочем пространстве должен остаться хотя бы один сервер.</small> : null}</div></> : null}</div><div className="drawer-footer"><button type="button" className="button" onClick={onClose}>Отмена</button><button className="button primary" type="submit" disabled={saving}>{item ? "Сохранить изменения" : "Добавить сервер"}<ArrowUpRight size={16} /></button></div></form></Modal>;
}

export function Servers() {
  const { servers, server, selectServer, loading } = useMonitor();
  const [query, setQuery] = useState("");
  const [editor, setEditor] = useState<Server | "new" | null>(null);
  const items = servers.filter(s => `${s.name} ${s.address} ${s.group} ${s.os}`.toLowerCase().includes(query.toLowerCase()));
  return <PageMotion><PageHeading title="Ваши серверы." subtitle="Одна панель. Вся инфраструктура." actions={<button className="button primary" onClick={() => setEditor("new")}><Plus size={17} />Добавить сервер</button>} /><div className="summary-strip"><span><strong>{servers.length}</strong> Всего серверов</span><span><i className="tiny-dot" /><strong>{servers.filter(s => s.online).length}</strong> В сети</span><span><strong>{servers.filter(s => !s.online).length}</strong> Недоступны</span></div>{loading ? <Skeleton /> : <Panel className="server-list"><div className="table-toolbar"><SearchInput label="Поиск серверов…" value={query} onChange={setQuery} /><span className="muted">Локальное рабочее пространство</span></div><div className="table-scroll"><table><thead><tr><th>Сервер</th><th>Статус</th><th>CPU</th><th>Память</th><th>Группа</th><th><span className="sr-only">Действия</span></th></tr></thead><tbody>{items.map(item => <tr key={item.id} className={server.id === item.id ? "selected-row" : ""}><td><div className="server-cell"><span className="server-symbol small"><OSIcon os={item.os} /></span><span><Link href="/" onClick={() => selectServer(item.id)}>{item.name}<ArrowUpRight size={12} /></Link><small>{item.os || "Ожидаем агента"} · {addressLabel(item.address)}</small></span></div></td><td><Status online={item.online} /></td><td className="mono">{item.online ? `${item.cpu.toFixed(1)}%` : "—"}</td><td className="mono">{item.online ? `${item.memory.toFixed(1)}%` : "—"}</td><td><span className="tag">{item.group}</span></td><td><button className="icon-button" aria-label={`Настройки ${item.name}`} onClick={() => setEditor(item)}><Settings2 size={18} /></button></td></tr>)}</tbody></table></div>{!items.length ? <div className="empty-state"><h3>Серверы не найдены</h3><p>Попробуйте другой запрос.</p></div> : null}<div className="table-footer">{items.length} из {servers.length} серверов<span>Linux + Windows</span></div></Panel>}{editor ? <ServerEditor item={editor === "new" ? undefined : editor} onClose={() => setEditor(null)} /> : null}</PageMotion>;
}

