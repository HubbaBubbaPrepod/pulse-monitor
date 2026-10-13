[Русский](TRANSLATIONS.md) · **English**

# Translations

UI dictionaries live in `lib/locales/ru.json` and `lib/locales/en.json`. Keys currently use the original Russian strings. `lib/i18n.ts` selects a dictionary and substitutes `{p0}` and other parameters; missing translations fall back to Russian text or the key. Remote log messages retain their source text.

To fix a translation, update the existing value, preserve every placeholder, and submit a PR with screen context. For a new UI string, add the key to both dictionaries. Run `node --test lib/i18n.test.mjs`, `npm run lint`, and `npm run typecheck`; manually check language switching and long strings on a narrow screen.

Adding a language also requires extending the `Locale` type, the UI language selector, and the backend's allowed preference values. A new JSON file alone does not enable a language. Node names, logs, and module data belong to their source, not the UI dictionary.

Documentation pairs use Russian `README.md` / English `README.en.md`, Russian `docs/*.md` / English `docs/*.en.md`, and English GitHub community files / Russian `*.ru.md`. Update both versions when commands, limitations, or behavior change.
