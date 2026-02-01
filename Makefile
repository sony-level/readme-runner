APP=readme-run

.PHONY: fmt lint test build run

fmt:
	go fmt ./...

lint:
	golangci-lint run ./...

test:
	go test ./... -race

build:
	go build -o bin/$(APP) .

run:
	go run . run --help
