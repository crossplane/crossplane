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
[testing] package to execute tests against any Kubernetes control plane. The
framework is designed such that it can be minimally implemented in the
short-term, deferring most of the execution and configuration to the tests that
are written for each implementation. It is intended to evolve over time to
reduce the amount of configuration required by each implementation, while still
allowing flexibility to test any scenario.

### System Design

The framework should be made up of two core components:

- `Job`: a testing environment. A job is made up of configuration, which informs
  the framework of how to setup the control plane, and `Tests` which test actual
  execution against the control plane.
- `Test`: a test is an action or set of actions to execute against the
  Kubernetes control plane. It could involve one or more steps, each of which
  may cause the test to fail. Each test should be a logical operation, meaning
  that tests that involve multiple steps should still be checking a single
  logical condition (i.e. a `TestCreateSubnetSuccessful` may create the
  `Subnet`, then wait to make sure that its `ConditionedStatus` is `Ready:
  True`). 

#### Jobs

A `Job` contains the following components:

```go
// A Job is a set of tests that are executed sequentially in the same cluster environment
type Job struct {
    Name        string
    Description string
    Tests       []Test
    cfg         *JobConfig
    runner      *testing.T
}
```

A `JobConfig` is made up of optional values that are applied via `JobOption`
functions:

```go
// JobConfig is a set of configuration values for a Job
type JobConfig struct {
    CRDDirectoryPaths []string
    Cluster           *rest.Config
    Builder           OperationFn
    Cleaner           OperationFn
    SetupWithManager  SetupWithManagerFunc
    AddToScheme       AddToSchemeFunc
    SyncPeriod        *time.Duration
}
```

An example of a `JobOption` function:

```go
// WithCRDDirectoryPaths sets custom CRD locations for a Job
func WithCRDDirectoryPaths(crds []string) JobOption {
    return func(j *Job) {
        j.cfg.CRDDirectoryPaths = crds
    }
}
```

A job's primary method is `func (j *Job) Run() error` which executes the
following broadly defined steps:

1. Starting the API Server

The Kubernetes [controller-runtime] project provides an [envtest] package for
creating local control planes for testing. However, it is likely that we will
primarily utilize remote clusters or alternative local clusters such as [KIND]
to execute our tests. `envtest` exposes the ability to provide configuration to
use an existing cluster instead of starting a new one locally.

2. Registering CRDs

For any controller's that watch CRDs to be successfully started, the CRDs must
have been successfully created in the cluster. If they are unable to be created,
the controller's will not be started and the `Job` will immediately return an
error. This serves as a validation test for CRDs.

3. Builder

```go
// OperationFn is a function that uses a Kubernetes client to perform and operation
type OperationFn func(client.Client) error
```

`Builder` is responsible for doing any additional pre-test setup in the cluster.
For instance, if you would like to install other stacks into the cluster besides
the one that is run with your `SetupWithManager` function, this would be the
appropriate place to do so.

4. Registering Controllers

Controllers can be registered directly with the manager by calling their
`SetUpWithManager` functions directly. Because all claim controllers
(`scheduling`, `defaulting`, `claim`) are now part of the individual stacks, it
is not necessary for the Crossplane controller to be running to test the stacks.
However, it is necessary for the Crossplane CRDs to be present.

5. Starting the Manager

After all controllers and CRDs are registered, the manager can be started. 

6. Executing Tests

Jobs contain a set of tests that are provided at creation time. After the
manager is started, tests are executed by using the Go [testing] package, which
is injected into the `Job`. Each test is executed as a [subtest]. If a test's
`Executor` function fails, the `Job` will run its `Janitor` function (more on
this below). If the `Janitor` function also fails, the `Job` will immediately
begin clean up, which involves executing the `Cleaner` function, stopping the
manager, and stopping the local control plane if an existing Kubernetes cluster
was not specified for the tests. 

7. Clean Up

```go
// OperationFn is a function that uses a Kubernetes client to perform and operation
type OperationFn func(client.Client) error
```

`Cleaner` is a special function that must be present in all `Jobs` and always
runs before a job exits. It specifies how to clean up any resources that are
left-over from tests executed as part of the `Job`. While it is sufficient to
simply destroy the control plane to delete Kubernetes built-in API types, CRDs
that create external resources must be cleaned up directly to ensure the
deletion of their external infrastructure. If `Cleaner` returns an `error` it
will notify the test runner that remnant external resources may still be in
existence.

#### Tests

A `Test` is made up of five components:

```go
// A Test is a logical operation in a cluster environment
type Test struct {
    Name        string
    Description string
    Executor    OperationFn
    Janitor     OperationFn
    Continue    bool
}
```

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
// OperationFn is a function that uses a Kubernetes client to perform an operation.
type OperationFn func(client.Client) error
```

The `executor` function is the logic for running an individual test. The
function will be passed a Kubernetes client and it can execute any commands
against the control plane. It returns an error if a command is unsuccessful or
did not achieve the desired result.

4. Janitor Function

```go
// OperationFn is a function that uses a Kubernetes client to perform an operation.
type OperationFn func(client.Client) error
```

The `janitor` function is only executed on a test that fails. Its purpose is to
do any clean up that may be relevant to the steps taken in that specific test.
If a `janitor` function fails (i.e. returns an `error`), the test's parent `Job`
will not attempt to run further tests, and will immediately commence its clean
up process.

5. Continue

`Continue` determines whether subsequent tests should continue if the current
test fails. If no value is provided, it defaults to `False`.

### Full Example

*Note: the integration testing package is referred to as [`athodyd`] in the
following example.*

A full example of what a `Job` could look like is included below. This job runs
three tests:

1. `TestCreateNamespaceSuccessful`: attempts to create a new `Namespace`.
   Because `cool-namespace` does not already exist, this will be successful
   (`executor` will return `nil`), and the `Job` will move on to the next test.

2. `TestGCPProvider`: creates a GCP `Provider` in the cluster. This is possible
   because `athodyd` has installed the CRDs as the path we specified
   (`../crds`), and we have supplied an `AddToScheme` function to register our
   custom API types with the manager.

3. `TestCloudSQLProvisioning`: creates a GCP `CloudSQLInstance`. Because we
   registered our GCP controller with its `SetupWithManager` function, this
   object will be watched by its managed reconciler. In the `executor` function
   for this test, we wait to make sure that the `CloudSQLInstance` is
   provisioned successfully by polling its `State` until it reports `Runnable`
   or we timeout.

4. `TestCreateYetAnotherNamespace`: attempts to create `keen-namespace`, which
   would be successful. However, if the previous test failed, because it
   specified `Continue: false`, this test would be skipped.

```go
package example

import (
    "context"
    "fmt"
    "testing"
    "time"

    "k8s.io/apimachinery/pkg/types"
    "k8s.io/apimachinery/pkg/util/wait"

    runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
    databasev1beta1 "github.com/crossplaneio/stack-gcp/apis/database/v1beta1"
    "github.com/crossplaneio/stack-gcp/apis/v1alpha3"
    "github.com/hasheddan/athodyd"
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "sigs.k8s.io/controller-runtime/pkg/client"
    "sigs.k8s.io/controller-runtime/pkg/client/config"
)

// TestThis tests this
func TestThis(t *testing.T) {
    name := "MyExampleJob"
    description := "An example job for testing athodyd"
    dbVersion := "MYSQL_5_7"
    ddt := "PD_SSD"
    dds := int64(10)

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

                return c.Create(context.TODO(), n)
            },
            Janitor: func(client.Client) error {
                return nil
            },
            Continue: true,
        },
        {
            Name:        "TestGCPProvider",
            Description: "This test checks to see if a GCP Provider can be created successfully.",
            Executor: func(c client.Client) error {
                p := &v1alpha3.Provider{
                    ObjectMeta: metav1.ObjectMeta{
                        Name: "gcp-provider",
                    },
                    Spec: v1alpha3.ProviderSpec{
                        Secret: runtimev1alpha1.SecretKeySelector{
                            Key: "credentials.json",
                            SecretReference: runtimev1alpha1.SecretReference{
                                Name:      "example-provider-gcp",
                                Namespace: "crossplane-system",
                            },
                        },
                        ProjectID: "crossplane-playground",
                    },
                }

                return c.Create(context.TODO(), p)
            },
            Janitor: func(client.Client) error {
                return nil
            },
            Continue: true,
        },
        {
            Name:        "TestCloudSQLProvisioning",
            Description: "This test checks to see if a GCP CloudSQL instance can be created successfully.",
            Executor: func(c client.Client) error {
                s := &databasev1beta1.CloudSQLInstance{
                    ObjectMeta: metav1.ObjectMeta{
                        Name: "gcp-cloudsql",
                    },
                    Spec: databasev1beta1.CloudSQLInstanceSpec{
                        ResourceSpec: runtimev1alpha1.ResourceSpec{
                            ProviderReference: &corev1.ObjectReference{
                                Name: "gcp-provider",
                            },
                            ReclaimPolicy: runtimev1alpha1.ReclaimDelete,
                        },
                        ForProvider: databasev1beta1.CloudSQLInstanceParameters{
                            Region:          "us-central1",
                            DatabaseVersion: &dbVersion,
                            Settings: databasev1beta1.Settings{
                                Tier:           "db-n1-standard-1",
                                DataDiskType:   &ddt,
                                DataDiskSizeGb: &dds,
                            },
                        },
                    },
                }

                if err := c.Create(context.TODO(), s); err != nil {
                    return err
                }

                d, err := time.ParseDuration("20s")
                if err != nil {
                    return err
                }

                dt, err := time.ParseDuration("500s")
                if err != nil {
                    return err
                }

                return wait.PollImmediate(d, dt, func() (bool, error) {
                    g := &databasev1beta1.CloudSQLInstance{}
                    if err := c.Get(context.TODO(), types.NamespacedName{Name: "gcp-cloudsql"}, g); err != nil {
                        return false, err
                    }
                    if g.Status.AtProvider.State == databasev1beta1.StateRunnable {
                        return true, nil
                    }
                    return false, nil
                })
            },
            Janitor: func(c client.Client) error {
                return c.Delete(context.TODO(), &databasev1beta1.CloudSQLInstance{
                    ObjectMeta: metav1.ObjectMeta{
                        Name: "gcp-cloudsql-7847238946237",
                    },
                })
            },
            Continue: false,
        },
        {
            Name:        "TestCreateAnotherNamespace",
            Description: "This test creates a different namespace.",
            Executor: func(c client.Client) error {
                n := &corev1.Namespace{
                    ObjectMeta: metav1.ObjectMeta{
                        Name: "keen-namespace",
                    },
                }

                return c.Create(context.TODO(), n)
            },
            Janitor: func(client.Client) error {
                return nil
            },
            Continue: true,
        },
    }

    // Get existing cluster configured in local kubeconfig
    cfg, err := config.GetConfig()
    if err != nil {
        t.Fatal(err)
    }
    
    cleaner := func(c client.Client) error {
        if err := c.DeleteAllOf(context.TODO(), &databasev1beta1.CloudSQLInstance{}); err != nil {
            return err
        }
        
        if err := c.DeleteAllOf(context.TODO(), &v1alpha3.Provider{}); err != nil {
            return err
        }

        return nil
    }

    job := athodyd.NewJob(name, description, tests, t,
        athodyd.WithCluster(cfg),
        athodyd.WithCRDDirectoryPaths([]string{"../crds"}),
        athodyd.WithSetupWithManager(controllerSetupWithManager),
        athodyd.WithAddToScheme(addToScheme),
        athodyd.WithCleaner(cleaner),
    )

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
[KIND]: https://github.com/kubernetes-sigs/kind
