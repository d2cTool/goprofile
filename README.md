# GophProfile

Сервис аватарок: загрузка, хранение, миниатюры и выдача изображений.

- REST API на Chi
- PostgreSQL — метаданные (мягкое удаление)
- MinIO (S3) — оригиналы и миниатюры
- Kafka — асинхронная обработка и удаление файлов
- Worker — 100×100 / 300×300, идемпотентность, retry с backoff

## Быстрый старт

```bash
cp .env.example .env
make compose-up
# или: docker compose --env-file .env -f docker/docker-compose.yml up --build
```

Compose читает корневой `.env`. Хосты `localhost` из файла для контейнеров заменяются на `postgres` / `minio` / `kafka` / `otel-collector`.

Веб-интерфейс: http://localhost:8080/web/upload  
Health: http://localhost:8080/health  
Метрики: http://localhost:8080/metrics  
MinIO console: http://localhost:9001 (`minioadmin` / `minioadmin`)

Наблюдаемость после `make compose-up`:

| Сервис | URL |
| --- | --- |
| Grafana | http://localhost:3000 (`admin` / `admin`), дашборды **GophProfile overview** и **Node Exporter** |
| Jaeger | http://localhost:16686 |
| Prometheus | http://localhost:9090 |
| Alertmanager | http://localhost:9093 |
| Loki | http://localhost:3100 |

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
| `GET` | `/metrics` | Prometheus scrape |

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

## Наблюдаемость

Сервер и worker пишут JSON-логи через `slog` в stdout (`service`, `trace_id`, `span_id`) и одновременно отправляют их по OTLP в OpenTelemetry Collector (`otel/opentelemetry-collector`). Collector разводит сигналы: трейсы → Jaeger, логи → Loki. Метрики Prometheus отдаются на `/metrics`.

Если `OTEL_EXPORTER_OTLP_ENDPOINT` пустой, приложение стартует без экспорта спанов и логов (остаётся только JSON в stdout).

| Переменная | Назначение |
| --- | --- |
| `OTEL_SERVICE_NAME` | имя сервиса в трейсах (`gophprofile-server` / `gophprofile-worker`) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `http://otel-collector:4318` в compose |
| `LOG_LEVEL` | `debug` / `info` / `warn` / `error` |
| `METRICS_ADDR` | HTTP-порт метрик воркера, по умолчанию `:9090` |

Алерты: `HighErrorRate`, `HighResponseTime`, `HighHTTPErrorRate`, `HighHTTPResponseTime` — `docker/alerts.yml`.
