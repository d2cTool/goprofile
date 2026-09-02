# Tests

Unit-тесты лежат рядом с пакетами (`*_test.go`).

```bash
go test ./... -count=1 -cover
```

Интеграционные проверки (testcontainers / живой compose) можно добавлять сюда.
