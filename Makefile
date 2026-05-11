BIN_DIR := bin

# backend
.PHONY: vet test lint build

vet:
	go vet ./cmd/... ./internal/...

test:
	go test ./cmd/... ./internal/...

lint:
	golangci-lint run ./cmd/... ./internal/...

build:
	mkdir -p $(BIN_DIR)
	go build -ldflags="-w -s" -o $(BIN_DIR)/cgmerge ./cmd/cgmerge
