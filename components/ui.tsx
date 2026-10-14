'use client';
import { useLocale, tr, trText } from './locale';
import { formatNumber } from '@/lib/i18n';
import { useRef, useEffect, type ReactNode, type PointerEvent } from 'react';
import gsap from 'gsap';
import { useGSAP } from '@gsap/react';
import { X, CircleCheck, CircleAlert, Search, ServerOff } from 'lucide-react';
import { useMonitor } from './provider';
gsap.registerPlugin(useGSAP);
export function PageMotion({ children }: { children: ReactNode }) {
  useLocale();
  const ref = useRef<HTMLDivElement>(null);
  const { preferences, loading } = useMonitor();
  useGSAP(
    () => {
      const mm = gsap.matchMedia();
      if (preferences.motion)
        mm.add(
          preferences.appearance?.motionMode === 'off'
            ? 'all'
            : '(prefers-reduced-motion: no-preference)',
          () => {
            const timeline = gsap.timeline({ defaults: { ease: 'power3.out', duration: 0.55 } });
            timeline.from('.page-heading', { y: 14, autoAlpha: 0 }, 0);
            const panels = ref.current?.querySelectorAll('.reveal');
            if (panels?.length)
              timeline.from(panels, { y: 20, autoAlpha: 0, stagger: 0.055 }, 0.07);
          },
          ref,
        );
      return () => mm.revert();
    },
    {
      scope: ref,
      dependencies: [preferences.motion, preferences.appearance?.motionMode, loading],
      revertOnUpdate: true,
    },
  );
  return (
    <div ref={ref} className="page-motion">
      {trText(children)}
    </div>
  );
}
export function Panel({
  children,
  className = '',
  spotlight = false,
}: {
  children: ReactNode;
  className?: string;
  spotlight?: boolean;
}) {
  useLocale();
  function onMove(event: PointerEvent<HTMLDivElement>) {
    if (!spotlight || event.pointerType !== 'mouse') return;
    const rect = event.currentTarget.getBoundingClientRect();
    event.currentTarget.style.setProperty('--pointer-x', `${event.clientX - rect.left}px`);
    event.currentTarget.style.setProperty('--pointer-y', `${event.clientY - rect.top}px`);
  }
  return (
    <div
      onPointerMove={onMove}
      className={`panel reveal ${spotlight ? 'spotlight' : ''} ${className}`}
    >
      {trText(children)}
    </div>
  );
}
export function AnimatedNumber({ value, decimals = 1 }: { value: number; decimals?: number }) {
  const locale = useLocale();
  const ref = useRef<HTMLSpanElement>(null);
  const previous = useRef(value);
  const { preferences } = useMonitor();
  useGSAP(
    () => {
      const mm = gsap.matchMedia();
      mm.add(
        {
          motion: '(prefers-reduced-motion: no-preference)',
          reduced: '(prefers-reduced-motion: reduce)',
        },
        (context) => {
          const current = { value: previous.current };
          gsap.to(current, {
            value,
            duration:
              (context.conditions?.motion || preferences.appearance?.motionMode === 'off') &&
              preferences.motion
                ? preferences.interval === 0
                  ? 0.3
                  : 0.6
                : 0,
            ease: 'power2.out',
            onUpdate: () => {
              if (ref.current)
                ref.current.textContent = formatNumber(current.value, locale, {
                  minimumFractionDigits: decimals,
                  maximumFractionDigits: decimals,
                });
            },
          });
          previous.current = value;
        },
      );
      return () => mm.revert();
    },
    {
      scope: ref,
      dependencies: [
        value,
        decimals,
        locale,
        preferences.motion,
        preferences.appearance?.motionMode,
        preferences.interval,
      ],
      revertOnUpdate: true,
    },
  );
  return (
    <span ref={ref}>
      {trText(
        formatNumber(value, locale, {
          minimumFractionDigits: decimals,
          maximumFractionDigits: decimals,
        }),
      )}
    </span>
  );
}
export function PageHeading({
  title,
  subtitle,
  actions,
}: {
  title: string;
  subtitle: string;
  actions?: ReactNode;
}) {
  useLocale();
  return (
    <div className="page-heading">
      <div>
        <h1>{trText(title)}</h1>
        <p>{trText(subtitle)}</p>
      </div>
      {trText(actions ? <div className="heading-actions">{trText(actions)}</div> : null)}
    </div>
  );
}
export function SearchInput({
  value,
  onChange,
  label,
}: {
  value: string;
  onChange: (value: string) => void;
  label: string;
}) {
  useLocale();
  return (
    <label className="search-field">
      <Search size={17} />
      <span className="sr-only">{trText(label)}</span>
      <input
        type="search"
        placeholder={label}
        value={value}
        onChange={(e) => onChange(e.target.value)}
      />
      {trText(
        value ? (
          <button
            className="clear-search"
            onClick={() => onChange('')}
            aria-label={tr('Очистить поиск')}
          >
            <X size={15} />
          </button>
        ) : null,
      )}
    </label>
  );
}
export function Status({ online }: { online: boolean }) {
  useLocale();
  return (
    <span className={`status ${online ? '' : 'unavailable'}`}>
      {trText(online ? <span className="status-dot" /> : <CircleAlert size={13} />)}
      {trText(online ? tr('В сети') : tr('Недоступен'))}
    </span>
  );
}
export function Empty({
  title = 'Ничего не найдено',
  text = 'Измените запрос или сбросьте фильтры.',
}: {
  title?: string;
  text?: string;
}) {
  useLocale();
  return (
    <div className="empty">
      <ServerOff size={30} strokeWidth={1.3} />
      <h3>{trText(title)}</h3>
      <p>{trText(text)}</p>
    </div>
  );
}
export function Skeleton({ dashboard = false }: { dashboard?: boolean }) {
  useLocale();
  return (
    <div className="loading-layout" role="status" aria-label={tr('Загрузка данных')}>
      <span className="sr-only">{tr('Загрузка данных\u2026')}</span>
      <div className="skeleton skeleton-banner" />
      {trText(
        dashboard ? (
          <>
            <div className="metrics-grid">
              {trText(
                [1, 2, 3, 4].map((n) => <div key={n} className="skeleton skeleton-metric" />),
              )}
            </div>
            <div className="chart-layout">
              <div className="skeleton skeleton-chart" />
              <div className="skeleton skeleton-chart" />
            </div>
            <div className="bottom-grid">
              <div className="skeleton skeleton-bottom" />
              <div className="skeleton skeleton-bottom" />
            </div>
          </>
        ) : (
          <div className="skeleton skeleton-table" />
        ),
      )}
    </div>
  );
}
export function Modal({
  title,
  subtitle,
  children,
  onClose,
  wide = false,
}: {
  title: string;
  subtitle?: string;
  children: ReactNode;
  onClose: () => void;
  wide?: boolean;
}) {
  useLocale();
  const ref = useRef<HTMLDialogElement>(null);
  const { preferences } = useMonitor();
  useEffect(() => {
    const dialog = ref.current;
    dialog?.showModal();
    return () => dialog?.close();
  }, []);
  useGSAP(
    () => {
      const mm = gsap.matchMedia();
      if (preferences.motion)
        mm.add(
          preferences.appearance?.motionMode === 'off'
            ? 'all'
            : '(prefers-reduced-motion: no-preference)',
          () => {
            gsap.from(ref.current, { x: 55, autoAlpha: 0, duration: 0.4, ease: 'power3.out' });
            const fields = ref.current?.querySelectorAll(
              '.field, .form-section, .modal-footer, .form-note, .detail-list',
            );
            if (fields?.length)
              gsap.from(fields, {
                y: 10,
                autoAlpha: 0,
                stagger: 0.04,
                duration: 0.35,
                delay: 0.12,
              });
          },
          ref,
        );
      return () => mm.revert();
    },
    { scope: ref },
  );
  return (
    <dialog
      ref={ref}
      className={`drawer ${wide ? 'drawer-wide' : ''}`}
      onCancel={onClose}
      onClick={(e) => {
        const bounds = e.currentTarget.getBoundingClientRect();
        if (
          e.target === e.currentTarget &&
          (e.clientX < bounds.left ||
            e.clientX > bounds.right ||
            e.clientY < bounds.top ||
            e.clientY > bounds.bottom)
        )
          onClose();
      }}
      aria-labelledby="dialog-title"
    >
      <div className="drawer-header">
        <div>
          <h2 id="dialog-title">{trText(title)}</h2>
          {trText(subtitle ? <p>{trText(subtitle)}</p> : null)}
        </div>
        <button className="icon-button" onClick={onClose} aria-label={tr('Закрыть окно')}>
          <X size={22} />
        </button>
      </div>
      {trText(children)}
    </dialog>
  );
}
export function Toast() {
  useLocale();
  const { notice } = useMonitor();
  return (
    <div className={`toast ${notice ? 'toast-visible' : ''}`} role="status">
      {trText(
        notice ? (
          <>
            <CircleCheck size={19} />
            {trText(notice)}
          </>
        ) : null,
      )}
    </div>
  );
}
