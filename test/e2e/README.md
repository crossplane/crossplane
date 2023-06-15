# Crossplane End-to-end Tests

This directory contains Crossplane's E2E tests. These tests exercise Crossplane
features by:

1. Creating a `kind` cluster.
1. Deploying Crossplane, using its Helm chart.
1. Applying a series of YAML manifests.
1. Testing whether the resulting resources work as expected.

These tests focus on core Crossplane features - i.e. the package manager,
Composition, etc. They should not take dependencies on 'real' Crossplane
providers or external systems. Instead they use 'fake' providers like
`provider-nop` and `provider-dummy`.

All Crossplane features must be exercised by these tests, as well as unit tests. 

## Running Tests

Run `make e2e` to run E2E tests.

This compiles Crossplane and an E2E test binary. It then runs the test binary.
Use the `E2E_TEST_FLAGS` to pass flags to the test binary. For example:

```console
# Most tests use t.Log to explain what they're doing. Use the -test.v flag
# (equivalent to go test -v) to see detailed test progress and logs.
E2E_TEST_FLAGS="-test.v" make e2e

# Some functions that setup the test environment (e.g. kind) use the klog logger
# The -v flag controls the verbosity of klog. Use -v=4 for debug logging.
E2E_TEST_FLAGS="-test.v -v=4"
```

## Adding a Test

Each feature under test consists of:

1. A directory of manifests - i.e. test fixtures.
1. A `features.Table` of assessment steps.

Features are grouped into broad feature areas - e.g. `TestComposition` in
`composition_test.go`. Features pertaining to Composition should be added to
`TestComposition`.

Some tests may involve updating an existing test - for example you might test a
new kind of transform by updating the `composition/patch-and-transform`
manifests and adding a new assessment or two to the `pandt` `features.Table`.

Other, larger tests may involve creating a new directory of manifests and a new
`features.Table`.

When adding a test:

* Use commentary to explain what your tests should do.
* Use `CamelCase` test names.
* Avoid adding complex logic in the `e2e` package.
* Implement new test logic as a `features.Func` in the `e2e/funcs` package.

## Design Principals

The goals of these tests are to:

* Simulate real-world use to exercise all of Crossplane's functionality.
* Run reliably, with no flaky tests.
* Converge on requiring mostly updated manifests - and little Go - to add tests.
* Use tools and patterns that are familiar and unsurprising to contributors.

The following design principals help achieve these goals:

* Use familiar Kubernetes YAML manifests as test fixtures.
* Provide a library of reusable test functions (e.g. `funcs.ResourcesCreatedIn`).
* Remain as close to idiomatic Go stdlib [`testing`] tests as possible.
* Use common, idiomatic Kubernetes ecosystem tooling - [`e2e-framework`].
* Implement as much _logic_ in Go as possible - avoid scripts or shelling out.

[`testing`]: https://pkg.go.dev/testing
[`e2e-framework`]: https://pkg.go.dev/sigs.k8s.io/e2e-framework