.PHONY: build build-pg run test clean dev lint tidy seed

BINARY := tld-redirect
BUILD_DIR := bin
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

build:
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) ./cmd/tld-redirect

# Build without CGO (PG-only, for production deployment)
build-pg:
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) ./cmd/tld-redirect

run: build
	./$(BUILD_DIR)/$(BINARY)

dev:
	go run ./cmd/tld-redirect

test:
	go test ./... -v -count=1

clean:
	rm -rf $(BUILD_DIR)

lint:
	golangci-lint run ./...

tidy:
	go mod tidy

seed: build
	./$(BUILD_DIR)/$(BINARY) -seed sample-data/redirects.json

# Deploy to multi-region infrastructure
deploy-control:
	@test -n "$(SERVER)" || (echo "Usage: make deploy-control SERVER=<ip> [ENV=<env-file>]" && exit 1)
	./scripts/deploy-multi.sh control $(SERVER) $(ENV)

deploy-data:
	@test -n "$(SERVER)" || (echo "Usage: make deploy-data SERVER=<ip> [ENV=<env-file>]" && exit 1)
	./scripts/deploy-multi.sh data $(SERVER) $(ENV)
