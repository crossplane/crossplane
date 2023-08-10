# Package Runtime Config

* Owner: Nic Cope (@negz)
* Reviewers: Hasan Turken (@turkenh), Dan Mangum (@hasheddan)
* Status: Accepted

## Background

Crossplane Providers are Kubernetes controller managers. They're packaged in an
OCI container and expect to connect to an API server. You install a Provider
declaratively by creating a Provider resource. When you do, the Crossplane
package manager installs your provider by creating a Kubernetes Deployment,
along with some accoutrements like a ServiceAccount and a Service. The RBAC
manager also creates some RBAC ClusterRoles and ClusterRoleBindings to allow the
Provider's ServiceAccount (and thus Deployment) to do what it needs to do.

There are two things we'd like to improve about the way this works today:

1. Folks want more control over how their Providers are deployed - over the
   configuration of the Deployment (etc) Crossplane creates. ([#3601])
2. In some cases, folks don't want Crossplane to create a Kubernetes Deployment
   at all. They want to run a Provider some other way. ([#2671])

Soon [Composition Functions][functions-beta-design] will also be deployed by the
package manager as Kubernetes Deployments. We expect they'll have similar
requirements. Since Functions aren't Kubernetes controllers, in this document
I'll refer to the long-running processes some packages need to deploy as
'package runtimes'.

Crossplane has a v1alpha1 ControllerConfig type that addresses the first issue
for Providers. It has been marked deprecated, to be removed if and when we find
a suitable replacement. We deprecated ControllerConfig because:

* It was growing piecemeal to support templatizing an entire Deployment.
* We think in some rare cases package runtimes won't use Deployments.

It's worth noting that the desire to deploy a package runtime as anything other
than a Kubernetes Deployment in the same cluster where Crossplane is running is
_quite rare_. To my knowledge only Upbound currently has this requirement.

## Goals

The goals of this design are to:

* Give folks full control over package runtime Deployments.
* Make it possible to run package runtimes as something other than a Deployment.

## Proposal

I propose a new flag to Crossplane: `--package-runtime`. This flag would have
two possible values (at least to begin with):

* `--package-runtime=Deployment` (default) - Create a Kubernetes deployment.
* `--package-runtime=External` - Do nothing, defer to an external controller.

When running in Deployment mode the package manager will function as it does
today. It will create a Kubernetes Deployment for each package runtime, plus any
additional supporting configuration such as a ServiceAccount, etc.

When running in External mode the package manager won't create a package runtime
at all. It will create a revision (e.g. a ProviderRevision) and deliver the
package's payload (e.g. a Provider's CustomResourceDefinitions), but do nothing
else. In External mode it's expected that an external controller will take care
of reconciling the relevant package revision to deploy a package runtime however
it sees fit.

I also propose we replace ControllerConfig with a new DeploymentRuntimeConfig
type in the pkg.crossplane.io API group. This type is used to configure the
package runtime when the package manager is running with
`--package-runtime=Deployment`.

A DeploymentRuntimeConfig is referenced from any package that uses a runtime,
such as a Provider. For example:

```yaml
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-example
spec:
  package: xpkg.upbound.io/crossplane-contrib/provider-example:v1.0.0
  runtimeConfigRef:
    apiVersion: pkg.crossplane.io/v1
    kind: DeploymentRuntimeConfig
    name: default
```

There's a few things to note here:

* The referencing field is `spec.runtimeConfigRef`. A DeploymentRuntimeConfig is
  one possible type of runtime config - for now, the only supported one.
* Because there could in future be other kinds of runtime config the reference
  requires an `apiVersion` and `kind`.

If the `runtimeConfigRef` is omitted it will default to a runtime config named
"default" of the type specified by the `--package-runtime` flag. For example
when Crossplane is run with `--package-runtime=Deployment` a Provider will use a
DeploymentRuntimeConfig. This behavior matches that of ProviderConfigs. If you
omit the `providerConfigRef` when creating an MR Crossplane defaults to using a
ProviderConfig named default. 

Automatically setting a default runtime config has two advantages:

* The default configuration is no longer [hardcoded][hardcoded-pkg-deployment]
  into the package manager.
* Administrators can override the configuration all runtimes use by default.

The Crossplane init container will create a default DeploymentRuntimeConfig at
install time if it does not exist. A Crossplane administrator could then replace
it with their own. For example Crossplane might install a default
DeploymentRuntimeConfig that limits all package runtimes to 1 CPU core, but a
Crossplane administrator might wish to change this to give all runtimes 2 CPU
cores by default. Individual packages can still be explicitly configured to use
a specific runtime config.

Given that we saw ControllerConfig growing into a template for a Deployment, I
propose we lean into that and make DeploymentRuntimeConfig exactly that. For
example:

```yaml
apiVersion: pkg.crossplane.io/v1alpha1
kind: DeploymentRuntimeConfig
metadata:
  name: default
spec:
  deploymentTemplate:
    metadata:
      labels:
        example: label
    spec:
      replicas: 1
      template:
        securityContext:
          runAsNonRoot: true
          runAsUser: 2000
          runAsGroup: 2000
        containers:
          # The container used to run the Provider or Function must be named
          # 'package-runtime'. The package manager will overlay the package's
          # runtime image, pull policy, etc into this container.
        - name: package-runtime
          securityContext:
            runAsNonRoot: true
            runAsUser: 2000
            runAsGroup: 2000
            privileged: false
            allowPrivilegeEscalation: false
          ports:
          - name: metrics
            containerPort: 8080
    # A DeploymentRuntimeConfig can also be used to configure the Service and
    # ServiceAccount the package manager creates to support the Deployment.
    serviceTemplate:
      metadata: {}
    serviceAccountTemplate:
      metadata: {}
```

The above DeploymentRuntimeConfig matches the values that are currently
[hardcoded into the package manager][hardcoded-pkg-deployment]. The
`deploymentTemplate`, etc fields are similar to a Deployment's `template` field.

The package manager will always be opinionated about some things, and will
overlay the following settings over the top of the provided template:

* The image, image pull policy, and image pull secrets (set from the package).
* The label selectors required to make sure the Deployment and Service match.
* Any volumes, env vars, and ports required by a runtime (e.g. for webhooks).

[Today][#2880] it's possible to specify the name of the desired ServiceAccount
in a ControllerConfig. If the named ServiceAccount doesn't exist, it's created.
If it does exist, it's updated (e.g. by propagating annotations). In order to
maintain compatibility with this behaviour, it will be possible to explicitly
specify a `metadata.name` for a ServiceAccount. If an existing ServiceAccount is
named, it will be updated. If a name is not provided, the name of the package
revision will be used. This pattern will also apply to Deployments and Services,
simply to make the behaviour of a DeploymentRuntimeConfig more consistent and
thus less surprising.

## Migration from ControllerConfig

ControllerConfig is an alpha feature. Typically we do not provide an automated
migration when we drop support for alpha features. ControllerConfig is however a
special case - it predates our use of feature flags so it's on by default. It's
also known to be very widely used. Dropping it without a migration story would
be particularly disruptive.

To ease the migration, we will add a new feature flag, `enable-runtime-config`.
When true, Crossplane will support `runtimeConfigRef` and __not__
`controllerConfigRef`. The new `DeploymentRuntimeConfig` type will be introduced
as an alpha feature, and go through the typical alpha -> beta -> GA lifecycle.
This means:

1. When first released as alpha, `DeploymentRuntimeConfig` support will be off
  by default, with `ControllerConfig` support on by default.
2. When the feature is promoted to beta, `DeploymentRuntimeConfig` support will
  be on by default, with `ControllerConfig` support off by default. It will
  still be possible to specify `--enable-runtime-config=false` to force support
  for `ControllerConfig`.
3. When the feature is promoted to GA it will no longer be possible to disable
  support for `DeploymentRuntimeConfig`. Support for `ControllerConfig` will be
  removed.

To assist with migration, a tool will be provided that automatically creates a
`DeploymentRuntimeConfig` manifest given a `ControllerConfig` manifest, and
updates references from a `Provider` manifest accordingly.

## Future Improvements

The `--package-runtime` flag and `runtimeConfig` API are intentionally designed
such that other runtimes _could_ be added to Crossplane either natively, or as
PRI plugins in future (See [Alternatives Considered](#alternatives-considered)).
This allows us to prototype new runtimes implemented by controllers running
_alongside_ Crossplane before potentially moving them into tree later.

Assume for example there was a desire to use [Google Cloud Run][cloud-run] as a
package runtime: to deploy Providers to Google Cloud Run rather than to the
Kubernetes cluster where Crossplane is running. Under this design someone could
write a controller, deployed alongside Crossplane, that reconciled a
ProviderRevision by running its controller OCI container in Cloud Run. To do so
they would just need to set `--package-runtime=External` to let Crossplane know
ProviderRevisions were handled by an external system.

The 'external' cloud run package runtime controller could introduce its own
CloudRunRuntimeConfig custom resource that Providers could use to configure how
they should be deployed to Cloud Run:

```yaml
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-example
spec:
  package: xpkg.upbound.io/crossplane-contrib/provider-example:v1.0.0
  runtimeConfigRef:
    apiVersion: pkg.example.org/v1
    kind: CloudRunRuntimeConfig
    name: default
```

If there was sufficient demand, the functionality of the external cloud run
controller could be later built-in as `--package-runtime=GoogleCloudRun`.

## Alternatives Considered

The primary alternative to this proposal is the Provider Runtime Interface RFC
captured in [#2671]. This RFC intends to make it possible to use other package
runtimes besides a typical Kubernetes deployment, but implies Crossplane would
be responsible for deploying such runtimes via an abstraction layer.

I believe adding the `--package-runtime` flag achieves the spirit of this
proposal without introducing any additional complexity or indirection into
Crossplane. Given how niche the desire to use alternative runtimes is I feel
it's reasonable to expect anyone who wants one to implement it using their own
controller.


[#3601]: https://github.com/crossplane/crossplane/issues/3601
[#2671]: https://github.com/crossplane/crossplane/issues/2671
[functions-beta-design]: https://github.com/crossplane/crossplane/pull/4306
[hardcoded-pkg-deployment]: https://github.com/crossplane/crossplane/blob/v1.12.2/internal/controller/pkg/revision/deployment.go#L60
[google-cloud-run]: https://cloud.google.com/run
[#2880]: https://github.com/crossplane/crossplane/pull/2880