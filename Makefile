.EXPORT_ALL_VARIABLES:
.PHONY: install-tools install-deps build test lint goconvey dist

GO111MODULE=on

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

goconvey: build
	goconvey -excludedDirs build,config,doc,dist,specs,vendor

dist: build
	zip -r dist/kynaptik-http.zip README.md http.go go.mod go.sum

build/%.so:
	go build -buildmode=plugin -o $@ $<
