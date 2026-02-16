BINARY_NAME := bin/bot

.PHONY: build run test

build:
	go build -o $(BINARY_NAME) ./cmd/bot

run:
	go run ./cmd/bot

test:
	go test ./...

