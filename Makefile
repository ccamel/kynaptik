.EXPORT_ALL_VARIABLES:
.PHONY: install-tools install-deps build test lint goconvey dist

GO111MODULE=on

SRC_CORE=$(shell ls | grep '.*\.go' | grep -v 'λ.*\.go' | grep -v '.*_test')

default: build

install-tools:
	@if [ ! -f $(GOPATH)/bin/golangci-lint ]; then \
		echo "installing golangci-lint..."; \
		curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b $(GOPATH)/bin v1.17.1; \
	fi

install-deps:
	go get .

build: build/http.so

test: build
	go test -v -covermode=count -coverprofile c.out .

lint: install-tools
	golangci-lint run

goconvey:
	goconvey -excludedDirs build,config,doc,dist,specs,vendor

dist:
	zip -r dist/kynaptik-http.zip README.md $(SRC_CORE) "λh77p.go" go.mod go.sum

build/%.so:
	go build -buildmode=plugin -o $@ $<
