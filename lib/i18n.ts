import ru from './locales/ru.json' with { type: 'json' };
import en from './locales/en.json' with { type: 'json' };
// Register a dictionary here to expose its locale throughout the interface.
export const dictionaries = { ru, en } satisfies Record<string, Record<string, string>>;
export type Locale = keyof typeof dictionaries;
export const defaultLocale: Locale = 'ru';
export const supportedLocales = Object.keys(dictionaries) as Locale[];
export function resolveLocale(value: unknown): Locale {
  return typeof value === 'string' && Object.hasOwn(dictionaries, value)
    ? (value as Locale)
    : defaultLocale;
}
export function currentLocale(): Locale {
  return resolveLocale(
    typeof document === 'undefined' ? defaultLocale : document.documentElement.lang,
  );
}
export function translate(
  key: string,
  locale: Locale = defaultLocale,
  params: Record<string, unknown> = {},
) {
  const text: string =
    (dictionaries[resolveLocale(locale)] as Record<string, string>)[key] ??
    (dictionaries[defaultLocale] as Record<string, string>)[key] ??
    key;
  return text.replace(/\{(\w+)\}/g, (match, k) =>
    params[k] === undefined ? match : String(params[k]),
  );
}
export function formatNumber(
  value: number,
  locale: Locale = currentLocale(),
  options: Intl.NumberFormatOptions = {},
) {
  return Number.isFinite(value)
    ? new Intl.NumberFormat(resolveLocale(locale), { maximumFractionDigits: 2, ...options }).format(
        value,
      )
    : '—';
}
export function formatDateTime(
  value: string | number | Date,
  locale: Locale = currentLocale(),
  options: Intl.DateTimeFormatOptions = {},
) {
  const date = value instanceof Date ? value : new Date(value);
  return Number.isFinite(date.getTime())
    ? new Intl.DateTimeFormat(resolveLocale(locale), options).format(date)
    : '—';
}
const errorKeys = new Set([
  'error.invalid_input',
  'error.unauthorized',
  'error.forbidden',
  'error.not_found',
  'error.conflict',
  'error.rate_limited',
  'error.internal',
  'error.method_not_allowed',
  'error.invalid_credentials',
  'error.registration_closed',
  'error.weak_password',
  'error.invalid_email',
  'error.invalid_name',
]);
export function backendErrorMessage(
  code: unknown,
  status: number,
  locale: Locale = currentLocale(),
) {
  const key =
    typeof code === 'string' && errorKeys.has(code)
      ? code
      : ((
          {
            400: 'error.invalid_input',
            401: 'error.unauthorized',
            403: 'error.forbidden',
            404: 'error.not_found',
            405: 'error.method_not_allowed',
            409: 'error.conflict',
            429: 'error.rate_limited',
          } as Record<number, string>
        )[status] ?? 'error.internal');
  return translate(key, locale);
}
