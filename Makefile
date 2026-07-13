BINARY := gwt
BIN := bin/$(BINARY)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build test install version
build:
	go build -buildvcs=false -trimpath -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/gwt
test:
	go vet ./...
	go test ./...
install:
	go install -ldflags "$(LDFLAGS)" ./cmd/gwt
version:
	@echo $(VERSION)
