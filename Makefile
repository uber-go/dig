BENCH_FLAGS ?= -cpuprofile=cpu.pprof -memprofile=mem.pprof -benchmem
PKGS ?= $(shell glide novendor | grep -v examples)
PKG_FILES ?= *.go
GO_VERSION := $(shell go version | cut -d " " -f 3)

.PHONY: all
all: lint test

.PHONY: dependencies
dependencies:
	@echo "Installing Glide and locked dependencies..."
	glide --version || go get -u -f github.com/Masterminds/glide
	glide install
	@$(call label,Installing md-to-godoc...)
	$(ECHO_V)go install ./vendor/github.com/sectioneight/md-to-godoc
	@echo "Installing uber-license tool..."
	update-license || go get -u -f go.uber.org/tools/update-license
ifdef SHOULD_LINT
	@echo "Installing golint..."
	go install ./vendor/github.com/golang/lint/golint
else
	@echo "Not installing golint, since we don't expect to lint on" $(GO_VERSION)
endif

.PHONY: lint
lint:
	@rm -rf lint.log
	@echo "Checking formatting..."
	@gofmt -d -s $(PKG_FILES) 2>&1 | tee lint.log
	@echo "Installing test dependencies for vet..."
	@go test -i $(PKGS)
	@echo "Checking vet..."
	@$(foreach dir,$(PKG_FILES),go tool vet $(VET_RULES) $(dir) 2>&1 | tee -a lint.log;)
	@echo "Checking lint..."
	@$(foreach dir,$(PKGS),golint $(dir) 2>&1 | tee -a lint.log;)
	@echo "Checking for unresolved FIXMEs..."
	@git grep -i fixme | grep -v -e vendor -e Makefile | tee -a lint.log
	@echo "Checking for license headers..."
	@DRY_RUN=1 ./check_license.sh | tee -a lint.log
	@[ ! -s lint.log ]

.PHONY: test
test:
	@.build/test.sh

.PHONY: ci
ci: SHELL := /bin/bash
ci: test
	bash <(curl -s https://codecov.io/bash)

.PHONY: bench
BENCH ?= .
bench:
	@$(foreach pkg,$(PKGS),go test -bench=$(BENCH) -run="^$$" $(BENCH_FLAGS) $(pkg);)
