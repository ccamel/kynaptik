.EXPORT_ALL_VARIABLES:
.PHONY: install-tools install-deps build test lint goconvey dist

GO111MODULE=on

LIB_CORE=$(shell find internal pkg -name  "*.go" | grep -v '.*_test')

FUNCTIONS=$(sort $(notdir $(abspath $(wildcard functions/*/.))))

default: build

tools: ./bin/golangci-lint $(GOPATH)/bin/goconvey $(GOPATH)/bin/gothanks $(GOPATH)/bin/generate-tls-cert

deps:
	go get ./...

build: $(addprefix build-,$(FUNCTIONS))

test: build
	go test -v -covermode=count -coverprofile c.out ./...

lint: tools
	./bin/golangci-lint run

goconvey: tools
	$(GOPATH)/bin/goconvey -cover -excludedDirs build,dist,doc,etc,specs,vendor

tidy:
	go mod tidy && go mod verify

thanks: tools
	$(GOPATH)/bin/gothanks -y | grep -v "is already"

certificates: tools clean-certificates
	cd etc/cert && $(GOPATH)/bin/generate-tls-cert --host localhost --duration 876000h

clean:
	rm -rf build
	rm -rf dist
	rm -rf bin

clean-certificates:
	rm -f etc/cert/*

dist: $(addprefix pkg-,$(FUNCTIONS))

build-%:
	@PKG=$(word 2,$(subst -, ,$@)); \
	echo "⚙️ building function kynaptik-$$PKG"; \
	go build -buildmode=plugin -v -o build/kynaptik-$$PKG.so ./functions/$$PKG/...

pkg-%:
	@PKG=$(word 2,$(subst -, ,$@)); \
	NAME=kynaptik-$$PKG.zip; \
	echo "⚙️ building package archive for kynaptik-$$PKG"; \
	mkdir -p build/$$NAME; \
	mkdir -p dist; \
	tar cpf - $(LIB_CORE) go.mod go.sum | tar xpf - -C build/$$NAME; \
	cp functions/$$PKG/λ.go build/$$NAME/; \
	cd build/$$NAME && zip -r ../../dist/$$NAME.zip .

./bin/golangci-lint:
	@echo "installing $(notdir $@)"
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s v1.24.0

$(GOPATH)/bin/goconvey:
	@echo "installing $(notdir $@)"
	go get github.com/smartystreets/goconvey

$(GOPATH)/bin/gothanks:
	@echo "installing $(notdir $@)"
	go get -u github.com/psampaz/gothanks

$(GOPATH)/bin/generate-tls-cert:
	@echo "installing $(notdir $@)"
	go get github.com/Shyp/generate-tls-cert

