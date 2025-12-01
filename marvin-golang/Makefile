GITCOMMIT := $(shell git rev-parse --short HEAD 2>/dev/null)
LDFLAGS := -s -w -X main.commit=${GITCOMMIT}

.PHONY: all
all: clean tidy format lint build-prod

.PHONY: clean
clean:
	rm -rf target

target:
	mkdir target

.PHONY: format
format:
	golangci-lint fmt

.PHONY: lint
lint:
	golangci-lint run --timeout 10m

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: build-dev
build-dev: target
	go build -tags dev -ldflags "${LDFLAGS}" -o target/chatbot main.go

.PHONY: build-prod
build-prod: target
	go build -tags prod -ldflags "${LDFLAGS}" -o target/chatbot main.go

.PHONY: dev
dev: build-dev
	./target/chatbot

.PHONY: run
run: build-prod
	./target/chatbot
