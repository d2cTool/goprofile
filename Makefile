.PHONY: tidy test lint run-server run-worker compose-up compose-down

tidy:
	go mod tidy

test:
	go test ./... -count=1 -cover

cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out

lint:
	golangci-lint run ./...

run-server:
	go run ./cmd/server

run-worker:
	go run ./cmd/worker

compose-up:
	docker compose up --build

compose-down:
	docker compose down -v
