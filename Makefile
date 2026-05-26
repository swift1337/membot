BINARY := membot
BIN_DIR := bin

.PHONY: build install codegen lint lint-fix clean

build:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY) ./cmd/membot

install:
	go install ./cmd/membot

codegen:
	go tool sqlc generate

lint:
	go tool golangci-lint run

lint-fix:
	go tool golangci-lint run --fix

clean:
	rm -rf $(BIN_DIR)
