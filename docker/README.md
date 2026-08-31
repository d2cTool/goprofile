# Docker

Стек: server, worker, PostgreSQL, Kafka (KRaft), MinIO.

Контекст сборки — корень репозитория (`..`): туда же Docker ищет `.dockerignore`.
Каноническая копия правил лежит здесь, дублируется в корне модуля.

```bash
make compose-up
# или из корня репозитория:
docker compose -f docker/docker-compose.yml up --build
```

Остановка:

```bash
make compose-down
```
