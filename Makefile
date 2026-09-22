# Makefile для сборки проекта shortener

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "N/A")
DATE    := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "N/A")

LDFLAGS := -X main.buildVersion=$(VERSION) \
           -X main.buildDate=$(DATE) \
           -X main.buildCommit=$(COMMIT)

.PHONY: build run test clean help

build:
	go build -ldflags "$(LDFLAGS)" -o bin/shortener ./cmd/shortener

run:
	go run -ldflags "$(LDFLAGS)" ./cmd/shortener

test:
	go test ./...

clean:
	rm -rf bin/

help:
	@echo "Доступные цели:"
	@echo "  build  — собрать бинарник с версией"
	@echo "  run    — запустить с версией"
	@echo "  test   — запустить тесты"
	@echo "  clean  — удалить bin/"
