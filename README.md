# TGProxy

Один контейнер с двумя сервисами:

- MTProto proxy для приложений Telegram (на базе Go-проекта `mtg` v2.2.8);
- защищенный reverse proxy к `https://api.telegram.org` для ботов;
- ежедневные лог-файлы, автоматически удаляемые через 7 дней;
- интерактивное меню для настройки, запуска, статуса, ссылок и логов.

## Требования

- удаленная Linux-машина с публичным IP;
- Docker Engine с Compose v2;
- открытые TCP-порты `443` и `8443` (или выбранные при настройке);
- Go нужен только для локального запуска меню. Сам сервер собирается Docker-ом.

## Быстрый запуск

```bash
cd /root/project/tgproxy
go run ./cmd/tgproxy menu
```

В меню выберите `1. Настроить`, укажите публичный IP/домен и порты, затем `2. Запустить`.
Настройки и ключи сохраняются в `.env` с правами `0600`. Не публикуйте этот файл.

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

Можно передать `?proxy_key=...`, если библиотека не умеет добавлять заголовок, но заголовок безопаснее: query string чаще попадает в журналы промежуточных систем. Токены ботов в логах TGProxy заменяются на `<redacted>`.

Для публичного сервера настоятельно рекомендуется HTTPS. Самый простой вариант: поставить Caddy/Nginx перед портом `8443`, выпустить сертификат для домена и проксировать на `127.0.0.1:8443`. Тогда ограничьте публикацию API-порта loopback-адресом в `compose.yaml`:

```yaml
- "127.0.0.1:${TGPROXY_API_PORT:-8443}:8080/tcp"
```

Либо смонтируйте сертификат/ключ в контейнер и задайте `TGPROXY_TLS_CERT` и `TGPROXY_TLS_KEY`.

## Обслуживание

```bash
go run ./cmd/tgproxy menu       # интерактивное управление
docker compose logs -f          # поток stdout/stderr
ls -lh logs/                    # файловые логи
docker compose pull --ignore-buildable
docker compose up -d --build    # пересборка/обновление
```

Приложение создает `logs/tgproxy-YYYY-MM-DD.log`. При смене даты оно открывает новый файл и удаляет файлы старше 7 суток. Для Docker stdout в Compose также задан предел: не более 7 файлов по 10 MB.

## Безопасность и диагностика

- Никому не передавайте `.env`, API-ключ и MTProto secret.
- Разрешите в firewall только SSH и выбранные TCP-порты.
- Healthcheck: `curl http://127.0.0.1:8443/healthz` (не требует ключа).
- Проверка настроек: `set -a; . ./.env; set +a; go run ./cmd/tgproxy check`.
- При смене `TGPROXY_API_KEY` выполните `docker compose up -d --force-recreate`.
- Это транспортный прокси, не локальный Telegram Bot API Server. Ограничения официального Bot API сохраняются.

Проект использует `mtg` под лицензией MIT. Документация: https://github.com/9seconds/mtg
