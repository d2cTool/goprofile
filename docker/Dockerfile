# Build stage
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server \
 && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker

# Runtime stage
FROM alpine:3.21
RUN apk --no-cache add ca-certificates tzdata \
 && adduser -D -H -u 10001 app
WORKDIR /app
COPY --from=builder /out/server /out/worker ./
USER app
EXPOSE 8080
CMD ["./server"]
