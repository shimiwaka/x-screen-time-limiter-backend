GO ?= $(shell go env GOPATH)/bin/go1.18.10

.PHONY: build test

build:
	GOOS=freebsd GOARCH=amd64 CGO_ENABLED=0 $(GO) build -trimpath -ldflags="-s -w" -o sync.cgi ./src

test:
	$(GO) test -race ./src
