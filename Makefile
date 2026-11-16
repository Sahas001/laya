.PHONY: all build run install uninstall test fmt vet check clean

BINARY_NAME := laya-tui
PREFIX      ?= $(HOME)/.local
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS     := -s -w -X github.com/Sahas001/laya/app.Version=$(VERSION)

all: build

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY_NAME) ./cmd/laya-tui

run: build
	./$(BINARY_NAME)

install: build
	install -Dm755 $(BINARY_NAME) $(PREFIX)/bin/$(BINARY_NAME)

uninstall:
	rm -f $(PREFIX)/bin/$(BINARY_NAME)

test:
	go test ./...

fmt:
	gofmt -w .

vet:
	go vet ./...

check: vet test
	@test -z "$$(gofmt -l .)" || (echo "gofmt needed:"; gofmt -l .; exit 1)

clean:
	go clean
	rm -f $(BINARY_NAME)
