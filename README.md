# TGProxy

Один контейнер с двумя сервисами:

- MTProto proxy для приложений Telegram (на базе Go-проекта `mtg` v2.2.8);
- защищенный reverse proxy к `https://api.telegram.org` для ботов;
- привязка собственного домена и автоматический HTTPS от Let's Encrypt через Caddy;
- ежедневные лог-файлы, автоматически удаляемые через 7 дней;
- интерактивное меню для настройки, запуска, статуса, ссылок и логов.

## Требования

- удаленная Linux-машина с публичным IP;
- Docker Engine с Compose v2;
- открытые TCP-порты `443` и `8443` (или выбранные при настройке);
- Go нужен только для локального запуска меню. Сам сервер собирается Docker-ом.

Docker-сборка использует Go 1.26, необходимый для `mtg` v2.2.8; устанавливать Go на сервер отдельно не требуется.

### Слабая ВМ

Dockerfile специально собирает TGProxy и MTG последовательно, с одним процессом компиляции (`GOMAXPROCS=1`, `-p=1`). Это уменьшает пиковое потребление памяти ценой более долгой первой сборки. Готовые контейнеры потребляют значительно меньше ресурсов, чем компиляция.

Для ВМ с 512 MB–1 GB RAM перед первой сборкой рекомендуется добавить 1 GB swap:

```bash
free -h
swapon --show
fallocate -l 1G /swapfile
chmod 600 /swapfile
mkswap /swapfile
swapon /swapfile
grep -q '^/swapfile ' /etc/fstab || echo '/swapfile none swap sw 0 0' >> /etc/fstab
free -h
```

После успешной сборки swap можно оставить: он помогает переживать кратковременные пики памяти. Если `fallocate` недоступен, используйте `dd if=/dev/zero of=/swapfile bs=1M count=1024 status=progress`.

## Быстрый запуск

```bash
cd /root/project/tgproxy
go run ./cmd/tgproxy menu
```

В меню выберите `1. Настроить`, укажите публичный IP/домен и порты, затем `2. Запустить`.
Настройки и ключи сохраняются в `.env` с правами `0600`. Не публикуйте этот файл.

## Домен и автоматический HTTPS

Самый простой вариант настройки:

1. Создайте у DNS-провайдера запись `A` для домена (например, `tg.example.com`) с публичным IPv4 сервера. Для IPv6 дополнительно создайте корректную запись `AAAA`.
2. Дождитесь обновления DNS и откройте входящие TCP-порты `80`, `443` и `8443` в firewall/security group.
3. Запустите `./tgproxy menu` (или `go run ./cmd/tgproxy menu` при установленном Go), выберите `1. Настроить`, введите домен и ответьте `да` на вопрос об HTTPS. В HTTPS-режиме порт Bot API `443` назначается автоматически; при ошибке в номере другого порта мастер попросит ввести его повторно.
4. Выберите `2. Запустить`. Caddy самостоятельно получит сертификат Let's Encrypt и будет автоматически его продлевать.

В HTTPS-режиме Bot API использует стандартный адрес `https://tg.example.com`, а MTProto по умолчанию переносится на порт `8443`. Это необходимо, потому что два сервиса не могут одновременно слушать порт `443` одного IP. DNS должен указывать прямо на сервер; если у провайдера есть режим HTTP-проксирования (например, «оранжевое облако» Cloudflare), отключите его для MTProto.

Без меню:

```bash
cp .env.example .env
# Отредактируйте .env и задайте длинные случайные ключи
docker compose up -d --build
docker compose ps
```

## Подключение Telegram

Выберите пункт `5. Ссылки MTProto` и откройте выведенную `tg://` или `https://t.me/proxy` ссылку на устройстве с Telegram. Порт `443` обычно лучше проходит через фильтрацию. Один порт нельзя одновременно отдать MTProto и HTTPS Bot API, поэтому API по умолчанию слушает `8443`.

## Использование Bot API proxy

Исходный запрос:

```text
https://api.telegram.org/bot<TOKEN>/getMe
```

Через прокси:

```bash
curl -H "X-TGProxy-Key: <TGPROXY_API_KEY>" \
  "http://SERVER:8443/bot<TOKEN>/getMe"
```

С доменом и включённым HTTPS:

```bash
curl -H "X-TGProxy-Key: <TGPROXY_API_KEY>" \
  "https://tg.example.com/bot<TOKEN>/getMe"
```

Можно передать `?proxy_key=...`, если библиотека не умеет добавлять заголовок, но заголовок безопаснее: query string чаще попадает в журналы промежуточных систем. Токены ботов в логах TGProxy заменяются на `<redacted>`.

Без HTTPS профиль `api-http` публикует API на выбранном порту по обычному HTTP. С HTTPS профиль `api-https` завершает TLS в Caddy, а основной контейнер доступен только во внутренней Docker-сети.

При открытии домена в обычном браузере отображается декоративный мок свадебного приглашения без сведений о назначении сервиса. API обрабатывает только пути Telegram Bot API вида `/bot...` и `/file/bot...`.

## Обслуживание

```bash
go run ./cmd/tgproxy menu       # интерактивное управление
docker compose logs -f          # поток stdout/stderr
docker compose exec tgproxy ls -lh /var/log/tgproxy  # файловые логи
docker compose pull --ignore-buildable
docker compose up -d --build    # пересборка/обновление
```

Приложение создает `tgproxy-YYYY-MM-DD.log` в Docker volume `tgproxy_logs`. При смене даты оно открывает новый файл и удаляет файлы старше 7 суток. Просмотреть файлы можно командой `docker compose exec tgproxy ls -lh /var/log/tgproxy`, а поток записей — через `docker compose logs -f`. Для Docker stdout в Compose также задан предел: не более 7 файлов по 10 MB.

## Безопасность и диагностика

- Никому не передавайте `.env`, API-ключ и MTProto secret.
- Разрешите в firewall только SSH и выбранные TCP-порты.
- Healthcheck: `curl http://127.0.0.1:8443/healthz` (не требует ключа).
- Для HTTPS healthcheck с хоста: `curl https://ДОМЕН/healthz`.
- Если сертификат не выпускается, проверьте `docker compose logs api-https`, DNS-запись и доступность портов `80/443` из интернета.
- Проверка настроек: `set -a; . ./.env; set +a; go run ./cmd/tgproxy check`.
- При смене `TGPROXY_API_KEY` выполните `docker compose up -d --force-recreate`.
- Это транспортный прокси, не локальный Telegram Bot API Server. Ограничения официального Bot API сохраняются.

Проект использует `mtg` под лицензией MIT. Документация: https://github.com/9seconds/mtg
