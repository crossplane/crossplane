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

# remove default suffixes as we dont use them
.SUFFIXES:

# set the shell to bash always
SHELL := /usr/bin/env bash

# default target is build
.PHONY: all
all: build

# ====================================================================================
# Colors

BLACK        := $(shell printf "\033[30m")
BLACK_BOLD   := $(shell printf "\033[30;1m")
RED          := $(shell printf "\033[31m")
RED_BOLD     := $(shell printf "\033[31;1m")
GREEN        := $(shell printf "\033[32m")
GREEN_BOLD   := $(shell printf "\033[32;1m")
YELLOW       := $(shell printf "\033[33m")
YELLOW_BOLD  := $(shell printf "\033[33;1m")
BLUE         := $(shell printf "\033[34m")
BLUE_BOLD    := $(shell printf "\033[34;1m")
MAGENTA      := $(shell printf "\033[35m")
MAGENTA_BOLD := $(shell printf "\033[35;1m")
CYAN         := $(shell printf "\033[36m")
CYAN_BOLD    := $(shell printf "\033[36;1m")
WHITE        := $(shell printf "\033[37m")
WHITE_BOLD   := $(shell printf "\033[37;1m")
CNone        := $(shell printf "\033[0m")

# ====================================================================================
# Logger

TIME_LONG	= `date +%Y-%m-%d' '%H:%M:%S`
TIME_SHORT	= `date +%H:%M:%S`
TIME		= $(TIME_SHORT)

INFO	= echo ${TIME} ${BLUE}[ .. ]${CNone}
WARN	= echo ${TIME} ${YELLOW}[WARN]${CNone}
ERR		= echo ${TIME} ${RED}[FAIL]${CNone}
OK		= echo ${TIME} ${GREEN}[ OK ]${CNone}
FAIL	= (echo ${TIME} ${RED}[FAIL]${CNone} && false)

# ====================================================================================
# Build Options

# Set V=1 to turn on more verbose build
V ?= 0
ifeq ($(V),1)
MAKEFLAGS += VERBOSE=1
else
MAKEFLAGS += --no-print-directory
endif

# Set DEBUG=1 to turn on a debug build
DEBUG ?= 0

# ====================================================================================
# Release Options

# TODO(hasheddan): change default to main and remove master as valid option.
CHANNEL ?= master
ifeq ($(filter master main alpha beta stable,$(CHANNEL)),)
$(error invalid channel $(CHANNEL))
endif

ifeq ($(COMMIT_HASH),)
override COMMIT_HASH := $(shell git rev-parse HEAD)
endif

ifeq ($(origin BRANCH_NAME), undefined)
BRANCH_NAME := $(shell git rev-parse --abbrev-ref HEAD)
endif

REMOTE_NAME ?= origin

# ====================================================================================
# Platform and cross build options

# all supported platforms we build for this can be set to other platforms if desired
# we use the golang os and arch names for convenience
PLATFORMS ?= darwin_amd64 darwin_arm64 windows_amd64 linux_amd64 linux_arm64

# Set the host's OS. Only linux and darwin supported for now
HOSTOS := $(shell uname -s | tr '[:upper:]' '[:lower:]')
ifeq ($(filter darwin linux,$(HOSTOS)),)
$(error build only supported on linux and darwin host currently)
endif

# Set the host's arch.
HOSTARCH := $(shell uname -m)

# If SAFEHOSTARCH and TARGETARCH have not been defined yet, use HOST
ifeq ($(origin SAFEHOSTARCH),undefined)
SAFEHOSTARCH := $(HOSTARCH)
endif
ifeq ($(origin TARGETARCH), undefined)
TARGETARCH := $(HOSTARCH)
endif

# Automatically translate x86_64 to amd64
ifeq ($(HOSTARCH),x86_64)
SAFEHOSTARCH := amd64
TARGETARCH := amd64
endif

# Automatically translate aarch64 to arm64
ifeq ($(HOSTARCH),aarch64)
SAFEHOSTARCH := arm64
TARGETARCH := arm64
endif

ifeq ($(filter amd64 arm64 ppc64le ,$(SAFEHOSTARCH)),)
$(error build only supported on amd64, arm64 and ppc64le host currently)
endif

# Standardize Host Platform variables
HOST_PLATFORM := $(HOSTOS)_$(HOSTARCH)
SAFEHOSTPLATFORM := $(HOSTOS)-$(SAFEHOSTARCH)
SAFEHOST_PLATFORM := $(HOSTOS)_$(SAFEHOSTARCH)
TARGET_PLATFORM := $(HOSTOS)_$(TARGETARCH)

# Set the platform to build if not currently defined
ifeq ($(origin PLATFORM),undefined)
PLATFORM := $(TARGET_PLATFORM)
# if the target platform is on the supported list add it to the single build target
ifneq ($(filter $(PLATFORMS),$(TARGET_PLATFORM)),)
BUILD_PLATFORMS = $(TARGET_PLATFORM)
endif

# for convenience always build the linux platform when building on mac
ifneq ($(HOSTOS),linux)
BUILD_PLATFORMS += linux_$(TARGETARCH)
endif

else
BUILD_PLATFORMS = $(PLATFORM)
endif

OS := $(word 1, $(subst _, ,$(PLATFORM)))
ARCH := $(word 2, $(subst _, ,$(PLATFORM)))

ifeq ($(HOSTOS),darwin)
NPROCS := $(shell sysctl -n hw.ncpu)
else
NPROCS := $(shell nproc)
endif

# ====================================================================================
# Setup directories and paths

# include the common make file
COMMON_SELF_DIR := $(dir $(lastword $(MAKEFILE_LIST)))

# the root directory of this repo
ifeq ($(origin ROOT_DIR),undefined)
ROOT_DIR := $(abspath $(shell cd $(COMMON_SELF_DIR)/../.. && pwd -P))
endif

# the output directory which holds final build produced artifacts
ifeq ($(origin OUTPUT_DIR),undefined)
OUTPUT_DIR := $(ROOT_DIR)/_output
endif

# a working directory that holds all temporary or working items generated
# during the build. The items will be discarded on a clean build and they
# will never be cached.
ifeq ($(origin WORK_DIR), undefined)
WORK_DIR := $(ROOT_DIR)/.work
endif

# a directory that holds tools and other items that are safe to cache
# across build invocations. removing this directory will trigger a
# re-download and waste time. Its safe to cache this directory on CI systems
ifeq ($(origin CACHE_DIR), undefined)
CACHE_DIR := $(ROOT_DIR)/.cache
endif

TOOLS_DIR := $(CACHE_DIR)/tools
TOOLS_HOST_DIR := $(TOOLS_DIR)/$(HOST_PLATFORM)

# ====================================================================================
# Version

ifeq ($(origin HOSTNAME), undefined)
HOSTNAME := $(shell hostname)
endif

# ====================================================================================
# Version and Tagging

# set if you want to use tag grouping, e.g. setting it to "aws" would produce tags like "aws/v0.1.0"
# and release branch would be named as "release-aws-0.1" but the version would still be "v0.1.0".
ifneq ($(PROJECT_VERSION_TAG_GROUP),)
VERSION_TAG_PREFIX := $(PROJECT_VERSION_TAG_GROUP)/
RELEASE_BRANCH_GROUP := $(PROJECT_VERSION_TAG_GROUP)-
endif

# set a semantic version number from git if VERSION is undefined.
ifeq ($(origin VERSION), undefined)
# check if there are any existing `git tag` values
ifeq ($(shell git tag),)
# no tags found - default to initial tag `v0.0.0`
VERSION := $(shell echo "v0.0.0-$$(git rev-list HEAD --count)-g$$(git describe --dirty --always)" | sed 's/-/./2' | sed 's/-/./2' | sed 's/-/./2')
else
# use tags
VERSION := $(shell git describe --dirty --always --tags --match '$(VERSION_TAG_PREFIX)*' | sed 's|.*/||' | sed 's/-/./2' | sed 's/-/./2' | sed 's/-/./2')
endif
endif
export VERSION

VERSION_REGEX := ^v\([0-9]*\)[.]\([0-9]*\)[.]\([0-9]*\)$$
VERSION_VALID := $(shell echo "$(VERSION)" | grep -q '$(VERSION_REGEX)' && echo 1 || echo 0)

# Given "v0.17.0-3.gb4eee9f.dirty" it returns "0".
VERSION_MAJOR := $(shell echo "$(VERSION)" | cut -d'.' -f1 | sed '1s/^.//')

# Given "v0.17.0-3.gb4eee9f.dirty" it returns "17".
VERSION_MINOR := $(shell echo "$(VERSION)" | cut -d'.' -f2)

# Given "v0.17.0-3.gb4eee9f.dirty" it returns "0-3.gb4eee9f.dirty".
# Given "v0.17.1" it returns "1".
VERSION_PATCH := $(shell echo "$(VERSION)" | cut -d'.' -f3-)

release.tag:
ifneq ($(VERSION_VALID),1)
	$(error invalid version $(VERSION). must be a semantic version with v[Major].[Minor].[Patch] only)
endif
	@$(INFO) tagging commit hash $(COMMIT_HASH) with $(VERSION_TAG_PREFIX)$(VERSION)
	git tag -f -m "$(VERSION_TAG_PREFIX)$(VERSION)" $(VERSION_TAG_PREFIX)$(VERSION) $(COMMIT_HASH)
	git push $(REMOTE_NAME) $(VERSION_TAG_PREFIX)$(VERSION)
	@set -e; if ! git ls-remote --heads $(REMOTE_NAME) | grep -q refs/heads/release-$(RELEASE_BRANCH_GROUP)$(VERSION_MAJOR).$(VERSION_MINOR); then \
		echo === creating new release branch release-$(RELEASE_BRANCH_GROUP)$(VERSION_MAJOR).$(VERSION_MINOR) ;\
		git branch -f release-$(RELEASE_BRANCH_GROUP)$(VERSION_MAJOR).$(VERSION_MINOR) $(COMMIT_HASH) ;\
		git push $(REMOTE_NAME) release-$(RELEASE_BRANCH_GROUP)$(VERSION_MAJOR).$(VERSION_MINOR) ;\
	fi
	@$(OK) tagging

# fail publish if the version is dirty
version.isdirty:
	@if [[ $(VERSION) = *.dirty ]]; then \
		$(ERR) version '$(VERSION)' is dirty aborting publish. The following files changed: ;\
		git status --short;\
		exit 1; \
	fi

# ====================================================================================
# Helpers

SED_CMD?=sed -i -e

COMMA := ,
EMPTY :=
SPACE := $(EMPTY) $(EMPTY)

# define a newline
define \n


endef

# ====================================================================================
# This is a special target used to support the build container

common.buildvars:
	@echo PROJECT_NAME=$(PROJECT_NAME)
	@echo PROJECT_REPO=$(PROJECT_REPO)
	@echo BUILD_HOST=$(HOSTNAME)
	@echo BUILD_REGISTRY=$(BUILD_REGISTRY)
	@echo DOCKER_REGISTRY=$(DOCKER_REGISTRY)
	@echo OUTPUT_DIR=$(OUTPUT_DIR)
	@echo WORK_DIR=$(WORK_DIR)
	@echo CACHE_DIR=$(CACHE_DIR)
	@echo HOSTOS=$(HOSTOS)
	@echo HOSTARCH=$(HOSTARCH)
	@echo SAFEHOSTARCH=$(SAFEHOSTARCH)
	@echo TARGETARCH=$(TARGETARCH)
	@echo PLATFORM=$(PLATFORM)
	@echo VERSION=$(VERSION)

build.vars: common.buildvars

# ====================================================================================
# Common Targets - Build and Test workflow

# run init steps before building code
# these will run once regardless of how many platforms we are building
build.init: ; @:

# check the code with fmt, lint, vet and other source level checks pre build
# these will run once regardless of how many platforms we are building
build.check: ; @:

# check the code with fmt, lint, vet and other source level checks pre build
# these will run for each platform being built
build.check.platform: ; @:

# build code. this will run once regardless of platform
build.code: ; @:

# build code. this will run for each platform built
build.code.platform: ; @:

# build releasable artifacts. this will run once regardless of platform
build.artifacts: ; @:

# build releasable artifacts. this will run for each platform being built
build.artifacts.platform: ; @:

# runs at the end of the build to do any cleanup, caching etc.
# these will run once regardless of how many platforms we are building
build.done: ; @:

# helper targets for building multiple platforms
do.build.platform.%:
	@$(MAKE) build.check.platform PLATFORM=$*
	@$(MAKE) build.code.platform PLATFORM=$*
do.build.platform: $(foreach p,$(PLATFORMS), do.build.platform.$(p))

# helper targets for building multiple platforms
do.build.artifacts.%:
	@$(MAKE) build.artifacts.platform PLATFORM=$*
do.build.artifacts: $(foreach p,$(PLATFORMS), do.build.artifacts.$(p))

# build for all platforms
build.all:
	@$(MAKE) build.init
	@$(MAKE) build.check
	@$(MAKE) build.code
	@$(MAKE) do.build.platform
	@$(MAKE) build.artifacts
	@$(MAKE) do.build.artifacts
	@$(MAKE) build.done

# build for a single platform if it's supported
build:
ifneq ($(BUILD_PLATFORMS),)
	@$(MAKE) build.all PLATFORMS="$(BUILD_PLATFORMS)"
else
	@:
endif

# clean all files created during the build.
clean:
	@rm -fr $(OUTPUT_DIR) $(WORK_DIR)

# clean all files created during the build, including caches across builds
distclean: clean
	@rm -fr $(CACHE_DIR)

# run lint and other code analysis
lint.init: ; @:
lint.run: ; @:
lint.done: ; @:
lint:
	@$(MAKE) lint.init
	@$(MAKE) lint.run
	@$(MAKE) lint.done

# unit tests
test.init: ; @:
test.run: ; @:
test.done: ; @:

test:
	@$(MAKE) test.init
	@$(MAKE) test.run
	@$(MAKE) test.done

# e2e tests
e2e.init: ; @:
e2e.run: ; @:
e2e.done: ; @:

e2e:
	@$(MAKE) e2e.init
	@$(MAKE) e2e.run
	@$(MAKE) e2e.done

.PHONY: build.init build.check build.check.platform build.code build.code.platform build.artifacts build.artifacts.platform
.PHONY: build.done do.build.platform.% do.build.platform do.build.artifacts.% do.build.artifacts
.PHONY: build.all build clean distclean lint test test.init test.run test.done e2e.init e2e.run e2e.done

# ====================================================================================
# Release Targets

# run init steps before publishing
publish.init: ; @:

# publish artifacts
publish.artifacts: ; @:

# publish all releasable artifacts
publish: version.isdirty
	@$(MAKE) publish.init
	@$(MAKE) publish.artifacts

# promote init runs before promote
promote.init: ; @:

# promote all artifacts to a release channel
promote.artifacts: ; @:

# promote to a release channel
promote:
	@$(MAKE) promote.init
	@$(MAKE) promote.artifacts

# tag a release
tag: release.tag

# run code generation
generate.init: ; @:
generate.run: ; @:
generate.done: ; @:

generate:
	@$(MAKE) generate.init
	@$(MAKE) generate.run
	@$(MAKE) generate.done

# prepare for code review
reviewable:
	@$(MAKE) generate
	@$(MAKE) lint
	@$(MAKE) test

# ensure generate target doesn't create a diff
check-diff: generate
	@$(INFO) checking that branch is clean
	@if git status --porcelain | grep . ; then $(FAIL); else $(OK) branch is clean; fi

.PHONY: publish.init publish.artifacts publish promote.init promote.artifacts promote tag generate reviewable check-diff

# ====================================================================================
# Help

define HELPTEXT
Usage: make [make-options] <target> [options]

Common Targets:
    build        Build source code and other artifacts for host platform.
    build.all    Build source code and other artifacts for all platforms.
    clean        Remove all files created during the build.
    distclean    Remove all files created during the build including cached tools.
    lint         Run lint and code analysis tools.
    help         Show this help info.
    test         Runs unit tests.
    e2e          Runs end-to-end integration tests.
    generate     Run code generation.
    reviewable   Validate that a PR is ready for review.
    check-diff   Ensure the reviewable target doesn't create a git diff.

Common Options:
    DEBUG        Whether to generate debug symbols. Default is 0.
    PLATFORM     The platform to build.
    SUITE        The test suite to run.
    TESTFILTER   Tests to run in a suite.
    V            Set to 1 enable verbose build. Default is 0.

Release Targets:
    publish      Build and publish final releasable artifacts
    promote      Promote a release to a release channel
    tag          Tag a release

Release Options:
    VERSION      The version information for binaries and releases.
    CHANNEL      Sets the release channel. Can be set to master, main, alpha, beta, or stable.

endef
export HELPTEXT

help-special: ; @:

help:
	@echo "$$HELPTEXT"
	@$(MAKE) help-special

.PHONY: help help-special
