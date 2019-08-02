export GO15VENDOREXPERIMENT=1

PKGS ?=
# Many Go tools take file globs or directories as arguments instead of packages.
PKG_FILES ?=*.go dump event meta
COVERALLS_TOKEN=WrkOJBvlULyqJtq7IeT5c8FcST2mkEy0q
# The linting tools evolve with each Go version, so run them only on the latest
# stable release.
GO_VERSION := $(shell go version | cut -d " " -f 3)
GO_MINOR_VERSION := $(word 2,$(subst ., ,$(GO_VERSION)))
LINTABLE_MINOR_VERSIONS := 12
ifneq ($(filter $(LINTABLE_MINOR_VERSIONS),$(GO_MINOR_VERSION)),)
SHOULD_LINT := true
endif


.PHONY: all
all: lint test ci

.PHONY: dependencies
dependencies:
	@echo "Installing test dependencies..."
	go get github.com/mattn/goveralls

# Disable printf-like invocation checking due to testify.assert.Error()
VET_RULES := -printf=false

.PHONY: lint
lint:
ifdef SHOULD_LINT
	@rm -rf lint.log
	@echo "Checking formatting..."
	@gofmt -d -s $(PKG_FILES) 2>&1 | tee lint.log
	@echo "Installing test dependencies for vet..."
	@go test -i $(PKGS)
	@echo "Checking vet..."
	@go vet $(VET_RULES) $(PKGS) 2>&1 | tee -a lint.log
	@echo "Checking lint..."
	@$(foreach dir,$(PKGS),golint $(dir) 2>&1 | tee -a lint.log;)
	@echo "Checking for unresolved FIXMEs..."
	@git grep -i fixme | grep -v -e vendor -e Makefile | tee -a lint.log
	@echo "Checking for license headers..."
else
	@echo "Skipping linters on" $(GO_VERSION)
endif

.PHONY: test
test:
	@go test -v  ./...

.PHONY: ci
ci:
	@go test -v  ./... -covermode=count -coverprofile=coverage.out
	@goveralls -coverprofile=coverage.out -service=travis-ci -repotoken ${COVERALLS_TOKEN}