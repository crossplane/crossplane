# Provider Development Guide

Crossplane allows you to manage infrastructure directly from Kubernetes. Each
infrastructure API resource that Crossplane orchestrates is known as a "managed
resource". This guide will walk through the process of adding support for a new
kind of managed resource to a Crossplane Provider.

> You can watch [TBS Episode 18] to follow along the live implementation of GCP PubSub
managed resource.

> If there is a corresponding Terraform Provider, please consider generating
a Crossplane Provider with [Upjet] by following the
[Generating a Crossplane Provider guide].

> If you plan to implement a managed resource for AWS, please see the
[code generation guide].

## What Makes a Crossplane Infrastructure Resource

Crossplane builds atop Kubernetes's powerful architecture in which declarative
configuration, known as resources, are continually 'reconciled' with reality by
one or more controllers. A controller is an endless loop that:

1. Observes the desired state (the declarative configuration resource).
1. Observes the actual state (the thing said configuration resource represents).
1. Tries to make the actual state match the desired state.

Typical Crossplane managed infrastructure consists of two configuration
resources and one controller. The GCP Provider's support for Google Cloud
Memorystore illustrates this. First, the configuration resources:

1. A managed resource. Managed resources are cluster scoped, high-fidelity
   representations of a resource in an external system such as a cloud
   provider's API. Managed resources are _non-portable_ across external systems
   (i.e. cloud providers); they're tightly coupled to the implementation details
   of the external resource they represent. Managed resources are defined by a
   Provider. The GCP Provider's [`CloudMemorystoreInstance`] resource is an
   example of a managed resource.
1. A provider. Providers enable access to an external system, typically by
   indicating a Kubernetes Secret containing any credentials required to
   authenticate to the system, as well as any other metadata required to
   connect. Providers are cluster scoped, like managed resources and classes.
   The GCP [`ProviderConfig`] is an example of a provider. Note that provider is a
   somewhat overloaded term in the Crossplane ecosystem - it's also used to
   refer to the controller manager for a particular cloud, for example
   `provider-gcp`.

A managed resource is powered by a controller. This controller is responsible
for taking instances of the aforementioned high-fidelity managed resource kind
and reconciling them with an external system. The `CloudMemorystoreInstance`
controller watches for changes to `CloudMemorystoreInstance` resources and calls
Google's Cloud Memorystore API to create, update, or delete an instance as
necessary.
  
Crossplane does not require controllers to be written in any particular
language. The Kubernetes API server is our API boundary, so any process capable
of [watching the API server] and updating resources can be a Crossplane
controller.

## Getting Started

At the time of writing all Crossplane Services controllers are written in Go,
and built using [crossplane-runtime]. While it is possible to write a controller
using any language and tooling with a Kubernetes client this set of tools are
the "[golden path]". They're well supported, broadly used, and provide a shared
language with the Crossplane community. This guide targets [crossplane-runtime
v0.9.0]. It assumes the reader is familiar with the Kubernetes [API Conventions]
and the [kubebuilder book].

> If you are building a new provider from scratch, instead of adding new
resources to an already existing one, please use [provider-template] repository
as a template by hitting the `Use this template` button in GitHub UI. It
codifies most of the best practices used by the Crossplane community so far and
is the easiest way to start a new provider.

## Defining Resource Kinds

Let's assume we want to add Crossplane support for your favourite cloud's
database-as-a-service. Your favourite cloud brands these instances as "Favourite
DB instances". Under the hood they're powered by the open source FancySQL
engine. We'll name the new managed resource kind `FavouriteDBInstance`.

The first step toward implementing a new managed service is to define the code
level schema of its configuration resources. These are referred to as
[resources], (resource) [kinds], and [objects] interchangeably. The kubebuilder
scaffolding is a good starting point for any new Crossplane API kind.

> Note that while Crossplane was originally derived from kubebuilder scaffolds
> its patterns have diverged somewhat. It is _possible_ to use kubebuilder to
> scaffold a resource, but the author must be careful to adapt said resource to
> Crossplane patterns. It may often be quicker to copy and modify a v1beta1 or
> above resource from the same provider repository, rather than using
> kubebuilder.

```console
kubebuilder create api \
    --group example --version v1alpha1 --kind FavouriteDBInstance \
    --resource=true --controller=false --namespaced=false
```

The above command should produce a scaffold similar to the below example:

```go
type FavouriteDBInstanceSpec struct {
    // INSERT ADDITIONAL SPEC FIELDS - desired state of infrastructure
    // Important: Run "make" to regenerate code after modifying this file
}

// FavouriteDBInstanceStatus defines the observed state of FavouriteDBInstance
type FavouriteDBInstanceStatus struct {
    // INSERT ADDITIONAL STATUS FIELD - define observed state of infrastructure
    // Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true

// FavouriteDBInstance is the Schema for the favouritedbinstance API
// +kubebuilder:resource:scope=Cluster
type FavouriteDBInstance struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   FavouriteDBInstanceeSpec  `json:"spec,omitempty"`
    Status FavouriteDBInstanceStatus `json:"status,omitempty"`
}
```

Crossplane requires that these newly generated API type scaffolds be extended
with a set of struct fields, getters, and setters that are standard to all
Crossplane resource kinds. The getters and setter methods required to satisfy
crossplane-runtime interfaces are omitted from the below examples for brevity.
They can be added by hand, but new services are encouraged to use [`angryjet`]
to generate them automatically using a `//go:generate` comment per the
[`angryjet` documentation].

Note that in many cases a suitable provider will already exist. Frequently
adding support for a new managed service requires only the definition of the
managed resource itself.

### Managed Resource Kinds

Managed resources must:

* Satisfy crossplane-runtime's [`resource.Managed`] interface.
* Embed a [`ResourceStatus`] struct in their `Status` struct.
* Embed a [`ResourceSpec`] struct in their `Spec` struct.
* Embed a `Parameters` struct in their `Spec` struct.
* Use the `+kubebuilder:subresource:status` [comment marker].
* Use the `+kubebuilder:resource:scope=Cluster` [comment marker].

The `Parameters` struct should be a _high fidelity_ representation of the
writeable fields of the external resource's API. Put otherwise, if your
favourite cloud represents Favourite DB instances as a JSON object then
`FavouriteDBParameters` should marshal to a something as close to that JSON
object as possible while still complying with Kubernetes API conventions.

For example, assume the external API object for Favourite DB instance was:

```json
{
    "id": 42,
    "name": "mycoolinstance",
    "fanciness_level": 100,
    "version": "2.3",
    "status": "ONLINE",
    "hostname": "cool.fcp.example.org"
}
```

Further assume the `id`, `status`, and `hostname` fields were output only, and
the `version` field was optional. The `FavouriteDBInstance` managed resource
should look as follows:

```go
// FavouriteDBInstanceParameters define the desired state of an FavouriteDB
// instance. Most fields map directly to an Instance:
// https://favourite.example.org/api/v1/db#Instance
type FavouriteDBInstanceParameters struct {

    // We're still working on a standard for naming external resources. See
    // https://github.com/crossplane/crossplane/issues/624 for context.

    // Name of this instance.
    Name string `json:"name"`

    // Note that fanciness_level becomes fancinessLevel below. Kubernetes API
    // conventions trump cloud provider fidelity.

    // FancinessLevel specifies exactly how fancy this instance is.
    FancinessLevel int `json:"fancinessLevel"`

    // Version specifies what version of FancySQL this instance will run.
    // +optional
    Version *string `json:"version,omitempty"`
}

// A FavouriteDBInstanceSpec defines the desired state of a FavouriteDBInstance.
type FavouriteDBInstanceSpec struct {
    xpv1.ResourceSpec  `json:",inline"`
    ForProvider FavouriteDBInstanceParameters `json:"forProvider"`
}

// A FavouriteDBInstanceStatus represents the observed state of a
// FavouriteDBInstance.
type FavouriteDBInstanceStatus struct {
    xpv1.ResourceStatus `json:",inline"`

    // Note that we add the three "output only" fields here in the status,
    // instead of the parameters. We want this representation to be high
    // fidelity just like the parameters.

    // ID of this instance.
    ID int `json:"id,omitempty"`

    // Status of this instance.
    Status string `json:"status,omitempty"`

    // Hostname of this instance.
    Hostname string `json:"hostname,omitempty"`
}

// A FavouriteDBInstance is a managed resource that represents a Favourite DB
// instance.
// +kubebuilder:subresource:status
type FavouriteDBInstance struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   FavouriteDBInstanceSpec   `json:"spec"`
    Status FavouriteDBInstanceStatus `json:"status,omitempty"`
}
```

Note that Crossplane uses the GoDoc strings of API kinds to generate user facing
API documentation. __Document all fields__ and prefer GoDoc that assumes the
reader is running `kubectl explain`, or reading an API reference, not reading
the code. Refer to the [Managed Resource API Patterns] one pager for more detail
on authoring high fidelity managed resources.

### Provider Kinds

You'll typically only need to add a new Provider kind if you're creating an
infrastructure provider that adds support for a new infrastructure provider.

Providers must:

* Be named exactly `ProviderConfig`.
* Embed a [`ProviderSpec`] struct in their `Spec` struct.
* Use the `+kubebuilder:resource:scope=Cluster` [comment marker].

The Favourite Cloud `ProviderConfig` would look as follows. Note that the cloud to
which it belongs should be indicated by its API group, i.e. its API Version
would be `favouritecloud.crossplane.io/v1alpha1` or similar.

```go
// A ProviderSpec defines the desired state of a Provider.
type ProviderSpec struct {
    xpv1.ProviderSpec `json:",inline"`

    // Information required outside of the Secret referenced in the embedded
    // xpv1.ProviderSpec that is required to authenticate to the provider.
    // ProjectID is used as an example here.
    ProjectID string `json:"projectID"`
}

// A Provider configures a Favourite Cloud 'provider', i.e. a connection to a
// particular Favourite Cloud project using a particular Favourite Cloud service
// account.
type Provider struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec ProviderSpec `json:"spec"`
}
```

### Finishing Touches

At this point we've defined the managed resource necessary to start
building controllers. Before moving on to the controllers:

* Add any kubebuilder [comment markers] that may be useful for your resource.
  Comment markers can be used to validate input, or add additional columns to
  the standard `kubectl get` output, among other things.
* Run `make reviewable` to generate Custom Resource Definitions and additional
  helper methods for your new resource kinds.
* Make sure any package documentation (i.e. `// Package v1alpha1...` GoDoc,
  including package level comment markers) are in a file named `doc.go`.
  kubebuilder adds them to `groupversion_info.go`, but several code generation
  tools only check `doc.go`.

Finally, add convenience [`GroupVersionKind`] variables for each new resource
kind. These are typically added to either `register.go` or
`groupversion_info.go` depending on which version of kubebuilder scaffolded the
API type:

```go
// FavouriteDBInstance type metadata.
var (
    FavouriteDBInstanceKind             = reflect.TypeOf(FavouriteDBInstance{}).Name()
    FavouriteDBInstanceKindAPIVersion   = FavouriteDBInstanceKind + "." + GroupVersion.String()
    FavouriteDBInstanceGroupVersionKind = GroupVersion.WithKind(FavouriteDBInstanceKind)
)
```

Consider opening a draft pull request and asking a Crossplane maintainer for
review before you start work on the controller!

## Adding Controllers

Crossplane controllers, like those scaffolded by kubebuilder, are built around
the [controller-runtime] library. controller-runtime flavoured controllers
encapsulate most of their domain-specific logic in a [`reconcile.Reconciler`]
implementation. Most Crossplane controllers are one of the three kinds mentioned
under [What Makes a Crossplane Infrastructure Resource]. Each of these controller kinds
are similar enough across implementations that [crossplane-runtime] provides
'default' reconcilers. These reconcilers encode what the Crossplane community
has learned about managing external systems and narrow the problem space from
reconciling a Kubernetes resource kind with an arbitrary system down to
Crossplane-specific tasks.

crossplane-runtime provides the following `reconcile.Reconcilers`:

* The [`managed.Reconciler`] reconciles managed resources with external systems
  by instantiating a client of the external API and using it to create, update,
  or delete the external resource as necessary.

Crossplane controllers typically differ sufficiently from those scaffolded by
kubebuilder that there is little value in using kubebuilder to generate a
controller scaffold.

### Managed Resource Controllers

Managed resource controllers should use [`managed.NewReconciler`] to wrap a
managed-resource specific implementation of [`managed.ExternalConnecter`]. Parts
of `managed.Reconciler`'s behaviour is customisable; refer to the
[`managed.NewReconciler`] GoDoc for a list of options. The following is an
example controller for the `FavouriteDBInstance` managed resource we defined
earlier:

```go
import (
    "context"
    "fmt"
    "strings"

    "github.com/pkg/errors"
    corev1 "k8s.io/api/core/v1"
    "k8s.io/apimachinery/pkg/types"
    ctrl "sigs.k8s.io/controller-runtime"
    "sigs.k8s.io/controller-runtime/pkg/client"

    // An API client of the hypothetical FavouriteDB service.
    "github.com/fcp-sdk/v1/services/database"

    xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
    "github.com/crossplane/crossplane-runtime/pkg/meta"
    "github.com/crossplane/crossplane-runtime/pkg/resource"
    "github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"

    "github.com/crossplane/provider-fcp/apis/database/v1alpha3"
    fcpv1alpha3 "github.com/crossplane/provider-fcp/apis/v1alpha3"
)

type FavouriteDBInstanceController struct{}

// SetupWithManager instantiates a new controller using a managed.Reconciler
// configured to reconcile FavouriteDBInstances using an ExternalClient produced by
// connecter, which satisfies the ExternalConnecter interface.
func (c *FavouriteDBInstanceController) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        Named(strings.ToLower(fmt.Sprintf("%s.%s", v1alpha3.FavouriteDBInstanceKind, v1alpha3.Group))).
        For(&v1alpha3.FavouriteDBInstance{}).
        Complete(managed.NewReconciler(mgr,
            resource.ManagedKind(v1alpha3.FavouriteDBInstanceGroupVersionKind),
            managed.WithExternalConnecter(&connecter{client: mgr.GetClient()})))
}

// Connecter satisfies the resource.ExternalConnecter interface.
type connecter struct{ client client.Client }

// Connect to the supplied resource.Managed (presumed to be a
// FavouriteDBInstance) by using the Provider it references to create a new
// database client.
func (c *connecter) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
    // Assert that resource.Managed we were passed in fact contains a
    // FavouriteDBInstance. We told NewControllerManagedBy that this was a
    // controller For FavouriteDBInstance, so something would have to go
    // horribly wrong for us to encounter another type.
    i, ok := mg.(*v1alpha3.FavouriteDBInstance)
    if !ok {
        return nil, errors.New("managed resource is not a FavouriteDBInstance")
    }

    // Get the Provider referenced by the FavouriteDBInstance.
    p := &fcpv1alpha3.Provider{}
    if err := c.client.Get(ctx, meta.NamespacedNameOf(i.Spec.ProviderReference), p); err != nil {
        return nil, errors.Wrap(err, "cannot get Provider")
    }

    // Get the Secret referenced by the Provider.
    s := &corev1.Secret{}
    n := types.NamespacedName{Namespace: p.Namespace, Name: p.Spec.Secret.Name}
    if err := c.client.Get(ctx, n, s); err != nil {
        return nil, errors.Wrap(err, "cannot get Provider secret")
    }

    // Create and return a new database client using the credentials read from
    // our Provider's Secret.
    client, err := database.NewClient(ctx, s.Data[p.Spec.Secret.Key])
    return &external{client: client}, errors.Wrap(err, "cannot create client")
}

// External satisfies the resource.ExternalClient interface.
type external struct{ client database.Client }

// Observe the existing external resource, if any. The managed.Reconciler
// calls Observe in order to determine whether an external resource needs to be
// created, updated, or deleted.
func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
    i, ok := mg.(*v1alpha3.FavouriteDBInstance)
    if !ok {
        return managed.ExternalObservation{}, errors.New("managed resource is not a FavouriteDBInstance")
    }

    // Use our FavouriteDB API client to get an up to date view of the external
    // resource.
    existing, err := e.client.GetInstance(ctx, i.Spec.Name)

    // If we encounter an error indicating the external resource does not exist
    // we want to let the managed.Reconciler know so it can create it.
    if database.IsNotFound(err) {
        return managed.ExternalObservation{ResourceExists: false}, nil
    }

    // Any other errors are wrapped (as is good Go practice) and returned to the
    // managed.Reconciler. It will update the "Synced" status condition
    // of the managed resource to reflect that the most recent reconcile failed
    // and ensure the reconcile is reattempted after a brief wait.
    if err != nil {
        return managed.ExternalObservation{}, errors.Wrap(err, "cannot get instance")
    }

    // The external resource exists. Copy any output-only fields to their
    // corresponding entries in our status field.
    i.Status.Status = existing.GetStatus()
    i.Status.Hostname = existing.GetHostname()
    i.Status.ID = existing.GetID()

    // Update our "Ready" status condition to reflect the status of the external
    // resource. Most managed resources use the below well known reasons that
    // the "Ready" status may be true or false, but managed resource authors
    // are welcome to define and use their own.
    switch i.Status.Status {
    case database.StatusOnline:
        resource.SetBindable(i)
        i.SetConditions(xpv1.Available())
    case database.StatusCreating:
        i.SetConditions(xpv1.Creating())
    case database.StatusDeleting:
        i.SetConditions(xpv1.Deleting())
    }

    // Finally, we report what we know about the external resource. In this
    // hypothetical case FancinessLevel is the only field that can be updated
    // after creation time, so the resource does not need to be updated if
    // the actual fanciness level matches our desired fanciness level. Any
    // ConnectionDetails we return will be published to the managed resource's
    // connection secret if it specified one.
    o := managed.ExternalObservation{
        ResourceExists:   true,
        ResourceUpToDate: existing.GetFancinessLevel == i.Spec.FancinessLevel,
        ConnectionDetails: managed.ConnectionDetails{
            xpv1.ResourceCredentialsSecretUserKey:     []byte(existing.GetUsername()),
            xpv1.ResourceCredentialsSecretEndpointKey: []byte(existing.GetHostname()),
        },
    }

    return o, nil
}

// Create a new external resource based on the specification of our managed
// resource. managed.Reconciler only calls Create if Observe reported
// that the external resource did not exist.
func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
    i, ok := mg.(*v1alpha3.FavouriteDBInstance)
    if !ok {
        return managed.ExternalCreation{}, errors.New("managed resource is not a FavouriteDBInstance")
    }
    // Indicate that we're about to create the instance. Remember ExternalClient
    // authors can use a bespoke condition reason here in cases where Creating
    // doesn't make sense.
    i.SetConditions(xpv1.Creating())

    // Create must return any connection details that are set or returned only
    // at creation time. The managed.Reconciler will merge any details
    // with those returned during the Observe phase.
    password := database.GeneratePassword()
    cd := managed.ConnectionDetails{xpv1.ResourceCredentialsSecretPasswordKey: []byte(password)}

    // Create a new instance.
    new := database.Instance{Name: i.Name, FancinessLevel: i.FancinessLevel, Version: i.Version}
    err := e.client.CreateInstance(ctx, new, password)

    // Note that we use resource.Ignore to squash any error that indicates the
    // external resource already exists. Create implementations must not return
    // an error if asked to create a resource that already exists. Real managed
    // resource controllers are advised to avoid unintentially 'adoptign' an
    // existing, unrelated external resource, per
    // https://github.com/crossplane/crossplane-runtime/issues/27
    return managed.ExternalCreation{ConnectionDetails: cd}, errors.Wrap(resource.Ignore(database.IsExists, err), "cannot create instance")
}

// Update the existing external resource to match the specifications of our
// managed resource. managed.Reconciler only calls Update if Observe
// reported that the external resource was not up to date.
func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
    i, ok := mg.(*v1alpha3.FavouriteDBInstance)
    if !ok {
        return managed.ExternalUpdate{}, errors.New("managed resource is not a FavouriteDBInstance")
    }

    // Recall that FancinessLevel is the only field that we _can_ update.
    new := database.Instance{Name: i.Name, FancinessLevel: i.FancinessLevel}
    err := e.client.UpdateInstance(ctx, new)
    return managed.ExternalUpdate{}, errors.Wrap(err, "cannot update instance")
}

// Delete the external resource. managed.Reconciler only calls Delete
// when a managed resource with the 'Delete' deletion policy (the default) has
// been deleted.
func (e *external) Delete(ctx context.Context, mg resource.Managed) error {
    i, ok := mg.(*v1alpha3.FavouriteDBInstance)
    if !ok {
        return errors.New("managed resource is not a FavouriteDBInstance")
    }
    // Indicate that we're about to delete the instance.
    i.SetConditions(xpv1.Deleting())

    // Delete the instance.
    err := e.client.DeleteInstance(ctx, i.Spec.Name)

    // Note that we use resource.Ignore to squash any error that indicates the
    // external resource does not exist. Delete implementations must not return
    // an error when asked to delete a non-existent external resource.
    return errors.Wrap(resource.Ignore(database.IsNotFound, err), "cannot delete instance")
}
```

### Wrapping Up

Once all your controllers are in place you'll want to test them. Note that most
projects under the [crossplane org] [favor] table driven tests that use Go's
standard library `testing` package over kubebuilder's Gingko based tests. Please
do not add or proliferate Gingko based tests.

Finally, don't forget to plumb any newly added resource kinds and controllers up
to your controller manager. Simple providers may do this for each type within
within `main()`, but most more complicated providers take an approach in which
each package exposes an `AddToScheme` (for resource kinds) or `SetupWithManager`
(for controllers) function that invokes the same function within its child
packages, resulting in a `main.go` like:

```go
import (
    "time"

    "sigs.k8s.io/controller-runtime/pkg/client/config"
    "sigs.k8s.io/controller-runtime/pkg/manager"
    "sigs.k8s.io/controller-runtime/pkg/manager/signals"

    crossplaneapis "github.com/crossplane/crossplane/apis"

    fcpapis "github.com/crossplane/provider-fcp/apis"
    "github.com/crossplane/provider-fcp/pkg/controller"
)

func main() {
    cfg, err := config.GetConfig()
    if err != nil {
        panic(err)
    }

    mgr, err := manager.New(cfg, manager.Options{SyncPeriod: 1 * time.Hour})
    if err != nil {
        panic(err)
    }

    if err := crossplaneapis.AddToScheme(mgr.GetScheme()); err != nil {
        panic(err)
    }

    if err := fcpapis.AddToScheme(mgr.GetScheme()); err != nil {
        panic(err)
    }

    if err := controller.SetupWithManager(mgr); err != nil {
        panic(err)
    }

    panic(mgr.Start(signals.SetupSignalHandler()))
}
```

## In Review

In this guide we walked through the process of defining the resource kinds and
controllers necessary to build support for new managed infrastructure; possibly
even a completely new infrastructure provider. Please do not hesitate to [reach
out] to the Crossplane maintainers and community for help designing and
implementing support for new managed services. We would highly value any
feedback you may have about the development process!

<!-- Named Links -->
[crossplane-runtime v0.9.0]: https://github.com/crossplane/crossplane-runtime/releases/tag/v0.9.0
[TBS Episode 18]: https://www.youtube.com/watch?v=rvQ8N0u3rkE&t=7s
[What Makes a Crossplane Infrastructure Resource]: #what-makes-a-crossplane-infrastructure-resource
[`CloudMemorystoreInstance`]: https://github.com/crossplane/provider-gcp/blob/85a6ed3c669a021f1d61be51b2cbe2714b0bc70b/apis/cache/v1beta1/cloudmemorystore_instance_types.go#L184
[`ProviderConfig`]: https://github.com/crossplane/provider-gcp/blob/be5aaf6/apis/v1beta1/providerconfig_types.go#L39
[watching the API server]: https://kubernetes.io/docs/reference/using-api/api-concepts/#efficient-detection-of-changes
[controller-runtime]: https://github.com/kubernetes-sigs/controller-runtime
[crossplane-runtime]: https://github.com/crossplane/crossplane-runtime/
[golden path]: https://charity.wtf/2018/12/02/software-sprawl-the-golden-path-and-scaling-teams-with-agency/
[API Conventions]: https://github.com/kubernetes/community/blob/c6e1e89a/contributors/devel/sig-architecture/api-conventions.md
[kubebuilder book]: https://book.kubebuilder.io/
[resources]: https://kubebuilder.io/cronjob-tutorial/gvks.html#kinds-and-resources
[kinds]: https://kubebuilder.io/cronjob-tutorial/gvks.html#kinds-and-resources
[objects]: https://kubernetes.io/docs/concepts/#kubernetes-objects
[comment marker]: https://kubebuilder.io/reference/markers.html
[comment markers]: https://kubebuilder.io/reference/markers.html
[`resource.Managed`]: https://godoc.org/github.com/crossplane/crossplane-runtime/pkg/resource#Managed
[`managed.Reconciler`]: https://godoc.org/github.com/crossplane/crossplane-runtime/pkg/reconciler/managed#Reconciler
[`managed.NewReconciler`]: https://godoc.org/github.com/crossplane/crossplane-runtime/pkg/reconciler/managed#NewReconciler
[`managed.ExternalConnecter`]: https://godoc.org/github.com/crossplane/crossplane-runtime/pkg/reconciler/managed#ExternalConnecter
[`managed.ExternalClient`]: https://godoc.org/github.com/crossplane/crossplane-runtime/pkg/reconciler/managed#ExternalClient
[`ResourceSpec`]: https://godoc.org/github.com/crossplane/crossplane-runtime/apis/common/v1#ResourceSpec
[`ResourceStatus`]: https://godoc.org/github.com/crossplane/crossplane-runtime/apis/common/v1#ResourceStatus
[`ProviderSpec`]: https://godoc.org/github.com/crossplane/crossplane-runtime/apis/common/v1#ProviderSpec
['managed.ExternalConnecter`]: https://godoc.org/github.com/crossplane/crossplane-runtime/pkg/reconciler/managed#ExternalConnecter
[opening a Crossplane issue]: https://github.com/crossplane/crossplane/issues/new/choose
[`GroupVersionKind`]: https://godoc.org/k8s.io/apimachinery/pkg/runtime/schema#GroupVersionKind
[`reconcile.Reconciler`]: https://godoc.org/sigs.k8s.io/controller-runtime/pkg/reconcile#Reconciler
[favor]: https://github.com/crossplane/crossplane/issues/452
[reach out]: https://github.com/crossplane/crossplane#get-involved
[crossplane org]: https://github.com/crossplane
[`angryjet`]: https://github.com/crossplane/crossplane-tools
[Managed Resource API Patterns]: https://github.com/crossplane/crossplane/blob/master/design/one-pager-managed-resource-api-design.md
[Crossplane CLI]: https://github.com/crossplane/crossplane-cli#quick-start-stacks
[`angryjet` documentation]: https://github.com/crossplane/crossplane-tools/blob/master/README.md
[code generation guide]: https://github.com/crossplane-contrib/provider-aws/blob/master/CODE_GENERATION.md
[Upjet]: https://github.com/upbound/upjet
[Generating a Crossplane Provider guide]: https://github.com/upbound/upjet/blob/main/docs/generating-a-provider.md
[provider-template]: https://github.com/crossplane/provider-template
