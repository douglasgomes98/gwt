BINARY := gwt
BIN := bin/$(BINARY)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build test coverage install version
build:
	go build -buildvcs=false -trimpath -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/gwt
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
