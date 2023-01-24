# Adding Secret Store Support

To add support for [External Secret Stores] in a provider, we need the following
changes at a high level:

1. Bump Crossplane Runtime and Crossplane Tools to latest and generate existing
resources to include `PublishConnectionDetails` API.
2. Add a new Type and CRD for Secret StoreConfig.
3. Add feature flag for enabling External Secret Store support.
4. Add Secret Store Connection Details Manager as a `ConnectionPublisher` if
feature enabled.

In this document, we will go through each step in details. You can check 
[this PR as a complete example].

> If your provider is a Terrajet based provider, then please check
> [this PR instead].

## Steps

**1. Bump Crossplane Runtime and Crossplane Tools to latest and generate
existing resources to include `PublishConnectionDetails` API.**

We need a workaround for code generation since latest runtime both adds new API
but also adds a new interface to managed.resourceSpec. Without this workaround,
expect errors similar to below:

  ```shell
  16:40:56 [ .. ] go generate darwin_amd64
  angryjet: error: error loading packages using pattern ./...: /Users/hasanturken/  Workspace/crossplane/provider-gcp/apis/cache/v1beta1/zz_  generated.managedlist.go:27:14: cannot use &l.Items[i] (value of type *  CloudMemorystoreInstance) as "github.com/crossplane/crossplane-runtime/pkg/  resource".Managed value in assignment: missing method GetPublishConnectionDetailsTo
  exit status 1
  apis/generate.go:30: running "go": exit status 1
  16:41:04 [FAIL]
  make[1]: *** [go.generate] Error 1
  make: *** [generate] Error 2
  ```

First, we need to consume a temporary runtime version together with the latest
Crossplane Tools:

  ```shell
  go mod edit -replace=github.com/crossplane/crossplane-runtime=github.com/turkenh/crossplane-runtime@v0.0.0-20220314141040-6f74175d3c1f
  go get github.com/crossplane/crossplane-tools@master

  go mod tidy
  ```

Then, remove `trivialVersions=true` in the file `api/generate.go`:

  ```diff
-//go:generate go run -tags generate sigs.k8s.io/controller-tools/cmd/controller-gen object:headerFile=../hack/boilerplate.go.txt paths=./... crd:trivialVersions=true,crdVersions=v1 output:artifacts:config=../package/crds
+//go:generate go run -tags generate sigs.k8s.io/controller-tools/cmd/controller-gen object:headerFile=../hack/boilerplate.go.txt paths=./... crd:crdVersions=v1 output:artifacts:config=../package/crds
  ```

Now, we can generate CRDs with `PublishConnectionDetailsTo` API:

  ```shell
  make generate
  ```

Finally, we can revert our workaround by consuming the latest Crossplane
Runtime:

  ```shell
  go mod edit -dropreplace=github.com/crossplane/crossplane-runtime
  go get github.com/crossplane/crossplane-runtime@master
  go mod tidy
  make generate
  ```

**2. Add a new Type and CRD for Secret StoreConfig.**

See [this commit as an example on how to add the type]. It is expected to be
almost same for all providers except groupName which includes the name short
name of the provider (e.g. `gcp.crossplane.io`)

Generate the CRD with:

  ```shell
  make generate
  ```

**3. Add feature flag for enabling External Secret Store support.**

We will add a feature flag to enable the feature which would be off by default.
As part of this step, we will also create a `default` `StoreConfig` during
provider start up, which stores connection secrets into the same Kubernetes
cluster.

To be consistent across all providers, please define
`--enable-external-secret-stores` as a boolean which is false by default.

See [this commit as an example for adding the feature flag].

**4. Add Secret Store Connection Details Manager as a `ConnectionPublisher` if
feature enabled.**

Add the following to the Setup function controller. Unfortunately this step
requires some dirty work as we need to this for all types:

  ```diff
   func SetupServiceAccountKey(mgr ctrl.Manager, o controller.Options) error {
        name := managed.ControllerName(v1alpha1.ServiceAccountKeyGroupKind)

+       cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}
+       if o.Features.Enabled(features.EnableAlphaExternalSecretStores) {
+               cps = append(cps, connection.NewDetailsManager(mgr.GetClient(), scv1alpha1.StoreConfigGroupVersionKind))
+       }
+
        r := managed.NewReconciler(mgr,
                resource.ManagedKind(v1alpha1.ServiceAccountKeyGroupVersionKind),
                managed.WithInitializers(),
                managed.WithExternalConnecter(&serviceAccountKeyServiceConnector{client: mgr.GetClient()}),
                managed.WithPollInterval(o.PollInterval),
                managed.WithLogger(o.Logger.WithValues("controller", name)),
-               managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))))
+               managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
+               managed.WithConnectionPublishers(cps...))

        return ctrl.NewControllerManagedBy(mgr).
                Named(name).
  ```

You can check [this commit as an example for changes in Setup functions] as an
example.

[External Secret Stores]: https://github.com/crossplane/crossplane/blob/master/design/design-doc-external-secret-stores.md
[this PR as a complete example]: https://github.com/crossplane/provider-gcp/pull/421
[this PR instead]: https://github.com/crossplane-contrib/provider-jet-template/pull/23/commits
[this commit as an example on how to add the type]: https://github.com/crossplane-contrib/provider-aws/pull/1242/commits/d8a2df323fa2489d82bf1843d2fe338de033c61d
[this commit as an example for adding the feature flag]: https://github.com/crossplane/provider-gcp/pull/421/commits/b5898c62dc6668d9918496de8aa9bc365c371f82
[this commit as an example for changes in Setup functions]: https://github.com/crossplane/provider-gcp/pull/421/commits/9700d0c4fdb7e1fba8805afa309c1b1c7aa167a6