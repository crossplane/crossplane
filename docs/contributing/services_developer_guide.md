---
title: Services Developer Guide
toc: true
weight: 101
indent: true
---

# Services Developer Guide

Crossplane Services supports managed service provisioning using `kubectl`. It
applies the Kubernetes pattern for Persistent Volume (PV) claims and classes to
managed service provisioning with support for a strong separation of concern
between app teams and cluster administrators. This guide will walk through the
process of adding support for a new managed service.

## What Makes a Crossplane Managed Service?

Crossplane builds atop Kubernetes's powerful architecture in which declarative
configuration, known as resources, are continually 'reconciled' with reality by
one or more controllers. A controller is an endless loop that:

1. Observes the desired state (the declarative configuration resource).
1. Observes the actual state (the thing said configuration resource represents).
1. Tries to make the actual state match the desired state.

A typical Crossplane managed service consists of five configuration resources
and five controllers. The GCP Provider's support for Google Cloud Memorystore
illustrates this. First, the configuration resources:

1. A [managed resource]. Managed resources are cluster scoped, high-fidelity
   representations of a resource in an external system such as a cloud
   provider's API. Managed resources are _non-portable_ across external systems
   (i.e. cloud providers); they're tightly coupled to the implementation details
   of the external resource they represent. Managed resources are defined by a
   Provider. The GCP Provider's [`CloudMemorystoreInstance`] resource is an
   example of a managed resource.
1. A [resource claim]. Resource claims are namespaced abstract declarations of a
   need for a service. Resource claims are frequently portable across external
   systems. Crossplane defines a series of common resource claim kinds,
   including [`RedisCluster`]. A resource claim is satisfied by _binding_ to a
   managed resource.
1. A [resource class]. Resource classes represent a class of a specific kind of
   managed resource. They are the template used to create a new managed resource
   in order to satisfy a resource claim during [dynamic provisioning]. Resource
   classes are cluster scoped, and tightly coupled to the managed resources they
   template. [`CloudMemorystoreInstanceClass`] is an example of a resource
   class.
1. A provider. Providers enable access to an external system, typically by
   indicating a Kubernetes Secret containing any credentials required to
   authenticate to the system, as well as any other metadata required to
   connect. Providers are cluster scoped, like managed resources and classes.
   The GCP [`Provider`] is an example of a provider.

These resources are powered by:

1. The managed resource controller. This controller is responsible for taking
   instances of the aforementioned high-fidelity managed resource kind and
   reconciling them with an external system. Managed resource controllers are
   unaware of resource claims or classes. The `CloudMemorystoreInstance`
   controller watches for changes to `CloudMemorystoreInstance` resources and
   calls Google's Cloud Memorystore API to create, update, or delete an instance
   as necessary.
1. The resource claim scheduling controller. A claim scheduling controller
   exists for each kind of resource class that could satisfy a resource claim.
   This controller is unaware of any external system - it simply schedules
   resource claims to resource classes that match their class selector labels,
   so that they may be handled by the resource claim controller.
1. The resource claim defaulting controller. A claim defaulting controller
   exists for each kind of resource class that could satisfy a resource claim.
   This controller is unaware of any external system - it allocates resource
   claims that do not specify a class selector to a resource class annotated as
   the default, if any, so that they may be handled by the claim controller.
1. The resource claim controller. A resource claim controller exists for each
   kind of managed resource that could satisfy a resource claim. This controller
   is unaware of any external system - it responsible only for taking resource
   claims and binding them to a managed resource.  The
   `CloudMemorystoreInstance` resource claim controller watches for
   `RedisCluster` resource claims that should be satisfied by a
   `CloudMemorystoreInstance`. It either binds to an explicitly referenced
   `CloudMemorystoreInstance` (static provisioning) or creates a new one and
   then binds to it (dynamic provisioning).
1. The secret propagation controller. Like the resource claim controller, a
   secret propagation controller exists for each kind of managed resource that
   could satisfy a resource claim. Its job is simply to ensure that changes to
   the connection secret of a managed resource are always propagated to the
   connection secret of the resource claim it is bound to. The secret
   propagation controller is optional - managed resources that only write to
   their connection secret at creation time may omit this controller.

Crossplane does not require controllers to be written in any particular
language. The Kubernetes API server is our API boundary, so any process capable
of [watching the API server] and updating resources can be a Crossplane
controller.

## Getting Started

At the time of writing all Crossplane Services controllers are written in Go,
and built using [kubebuilder] v0.2.x and [crossplane-runtime]. Per [What Makes a
Crossplane Managed Service] it is possible to write a controller using any
language and tooling with a Kubernetes client, but this set of tools are the
"[golden path]". They're well supported, broadly used, and provide a shared
language with the Crossplane maintainers. This guide targets [crossplane-runtime
v0.4.0].

This guide assumes the reader is familiar with the Kubernetes [API Conventions]
and the [kubebuilder book]. If you're not adding a new managed service to an
existing Crossplane Provider you should start by working through the [Stacks
quick start] to scaffold a new Provider in which the new types and controllers
will live.

## Defining Resource Kinds

Let's assume we want to add Crossplane support for your favourite cloud's
database-as-a-service. Your favourite cloud brands these instances as "Favourite
DB instances". Under the hood they're powered by the open source FancySQL
engine. We'll name the new managed resource kind `FavouriteDBInstance` and the
new resource claim `FancySQLInstance`.

The first step toward implementing a new managed service is to define the code
level schema of its configuration resources. These are referred to as
[resources], (resource) [kinds], and [objects] interchangeably. The kubebuilder
scaffolding is a good starting point for any new Crossplane API kind, whether
they'll be a managed resource, resource class, or resource claim.

```console
# The resource claim.
kubebuilder create api \
    --group example --version v1alpha1 --kind FancySQLInstance \
    --resource=true --controller=false
```

The above command should produce a scaffold similar to the below example:

```go
type FancySQLInstanceSpec struct {
    // INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
    // Important: Run "make" to regenerate code after modifying this file
}

// FancySQLInstanceStatus defines the observed state of FancySQLInstance
type FancySQLInstanceStatus struct {
    // INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
    // Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true

// FancySQLInstance is the Schema for the fancysqlinstances API
type FancySQLInstance struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   FancySQLInstanceSpec   `json:"spec,omitempty"`
    Status FancySQLInstanceStatus `json:"status,omitempty"`
}
```

Crossplane requires that these newly generated API type scaffolds be extended
with a set of struct fields, getters, and setters that are standard to all
Crossplane resource kinds. The fields and setters differ depending on whether
the new resource kind is a managed resource, resource claim, or resource class.
The getters and setter methods required to satisfy the various
crossplane-runtime interfaces are omitted from the below examples for brevity.
They can be added by hand, but new services are encouraged to use [`angryjet`]
to generate them automatically using a `//go:generate` comment per the
[`angryjet` documentation].

Note that in many cases a suitable provider and resource claim will already
exist. Frequently adding support for a new managed service requires only the
definition of a new managed resource and resource class.

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
    runtimev1alpha1.ResourceSpec  `json:",inline"`
    ForProvider FavouriteDBInstanceParameters `json:"forProvider"`
}

// A FavouriteDBInstanceStatus represents the observed state of a
// FavouriteDBInstance.
type FavouriteDBInstanceStatus struct {
    runtimev1alpha1.ResourceStatus `json:",inline"`

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

### Resource Class Kinds

The resource class kind for a particular managed resource kind are typically
defined in the same file as their the managed resource. Resource classes must:

* Satisfy crossplane-runtime's [`resource.Class`] interface.
* Have a `SpecTemplate` struct field instead of a `Spec`.
* Embed a [`ClassSpecTemplate`] struct in their `SpecTemplate` struct.
* Embed their managed resource's `Parameters` struct as `ForProvider` in their
  `SpecTemplate` struct.
* Not have a `Status` struct.
* Use the `+kubebuilder:resource:scope=Cluster` [comment marker].

A resource class for the above `FavouriteDBInstance` would look as follows:

```go
// A FavouriteDBInstanceClassSpecTemplate is a template for the spec of a
// dynamically provisioned FavouriteDBInstance.
type FavouriteDBInstanceClassSpecTemplate struct {
    runtimev1alpha1.ClassSpecTemplate `json:",inline"`
    ForProvider FavouriteDBInstanceParameters     `json:"forProvider"`
}

// A FavouriteDBInstanceClass is a resource class. It defines the desired spec
// of resource claims that use it to dynamically provision a managed resource.
type FavouriteDBInstanceClass struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    // SpecTemplate is a template for the spec of a dynamically provisioned
    // FavouriteDBInstance.
    SpecTemplate FavouriteDBInstanceSpecTemplate `json:"specTemplate,omitempty"`
}
```

### Resource Claim Kinds

Once the underlying managed resource and its resource class have been defined
the next step is to define the resource claim. Resource claim controllers
typically live alongside their managed resource controllers (i.e. in an
infrastructure provider), but at the time of writing all resource claim kinds
are defined in Crossplane core. This is because resource claims can frequently
be satisfied by binding to managed resources from more than one cloud. Consider
[opening a Crossplane issue] to propose adding your new resource claim kind to
Crossplane if it could be satisfied by managed resources from more than one
infrastructure provider.

Resource claims must:

* Satisfy crossplane-runtime's [`resource.Claim`] interface.
* Use (not embed) a [`ResourceClaimStatus`] struct as their `Status` field.
* Embed a [`ResourceClaimSpec`] struct in their `Spec` struct.
* Use the `+kubebuilder:subresource:status` [comment marker].
* **Not** use the `+kubebuilder:resource:scope=Cluster` [comment marker].

The `FancySQLInstance` resource claim would look as follows:

```go
// A FancySQLInstanceSpec defines the desired state of a FancySQLInstance.
type FancySQLInstanceSpec struct {
    runtimev1alpha1.ResourceClaimSpec `json:",inline"`

    // Resource claims typically expose few to no spec fields, instead
    // leveraging resource classes to specify detailed configuration. A resource
    // claim should only support a very small set of fields that:
    //
    // * Are applicable to every conceivable managed resource kind that might
    //   ever satisfy the claim kind.
    // * Are more likely than average to be interesting to resource claim
    //   authors, who frequently want to be concerned with as few configuration
    //   details as possible.

    // Version specifies what version of FancySQL this instance will run.
    // +optional
    Version *string `json:"version,omitempty"`
}

// A FancySQLInstance is a portable resource claim that may be satisfied by
// binding to FancySQL managed resources such as a Favourite Cloud FavouriteDB
// instance or an Other Cloud AmbivalentDB instance.
// +kubebuilder:subresource:status
type FancySQLInstance struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   FancySQLInstanceSpec                `json:"spec,omitempty"`
    Status runtimev1alpha1.ResourceClaimStatus `json:"status,omitempty"`
}

// SetBindingPhase of this FancySQLInstance.
func (i *FancySQLInstance) SetBindingPhase(p runtimev1alpha1.BindingPhase) {
    i.Status.SetBindingPhase(p)
}
```

### Provider Kinds

You'll typically only need to add a new Provider kind if you're creating an
infrastructure provider that adds support for a new infrastructure provider.

Providers must:

* Be named exactly `Provider`.
* Embed a [`ProviderSpec`] struct in their `Spec` struct.
* Use the `+kubebuilder:resource:scope=Cluster` [comment marker].

The Favourite Cloud `Provider` would look as follows. Note that the cloud to
which it belongs should be indicated by its API group, i.e. its API Version
would be `favouritecloud.crossplane.io/v1alpha1` or similar.

```go
// A ProviderSpec defines the desired state of a Provider.
type ProviderSpec struct {
    runtimev1alpha1.ProviderSpec `json:",inline"`

    // Information required outside of the Secret referenced in the embedded
    // runtimev1alpha1.ProviderSpec that is required to authenticate to the provider.
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

At this point we've defined all of the resource kinds necessary to start
building controllers - a managed resource, a resource class, and a resource
claim. Before moving on to the controllers:

* Add any kubebuilder [comment markers] that may be useful for your resource.
  Comment markers can be used to validate input, or add additional columns to
  the standard `kubectl get` output, among other things.
* Run `make generate && make manifests` (or `make reviewable` if you're working
  in one of the projects in the [crossplane org]) to generate Custom Resource
  Definitions and additional helper methods for your new resource kinds.
* Make sure a `//go:generate` comment exists for [angryjet] and you ran `go
  generate -v ./...`
* Make sure any package documentation (i.e. `// Package v1alpha1...` GoDoc,
  including package level comment markers) are in a file named `doc.go`.
  kubebuilder adds them to `groupversion_info.go`, but several code generation
  tools only check `doc.go`.

Finally, add convenience [`GroupVersionKind`] variables for each new resource
kind. These are typically added to either `register.go` or
`groupversion_info.go` depending on which version of kubebuilder scaffolded the
API type:

```go
// FancySQLInstance type metadata.
var (
    FancySQLInstanceKind             = reflect.TypeOf(FancySQLInstance{}).Name()
    FancySQLInstanceKindAPIVersion   = FancySQLInstanceKind + "." + GroupVersion.String()
    FancySQLInstanceGroupVersionKind = GroupVersion.WithKind(FancySQLInstanceKind)
)
```

Consider opening a draft pull request and asking a Crossplane maintainer for
review before you start work on the controller!

## Adding Controllers

Crossplane controllers, like those scaffolded by kubebuilder, are built around
the [controller-runtime] library. controller-runtime flavoured controllers
encapsulate most of their domain-specific logic in a [`reconcile.Reconciler`]
implementation. Most Crossplane controllers are one of the three kinds mentioned
under [What Makes a Crossplane Managed Service]. Each of these controller kinds
are similar enough across implementations that [crossplane-runtime] provides
'default' reconcilers. These reconcilers encode what the Crossplane community
has learned about managing external systems and narrow the problem space from
reconciling a Kubernetes resource kind with an arbitrary system down to
Crossplane-specific tasks.

crossplane-runtime provides the following `reconcile.Reconcilers`:

* The [`managed.Reconciler`] reconciles managed resources with external systems
  by instantiating a client of the external API and using it to create, update,
  or delete the external resource as necessary.
* [`claimscheduling.Reconciler`] reconciles resource claims by scheduling them
  to a resource class that matches their class selector labels (if any).
* [`claimdefaulting.Reconciler`] reconciles resource claims that omit their
  class selector by defaulting them to a resource class annotated as the default
  (if any).
* [`claimbinding.Reconciler`] reconciles resource claims with managed resources
  by either binding or dynamically provisioning and then binding them.
* [`secret.NewReconciler`] reconciles secrets by propagating their data to
  another secret. This controller is typically used to ensure resource claim
  connection secrets remain in sync with the connection secrets of their bound
  managed resources.
* [`target.Reconciler`] reconciles `KubernetesTarget` resources that reference
  managed resources that provide a hosted Kubernetes service (i.e. GKE, EKS,
  AKS). This controller is used to propagate the connection information of the
  referenced Kubernetes cluster to the namespace of the `KubernetesTarget` in
  the form of a secret.

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

    runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
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
        // If the resource is available we also want to mark it as bindable to
        // resource claims.
        resource.SetBindable(i)
        i.SetConditions(runtimev1alpha1.Available())
    case database.StatusCreating:
        i.SetConditions(runtimev1alpha1.Creating())
    case database.StatusDeleting:
        i.SetConditions(runtimev1alpha1.Deleting())
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
            runtimev1alpha1.ResourceCredentialsSecretUserKey:     []byte(existing.GetUsername()),
            runtimev1alpha1.ResourceCredentialsSecretEndpointKey: []byte(existing.GetHostname()),
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
    i.SetConditions(runtimev1alpha1.Creating())

    // Create must return any connection details that are set or returned only
    // at creation time. The managed.Reconciler will merge any details
    // with those returned during the Observe phase.
    password := database.GeneratePassword()
    cd := managed.ConnectionDetails{runtimev1alpha1.ResourceCredentialsSecretPasswordKey: []byte(password)}

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
// when a managed resource with the 'Delete' reclaim policy has been deleted.
func (e *external) Delete(ctx context.Context, mg resource.Managed) error {
    i, ok := mg.(*v1alpha3.FavouriteDBInstance)
    if !ok {
        return errors.New("managed resource is not a FavouriteDBInstance")
    }
    // Indicate that we're about to delete the instance.
    i.SetConditions(runtimev1alpha1.Deleting())

    // Delete the instance.
    err := e.client.DeleteInstance(ctx, i.Spec.Name)

    // Note that we use resource.Ignore to squash any error that indicates the
    // external resource does not exist. Delete implementations must not return
    // an error when asked to delete a non-existent external resource.
    return errors.Wrap(resource.Ignore(database.IsNotFound, err), "cannot delete instance")
}
```

### Resource Claim Scheduling Controllers

Scheduling controllers should use [`claimscheduling.NewReconciler`] to specify
the resource claim kind it schedules and the resource class kind it schedules
them to. Note that unlike their resource claim kinds, resource claim scheduling
controllers are always part of the infrastructure provider that defines the
resource class they schedule claims to. The following is an example controller
that reconciles the `FancySQLInstance` resource claim by scheduling it to a
`FavouriteDBInstanceClass`:

```go
import (
    "fmt"
    "strings"

    ctrl "sigs.k8s.io/controller-runtime"

    runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
    "github.com/crossplane/crossplane-runtime/pkg/resource"
    "github.com/crossplane/crossplane-runtime/pkg/reconciler/claimscheduling"

    // Note that the hypothetical FancySQL resource claim is part of Crossplane,
    // not provider-fcp, because it is (hypothetically) portable across multiple
    // infrastructure providers.
    databasev1alpha1 "github.com/crossplane/crossplane/apis/database/v1alpha1"

    "github.com/crossplane/provider-fcp/apis/database/v1alpha3"
)

type PostgreSQLInstanceClaimSchedulingController struct{}

// SetupWithManager instantiates a new controller using a
// resource.ClaimSchedulingReconciler configured to reconcile FancySQLInstances
// by scheduling them to FavouriteDBInstanceClasses.
func (c *FancySQLInstanceClaimSchedulingController) SetupWithManager(mgr ctrl.Manager) error {
    // It's Crossplane convention to name resource claim scheduling controllers
    // "scheduler.claimkind.resourcekind.resourceapigroup", for example in this
    // case "fancysqlinstance.favouritedbinstance.fcp.crossplane.io".
    name := strings.ToLower(fmt.Sprintf("scheduler.%s.%s.%s",
        databasev1alpha1.FancySQLInstanceKind,
        v1alpha3.FavouriteDBInstanceKind,
        v1alpha3.Group))

    return ctrl.NewControllerManagedBy(mgr).
        Named(name).
        For(&databasev1alpha1.FancySQLInstance{}).
        WithEventFilter(resource.NewPredicates(resource.AllOf(
            // Claims must supply a class selector to be scheduled. Claims that
            // do not supply a class selector use a default resource class, if
            // one exists.
            resource.HasClassSelector(),

            // Claims with a class reference have either already been scheduled
            // to a resource class, or specified one explicitly.
            resource.HasNoClassReference(),

            // Claims with a managed resource reference are either already bound
            // to a managed resource, or are requesting to be bound to an
            // existing managed resource.
            resource.HasNoManagedResourceReference(),
        ))).
        Complete(claimscheduling.NewReconciler(mgr,
            resource.ClaimKind(databasev1alpha1.FancySQLInstanceGroupVersionKind),
            resource.ClassKind(v1alpha3.FavouriteDBInstanceClassGroupVersionKind),
        ))
}
```

### Resource Claim Defaulting Controllers

Defaulting controllers are configured almost (but not quite) identically to
scheduling controllers. They use a [`claimdefaulting.NewReconciler`] to specify
the resource claim kind they configure and the resource class kind they default
to. Unlike their resource claim kinds, defaulting controllers are always part of
the infrastructure provider that defines the resource class they default claims
to. The following is an example controller that reconciles the
`FancySQLInstance` resource claim by setting its class reference to a
`FavouriteDBInstanceClass` annotated as the default class:

```go
import (
    "fmt"
    "strings"

    ctrl "sigs.k8s.io/controller-runtime"

    runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
    "github.com/crossplane/crossplane-runtime/pkg/resource"
    "github.com/crossplane/crossplane-runtime/pkg/reconciler/claimdefaulting"

    // Note that the hypothetical FancySQL resource claim is part of Crossplane,
    // not provider-fcp, because it is (hypothetically) portable across multiple
    // infrastructure providers.
    databasev1alpha1 "github.com/crossplane/crossplane/apis/database/v1alpha1"

    "github.com/crossplane/provider-fcp/apis/database/v1alpha3"
)

type PostgreSQLInstanceClaimDefaultingController struct{}

// SetupWithManager instantiates a new controller using a
// resource.ClaimDefaultingReconciler configured to reconcile FancySQLInstances
// by scheduling them to FavouriteDBInstanceClasses.
func (c *FancySQLInstanceClaimDefaultingController) SetupWithManager(mgr ctrl.Manager) error {
    // It's Crossplane convention to name resource claim scheduling controllers
    // "defaulter.claimkind.resourcekind.resourceapigroup", for example in this
    // case "fancysqlinstance.favouritedbinstance.fcp.crossplane.io".
    name := strings.ToLower(fmt.Sprintf("scheduler.%s.%s.%s",
        databasev1alpha1.FancySQLInstanceKind,
        v1alpha3.FavouriteDBInstanceKind,
        v1alpha3.Group))

    return ctrl.NewControllerManagedBy(mgr).
        Named(name).
        For(&databasev1alpha1.FancySQLInstance{}).
        WithEventFilter(resource.NewPredicates(resource.AllOf(
            // Claims with a class selector desire scheduling to a matching
            // resource class, and are not subject to defaulting.
            resource.HasNoClassSelector(),

            // Claims with a class reference have either already been scheduled
            // to a resource class, or specified one explicitly.
            resource.HasNoClassReference(),

            // Claims with a managed resource reference are either already bound
            // to a managed resource, or are requesting to be bound to an
            // existing managed resource.
            resource.HasNoManagedResourceReference(),
        ))).
        Complete(claimdefaulting.NewReconciler(mgr,
            resource.ClaimKind(databasev1alpha1.FancySQLInstanceGroupVersionKind),
            resource.ClassKind(v1alpha3.FavouriteDBInstanceClassGroupVersionKind),
        ))
}
```

### Resource Claim Controllers

Resource claim controllers should use [`claimbinding.NewReconciler`] to wrap a
managed-resource specific implementation of
[`claimbinding.ManagedConfigurator`]. Parts of `claimbinding.Reconciler`'s
behaviour is customisable; refer to the [`claimbinding.NewReconciler`] GoDoc for
a list of options. Note that unlike their resource claim kinds, resource claim
controllers are always part of the infrastructure provider that defines the
managed resource they reconcile claims with. The following is an example
controller that reconciles the `FancySQLInstance` resource claim with the
`FavouriteDBInstance` managed resource:

```go
import (
    "context"
    "fmt"
    "strings"

    "github.com/pkg/errors"
    corev1 "k8s.io/api/core/v1"
    ctrl "sigs.k8s.io/controller-runtime"
    "sigs.k8s.io/controller-runtime/pkg/source"

    runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
    "github.com/crossplane/crossplane-runtime/pkg/resource"
    "github.com/crossplane/crossplane-runtime/pkg/reconciler/claimbinding"

    // Note that the hypothetical FancySQL resource claim is part of Crossplane,
    // not provider-fcp, because it is (hypothetically) portable across multiple
    // infrastructure providers.
    databasev1alpha1 "github.com/crossplane/crossplane/apis/database/v1alpha1"

    "github.com/crossplane/provider-fcp/apis/database/v1alpha3"
)

type FavouriteDBInstanceClaimController struct{}

// SetupWithManager instantiates a new controller using a resource.ClaimReconciler
// configured to reconcile FancySQLInstances by binding them to FavouriteDBInstances.
func (c *FavouriteDBInstanceClaimController) SetupWithManager(mgr ctrl.Manager) error {
    // It's Crossplane convention to name resource claim controllers
    // "claimkind.resourcekind.resourceapigroup", for example in this case
    // "fancysqlinstance.favouritedbinstance.fcp.crossplane.io".
    name := strings.ToLower(fmt.Sprintf("%s.%s.%s",
        databasev1alpha1.FancySQLInstanceKind,
        v1alpha3.FavouriteDBInstanceKind,
        v1alpha3.Group))

    // The controller below watches for changes to both FancySQLInstance and
    // FavouriteDBInstance kind resources. We use watch predicates to filter
    // out any requests to reconcile resources that we're not interested in.
    p := resource.NewPredicates(resource.AnyOf(
        // We want to reconcile FancySQLInstance kind resource claims that
        // reference a FavouriteDBInstanceClass.
        resource.HasClassReferenceKind(resource.ClassKind(v1alpha3.FavouriteDBInstanceClassGroupVersionKind),

        // We want to reconcile FancySQLInstance kind resource claims that
        // explicitly set their .spec.resourceRef to a FavouriteDBInstance kind
        // managed resource.
        resource.HasManagedResourceReferenceKind(resource.ManagedKind(v1alpha3.FavouriteDBInstanceGroupVersionKind)),

        // We want to reconcile FavouriteDBInstance managed resources. Resources
        // without a claim reference will be filtered by the below
        // EnqueueRequestForClaim watch event handler.
        resource.IsManagedKind(resource.ManagedKind(v1alpha3.FavouriteDBInstanceClassGroupVersionKind), mgr.GetScheme()),
    ))

    // Create a new resource claim reconciler...
    r := claimbinding.NewReconciler(mgr,
        // ..that uses the supplied claim, class, and managed resource kinds.
        resource.ClaimKind(databasev1alpha1.FancySQLInstanceGroupVersionKind),
        resource.ClassKind(v1alpha3.FavouriteDBInstanceClassGroupVersionKind),
        resource.ManagedKind(v1alpha3.FavouriteDBInstanceGroupVersionKind),
        // The following configurators configure how a managed resource will be
        // configured when one must be dynamically provisioned.
        claimbinding.WithManagedConfigurators(
            claimbinding.ManagedConfiguratorFn(ConfigureFavouriteDBInstance),
            claimbinding.NewObjectMetaConfigurator(mgr.GetScheme()),
        ))

    // Note that we watch for both FancySQLInstance and FavouriteDBInstance
    // resources. When the latter passes our predicates we look up the resource
    // claim it references and reconcile that claim.
    return ctrl.NewControllerManagedBy(mgr).
        Named(name).
        Watches(&source.Kind{Type: &v1alpha3.FavouriteDBInstance{}}, &resource.EnqueueRequestForClaim{}).
        For(&databasev1alpha1.FancySQLInstance{}).
        WithEventFilter(p).
        Complete(r)
}

// ConfigureFavouriteDBInstance is responsible for updating the supplied managed
// resource using the supplied resource class.
func ConfigureFavouriteDBInstance(_ context.Context, cm resource.Claim, cs resource.Class, mg resource.Managed) error {
    if _, ok := cm.(*databasev1alpha1.FancySQLInstance); !ok {
        return errors.New("resource claim is not a FancySQLInstance")
    }

    class, ok := cs.(*v1alpha3.FavouriteDBInstanceClass)
    if !ok {
        return errors.New("resource class is not a FavouriteDBInstanceClass")
    }

    instance, ok := mg.(*v1alpha3.FavouriteDBInstance)
    if !ok {
        return errors.New("managed resource is not a FavouriteDBInstance")
    }

    instance.Spec = v1alpha3.FavouriteDBInstanceSpec{
        ResourceSpec: runtimev1alpha1.ResourceSpec{
            // It's typical for dynamically provisioned managed resources to
            // store their connection details in a Secret named for the claim's
            // UID. Managed resource secrets are not intended for human
            // consumption; they're copied to the resource claim's secret when
            // the resource is bound.
            WriteConnectionSecretToReference: runtimev1alpha1.SecretReference{
                Namespace: class.SpecTemplate.WriteConnectionSecretsToNamespace,
                Name:      string(cm.GetUID()),
            },
            ProviderReference:                class.SpecTemplate.ProviderReference,
            ReclaimPolicy:                    class.SpecTemplate.ReclaimPolicy,
        },
        FavouriteDBInstanceParameters: class.SpecTemplate.FavouriteDBInstanceParameters,
    }

    return nil
}
```

### Connection Secret Propagation Controller

Managed resource kinds that may update their connection secrets after creation
time must instantiate a connection secret propagation controller. This
controller ensures any updates to the managed resource's connection secret are
propagated to the connection secret of its bound resource claim. The resource
claim reconciler ensures managed resource and resource claim secrets are
eligible for use with by the secret propagatation controller by adding the
appropriate annotations and controller references.

The following controller propagates any changes made to a `FavouriteDBInstance`
connection secret to the connection secret of its bound `FancySQLInstance`:

```go
import (
    "fmt"
    "strings"

    corev1 "k8s.io/api/core/v1"
    ctrl "sigs.k8s.io/controller-runtime"
    "sigs.k8s.io/controller-runtime/pkg/source"

    "github.com/crossplane/crossplane-runtime/pkg/resource"
    "github.com/crossplane/crossplane-runtime/pkg/reconciler/secret"
    databasev1alpha1 "github.com/crossplane/crossplane/apis/database/v1alpha1"

    "github.com/crossplane/provider-fcp/apis/database/v1alpha3"
)

type FavouriteDBInstanceSecretController struct{}

func (c *FavouriteDBInstanceSecretController) SetupWithManager(mgr ctrl.Manager) error {
    p := resource.NewPredicates(resource.AnyOf(
        resource.AllOf(resource.IsControlledByKind(databasev1alpha1.FancySQLInstanceGroupVersionKind), resource.IsPropagated()),
        resource.AllOf(resource.IsControlledByKind(v1alpha3.FavouriteDBInstanceGroupVersionKind), resource.IsPropagator()),
    ))

    return ctrl.NewControllerManagedBy(mgr).
        Named(strings.ToLower(fmt.Sprintf("connectionsecret.%s.%s", v1alpha3.FavouriteDBInstanceKind, v1alpha3.Group))).
        Watches(&source.Kind{Type: &corev1.Secret{}}, &resource.EnqueueRequestForPropagated{}).
        For(&corev1.Secret{}).
        WithEventFilter(p).
        Complete(secret.NewReconciler(mgr))
}
```

### Target Controller

Managed resources that represent a hosted Kubernetes cluster can be referenced
by `KubernetesTarget` resources in a namespace where `KubernetesApplication`
resources want to be created and scheduled to the remote cluster. This
controller evaluates if a newly created `KubernetesTarget` references its hosted
Kubernetes cluster managed resource, and if so, propagates its connection
information to the namespace of the `KubernetesTarget`. It will also set
annotations on the propagated secret in case there is a connection secret
controller that is set to continously propagate the connection information as it
changes.

The following controller propagates the connection secret of a
`FavouriteCluster` to the namespace of a `KubernetesTarget` that references it.
Note that `FavouriteCluster` is used instead of `FavouriteDBInstance` due to the
fact that Target controllers are currently only utilized for managed resources
that represent a hosted Kubernetes cluster offering.

```go
import (
    "fmt"
    "strings"

    ctrl "sigs.k8s.io/controller-runtime"

    "github.com/crossplane/crossplane-runtime/pkg/reconciler/target"
    "github.com/crossplane/crossplane-runtime/pkg/resource"
    workloadv1alpha1 "github.com/crossplane/crossplane/apis/workload/v1alpha1"

    "github.com/crossplane/provider-fcp/apis/compute/v1alpha3"
)

type FavoriteClusterTargetController struct{}

func (c *FavouriteClusterTargetController) SetupWithManager(mgr ctrl.Manager) error {
    p := resource.NewPredicates(resource.HasManagedResourceReferenceKind(resource.ManagedKind(v1alpha3.FavouriteClusterGroupVersionKind)))

    r := target.NewReconciler(mgr,
        resource.TargetKind(workloadv1alpha1.KubernetesTargetGroupVersionKind),
        resource.ManagedKind(v1alpha3.FavouriteClusterGroupVersionKind))

    return ctrl.NewControllerManagedBy(mgr).
        Named(strings.ToLower(fmt.Sprintf("kubernetestarget.%s.%s", v1alpha3.FavouriteClusterKind, v1alpha3.Group))).
        For(&workloadv1alpha1.KubernetesTarget{}).
        WithEventFilter(p).
        Complete(r)
}
```

### Wrapping Up

Once all your controllers are in place you'll want to test them. Note that most
projects under the [crossplane org] [favor] table driven tests that use Go's
standard library `testing` package over kubebuilder's Gingko based tests.

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

In this guide we walked through the process of defining all of the resource
kinds and controllers necessary to build support for a new managed service;
possibly even a completely new infrastructure provider. Please do not hesitate
to [reach out] to the Crossplane maintainers and community for help designing
and implementing support for new managed services. [#sig-services] would highly
value any feedback you may have about the services development process!

<!-- Named Links -->

[What Makes a Crossplane Managed Service]: #what-makes-a-crossplane-managed-service
[managed resource]: concepts.md#managed-resource
[resource claim]: concepts.md#resource-claim
[resource class]: concepts.md#resource-class
[dynamic provisioning]: concepts.md#dynamic-and-static-provisioning
[`CloudMemorystoreInstance`]: https://github.com/crossplane/provider-gcp/blob/85a6ed3c669a021f1d61be51b2cbe2714b0bc70b/apis/cache/v1beta1/cloudmemorystore_instance_types.go#L184
[`CloudMemorystoreInstanceClass`]: https://github.com/crossplane/provider-gcp/blob/85a6ed3c669a021f1d61be51b2cbe2714b0bc70b/apis/cache/v1beta1/cloudmemorystore_instance_types.go#L217
[`Provider`]: https://github.com/crossplane/provider-gcp/blob/85a6ed3c669a021f1d61be51b2cbe2714b0bc70b/apis/v1alpha3/types.go#L41
[`RedisCluster`]: https://github.com/crossplane/crossplane/blob/3c6cf4e/apis/cache/v1alpha1/rediscluster_types.go#L40
[`RedisClusterClass`]: https://github.com/crossplane/crossplane/blob/3c6cf4e/apis/cache/v1alpha1/rediscluster_types.go#L116
[watching the API server]: https://kubernetes.io/docs/reference/using-api/api-concepts/#efficient-detection-of-changes
[kubebuilder]: https://kubebuilder.io/
[controller-runtime]: https://github.com/kubernetes-sigs/controller-runtime
[crossplane-runtime]: https://github.com/crossplane/crossplane-runtime/
[crossplane-runtime v0.4.0]: https://github.com/crossplane/crossplane-runtime/releases/tag/v0.4.0
[golden path]: https://charity.wtf/2018/12/02/software-sprawl-the-golden-path-and-scaling-teams-with-agency/
[API Conventions]: https://github.com/kubernetes/community/blob/c6e1e89a/contributors/devel/sig-architecture/api-conventions.md
[kubebuilder book]: https://book.kubebuilder.io/
[Stacks quick start]: https://github.com/crossplane/crossplane-cli/blob/357d18e7b/README.md#quick-start-stacks
[resources]: https://kubebuilder.io/cronjob-tutorial/gvks.html#kinds-and-resources
[kinds]: https://kubebuilder.io/cronjob-tutorial/gvks.html#kinds-and-resources
[objects]: https://kubernetes.io/docs/concepts/#kubernetes-objects
[comment marker]: https://kubebuilder.io/reference/markers.html
[comment markers]: https://kubebuilder.io/reference/markers.html
[`resource.Managed`]: https://godoc.org/github.com/crossplane/crossplane-runtime/pkg/resource#Managed
[`resource.Claim`]: https://godoc.org/github.com/crossplane/crossplane-runtime/pkg/resource#Claim
[`resource.Class`]: https://godoc.org/github.com/crossplane/crossplane-runtime/pkg/resource#Class
[`managed.Reconciler`]: https://godoc.org/github.com/crossplane/crossplane-runtime/pkg/reconciler/managed#Reconciler
[`managed.NewReconciler`]: https://godoc.org/github.com/crossplane/crossplane-runtime/pkg/reconciler/managed#NewReconciler
[`claimbinding.Reconciler`]: https://godoc.org/github.com/crossplane/crossplane-runtime/pkg/reconciler/claimbinding#Reconciler
[`claimbinding.NewReconciler`]: https://godoc.org/github.com/crossplane/crossplane-runtime/pkg/reconciler/claimbinding#NewReconciler
[`claimscheduling.Reconciler`]: https://godoc.org/github.com/crossplane/crossplane-runtime/pkg/reconciler/claimscheduling#Reconciler
[`claimscheduling.NewReconciler`]: https://github.com/crossplane/crossplane-runtime/blob/master/pkg/reconciler/claimscheduling/reconciler.go#L83
[`claimdefaulting.Reconciler`]: https://godoc.org/github.com/crossplane/crossplane-runtime/pkg/reconciler/claimdefaulting#Reconciler
[`claimdefaulting.NewReconciler`]: https://godoc.org/github.com/crossplane/crossplane-runtime/pkg/reconciler/claimdefaulting#NewReconciler
[`secret.NewReconciler`]: https://godoc.org/github.com/crossplane/crossplane-runtime/pkg/reconciler/secret#NewReconciler
[`managed.ExternalConnecter`]: https://godoc.org/github.com/crossplane/crossplane-runtime/pkg/reconciler/managed#ExternalConnecter
[`managed.ExternalClient`]: https://godoc.org/github.com/crossplane/crossplane-runtime/pkg/reconciler/managed#ExternalClient
[`claimbinding.ManagedConfigurator`]: https://godoc.org/github.com/crossplane/crossplane-runtime/pkg/reconciler/claimbinding#ManagedConfigurator
[`target.Reconciler`]: https://godoc.org/github.com/crossplane/crossplane-runtime/pkg/reconciler/target#Reconciler
[`ResourceSpec`]: https://godoc.org/github.com/crossplane/crossplane-runtime/apis/core/v1alpha1#ResourceSpec
[`ResourceStatus`]: https://godoc.org/github.com/crossplane/crossplane-runtime/apis/core/v1alpha1#ResourceStatus
[`ResourceClaimSpec`]: https://godoc.org/github.com/crossplane/crossplane-runtime/apis/core/v1alpha1#ResourceClaimSpec
[`ResourceClaimStatus`]: https://godoc.org/github.com/crossplane/crossplane-runtime/apis/core/v1alpha1#ResourceClaimStatus
[`ClassSpecTemplate`]: https://godoc.org/github.com/crossplane/crossplane-runtime/apis/core/v1alpha1#ClassSpecTemplate
[`ProviderSpec`]: https://godoc.org/github.com/crossplane/crossplane-runtime/apis/core/v1alpha1#ProviderSpec
['managed.ExternalConnecter`]: https://godoc.org/github.com/crossplane/crossplane-runtime/pkg/reconciler/managed#ExternalConnecter
[opening a Crossplane issue]: https://github.com/crossplane/crossplane/issues/new/choose
[`GroupVersionKind`]: https://godoc.org/k8s.io/apimachinery/pkg/runtime/schema#GroupVersionKind
[`reconcile.Reconciler`]: https://godoc.org/sigs.k8s.io/controller-runtime/pkg/reconcile#Reconciler
[favor]: https://github.com/crossplane/crossplane/issues/452
[reach out]: https://github.com/crossplane/crossplane#contact
[#sig-services]: https://crossplane.slack.com/messages/sig-services
[crossplane org]: https://github.com/crossplane
[`angryjet`]: https://github.com/crossplane/crossplane-tools
[Managed Resource API Patterns]: ../design/one-pager-managed-resource-api-design.md
[Crossplane CLI]: https://github.com/crossplane/crossplane-cli#quick-start-stacks
[`angryjet` documentation]: https://github.com/crossplane/crossplane-tools/blob/master/README.md
