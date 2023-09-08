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

SELF_DIR := $(dir $(lastword $(MAKEFILE_LIST)))

YARN_DIR ?= $(SELF_DIR)/../..

YARN := yarn
YARN_MODULE_DIR := $(YARN_DIR)/node_modules
YARN_PACKAGE_FILE := $(YARN_DIR)/package.json
YARN_PACKAGE_LOCK_FILE := $(YARN_DIR)/yarn.lock

YARN_OUTDIR ?= $(OUTPUT_DIR)/yarn
export YARN_OUTDIR

# ====================================================================================
# YARN Targets

# some node packages like node-sass require platform/arch specific install. we need
# to run yarn for each platform. As a result we track a stamp file per host
YARN_INSTALL_STAMP := $(YARN_MODULE_DIR)/yarn.install.$(HOST_PLATFORM).stamp

# only run "yarn" if the package.json has changed
$(YARN_INSTALL_STAMP): $(YARN_PACKAGE_FILE) $(YARN_PACKAGE_LOCK_FILE)
	@echo === yarn
	@cd $(YARN_DIR); $(YARN) --frozen-lockfile --non-interactive
	@touch $(YARN_INSTALL_STAMP)

yarn.install: $(YARN_INSTALL_STAMP)

.PHONY: yarn.install

# ====================================================================================
# Razzle Project Targets

yarn.build: yarn.install
	@echo === yarn build $(PLATFORM)
	@cd $(YARN_DIR); $(YARN) build
	@mkdir -p $(YARN_OUTDIR) && cp -a build $(YARN_OUTDIR)

yarn.test: yarn.install
	@echo === yarn test
	@cd $(YARN_DIR); $(YARN) test-ci

yarn.clean:
	@rm -fr $(YARN_DIR)/build _output .work

yarn.distclean:
	@rm -fr $(YARN_MODULE_DIR) .cache

.PHONY: yarn.build yarn.lint yarn.test yarn.test-integration yarn.clean yarn.distclean

# ====================================================================================
# Common Targets

build.code: yarn.build
clean: yarn.clean
distclean: yarn.distclean
lint: yarn.lint
test.run: yarn.test
e2e.run: yarn.test

