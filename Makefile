BINARY_NAME := bin/bot

.PHONY: build run test docker-build docker-up docker-down docker-logs

build:
	go build -o $(BINARY_NAME) ./cmd/bot

run:
	go run ./cmd/bot

test:
	go test ./...

# Docker quick start (requires .env)
docker-build:
	docker build -t bugreport-bot:latest .

docker-up:
	docker compose up -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f

