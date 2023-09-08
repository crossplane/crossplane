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

NPM := npm
NPM_MODULE_DIR := $(SELF_DIR)/../../node_modules
NPM_PACKAGE_FILE := $(SELF_DIR)/../../package.json
NPM_PACKAGE_LOCK_FILE := $(SELF_DIR)/../../package-lock.json

NG := $(NPM) run ng --

# TODO: link this to overall TTY support
ifneq ($(origin NG_NO_PROGRESS), undefined)
NG_PROGRESS_ARG ?= --progress=false
npm_config_progress = false
export npm_config_progress
endif

NG_KARMA_CONFIG ?= karma.ci.conf.js

NG_OUTDIR ?= $(OUTPUT_DIR)/angular
export NG_OUTDIR

# ====================================================================================
# NPM Targets

# some node packages like node-sass require platform/arch specific install. we need
# to run npm install for each platform. As a result we track a stamp file per host
NPM_INSTALL_STAMP := $(NPM_MODULE_DIR)/npm.install.$(HOST_PLATFORM).stamp

# only run "npm install" if the package.json has changed
$(NPM_INSTALL_STAMP): $(NPM_PACKAGE_FILE) $(NPM_PACKAGE_LOCK_FILE)
	@echo === npm install $(HOST_PLATFORM)
	@$(NPM) install --no-save
#	rebuild node-sass since it has platform dependent bits
	@[ ! -d "$(NPM_MODULE_DIR)/node-sass" ] || $(NPM) rebuild node-sass
	@touch $(NPM_INSTALL_STAMP)

npm.install: $(NPM_INSTALL_STAMP)

.PHONY: npm.install

# ====================================================================================
# Angular Project Targets

ng.build: npm.install
	@echo === ng build $(PLATFORM)
	@$(NG) build --prod $(NG_PROGRESS_ARG)

ng.lint: npm.install
	@echo === ng lint
	@$(NG) lint

ng.test: npm.install
	@echo === ng test
	@$(NG) test $(NG_PROGRESS_ARG) --code-coverage --karma-config $(NG_KARMA_CONFIG)

ng.test-integration: npm.install
	@echo === ng e2e
	@$(NG) e2e

ng.clean:
	@:

ng.distclean:
	@rm -fr $(NPM_MODULE_DIR)

.PHONY: ng.build ng.lint ng.test ng.test-integration ng.clean ng.distclean

# ====================================================================================
# Common Targets

build.code: ng.build
clean: ng.clean
distclean: ng.distclean
lint: ng.lint
test.run: ng.test
e2e.run: ng.test

