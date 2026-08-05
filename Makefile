APP := bubble-note
BIN_DIR := bin
BINARY := $(BIN_DIR)/$(APP)

.PHONY: all build run test lint fmt tidy check install clean help

all: check build

build:
	mkdir -p $(BIN_DIR)
	go build -o $(BINARY) ./cmd/bubble-note

run:
	go run ./cmd/bubble-note

test:
	go test ./...

lint:
	go vet ./...

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

tidy:
	go mod tidy

check: test lint

install:
	go install ./cmd/bubble-note

clean:
	rm -rf $(BIN_DIR)

help:
	@printf '%s\n' \
		'make build    Build the binary into bin/' \
		'make run      Run bubble-note locally' \
		'make test     Run all tests' \
		'make lint     Run go vet' \
		'make fmt      Format Go source files' \
		'make tidy     Tidy Go module dependencies' \
		'make check    Run tests and lint' \
		'make install  Install the CLI with go install' \
		'make clean    Remove build artifacts'
