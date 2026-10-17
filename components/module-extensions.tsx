'use client';
import { useState, useEffect, useRef } from 'react';
import { api } from '@/lib/monitor';
import {
  type ExtensionPackage,
  type ExtensionComponent,
  type ExtensionConfiguration,
  type ExtensionTrigger,
  type ExtensionView,
  type ExtensionOperation,
  componentEndpoint,
  operationMatches,
  actionAvailable,
} from '@/lib/module-extensions';
import { type ModuleManifest, formatModuleValue } from '@/lib/modules';
import { useLocale, tr, trText } from './locale';
import { useMonitor } from './provider';
import { Modal, Skeleton } from './ui';
import { ModuleDataView } from './module-data-view';
const capabilities = [
  'state.read',
  'state.write',
  'events.receive',
  'network.outbound',
  'actions.execute',
];
function defaultConfig(component: ExtensionComponent): ExtensionConfiguration {
  return {
    enabled: false,
    grants: [],
    config: Object.fromEntries(
      component.fields
        .filter((f) => f.type !== 'password')
        .map((f) => [
          f.key,
          f.default ?? (f.type === 'boolean' ? false : f.type === 'number' ? 0 : ''),
        ]),
    ),
  };
}
function ConfigurationFields({
  module,
  component,
  server,
  onClose,
}: {
  module: ExtensionPackage;
  component: ExtensionComponent;
  server: string;
  onClose: () => void;
}) {
  const [config, setConfig] = useState(() => defaultConfig(component));
  const [ready, setReady] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const { notify } = useMonitor();
  useEffect(() => {
    const abort = new AbortController();
    api<ExtensionConfiguration>(
      componentEndpoint(module.id, component.id, server, 'configuration'),
      { signal: abort.signal },
    )
      .then((value) => {
        if (!abort.signal.aborted) {
          setConfig({
            ...defaultConfig(component),
            ...value,
            config: { ...defaultConfig(component).config, ...value.config },
          });
          setReady(true);
        }
      })
      .catch((e) => {
        if (!abort.signal.aborted) setError(e.message);
      });
    return () => abort.abort();
  }, [module.id, component, server]);
  async function save() {
    if (!ready || busy) return;
    setBusy(true);
    setError('');
    try {
      await api(componentEndpoint(module.id, component.id, server, 'configuration'), {
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
  if (!ready)
    return error ? (
      <p className="form-error" role="alert">
        {error}
      </p>
    ) : (
      <Skeleton />
    );
  return (
    <>
      <fieldset disabled={busy} className="extension-fields">
        <label className="field">
          <span>{tr('Включить компонент')}</span>
          <input
            type="checkbox"
            checked={config.enabled}
            onChange={(e) => setConfig({ ...config, enabled: e.target.checked })}
          />
        </label>
        {component.fields.map((f) => (
          <label className="field" key={f.key}>
            {trText(f.label || f.key)}
            {f.type === 'boolean' ? (
              <input
                type="checkbox"
                checked={!!config.config[f.key]}
                onChange={(e) =>
                  setConfig({ ...config, config: { ...config.config, [f.key]: e.target.checked } })
                }
              />
            ) : (
              <input
                type={f.type === 'password' ? 'password' : f.type === 'number' ? 'number' : 'text'}
                autoComplete={f.type === 'password' ? 'new-password' : 'off'}
                value={String(config.config[f.key] ?? '')}
                placeholder={
                  f.type === 'password' && config.secrets?.includes(f.key)
                    ? tr('Секрет сохранён. Оставьте пустым, чтобы сохранить.')
                    : (trText(f.label || f.key) as string)
                }
                onChange={(e) => {
                  const next = { ...config.config };
                  if (f.type === 'password' && e.target.value === '') delete next[f.key];
                  else next[f.key] = f.type === 'number' ? Number(e.target.value) : e.target.value;
                  setConfig({ ...config, config: next });
                }}
              />
            )}
            {f.type === 'password' && config.secrets?.includes(f.key) && (
              <span className="extension-secret-clear">
                <input
                  type="checkbox"
                  checked={config.clearSecrets?.includes(f.key) ?? false}
                  onChange={(e) =>
                    setConfig({
                      ...config,
                      clearSecrets: e.target.checked
                        ? [...(config.clearSecrets ?? []), f.key]
                        : (config.clearSecrets ?? []).filter((k) => k !== f.key),
                    })
                  }
                />
                {tr('Удалить сохранённый секрет')}
              </span>
            )}
          </label>
        ))}
        <div className="form-section">
          <h3>{tr('Разрешения компонента')}</h3>
          <p className="muted">
            {tr(
              'Разрешения действуют только для выбранного компонента и узла. Нативный код не изолирован от операционной системы.',
            )}
          </p>
          {capabilities
            .filter(
              (cap) =>
                component.requires.includes(cap) ||
                (cap === 'actions.execute' && component.triggers.some((t) => t.mutating)),
            )
            .map((cap) => (
              <label className="extension-grant" key={cap}>
                <input
                  type="checkbox"
                  checked={config.grants.includes(cap)}
                  onChange={(e) =>
                    setConfig({
                      ...config,
                      grants: e.target.checked
                        ? [...config.grants, cap]
                        : config.grants.filter((g) => g !== cap),
                    })
                  }
                />
                <span>
                  {tr(cap)}
                  {component.requires.includes(cap) ? ` · ${tr('Обязательно')}` : ''}
                </span>
              </label>
            ))}
        </div>
      </fieldset>
      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}
      <div className="drawer-footer">
        <button className="button" disabled={busy} onClick={onClose}>
          {tr('Отмена')}
        </button>
        <button className="button primary" disabled={busy} onClick={() => void save()}>
          {busy ? tr('Сохранение…') : tr('Сохранить')}
        </button>
      </div>
    </>
  );
}
export function ConfigureExtension({
  module,
  onClose,
}: {
  module: ExtensionPackage;
  onClose: () => void;
}) {
  useLocale();
  const { server, servers } = useMonitor();
  const [componentId, setComponentId] = useState(module.components[0].id);
  const [node, setNode] = useState(server?.id ?? servers[0]?.id ?? '');
  const component = module.components.find((c) => c.id === componentId)!;
  const scope = component.location === 'monitor' ? '' : node;
  return (
    <Modal title={module.name} subtitle={tr('Настройка компонента')} onClose={onClose}>
      <div className="drawer-body">
        <p className="form-note">
          {tr(
            'Этот модуль запускает нативный код. Устанавливайте только модули авторов, которым доверяете.',
          )}
        </p>
        {module.permissions.map((p) => (
          <p className="muted" key={p}>
            {trText(p)}
          </p>
        ))}
        <label className="field">
          {tr('Компонент')}
          <select value={componentId} onChange={(e) => setComponentId(e.target.value)}>
            {module.components.map((c) => (
              <option key={c.id} value={c.id}>
                {c.id} · {c.location}
              </option>
            ))}
          </select>
        </label>
        {component.location === 'agent' && (
          <label className="field">
            {tr('Сервер')}
            <select value={node} onChange={(e) => setNode(e.target.value)}>
              {servers.map((s) => (
                <option key={s.id} value={s.id}>
                  {s.name}
                </option>
              ))}
            </select>
          </label>
        )}
        {component.location === 'monitor' || node ? (
          <ConfigurationFields
            key={`${component.id}/${scope}`}
            module={module}
            component={component}
            server={scope}
            onClose={onClose}
          />
        ) : (
          <p>{tr('Подключите сервер')}</p>
        )}
      </div>
    </Modal>
  );
}
function delay(signal: AbortSignal) {
  return new Promise<void>((resolve) => {
    if (signal.aborted) {
      resolve();
      return;
    }
    const done = () => {
      clearTimeout(timer);
      signal.removeEventListener('abort', done);
      resolve();
    };
    const timer = setTimeout(done, 1000);
    signal.addEventListener('abort', done, { once: true });
  });
}
type Selection = {
  action: ExtensionTrigger;
  row: Record<string, unknown>;
  id: string;
  input?: Record<string, string | number | boolean>;
};
function ExtensionComponentView({
  module,
  component,
}: {
  module: ExtensionPackage;
  component: ExtensionComponent;
}) {
  const { server } = useMonitor();
  const scope = component.location === 'monitor' ? '' : (server?.id ?? '');
  const [config, setConfig] = useState<ExtensionConfiguration | null>(null);
  const [selection, setSelection] = useState<Selection | null>(null);
  const [operation, setOperation] = useState<ExtensionOperation | null>(null);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const [revision, setRevision] = useState(0);
  const inflight = useRef(false);
  const controller = useRef<AbortController | null>(null);
  useEffect(() => {
    const abort = new AbortController();
    api<ExtensionConfiguration>(
      componentEndpoint(module.id, component.id, scope, 'configuration'),
      { signal: abort.signal },
    )
      .then((v) => {
        if (!abort.signal.aborted) setConfig(v);
      })
      .catch((e) => {
        if (!abort.signal.aborted) setError(e.message);
      });
    return () => {
      abort.abort();
      controller.current?.abort();
    };
  }, [module.id, component.id, scope]);
  async function execute(target: Selection) {
    if (inflight.current || !config?.enabled) return;
    inflight.current = true;
    setBusy(true);
    setOperation(null);
    setError('');
    controller.current?.abort();
    const abort = new AbortController();
    controller.current = abort;
    let timedOut = false;
    const timeout = setTimeout(() => {
      timedOut = true;
      abort.abort();
    }, 75000);
    try {
      const input = target.action.inputKey
        ? { [target.action.inputKey]: target.id }
        : (target.input ?? {});
      const op = await api<ExtensionOperation>(
        componentEndpoint(module.id, component.id, scope, 'operations'),
        {
          method: 'POST',
          body: JSON.stringify({ action: target.action.action, input }),
          signal: abort.signal,
        },
      );
      const expected = {
        id: op.id,
        moduleId: module.id,
        componentId: component.id,
        serverId: scope,
        action: target.action.action!,
      };
      let current = op;
      while (!abort.signal.aborted) {
        if (!operationMatches(current, expected))
          throw new Error(tr('Результат относится к другому запросу'));
        if (
          current.status === 'succeeded' &&
          target.action.inputKey &&
          current.data?.[target.action.inputKey] !== target.id
        )
          throw new Error(tr('Результат относится к другому объекту'));
        setOperation(current);
        if (!['pending', 'running'].includes(current.status)) {
          setRevision((n) => n + 1);
          return;
        }
        await delay(abort.signal);
        if (abort.signal.aborted) return;
        current = await api<ExtensionOperation>(`/module-operations/${encodeURIComponent(op.id)}`, {
          signal: abort.signal,
        });
      }
    } catch (e) {
      if (!abort.signal.aborted) setOperation(null);
      if (!abort.signal.aborted) setError(e instanceof Error ? e.message : tr('Ошибка операции'));
    } finally {
      clearTimeout(timeout);
      if (!abort.signal.aborted || timedOut) {
        if (timedOut)
          setError(tr('Статус операции не получен. Проверьте состояние перед повтором.'));
        inflight.current = false;
        setBusy(false);
      }
    }
  }
  function select(action: ExtensionTrigger, row: Record<string, unknown>, rowKey: string) {
    if (
      !actionAvailable(
        action,
        row,
        rowKey,
        !!config?.enabled,
        config?.grants ?? [],
        inflight.current,
      )
    )
      return;
    const target = { action, row, id: String(row[rowKey]) };
    setSelection(target);
    setOperation(null);
    setError('');
    if (!action.mutating) void execute(target);
  }
  const manifest = { id: `${module.id}-${component.id}`, views: component.views ?? [] } as Pick<
    ModuleManifest,
    'id' | 'views'
  >;
  const rowControls = (view: ModuleManifest['views'][number], row: Record<string, unknown>) => {
    const v = view as ExtensionView;
    return (
      <div className="button-row extension-row-actions">
        {(v.actions ?? []).map((id) => {
          const action = component.triggers.find((t) => t.type === 'manual' && t.action === id);
          return action ? (
            <button
              className="button"
              key={id}
              disabled={
                !actionAvailable(
                  action,
                  row,
                  v.rowKey ?? '',
                  !!config?.enabled,
                  config?.grants ?? [],
                  busy,
                )
              }
              onClick={() => select(action, row, v.rowKey ?? '')}
            >
              {trText(action.label || id)}
            </button>
          ) : null;
        })}
      </div>
    );
  };
  return (
    <section className="extension-component">
      <div className="panel-title">
        <h2>{component.id}</h2>
        <span className="tag">{component.location}</span>
      </div>
      {!selection && error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}
      <div className="button-row">
        {component.triggers
          .filter(
            (t) =>
              t.type === 'manual' &&
              !(component.views ?? []).some((v) => v.actions?.includes(t.action ?? '')),
          )
          .map((action) => (
            <button
              key={action.action}
              className="button"
              disabled={
                !config?.enabled ||
                busy ||
                (!!action.mutating && !config?.grants.includes('actions.execute'))
              }
              onClick={() => {
                const target = {
                  action,
                  row: {},
                  id: '',
                  input: Object.fromEntries(
                    (action.inputFields ?? []).map((f) => [
                      f.key,
                      f.type === 'boolean' ? false : f.type === 'number' ? 0 : '',
                    ]),
                  ),
                };
                setSelection(target);
                setOperation(null);
                setError('');
                if (!action.mutating && !action.inputFields?.length) void execute(target);
              }}
            >
              {trText(action.label || action.action)}
            </button>
          ))}
      </div>
      <ModuleDataView
        module={manifest}
        endpoint={componentEndpoint(module.id, component.id, scope, 'data')}
        monitorScope={component.location === 'monitor'}
        revision={revision}
        rowControls={rowControls}
        onRowSelect={(view, row) => {
          const v = view as ExtensionView;
          const action = component.triggers.find(
            (t) =>
              t.type === 'manual' && t.resultView === 'logs' && v.actions?.includes(t.action ?? ''),
          );
          if (action) select(action, row, v.rowKey ?? '');
        }}
      />
      {selection && (
        <Modal
          wide
          title={trText(selection.action.label || selection.action.action) as string}
          subtitle={`${component.location === 'monitor' ? tr('Монитор') : (server?.name ?? '')} · ${String(selection.row.name ?? selection.id)}`}
          onClose={() => {
            if (!busy) {
              setSelection(null);
              setError('');
            }
          }}
        >
          <div className="drawer-body">
            <code className="extension-target">{selection.id}</code>
            {(selection.action.mutating ||
              (selection.input && selection.action.inputFields?.length)) &&
            !operation &&
            !busy &&
            !error ? (
              <>
                <p>
                  {tr(
                    'Действие может прервать работу приложения. Выполнить его для выбранного объекта?',
                  )}
                </p>
                {selection.input &&
                  selection.action.inputFields?.map((f) => (
                    <label className="field" key={f.key}>
                      {trText(f.label || f.key)}
                      <input
                        type={
                          f.type === 'boolean'
                            ? 'checkbox'
                            : f.type === 'password'
                              ? 'password'
                              : f.type === 'number'
                                ? 'number'
                                : 'text'
                        }
                        value={
                          f.type === 'boolean' ? undefined : String(selection.input?.[f.key] ?? '')
                        }
                        checked={f.type === 'boolean' ? !!selection.input?.[f.key] : undefined}
                        onChange={(e) =>
                          setSelection({
                            ...selection,
                            input: {
                              ...selection.input,
                              [f.key]:
                                f.type === 'boolean'
                                  ? e.target.checked
                                  : f.type === 'number'
                                    ? Number(e.target.value)
                                    : e.target.value,
                            },
                          })
                        }
                      />
                    </label>
                  ))}
                <button className="button primary" onClick={() => void execute(selection)}>
                  {tr('Подтвердить действие')}
                </button>
              </>
            ) : null}
            {busy && (
              <>
                <p role="status">{tr('Операция выполняется…')}</p>
                <Skeleton />
              </>
            )}
            {error && (
              <p className="form-error" role="alert">
                {error}
              </p>
            )}
            {operation && !busy && (
              <>
                <p role="status">{tr(`operation.${operation.status}`)}</p>
                {operation.error && <p className="form-error">{operation.error}</p>}
                {operation.status === 'succeeded' && selection.action.resultView === 'logs' && (
                  <div className="module-log-feed mono" aria-label={tr('Логи выбранного объекта')}>
                    {Array.isArray(operation.data?.logs) && operation.data.logs.length ? (
                      operation.data.logs.map((line, i) => (
                        <pre key={i}>{formatModuleValue(line)}</pre>
                      ))
                    ) : (
                      <p>{tr('Логи пока пусты')}</p>
                    )}
                  </div>
                )}
                {operation.status === 'succeeded' &&
                  selection.action.resultView &&
                  selection.action.resultView !== 'logs' && (
                    <pre className="extension-result">
                      {JSON.stringify(operation.data, null, 2)}
                    </pre>
                  )}
              </>
            )}
            {!busy && selection.action.resultView === 'logs' && (
              <button className="button" onClick={() => void execute(selection)}>
                {tr('Обновить')}
              </button>
            )}
          </div>
        </Modal>
      )}
    </section>
  );
}
export function ExtensionModuleView({ module }: { module: ExtensionPackage }) {
  useLocale();
  const { server } = useMonitor();
  return (
    <div className="module-views">
      {module.components.map((component) => (
        <ExtensionComponentView
          key={`${component.id}/${component.location === 'agent' ? server?.id : ''}`}
          module={module}
          component={component}
        />
      ))}
    </div>
  );
}
