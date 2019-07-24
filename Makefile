default: ci

.PHONY: deps build test lint

deps:
	go get github.com/smartystreets/goconvey/convey
	go get -u github.com/golangci/golangci-lint/cmd/golangci-lint

build:
	go build ./...

test: build
	go test ./...

lint:
	golangci-lint run

goconvey: build
	goconvey -excludedDirs config

