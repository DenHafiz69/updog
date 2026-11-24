.PHONY: build run test tidy

BINARY=bin/updog

build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BINARY) ./cmd/updog

run:
	go run ./cmd/updog

test:
	go test -v -race ./...

tidy:
	go mod tidy
