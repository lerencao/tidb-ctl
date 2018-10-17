GOPATH ?= $(shell go env GOPATH)

# Ensure GOPATH is set before running build process.
ifeq "$(GOPATH)" ""
  $(error Please set the environment variable GOPATH before running `make`)
endif

PACKAGES  := $$(go list ./...)
FILES     := $$(find . -name "*.go" | grep -vE "vendor")
TOPDIRS   := $$(ls -d */ | grep -vE "vendor")

GOMOD := -mod=vendor

GOVER_MAJOR := $(shell go version | sed -E -e "s/.*go([0-9]+)[.]([0-9]+).*/\1/")
GOVER_MINOR := $(shell go version | sed -E -e "s/.*go([0-9]+)[.]([0-9]+).*/\2/")
GO111 := $(shell [ $(GOVER_MAJOR) -gt 1 ] || [ $(GOVER_MAJOR) -eq 1 ] && [ $(GOVER_MINOR) -ge 11 ]; echo $$?)
ifeq ($(GO111), 1)
$(warning "go below 1.11 does not support modules")
GOMOD :=
endif

.PHONY: default test check doc

default: build

build: export GO111MODULE=on
build:
	go build $(GOMOD) -o bin/tidb-ctl main.go

check:
	@echo "gofmt (simplify)"
	@ gofmt -s -l -w $(FILES) 2>&1 | awk '{print} END{if(NR>0) {exit 1}}'

	@echo "vet"
	@ go tool vet -all -shadow *.go 2>&1 | awk '{print} END{if(NR>0) {exit 1}}'
	@ go tool vet -all -shadow $(TOPDIRS) 2>&1 | awk '{print} END{if(NR>0) {exit 1}}'

	@echo "golint"
	go get github.com/golang/lint/golint
	@ golint -set_exit_status $(PACKAGES)

	@echo "errcheck"
	go get github.com/kisielk/errcheck
	@ errcheck -blank $(PACKAGES) | grep -v "_test\.go" | awk '{print} END{if(NR>0) {exit 1}}'

test: check
	@ log_level=debug GO111MODULE=on go test $(GOMOD) -p 3 -cover $(PACKAGES)

doc:
	@mkdir -p doc
	@ go run main.go --doc
