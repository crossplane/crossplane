# Copyright 2016 The Upbound Authors. All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# ====================================================================================
# Options

# Optional. The Go Binary to use
GO ?= go

# The go project including repo name, for example, github.com/rook/rook
GO_PROJECT ?= $(PROJECT_REPO)

# Optional. These are subdirs that we look for all go files to test, vet, and fmt
GO_SUBDIRS ?= cmd pkg

# Optional. Additional subdirs used for integration or e2e testings
GO_INTEGRATION_TESTS_SUBDIRS ?=

# Optional build flags passed to go tools
GO_BUILDFLAGS ?=
GO_LDFLAGS ?=
GO_TAGS ?=
GO_TEST_FLAGS ?=
GO_TEST_SUITE ?=
GO_NOCOV ?=
GO_COVER_MODE ?= count
GO_CGO_ENABLED ?= 0

# ====================================================================================
# Setup go environment

# turn on more verbose build when V=1
ifeq ($(V),1)
GO_LDFLAGS += -v -n
GO_BUILDFLAGS += -x
endif

# whether to generate debug information in binaries. this includes DWARF and symbol tables.
ifeq ($(DEBUG),0)
GO_LDFLAGS += -s -w
endif

# set GOOS and GOARCH
GOOS := $(OS)
GOARCH := $(ARCH)
export GOOS GOARCH

# set GOHOSTOS and GOHOSTARCH
GOHOSTOS := $(HOSTOS)
GOHOSTARCH := $(TARGETARCH)

GO_PACKAGES := $(foreach t,$(GO_SUBDIRS),$(GO_PROJECT)/$(t)/...)
GO_INTEGRATION_TEST_PACKAGES := $(foreach t,$(GO_INTEGRATION_TESTS_SUBDIRS),$(GO_PROJECT)/$(t)/integration)

ifneq ($(GO_TEST_PARALLEL),)
GO_TEST_FLAGS += -p $(GO_TEST_PARALLEL)
endif

ifneq ($(GO_TEST_SUITE),)
GO_TEST_FLAGS += -run '$(GO_TEST_SUITE)'
endif

GOPATH := $(shell $(GO) env GOPATH)

# setup tools used during the build
GOJUNIT := $(TOOLS_HOST_DIR)/go-junit-report
GOJUNIT_VERSION ?= v2.0.0

GOCOVER_COBERTURA := $(TOOLS_HOST_DIR)/gocover-cobertura
# https://github.com/t-yuki/gocover-cobertura/commit/aaee18c8195c3f2d90e5ef80ca918d265463842a
GOCOVER_COBERTURA_VERSION ?= aaee18c8195c3f2d90e5ef80ca918d265463842a

GOIMPORTS := $(TOOLS_HOST_DIR)/goimports
GOIMPORTS_VERSION ?= v0.1.12

GOHOST := GOOS=$(GOHOSTOS) GOARCH=$(GOHOSTARCH) $(GO)
GO_VERSION := $(shell $(GO) version | sed -ne 's/[^0-9]*\(\([0-9]\.\)\{0,4\}[0-9][^.]\).*/\1/p')

# we use a consistent version of gofmt even while running different go compilers.
# see https://github.com/golang/go/issues/26397 for more details
GOFMT_VERSION := $(GO_VERSION)
ifneq ($(findstring $(GOFMT_VERSION),$(GO_VERSION)),)
GOFMT := $(shell which gofmt)
else
GOFMT := $(TOOLS_HOST_DIR)/gofmt$(GOFMT_VERSION)
endif

# We use a consistent version of golangci-lint to ensure everyone gets the same
# linters.
GOLANGCILINT_VERSION ?= 1.50.1
GOLANGCILINT := $(TOOLS_HOST_DIR)/golangci-lint-v$(GOLANGCILINT_VERSION)

GO_BIN_DIR := $(abspath $(OUTPUT_DIR)/bin)
GO_OUT_DIR := $(GO_BIN_DIR)/$(PLATFORM)
GO_TEST_DIR := $(abspath $(OUTPUT_DIR)/tests)
GO_TEST_OUTPUT := $(GO_TEST_DIR)/$(PLATFORM)
GO_LINT_DIR := $(abspath $(OUTPUT_DIR)/lint)
GO_LINT_OUTPUT := $(GO_LINT_DIR)/$(PLATFORM)

ifeq ($(GOOS),windows)
GO_OUT_EXT := .exe
endif

ifeq ($(RUNNING_IN_CI),true)
# Output checkstyle XML rather than human readable output.
# the timeout is increased to 10m, to accommodate CI machines with low resources.
GO_LINT_ARGS += --timeout 10m0s --out-format=checkstyle > $(GO_LINT_OUTPUT)/checkstyle.xml
endif

GO_COMMON_FLAGS = $(GO_BUILDFLAGS) -tags '$(GO_TAGS)' -trimpath
GO_STATIC_FLAGS = $(GO_COMMON_FLAGS) -installsuffix static -ldflags '$(GO_LDFLAGS)'
GO_GENERATE_FLAGS = $(GO_BUILDFLAGS) -tags 'generate $(GO_TAGS)'

# ====================================================================================
# Go Targets

go.init: go.vendor.lite
	@if [ "$(GO111MODULE)" != "on" ] && [ "$(realpath ../../../..)" !=  "$(realpath $(GOPATH))" ]; then \
		$(WARN) the source directory is not relative to the GOPATH at $(GOPATH) or you are you using symlinks. The build might run into issue. Please move the source directory to be at $(GOPATH)/src/$(GO_PROJECT) ;\
	fi

go.build:
	@$(INFO) go build $(PLATFORM)
	$(foreach p,$(GO_STATIC_PACKAGES),@CGO_ENABLED=0 $(GO) build -v -o $(GO_OUT_DIR)/$(lastword $(subst /, ,$(p)))$(GO_OUT_EXT) $(GO_STATIC_FLAGS) $(p) || $(FAIL) ${\n})
	$(foreach p,$(GO_TEST_PACKAGES) $(GO_LONGHAUL_TEST_PACKAGES),@CGO_ENABLED=0 $(GO) test -c -o $(GO_TEST_OUTPUT)/$(lastword $(subst /, ,$(p)))$(GO_OUT_EXT) $(GO_STATIC_FLAGS) $(p) || $(FAIL) ${\n})
	@$(OK) go build $(PLATFORM)

go.install:
	@$(INFO) go install $(PLATFORM)
	$(foreach p,$(GO_STATIC_PACKAGES),@CGO_ENABLED=0 $(GO) install -v $(GO_STATIC_FLAGS) $(p) || $(FAIL) ${\n})
	@$(OK) go install $(PLATFORM)

go.test.unit: $(GOJUNIT) $(GOCOVER_COBERTURA)
	@$(INFO) go test unit-tests
ifeq ($(GO_NOCOV),true)
	@$(WARN) coverage analysis is disabled
	@CGO_ENABLED=0 $(GOHOST) test $(GO_TEST_FLAGS) $(GO_STATIC_FLAGS) $(GO_PACKAGES) || $(FAIL)
else
	@mkdir -p $(GO_TEST_OUTPUT)
	@CGO_ENABLED=$(GO_CGO_ENABLED) $(GOHOST) test -cover $(GO_STATIC_FLAGS) $(GO_PACKAGES) || $(FAIL)
	@CGO_ENABLED=$(GO_CGO_ENABLED) $(GOHOST) test -v -covermode=$(GO_COVER_MODE) -coverprofile=$(GO_TEST_OUTPUT)/coverage.txt $(GO_TEST_FLAGS) $(GO_STATIC_FLAGS) $(GO_PACKAGES) 2>&1 | tee $(GO_TEST_OUTPUT)/unit-tests.log || $(FAIL)
	@cat $(GO_TEST_OUTPUT)/unit-tests.log | $(GOJUNIT) -set-exit-code > $(GO_TEST_OUTPUT)/unit-tests.xml || $(FAIL)
	@$(GOCOVER_COBERTURA) < $(GO_TEST_OUTPUT)/coverage.txt > $(GO_TEST_OUTPUT)/coverage.xml
endif
	@$(OK) go test unit-tests

# Depends on go.test.unit, but is only run in CI with a valid token after unit-testing is complete
# DO NOT run locally.
go.test.codecov:
	@$(INFO) go test codecov
	@cd $(GO_TEST_OUTPUT) && bash <(curl -s https://codecov.io/bash) || $(FAIL)
	@$(OK) go test codecov

go.test.integration: $(GOJUNIT)
	@$(INFO) go test integration-tests
	@mkdir -p $(GO_TEST_OUTPUT) || $(FAIL)
	@CGO_ENABLED=0 $(GOHOST) test $(GO_STATIC_FLAGS) $(GO_INTEGRATION_TEST_PACKAGES) || $(FAIL)
	@CGO_ENABLED=0 $(GOHOST) test $(GO_TEST_FLAGS) $(GO_STATIC_FLAGS) $(GO_INTEGRATION_TEST_PACKAGES) $(TEST_FILTER_PARAM) 2>&1 | tee $(GO_TEST_OUTPUT)/integration-tests.log || $(FAIL)
	@cat $(GO_TEST_OUTPUT)/integration-tests.log | $(GOJUNIT) -set-exit-code > $(GO_TEST_OUTPUT)/integration-tests.xml || $(FAIL)
	@$(OK) go test integration-tests

go.lint: $(GOLANGCILINT)
	@$(INFO) golangci-lint
	@mkdir -p $(GO_LINT_OUTPUT)
	@$(GOLANGCILINT) run $(GO_LINT_ARGS) || $(FAIL)
	@$(OK) golangci-lint

go.vet:
	@$(INFO) go vet $(PLATFORM)
	@CGO_ENABLED=0 $(GOHOST) vet $(GO_COMMON_FLAGS) $(GO_PACKAGES) $(GO_INTEGRATION_TEST_PACKAGES) || $(FAIL)
	@$(OK) go vet $(PLATFORM)

go.fmt: $(GOFMT)
	@$(INFO) go fmt
	@gofmt_out=$$($(GOFMT) -s -d -e $(GO_SUBDIRS) $(GO_INTEGRATION_TESTS_SUBDIRS) 2>&1) && [ -z "$${gofmt_out}" ] || (echo "$${gofmt_out}" 1>&2; $(FAIL))
	@$(OK) go fmt

go.fmt.simplify: $(GOFMT)
	@$(INFO) gofmt simplify
	@$(GOFMT) -l -s -w $(GO_SUBDIRS) $(GO_INTEGRATION_TESTS_SUBDIRS) || $(FAIL)
	@$(OK) gofmt simplify

go.validate: go.modules.check go.vet go.fmt

go.vendor.lite: go.modules.verify
go.vendor.check: go.modules.check
go.vendor.update: go.modules.update
go.vendor: go.modules.download

go.modules.check: go.modules.tidy.check go.modules.verify

go.modules.download:
	@$(INFO) mod download
	@$(GO) mod download || $(FAIL)
	@$(OK) mod download

go.modules.verify:
	@$(INFO) verify go modules dependencies have expected content
	@$(GO) mod verify || $(FAIL)
	@$(OK) go modules dependencies verified

go.modules.tidy:
	@$(INFO) mod tidy
	@$(GO) mod tidy
	@$(OK) mod tidy

go.modules.tidy.check:
	@$(INFO) verify go modules dependencies are tidy
	@$(GO) mod tidy
	@changed=$$(git diff --exit-code --name-only go.mod go.sum 2>&1) && [ -z "$${changed}" ] || (echo "go.mod is not tidy. Please run 'make go.modules.tidy' and stage the changes" 1>&2; $(FAIL))
	@$(OK) go modules are tidy

go.modules.update:
	@$(INFO) update go modules
	@$(GO) get -u ./... || $(FAIL)
	@$(MAKE) go.modules.tidy
	@$(MAKE) go.modules.verify
	@$(OK) update go modules

go.modules.clean:
	@$(GO) clean -modcache

go.clean:
	@$(GO) clean -cache -testcache -modcache
	@rm -fr $(GO_BIN_DIR) $(GO_TEST_DIR)

go.generate:
	@$(INFO) go generate $(PLATFORM)
	@CGO_ENABLED=0 $(GOHOST) generate $(GO_GENERATE_FLAGS) $(GO_PACKAGES) $(GO_INTEGRATION_TEST_PACKAGES) || $(FAIL)
	@$(OK) go generate $(PLATFORM)
	@$(INFO) go mod tidy
	@$(GOHOST) mod tidy || $(FAIL)
	@$(OK) go mod tidy

.PHONY: go.init go.build go.install go.test.unit go.test.integration go.test.codecov go.lint go.vet go.fmt go.generate
.PHONY: go.validate go.vendor.lite go.vendor go.vendor.check go.vendor.update go.clean
.PHONY: go.modules.check go.modules.download go.modules.verify go.modules.tidy go.modules.tidy.check go.modules.update go.modules.clean

# ====================================================================================
# Common Targets

build.init: go.init
build.code.platform: go.build
clean: go.clean
distclean: go.distclean
lint.init: go.init
lint.run: go.lint
test.init: go.init
test.run: go.test.unit
generate.init: go.init
generate.run: go.generate

# ====================================================================================
# Special Targets

fmt: go.imports
fmt.simplify: go.fmt.simplify
imports: go.imports
imports.fix: go.imports.fix
vendor: go.vendor
vendor.check: go.vendor.check
vendor.update: go.vendor.update
vet: go.vet

define GO_HELPTEXT
Go Targets:
    generate        Runs go code generation followed by goimports on generated files.
    fmt             Checks go source code for formatting issues.
    fmt.simplify    Format, simplify, update source files.
    imports         Checks go source code import lines for issues.
    imports.fix     Updates go source files to fix issues with import lines.
    vendor          Updates vendor packages.
    vendor.check    Fail the build if vendor packages have changed.
    vendor.update   Update vendor dependencies.
    vet             Checks go source code and reports suspicious constructs.
    test.unit.nocov Runs unit tests without coverage (faster for iterative development)
endef
export GO_HELPTEXT

go.help:
	@echo "$$GO_HELPTEXT"

help-special: go.help

.PHONY: fmt vendor vet go.help

# ====================================================================================
# Tools install targets

$(GOLANGCILINT):
	@$(INFO) installing golangci-lint-v$(GOLANGCILINT_VERSION) $(SAFEHOSTPLATFORM)
	@mkdir -p $(TOOLS_HOST_DIR)/tmp-golangci-lint || $(FAIL)
	@curl -fsSL https://github.com/golangci/golangci-lint/releases/download/v$(GOLANGCILINT_VERSION)/golangci-lint-$(GOLANGCILINT_VERSION)-$(SAFEHOSTPLATFORM).tar.gz | tar -xz --strip-components=1 -C $(TOOLS_HOST_DIR)/tmp-golangci-lint || $(FAIL)
	@mv $(TOOLS_HOST_DIR)/tmp-golangci-lint/golangci-lint $(GOLANGCILINT) || $(FAIL)
	@rm -fr $(TOOLS_HOST_DIR)/tmp-golangci-lint
	@$(OK) installing golangci-lint-v$(GOLANGCILINT_VERSION) $(SAFEHOSTPLATFORM)

$(GOFMT):
	@$(INFO) installing gofmt$(GOFMT_VERSION)
	@mkdir -p $(TOOLS_HOST_DIR)/tmp-fmt || $(FAIL)
	@curl -sL https://dl.google.com/go/go$(GOFMT_VERSION).$(SAFEHOSTPLATFORM).tar.gz | tar -xz -C $(TOOLS_HOST_DIR)/tmp-fmt || $(FAIL)
	@mv $(TOOLS_HOST_DIR)/tmp-fmt/go/bin/gofmt $(GOFMT) || $(FAIL)
	@rm -fr $(TOOLS_HOST_DIR)/tmp-fmt
	@$(OK) installing gofmt$(GOFMT_VERSION)

$(GOIMPORTS):
	@$(INFO) installing goimports
	@GOBIN=$(TOOLS_HOST_DIR) $(GOHOST) install golang.org/x/tools/cmd/goimports@$(GOIMPORTS_VERSION) || $(FAIL)
	@$(OK) installing goimports

$(GOJUNIT):
	@$(INFO) installing go-junit-report
	@GOBIN=$(TOOLS_HOST_DIR) $(GOHOST) install github.com/jstemmer/go-junit-report/v2@$(GOJUNIT_VERSION) || $(FAIL)
	@$(OK) installing go-junit-report

$(GOCOVER_COBERTURA):
	@$(INFO) installing gocover-cobertura
	@GOBIN=$(TOOLS_HOST_DIR) $(GOHOST) install github.com/t-yuki/gocover-cobertura@$(GOCOVER_COBERTURA_VERSION) || $(FAIL)
	@$(OK) installing gocover-cobertura
