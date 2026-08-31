# GophProfile

Сервис аватарок: загрузка, хранение, миниатюры и выдача изображений.

- REST API на Chi
- PostgreSQL — метаданные (мягкое удаление)
- MinIO (S3) — оригиналы и миниатюры
- Kafka — асинхронная обработка и удаление файлов
- Worker — 100×100 / 300×300, идемпотентность, retry с backoff

## Быстрый старт

```bash
docker compose up --build
```

Веб-интерфейс: http://localhost:8080/web/upload  
Health: http://localhost:8080/health  
MinIO console: http://localhost:9001 (`minioadmin` / `minioadmin`)

Локальный запуск бинарников (инфраструктура уже поднята):

```bash
cp .env.example .env
make run-server
make run-worker
```

## API

| Метод | Путь | Заголовки |
| --- | --- | --- |
| `POST` | `/api/v1/avatars` | `X-User-ID`, `multipart/form-data` поле `file` |
| `GET` | `/api/v1/avatars/{avatar_id}` | `?size=100x100\|300x300\|original&format=jpeg\|png\|webp` |
| `GET` | `/api/v1/users/{user_id}/avatar` | текущая аватарка |
| `GET` | `/api/v1/avatars/{avatar_id}/metadata` | |
| `GET` | `/api/v1/users/{user_id}/avatars` | список |
| `DELETE` | `/api/v1/avatars/{avatar_id}` | `X-User-ID` |
| `DELETE` | `/api/v1/users/{user_id}/avatar` | `X-User-ID` |
| `GET` | `/health` | postgres / s3 / kafka |

Ограничения: JPEG/PNG/WebP по magic bytes, до 10 МБ, rate limit на загрузку.

## Топики Kafka

- `avatar.uploaded` — после сохранения оригинала в S3
- `avatar.process` — операции ресайза
- `avatar.deleted` — асинхронное удаление объектов из S3

## Тесты и линт

```bash
make test
make cover   # суммарно >50%
make lint    # golangci-lint v2
```
