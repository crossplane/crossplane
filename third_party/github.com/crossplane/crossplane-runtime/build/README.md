# Build

This project contains common build scripts that are used by a number of open source projects at Upbound. We standardize on a makefile interface to all projects with simple verbs. We attempt to put more logic in makefiles and closer to the project and less on side of CI. This enables the builds to run on multiple environments including laptops and different CI/CD systems. Most build and test operations are designed to run in a container that contains all the tools necessary.

Some features include:
 * support for multiple platforms and architectures
 * support for Linux and macOS hosts
 * support for build channels for releases
 * support for releasing artifacts
 * support for golang, helm, kubernetes, nodejs, yarn, s3, and docker

# Quickstart

To use this build project just add a submodule to your project:

```
git submodule add https://github.com/upbound/build build
```

and add a `Makefile` in the root. For example, the following will build a go project that publishes containers and helm charts.

```
# Project Setup
PROJECT_NAME := myrepo
PROJECT_REPO := github.com/upbound/$(PROJECT_NAME)

PLATFORMS ?= linux_amd64 linux_arm64
include build/makelib/common.mk

S3_BUCKET ?= upbound.releases/myrepo
include build/makelib/output.mk

# Setup Go
GO_STATIC_PACKAGES = $(GO_PROJECT)/cmd/api $(GO_PROJECT)/cmd/installer
GO_LDFLAGS += -X $(GO_PROJECT)/pkg/version.Version=$(VERSION)
include build/makelib/golang.mk

# Setup Helm
HELM_BASE_URL = https://charts.upbound.io
HELM_S3_BUCKET = upbound.charts
HELM_CHARTS = myrepo-api
HELM_CHART_LINT_ARGS_myrepo-api = --set nameOverride='',imagePullSecrets=''
include build/makelib/k8s_tools.mk
include build/makelib/helm.mk

# Docker images
DOCKER_myrepo = upbound
IMAGES = myrepo
include build/makelib/image.mk
```

Now build the project by running:

```
make -j build
```

and run tests:

```
make -j tests
```

To see the help run `make help`.

## Local Development Setup

To use local development targets, first include `deploy.mk` in your make file:

```
include build/makelib/local.mk
```

Then, run the following command to initialize a local development configuration:

```
make local.scaffold
```

You can now configure and add more components (i.e. helm releases) to your local development setup.

## Contributing

We welcome contributions. See [Contributing](CONTRIBUTING.md) to get started.

## Report a Bug

For filing bugs, suggesting improvements, or requesting new features, please open an [issue](https://github.com/upbound/build/issues).

## Licensing

The build project is under the Apache 2.0 license.
