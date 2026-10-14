import test from 'node:test';
import assert from 'node:assert/strict';
import * as i18n from './i18n.ts';
import ru from './locales/ru.json' with { type: 'json' };
import en from './locales/en.json' with { type: 'json' };
test('translations preserve interpolation and unknown remote log text', () => {
  assert.equal(i18n.translate('Сервер', 'en'), 'Server');
  assert.equal(i18n.translate('Обновление {p0} с', 'en', { p0: 5 }), 'Refresh every 5 s');
  assert.equal(i18n.translate('raw journal message', 'en'), 'raw journal message');
  assert.equal(i18n.translate('Сервер', 'ru'), 'Сервер');
  assert.equal(i18n.translate('Сервер', 'unknown'), 'Сервер');
  assert.equal(i18n.translate('Обновление {p0} с', 'en'), 'Refresh every {p0} s');
});
test('both dictionaries cover the same translation keys', () =>
  assert.deepEqual(Object.keys(ru).sort(), Object.keys(en).sort()));
test('locale resolution uses the dictionary registry and rejects unknown locales', () => {
  assert.equal(typeof i18n.resolveLocale, 'function');
  assert.equal(i18n.resolveLocale('en'), 'en');
  assert.equal(i18n.resolveLocale('bad'), 'ru');
  assert.deepEqual(i18n.supportedLocales, ['ru', 'en']);
});
test('numbers follow selected decimal conventions and hide nonfinite telemetry', () => {
  assert.equal(typeof i18n.formatNumber, 'function');
  assert.equal(i18n.formatNumber(12.5, 'ru', { minimumFractionDigits: 1 }), '12,5');
  assert.equal(i18n.formatNumber(12.5, 'en', { minimumFractionDigits: 1 }), '12.5');
  assert.equal(i18n.formatNumber(NaN, 'en'), '—');
});
test('dates follow the selected language and invalid dates remain empty', () => {
  assert.equal(typeof i18n.formatDateTime, 'function');
  assert.equal(
    i18n.formatDateTime('2026-10-12T15:04:05Z', 'en', {
      month: 'long',
      day: 'numeric',
      timeZone: 'UTC',
    }),
    'October 12',
  );
  assert.equal(
    i18n.formatDateTime('2026-10-12T15:04:05Z', 'ru', {
      month: 'long',
      day: 'numeric',
      timeZone: 'UTC',
    }),
    '12 октября',
  );
  assert.equal(i18n.formatDateTime('invalid', 'en'), '—');
});
test('API errors use stable codes with a safe localized HTTP fallback', () => {
  assert.equal(typeof i18n.backendErrorMessage, 'function');
  assert.equal(
    i18n.backendErrorMessage('error.invalid_credentials', 401, 'en'),
    'Invalid email or password.',
  );
  assert.equal(
    i18n.backendErrorMessage('error.invalid_credentials', 401, 'ru'),
    'Неверная почта или пароль.',
  );
  assert.equal(i18n.backendErrorMessage('unknown', 403, 'en'), 'Access denied.');
  assert.equal(i18n.backendErrorMessage(undefined, 502, 'en'), 'Server error. Try again later.');
});
