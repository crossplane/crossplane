# Crossplane Integration Testing Framework
* Owner: Dan Mangum (@hasheddan)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Background

As Crossplane and its stacks have grown and evolved, the surface area for
potential bugs has increased as well. Every new controller in a stack has the
potential to behave in an undesirable manner given any number of edge-cases.
Currently, automated integration tests are performed at the stack level via
[bash scripts][current-integration-tests] that are triggered in a Jenkins
pipeline. While this is effective for determining if the stack can successfully
be installed into a Crossplane Kubernetes cluster, it does not go so far as to
test any of the controllers beyond that they start.

All other testing is performed in a manual ad-hoc manner for individual PR's or
leading up to a release. This leads to uncertainty around how last-minute
changes landing at the same time will affect the functionality of stacks.

## Goals

This design intends to create a testing framework with the following attributes:

- It is written in Go, such that it can be tested and distributed in a
  consistent and reliable manner.
- It is configurable, meaning that it does not require all controllers and
  resources to be tested for the same conditions.
- It is pluggable, meaning that it can easily be incorporated into a new project
  with minimal time and effort.
- It is familiar, meaning using the framework should feel like writing other
  tests in Go.
- It is simple, meaning efforts to develop and maintain the framework should be
  miniscule in comparison to the value it adds to the project.

The testing framework should *not* achieve the following:

- Replace the execution of unit tests.
- Serve as a build / deployment system.

The features above are already handled by robust platforms that are currently
serving their purpose effectively. Addressing them in the initial implementation
of this framework would require more effort than the return they would provide.

## Definition of a Successful Test

As mentioned above, the framework should unopinionated about the desired outcome
of a test. Every test should be able to define what it deems to be successful
and the framework should merely verify that that the successful conditions are
met or not.

### Definition of a Testable Unit

Any controller that that is able to successfully register with the Kubernetes
control plane should be testable. Any valid CustomResourceDefinitions that are
able to be registered with the [Scheme] of the Kubernetes API should be
testable.

## Proposal

The following sections describe a testing framework that utilizes the Go
[testing] package to execute tests against a local Kubernetes control plane. The
framework is designed such that it can be minimally implemented in the
short-term, deferring most of the execution and configuration to the tests that
are written for each implementation. It is intended to evolve over time to
reduce the amount of configuration required by each implementation, while still
allowing flexibility to test any scenario.

### System Design

The framework should be made up of two core components:

- `Job`: a testing environment. Every job runs the full setup and tear-down of a
  local Kubernetes control plane. A job is made up of configuration, which
  informs the framework of how to setup the control plane, and `Tests` which
  test actual execution in the control plane.
- `Test`: a test is an action or set of actions to execute against the
  Kubernetes control plane. It could involve one or more steps, each of which
  may cause the test to fail. Each test should be a logical operation, meaning
  that tests that involve multiple steps should still be checking a single
  logical condition (i.e. a `TestCreateSubnetSuccessful` may create the
  `Subnet`, then wait to make sure that its `ConditionedStatus` is `Ready:
  True`). 

#### Jobs

A `Job` contains the following components. `*Config` will likely evolve over
time, but will start with just defining the `SyncPeriod` for the controller
`manager`.

```go
// A Job is a set of tests that run in a custom configured local Kubernetes environment.
type Job struct {
    name             string
    description      string
    cfg              *Config
    runner           *testing.T
    setupWithManager setupWithManagerFunc
    addToScheme      addToSchemeFunc
    jobClean         jobCleanFunc
    tests            []Test
}
```

A job's primary method is `func (j *Job) Run() error` which executes the
following broadly defined steps:

1. Starting the API Server

The Kubernetes [controller-runtime] project provides an [envtest] package for
creating local control planes for testing. 

2. Registering CRDs

For any controller's that watch CRDs to be successfully started, the CRDs must
have been successfully created in the cluster. If they are unable to be created,
the controller's will not be started and the `Job` will immediately return an
error. This serves as a validation test for CRDs.

3. Registering Controllers

Controllers can be registered directly with the manager by calling their
`SetUpWithManager` functions directly. Because all claim controllers
(`scheduling`, `defaulting`, `claim`) are now part of the individual stacks, it
is not necessary for the Crossplane controller to be running to test the stacks.
However, it is necessary for the Crossplane CRDs to be present.

4. Starting the Manager

After all controllers and CRDs are registered, the manager can be started. 

5. Executing Tests

Jobs contain a set of tests that are provided at creation time. After the
manager is started, tests are executed by using the Go [testing] package, which
is injected into the `Job`. Each test is executed as a [subtest]. If a test's
`Executor` function fails, the `Job` will run its `Janitor` function (more on
this below). If the `Janitor` function also fails, the `Job` will immediately
begin clean up, which involves executing the `Clean` function, stopping the
manager, and stopping the local control plane. 

6. Clean Up

```go
type jobCleanFunc func(client.Client) error
```

`Clean` is a special function that must be present in all `Jobs` and always runs
before a job exits. It specifies how to clean up any resources that are
left-over from tests executed as part of the `Job`. While it is sufficient to
simply destroy the control plane to delete Kubernetes built-in API types, CRDs
that create external resources must be cleaned up directly to ensure the
deletion of their external infrastructure.

If no `Clean` function is supplied, the default `Clean` will be used, which
deletes all instances of CustomResourceDefinitions that were registered for the
`Job`. If `Clean` returns an `error` it will notify the test runner that remnant
external resources may still be in existence.

#### Tests

A `Test` defines four core properties:

1. Name

The `Name` of the test should be a succinct title that describes the test's
broad purpose.

2. Description

The `Description` of the test should describe the *purpose* for the test. As
more and more integration tests build up, it can become difficult to identify
why a certain test is being executed. The `Description` should be informative
enough that someone who is unfamiliar with the test can debug it effectively.
Keep in mind that the `Job` also has a description, so a test description should
focus specifically on why that test is being run as part of the job.

3. Executor Function

```go
type executorFn func(*client.Client) error
```

The `executor` function is the logic for running an individual test. The
function will be passed a Kubernetes client and it can execute any commands
against the control plane. It returns an error if a command is unsuccessful or
did not achieve the desired result.

4. Janitor Function

```go
type janitorFn func(*client.Client) (bool, error)
```

The `janitor` function is only executed on a test that fails. Its purpose is to
do any clean up that may be relevant to the steps taken in that specific test.
If a `janitor` function fails (i.e. returns an `error`), the test's parent `Job`
will not attempt to run further tests, and will immediately commence its clean
up process. If subsequent tests are reliant on the current test passing, the
`janitor` function should always return `false` regardless of if an `error` is
returned or not to instruct the `Job` to cease execution of further tests and
begin its clean up process.

### Full Example

*Note: the integration testing package is referred to as [`athodyd`] in the
following example.*

A full example of what a `Job` could look like is included below. This job runs
three tests:

1. `TestCreateNamespaceSuccessful`: attempts to create a new `Namespace`.
   Because `cool-namespace` does not already exist, this will be successful
   (`executor` will return `nil`), and the `Job` will move on to the next test.

2. `TestCreateAnotherNamespace`: attempts to create `cool-namespace` again. This
   will fail because `cool-namespace` already exists. The `Job` will run the
   `janitor` which is a no-op here, but returns `false`, indicating that further
   tests should not be run. The `Job` will then run its `Clean` function, which
   is not defined here, so the default will be used.

3. `TestCreateYetAnotherNamespace`: attempts to create `keen-namespace`, which
   would be successful. However, because the previous test failed and its
   `janitor` returned `false`, this test will never be run.

```go
func TestThis(t *testing.T) {
    tests := []athodyd.Test{
        {
            Name:        "TestCreateNamespaceSuccessful",
            Description: "This test checks to see if a namespace is created successfully.",
            Executor: func(c client.Client) error {
                n := &corev1.Namespace{
                    ObjectMeta: metav1.ObjectMeta{
                        Name: "cool-namespace",
                    },
                }
                if err := c.Create(context.TODO(), n); err != nil {
                    return err
                }

                return nil
            },
            Janitor: func(client.Client) (bool, error) {
                // The executor in this test will be successful, so Janitor will not be called
                return false, nil
            },
        },
        {
            Name:        "TestCreateAnotherNamespace",
            Description: "This test creates the same namespace as before.",
            Executor: func(c client.Client) error {
                n := &corev1.Namespace{
                    ObjectMeta: metav1.ObjectMeta{
                        Name: "cool-namespace",
                    },
                }
                if err := c.Create(context.TODO(), n); err != nil {
                    // This namespace already exists so we will fail to create
                    return err
                }

                return nil
            },
            Janitor: func(client.Client) (bool, error) {
                // The Janitor will run successfully (error is nil), but will stop the Job because it returns false. The Job will run the default Clean after this returns.
                return false, nil
            },
        },
        {
            // This test will never run because the previous test's Janitor returned false
            Name:        "TestCreateYetAnotherNamespace",
            Description: "This test creates a different namespace.",
            Executor: func(c client.Client) error {
                n := &corev1.Namespace{
                    ObjectMeta: metav1.ObjectMeta{
                        Name: "keen-namespace",
                    },
                }
                if err := c.Create(context.TODO(), n); err != nil {
                    return err
                }

                return nil
            },
            Janitor: func(client.Client) (bool, error) {
                return false, nil
            },
        },
    }

    c := &C{} // This is a dummy controller with a no-op reconciler
    job, err := athodyd.NewJob("myjobname", "myjobdescription" tests, "30s", t, c.SetupWithManager, apis.AddToScheme)
    if err != nil {
        t.Fatal(err)
    }

    if err := job.Run(); err != nil {
        t.Fatal(err) 
    }
}
```

## Future Considerations

This initial proposal is meant to provide a framework for implementing
integration tests into the Crossplane ecosystem *as soon as possible*. As our
testing suite grows, it will be desirable to move common functionality into the
framework itself, such that new job / test implementation can be less
burdensome. However, it should always be a goal to allow for broad
applicability, so new features should be added as optional layers rather than
core changes.

One example of common functionality that may be desirable to implement in the
framework itself would be a function that returns a `Test` that creates the
supplied managed resource and waits for it to reach a `ConditionedStatus` of
`Ready: True`. 

## Inspiration

This framework is loosely based off the work that has been done on [Kubernetes
e2e tests].

<!-- Named Links -->

[current-integration-tests]: https://github.com/crossplaneio/stack-gcp/blob/master/cluster/local/integration_tests.sh
[controller-runtime]: https://github.com/kubernetes-sigs/controller-runtime
[envtest]: https://godoc.org/sigs.k8s.io/controller-runtime/pkg/envtest
[testing]: https://golang.org/pkg/testing/
[Scheme]: https://godoc.org/k8s.io/apimachinery/pkg/runtime#Scheme
[subtest]: https://blog.golang.org/subtests
[`athodyd`]: https://en.wikipedia.org/wiki/Ramjet
[Kubernetes e2e tests]: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-testing/e2e-tests.md