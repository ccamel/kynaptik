.EXPORT_ALL_VARIABLES:
.PHONY: install-tools install-deps build test lint goconvey dist

GO111MODULE=on

SRC_CORE=$(shell ls | grep '.*\.go' | grep -v 'λ.*\.go' | grep -v '.*_test')

default: build

install-tools:
	@if [ ! -f $(GOPATH)/bin/golangci-lint ]; then \
		echo "installing golangci-lint..."; \
		curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b $(GOPATH)/bin v1.18.0; \
	fi
	@if [ ! -f $(GOPATH)/bin/goconvey ]; then \
		echo "installing goconvey..."; \
		go get github.com/smartystreets/goconvey; \
	fi
	@if [ ! -f $(GOPATH)/bin/gothanks ]; then \
		echo "installing gothanks..."; \
		go get -u github.com/psampaz/gothanks; \
	fi

install-deps:
	go get .

build:
	go build -buildmode=plugin -o build/kynaptik.so

test: build
	go test -v -covermode=count -coverprofile c.out .

lint: install-tools
	$(GOPATH)/bin/golangci-lint run

goconvey: install-tools
	$(GOPATH)/bin/goconvey -excludedDirs build,config,doc,dist,specs,vendor

thanks: install-tools
	$(GOPATH)/bin/gothanks -y | grep -v "is already"

dist:
	zip -r dist/kynaptik-http.zip README.md $(SRC_CORE) "λh77p.go" go.mod go.sum
	zip -r dist/kynaptik-graphql.zip README.md $(SRC_CORE) "λ6r4phql.go" go.mod go.sum
