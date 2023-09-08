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

ifndef SOURCE_DOCS_DIR
$(error SOURCE_DOCS_DIR must be defined)
endif

ifndef DEST_DOCS_DIR
$(error DEST_DOCS_DIR must be defined)
endif

ifndef DOCS_GIT_REPO
$(error DOCS_GIT_REPO must be defined)
endif

# Optional. If false the publish step will remove this version from the
# documentation repository.
DOCS_VERSION_ACTIVE ?= true

DOCS_VERSION ?= $(shell echo "$(BRANCH_NAME)" | sed -E "s/^release\-([0-9]+)\.([0-9]+)$$/v\1.\2/g")
DOCS_WORK_DIR := $(WORK_DIR)/docs-repo
DOCS_VERSION_DIR := $(DOCS_WORK_DIR)/$(DEST_DOCS_DIR)/$(DOCS_VERSION)

# ====================================================================================
# Targets

docs.init:
	rm -rf $(DOCS_WORK_DIR)
	mkdir -p $(DOCS_WORK_DIR)
	git clone --depth=1 -b master $(DOCS_GIT_REPO) $(DOCS_WORK_DIR)

docs.generate: docs.init
	rm -rf $(DOCS_VERSION_DIR)
	@if [ "$(DOCS_VERSION_ACTIVE)" == "true" ]; then \
		$(INFO) Including version in documentation ; \
		cp -r $(SOURCE_DOCS_DIR)/ $(DOCS_VERSION_DIR); \
		$(OK) Version included in documentation ; \
	fi

docs.run: docs.init
	@if [ "$(DOCS_VERSION_ACTIVE)" == "true" ]; then \
		$(INFO) Including version in documentation ; \
		ln -s $(ROOT_DIR)/$(SOURCE_DOCS_DIR) $(DOCS_VERSION_DIR); \
		$(OK) Version included in documentation ; \
	fi
	cd $(DOCS_WORK_DIR) && DOCS_VERSION=$(DOCS_VERSION) $(MAKE) run

docs.validate: docs.generate
	cd $(DOCS_WORK_DIR) && DOCS_VERSION=$(DOCS_VERSION) $(MAKE) validate

docs.publish: docs.generate
	cd $(DOCS_WORK_DIR) && DOCS_VERSION=$(DOCS_VERSION) $(MAKE) publish

# ====================================================================================
# Common Targets

# only publish docs for master and release branches
ifneq ($(filter master release-%,$(BRANCH_NAME)),)
publish.artifacts: docs.publish
endif
