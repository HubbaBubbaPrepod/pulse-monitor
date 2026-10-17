'use client';
import { formatNumber } from '@/lib/i18n';
import { useLocale, tr, trText } from './locale';
import { useState } from 'react';
import Link from 'next/link';
import { Plus, Server as ServerIcon, ArrowUpRight, Settings2, Trash2 } from 'lucide-react';
import { useMonitor } from './provider';
import { Modal, PageHeading, PageMotion, Panel, SearchInput, Skeleton, Status } from './ui';
import { addressLabel, validAddress, type Server } from '@/lib/data';
import { OSIcon } from './icons';
import { api } from '@/lib/monitor';
import { parseRetentionDays, retentionNeedsConfirmation } from '@/lib/retention';
function AgentSetup({ id }: { id: string }) {
  useLocale();
  const { notify } = useMonitor();
  const [token, setToken] = useState(''),
    [error, setError] = useState(''),
    [busy, setBusy] = useState(false);
  async function issue() {
    setBusy(true);
    setError('');
    try {
      const result = await api<{
        token: string;
      }>(`/servers/${encodeURIComponent(id)}/token`, { method: 'POST', body: '{}' });
      setToken(result.token);
      notify(tr('Новый токен создан. Предыдущий токен отозван.'));
    } catch (e) {
      setError(e instanceof Error ? e.message : tr('Не удалось создать токен'));
    } finally {
      setBusy(false);
    }
  }
  async function revoke() {
    setBusy(true);
    setError('');
    try {
      await api(`/servers/${encodeURIComponent(id)}/token`, { method: 'DELETE' });
      setToken('');
      notify(tr('Токен отозван'));
    } catch (e) {
      setError(e instanceof Error ? e.message : tr('Не удалось отозвать токен'));
    } finally {
      setBusy(false);
    }
  }
  return (
    <div className="agent-setup">
      <h3>{tr('Подключение Go-агента')}</h3>
      <p className="muted">
        {tr(
          'Агент сам отправляет данные в панель. Сохраните токен в файл на узле. Токен показывается только при создании.',
        )}
      </p>
      <div className="button-row">
        <button type="button" className="button" disabled={busy} onClick={() => void issue()}>
          {tr('Создать / заменить токен')}
        </button>
        <button type="button" className="button" disabled={busy} onClick={() => void revoke()}>
          {tr('Отозвать')}
        </button>
      </div>
      {trText(
        token ? (
          <>
            <label className="field">
              {tr('Токен')}
              <input readOnly value={token} onFocus={(e) => e.target.select()} />
            </label>
            <label className="field">
              {tr('Команда запуска')}
              <textarea
                readOnly
                rows={4}
                value={`pulse-agent -url ${window.location.origin} -server ${id} -token-file agent.token`}
              />
            </label>
            <small>
              {tr(
                'Бинарники: artifacts/releases. После первого отчёта ОС определится автоматически.',
              )}
            </small>
          </>
        ) : null,
      )}
      {trText(
        error ? (
          <p className="form-error" role="alert">
            {trText(error)}
          </p>
        ) : null,
      )}
    </div>
  );
}
function ServerEditor({ item, onClose }: { item?: Server; onClose: () => void }) {
  useLocale();
  const { servers, saveServer, removeServer, notify } = useMonitor();
  const [id] = useState(() => item?.id ?? crypto.randomUUID());
  const [name, setName] = useState(item?.name ?? '');
  const [address, setAddress] = useState(item?.address ?? '');
  const [group, setGroup] = useState(item?.group ?? 'Production');
  const [threshold, setThreshold] = useState(item?.threshold ?? 85);
  const [retention, setRetention] = useState(String(item?.logRetentionDays ?? (item ? 0 : 30)));
  const [customRetention, setCustomRetention] = useState(
    !['0', '1', '7', '14', '30', '90'].includes(retention),
  );
  const [confirmRetention, setConfirmRetention] = useState(false);
  const retentionValue = parseRetentionDays(retention);
  const needsRetentionConfirmation =
    !!item &&
    retentionValue !== null &&
    retentionNeedsConfirmation(item.logRetentionDays ?? 0, retentionValue);
  const [error, setError] = useState('');
  const [saving, setSaving] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  function validate() {
    if (retentionValue === null) {
      setError(tr('Укажите целое число дней от 1 до 3650 или отключите очистку'));
      return false;
    }
    if (needsRetentionConfirmation && !confirmRetention) {
      setError(tr('Подтвердите удаление старых логов'));
      return false;
    }
    if (!name.trim()) {
      setError(tr('Укажите название сервера'));
      return false;
    }
    if (!validAddress(address)) {
      setError(tr('Укажите URL узла: https://host без пароля и параметров'));
      return false;
    }
    if (servers.some((s) => s.id !== id && s.name.toLowerCase() === name.trim().toLowerCase())) {
      setError(tr('Сервер с таким названием уже существует'));
      return false;
    }
    setError('');
    return true;
  }
  async function save(e: React.FormEvent) {
    e.preventDefault();
    if (!validate()) return;
    setSaving(true);
    try {
      await saveServer({
        id,
        name: name.trim(),
        address: address.trim(),
        os: item?.os ?? '',
        group: group.trim() || tr('Без группы'),
        online: item?.online ?? false,
        cpu: item?.cpu ?? 0,
        memory: item?.memory ?? 0,
        disk: item?.disk ?? 0,
        interval: item?.interval ?? 5,
        threshold,
      });
      if (retentionValue !== null && (!item || retentionValue !== (item.logRetentionDays ?? 0))) {
        await api(`/servers/${encodeURIComponent(id)}/log-retention`, {
          method: 'PUT',
          body: JSON.stringify({ days: retentionValue }),
        });
      }
      notify(item ? tr('Настройки сервера сохранены') : tr('Сервер добавлен'));
      onClose();
    } catch (e) {
      setError(e instanceof Error ? e.message : tr('Не удалось сохранить сервер'));
    } finally {
      setSaving(false);
    }
  }
  return (
    <Modal
      title={item ? tr('Настройки сервера') : tr('Добавить сервер')}
      subtitle={tr('Настройте узел вашего рабочего пространства')}
      onClose={onClose}
    >
      <form onSubmit={save} className="drawer-form">
        <div className="drawer-body">
          <div className="setup-steps">
            <span className="step-active">{tr('01 / Подключение')}</span>
            <span>{tr('02 / Мониторинг')}</span>
          </div>
          <div className="form-note">
            <ServerIcon size={20} />
            <p>
              {tr(
                'ОС и конфигурация определятся автоматически при подключении агента. Сейчас сохраняется адрес узла.',
              )}
            </p>
          </div>
          <label className="field">
            {tr('Название')}
            <input
              autoFocus
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="production-04"
              maxLength={48}
            />
          </label>
          <label className="field">
            {tr('Адрес узла')}
            <input
              value={address}
              onChange={(e) => {
                setAddress(e.target.value);
              }}
              placeholder="https://192.168.1.10:9443"
            />
            <small>{tr('Полный HTTP(S) адрес сервера')}</small>
          </label>
          <div className="form-note">
            <OSIcon os={item?.os ?? ''} />
            <p>
              {tr('Операционная система')}
              <br />
              <strong>{trText(item?.os || tr('Определится при подключении агента'))}</strong>
            </p>
          </div>
          <label className="field">
            {tr('Группа')}
            <input value={group} onChange={(e) => setGroup(e.target.value)} maxLength={32} />
          </label>
          <div className="form-divider" />
          <h3>{tr('Мониторинг')}</h3>
          <label className="field">
            {tr('Порог предупреждений \u00B7') + ' '}
            {trText(threshold)}%
            <input
              type="range"
              min="50"
              max="99"
              value={threshold}
              onChange={(e) => setThreshold(Number(e.target.value))}
            />
            <small>{tr('Порог для CPU, памяти, GPU и заполнения дисков.')}</small>
          </label>
          <label className="field">
            {tr('Хранение логов в Monitor')}
            <select
              value={customRetention ? 'custom' : retention}
              onChange={(e) => {
                setCustomRetention(e.target.value === 'custom');
                setRetention(e.target.value === 'custom' ? '' : e.target.value);
                setConfirmRetention(false);
              }}
            >
              <option value="0">{tr('Не удалять автоматически')}</option>
              {[1, 7, 14, 30, 90].map((days) => (
                <option key={days} value={String(days)}>
                  {days} {tr('дней')}
                </option>
              ))}
              <option value="custom">{tr('Свой срок')}</option>
            </select>
            {customRetention && (
              <input
                type="number"
                min="1"
                max="3650"
                step="1"
                value={retention}
                placeholder="30"
                aria-label={tr('Срок хранения, дней')}
                onChange={(e) => {
                  setRetention(e.target.value);
                  setConfirmRetention(false);
                }}
              />
            )}
            <small>
              {tr(
                'Срок считается с момента записи в Monitor. Логи на самом сервере не удаляются. Метрики и события сохраняются отдельно.',
              )}
            </small>
          </label>
          {needsRetentionConfirmation && (
            <label className="field">
              <span>
                {tr(
                  'Записи старше выбранного срока будут удаляться без возможности восстановления из Monitor.',
                )}
              </span>
              <span>
                <input
                  type="checkbox"
                  checked={confirmRetention}
                  onChange={(e) => setConfirmRetention(e.target.checked)}
                />{' '}
                {tr('Подтверждаю удаление старых логов')}
              </span>
            </label>
          )}
          {trText(
            error ? (
              <p className="form-error" role="alert">
                {trText(error)}
              </p>
            ) : null,
          )}
          {trText(
            item ? (
              <>
                <AgentSetup id={item.id} />
                <div className="danger-zone">
                  <h3>{tr('Удаление сервера')}</h3>
                  <p>{tr('Сервер будет удалён из реестра. Архив логов сохранится.')}</p>
                  {trText(
                    confirmDelete ? (
                      <div className="button-row">
                        <button
                          type="button"
                          className="button primary"
                          disabled={saving}
                          onClick={async () => {
                            setSaving(true);
                            try {
                              await removeServer(item.id);
                              notify(tr('Сервер удалён'));
                              onClose();
                            } catch (e) {
                              setError(
                                e instanceof Error ? e.message : tr('Не удалось удалить сервер'),
                              );
                            } finally {
                              setSaving(false);
                            }
                          }}
                        >
                          {tr('Подтвердить удаление')}
                        </button>
                        <button
                          type="button"
                          className="button"
                          onClick={() => setConfirmDelete(false)}
                        >
                          {tr('Отмена')}
                        </button>
                      </div>
                    ) : (
                      <button
                        type="button"
                        className="button"
                        onClick={() => setConfirmDelete(true)}
                      >
                        <Trash2 size={15} />
                        {tr('Удалить сервер')}
                      </button>
                    ),
                  )}
                </div>
              </>
            ) : null,
          )}
        </div>
        <div className="drawer-footer">
          <button type="button" className="button" onClick={onClose}>
            {tr('Отмена')}
          </button>
          <button className="button primary" type="submit" disabled={saving}>
            {trText(item ? tr('Сохранить изменения') : tr('Добавить сервер'))}
            <ArrowUpRight size={16} />
          </button>
        </div>
      </form>
    </Modal>
  );
}
export function Servers() {
  useLocale();
  const { servers, server, selectServer, loading } = useMonitor();
  const [query, setQuery] = useState('');
  const [editor, setEditor] = useState<Server | 'new' | null>(null);
  const items = servers.filter((s) =>
    `${s.name} ${s.address} ${s.group} ${s.os}`.toLowerCase().includes(query.toLowerCase()),
  );
  return (
    <PageMotion>
      <PageHeading
        title={tr('Ваши серверы.')}
        subtitle={tr('Одна панель. Вся инфраструктура.')}
        actions={
          <button className="button primary" onClick={() => setEditor('new')}>
            <Plus size={17} />
            {tr('Добавить сервер')}
          </button>
        }
      />
      <div className="summary-strip">
        <span>
          <strong>{trText(servers.length)}</strong>
          {' ' + tr('Всего серверов')}
        </span>
        <span>
          <i className="tiny-dot" />
          <strong>{trText(servers.filter((s) => s.online).length)}</strong>
          {' ' + tr('В сети')}
        </span>
        <span>
          <strong>{trText(servers.filter((s) => !s.online).length)}</strong>
          {' ' + tr('Недоступны')}
        </span>
      </div>
      {trText(
        loading ? (
          <Skeleton />
        ) : (
          <Panel className="server-list">
            <div className="table-toolbar">
              <SearchInput label={tr('Поиск серверов\u2026')} value={query} onChange={setQuery} />
              <span className="muted">{tr('Локальное рабочее пространство')}</span>
            </div>
            <div className="table-scroll">
              <table>
                <thead>
                  <tr>
                    <th>{tr('Сервер')}</th>
                    <th>{tr('Статус')}</th>
                    <th>CPU</th>
                    <th>{tr('Память')}</th>
                    <th>{tr('Группа')}</th>
                    <th>
                      <span className="sr-only">{tr('Действия')}</span>
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {trText(
                    items.map((item) => (
                      <tr key={item.id} className={server?.id === item.id ? 'selected-row' : ''}>
                        <td>
                          <div className="server-cell">
                            <span className="server-symbol small">
                              <OSIcon os={item.os} />
                            </span>
                            <span>
                              <Link href="/" onClick={() => selectServer(item.id)}>
                                {item.name}
                                <ArrowUpRight size={12} />
                              </Link>
                              <small>
                                {trText(item.os || tr('Ожидаем агента'))} ·{' '}
                                {trText(addressLabel(item.address))}
                              </small>
                            </span>
                          </div>
                        </td>
                        <td>
                          <Status online={item.online} />
                        </td>
                        <td className="mono">
                          {trText(
                            item.online
                              ? `${formatNumber(item.cpu, undefined, { minimumFractionDigits: 1, maximumFractionDigits: 1 })}%`
                              : '—',
                          )}
                        </td>
                        <td className="mono">
                          {trText(
                            item.online
                              ? `${formatNumber(item.memory, undefined, { minimumFractionDigits: 1, maximumFractionDigits: 1 })}%`
                              : '—',
                          )}
                        </td>
                        <td>
                          <span className="tag">{item.group}</span>
                        </td>
                        <td>
                          <button
                            className="icon-button"
                            aria-label={tr('Настройки {p0}', { p0: item.name })}
                            onClick={() => setEditor(item)}
                          >
                            <Settings2 size={18} />
                          </button>
                        </td>
                      </tr>
                    )),
                  )}
                </tbody>
              </table>
            </div>
            {trText(
              !items.length ? (
                <div className="empty-state">
                  <h3>
                    {trText(
                      servers.length ? tr('Серверы не найдены') : tr('Подключите первый сервер'),
                    )}
                  </h3>
                  <p>
                    {trText(
                      servers.length
                        ? tr('Попробуйте другой запрос.')
                        : tr(
                            'Добавьте узел и установите на нём агент. Его ОС и оборудование появятся автоматически.',
                          ),
                    )}
                  </p>
                </div>
              ) : null,
            )}
            <div className="table-footer">
              {trText(items.length)}
              {' ' + tr('из') + ' '}
              {trText(servers.length)}
              {' ' + tr('серверов')}
              <span>Linux + Windows</span>
            </div>
          </Panel>
        ),
      )}
      {trText(
        editor ? (
          <ServerEditor
            item={editor === 'new' ? undefined : editor}
            onClose={() => setEditor(null)}
          />
        ) : null,
      )}
    </PageMotion>
  );
}
