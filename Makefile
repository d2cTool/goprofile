.PHONY: tidy test cover lint run-server run-worker compose-up compose-down

tidy:
	go mod tidy

test:
	go test ./... -count=1 -cover

cover:
	go test "-coverpkg=./..." ./... "-coverprofile=coverage.out" -count=1
	go tool cover -func=coverage.out

lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2 run ./...

run-server:
	go run ./cmd/server

run-worker:
	go run ./cmd/worker

compose-up:
	docker compose up --build

compose-down:
	docker compose down -v
