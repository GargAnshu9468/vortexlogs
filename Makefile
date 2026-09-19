.PHONY: build test test-race bench clean run run-cli lint

BINARY_SERVER=bin/vortexlogs
BINARY_CLI=bin/vortexlogs-cli

build:
	@mkdir -p bin
	@echo "Building VortexLogs Server and CLI..."
	go build -ldflags="-s -w" -o $(BINARY_SERVER) cmd/vortexlogs/main.go
	go build -ldflags="-s -w" -o $(BINARY_CLI) cmd/vortexlogs-cli/main.go
	@echo "Build complete! Server: $(BINARY_SERVER), CLI: $(BINARY_CLI)"

run: build
	./$(BINARY_SERVER)

run-cli: build
	./$(BINARY_CLI)

test:
	go test -v ./...

test-race:
	go test -race -v -count=1 ./...

bench:
	go test -benchmem -bench=. ./benchmarks/...

lint:
	go vet ./...

clean:
	rm -rf bin/ data/ dist/ *.log coverage.html
