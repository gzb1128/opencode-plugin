.PHONY: build test clean install test-e2e

BINARY_NAME=opencode-plugin
BIN_DIR=bin

build:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY_NAME)

test:
	go test ./... -v

test-e2e:
	go test ./test/e2e -v

test-coverage:
	go test ./... -cover

clean:
	rm -rf $(BIN_DIR)
	go clean

install: build
	cp $(BIN_DIR)/$(BINARY_NAME) /usr/local/bin/

run: build
	./$(BIN_DIR)/$(BINARY_NAME)

fmt:
	go fmt ./...

lint:
	golangci-lint run

all: fmt test build
