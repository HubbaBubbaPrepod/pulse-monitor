'use client';
import { useLocale, tr, trText } from './locale';
import { useState } from 'react';
import { Download, Upload, Copy, Trash2, RotateCcw, ChevronDown } from 'lucide-react';
import {
  builtinThemes,
  colorKeys,
  validateTheme,
  defaultOptions,
  contrastRatio,
  type ThemeDefinition,
  type ThemeColor,
} from '@/lib/themes';
import { download } from '@/lib/data';
import { useMonitor, type Preferences } from './provider';
const labels: Record<string, string> = {
  surface: 'Панели',
  'surface-hover': 'Наведение',
  raised: 'Выделение',
  muted: 'Вторичный текст',
  subtle: 'Подписи',
  border: 'Границы',
  track: 'Фон шкал',
  'chart-primary': 'График 1',
  'chart-secondary': 'График 2',
  'chart-tertiary': 'График 3',
};
function Row({
  title,
  description,
  children,
}: {
  title: string;
  description?: string;
  children: React.ReactNode;
}) {
  useLocale();
  return (
    <div className="appearance-row">
      <div>
        <strong>{trText(title)}</strong>
        {trText(description ? <p>{trText(description)}</p> : null)}
      </div>
      <div className="appearance-control">{trText(children)}</div>
    </div>
  );
}
function Color({
  value,
  onChange,
  label,
}: {
  value: string;
  onChange: (v: string) => void;
  label: string;
}) {
  useLocale();
  return (
    <label className="color-pill">
      <input
        aria-label={label}
        type="color"
        value={value}
        onChange={(e) => onChange(e.target.value)}
      />
      <span>{trText(value.toUpperCase())}</span>
    </label>
  );
}
function Switch({
  value,
  onChange,
  label,
}: {
  value: boolean;
  onChange: () => void;
  label: string;
}) {
  useLocale();
  return (
    <button
      className={`switch ${value ? 'on' : ''}`}
      role="switch"
      aria-label={label}
      aria-checked={value}
      onClick={onChange}
    >
      <span />
    </button>
  );
}
export function ThemeEditor() {
  useLocale();
  const { preferences, setPreferences, notify, theme: mode } = useMonitor();
  const [error, setError] = useState('');
  const themes = [...builtinThemes, ...preferences.themes],
    a = preferences.appearance ?? {
      mode,
      lightThemeId: 'light',
      darkThemeId: 'dark',
      separate: false,
      motionMode: preferences.motion ? 'system' : 'on',
    };
  const selected =
    a.mode === 'system' ? (mode === 'dark' ? a.darkThemeId : a.lightThemeId) : preferences.themeId;
  const current = themes.find((t) => t.id === selected) ?? builtinThemes[0],
    o = { ...defaultOptions, ...current.options };
  function profiles(theme: ThemeDefinition) {
    return {
      lightThemeId: theme.mode === 'light' ? theme.id : a.separate ? a.lightThemeId : 'light',
      darkThemeId: theme.mode === 'dark' ? theme.id : a.separate ? a.darkThemeId : 'dark',
    };
  }
  function choose(id: string) {
    const t = themes.find((t) => t.id === id)!;
    setPreferences({
      ...preferences,
      themeId: id,
      appearance: { ...a, mode: a.mode === 'system' ? 'system' : t.mode, ...profiles(t) },
    });
  }
  function patch(update: Partial<ThemeDefinition>) {
    try {
      const custom = !builtinThemes.some((t) => t.id === current.id);
      if (!custom && preferences.themes.length >= 16)
        throw new Error(tr('Максимум 16 пользовательских тем'));
      const theme = validateTheme({
        ...current,
        ...update,
        id: custom ? current.id : crypto.randomUUID(),
        name: update.name ?? (custom ? current.name : current.name.slice(0, 55) + ' · Custom'),
      });
      setPreferences({
        ...preferences,
        themeId: theme.id,
        appearance: { ...a, ...profiles(theme) },
        themes: [...preferences.themes.filter((t) => t.id !== theme.id), theme],
      });
      setError('');
    } catch (e) {
      setError(e instanceof Error ? e.message : tr('Некорректная тема'));
    }
  }
  function options(update: Partial<typeof o>) {
    patch({ options: { ...o, ...update } });
  }
  function appearance(update: Partial<NonNullable<Preferences['appearance']>>) {
    const next = { ...a, ...update };
    if (update.separate === false) {
      next.lightThemeId = mode === 'light' ? current.id : 'light';
      next.darkThemeId = mode === 'dark' ? current.id : 'dark';
    }
    setPreferences({
      ...preferences,
      appearance: next,
      motion: next.motionMode !== 'on',
      themeId:
        next.mode === 'light'
          ? next.lightThemeId
          : next.mode === 'dark'
            ? next.darkThemeId
            : preferences.themeId,
    });
  }
  function reset() {
    setPreferences({
      ...preferences,
      themeId: mode === 'dark' ? 'dark' : 'light',
      appearance: {
        mode,
        lightThemeId: 'light',
        darkThemeId: 'dark',
        separate: false,
        motionMode: 'system',
      },
      motion: true,
    });
    setError('');
  }
  return (
    <div className="appearance-editor">
      <div className="appearance-card mode-card">
        <Row title={tr('Режим')}>
          <div className="mode-choices">
            {trText(
              (['system', 'light', 'dark'] as const).map((value) => (
                <button
                  key={value}
                  aria-label={
                    value === 'system'
                      ? tr('Системная тема')
                      : value === 'light'
                        ? tr('Светлая тема')
                        : tr('Тёмная тема')
                  }
                  aria-pressed={a.mode === value}
                  title={
                    value === 'system'
                      ? tr('Системная')
                      : value === 'light'
                        ? tr('Светлая')
                        : tr('Тёмная')
                  }
                  className={`mode-preview ${value} ${a.mode === value ? 'selected' : ''}`}
                  onClick={() => appearance({ mode: value })}
                >
                  <span className="preview-sidebar" />
                  <span className="preview-content">
                    <i />
                    <i />
                    <i />
                    <b />
                    <i />
                  </span>
                </button>
              )),
            )}
          </div>
        </Row>
      </div>
      <div className="appearance-card">
        <Row title={tr('Тема')}>
          <button
            className="icon-button"
            title={tr('Экспорт темы')}
            aria-label={tr('Экспорт темы')}
            onClick={() =>
              download(current.name + '.json', JSON.stringify(current, null, 2), 'application/json')
            }
          >
            <Download size={17} />
          </button>
          <label className="icon-button theme-upload" title={tr('Импорт темы')}>
            <Upload size={17} />
            <input
              aria-label={tr('Импорт темы')}
              type="file"
              accept=".json"
              onChange={async (e) => {
                try {
                  const file = e.target.files?.[0];
                  if (!file) return;
                  if (file.size > 16384) throw new Error(tr('Файл темы слишком большой'));
                  const t = validateTheme(JSON.parse(await file.text()));
                  if (preferences.themes.length >= 16)
                    throw new Error(tr('Максимум 16 пользовательских тем'));
                  const imported = { ...t, id: crypto.randomUUID() };
                  setPreferences({
                    ...preferences,
                    themeId: imported.id,
                    themes: [...preferences.themes, imported],
                    appearance: { ...a, mode: imported.mode, ...profiles(imported) },
                  });
                  setError('');
                  e.target.value = '';
                  notify(tr('Тема импортирована'));
                } catch (e) {
                  setError(e instanceof Error ? e.message : tr('Некорректная тема'));
                }
              }}
            />
          </label>
          <button
            className="icon-button"
            title={tr('Дублировать тему')}
            aria-label={tr('Дублировать тему')}
            onClick={() => {
              if (preferences.themes.length >= 16) {
                setError(tr('Максимум 16 пользовательских тем'));
                return;
              }
              const t = {
                ...current,
                id: crypto.randomUUID(),
                name: current.name.slice(0, 57) + ' · Copy',
              };
              setPreferences({
                ...preferences,
                themeId: t.id,
                appearance: { ...a, ...profiles(t) },
                themes: [...preferences.themes, t],
              });
            }}
          >
            <Copy size={17} />
          </button>
          <select
            aria-label={tr('Выбрать тему')}
            value={selected}
            onChange={(e) => choose(e.target.value)}
          >
            {trText(
              themes.map((t) => (
                <option key={t.id} value={t.id}>
                  {builtinThemes.some((b) => b.id === t.id) ? tr(t.name) : t.name}
                </option>
              )),
            )}
          </select>
        </Row>
        <Row title={tr('Акцентный цвет')}>
          <span className="control-caption">{tr('Свой цвет')}</span>
          <Color
            label={tr('Акцентный цвет')}
            value={current.accent ?? current.colors.text}
            onChange={(accent) => patch({ accent })}
          />
        </Row>
        <Row title={tr('Фон')}>
          <Color
            label={tr('Фон')}
            value={current.colors.bg}
            onChange={(bg) => patch({ colors: { ...current.colors, bg } })}
          />
        </Row>
        <Row title={tr('Цвет переднего плана')}>
          <Color
            label={tr('Цвет переднего плана')}
            value={current.colors.text}
            onChange={(text) => patch({ colors: { ...current.colors, text } })}
          />
        </Row>
        <Row title={tr('Шрифт')}>
          <select
            aria-label={tr('Шрифт интерфейса')}
            value={o.font}
            onChange={(e) => options({ font: e.target.value as typeof o.font })}
          >
            <option value="system">{tr('Системный')}</option>
            <option value="arial">Arial</option>
            <option value="segoe">Segoe UI</option>
          </select>
        </Row>
      </div>
      <details className="appearance-advanced" open>
        <summary>
          <span>
            {tr('Расширенные') + ' '}
            <ChevronDown size={14} />
          </span>
          <button
            className="text-button"
            onClick={(e) => {
              e.preventDefault();
              reset();
            }}
          >
            <RotateCcw size={13} />
            {tr('Сбросить до значений по умолчанию')}
          </button>
        </summary>
        <div className="appearance-card">
          <Row
            title={tr('Размер шрифта интерфейса')}
            description={tr('Основной текст, таблицы и элементы управления')}
          >
            <input
              aria-label={tr('Размер шрифта интерфейса')}
              type="number"
              min={12}
              max={20}
              value={o.fontSize}
              onChange={(e) => options({ fontSize: Number(e.target.value) })}
            />
            <span>px</span>
          </Row>
          <Row
            title={tr('Размер шрифта логов')}
            description={tr('Моноширинный текст журналов и команд')}
          >
            <input
              aria-label={tr('Размер шрифта логов')}
              type="number"
              min={10}
              max={20}
              value={o.codeSize}
              onChange={(e) => options({ codeSize: Number(e.target.value) })}
            />
            <span>px</span>
          </Row>
        </div>
        <div className="appearance-card">
          <Row
            title={tr('Уменьшение движения')}
            description={tr('Уменьшите анимации или используйте настройки системы')}
          >
            <div className="appearance-segment">
              {trText(
                (['system', 'on', 'off'] as const).map((v) => (
                  <button
                    key={v}
                    className={a.motionMode === v ? 'active' : ''}
                    onClick={() => appearance({ motionMode: v })}
                  >
                    {trText(
                      v === 'system' ? tr('Системный') : v === 'on' ? tr('Вкл.') : tr('Выкл.'),
                    )}
                  </button>
                )),
              )}
            </div>
          </Row>
          <Row
            title={tr('Раздельные режимы светлой и тёмной тем')}
            description={tr('Сохраняйте отдельную палитру для каждого режима')}
          >
            <Switch
              label={tr('Раздельные режимы тем')}
              value={a.separate}
              onChange={() => appearance({ separate: !a.separate })}
            />
          </Row>
          {trText(
            a.separate ? (
              <>
                <Row title={tr('Светлая тема')}>
                  <select
                    aria-label={tr('Светлая тема')}
                    value={a.lightThemeId}
                    onChange={(e) => appearance({ lightThemeId: e.target.value })}
                  >
                    {trText(
                      themes
                        .filter((t) => t.mode === 'light')
                        .map((t) => (
                          <option key={t.id} value={t.id}>
                            {builtinThemes.some((b) => b.id === t.id) ? tr(t.name) : t.name}
                          </option>
                        )),
                    )}
                  </select>
                </Row>
                <Row title={tr('Тёмная тема')}>
                  <select
                    aria-label={tr('Тёмная тема')}
                    value={a.darkThemeId}
                    onChange={(e) => appearance({ darkThemeId: e.target.value })}
                  >
                    {trText(
                      themes
                        .filter((t) => t.mode === 'dark')
                        .map((t) => (
                          <option key={t.id} value={t.id}>
                            {builtinThemes.some((b) => b.id === t.id) ? tr(t.name) : t.name}
                          </option>
                        )),
                    )}
                  </select>
                </Row>
              </>
            ) : null,
          )}
        </div>
        <div className="appearance-card">
          <Row title={tr('Стиль шрифта интерфейса')}>
            <select
              aria-label={tr('Стиль шрифта интерфейса')}
              value={o.italic ? 'italic' : 'normal'}
              onChange={(e) => options({ italic: e.target.value === 'italic' })}
            >
              <option value="normal">{tr('Обычный')}</option>
              <option value="italic">{tr('Курсив')}</option>
            </select>
          </Row>
          <Row title={tr('Шрифт контента')}>
            <select
              aria-label={tr('Шрифт контента')}
              value={o.contentFont}
              onChange={(e) => options({ contentFont: e.target.value as typeof o.contentFont })}
            >
              <option value="inherit">{tr('Как шрифт интерфейса')}</option>
              <option value="arial">Arial</option>
              <option value="segoe">Segoe UI</option>
              <option value="system">{tr('Системный')}</option>
            </select>
          </Row>
          <Row title={tr('Шрифт логов')}>
            <select
              aria-label={tr('Шрифт логов')}
              value={o.codeFont}
              onChange={(e) => options({ codeFont: e.target.value as typeof o.codeFont })}
            >
              <option value="system">{tr('Системный')}</option>
              <option value="consolas">Consolas</option>
              <option value="courier">Courier New</option>
            </select>
          </Row>
          <Row title={tr('Полупрозрачная боковая панель')}>
            <Switch
              label={tr('Полупрозрачная боковая панель')}
              value={o.transparent}
              onChange={() => options({ transparent: !o.transparent })}
            />
          </Row>
          <Row title={tr('Контраст')}>
            <input
              aria-label={tr('Контраст')}
              type="range"
              min={0}
              max={100}
              value={o.contrast}
              onChange={(e) => options({ contrast: Number(e.target.value) })}
            />
            <span>{trText(o.contrast)}</span>
          </Row>
        </div>
        <div className="appearance-card">
          <Row
            title={tr('Используйте курсор-указатель')}
            description={tr('Меняйте курсор при наведении на интерактивные элементы')}
          >
            <Switch
              label={tr('Используйте курсор-указатель')}
              value={o.pointer}
              onChange={() => options({ pointer: !o.pointer })}
            />
          </Row>
          <Row title={tr('Скругление')}>
            <input
              aria-label={tr('Скругление')}
              type="range"
              min={0}
              max={24}
              value={current.radius}
              onChange={(e) => patch({ radius: Number(e.target.value) })}
            />
            <span>{trText(current.radius)}px</span>
          </Row>
        </div>
        <div className="appearance-card">
          {trText(
            colorKeys
              .filter((k) => !['bg', 'text'].includes(k))
              .map((key) => (
                <Row title={labels[key]} key={key}>
                  <Color
                    label={tr(labels[key])}
                    value={current.colors[key]}
                    onChange={(v) =>
                      patch({ colors: { ...current.colors, [key as ThemeColor]: v } })
                    }
                  />
                </Row>
              )),
          )}
          {trText(
            !builtinThemes.some((t) => t.id === current.id) ? (
              <>
                <Row title={tr('Название темы')}>
                  <input
                    aria-label={tr('Название темы')}
                    value={current.name}
                    maxLength={64}
                    onChange={(e) => patch({ name: e.target.value })}
                  />
                </Row>
                <Row title={tr('Удаление темы')}>
                  <button
                    className="button"
                    onClick={() => {
                      const rest = preferences.themes.filter((t) => t.id !== current.id);
                      setPreferences({
                        ...preferences,
                        themes: rest,
                        themeId: mode === 'dark' ? 'dark' : 'light',
                        appearance: {
                          ...a,
                          lightThemeId: a.lightThemeId === current.id ? 'light' : a.lightThemeId,
                          darkThemeId: a.darkThemeId === current.id ? 'dark' : a.darkThemeId,
                        },
                      });
                    }}
                  >
                    <Trash2 size={14} />
                    {tr('Удалить тему')}
                  </button>
                </Row>
              </>
            ) : null,
          )}
        </div>
      </details>
      {trText(
        contrastRatio(current.colors.bg, current.colors.text) < 4.5 ? (
          <p className="form-error">
            {tr('Низкий контраст текста и фона. Рекомендуется не менее 4.5:1.')}
          </p>
        ) : null,
      )}
      {trText(
        error ? (
          <p role="alert" className="form-error">
            {trText(error)}
          </p>
        ) : null,
      )}
      <p className="appearance-hint">
        {tr('Изменения сохраняются автоматически и доступны на других устройствах.')}
      </p>
    </div>
  );
}
