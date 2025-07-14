# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with
code in this repository.

## Project Overview

Crossplane is a cloud native control plane framework for building cloud native
control planes without needing to write code. It's a CNCF project that enables
orchestrating applications and infrastructure across multiple environments
through a declarative API.

## Development Commands

### Building and Testing
- `earthly +build` - Build Crossplane binaries for your native OS/architecture
- `earthly +reviewable` - Run code generators, linters, and unit tests (run
  before opening PRs)
- `earthly +test` - Run unit tests
- `earthly +lint` - Run linters (Go and Helm)
- `earthly +generate` - Run code generation (required when updating API types)
- `earthly -P +e2e` - Run end-to-end tests

### Development Environment
- `earthly +hack` - Build and deploy Crossplane to a local kind cluster
- `earthly +unhack` - Delete the kind cluster created by hack target

### Multi-platform
- `earthly +multiplatform-build` - Build for all supported platforms
- `earthly +image` - Build OCI image
- `earthly +helm-build` - Package Helm chart

## Code Architecture

### Core Components

**APIs (`/apis/`)**
- `apiextensions/` - Core Crossplane API types (Compositions, XRDs, etc.)
- `ops/` - Operations API for managing Crossplane lifecycle
- `pkg/` - Package management APIs (Providers, Functions, Configurations)
- `protection/` - Usage tracking and resource protection

**Controllers (`/internal/controller/`)**
- `apiextensions/` - Controllers for core Crossplane resources
  - `composite/` - Manages composite resources and composition functions
  - `claim/` - Manages composite resource claims
  - `composition/` - Manages composition revisions
  - `definition/` - Manages composite resource definitions
- `pkg/` - Package management controllers
- `ops/` - Operations controllers
- `rbac/` - RBAC management controllers

**Key Internal Packages**
- `internal/xfn/` - Composition function runner and utilities
- `internal/engine/` - Core composition engine
- `internal/xcrd/` - CRD generation and schema utilities
- `internal/xpkg/` - Package management (build, cache, parsing)

### Architecture Patterns

**Composition Functions**: Crossplane uses composition functions for advanced
resource composition. Functions are containerized and run via gRPC. The
composition engine in `internal/engine/` orchestrates function execution.

**Package Management**: Crossplane uses OCI-compatible packages (xpkg format)
for distributing Providers, Functions, and Configurations. The package manager
handles installation, dependency resolution, and lifecycle management.

**RBAC Manager**: Crossplane includes a dedicated RBAC manager that
automatically creates appropriate ClusterRoles and bindings for installed
packages based on their requirements.

## Development Guidelines

### Code Style
- Follow Go's upstream [Code Review Comments](https://go.dev/wiki/CodeReviewComments) for general Go style guidelines
- Follow Go's upstream [TestReviewComments](https://go.dev/wiki/TestReviewComments) for testing guidelines
- Crossplane's `contributing/README.md` takes precedence over upstream guides when there are conflicts
- Follow Go project style guidelines and `earthly +lint` requirements
- Use table-driven tests (see `contributing/README.md` for examples)
- Keep error handling narrow in scope
- Return early from functions
- Use descriptive but concise variable names
- Wrap errors with context using `crossplane-runtime/pkg/errors`
- **Error Constants**: Avoid `errFoo` style error constants in new files. This is an old pattern we're moving away from. Only use this pattern if it already appears in the file you're editing.
- **Comment Formatting**: Wrap all comments at 80 columns for consistency and
  readability.
- **Markdown Formatting**: Wrap all markdown documents at 80 columns. Lines
  can be longer if it makes links more readable.

### Testing Requirements
- Unit tests are required for all code changes (~80% coverage target)
- E2E tests required for significant features (see `test/e2e/`)
- Use `cmpopts.EquateErrors()` for error testing
- Test error properties, not error strings
- Test function names use PascalCase without underscores (e.g. `TestMyFunction`,
  not `TestMyFunction_SomeCase`)

### API Changes
- Run `earthly +generate` when updating API types
- Follow Kubernetes API conventions
- Ensure proper conversion functions for API version upgrades

### Package Development
- Use `cmd/crank` CLI for package operations
- Packages follow OCI image format with custom media types
- Package metadata defined in `crossplane.yaml`

## Common Workflows

### Adding a New API Type
1. Define the type in appropriate `apis/` subdirectory
2. Run `earthly +generate` to update generated code
3. Update controllers and tests
4. Run `earthly +reviewable` to verify

### Working with Composition Functions
- Functions communicate via gRPC protocol defined in `proto/fn/`
- Use `crank beta render` for local testing
- Function SDK available for Go and Python

### Package Management
- Use `crank xpkg build` to build packages
- `crank xpkg install` for installation
- Package dependencies handled automatically

## Repository Structure Notes

- `cluster/` - Kubernetes manifests (CRDs, Helm charts, compositions)
- `cmd/` - CLI tools (`crossplane` controller, `crank` CLI)
- `design/` - Design documents and architecture decisions
- `security/` - Security audit reports and assessments
- Uses Earthly instead of traditional Makefile for build automation
- Go modules with dependencies in `go.mod`

## Important Files

- `Earthfile` - Main build configuration
- `generate.go` - Code generation entry point
- `contributing/README.md` - Detailed contribution guidelines
- `design/` - Architecture and design documents