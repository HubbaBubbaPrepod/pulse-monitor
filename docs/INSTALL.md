**Русский** · [English](INSTALL.en.md)

# Установка

## Архивы и первый запуск

В [Releases](https://github.com/HubbaBubbaPrepod/pulse-monitor/releases) выбирайте `pulse-monitor_v0.6.5_linux_amd64.tar.gz`, `pulse-monitor_v0.6.5_linux_arm64.tar.gz` или `pulse-monitor_v0.6.5_windows_amd64.zip`. Сверьте SHA-256 с `checksums.txt`: Linux `sha256sum <archive>`, Windows `Get-FileHash <archive> -Algorithm SHA256`. Распакуйте бинарник, `out` и `deployment` вместе. Не размещайте `data`, токены или закрытый ключ в `out`.

Первый запуск выполняется на loopback; команды — в [README](../README.md#быстрый-старт). Пароль владельца: минимум 12 символов, максимум 72 байта UTF-8. После регистрации второго владельца создать нельзя. При SSH-туннеле сервер запущен на `127.0.0.1:8080`, браузер открывает локальный адрес туннеля.

## HTTPS

Для удалённой работы нужны DNS, доверенный сертификат и закрытый ключ. Укажите одновременно `-tls-cert`, `-tls-key`, `-public-url`; последний — HTTPS origin без пути, query, credentials или fragment, например `https://monitor.example.com:8443`. Откройте выбранный порт в firewall. Процессу нужны права чтения ключа и записи каталога данных. Для внутреннего CA добавьте его в доверенные сертификаты ОС агента и браузера. Проверка TLS не отключается.

## Автозапуск

Регистрация должна использовать тот же каталог данных, что и служба. Остановите ручной экземпляр перед запуском службы. Если владелец уже создан в `data` распакованного архива, остановите службу и перенесите весь этот каталог в её каталог данных с правильными правами; установщик сам его не переносит. На Linux это `/var/lib/pulse-monitor` (владелец `pulse-monitor`, каталог `0700`), на Windows — `C:\ProgramData\PulseMonitor\data`. Иначе после установки создайте владельца в новой пустой панели на loopback.

Linux, из распакованного архива:

```bash
sudo bash deployment/install-monitor.sh .
sudo systemctl status pulse-monitor
```

По умолчанию: loopback, `/opt/pulse-monitor` и `/var/lib/pulse-monitor`. После установки создайте владельца через SSH-туннель. Для HTTPS настройте `PULSE_ADDR`, `PULSE_PUBLIC_URL`, `PULSE_TLS_CERT`, `PULSE_TLS_KEY` при установке либо аргументы в unit. Повторная установка сохраняет данные. Проверяйте пути и права сертификатов перед удалённым запуском.

Windows, административный PowerShell:

```powershell
./deployment/install-monitor.ps1 -PackageDirectory .
```

Устанавливается задание Планировщика `PulseMonitor`, каталог по умолчанию `C:\ProgramData\PulseMonitor`. Для HTTPS задайте `-ListenAddress 0.0.0.0:8443 -PublicURL https://monitor.example.com:8443 -TLSCert C:\PulseTLS\fullchain.pem -TLSKey C:\PulseTLS\privkey.pem`. Создайте владельца в каталоге данных установленной панели до удалённого прослушивания.

## Агент и сборка

Следуйте [инструкции Pulse Agent](https://github.com/HubbaBubbaPrepod/pulse-agent#быстрый-старт). У каждого узла свой ID, токен и каталог состояния. Адрес в реестре панели — описание расположения узла; агент использует `-url` панели. Замена токена отзывает старый: обновите файл и перезапустите агент.

Сборка из исходников (Node.js 22.13+, Go 1.27.3+, PowerShell 7.4+; Go в PATH либо аргумент `-GoExe`):

```powershell
npm ci
./scripts/build.ps1 -Version v0.6.5
```

Результат — архивы в `artifacts/releases` и `checksums.txt`. `-SkipFrontend` использует существующий `out`; экспорт должен соответствовать текущему исходному коду.

Windows-установка требует NTFS. Задача работает от Local Service, читает бинарник/статику и изменяет приватные данные. Существующую установку на exFAT вместе с защищёнными резервными копиями нужно перенести на NTFS до обновления. Установщик сохраняет данные, но не переносит их.
