# End-to-end Testing in Crossplane

* Owner: Predrag Knežević (@pedjak), Lovro Sviben (@lsviben)
* Reviewers: @negz, @hasheddan
* Status: Draft

## Background

As a step in maturing Crossplane, we should be thinking about setting up
processes and methodology that would enable automated end-to-end testing,
covering all user-facing functionalities.

Currently, as reported in [this issue], there seems to be a lack of e2e tests
for Crossplane, impacting directly confidence in changes submitted in PRs and
the quality of published releases.

The following repos contains some sort of e2e tests:

* [crossplane/test], last test change was 9 month ago
* Crossplane repo itself [crossplane/crossplane e2e], last update was 10 months
  ago
* [crossplane/conformance], last change in June, 2021

Apart for the stale development, the existing tests:

* Cover only tiny fraction of all user-facing features
* Do not cover any error cases
* Are reported to be flaky sometimes

The tests are written in pure Go, requesting to keep the number of library
dependencies at minimum.

Contrary to unit tests, end-to-end testing requires very often complex test 
fixtures (e.g. installing Crossplane, providers, reaching some state in the
cluster) before we can focus on validating a particular system behavior.
Defining the fixture only with the help of Go standard library can be
time-consuming, requiring to write a significant amount of boiler-plate code,
hence reducing test readability and maintenance, and eventually not very
motivating for creating new tests. In order to improve the readability, one
would start looking for patterns and either eventually end up with a custom
framework or adopt an existing one.

**_NOTE_**: The document uses end-to-end testing term interchangeably
with [system testing].

## Goals

Going forward we would like to explore and propose using an e2e testing
framework, preferably one that would be already familiar to the community.
The chosen framework should poses the following properties:

* Support for declarative tests.

  Similar to kubernetes object management, each test action should specify what
  is the system state we would like to reach/check against. Such approach allows 
  us to test effectively business rules exposed to users

* Excellent readability for everybody (developers, users, Crossplane community
  in general)

  This would enable all stakeholders to have better/clear understanding about
  the system behaviors.

* Black-box testing.

  The end-to-end tests should treat the system as a black box, and do not rely
  on any implementation details/internal knowledge. In order to interact with
  the system, we should use only methods/tools available and advocated to users.
  In this way, users would be able to run these tests against their system
  instances, either manually or in an automated way.

* Parallel test execution.

  End-to-end tests can take significantly more time to complete then unit tests,
  and in order to keep CI jobs within acceptable limits, it should be possible
  to execute tests in parallel against the same system instance

* Run tests against an arbitrary system deployment.

  In this way we can assure that Crossplane works properly in a number of k8s
  clusters, deployed within a number of cloud providers

* Test code reusability/maintainability/ease of development

  Effort for adding a new test should decrease with the number of already
  available tests, because we should be able to reuse the same/similar setup and
  assertion logic. Similar to that, global changes in setup/assertion logic
  should be kept in the single place ([DRY principle])

As a part of this effort, already existing tests should be converted to the
chosen approach/framework.

## Non-Goals

* Further followup actions should introduce more tests covering all Crossplane
  core user-facing functionalities
    * Package Manager
        * Providers install/remove/update
        * Configurations install/remove/update
        * Error scenarios
    * Composition Engine
        * XRD createl/remove/update → verify effects on CRD
        * Composition install/remove/update →effects on CompositionRevisions
        * Create/update/delete a claim → verify the xr and mr and resource
          readiness
        * Patching → should have a patch of each type tested
        * ConnectionDetails → verify the secret
        * Late initialization → verify its propagated from mr->xr->claim
        * Composition functions
        * Composition validation
        * ESS
        * ObserveOnly scenarios
        * Error scenarios
    * Crossplane upgrades
* Setting up CI to run e2e tests as a pre-merge checks

These non-goals are important but out of the scope of this proposal.

## Proposal

We would like to propose the adoption of “Specification by Example”
([wikipedia], [book]) methodology by defining tests using [Cucumber] framework
and its [godog] bindings (2k stars on GitHub) for managing Crossplane
end-to-end/acceptance tests. [Cucumber wiki] quote: _“Cucumber is a software
tool that supports behavior-driven development (BDD). Central to the Cucumber
BDD approach is its ordinary language parser called [Gherkin]. It allows
expected software behaviors to be specified in a logical language that customers
can understand. As such, Cucumber allows the execution of feature documentation
written in business-facing text”. _

Cucumber is a mature project/approach used successfully for testing systems of
various complexity and size, for example:

* [Service Binding Operator]
* [Conformance tests for servicebinding specification]
* [Primaza]
* [Elastic end-to-end testing]

Gherkin language defines just a handful of keywords and its purpose is to
improve the readability of feature files. Other than that, the test authors have
a full flexibility to define a human oriented language that describes test
actions.

### Anatomy of Cucumber Feature

* Consist of a number of scenarios
* Each scenario consists of sequential number of steps
    * Each step describes an interaction with the system as a blackbox, using
      only tools and clients available to
      users (e.g. kubectl)
    * Each step is described with a human readable sentence, not exposing system
      implementation details
    * Steps grammar is defined by us, i.e. the language grows with the system
      features
    * Steps can be parametrized
    * Steps are reusable across many scenarios/features
* Support for table driven tests
* Support for common fixture management in single place

### Steps Implementations

Cucumber has bindings for [all popular languages]. [Go] bindings enable native
integration into [go testing]. The project is very mature and has a large
community. In essence:

* Each scenario step maps to a Go function, enabling us even to use additional
  go libraries where appropriate
* A common context can be used to share state between test steps (e.g. to
  further simplify step naming)
* Through a number of different hooks, arbitrary code could be injected into
  test workflow, e.g. to create some common
  fixtures or perform required cleanups without cluttering the test description
* A number of extensions available (e.g. [kubedog], [godox])


* The tests are executed using the standard Go approach → they could be easily
  triggered/debugged from various IDEs as well.
* The amount of Go code implementing steps is very low, and yet the implemented
  steps are enough for declaring a large number of test cases

### Declarative Testing

Cucumber allows us to perform [both imperative and declarative testing].
Thanks to the ability to define a grammar that fits our needs, we decided to
craft a declarative one. Here are some steps:

* Crossplane is running in cluster
* Provider <name> is running in cluster
* CompositeResourceDefinition is present
* Composition is present
* Claim gets deployed
* Claim becomes synchronized and ready
* Claim composite resource becomes synchronized and ready
* Composed managed resources become ready and synchronized

Example scenario:

```gherkin
Scenario: composite resource get ready and synchronized
 Given Crossplane is running in cluster
 And provider crossplane/provider-nop:main is running in cluster
 And CompositeResourceDefinition is present
   """
     apiVersion: apiextensions.crossplane.io/v1
     kind: CompositeResourceDefinition
     metadata:
       name: clusternopresources.nop.example.org
     spec:
       group: nop.example.org
       names:
         kind: ClusterNopResource
         listKind: ClusterNopResourceList
         plural: clusternopresources
         singular: clusternopresource
       claimNames:
         kind: NopResource
         listKind: NopResourceList
         plural: nopresources
         singular: nopresource
       connectionSecretKeys:
         - test
       versions:
         - name: v1alpha1
           served: true
           referenceable: true
           schema:
             openAPIV3Schema:
               type: object
               properties:
                 spec:
                   type: object
                   properties:
                     coolField:
                       type: string
                   required:
                     - coolField
   """
 And Composition is present
   """
     apiVersion: apiextensions.crossplane.io/v1
     kind: Composition
     metadata:
       name: clusternopresources.sample.nop.example.org
       labels:
         provider: provider-nop
     spec:
       compositeTypeRef:
         apiVersion: nop.example.org/v1alpha1
         kind: ClusterNopResource
       resources:
         - name: nopinstance1
           base:
             apiVersion: nop.crossplane.io/v1alpha1
             kind: NopResource
             spec:
               forProvider:
                 conditionAfter:
                   - conditionType: Ready
                     conditionStatus: "False"
                     time: 0s
                   - conditionType: Ready
                     conditionStatus: "True"
                     time: 10s
                   - conditionType: Synced
                     conditionStatus: "False"
                     time: 0s
                   - conditionType: Synced
                     conditionStatus: "True"
                     time: 10s
               writeConnectionSecretsToRef:
                 namespace: crossplane-system
                 name: nop-example-resource
         - name: nopinstance2
           base:
             apiVersion: nop.crossplane.io/v1alpha1
             kind: NopResource
             spec:
               forProvider:
                 conditionAfter:
                   - conditionType: Ready
                     conditionStatus: "False"
                     time: 0s
                   - conditionType: Ready
                     conditionStatus: "True"
                     time: 10s
                   - conditionType: Synced
                     conditionStatus: "False"
                     time: 0s
                   - conditionType: Synced
                     conditionStatus: "True"
                     time: 10s
               writeConnectionSecretsToRef:
                 namespace: crossplane-system
                 name: nop-example-resource
   """
 When claim gets deployed
   """
     apiVersion: nop.example.org/v1alpha1
     kind: NopResource
     metadata:
       name: nop-example
     spec:
       coolField: example
   """
 Then claim becomes synchronized and ready
 And claim composite resource becomes synchronized and ready
 And composed managed resources become ready and synchronized
```

Step "claim gets deployed" is implemented in the following Go function:
```go
func claimGetsDeployed(ctx context.Context, rawYaml *godog.DocString) (context.Context, error) {
	sc := ScenarioContextValue(ctx)
	claim, err := ToUnstructured(rawYaml.Content)
	if err != nil {
		return ctx, err
	}
	sc.Claim = claim
	return ctx, sc.Cluster.ApplyYamlToNamespace(sc.Namespace, rawYaml.Content)
}
```

These functions are then bound to steps at the initialization:
```go
func InitializeScenario(ctx *godog.ScenarioContext) {
	ctx.Before(func(ctx context.Context, scenario *godog.Scenario) (context.Context, error) {
		sc := &scenarioContext{
			Cluster: &cluster{
				cli: "kubectl",
			},
			Namespace: fmt.Sprintf("test-%s", scenario.Id),
		}
		if err := sc.Cluster.createNamespace(sc.Namespace); err != nil {
			return ctx, err
		}
		ctx = context.WithValue(ctx, scenarioContextKey, sc)
		return ctx, nil
	})
	ctx.Step(`^claim becomes synchronized and ready$`, claimBecomesSynchronizedAndReady)
	ctx.Step(`^claim composite resource becomes synchronized and ready$`, claimCompositeResourceBecomesSynchronizedAndReady)
	ctx.Step(`^claim gets deployed$`, claimGetsDeployed)
	ctx.Step(`^composed managed resources become ready and synchronized$`, composedManagedResourcesBecomeReadyAndSynchronized)
	ctx.Step(`^CompositeResourceDefinition is present$`, clusterScopedResourceIsPresent)
	ctx.Step(`^Composition is present$`, clusterScopedResourceIsPresent)
	ctx.Step(`^Configuration is applied$`, configurationGetsDeployed)
	ctx.Step(`^configuration is marked as installed and healthy$`, configurationMarkedAsInstalledAndHealthy)
	ctx.Step(`^Crossplane is running in cluster$`, crossplaneIsRunningInCluster)
	ctx.Step(`^provider (\S+) does not get installed$`, providerNotInstalled)
	ctx.Step(`^provider (\S+) is marked as installed and healthy$`, providerMarkedAsInstalledAndHealthy)
	ctx.Step(`^provider (\S+) is running in cluster$`, providerGetsInstalled)
}
```
Additional hooks and context are registered as well.

Finally, the tests are triggered as regular go tests:

```go
var opts = godog.Options{
	Output: colors.Colored(os.Stdout),
	Format: "pretty",
}

func init() {
	godog.BindFlags("godog.", flag.CommandLine, &opts)
}

func TestE2E(t *testing.T) {
	o := opts
	o.TestingT = t

	status := godog.TestSuite{
		ScenarioInitializer: InitializeScenario,
		Options:             &o,
	}.Run()

	if status == 2 {
		t.SkipNow()
	}

	if status != 0 {
		t.Fatal("non-zero status returned, failed to run feature tests")
	}
}
```
The existing e2e tests have been converted to Cucumber scenarios, and they could be executed with:

```shell
make build e2e
```

When it comes to choosing a provider to run tests against, we currently have the
[NOP provider] used by the current e2e tests. An alternative provider could be
[provider-dummy]. We believe that most of the tests could be even written so
that they do not require any provider to be installed, i.e. it would be enough
to install some carefully crafted CRDs, and create XRDs and claims against them.
Such a strategy would make required setups even lighter and tests would
potentially run faster.

### Blackbox Testing

Steps implementation communicates with the cluster only by using [kubectl CLI].
In this way we can be sure that users are able to reproduce tests either
manually or in an automated way.

### Benefits

* Feature files and scenarios are easy to read and understand, all needed
  details are in a single place, written in natural English sentences. Required 
  k8s resources are represented in their YAML form.
* Scenarios become a communication medium between all stakeholders and it gets
  easy to add new test cases even for non-developers.
* Testable documentation: examples/snippets provided in the documentation can be
  implemented as scenarios and then included back into documentation using some
  tooling like [embedme],  [AsciiDoc] or via [GitHub permalink].
* As mentioned above [godog] project has built-in support for running tests in
  [parallel]
* Full control on how a step is implemented, no strongly opinionated frameworks
  in the way

### Drawbacks

Learning a few new concepts and getting familiar with godog API. However, the
learning curve is very lean.

## Alternatives Considered

### KUTTL

The KUbernetes Test TooL ([KUTTL]) provides a yaml-driven declarative approach
to testing production-grade Kubernetes. Kuttl is already used in [Uptest], which
provides e2e test generation for providers. In general it's a cool tool, but it
has some limitations.

#### Anatomy of a KUTTL test

1. Setup step
2. Assert step
3. Repeat
4. Each of tests step is placed in a separate file, and filenames need to follow
   a naming convention, impacting test flow
5. At the end of the tests, kuttl cleans up all the resources created during the
   test (unless specified otherwise).

Although the framework aims to deliver declarative testing experience, there are
a number of test scenarios where just deploying resources and asserting their
content is not sufficient to describe user-facing system functionalities. Hence,
the framework introduces a dedicated TestStep (not deployed in cluster) resource
that is then used for invoking scripts (e.g. kubectl) with arbitrary imperative
logic. At the end, the defined tests are neither fully declarative or
imperative, i.e. we end up with mixed abstractions.

Example:

00-provider.yaml:

```yaml
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-dummy
spec:
  package: xpkg.upbound.io/upbound/provider-dummy:v0.3.0
```

00-assert.yaml:

```yaml
apiVersion: pkg.crossplane.io/v1
kind: ProviderRevision
metadata:
  labels:
    pkg.crossplane.io/package: provider-dummy
spec:
  revision: 1
status:
  conditions:
    - reason: HealthyPackageRevision
      status: "True"
      type: Healthy
```

The example is available here: [Kuttl POC].

#### Blackbox testing

The communication with the cluster where Crossplane is installed happens via
kubectl CLI.

#### Readability

KUTTL test consists of a bunch of yaml files. Although yaml format is pretty
readable, by looking at a file it is harder to understand the context or
intention. In order to improve that, additional comments are needed in the file,
and exactly those kinds of comments are essentially Cucumber steps. Furthermore,
following the test flow requires understanding the naming convention and
switching from file to file. Opposite to this approach, Cucumber scenarios are
self-contained: all test actions and resources are kept in the single place,
ensuring that tests are easily readable.

#### Test code reusability/maintainability/ease of development

Initially it's very easy to write the first test, but as the test base grows, it
becomes harder to maintain them. The issue is that we have to repeat the setup
steps in every test, so we will have a bunch of yaml files that are repeated
across tests. If something changes in the setup, we will have to update all the
tests that use it.

#### Conclusion

In conclusion, we think that kuttl is a great tool, but it's not the best fit
for our use case. As we could see, it is not fully declarative. For a project
the size of Crossplane we need something that is easy to develop, readable and
maintainable in the long run.

### E2E-framework

[kubernetes-sig/e2e-framework] is a new framework for writing imperative
end-to-end tests for Kubernetes.

#### Anatomy of a test

Tests are organized in test suites, each test suite being a Go package. The
function TestMain is used to define package-wide testing steps and configure 
behavior. The framework exposes the Environment interface, which is used to
interact with the cluster.

Furthermore, the environment is set up by defining steps that are executed in
different phases of the test, like Setup, Finish, BeforeEach, kinda similar to
Ginkgo.

TestMain:

```go
var testenv *env.Environment

func TestMain(m *testing.M) {
	testenv = env.NewInClusterConfig()

	testenv.Setup(
		setupSchema,
		setupDummyProvider,
	)

	testenv.Finish(
		teardownDummyProvider,
	)

	// launch package tests
	os.Exit(testenv.Run(m))
}
```

The tests themselves are organized in Features for which steps (Setup, Assess,
Teardown) need to be defined.

Test example:

```go
func TestFlow(t *testing.T) {

	f := features.New("create claim flow").
		WithLabel("type", "flow-claim").
		WithSetup("install the XRD", setupXRD).
		WithTeardown("teardown the XRD", teardownXRD).
		WithSetup("install the Composition", setupComposition).
		WithTeardown("teardown the Composition", teardownComposition).
		WithSetup("xrd is established and offered", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			xrd := genXRD()
			err := wait.For(conditions.New(cfg.Client().Resources()).ResourceMatch(xrd, func(object k8s.Object) bool {
				o := object.(*extv1.CompositeResourceDefinition)
				return o.Status.GetCondition(extv1.TypeEstablished).Status == corev1.ConditionTrue &&
					o.Status.GetCondition(extv1.TypeOffered).Status == corev1.ConditionTrue
			}), wait.WithTimeout(time.Minute*1))
			if err != nil {
				t.Fatalf("failed to wait for XRD to be established and offered: %v", err)
			}
			return ctx
		}).
		Setup(setupClaim).
		Teardown(teardownClaim).
		Assess("claim", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			claim, err := getClaim()
			if err != nil {
				t.Fatalf("failed to get claim: %v", err)
			}
			err = wait.For(conditions.New(cfg.Client().Resources()).ResourceMatch(claim, func(object k8s.Object) bool {
				claimObj := composed.Unstructured{Unstructured: *claim}
				isReady := claimObj.GetCondition(xpv1.TypeReady)
				if isReady.Status != corev1.ConditionTrue {
					t.Logf("claim %q is not yet Ready", claim.GetName())
					return false
				}
				return isReady.Status == corev1.ConditionTrue
			}), wait.WithTimeout(time.Minute*1))
			if err != nil {
				t.Fatalf("failed to wait for claim to be Ready: %v", err)
			}
			return ctx
		}).Feature()

	// test feature
	testenv.Test(t, f)
}
...
```

The test steps are run in the order based on their level (Setup, Assess,
Teardown).

The example is available here: [e2e-framework POC]

#### Blackbox testing

The framework is using the controller-runtime client underneath, wrapped under
its `klient`, which offers some abstractions to make testing easier. In order to
deploy Crossplane resources, we would need to register Crossplane schemas within
the client. Such a setup is more convenient for running integration tests. For
end-to-end testing we should simulate users' interaction with the system and
that happens very often (as advocated in Crossplane doc)typically using kubectl
CLI.

#### Readability

Depending on the creativity of the developer, the tests could be as readable as
the Go unit tests, the trick here being hiding the complexity of setting up the
test environment under TestMain and Feature setup.

When the setup is more complex and features like BeforeEach are needed, it would
get complicated to understand and debug the setup itself.

#### Test code reusability/maintainability/ease of development

We should be able to reuse similar setup step functions across tests. The
difficult part here will be to set up the test environment.

As there are only 3 levels of steps in a feature, when we need to do something
like:
1. setup a resource
2. assert that the resource is in a certain state
3. setup another resource based on the first one
4. assert that the second resource is in a certain state

there is no way to do this from what we saw, as the setup steps are run before
the assert steps. So we would need to do 1-3 for example in setup steps. Other
than that, the framework has a lot of helper functions that help us write tests
faster, like inbuilt wait for conditions, or a more simplified way to use
controller-runtime/client.

The framework, although not very popularized yet, is being developed actively,
but is still in an early phase (0.2.0 version and ~300 stars on GitHub). To make
it clear, this is not the e2e testing framework that is used in Kubernetes
upstream e2e tests. Rather, it's an effort to make projects using Kubernetes
have a standardized and convenient way to test. From what we see, it is for now
used by Cilium.

#### Conclusion

The e2e-framework might be a good candidate for writing integration tests, but
it is not well-suited for declarative e2e testing. Given the nature of e2e
tests, the setup might be quite complex, requiring a lot of discipline to keep
code readable and maintainable.

### Terratest

[Terratest] is a Go library that makes it easier to write automated imperative
tests for your infrastructure. It provides a variety of helper functions and
patterns for common infrastructure testing tasks, including Terraform, Docker,
providers like GCP, AWS, Azure, Helm, and Kubernetes.

#### Anatomy of a test

It’s a very lightweight framework, as we only get some helper functions, and as
for the test structure, we are left to our own devices.

The provided helper functions allow us to apply a resource managed in a file to
the cluster or run arbitrary kubectl commands.

Example test:
```go
func TestBasicClaimFlow(t *testing.T) {
	t.Parallel()

	// create specific namespace for the test
	namespaceName := fmt.Sprintf("kubernetes-basic-example-%s", strings.ToLower(random.UniqueId()))
	options := k8s.NewKubectlOptions("", "", namespaceName)

	k8s.CreateNamespace(t, options, namespaceName)
	defer k8s.DeleteNamespace(t, options, namespaceName)

	// ensure that the provider is deployed and ready
	applyFromPath(t, "testData/provider.yaml", options)
	if err := waitForConditionStatus(t, "providers", "provider-dummy", namespaceName, installedAndHealthyConditions, ""); err != nil {
		t.Fatalf("Error waiting for provider to be installed and healthy: %v", err)
	}
	applyFromPath(t, "testData/providerconfig.yaml", options)

	applyFromPath(t, "testData/deployment.yaml", &k8s.KubectlOptions{Namespace: "crossplane-system"})
	applyFromPath(t, "testData/service.yaml", &k8s.KubectlOptions{Namespace: "crossplane-system"})

	// install the xrd
	applyFromPath(t, "testData/xrd.yaml", options)
	if err := waitForConditionStatus(t, "xrd", "xrobots.dummy.crossplane.io", namespaceName, establishedAndOfferedConditions, ""); err != nil {
		t.Fatalf("Error waiting for xrd to be installed and healthy: %v", err)
	}
	// install the composition
	applyFromPath(t, "testData/composition.yaml", options)
	// apply claim
	applyFromPath(t, "testData/claim.yaml", options)

	// check if claim is ready and synced
	if err := waitForConditionStatus(t, "claim", "test-robot", namespaceName, syncAndReadyConditions, ""); err != nil {
		t.Fatalf("Error waiting for claim to be ready and synced: %v", err)
	}

	// check if composite is ready and synced
	if err := waitForConditionStatus(t, "composite", "", namespaceName, syncAndReadyConditions, fmt.Sprintf("crossplane.io/claim-namespace=%s", namespaceName)); err != nil {
		t.Fatalf("Error waiting for composite to be ready and synced: %v", err)
	}
}
...
func applyFromPath(t *testing.T, path string, options *k8s.KubectlOptions) {
    absPath, err := filepath.Abs(path)
    if err != nil {
        t.Fatalf("Error getting path: %v", err)
    }

    k8s.KubectlApply(t, options, absPath)
}
```

Example can be found here: [Terratest POC].

#### Blackbox Testing

As the helper functions that we would use are those that use kubectl underneath,
the condition of implementing blackbox tests in this case is true.

#### Readability

These tests are not a big improvement over the initial basic go tests that we
already have except that we would get some nice helper functions. Still the code
to set the environment up would be complex, and the tests themselves might not
be that readable.

#### Test code reusability/maintainability/ease of development

Similarly to the previous point, the framework does not bring a revolution
compared to the basic Go tests we already have, although by providing handy
helper functions it hides away a lot of the complexity.

#### Conclusion

It’s a really neat project, which is quite mature and popular. We could use it
as the library within the Cucumber step implementation. By itself, it does not
provide enough structure to make the tests better organized and readable. But it
could be a great choice if we decide that we don’t want to introduce any kind of
opinionated framework.

## Framework Comparison Matrix

| Goal                      |                                  Cucumber/Godog                                   |                                          KUTTL                                          |              k8s e2e-framework              |                  Terratest                  |
|---------------------------|:---------------------------------------------------------------------------------:|:---------------------------------------------------------------------------------------:|:-------------------------------------------:|:-------------------------------------------:|
| Declarative Tests         |                                        yes                                        |                                   yes to some extent                                    |                     no                      |                     no                      |
| Readability               | Excellent, test steps written in pure English, embedding yaml content when needed | Hard to follow flow, need to switch from file to file and understand naming conventions |      Readable as any Go code + own DSL      |      Readable as any Go code + own DSL      |
| Black-box Testing         |                                        yes                                        |                                           yes                                           |               uses client-go                |                     yes                     |
| Parallel Test Execution   |                                        yes                                        |                                           yes                                           |                     yes                     |                     yes                     |
| Test code reusability     |                high, test steps can be used across many scenarios                 |                         low, requires file copying when needed                          | common code need to be wrapped in functions | common code need to be wrapped in functions |
| Test code maintainability |    High. easy change of step behaviour is reflected automatically in all tests    |                          Low, all resources need to be edited                           |         depends on test code design         |         depends on test code design         |


[this issue]: https://github.com/crossplane/crossplane/issues/4013
[crossplane/test]: https://github.com/crossplane/test
[crossplane/crossplane e2e]: https://github.com/crossplane/crossplane/tree/master/test/e2e
[crossplane/conformance]: https://github.com/crossplane/conformance
[DRY principle]: https://en.wikipedia.org/wiki/Don%27t_repeat_yourself
[wikipedia]: https://en.wikipedia.org/wiki/Specification_by_example
[book]: https://www.manning.com/books/specification-by-example
[system testing]: https://en.wikipedia.org/wiki/System_testing
[Cucumber]: https://cucumber.io/docs/installation/
[godog]: https://github.com/cucumber/godog
[Cucumber wiki]: https://en.wikipedia.org/wiki/Cucumber_(software)
[Gherkin]: https://cucumber.io/docs/gherkin/reference/
[Service Binding Operator]: https://github.com/redhat-developer/service-binding-operator
[Conformance tests for servicebinding specification]: https://github.com/servicebinding/conformance
[Primaza]: https://github.com/primaza/primaza
[Elastic end-to-end testing]: https://github.com/elastic/e2e-testing
[all popular languages]: https://cucumber.io/docs/installation/
[Go]: https://github.com/cucumber/godog
[go testing]: https://pkg.go.dev/testing
[kubedog]: https://github.com/keikoproj/kubedog
[godox]: https://github.com/godogx
[Cucumber POC]: https://github.com/pedjak/crossplane/tree/cucumber-godog-demo/test/acceptance
[both imperative and declarative testing]: https://itsadeliverything.com/declarative-vs-imperative-gherkin-scenarios-for-cucumber
[Composition e2e]: https://github.com/pedjak/crossplane/blob/cucumber-godog-demo/test/acceptance/features/composition.feature
[Configuration e2e]: https://github.com/pedjak/crossplane/blob/cucumber-godog-demo/test/acceptance/features/configurationPackages.feature
[NOP provider]: https://github.com/crossplane-contrib/provider-nop
[provider-dummy]: https://github.com/upbound/provider-dummy
[kubectl CLI]: https://kubernetes.io/docs/reference/kubectl/
[embedme]: https://github.com/zakhenry/embedme  
[AsciiDoc]: https://docs.asciidoctor.org/asciidoc/latest/directives/include-tagged-regions/
[GitHub permalink]: https://docs.github.com/en/get-started/writing-on-github/working-with-advanced-formatting/creating-a-permanent-link-to-a-code-snippet
[parallel]: https://github.com/cucumber/godog#concurrency
[KUTTL]: https://kuttl.dev/
[Uptest]: https://github.com/upbound/uptest
[Kuttl POC]: https://github.com/lsviben/crossplane/tree/e2e-kuttl/test/e2e/kuttl/basic
[kubernetes-sig/e2e-framework]: https://github.com/kubernetes-sigs/e2e-framework
[e2e-framework POC]: https://github.com/lsviben/crossplane/tree/e2e-framework-evaluation/test/e2e/e2e-framework
[Terratest]: https://github.com/gruntwork-io/terratest
[Terratest POC]: https://github.com/lsviben/crossplane/tree/e2e-framework-evaluation/test/e2e/terratest