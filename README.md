# YM Inline Telegram Bot (Go 1.22+)

Бот для Telegram в inline-режиме: ищет треки в Яндекс Музыке и сразу отправляет выбранный трек как аудио (`SendAudio`).

## Возможности
- Inline-поиск: `@бот <трек или артист>`.
- В выдаче: название, артист, обложка (thumb).
- При выборе результата бот скачивает и отправляет MP3.
- Пагинация inline-выдачи через Telegram `offset` (скролл вниз — новая страница).

## Требования
- Go 1.22+ (или Docker).
- `TELEGRAM_TOKEN` — токен бота.
- `YANDEX_TOKEN` (опционально, но нужен если API требует OAuth).

## Структура
- `cmd/bot/main.go` — точка входа.
- `internal/config` — конфиг из env.
- `internal/utils` — логгер.
- `internal/client/yandex` — поиск/мета/получение download URL/скачивание.
- `internal/services/music` — бизнес-логика.
- `internal/transport/telegram` — inline обработка и отправка аудио.

## Быстрый старт (локально)
```bash
cp env.example .env
# заполните TELEGRAM_TOKEN и при необходимости YANDEX_TOKEN
go run ./cmd/bot
```

## Docker / Docker Compose
```bash
cp env.example .env
# заполните TELEGRAM_TOKEN / YANDEX_TOKEN
docker-compose up --build
```
Или собрать образ:
```bash
docker build -t ym-bot .
docker run --rm --env-file .env ym-bot
```

## Makefile
- `make run` — запуск локально.
- `make build` — бинарь `bin/ym-bot`.
- `make lint` — `go vet ./...`.

## Примечания по Yandex Music API
- Используется web API `https://api.music.yandex.net/search?text=<q>&type=track`.
- Для скачивания дергаем `tracks/{id}/download-info` и разрешаем `downloadInfoUrl` (JSON/XML/redirect).
- OAuth токен может понадобиться — задайте `YANDEX_TOKEN`.
- Возможна замена клиента на `github.com/ndrewnee/go-yandex-music` (достаточно реализовать интерфейс клиента).

## Очистка временных файлов
Скачивание идёт во временную директорию `os.MkdirTemp("", "ym-bot-*")`, после отправки файлы удаляются.

## Ограничения и идеи
- Inline аудио (Telegram) не поддерживает показ обложки в плеере — thumb виден только в списке.
- Возможные улучшения: кэш поиска, выбор битрейта, плейлисты/альбомы, прогрев downloadInfoUrl.

