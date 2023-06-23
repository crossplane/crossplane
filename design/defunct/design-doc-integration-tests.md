# Crossplane Integration Testing Framework
* Owner: Dan Mangum (@hasheddan)
* Reviewers: Crossplane Maintainers
* Status: Defunct

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
  resources to be tested in the same manner
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
of a test. The framework provides setup, tear down, and a method to connect to
testing environments but it not responsible for the actual execution of tests.

### Definition of a Testable Unit

Any controller that that is able to successfully register with the Kubernetes
control plane should be testable. Any valid CustomResourceDefinitions that are
able to be registered with the [Scheme] of the Kubernetes API should be
testable.

## Proposal

*Note: the integration testing package is referred to as [`athodyd`] in the
following section.*

The following sections describe a testing framework that utilizes the Go
[testing] package to execute tests against any Kubernetes control plane. The
framework is designed such that it can be minimally implemented in the
short-term, deferring most of the execution and configuration to the tests that
are written for each implementation. It is intended to evolve over time to
reduce the amount of configuration required by each implementation, while still
allowing flexibility to test any scenario.

Importantly, the framework is only responsible for environment setup *before*
custom controllers and API types are added and *after* they are removed.
Everything that happens between the start and stop of the controller manager
should be handled in the test implementations themselves.

### System Design

The framework encompasses three broad responsibilities:

1. Environment Setup

The framework sets up an environment by taking a Kubernetes REST configuration
and installing CustomResourceDefinitions, starting a controller manager, and
creating a client that can be used to communicate to the cluster. Because the
framework is a wrapper around the [envtest] package from [controller-runtime] it
can also start a local control plane if no REST configuration is applied.
Minimal setup that just installs CRDs into an existing cluster and returns a
controller manager would look as follows:

```go
cfg, err := config.GetConfig()
if err != nil {
    t.Fatal(err)
}

a, err := athodyd.New(cfg, athodyd.WithCRDDirectoryPaths([]string{"../crds"}))
if err != nil {
    t.Fatal(err)
}
```

The return value of `athodyd.New()` is of type `*athodyd.Manager`, which
implements the [controller-runtime] `manager.Manager` interface. This means that
controllers and API types can be added to the manager in the same manner they
are when the controller is being run in production:

```go
addToScheme(a.GetScheme()) // add API types to the manager scheme
controllerSetupWithManager(a) // register controllers with the manager
```

Once all setup is complete, the manager can be started with `a.Run()`.

2. Environment Connection

It is expected that most integration tests will want to interact with the API
server. To do so, a client can be retrieved and injected into the test function
at runtime.

```go
for name, tc := range cases {
    t.Run(name, func(t *testing.T) {
        err := tc.test(a.GetClient()) // retrieve the kubernetes client from athodyd
        if err != nil {
            t.Error(err)
        }
    })
}
```

3. Environment Cleanup

Because testing controllers usually involves creating some number of CRDs, it is
necessary to perform cleanup when external clusters are used. The framework will
default to deleting all CRDs that were installed in environment setup during its
cleanup if no alternative cleanup is supplied. During this step, the controller
manager is stopped, the `Cleaner` function is executed, and the connection to
the cluster will be terminated. If no configuration was supplied in the initial
setup, the local control plane will be destroyed in this step as well. If your
tests are dependent on successful cleanup, it may be desirable to fail if an
error is returned:

```go
defer func() {
    if err := a.Cleanup(); err != nil {
        t.Fatal(err)
    }
}()
```

To override the default `Cleaner` function, pass in your own to `athodyd.New()`:

```go
a, err := athodyd.New(cfg,
    athodyd.WithCRDDirectoryPaths([]string{"../crds"}),
    athodyd.WithCleaner(func(*envtest.Environment, client.Client){ return nil }))

if err != nil {
    t.Fatal(err)
}
```

The example above would not execute any action on cleanup and would always be
successful.

### Optional Configuration

As previously mentioned, the framework exposes a few point of customization.
These can be configured using `Option` functions:

```go
// WithBuilder sets a custom builder function for an Athodyd Config.
func WithBuilder(builder OperationFn) Option {
    return func(c *Config) {
        c.Builder = builder
    }
}

// WithCleaner sets a custom cleaner function for an Athodyd Config.
func WithCleaner(cleaner OperationFn) Option {
    return func(c *Config) {
        c.Cleaner = cleaner
    }
}

// WithCRDDirectoryPaths sets custom CRD locations for an Athodyd Config.
func WithCRDDirectoryPaths(crds []string) Option {
    return func(c *Config) {
        c.CRDDirectoryPaths = crds
    }
}

// WithManagerOptions sets custom options for the manager configured by Athodyd
// Config.
func WithManagerOptions(m manager.Options) Option {
    return func(c *Config) {
        c.ManagerOptions = m
    }
}
```

### Full Example

A simple example of an integration test using `athodyd` could look as follows:

```go
// TestThis tests this
func TestThis(t *testing.T) {
    cases := map[string]struct {
        reason string
        test   func(c client.Client) error
    }{
        "CreateProvider": {
            reason: "A GCP Provider should be created without error.",
            test: func(c client.Client) error {
                p := &v1alpha3.Provider{
                    ObjectMeta: metav1.ObjectMeta{
                        Name: "gcp-provider",
                    },
                    Spec: v1alpha3.ProviderSpec{
                        Secret: xpv1.SecretKeySelector{
                            Key: "credentials.json",
                            SecretReference: xpv1.SecretReference{
                                Name:      "example-provider-gcp",
                                Namespace: "crossplane-system",
                            },
                        },
                        ProjectID: "crossplane-playground",
                    },
                }

                defer func() {
                    if err := c.Delete(context.Background(), p); err != nil {
                        t.Error(err)
                    }
                }()

                return c.Create(context.Background(), p)
            },
        },
    }

    cfg, err := config.GetConfig()
    if err != nil {
        t.Fatal(err)
    }

    a, err := athodyd.New(cfg, athodyd.WithCRDDirectoryPaths([]string{"../crds"}))
    if err != nil {
        t.Fatal(err)
    }

    addToScheme(a.GetScheme())
    controllerSetupWithManager(a)

    a.Run()

    defer func() {
        if err := a.Cleanup(); err != nil {
            t.Fatal(err)
        }
    }()

    for name, tc := range cases {
        t.Run(name, func(t *testing.T) {
            err := tc.test(a.GetClient())
            if err != nil {
                t.Error(err)
            }
        })
    }
}
```

## Future Considerations

This initial proposal is meant to provide a framework for implementing
integration tests into the Crossplane ecosystem *as soon as possible*. As our
testing suite grows, it will be desirable to move common functionality into the
framework itself, such that new test implementation can be less burdensome.
However, it should always be a goal to allow for broad applicability, so new
features should be added as optional layers rather than core changes.

## Inspiration

This framework is loosely based off the work that has been done on [Kubernetes
e2e tests].

<!-- Named Links -->

[current-integration-tests]: https://github.com/crossplane/provider-gcp/blob/master/cluster/local/integration_tests.sh
[controller-runtime]: https://github.com/kubernetes-sigs/controller-runtime
[envtest]: https://godoc.org/sigs.k8s.io/controller-runtime/pkg/envtest
[testing]: https://golang.org/pkg/testing/
[Scheme]: https://godoc.org/k8s.io/apimachinery/pkg/runtime#Scheme
[`athodyd`]: https://en.wikipedia.org/wiki/Ramjet
[Kubernetes e2e tests]: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-testing/e2e-tests.md
[KIND]: https://github.com/kubernetes-sigs/kind
