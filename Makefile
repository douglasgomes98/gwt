BINARY := gwt
BIN := bin/$(BINARY)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: deps build lint test coverage install version
deps:
	go mod download
	npm ci
build:
	go build -buildvcs=false -trimpath -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/gwt
lint:
	go vet ./...
	GOCACHE="$${GOCACHE:-$${TMPDIR:-/tmp}/gwt-go-build}" GOLANGCI_LINT_CACHE="$${GOLANGCI_LINT_CACHE:-$${TMPDIR:-/tmp}/gwt-golangci-lint}" golangci-lint run ./...
test:
	go vet ./...
	go test ./...
coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out
install:
	go install -ldflags "$(LDFLAGS)" ./cmd/gwt
version:
	@echo $(VERSION)
