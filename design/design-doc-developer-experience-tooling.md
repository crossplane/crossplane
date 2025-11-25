# Developer Experience (DevEx) Tooling for Crossplane

* Owner: Adam Wolfe Gordon (@adamwg)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Background

Crossplane is a powerful tool for building platforms, but building a platform on
top of Crossplane is non-trivial. A major contributing factor to this difficulty
is the lack of a coherent, opinionated platform developer experience
(DevEx). Each team building on top of Crossplane is left to determine for
themselves how they will build, test, and package the definitions, compositions,
functions, and operations that make up their platform.

This document proposes a set of DevEx tools built around the concept of a
project, which is an opinionated on-disk format for building platforms on top of
Crossplane. The project defines a standard way to organize files containing
Crossplane resources (XRDs, Compositions, Operations, etc.) and function source
code. A project must be built into a set of Crossplane packages before being
installed into a running Crossplane instance. The DevEx tooling implements this
build step, along with other development lifecycle activities: scaffolding
projects and resources, testing compositions and operations, and pushing
packages to registries.

## Goals

* Define a standard developer experience for building platforms on top of
  Crossplane.
* Make it simple to build composition and operation functions alongside the
  compositions and operations that consume them, and to test them together,
  avoiding the need to embed source code in YAML manifests.
* Support functions built in any language (including general purpose languages
  and templating/configuration languages) and using any SDK or framework.
* Enable authors to take advantage of IDE features such as tab completion and
  syntax highlighting when building functions.
* Design with extensibility in mind, such that we can enable users to plug in
  their own implementations of individual parts of the DevEx in the future
  without an architectural overhaul.

## Prior Art and Related Work

This design is based on the [Upbound developer experience], which has already
been implemented as a proprietary tool. Upbound intends to contribute code from
their proprietary tooling to the Crossplane community to implement this design.

This design includes a section on testing. A number of tools have been built or
proposed for testing Crossplane configurations, and may be integrated with this
design. Examples include [xprin] (pending open-sourcing) and the proposed
[`crossplane beta test`] tool.

[function-pythonic] offers a Python-based developer experience for building
composition functions, and could be integrated into this design as a builder
(see the section on functions below).

## Proposal

### Overview

> [!NOTE]
> This document proposes a tree of commands for the `crossplane` CLI. These
> commands will likely start out as `beta` commands, but are written in this
> design without the `beta` prefix to demonstrate the ultimate future state. For
> example, `crossplane project build` will initially be `crossplane beta project
> build`.

A project will have a directory layout similar to the following:

```text
project.yaml
apis
├── cluster
│───├── definition.yaml
│───├── composition.yaml
examples
├── cluster
├───├── xr.yaml
functions
├── compose-cluster
├───├── main.k
├───├── helpers.k
├── propagate-status
├───├── go.mod
├───├── go.sum
├───├── main.go
├── recycle-nodes
├───├── main.py
operations
├── recycle-nodes
│───├── operation.yaml
tests
├── e2etest-cluster-api
├───├── test.yaml.gotmpl
├── test-cluster-api
├───├── main.py
├── test-recycle-nodes
├───├── go.mod
├───├── go.sum
├───├── main.go
```

This example project contains one composite type (`cluster`) supported by two
functions (`compose-cluster` and `propagate-status`). The `compose-cluster`
function is built in KCL, while `propagate-status` is built in Go. The project
also contains one operation (`recycle-nodes`) that runs an operation function of
the same name, built in Python. The `tests` directory contains tests for the
composition and operation. We call the functions built as part of a project
"embedded functions", since they sit alongside configuration rather than in
their own repositories.

The `project.yaml` file contains metadata and configuration for the project. It
configures the build tooling and lists the project's dependencies (other
Crossplane packages such as Providers). When building with projects, the
`project.yaml` file replaces the `crossplane.yaml` file found in non-project
Crossplane package source trees. As described below, the build tooling
constructs a `crossplane.yaml` for each package it produces based on the
contents of the project, including the `project.yaml`.

When a user runs `crossplane project build`, four Crossplane packages will be
produced: a Configuration and three Functions. The Configuration will include
automatically generated dependencies on the functions, as well as any additional
dependencies specified in `project.yaml`. Automated tests can be executed with
`crossplane project test run`, and the project can be installed on a local
development control plane for manual testing with `crossplane project
run`. Running `crossplane project push` will push all four packages to a
registry.

### The Project File

The project configuration file (`project.yaml` by default, overridable with a
CLI argument) looks like this:

```yaml
apiVersion: dev.crossplane.io/v1alpha1
kind: Project
metadata:
  name: my-platform
spec:
  # These optional fields are converted to Configuration annotations.
  maintainer: "Platform Team <platform@example.com>"
  source: github.com/examplecom/my-platform
  license: Apache-2.0
  description: An example configuration using functions.
  readme: This is just an example.

  # OCI repository where the project will be pushed. This is used as part of the
  # build process to construct dependencies on the embedded functions.
  repository: ghcr.io/examplecom/my-platform

  # Crossplane version constraints (optional).
  crossplane:
    version: ">=v1.17.0-0"

  # External dependencies (optional).
  dependsOn:
    - provider: xpkg.crossplane.io/crossplane-contrib/provider-nop
      version: ">=v0.2.1"
    - function: xpkg.crossplane.io/crossplane-contrib/function-auto-ready
      version: ">=v0.2.1"

  # Dependencies that are used only for authoring functions, but do not need to
  # be installed as dependencies of the configuration (optional).
  apiDependencies:
    - type: k8s
      k8s:
        version: v1.33.0
    - type: crd
      git:
        repository: github.com/kubernetes-sigs/cluster-api
        ref: v1.11.3
        path: config/crd/bases

  # Where the build tooling should look for various parts of the configuration,
  # relative to the location of the metadata file. (optional).
  paths:
    apis: apis
    examples: examples
    functions: functions
    operations: operations
    tests: tests

  # Architectures for which to build functions (optional).
  architectures:
    - amd64
    - arm64

  # Optional image configs to rewrite package locations during development, for
  # example to enable use of the DevEx tools in network restricted environments.
  imageConfigs:
    - matchImages:
        - type: prefix
          prefix: xpkg.crossplane.io/crossplane-contrib
      rewriteImage:
        prefix: internal-registry.example.com/mirror/crossplane-contrib
```

Note that we are intentionally using an API group distinct from the existing
Crossplane package manager group (`pkg.crossplane.io`). This makes it clear that
a project is not itself a Crossplane package, but a development artifact that
can be built into a set of packages. As described in subsequent sections, valid
Crossplane package metadata is generated based partly on the contents of the
project metadata file during `crossplane project build`.

The tooling will include helper commands for managing dependencies. These
commands not only mutate the `dependsOn` in the project metadata, but also
download language bindings for dependency packages (see the Language Bindings
section below) so they can be used when writing functions in the project.

### Embedded Functions

A project can include an arbitrary number of embedded functions, which by
convention will live in subdirectories of `functions/` (this path can be
configured). Functions can, theoretically, be built in any language and using
any SDK or framework; Go, Python, KCL, and go-templating will be the initial
supported languages.

The `crossplane project build` command builds each embedded function into its
own Crossplane Function package. This involves first building a runtime image,
then generating and adding a package metadata layer as required by the XPKG
specification. The details of how the runtime image are built vary depending on
the details of how the runtime image are built vary depending on the language used for the function:

* **Go:** [`ko`] is invoked as a library to build an image.
* **go-templating:** Template files are added to the
  [crossplane-contrib/function-go-templating] image and add an environment
  variable to make the embedded templates the default source (see
  [function-go-templating#397] for the function changes needed to make this
  work).
* **KCL:** Similar to go-templating, add KCL files to
  [crossplane-contrib/function-kcl] and set an environment variable setting the
  default source.
* **Python:** Source files are added to a Python interpreter function that
  invokes them, similar to [crossplane-contrib/function-python].

Function packages are named by appending the function name to the project's
top-level package name with an underscore. For example, if the project metadata
file configures `repository: ghcr.io/examplecom/my-platform`, the
`compose-cluster` function package will be called
`ghcr.io/examplecom/my-platform_compose-cluster`.

The tooling will include helper commands for scaffolding functions.

#### Extensibility

As described above, functions are built differently depending on the language
used. To make the tooling extensible, builders for specific languages can be
implemented outside of the core `crossplane project build` code.

A builder must be able to do two things:

1. Detect whether it can build a given function, based on the contents of the
   function's directory.
2. Build the function into a multi-architecture container image and write it to
   a given location as an [OCI layout].

For each function, the core of `crossplane project build` finds the relevant
builder (by running each known builder's detect step), then uses the builder to
create an image.

Initially, all supported builders can be built as part of the DevEx tooling,
ensuring that we provide an experience that works out of the box. In the future,
we may expose builders as an extension point, allowing external builder
implementations to be configured. The design of builders is inspired by [Cloud
Native Buildpacks], which could themselves be used as a builder implementation.

### XRDs, Compositions, and Operations

XRDs, Compositions, and Operations in a project are regular Crossplane
resources. To invoke an embedded function in a composition or operation
pipeline, the user refers to it by package name, just like any other function.

Note that the package name for an embedded function is constructed in the same
manner used by the Crossplane package manager when resolving dependencies, based
on the repository naming scheme described above.

Example:

```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: xexample.example.org
spec:
  mode: Pipeline
  pipeline:
    - step: compose
      functionRef:
        name: examplecom-my-platformcompose-cluster
    - step: propagate-status
      functionRef:
        name: examplecom-my-platformpropagate-status
    - step: function-auto-ready
      functionRef:
        name: crossplane-contrib-function-auto-ready
```

The tooling will include helper commands for building XRDs, compositions, and
operations. For example, we can generate an XRD from an example XR (inferring
the OpenAPI spec), convert other API specification formats (e.g., kro's [Simple
Schema]) to XRDs, scaffold a composition for an XRD, and add steps to pipelines.

### Language Bindings (Schemas)

To make it easier to build functions, we will provide tools to generate language
bindings (also referred to as schemas) for XRDs and CRDs. The mechanism used to
generate language bindings varies by language; current implementations in the
Upbound tooling are:

* **Go:** [oapi-codegen], with some custom mutation to produce better code.
* **go-templating:** Direct conversion of OpenAPI specs to [JSONSchema], with
  some custom mutation to produce better schemas.
* **KCL:** `kcl import` to convert CRDs to KCL schemas.
* **Python:** [datamodel-code-generator], producing [Pydantic] models.

The generated language bindings provide two advantages when authoring functions:

1. Type safety and schema validation, to varying degrees depending on the
   language.
2. IDE integration via standard tooling, providing features like autocomplete,
   inline documentation, linting, etc.

Language bindings can be generated for any Crossplane package that contains CRDs
or XRDs. Generation can run client-side (as part of pulling a dependency package
or manually via a command), but the bindings can also be packaged as part of a
Crossplane package to avoid the need to generate them on download.

`crossplane project build` builds language bindings for the XRDs in a project
and adds a separate image layer for each language to the resulting Configuration
package. The layers are annotated with `io.crossplane.xpkg: schema.<language>`
in conformance with the [XPKG specification]. When pulling dependency packages
that include such layers, the tooling will automatically save the bindings in
the project to be used in embedded functions. The layout of the saved bindings
varies by language.

#### Extensibility

Similar to function builders, schema generation is designed to be extensible. A
schema generator implementation is given a directory of CRDs and outputs a
directory of language bindings. The output directory can have an arbitrary
layout, since every language expects files to be organized differently.

### Testing

Testing is another key facet of software development facilitated by
projects. Projects allow for three layers of testing:

1. Language-specific tests for embedded functions (e.g., Go or Python unit
   tests).
2. Composition tests and operation tests, which use `crossplane render` to run
   composition or operation pipelines (including embedded functions from the
   project) and check assertions on the output.
3. E2E tests, which install a project's packages into a real control plane,
   apply resources, and wait for them to have certain conditions. E2E tests are
   executed using [uptest].

Language-specific tests may use the language bindings described above, but
otherwise are built using language-specific tools outside the scope of this
design.

Composition tests, operation tests, and E2E tests are written as YAML manifests
describing the test to run. The tooling will include the ability to generate
test manifests from code on-the-fly (in the same languages supported for
embedded functions), so that extensive test suites can be built easily without
duplicating many lines of YAML. Multiple tests can be specified in a single
file, and will be run in sequence.

The composition test API looks like this:

```yaml
apiVersion: test.crossplane.io/v1alpha1
kind: CompositionTest
metadata:
  name: test-cluster
spec:
  tests:
    - name: "First reconciliation loop"
      patches:
        # The XRD, for schema validation.
        xrdPath: apis/cluster/definition.yaml
        # Add fields to the input XR
        addField:
          "spec.something": "value"
          "metadata.labels": "mylabel"
      inputs:
        # The XR to render as input to the test.
        xrPath: examples/cluster/xr.yaml
        # The composition to execute for the test.
        compositionPath: apis/cluster/composition.yaml
        # Optional observed resources for the composition pipeline, e.g. to test
        # conditional logic.
        observedResources: []
      # Assertions on the resources rendered by the test, which can include any
      # expected updates to the XR as well as composed resources.
      assertions:
        # Use chainsaw to compare resources.
        - type: chainsaw
          chainsaw:
            resources:
              - apiVersion: platform.example.com/v1alpha1
                kind: Cluster
                metadata:
                  name: example
                spec:
                  version: 1.33
                  region: us-west1
              - apiVersion: container.gcp.upbound.io/v1beta1
                kind: Cluster
                metadata:
                annotations:
                  crossplane.io/composition-resource-name: cluster
                spec:
                  forProvider:
                    location: us-west1
                    minMasterVersion: 1.33
                    nodeVersion: 1.33
      # Timeout for the test.
      timeoutSeconds: 120
      # Whether to validate the output of the render.
      validate: false
```

The test specifies an XR to render, and some [chainsaw] assertions on the output
of the render. This test runs entirely locally, not using a real control
plane. Necessary functions (including embedded functions from the project, which
are built on-the-fly) are run in containers.

The E2E test API is similar:

```yaml
apiVersion: test.crossplane.io/v1alpha1
kind: E2ETest
metadata:
  name: e2e-test-cluster
spec:
  tests:
    - crossplane:
        # Crossplane version to test against when using an ephemeral test cluster.
        version: 2.1.0
      # Manifests to apply as part of the test.
      manifests:
        - apiVersion: platform.example.com/v1alpha1
          kind: Cluster
          metadata:
            name: test-cluster
          spec:
            version: 1.33
            region: us-west1
      # Extra resources that should be installed in the cluster before the test is
      # executed. This allows for configuration of provider credentials, for example.
      extraResources:
        - apiVersion: gcp.upbound.io/v1beta1
          kind: ProviderConfig
          metadata:
            name: default
          spec:
            credentials:
              secretRef:
                key: credentials
                name: gcp-credentials
                namespace: crossplane-system
              source: Secret
            projectID: example-dot-com-testing
        - apiVersion: v1
          data:
            credentials: c3VwZXIgc2VjcmV0IHBhc3N3b3JkIGluc2lkZQo=
          kind: Secret
          metadata:
            name: gcp-credentials
            namespace: crossplane-system
      # Conditions the test will wait for the applied resources to have.
      defaultConditions:
        - Ready
      # Whether to skip deletion of applied resources.
      skipDelete: false
      # Timeout for the test.
      timeoutSeconds: 300
      # Timeout for post-test cleanup, which tries to ensure no resources are left behind.
      cleanupTimeoutSeconds: 600
```

The tooling can either create a local, ephemeral test cluster (using `kind`) in
which to run e2e tests, or run them against an arbitrary kubeconfig
context. Either way, the test is converted into an `uptest` test case and
executed against the test cluster. The tooling takes care of cleaning up
resources after the test runs, to try and avoid potentially leaving behind any
cloud resources that were created.

#### Extensibility

Composition tests have two phases: render and assertion. The render phase is a
core part of the tooling, but assertion could be open to extension. The API
above includes a `type` field for assertions, allowing for other assertion
frameworks to be added. A new type could be introduced that runs an arbitrary
command and provides the results of the render on standard input, allowing for
assertions to be written using any tool the user prefers.

E2E tests are less extensible, since they are executed using uptest. Given the
comparatively higher complexity of e2e tests (which deal with actual clusters
and potentially real cloud resources), it is likely more appropriate to
introduce extensibility points in uptest rather than the wrapper provided by the
DevEx tooling.

## Appendix: Proposed New Commands

For clarity, this is the full tree of commands this design proposes adding to
the `crossplane` CLI:

* `crossplane project`
  * `init` - Initialize a new project.
  * `build` - Build a project into a set of Crossplane packages.
  * `push` - Push packages built from a project to an OCI registry.
  * `run` - Build a project and install it into a control plane for testing; by
    default, create and use a local control plane with `kind`.
  * `stop` - Tear down the control plane started by `run`.
* `crossplane generate`
  * `example` - Interactively generate an example XR.
  * `xrd` - Scaffold an XRD, optionally using an example to determine the
    schema.
  * `composition` - Scaffold a composition pipeline for an XRD.
  * `operation` - Scaffold an operation pipeline.
  * `function` - Scaffold an embedded composition or operation function within a
    project. Optionally add the new function to a pipeline.
  * `test` - Scaffold a composition, operation, or e2e test.
  * `schemas` - Generate language bindings for an existing Crossplane package
    and add them as new package layers.
* `crossplane test` - Execute one or more composition, operation, or e2e
  tests. For e2e tests, optionally create a local control plane and use it (like
  `crossplane project run`).
* `crossplane dependency`
  * `add` - Add a dependency to a project and generate or cache language
    bindings for its resource types.
  * `update-cache` - Update the dependency cache, re-generating or caching
    language bindings as needed.

Additionally, the `crossplane render` command will be updated to work in project
contexts. Specifically, when run in a project with embedded functions, the
functions will automatically be built and run as part of the render operation,
avoiding the need to build and/or push them separately.

## Appendix: API Definitions

The APIs described above for projects and tests are Kubernetes-like, but are
never actually installed into a Kubernetes cluster. Nonetheless, their specs are
provided below as kubebuilder Go structs to show the available fields.

<details>

<summary>Project Metadata</summary>

```go
package v1alpha1

import (
	pkgmetav1 "github.com/crossplane/crossplane/v2/apis/pkg/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Project defines a Crossplane development project, which can be built into a
// set of installable Crossplane packages.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Project struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec *ProjectSpec `json:"spec,omitempty"`
}

// ProjectSpec is the spec for a Project.
//
// +k8s:deepcopy-gen=true
type ProjectSpec struct {
	ProjectPackageMetadata `json:",inline"`

	// Repository is the OCI repository where the project will be pushed. This
	// is used as part of the build process to construct dependencies on the
	// embedded functions.
	Repository string `json:"repository"`
	// Crossplane version constraints (optional).
	Crossplane *pkgmetav1.CrossplaneConstraints `json:"crossplane,omitempty"`
	// DependsOn contains external Crossplane package dependencies
	// (optional). These will be copied to the Crossplane package metadata as
	// part of the build process.
	DependsOn []pkgmetav1.Dependency `json:"dependsOn,omitempty"`
	// APIDependencies are used only for authoring functions, but do not need to
	// be installed as dependencies of the built packages (optional). For
	// example, this can be used to get language bindings for external
	// controllers' CRDs.
	// +optional
	APIDependencies []APIDependency `json:"apiDependencies,omitempty"`
	// Paths defines where the build tooling should look for various parts of
	// the configuration, relative to the location of the metadata
	// file. (optional).
	Paths *ProjectPaths `json:"paths,omitempty"`
	// Architectures for which to build functions (optional).
	Architectures []string `json:"architectures,omitempty"`
	// ImageConfig allows rewriting of package locations during development, for
	// example to enable use of the DevEx tools in network restricted
	// environments. Note that only a subset of Crossplane's ImageConfig
	// functionality is supported here.
	ImageConfigs []ImageConfig `json:"imageConfigs,omitempty"`
}

// ProjectPackageMetadata holds metadata about the project, which will become
// package metadata when a project is built into a Crossplane package.
type ProjectPackageMetadata struct {
	Maintainer  string `json:"maintainer,omitempty"`
	Source      string `json:"source,omitempty"`
	License     string `json:"license,omitempty"`
	Description string `json:"description,omitempty"`
	Readme      string `json:"readme,omitempty"`
}

// ProjectPaths configures the locations of various parts of the project, for
// use at build time.
type ProjectPaths struct {
	// APIs is the directory holding the project's apis. If not
	// specified, it defaults to `apis/`.
	APIs string `json:"apis,omitempty"`
	// Functions is the directory holding the project's functions. If not
	// specified, it defaults to `functions/`.
	Functions string `json:"functions,omitempty"`
	// Examples is the directory holding the project's examples. If not
	// specified, it defaults to `examples/`.
	Examples string `json:"examples,omitempty"`
	// Tests is the directory holding the project's tests. If not
	// specified, it defaults to `tests/`.
	Tests string `json:"tests,omitempty"`
	// Operations is the directory holding the project's operations. If not
	// specified, it defaults to `operations/`.
	Operations string `json:"operations,omitempty"`
}

// ImageMatch defines a rule for matching image.
type ImageMatch struct {
	// Type is the type of match.
	// +optional
	// +kubebuilder:validation:Enum=Prefix
	// +kubebuilder:default=Prefix
	Type string `json:"type"`

	// Prefix is the prefix that should be matched.
	Prefix string `json:"prefix"`
}

// ImageRewrite defines how a matched image should be rewritten.
type ImageRewrite struct {
	// Prefix is the prefix to use when rewriting the image.
	Prefix string `json:"prefix"`
}

// ImageConfig defines a set of rules for matching and rewriting images.
type ImageConfig struct {
	// MatchImages is a list of image matching rules that should be satisfied.
	// +kubebuilder:validation:XValidation:rule="size(self) > 0",message="matchImages should have at least one element."
	MatchImages []ImageMatch `json:"matchImages"`

	// RewriteImage defines how a matched image should be rewritten.
	RewriteImage ImageRewrite `json:"rewriteImage"`
}

// API dependency type constants.
const (
	// APIDependencyTypeK8s represents Kubernetes API dependencies.
	APIDependencyTypeK8s = "k8s"
	// APIDependencyTypeCRD represents Custom Resource Definition dependencies.
	APIDependencyTypeCRD = "crd"
)

// APIDependency defines a reference to an external API dependency.
type APIDependency struct {
	// Type defines the type of API dependency.
	// +kubebuilder:validation:Enum=k8s;crd
	Type string `json:"type"`

	// Git defines the git repository source for the API dependency.
	// +optional
	Git *APIGitReference `json:"git,omitempty"`

	// HTTP defines the HTTP source for the API dependency.
	// +optional
	HTTP *APIHTTPReference `json:"http,omitempty"`

	// K8s defines the Kubernetes API version for the dependency.
	// +optional
	K8s *APIK8sReference `json:"k8s,omitempty"`
}

// APIGitReference defines a git repository source for an API dependency.
type APIGitReference struct {
	// Repository is the git repository URL.
	Repository string `json:"repository"`

	// Ref is the git reference (branch, tag, or commit SHA).
	// +optional
	Ref string `json:"ref,omitempty"`

	// Path is the path within the repository to the API definition.
	// +optional
	Path string `json:"path,omitempty"`
}

// APIHTTPReference defines an HTTP source for an API dependency.
type APIHTTPReference struct {
	// URL is the HTTP/HTTPS URL to fetch the API dependency from.
	URL string `json:"url"`
}

// APIK8sReference defines a Kubernetes API version reference.
type APIK8sReference struct {
	// Version is the Kubernetes API version (e.g., "v1.33.0").
	Version string `json:"version"`
}
```

</details>

<details>

<summary>Composition Tests</summary>

```go
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// CompositionTest defines a test that runs a composition pipeline and
// executes assertions on the resulting resources.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=comptest,categories=meta
type CompositionTest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec CompositionTestSpec `json:"spec"`
}

// CompositionTestSpec defines the specification for the CompositionTest.
//
// +k8s:deepcopy-gen=true
type CompositionTestSpec struct {
	Tests []CompositionTestCase `json:"tests"`
}

// CompositionTestCase defines the specification of a single test case
//
// +k8s:deepcopy-gen=true
type CompositionTestCase struct {
	Name    string                 `yaml:"name"`              // Mandatory descriptive name
	ID      string                 `yaml:"id,omitempty"`      // Optional unique identifier
	Patches CompositionTestPatches `yaml:"patches,omitempty"` // Optional XR patching configuration
	Inputs  CompositionTestInputs  `yaml:"inputs"`            // Inputs of a test case
}

// CompositionTestPatches defines the patches for a single test case
//
// +k8s:deepcopy-gen=true
type CompositionTestPatches struct {
	// XRD specifies the XRD definition inline.
	// Optional.
	XRD runtime.RawExtension `json:"xrd,omitempty"`

	// XRD specifies the XRD definition path.
	// Optional.
	XRDPath string `json:"xrdPath,omitempty"`

	AddField    string `json:"addField,omitempty"`
	RemoveField string `json:"removeField,omitempty"`
}

// CompositionTestInputs defines the inputs for a single test case
//
// +k8s:deepcopy-gen=true
type CompositionTestInputs struct {
	// Timeout for the test in seconds
	// Required. Default is 30s.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=30
	TimeoutSeconds int `json:"timeoutSeconds"`

	// Validate indicates whether to validate managed resources against schemas.
	// Optional.
	// +kubebuilder:validation:Optional
	Validate *bool `json:"validate,omitempty"`

	// XR specifies the composite resource (XR) inline.
	// Mutually exclusive with XRPath. At least one of XR or XRPath must be specified.
	XR runtime.RawExtension `json:"xr,omitempty"`

	// XRPath specifies the composite resource (XR) path.
	// Mutually exclusive with XR. At least one of XR or XRPath must be specified.
	XRPath string `json:"xrPath,omitempty"`

	// Composition specifies the composition definition inline.
	// Optional.
	Composition runtime.RawExtension `json:"composition,omitempty"`

	// Composition specifies the composition definition path.
	// Optional.
	CompositionPath string `json:"compositionPath,omitempty"`

	// FunctionsPath specifies the functions path.
	// Optional.
	FunctionsPath string `json:"functionsPath,omitempty"`

	// ObservedResources specifies additional observed resources inline.
	// Optional.
	// +kubebuilder:validation:Optional
	ObservedResources []runtime.RawExtension `json:"observedResources,omitempty"`

	// ExtraResources specifies additional resources inline.
	// Optional.
	// +kubebuilder:validation:Optional
	ExtraResources []runtime.RawExtension `json:"extraResources,omitempty"`

	// FunctionCredentialsPath specifies a path to a credentials file to be passed to tests.
	// Optional.
	// +kubebuilder:validation:Optional
	FunctionCredentialsPath string `json:"functionCredentialsPath,omitempty"`

	// Context specifies context for the Function Pipeline inline as key-value pairs.
	// Keys are context keys, values are JSON data.
	// Optional.
	// +kubebuilder:validation:Optional
	Context map[string]runtime.RawExtension `json:"context,omitempty"`

	// Assertions defines assertions to validate resources after test completion.
	// Optional.
	// +kubebuilder:validation:Optional
	Assertions []runtime.RawExtension `json:"assertions,omitempty"`
}
```

</details>

<details>

<summary>Operation Tests</summary>

```go
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// OperationTest defines a test that runs an operation pipeline and executes
// assertions on the resulting resources.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=optest,categories=meta
type OperationTest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec OperationTestSpec `json:"spec"`
}

// OperationTestSpec defines the specification for the OperationTest.
//
// +k8s:deepcopy-gen=true
type OperationTestSpec struct {
	Tests []OperationTestCase `json:"tests"`
}

// OperationTestCase defines the specification of a single test case
//
// +k8s:deepcopy-gen=true
type OperationTestCase struct {
	Name    string               `yaml:"name"`              // Mandatory descriptive name
	ID      string               `yaml:"id,omitempty"`      // Optional unique identifier
	Inputs  OperationTestInputs  `yaml:"inputs"`            // Inputs of a test case
}

// OperationTestInputs defines the inputs for a single test case
//
// +k8s:deepcopy-gen=true
type OperationTestInputs struct {
	// Timeout for the test in seconds
	// Required. Default is 30s.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=30
	TimeoutSeconds int `json:"timeoutSeconds"`

	// Operation specifies the Operation definition inline.
	// Optional.
	Operation runtime.RawExtension `json:"operation,omitempty"`

	// OperationPath specifies the XRD definition path.
	// Optional.
	OperationPath string `json:"operationPath,omitempty"`

	// RequiredResources specifies additional required resources inline.
	// Optional.
	// +kubebuilder:validation:Optional
	RequiredResources []runtime.RawExtension `json:"requiredResources,omitempty"`

	// RequiredResourcesPath specifies a path to required resources file.
	// Optional.
	// +kubebuilder:validation:Optional
	RequiredResourcesPath string `json:"requiredResourcesPath,omitempty"`

	// FunctionCredentialsPath specifies a path to a credentials file to be passed to tests.
	// Optional.
	// +kubebuilder:validation:Optional
	FunctionCredentialsPath string `json:"functionCredentialsPath,omitempty"`

	// Context specifies context for the Function Pipeline inline as key-value pairs.
	// Keys are context keys, values are JSON data.
	// Optional.
	// +kubebuilder:validation:Optional
	Context map[string]runtime.RawExtension `json:"context,omitempty"`

	// Assertions defines assertions to validate resources after test completion.
	// Optional.
	// +kubebuilder:validation:Optional
	Assertions []runtime.RawExtension `json:"assertions,omitempty"`
}
```

</details>

<details>

<summary>E2E Tests</summary>

```go
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// E2ETest defines an end-to-end test where packages are installed into a real
// control plane instance, resources are applied, and assertions are executed
// against the resulting state. E2E tests are executed using the uptest tool.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=e2e,categories=meta
type E2ETest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec E2ETestSpec `json:"spec"`
}

// E2ETestSpec defines the specification for e2e testing of Crossplane
// configurations. It orchestrates the complete test lifecycle including setting
// up controlplane, applying test resources in the correct order (InitResources
// → Configuration → ExtraResources → Manifests), validating conditions, and
// handling cleanup. This spec allows you to define e2e tests that verify your
// Crossplane compositions, providers, and managed resources work correctly
// together in a real controlplane environment.
//
// +k8s:deepcopy-gen=true
// +kubebuilder:validation:Required
type E2ETestSpec struct {
	Tests []E2ETestCase `json:"tests"`
}

// E2ETestCase defines the specification of a single test case
//
// +k8s:deepcopy-gen=true
type E2ETestCase struct {
	Name    string         `yaml:"name"`              // Mandatory descriptive name
	ID      string         `yaml:"id,omitempty"`      // Optional unique identifier
	Inputs  E2ETestInputs  `yaml:"inputs"`            // Inputs of a test case
}

// E2ETestInputs defines the inputs for a test case.
//
// +k8s:deepcopy-gen=true
type E2ETestInputs struct {
	// CrossplaneVersion specifies the Crossplane version required for this
	// test.
	// +kubebuilder:validation:Required
	CrossplaneVersion string `json:"crossplaneVersion,omitempty"`

	// TimeoutSeconds defines the maximum duration in seconds that the test is
	// allowed to run before being marked as failed. This includes time for
	// resource creation, condition checks, and any reconciliation processes. If
	// not specified, a default timeout will be used. Consider setting higher
	// values for tests involving complex resources or those requiring multiple
	// reconciliation cycles.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=1
	TimeoutSeconds *int `json:"timeoutSeconds,omitempty"`

	// CleanupTimeoutSeconds defines the maximum duration in seconds for cleanup
	// operations after the test completes. This timeout applies to the deletion
	// of test resources and any associated managed resources. If not specified,
	// defaults to 600 seconds (10 minutes). Consider increasing this value for
	// tests with many resources or complex deletion dependencies.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=600
	CleanupTimeoutSeconds *int `json:"cleanupTimeoutSeconds,omitempty"`

	// If true, skip resource deletion after test
	// +kubebuilder:validation:Optional
	SkipDelete *bool `json:"skipDelete,omitempty"`

	// DefaultConditions specifies the expected conditions that should be met
	// after the manifests are applied. These are validation checks that verify
	// the resources are functioning correctly. Each condition is a string
	// expression that will be evaluated against the deployed resources. Common
	// conditions include checking resource status for readiness
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MinItems=1
	DefaultConditions []string `json:"defaultConditions,omitempty"`

	// Manifests contains the Kubernetes resources that will be applied as part
	// of this e2e test. These are the primary resources being tested - they
	// will be created in the controlplane and then validated against the
	// conditions specified in DefaultConditions. Each manifest must be a valid
	// Kubernetes object. At least one manifest is required. Examples include
	// Claims, Composite Resources or any Kubernetes resource you want to test.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Manifests []runtime.RawExtension `json:"manifests"`

	// ExtraResources specifies additional Kubernetes resources that should be
	// created or updated after the configuration has been successfully applied.
	// These resources may depend on the primary configuration being in place.
	// Common use cases include ConfigMaps, Secrets, providerConfigs. Each
	// resource must be a valid Kubernetes object.
	// +kubebuilder:validation:Optional
	ExtraResources []runtime.RawExtension `json:"extraResources,omitempty"`

	// InitResources specifies Kubernetes resources that must be created or
	// updated before the configuration is applied. These are typically
	// prerequisite resources that the configuration depends on. Common use
	// cases include ImageConfigs, DeploymentRuntimeConfigs, or any foundational
	// resources required for the configuration to work. Each resource must be a
	// valid Kubernetes object.
	// +kubebuilder:validation:Optional
	InitResources []runtime.RawExtension `json:"initResources,omitempty"`
}
```

</details>

[OCI layout]: https://specs.opencontainers.org/image-spec/image-layout/
[crossplane-contrib/function-python]: https://github.com/crossplane-contrib/function-python
[`ko`]: https://ko.build
[Cloud Native Buildpacks]: https://buildpacks.io/
[crossplane-contrib/function-go-templating]: https://github.com/crossplane-contrib/function-go-templating
[function-go-templating#397]: https://github.com/crossplane-contrib/function-go-templating/pull/397
[crossplane-contrib/function-kcl]: https://github.com/crossplane-contrib/function-kcl
[Upbound developer experience]: https://docs.upbound.io/manuals/cli/howtos/project/
[function-pythonic]: https://github.com/fortra/function-pythonic/
[`crossplane beta test`]: https://github.com/crossplane/crossplane/issues/6810
[xprin]: https://github.com/crossplane-contrib/xprin
[Simple Schema]: https://kro.run/docs/concepts/simple-schema/
[oapi-codegen]: https://github.com/oapi-codegen/oapi-codegen/
[JSONSchema]: https://json-schema.org/
[datamodel-code-generator]: https://github.com/koxudaxi/datamodel-code-generator
[Pydantic]: https://pydantic.dev/
[XPKG specification]: https://github.com/crossplane/crossplane/blob/main/contributing/specifications/xpkg.md#manifests
[chainsaw]: https://github.com/kyverno/chainsaw
[uptest]: https://github.com/crossplane/uptest
