'use client';
import { useLocale, tr, trText } from './locale';
import { useState, useEffect } from 'react';
import { useSearchParams } from 'next/navigation';
import { Puzzle, Upload, Trash2, Check, Settings2 } from 'lucide-react';
import { api } from '@/lib/monitor';
import { type ModuleManifest, type ModuleAssignment } from '@/lib/modules';
import { ModuleDataView } from './module-data-view';
import { ConfigureExtension, ExtensionModuleView } from './module-extensions';
import type { InstalledModule, ExtensionPackage } from '@/lib/module-extensions';
import { useMonitor } from './provider';
import { Marketplace, ModuleUpdates } from './marketplace';
import { PageHeading, PageMotion, Panel, Modal, Empty, Skeleton } from './ui';
function ConfigureModule({ module, onClose }: { module: ModuleManifest; onClose: () => void }) {
  useLocale();
  const { servers, server, notify } = useMonitor();
  const [node, setNode] = useState(server?.id ?? servers[0]?.id ?? ''),
    [config, setConfig] = useState<ModuleAssignment>({
      enabled: false,
      config: Object.fromEntries(
        module.fields.map((f) => [
          f.key,
          f.default ?? (f.type === 'boolean' ? false : f.type === 'number' ? 0 : ''),
        ]),
      ),
    }),
    [error, setError] = useState(''),
    [busy, setBusy] = useState(false),
    [loadedNode, setLoadedNode] = useState('');
  const ready = !!node && loadedNode === node;
  useEffect(() => {
    if (!node) return;
    const abort = new AbortController();
    api<Record<string, ModuleAssignment>>(`/servers/${encodeURIComponent(node)}/modules`, {
      signal: abort.signal,
    })
      .then((items) => {
        if (abort.signal.aborted) return;
        setConfig(
          items[module.id] ?? {
            enabled: false,
            config: Object.fromEntries(
              module.fields.map((f) => [
                f.key,
                f.default ?? (f.type === 'boolean' ? false : f.type === 'number' ? 0 : ''),
              ]),
            ),
          },
        );
        setLoadedNode(node);
      })
      .catch((e) => {
        if (!abort.signal.aborted) setError(e.message);
      });
    return () => abort.abort();
  }, [node, module]);
  async function save() {
    if (!ready || busy) return;
    setBusy(true);
    setError('');
    try {
      await api(`/servers/${encodeURIComponent(node)}/modules/${encodeURIComponent(module.id)}`, {
        method: 'PUT',
        body: JSON.stringify(config),
      });
      notify(tr('Настройки модуля сохранены'));
      window.dispatchEvent(new Event('pulse:modules'));
      onClose();
    } catch (e) {
      setError(e instanceof Error ? e.message : tr('Не удалось сохранить модуль'));
    } finally {
      setBusy(false);
    }
  }
  return (
    <Modal
      title={module.name}
      subtitle={tr('Настройка модуля на выбранном сервере')}
      onClose={onClose}
    >
      <div className="drawer-body" aria-busy={!ready}>
        <label className="field">
          {tr('Сервер')}
          <select
            value={node}
            disabled={busy}
            onChange={(e) => {
              setLoadedNode('');
              setError('');
              setConfig({
                enabled: false,
                config: Object.fromEntries(
                  module.fields.map((f) => [
                    f.key,
                    f.default ?? (f.type === 'boolean' ? false : f.type === 'number' ? 0 : ''),
                  ]),
                ),
              });
              setNode(e.target.value);
            }}
          >
            {trText(
              servers.map((s) => (
                <option key={s.id} value={s.id}>
                  {s.name}
                </option>
              )),
            )}
          </select>
        </label>
        {!ready && !error ? (
          <p className="muted" role="status">
            {tr('Загрузка данных…')}
          </p>
        ) : null}
        <div className="form-note">
          <p>
            {tr(
              'Этот модуль запускает код на сервере с правами агента. Устанавливайте модули авторов, которым доверяете.',
            )}
          </p>
        </div>
        {trText(
          module.permissions?.length ? (
            <div className="module-permissions">
              <strong>{tr('Необходимый доступ')}</strong>
              {trText(module.permissions.map((p) => <p key={p}>{trText(p)}</p>))}
            </div>
          ) : null,
        )}
        {trText(
          module.fields.map((field) => (
            <label className="field" key={field.key}>
              {trText(field.label)}
              {trText(
                field.type === 'boolean' ? (
                  <input
                    type="checkbox"
                    disabled={!ready || busy}
                    checked={!!config.config[field.key]}
                    onChange={(e) =>
                      setConfig({
                        ...config,
                        config: { ...config.config, [field.key]: e.target.checked },
                      })
                    }
                  />
                ) : (
                  <input
                    disabled={!ready || busy}
                    type={
                      field.type === 'password'
                        ? 'password'
                        : field.type === 'number'
                          ? 'number'
                          : 'text'
                    }
                    autoComplete="off"
                    required={field.required}
                    value={String(config.config[field.key] ?? '')}
                    onChange={(e) =>
                      setConfig({
                        ...config,
                        config: {
                          ...config.config,
                          [field.key]:
                            field.type === 'number' ? Number(e.target.value) : e.target.value,
                        },
                      })
                    }
                  />
                ),
              )}
            </label>
          )),
        )}
        <label className="module-enable">
          <input
            type="checkbox"
            disabled={!ready || busy}
            checked={config.enabled}
            onChange={(e) => setConfig({ ...config, enabled: e.target.checked })}
          />
          {tr('Включить модуль на этом сервере')}
        </label>
        {trText(
          error ? (
            <p role="alert" className="form-error">
              {trText(error)}
            </p>
          ) : null,
        )}
      </div>
      <div className="drawer-footer">
        <button className="button" onClick={onClose}>
          {tr('Отмена')}
        </button>
        <button className="button primary" disabled={busy || !ready} onClick={() => void save()}>
          <Check size={16} />
          {tr('Сохранить')}
        </button>
      </div>
    </Modal>
  );
}
export function Modules() {
  useLocale();
  const { servers, server, notify } = useMonitor();
  const params = useSearchParams();
  const [modules, setModules] = useState<InstalledModule[]>([]),
    [revision, setRevision] = useState(0),
    [error, setError] = useState(''),
    [loading, setLoading] = useState(true),
    [configuring, setConfiguring] = useState<InstalledModule | null>(null),
    [confirm, setConfirm] = useState<InstalledModule | null>(null);
  const [uploading, setUploading] = useState(false);
  useEffect(() => {
    const abort = new AbortController();
    Promise.all([
      api<ModuleManifest[]>('/modules', { signal: abort.signal }),
      api<ExtensionPackage[]>('/extensions', { signal: abort.signal }),
    ])
      .then((items) => {
        setModules(items.flat());
        setLoading(false);
      })
      .catch((e) => {
        if (!abort.signal.aborted) {
          setError(e.message);
          setLoading(false);
        }
      });
    const changed = () => setRevision((n) => n + 1);
    window.addEventListener('pulse:modules', changed);
    return () => {
      abort.abort();
      window.removeEventListener('pulse:modules', changed);
    };
  }, [revision]);
  async function upload(file: File) {
    if (uploading) return;
    setUploading(true);
    try {
      if (file.size > 68 * 1024 * 1024) throw new Error(tr('Пакет модуля слишком большой'));
      const body = await file.text();
      const value = JSON.parse(body);
      await api(value.schemaVersion === 2 ? '/extensions' : '/modules', { method: 'POST', body });
      setRevision((n) => n + 1);
      notify(tr('Модуль установлен'));
      window.dispatchEvent(new Event('pulse:modules'));
      setError('');
    } catch (e) {
      setError(e instanceof Error ? e.message : tr('Не удалось установить модуль'));
    } finally {
      setUploading(false);
    }
  }
  const [view, setView] = useState<'installed' | 'marketplace'>('installed');
  const selected = modules.find((m) => m.id === params.get('id'));
  return (
    <PageMotion>
      <PageHeading
        title={selected?.name ?? tr('Модули')}
        subtitle={selected?.description ?? tr('Расширяйте мониторинг без изменения ядра.')}
        actions={
          <label className="button primary module-upload">
            <Upload size={16} />
            {tr(uploading ? 'Устанавливаем…' : 'Установить / обновить JSON')}
            <input
              type="file"
              accept=".json"
              disabled={uploading}
              aria-label={tr('Установить модуль')}
              onChange={(e) => {
                const file = e.target.files?.[0];
                if (file) void upload(file);
                e.target.value = '';
              }}
            />
          </label>
        }
      />
      {uploading && (
        <div className="module-install-status" role="status" aria-live="polite">
          <span>{tr('Проверяем пакет и устанавливаем модуль. Не закрывайте страницу.')}</span>
          <progress aria-label={tr('Установка модуля')} />
        </div>
      )}
      {!selected && (
        <div className="button-row module-tabs" role="group" aria-label={tr('Разделы модулей')}>
          <button
            className={`button ${view === 'installed' ? 'primary' : ''}`}
            aria-pressed={view === 'installed'}
            onClick={() => setView('installed')}
          >
            {tr('Установленные')}
          </button>
          <button
            className={`button ${view === 'marketplace' ? 'primary' : ''}`}
            aria-pressed={view === 'marketplace'}
            onClick={() => setView('marketplace')}
          >
            {tr('Маркетплейс')}
          </button>
        </div>
      )}
      {view === 'installed' && !selected && <ModuleUpdates />}
      {view === 'marketplace' && !selected && <Marketplace installed={modules} />}
      {trText(
        error ? (
          <p className="form-error" role="alert">
            {trText(error)}
          </p>
        ) : null,
      )}
      {trText(
        view === 'marketplace' && !selected ? null : selected ? (
          <>
            <button
              className="button"
              disabled={
                !servers.length &&
                !(
                  selected?.schemaVersion === 2 &&
                  selected.components.some((c) => c.location === 'monitor')
                )
              }
              onClick={() => setConfiguring(selected)}
            >
              <Settings2 size={16} />
              {tr('Настроить')}
            </button>
            {selected.schemaVersion === 2 ? (
              <ExtensionModuleView
                key={`${selected.id}-${selected.version}-${server?.id}-${revision}`}
                module={selected}
              />
            ) : (
              <ModuleDataView
                key={`${selected.id}-${selected.version}-${server?.id}-${revision}`}
                module={selected}
                endpoint={`/servers/${encodeURIComponent(server?.id ?? '')}/modules/${encodeURIComponent(selected.id)}/data`}
              />
            )}
          </>
        ) : loading ? (
          <Skeleton />
        ) : !modules.length ? (
          <Empty
            title={tr('Подключите первый модуль')}
            text={tr(
              'Установите пакет разработчика, выберите сервер и включите сбор данных. Docker-модуль доступен в отдельном релизе pulse-module-docker.',
            )}
          />
        ) : (
          <div className="module-list">
            {trText(
              modules.map((m) => (
                <Panel key={m.id}>
                  <div className="panel-title">
                    <h2>
                      <Puzzle size={18} />
                      {m.name}
                    </h2>
                    <span className="tag">{trText(m.version)}</span>
                  </div>
                  <div className="module-info">
                    <p>{trText(m.description)}</p>
                    <small>
                      {trText(m.author)} ·{' '}
                      {trText(
                        m.schemaVersion === 1
                          ? Object.keys(m.binaries).join(', ')
                          : m.components
                              .map(
                                (c) =>
                                  c.location +
                                  ': ' +
                                  c.id +
                                  ' · ' +
                                  (c.runtime === 'wasip1' ? 'WASI' : 'native'),
                              )
                              .join(', '),
                      )}
                    </small>
                    <div className="button-row">
                      <a className="button" href={`/modules/?id=${encodeURIComponent(m.id)}`}>
                        {tr('Открыть')}
                      </a>
                      <button
                        className="button"
                        disabled={
                          !servers.length &&
                          !(
                            m.schemaVersion === 2 &&
                            m.components.some((c) => c.location === 'monitor')
                          )
                        }
                        onClick={() => setConfiguring(m)}
                      >
                        {tr('Настроить')}
                      </button>
                      <button
                        className="icon-button"
                        aria-label={tr('Удалить модуль')}
                        onClick={() => setConfirm(m)}
                      >
                        <Trash2 size={16} />
                      </button>
                    </div>
                  </div>
                </Panel>
              )),
            )}
          </div>
        ),
      )}
      {trText(
        configuring ? (
          configuring.schemaVersion === 2 ? (
            <ConfigureExtension module={configuring} onClose={() => setConfiguring(null)} />
          ) : (
            <ConfigureModule module={configuring} onClose={() => setConfiguring(null)} />
          )
        ) : null,
      )}
      {trText(
        confirm ? (
          <Modal
            title={tr('Удалить модуль?')}
            subtitle={confirm.name}
            onClose={() => setConfirm(null)}
          >
            <div className="drawer-body">
              <p>{tr('Сбор прекратится на всех узлах. Сохранённые данные останутся.')}</p>
            </div>
            <div className="drawer-footer">
              <button className="button" onClick={() => setConfirm(null)}>
                {tr('Отмена')}
              </button>
              <button
                className="button primary"
                onClick={async () => {
                  try {
                    await api(
                      `${confirm.schemaVersion === 2 ? '/extensions' : '/modules'}/${encodeURIComponent(confirm.id)}`,
                      { method: 'DELETE' },
                    );
                    setConfirm(null);
                    setRevision((n) => n + 1);
                    window.dispatchEvent(new Event('pulse:modules'));
                  } catch (e) {
                    setError(e instanceof Error ? e.message : tr('Не удалось удалить модуль'));
                  }
                }}
              >
                {tr('Удалить')}
              </button>
            </div>
          </Modal>
        ) : null,
      )}
    </PageMotion>
  );
}
