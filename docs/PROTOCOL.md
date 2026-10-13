# Совместимость и протокол v1

Pulse Monitor и Pulse Agent — самостоятельные приложения. Первые публичные версии `v0.4.0` рассчитаны друг на друга. Версия приложения и версия протокола — разные понятия. До v1.0 читайте changelog перед обновлением; будущая совместимость не подразумевается.

## Отчёт агента

`POST /api/agent/{id}/report`, `Content-Type: application/json`, `Authorization: Bearer <node-token>`. HTTPS обязателен для удалённого origin; HTTP разрешён только на loopback. Токен не передаётся в URL. Агент не следует перенаправлениям и проверяет TLS-сертификат.

Тело: `bootId`, возрастающий `sequence`, `os`, `hardware`, `sample`, `processes`, `disks`, `gpus`, `connections`, `packets`, `logs`, `capabilities`, необязательный `logCursor`. Формальная структура — в `backend/telemetry.go` и `backend/models.go`, соответствующая структура агента — в `wire.go` его репозитория. У отчёта нет отдельного поля `protocol`: v1 здесь обозначает совместимый контракт выпуска, а не согласование версии в HTTP.

Лимит тела — 2 MiB. Повтор `bootId`/`sequence` идемпотентен: не дублирует принятые журналы. Агент сохраняет один неотправленный отчёт в `pending.json`, повторяет с задержкой до 30 секунд и после приёма продолжает сбор. Это не неограниченный offline-буфер. Нет отчёта 15 секунд — узел offline. После перезапуска панели требуется свежий heartbeat.

Недоступные GPU-показатели процессов — `null`; карты/диски имеют карту `supported`, отчёт — `capabilities`. Отсутствие метрики не означает нулевую нагрузку. Снимки реального узла имеют `source: agent`, узла без данных — `waiting`.

## API владельца

Маршруты мониторинга требуют cookie-сессию, изменения — `X-CSRF-Token` и JSON. Регистрация первого владельца выполняется на loopback. `GET /api/bootstrap` возвращает реестр; `PUT/DELETE /api/servers/{id}` изменяет узел; `GET /api/servers/{id}/snapshot`, `/api/servers/{id}/stream`, `/api/servers/{id}/logs` читает данные. Другие маршруты: `/api/servers/{id}/token`, `/api/events`, `/api/preferences`, `/api/modules`. Токены создаёт и отзывает владелец; исходный токен показывается при выпуске, сервер хранит SHA-256.

## Модули

Манифест и stdin сборщика явно содержат `protocol: 1`. Агент получает список через `GET /api/agent/{id}/modules?os=linux&arch=amd64`, бинарник через `/modules/{module}/binary`, отправляет результат через `POST /api/agent/{id}/modules/{module}/data`. Все запросы используют Bearer-токен своего узла. Результат: `status` (`ok`, `error`, `unsupported`), `error`, `data`; сервер присваивает `at`. [Формат и ограничения](MODULES.md).
