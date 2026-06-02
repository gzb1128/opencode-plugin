.PHONY: build test clean install local-install uninstall-local test-e2e

BINARY_NAME=opencode-plugin
BIN_DIR=bin
LOCAL_BIN_DIR=$(HOME)/.local/bin

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

local-install: build
	@mkdir -p $(LOCAL_BIN_DIR)
	cp $(BIN_DIR)/$(BINARY_NAME) $(LOCAL_BIN_DIR)/
	@echo "✓ Installed to $(LOCAL_BIN_DIR)/$(BINARY_NAME)"
	@echo "  Make sure $(LOCAL_BIN_DIR) is in your PATH"

uninstall-local:
	rm -f $(LOCAL_BIN_DIR)/$(BINARY_NAME)
	@echo "✓ Removed $(LOCAL_BIN_DIR)/$(BINARY_NAME)"

run: build
	./$(BIN_DIR)/$(BINARY_NAME)

fmt:
	go fmt ./...

lint:
	golangci-lint run

all: fmt test build
