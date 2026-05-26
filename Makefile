BINARY := membot
BIN_DIR := bin

.PHONY: build install codegen clean

build:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY) ./cmd/membot

install:
	go install ./cmd/membot

codegen:
	go tool sqlc generate

clean:
	rm -rf $(BIN_DIR)
