APP=readme-run
ALIAS=rd-run

.PHONY: fmt vet lint test build install clean run help

help:
	@echo "Available targets:"
	@echo "  make build   - Build the binary"
	@echo "  make test    - Run tests with race detector"
	@echo "  make fmt     - Format code"
	@echo "  make vet     - Run go vet"
	@echo "  make lint    - Run golangci-lint"
	@echo "  make install - Install binary to GOPATH/bin"
	@echo "  make clean   - Remove build artifacts and temp directories"
	@echo "  make run     - Run the CLI help"

fmt:
	go fmt ./...

vet:
	go vet ./...

lint:
	golangci-lint run ./...

test:
	go test ./... -race -v

build:
	go build -o $(APP) .
	ln -sf $(APP) $(ALIAS)

install:
	go install .

clean:
	rm -f $(APP) $(ALIAS)
	rm -rf .rr-temp

run: build
	./$(APP) run --help
