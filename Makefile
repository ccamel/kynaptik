.EXPORT_ALL_VARIABLES:

GO111MODULE=on

default: build

.PHONY: install-tools build test lint

install-tools:
	@if [ ! -f $(GOPATH)/bin/golangci-lint ]; then \
		echo "installing golangci-lint..."; \
		curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b $(GOPATH)/bin v1.17.1; \
	fi

build:
	go build ./...

test: build
	go test ./...

lint: install-tools
	golangci-lint run

goconvey: build
	goconvey -excludedDirs config

