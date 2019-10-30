export GOBIN ?= $(shell pwd)/bin

BENCH_FLAGS ?= -cpuprofile=cpu.pprof -memprofile=mem.pprof -benchmem

UPDATE_LICENSE = $(GOBIN)/update-license
GOLINT = $(GOBIN)/golint

GO_FILES := $(shell \
	find . '(' -path '*/.*' -o -path './vendor' ')' -prune \
	-o -name '*.go' -print | cut -b3-)

.PHONY: all
all: build lint test

.PHONY: build
build:
	go build ./...

.PHONY: license
license: $(UPDATE_LICENSE)
	PATH=$(GOBIN):$$PATH ./check_license.sh | tee -a lint.log

.PHONY: lint
lint: $(GOLINT) $(UPDATE_LICENSE)
	@rm -rf lint.log
	@echo "Checking formatting..."
	@gofmt -d -s $(GO_FILES) 2>&1 | tee lint.log
	@echo "Checking vet..."
	@go vet ./... 2>&1 | tee -a lint.log
	@echo "Checking lint..."
	@$(GOLINT) ./... 2>&1 | tee -a lint.log
	@echo "Checking for unresolved FIXMEs..."
	@git grep -i fixme | grep -v -e Makefile | tee -a lint.log
	@echo "Checking for license headers..."
	@PATH=$(GOBIN):$$PATH DRY_RUN=1 ./check_license.sh | tee -a lint.log
	@[ ! -s lint.log ]

$(GOLINT):
	go install golang.org/x/lint/golint

$(UPDATE_LICENSE):
	go install go.uber.org/tools/update-license


.PHONY: test
test:
	go test -race ./...

.PHONY: cover
cover:
	go test -race -coverprofile=cover.out -coverpkg=./... ./...
	go tool cover -html=cover.out -o cover.html

.PHONY: bench
BENCH ?= .
bench:
	go list ./... | xargs -n1 go test -bench=$(BENCH) -run="^$$" $(BENCH_FLAGS)
