.PHONY: all build run install test clean

BINARY_NAME=laya-tui

all: build

build:
	go build -o $(BINARY_NAME) main.go

run: build
	./$(BINARY_NAME)

install:
	go install

test:
	go test ./...

clean:
	go clean
	rm -f $(BINARY_NAME)
