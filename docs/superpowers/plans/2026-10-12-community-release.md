# Pulse Community Release Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Пользователь устанавливает монитор на одном сервере, агенты на других и получает реальный мониторинг с любого устройства; темы, переводы и модули расширяются без изменения ядра.

**Architecture:** Два самостоятельных репозитория: `pulse-monitor` (Next.js UI + Go backend) и `pulse-agent` (Go collector + services). Агент инициирует HTTPS-соединение с монитором. Модули собирают дополнительные данные отдельными процессами на наблюдаемом узле; монитор устанавливает пакет, хранит настройки и показывает декларативную вкладку.

**Tech Stack:** Существующие Next.js, React, TypeScript, GSAP, Go 1.23, gopsutil; новые зависимости только при доказанной необходимости и согласовании.

**Spec:** Девять пунктов пользователя от 2026-10-12, сохранённые в задачах ниже.

## Global Constraints

- Linux и Windows; релизные сборки Linux amd64/arm64 и Windows amd64.
- Светлая и тёмная темы сохраняются, motion и skeleton loading сохраняются.
- Личные данные, account.json, токены, сертификаты и QA-артефакты не попадают в Git.
- Нулевой серверный реестр — допустимое состояние, никаких синтетических метрик при ошибках связи.
- Установка на конечные серверы не требует Node.js, Go или пересборки ядра.
- Русский и английский в первом релизе; добавление языка через отдельный словарь.
- Не удалять AGENTS.md и лицензии/атрибуцию иконок как «ненужную документацию».

## Review Focus

- Удаление последнего сервера и перезапуск: пустое состояние, никаких аварий или демо-fallback.
- Повреждённый импорт темы: ошибка без потери текущей темы и возможность сброса.
- Несовместимый/зависший модуль: отказ установки или остановка процесса без остановки агента.
- Обновление монитора и агентов в разное время: явная совместимость протокола и модулей.
- Разрыв соединения и перезапуск служб: данные сохраняются, интерфейс показывает реальное состояние.

## Current Evidence

- `components/shell.tsx` уже выставляет `density-compact`; `app/globals.css` меняет только вертикальные отступы td/log-line. Превью имеют отдельные правила, а большая часть интерфейса не меняется.
- Превью процессов использует 9–10 px, логи 7–8 px. Размеры нужно исправить до настройки компактности.
- `components/provider.tsx` стартует с initialServers и считает пустой bootstrap ошибкой; `backend/store.go` создаёт demoServers и запрещает пустые сохранённые определения.
- `scripts/build.ps1` собирает агент и монитор из одного дерева. `git remote -v` пустой; исходники ещё не закоммичены.
- Linux systemd и Windows SCM поддержка агента уже существуют, но нужен законченный установочный сценарий и проверка автозапуска.
- Graph MCP вернул Transport closed; приведённые выводы проверены по исходникам.

## Task 1: Читаемый интерфейс и плотность (пункты 1–2)

**Files:** Modify `app/globals.css`, `components/settings.tsx`, `components/shell.tsx`, `components/overview.tsx`.
**Interfaces:** Preserve `Preferences.compact: boolean`; apply `density-compact` across shell.

- [ ] Поднять основной текст процессов/логов до 13–14 px, вспомогательный до 11–12 px; увеличить строки и иконки, убрать микрошрифт превью.
- [ ] Задать обычные/компактные CSS-токены отступов и высоты строк; покрыть таблицы, логи, карточки, навигацию и панели. Компактность уменьшает отступы, а не читаемость шрифта.
- [ ] Проверить визуально 1920×1080, 1366×768 и 390 px: обе плотности заметно различаются, сохраняются после reload, данные и меню доступны без обрезания.
- [ ] Run `npm run lint`, `npm run typecheck`, `npm run build`.

## Task 2: Реальные данные и подключение первого узла (пункты 6–7)

**Files:** Modify `backend/store.go`, `backend/server.go`, `backend/demo.go`, `backend/telemetry.go`, `components/provider.tsx`, `components/servers.tsx`, `components/connection.tsx`, all node-dependent pages, `lib/demo.ts`, `lib/monitor.ts`; update corresponding Go/Node tests.
**Interfaces:** `Store.server: Server | null`; `Bootstrap.servers` may be `[]`; no synthetic fallback. Move production shared types out of `lib/demo.ts` before deleting its generators.

- [ ] Write failing tests: clean data directory yields zero nodes; persisted empty list survives restart; deleting last node succeeds; unavailable backend never produces synthetic samples/logs.
- [ ] Удалить production-генераторы демо, автоматические демо-серверы и локальные демо-fallback. Тестовые фикстуры оставить только внутри тестов.
- [ ] Миграция удаляет только явно помеченные demo-серверы/логи/события; неизвестные старые записи сохраняет для проверки. Перед миграцией делает backup. Аккаунт и реальные архивы сохраняет.
- [ ] Сделать общий экран «Подключить первый сервер»; загрузка, отсутствие агента и ошибка backend — отдельные состояния.
- [ ] После сохранения сервера сразу показать загрузку агента, токен и готовую команду установки службы. Токен не передавать в URL; не обходить TLS validation.
- [ ] Проверить Linux systemd и Windows SCM: запуск, остановка, автозапуск, восстановление связи, ротация токена; README точно указывает необходимые права.
- [ ] Run backend `go test ./...`, `go vet ./...`; frontend checks from Task 1; agent unit/integration checks on supported hosts.

## Task 3: Два независимых репозитория (пункт 3)

**Files:** Monitor keeps `app/`, `components/`, `lib/`, `backend/`; move `agent/*` to sibling `../PulseAgent/`; split `deployment/` and `scripts/build.ps1` by product; create product-specific README, CHANGELOG, release workflows.
**Interfaces:** Document current agent HTTP report as protocol v1. Each product builds without source files of the other.

- [ ] Создать отдельное локальное Git-дерево агента; переносить только исходники, go.mod/go.sum, тесты, установщики и нужную документацию. Проверить копию и сборку до удаления исходного agent-каталога.
- [ ] Разделить сборки, релизные архивы и версии. Сохранять совместимость действующих отчётов; расширения протокола вводить сначала на принимающей стороне.
- [ ] Проверить чистые checkout: monitor build без agent; agent build без frontend/backend.
- [ ] Зафиксировать исходники Conventional Commits; проверить staged tree на данные, токены и бинарные QA-файлы.

## Task 4: Редактор тем (пункт 4)

**Files:** Create `lib/themes.ts`, `lib/themes.test.mjs`, `components/theme-editor.tsx`; modify provider/settings/global styles; backend preference persistence in dedicated `backend/preferences.go` and tests.
**Interfaces:** `ThemeDefinition = { schemaVersion: 1; id: string; name: string; mode: 'light'|'dark'; colors: Record<ThemeColor,string>; radius: number }`; ThemeColor is a fixed allowlist based on existing CSS variables. `GET/PUT /api/preferences` owner-authenticated, mutation CSRF-protected.

- [ ] Test valid export/import roundtrip; reject unknown schema, invalid colors, CSS/script strings, oversized input; reset works after invalid saved data.
- [ ] Добавить live-preview фона, поверхностей, текста, границ и трёх графических цветов; название, скругление, сохранение, дублирование, сброс, JSON import/export.
- [ ] Показывать предупреждение о низком контрасте. Чёрная и белая встроенные темы остаются доступны как восстановление.
- [ ] Хранить пользовательские темы и выбранную тему на backend, кешировать для первого отображения в браузере; проверять сохранение после backend restart и входа с другого устройства.
- [ ] Run theme tests and frontend/backend checks.

## Task 5: Многоязычность (пункт 8)

**Files:** Create `lib/i18n.ts`, `lib/locales/ru.json`, `lib/locales/en.json`, `lib/i18n.test.mjs`; modify all UI components, `app/layout.tsx`, preference persistence.
**Interfaces:** `Locale = 'ru'|'en'`; `t(key, params?)`; persist locale with Task 4 preferences. Dictionary keys identical across locales.

- [ ] Test matching translation keys, interpolation and fallback to ru; raw remote logs remain unmodified.
- [ ] Вынести строки экранов, входа, регистрации, ошибок и подключения агента. Backend errors get stable codes translated by UI; raw diagnostic details remain available.
- [ ] Переключать язык без перезагрузки, обновлять document.lang; даты и числа форматировать через Intl выбранной локали.
- [ ] Проверить длинные английские строки на мобильном; добавление третьего словаря не требует переписывания компонентов.
- [ ] Run i18n tests and full frontend/backend checks.

## Task 6: Подключаемые модули и рабочий Docker-пример (пункт 5)

**Files:** Monitor create `backend/modules.go`, `backend/modules_test.go`, `lib/modules.ts`, `components/modules.tsx`, `components/module-view.tsx`, static `app/modules/page.tsx`; agent repo create `modules/runner.go`, runner tests, `examples/docker/`; both repos document protocol/schema.
**Interfaces:** Module manifest v1 declares id, version, compatible protocol, platform binaries with SHA256, configuration schema and view schema. Monitor API: `GET/POST /api/modules`, `PUT/DELETE /api/modules/{id}`, `PUT /api/servers/{id}/modules/{moduleId}`, `GET /api/servers/{id}/modules/{moduleId}/data`. Agent control endpoint returns desired revision; agent reports applied revision/status. Collector executable uses bounded JSON requests/responses over stdin/stdout; no dependence on Go shared-library plugins.

- [ ] Write tests: reject traversal/archive bombs and incompatible manifests; reject executable for wrong OS/arch or wrong hash; module timeout/crash never stops agent; disabled module stops collecting; secrets absent from public config/logs.
- [ ] Реализовать upload пакета в панели с показом автора, версии, требуемых прав и выбора узлов. Исполнять только явно включённые владельцем модули; digest проверяет целостность, доверие автору устанавливает владелец.
- [ ] Агент получает отдельную конфигурацию узла и совместимый executable через авторизованный endpoint; atomic installation, bounded requests, timeout and process restart/backoff.
- [ ] Вкладка модуля строится из декларативных таблиц/графиков/карточек и конфигурационных полей; пакет не получает произвольный same-origin JavaScript или session cookies. Динамические модули отображать внутри одной статически экспортируемой страницы.
- [ ] Docker-модуль использует локальный Docker Engine API: Unix socket на Linux, named pipe на Windows. Добавляет контейнеры, CPU/RAM, network I/O, состояние и логи; только наблюдение в первом релизе. Проверить на реальном Docker host.
- [ ] При отключении UI убирает вкладку, collector прекращает работу; архив сохраняется до отдельного удаления. Не требовать пересборки ядра или агента.
- [ ] Написать tutorial: независимый разработчик создаёт пример модуля по документации, собирает пакет и устанавливает через панель. Проверить сценарий целиком.

## Task 7: Документация, очистка и GitHub Releases (пункт 9)

**Files:** Rewrite product README and CHANGELOG; keep focused install/troubleshooting/module-development docs; remove obsolete `design-qa.md` and completed scratch plans after merging final decisions into maintained docs. Inspect every deletion individually.
**Interfaces:** Separate monitor and agent archives, SemVer tags, release notes, SHA256 checksums; compatible versions stated in both READMEs.

- [ ] Документация начинается с простой схемы и quickstart: установить монитор → создать владельца → добавить сервер → установить агент → открыть панель с другого устройства.
- [ ] Добавить upgrade/backup/uninstall, TLS, firewall, права, GPU/packet-capture limitations, разработку модулей, CONTRIBUTING и переводов. Не обещать непроверенные платформенные функции.
- [ ] Проверить чистую установку двух репозиториев, сервер без frontend tooling, телефон, потерю сети, перезапуск и восстановление архивов; записать реальные результаты.
- [ ] Выяснить GitHub owner, названия, visibility и выбранную пользователем open-source лицензию перед публикацией. Не выбирать правовую лицензию за владельца.
- [ ] Создать два GitHub repo, push tested source, publish separate releases with binaries, static UI, installers and checksums. Сообщить точные ссылки и всё, что осталось непроверенным.

## Completion Criterion

Новый пользователь проходит README на чистом сервере без чтения исходников. Панель изначально пуста, подключённый узел отдаёт реальные метрики, настройки работают и сохраняются, RU/EN доступны, Docker устанавливается отдельным модулем, оба GitHub-репозитория и релизы существуют.
