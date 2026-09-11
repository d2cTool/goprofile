# Docker

Стек: server, worker, PostgreSQL, Kafka (KRaft), MinIO, OpenTelemetry Collector, Jaeger, Prometheus, Alertmanager, Loki, Grafana.

Контекст сборки — корень репозитория (`..`): туда же Docker ищет `.dockerignore`.
Каноническая копия правил лежит здесь, дублируется в корне модуля.

```bash
cp .env.example .env
make compose-up
# или из корня репозитория:
docker compose --env-file .env -f docker/docker-compose.yml up --build
```

Переменные — из корневого `.env`. Внутри сети compose для server/worker подставляются `postgres`, `minio`, `kafka`, `otel-collector`.

Остановка:

```bash
make compose-down
```

После старта:

- Grafana: http://localhost:3000 (`admin` / `admin`) — **GophProfile overview**, **Node Exporter**
- Jaeger: http://localhost:16686
- Prometheus: http://localhost:9090
- Alertmanager: http://localhost:9093
- Loki: http://localhost:3100
- Server metrics: http://localhost:8080/metrics
- Worker metrics: http://localhost:9092/metrics
