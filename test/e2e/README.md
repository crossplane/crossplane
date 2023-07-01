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

```shell
# Most tests use t.Log to explain what they're doing. Use the -test.v flag
# (equivalent to go test -v) to see detailed test progress and logs.
E2E_TEST_FLAGS="-test.v" make e2e

# Some functions that setup the test environment (e.g. kind) use the klog logger
# The -v flag controls the verbosity of klog. Use -v=4 for debug logging.
E2E_TEST_FLAGS="-test.v -v=4" make e2e

# To run only a specific test, match it by regular expression
E2E_TEST_FLAGS="-test.run ^TestConfiguration" make e2e

# To test features with certain labels, use the labels flag
E2E_TEST_FLAGS="-labels area=apiextensions" make e2e

# To test a specific feature, use the feature flag
E2E_TEST_FLAGS="-feature=ConfigurationWithDependency" make e2e

# Stop immediately on first test failure, and leave the kind cluster to debug.
E2E_TEST_FLAGS="-test.v -test.failfast -destroy-kind-cluster=false"

# Use an existing Kubernetes cluster. Note that the E2E tests can't deploy your
# local build of Crossplane in this scenario, so you'll have to do it yourself.
E2E_TEST_FLAGS="-create-kind-cluster=false -destroy-kind-cluster=false -kubeconfig=$HOME/.kube/config"

# Run the CrossplaneUpgrade feature, against an existing kind cluster named
# "kind" (or creating it if it doesn't exist), # without installing Crossplane
# first, as the feature expects the cluster to be empty, but still loading the
# images to # it. Setting the tests to fail fast and not destroying the cluster
# afterward in order to allow debugging it.
E2E_TEST_FLAGS="-test.v -v 4 -test.failfast \
  -destroy-kind-cluster=false \
  -kind-cluster-name=kind \
  -install-crossplane=false \
  -feature=CrossplaneUpgrade" make e2e

# Run the all tests not installing or upgrading Crossplane against the currently
# selected cluster where Crossplane has already been installed.
E2E_TEST_FLAGS="-test.v -v 4 -test.failfast \
  -kubeconfig=$HOME/.kube/config \
  -skip-labels modify-crossplane-installation=true \
  -create-kind-cluster=false \
  -install-crossplane=false" make go.build e2e-run-tests
```

## Test Parallelism

`make e2e` runs all defined E2E tests serially. Tests do not run in parallel.
This is because all tests run against the same API server and Crossplane has a
lot of cluster-scoped state - XRDs, Providers, Compositions, etc. It's easier
and less error-prone to write tests when you don't have to worry about one test
potentially conflicting with another - for example by installing the same
provider another test would install.

In order to achieve some parallelism at the CI level all tests are labelled with
an area (e.g. `pkg`, `install`, `apiextensions`, etc). The [CI GitHub workflow]
uses a matrix strategy to invoke each area as its own job, running in parallel.

## Adding a Test

> We're still learning what the best way to arrange E2E tests is. It's okay for
> this pattern to change if it's not working well, but please discuss first!

We try to follow this pattern when adding a new test:

1. Define a single feature per Test function, if possible.
1. Setup a directory of plain YAML manifests per test - i.e. test fixtures - at
   `e2e/manifests/<area>/<test>`, usually with a `setup` sub-folder
   containing resources to be deployed at setup phase and cleaned up during the
   teardown. Try to avoid reusing other feature's fixtures, as this would introduce
   hidden dependencies between tests.
1. Try reusing existing helpers as much as possible, see package
   `github.com/crossplane/crossplane/test/e2e/funcs`, or add new ones there if
   needed.
1. Prefer using the Fluent APIs to define features
   (`features.New(...).WithSetup(...).Assess(...).WithTeardown(...).Feature()`).
   1. `features.Table` should be used only to define multiple self-contained
      assessments to be run sequentially, but without assuming any ordering among
      them, similarly to the usual table driven style we adopt for unit testing.
1. Prefer the usage of `WithSetup` and `WithTeardown` to their unnamed
   counterparts (`Setup` and `Teardown`) to define the setup and teardown phases of
   a feature, as they allow to provide a description.
1. Use short but explicative `CamelCase` sentences as descriptions for
   everything used to define the name of tests/subtests, e.g.
   `features.New("CrossplaneUpgrade", ...)` `WithSetup("InstallProviderNop",
   ...)`, `Assess("ProviderNopIsInstalled", ...)`,
   `WithTeardown("UninstallProviderNop", ...)`.
1. Use the `Setup` and `Teardown` phases to define respectively actions that are
   not strictly part of the feature being tested, but are needed to make it
   work, and actions that are needed to clean up the environment after the test
   has run.
1. Use `Assess` steps to define the steps required to exercise the actual
   feature at hand.
1. Use `Assess` steps to define both conditions that should hold and actions that
   should be performed. In the former case use active descriptions, e.g.
   `InstallProviderNop`, while in the latter use passive descriptions, e.g.
   `ProviderNopIsInstalled`.
1. Try to group actions and the checks of what you have done `Assess` step in a
   single step with an active description if possible, to avoid having twice the
   steps and making it explicit that we are checking the action executed by the
   previous function. e.g. `"UpgradeProvider"` should both upgrade the provider
   and check that it  becomes healthy within a reasonable time.
1. Avoid using the available context to pass data between steps, as it makes it
   harder to understand the flow of the test and could lead to data races if not
   handled properly.
1. Keep in mind that all `Setup` and `Teardown` steps, wherever are defined are
   always going to be executed respectively before and after all the `Assess`
   steps defined, so you can define `Teardowns` immediately after the step that defined the
   resource to be deleted as a sort of `defer` statement. Same applies to `Setup`
   steps which could actually be located immediately before the step requiring
   them. But be careful with this, as a non-linear narrative is going to be easier
   to follow, so if possible stick to all Setups at the beginning and all Teardowns
   at the end of the feature.
1. Features can be assigned labels, to allow dicing and slicing the test suite,
   see below for more details about available labels, but overall try to define
   all the labels that could be useful to select the test in the future and make
   sure it's actually being selected when run in CI.

Here an example of a test following the above guidelines:

```go
package e2e

// ...

// TestSomeFeature ...
func TestSomeFeature(t *testing.T) {
	manifests := "test/e2e/manifests/pkg/some-area/some-feature"
	namespace := "some-namespace"
	// ... other variables or constants ...

	environment.Test(t,
		features.New("ConfigurationWithDependency").
			WithLabel(LabelArea, ...).
			WithLabel(LabelSize, ...).
            // ...
			WithSetup("ReadyPrerequisites", ... ).
            // ... other setup steps ...
			Assess("DoSomething", ... ).
			Assess("SomethingElseIsInSomeState", ... ).
			// ... other assess steps ...
			WithTeardown("DeleteCreatedResources", ...).
			// ... other teardown steps ...
			Feature(),
	)
}

// ...
```

### Features' Labels

Features are grouped into broad feature areas - e.g. `TestComposition` in
`composition_test.go`. Features pertaining to Composition should be added to
`TestComposition`.

Some tests may involve updating an existing test - for example you might test a
new kind of transform by updating the `composition/patch-and-transform`
manifests and adding a new assessment or two to the `pandt` `features.Table`.

Other, larger tests may involve creating a new directory of manifests and a new
`features.Table`. Every `features.Table` must be passed to `environment.Test` to
be run.

When you pass a feature to `environment.Test` you can add arbitrary labels that
may be used to filter which tests are run. Common labels include:

* `area`: The area of Crossplane being tested - `pkg`, `apiextensions`, etc.
* `size`: `small` if the test completes in under a minute, otherwise `large`.

If you add a new `area` label, be sure to add it to the matrix strategy of the
e2e-tests job in the [CI GitHub workflow]. We run E2E tests for each area
of Crossplane in parallel.

When adding a test:

* Use commentary to explain what your tests should do.
* Use brief `CamelCase` test names, not descriptive sentences.
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

Refer to the [E2E one-pager] for more context.

[CI GitHub workflow]: ../../.github/workflows/ci.yml
[`testing`]: https://pkg.go.dev/testing
[`e2e-framework`]: https://pkg.go.dev/sigs.k8s.io/e2e-framework
[E2e one-pager]: ../../design/one-pager-e2e-tests.md
