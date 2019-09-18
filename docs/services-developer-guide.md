---
title: Services Developer Guide
toc: true
weight: 720
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
and three controllers. The GCP Stack's support for Google Cloud Memorystore
illustrates this. First, the configuration resources:

1. A [managed resource]. Managed resources are high-fidelity representations of
   a resource in an external system such as a cloud provider's API. Managed
   resources are _non-portable_ across external systems (i.e. cloud providers);
   they're tightly coupled to the implementation details of the external
   resource they represent. Managed resources are defined by a Stack. The GCP
   Stack's [`CloudMemorystoreInstance`] resource is an example of a managed
   resource.
1. A [resource claim]. Resource claims are abstract declarations of a need for a
   service. Resource claims are frequently portable across external systems.
   Crossplane defines a series of common resource claim kinds, including
   [`RedisCluster`]. A resource claim is satisfied by _binding_ to a managed
   resource.
1. A non-portable [resource class]. Non-portable resource classes represent a
   class of a specific kind of managed resource. They are the template used to
   create a new managed resource in order to satisfy a resource claim during
   [dynamic provisioning]. Non-portable resource classes are tightly coupled to
   the managed resources they template. [`CloudMemorystoreInstanceClass`] is an
   example of a non-portable resource class.
1. A portable [resource class]. Portable resource classes are a pointer from a
   resource claim to a non-portable resource class that should be used for
   dynamic provisioning. [`RedisClusterClass`] is an example of a portable
   resource claim.
1. A provider. Providers enable access to an external system, typically by
   indicating a Kubernetes Secret containing any credentials required to
   authenticate to the system, as well as any other metadata required to
   connect. The GCP [`Provider`] is an example of a provider.

These resources are powered by:

1. The managed resource controller. This controller is responsible for taking
   instances of the aforementioned high-fidelity managed resource kind and
   reconciling them with an external system. Managed resource controllers are
   unaware of resource claims or classes. The `CloudMemorystoreInstance`
   controller watches for changes to `CloudMemorystoreInstance` resources and
   calls Google's Cloud Memorystore API to create, update, or delete an instance
   as necessary.
1. The resource claim controller. A resource claim controller exists for each
   kind of managed resource that could satisfy a resource claim. This controller
   is unaware of any external system - it responsible only for taking resource
   claims and binding them to a managed resource.  The
   `CloudMemorystoreInstance` resource claim controller watches for
   `RedisCluster` resource claims that should be satisfied by a
   `CloudMemorystoreInstance`. It either binds to an explicitly referenced
   `CloudMemorystoreInstance` (static provisioning) or creates a new one and
   then binds to it (dynamic provisioning).
1. A default resource class controller. The `RedisCluster` default resource
   class controller watches for `RedisCluster` instances that don't explicitly
   reference a managed resource _or_ a portable resource class, and sets a
   default resource class if one is set. Only one instance of this defaulting
   controller exists per resource claim kind.

Crossplane does not require controllers to be written in any particular
language. The Kubernetes API server is our API boundary, so any process capable
of [watching the API server] and updating resources can be a Crossplane
controller.

## Getting Started

At the time of writing all Crossplane Services controllers are written in Go,
and built using [kubebuilder] v0.2.x and [crossplane-runtime].  Per [What Makes
a Crossplane Managed Service] it is possible to write a controller using any
language and tooling with a Kubernetes client, but this set of tools are the
"[golden path]". They're well supported, broadly used, and provide a shared
language with the Crossplane maintainers.

This guide assumes the reader is familiar with the Kubernetes [API Conventions]
and the [kubebuilder book]. If you're not adding a new managed service to an
existing Crossplane Stack you should start by working through the [Stacks quick
start] to scaffold a new Stack in which the new types and controllers will live.

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
they'll be a managed resource, resource class, or resource claim:

```shell
# The resource claim.
kubebuilder create api \
    --group example --version v1alpha1 --kind FancySQLInstance \
    --resource=true --controller=false
```

The above command should produce a scaffold similar to the below
example:

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

Note that in many cases a suitable provider, resource claim, and portable
resource class will already exist. Frequently adding support for a new managed
service requires only the definition of a new managed resource and non-portable
resource class.

### Managed Resource Kinds

Managed resources must:

* Satisfy crossplane-runtime's [`resource.Managed`] interface.
* Embed a [`ResourceStatus`] struct in their `Status` struct.
* Embed a [`ResourceSpec`] struct in their `Spec` struct.
* Embed a `Parameters` struct in their `Spec` struct.
* Use the `+kubebuilder:subresource:status` [comment marker].

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
    // https://github.com/crossplaneio/crossplane/issues/624 for context.

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
    FavouriteDBInstanceParameters `json:",inline"`
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

// +kubebuilder:object:root=true

// A FavouriteDBInstance is a managed resource that represents a Favourite DB
// instance.
// +kubebuilder:subresource:status
type FavouriteDBInstance struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   FavouriteDBInstanceSpec   `json:"spec,omitempty"`
    Status FavouriteDBInstanceStatus `json:"status,omitempty"`
}


// SetBindingPhase of this FavouriteDBInstance.
func (i *FavouriteDBInstance) SetBindingPhase(p runtimev1alpha1.BindingPhase) {
    i.Status.SetBindingPhase(p)
}

// This example omits several getters and setters that are required to satisfy
// the resource.Managed interface in the interest of brevity. These methods all
// set or get a field of one of the embedded structs, similar to SetBindingPhase
// above.
```

Note that Crossplane uses the GoDoc strings of API kinds to generate user facing
API documentation. __Document all fields__ and prefer GoDoc that assumes the
reader is running `kubectl explain`, or reading an API reference, not reading
the code.

### Non-Portable Class Kinds

The non-portable resource class kind for a particular managed resource kind are
typically defined in the same file as their the managed resource. Non-portable
resource classes must:

* Satisfy crossplane-runtime's [`resource.NonPortableClass`] interface.
* Have a `SpecTemplate` struct field instead of a `Spec`.
* Embed a [`NonPortableClassSpecTemplate`] struct in their `SpecTemplate`
  struct.
* Embed their managed resource's `Parameters` struct in their `SpecTemplate`
  struct.
* Not have a `Status` struct.

A non-portable resource class for the above `FavouriteDBInstance` would look as
follows:

```go
// A FavouriteDBInstanceClassSpecTemplate is a template for the spec of a
// dynamically provisioned FavouriteDBInstance.
type FavouriteDBInstanceClassSpecTemplate struct {
    runtimev1alpha1.NonPortableClassSpecTemplate `json:",inline"`
    FavouriteDBInstanceParameters                `json:",inline"`
}

// +kubebuilder:object:root=true

// A FavouriteDBInstanceClass is a non-portable resource class. It defines the
// desired spec of resource claims that use it to dynamically provision a
// managed resource.
type FavouriteDBInstanceClass struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    // SpecTemplate is a template for the spec of a dynamically provisioned
    // FavouriteDBInstance.
    SpecTemplate FavouriteDBInstanceSpecTemplate `json:"specTemplate,omitempty"`
}

// This example omits getters and setters that are required to satisfy the
// resource.NonPortableClass interface in the interest of brevity. These methods
// all set or get a field of one of the embedded structs.
```

### Resource Claim Kinds

Once the underlying managed resource and its non-portable resource class have
been defined the next step is to define the resource claim. Resource claim
controllers typically live alongside their managed resource controllers (i.e. in
an infrastructure stack), but at the time of writing all resource claim kinds
are defined in Crossplane core. This is because resource claims can frequently
be satisfied by binding to managed resources from more than one cloud. Consider
[opening a Crossplane issue] to propose adding your new resource claim kind to
Crossplane if it could be satisfied by managed resources from more than one
infrastructure stack.

Resource claims must:

* Satisfy crossplane-runtime's [`resource.Claim`] interface.
* Use (not embed) a [`ResourceClaimStatus`] struct as their `Status` field.
* Embed a [`ResourceClaimSpec`] struct in their `Spec` struct.
* Use the `+kubebuilder:subresource:status` [comment marker].

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

// +kubebuilder:object:root=true

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

// This example omits several getters and setters that are required to satisfy
// the resource.Claim interface in the interest of brevity. These methods all
// set or get a field of one of the embedded structs, similar to SetBindingPhase
// above.
```

### Portable Resource Class Kinds

Portable resource classes are typically defined alongside the resource claim
they align with, similar to how non-portable resource classes are defined
alongside the managed resources they align with.

Portable resource classes must:

* Satisfy crossplane-runtime's [`resource.PortableClass`] interface.
* Directly embed a [`PortableClass`] struct.
* Have no `Spec` field.
* Have no `Status` field.
* Have a corresponding List type that satisfies the
  [`resource.PortableClassList`] interface.

The `FancySQLInstanceClass` portable resource class would look as follows:

```go
// FancySQLInstanceClass contains a namespace-scoped portable class for
// FancySQLInstance
type FancySQLInstanceClass struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    runtimev1alpha1.PortableClass `json:",inline"`
}

// +kubebuilder:object:root=true

// FancySQLInstanceClassList contains a list of FancySQLInstanceClass.
type FancySQLInstanceClassList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata,omitempty"`

    Items           []FancySQLInstanceClass `json:"items"`
}

// SetPortableClassItems of this FancySQLInstanceClassList.
func (rc *FancySQLInstanceClassList) SetPortableClassItems(r []resource.PortableClass) {
    items := make([]FancySQLInstanceClass, 0, len(r))
    for i := range r {
        if item, ok := r[i].(*FancySQLInstanceClass); ok {
            items = append(items, *item)
        }
    }
    rc.Items = items
}

// GetPortableClassItems of this FancySQLInstanceClassList.
func (rc *FancySQLInstanceClassList) GetPortableClassItems() []resource.PortableClass {
    items := make([]resource.PortableClass, len(rc.Items))
    for i, item := range rc.Items {
        item := item
        items[i] = resource.PortableClass(&item)
    }
    return items
}
```

### Provider Kinds

You'll typically only need to add a new Provider kind if you're creating an
infrastructure stack that adds support for a new infrastructure provider.

Providers must:

* Be named exactly `Provider`.
* Have a `Spec` struct with a `Secret` field indicating where to find
  credentials for this provider.

The Favourite Cloud `Provider` would look as follows. Note that the cloud to
which it belongs should be indicated by its API group, i.e. its API Version
would be `favouritecloud.crossplane.io/v1alpha1` or similar.

```go
// A ProviderSpec defines the desired state of a Provider.
type ProviderSpec struct {

    // A Secret containing credentials for a Favourite Cloud Service Account
    // that will be used to authenticate to this Provider.
    Secret corev1.SecretKeySelector `json:"credentialsSecretRef"`
}

// +kubebuilder:object:root=true

// A Provider configures a Favourite Cloud 'provider', i.e. a connection to a
// particular Favourite Cloud project using a particular Favourite Cloud service
// account.
type Provider struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec ProviderSpec `json:"spec,omitempty"`
}
```

### Finishing Touches

At this point we've defined all of the resource kinds necessary to start
building controllers - a managed resource, a non-portable resource class, a
resource claim, and a portable resource class. Before moving on to the
controllers:

* Add any kubebuilder [comment markers] that may be useful for your resource.
  Comment markers can be used to validate input, or add additional columns to
  the standard `kubectl get` output, among other things.
* Run `make generate && make manifests` (or `make reviewable` if you're working
  in one of the projects in the [crossplaneio org]) to generate Custom Resource
  Definitions and additional helper methods for your new resource kinds.
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

* The [`resource.ManagedReconciler`] reconciles managed resources with external
  systems by instantiating a client of the external API and using it to create,
  update, or delete the external resource as necessary.
* [`resource.ClaimReconciler`] reconciles resource claims with managed resources
  by either binding or dynamically provisioning and then binding them.
* [`resource.DefaultClassReconciler`] sets default resource classes for resource
  claims that need them.

Crossplane controllers typically differ sufficiently from those scaffolded by
kubebuilder that there is little value in using kubebuilder to generate a
controller scaffold.

### Managed Resource Controllers

Managed resource controllers should use [`resource.NewManagedReconciler`] to
wrap a managed-resource specific implementation of
[`resource.ExternalConnecter`]. Parts of `resource.ManagedReconciler`'s
behaviour is customisable; refer to the [`resource.NewManagedReconciler`] GoDoc
for a list of options. The following is an example controller for the
`FavouriteDBInstance` managed resource we defined earlier:

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

    runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
    "github.com/crossplaneio/crossplane-runtime/pkg/meta"
    "github.com/crossplaneio/crossplane-runtime/pkg/resource"

    "github.com/crossplaneio/stack-fcp/apis/database/v1alpha2"
    fcpv1alpha2 "github.com/crossplaneio/stack-fcp/apis/v1alpha2"
)

type FavouriteDBInstanceController struct{}

// SetupWithManager instantiates a new controller using a resource.ManagedReconciler
// configured to reconcile FavouriteDBInstances using an ExternalClient produced by
// connecter, which satisfies the ExternalConnecter interface.
func (c *FavouriteDBInstanceController) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        Named(strings.ToLower(fmt.Sprintf("%s.%s", v1alpha2.FavouriteDBInstanceKind, v1alpha2.Group))).
        For(&v1alpha2.FavouriteDBInstance{}).
        Complete(resource.NewManagedReconciler(mgr,
            resource.ManagedKind(v1alpha2.FavouriteDBInstanceGroupVersionKind),
            resource.WithExternalConnecter(&connecter{client: mgr.GetClient()})))
}

// Connecter satisfies the resource.ExternalConnecter interface.
type connecter struct{ client client.Client }

// Connect to the supplied resource.Managed (presumed to be a
// FavouriteDBInstance) by using the Provider it references to create a new
// database client.
func (c *connecter) Connect(ctx context.Context, mg resource.Managed) (resource.ExternalClient, error) {
    // Assert that resource.Managed we were passed in fact contains a
    // FavouriteDBInstance. We told NewControllerManagedBy that this was a
    // controller For FavouriteDBInstance, so something would have to go
    // horribly wrong for us to encounter another type.
    i, ok := mg.(*v1alpha2.FavouriteDBInstance)
    if !ok {
        return nil, errors.New("managed resource is not a FavouriteDBInstance")
    }

    // Get the Provider referenced by the FavouriteDBInstance.
    p := &fcpv1alpha2.Provider{}
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

// Observe the existing external resource, if any. The resource.ManagedReconciler
// calls Observe in order to determine whether an external resource needs to be
// created, updated, or deleted.
func (e *external) Observe(ctx context.Context, mg resource.Managed) (resource.ExternalObservation, error) {
    i, ok := mg.(*v1alpha2.FavouriteDBInstance)
    if !ok {
        return resource.ExternalObservation{}, errors.New("managed resource is not a FavouriteDBInstance")
    }

    // Use our FavouriteDB API client to get an up to date view of the external
    // resource.
    existing, err := e.client.GetInstance(ctx, i.Spec.Name)

    // If we encounter an error indicating the external resource does not exist
    // we want to let the resource.ManagedReconciler know so it can create it.
    if database.IsNotFound(err) {
        return resource.ExternalObservation{ResourceExists: false}, nil
    }

    // Any other errors are wrapped (as is good Go practice) and returned to the
    // resource.ManagedReconciler. It will update the "Synced" status condition
    // of the managed resource to reflect that the most recent reconcile failed
    // and ensure the reconcile is reattempted after a brief wait.
    if err != nil {
        return resource.ExternalObservation{}, errors.Wrap(err, "cannot get instance")
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
    o := resource.ExternalObservation{
        ResourceExists:   true,
        ResourceUpToDate: existing.GetFancinessLevel == i.Spec.FancinessLevel,
        ConnectionDetails: resource.ConnectionDetails{
            runtimev1alpha1.ResourceCredentialsSecretUserKey:     []byte(existing.GetUsername()),
            runtimev1alpha1.ResourceCredentialsSecretEndpointKey: []byte(existing.GetHostname()),
        },
    }

    return o, nil
}

// Create a new external resource based on the specification of our managed
// resource. resource.ManagedReconciler only calls Create if Observe reported
// that the external resource did not exist.
func (e *external) Create(ctx context.Context, mg resource.Managed) (resource.ExternalCreation, error) {
    i, ok := mg.(*v1alpha2.FavouriteDBInstance)
    if !ok {
        return resource.ExternalCreation{}, errors.New("managed resource is not a FavouriteDBInstance")
    }
    // Indicate that we're about to create the instance. Remember ExternalClient
    // authors can use a bespoke condition reason here in cases where Creating
    // doesn't make sense.
    i.SetConditions(runtimev1alpha1.Creating())

    // Create must return any connection details that are set or returned only
    // at creation time. The resource.ManagedReconciler will merge any details
    // with those returned during the Observe phase.
    password := database.GeneratePassword()
    cd := resource.ConnectionDetails{runtimev1alpha1.ResourceCredentialsSecretPasswordKey: []byte(password)}

    // Create a new instance.
    new := database.Instance{Name: i.Name, FancinessLevel: i.FancinessLevel, Version: i.Version}
    err := e.client.CreateInstance(ctx, new, password)

    // Note that we use resource.Ignore to squash any error that indicates the
    // external resource already exists. Create implementations must not return
    // an error if asked to create a resource that already exists. Real managed
    // resource controllers are advised to avoid unintentially 'adoptign' an
    // existing, unrelated external resource, per
    // https://github.com/crossplaneio/crossplane-runtime/issues/27
    return resource.ExternalCreation{ConnectionDetails: cd}, errors.Wrap(resource.Ignore(database.IsExists, err), "cannot create instance")
}

// Update the existing external resource to match the specifications of our
// managed resource. resource.ManagedReconciler only calls Update if Observe
// reported that the external resource was not up to date.
func (e *external) Update(ctx context.Context, mg resource.Managed) (resource.ExternalUpdate, error) {
    i, ok := mg.(*v1alpha2.FavouriteDBInstance)
    if !ok {
        return resource.ExternalUpdate{}, errors.New("managed resource is not a FavouriteDBInstance")
    }

    // Recall that FancinessLevel is the only field that we _can_ update.
    new := database.Instance{Name: i.Name, FancinessLevel: i.FancinessLevel}
    err := e.client.UpdateInstance(ctx, new)
    return resource.ExternalUpdate{}, errors.Wrap(err, "cannot update instance")
}

// Delete the external resource. resource.ManagedReconciler only calls Delete
// when a managed resource with the 'Delete' reclaim policy has been deleted.
func (e *external) Delete(ctx context.Context, mg resource.Managed) error {
    i, ok := mg.(*v1alpha2.FavouriteDBInstance)
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

### Resource Claim Controllers

Resource claim controllers should use [`resource.NewClaimReconciler`] to wrap a
managed-resource specific implementation of [`resource.ManagedConfigurator`].
Parts of `resource.ClaimReconciler`'s behaviour is customisable; refer to the
[`resource.NewClaimReconciler`] GoDoc for a list of options. Note that unlike
their resource claim kinds, resource claim controllers are always part of the
infrastructure stack that defines the managed resource they reconcile claims
with. The following is an example controller that reconciles the
`FancySQLInstance` resource claim with the `FavouriteDBInstance` managed
resource:

```go
import (
    "context"
    "fmt"
    "strings"

    "github.com/pkg/errors"
    corev1 "k8s.io/api/core/v1"
    ctrl "sigs.k8s.io/controller-runtime"
    "sigs.k8s.io/controller-runtime/pkg/source"

    runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
    "github.com/crossplaneio/crossplane-runtime/pkg/resource"

    // Note that the hypothetical FancySQL resource claim is part of Crossplane,
    // not stack-fcp, because it is (hypothetically) portable across multiple
    // infrastructure stacks.
    databasev1alpha1 "github.com/crossplaneio/crossplane/apis/database/v1alpha1"

    "github.com/crossplaneio/stack-fcp/apis/database/v1alpha2"
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
        v1alpha2.FavouriteDBInstanceKind,
        v1alpha2.Group))

    // The controller below watches for changes to both FancySQLInstance and
    // FavouriteDBInstance kind resources. We use watch predicates to filter
    // out any requests to reconcile resources that we're not interested in.
    p := resource.NewPredicates(resource.AnyOf(
        // We want to reconcile FancySQLInstance kind resource claims that
        // explicitly set their .spec.resourceRef to a FavouriteDBInstance kind
        // managed resource.
        resource.HasManagedResourceReferenceKind(resource.ManagedKind(v1alpha2.FavouriteDBInstanceGroupVersionKind)),

        // We want to reconcile FavouriteDBInstance managed resources that were
        // dynamically provisioned using the FavouriteDBInstanceClass. Note that
        // this predicate will soon be replaced by resource.IsManagedKind per
        // https://github.com/crossplaneio/crossplane-runtime/issues/28
        resource.HasDirectClassReferenceKind(resource.NonPortableClassKind(v1alpha2.FavouriteDBInstanceClassGroupVersionKind)),

        // We want to reconcile FancySQLInstance kind resource claims that
        // indirectly reference a FavouriteDBInstanceClass via a
        // FancySQLInstanceClass.
        resource.HasIndirectClassReferenceKind(mgr.GetClient(), mgr.GetScheme(), resource.ClassKinds{
            Portable:    databasev1alpha1.FancySQLInstanceClassGroupVersionKind,
            NonPortable: v1alpha2.FavouriteDBInstanceClassGroupVersionKind,
        })))

    // Create a new resource claim reconciler...
    r := resource.NewClaimReconciler(mgr,
        // ..that uses the supplied claim, class, and managed resource kinds.
        resource.ClaimKind(databasev1alpha1.FancySQLInstanceGroupVersionKind),
        resource.ClassKinds{
            Portable:    databasev1alpha1.FancySQLInstanceClassGroupVersionKind,
            NonPortable: v1alpha2.FavouriteDBInstanceClassGroupVersionKind,
        },
        resource.ManagedKind(v1alpha2.FavouriteDBInstanceGroupVersionKind),
        // The resource claim reconciler assumes managed resources do not
        // use the status subresource for compatibility with older managed
        // resource kinds, so well behaved resources must explicitly tell the
        // reconciler to update the status subresource per
        // https://github.com/crossplaneio/crossplane-runtime/issues/29
        resource.WithManagedBinder(resource.NewAPIManagedStatusBinder(mgr.GetClient())),
        resource.WithManagedFinalizer(resource.NewAPIManagedStatusUnbinder(mgr.GetClient())),
        // The following configurators configure how a managed resource will be
        // configured when one must be dynamically provisioned.
        resource.WithManagedConfigurators(
            resource.ManagedConfiguratorFn(ConfigureFavouriteDBInstance),
            resource.NewObjectMetaConfigurator(mgr.GetScheme()),
        ))

    // Note that we watch for both FancySQLInstance and FavouriteDBInstance
    // resources. When the latter passes our predicates we look up the resource
    // claim it references and reconcile that claim.
    return ctrl.NewControllerManagedBy(mgr).
        Named(name).
        Watches(&source.Kind{Type: &v1alpha2.FavouriteDBInstance{}}, &resource.EnqueueRequestForClaim{}).
        For(&databasev1alpha1.FancySQLInstance{}).
        WithEventFilter(p).
        Complete(r)
}

// ConfigureFavouriteDBInstance is responsible for updating the supplied managed
// resource using the supplied non-portable resource class.
func ConfigureFavouriteDBInstance(_ context.Context, cm resource.Claim, cs resource.NonPortableClass, mg resource.Managed) error {
    if _, ok := cm.(*databasev1alpha1.FancySQLInstance); !ok {
        return errors.New("resource claim is not a FancySQLInstance")
    }

    class, ok := cs.(*v1alpha2.FavouriteDBInstanceClass)
    if !ok {
        return errors.New("resource class is not a FavouriteDBInstanceClass")
    }

    instance, ok := mg.(*v1alpha2.FavouriteDBInstance)
    if !ok {
        return errors.New("managed resource is not a FavouriteDBInstance")
    }

    instance.Spec = v1alpha2.FavouriteDBInstanceSpec{
        ResourceSpec: runtimev1alpha1.ResourceSpec{
            // It's typical for dynamically provisioned managed resources to
            // store their connection details in a Secret named for the claim's
            // UID. Managed resource secrets are not intended for human
            // consumption; they're copied to the resource claim's secret when
            // the resource is bound.
            WriteConnectionSecretToReference: corev1.LocalObjectReference{Name: string(cm.GetUID())},
            ProviderReference:                class.SpecTemplate.ProviderReference,
            ReclaimPolicy:                    class.SpecTemplate.ReclaimPolicy,
        },
        FavouriteDBInstanceParameters: class.SpecTemplate.FavouriteDBInstanceParameters,
    }

    return nil
}
```

### Default Resource Class Controller

When adding support for a new resource claim kind a default resource class
controller is also necessary. This controller watches for resource claims that
don't have a managed resource reference _or_ a portable resource class reference
and sets their resource class reference to the default portable class for their
namespace, if one exists. Default resource class controllers should live near
the resource claim and portable resource class kind definitions; frequently in
Crossplane proper.

The following controller sets default resource classes for `FancySQLInstance`
resource claims:

```go
import (
    "fmt"
    "strings"

    ctrl "sigs.k8s.io/controller-runtime"

    "github.com/crossplaneio/crossplane-runtime/pkg/resource"

    databasev1alpha1 "github.com/crossplaneio/crossplane/apis/database/v1alpha1"
)

type FancySQLInstanceController struct{}

func (c *FancySQLInstanceController) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        Named(strings.ToLower(fmt.Sprintf("default.%s.%s", databasev1alpha1.FancySQLInstanceKind, databasev1alpha1.Group))).
        For(&databasev1alpha1.FancySQLInstance{}).
        WithEventFilter(resource.NewPredicates(resource.HasNoPortableClassReference())).
        WithEventFilter(resource.NewPredicates(resource.HasNoManagedResourceReference())).
        Complete(resource.NewDefaultClassReconciler(mgr,
            resource.ClaimKind(databasev1alpha1.FancySQLInstanceGroupVersionKind),
            resource.PortableClassKind{
                Singular: databasev1alpha1.FancySQLInstanceClassGroupVersionKind,
                Plural:   databasev1alpha1.FancySQLInstanceClassListGroupVersionKind,
            },
        ))
}
```

### Wrapping Up

Once all your controllers are in place you'll want to test them. Note that most
projects under the [crossplaneio org] [favor] table driven tests that use Go's
standard library `testing` package over kubebuilder's Gingko based tests.

Finally, don't forget to plumb any newly added resource kinds and controllers up
to your controller manager. Simple stacks may do this for each type within
within `main()`, but most more complicated stacks take an approach in which each
package exposes an `AddToScheme` (for resource kinds) or `SetupWithManager` (for
controllers) function that invokes the same function within its child packages,
resulting in a `main.go` like:

```go
import (
    "time"

    "sigs.k8s.io/controller-runtime/pkg/client/config"
    "sigs.k8s.io/controller-runtime/pkg/manager"
    "sigs.k8s.io/controller-runtime/pkg/runtime/signals"

    "github.com/crossplaneio/crossplane/apis"

    fcpapis "github.com/crossplaneio/stack-fcp/apis"
    "github.com/crossplaneio/stack-fcp/pkg/controller"
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

    if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
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
possibly even a completely new infrastructure stack. Please do not hesitate to
[reach out] to the Crossplane maintainers and community for help designing and
implementing support for new managed services. [#sig-services] would highly
value any feedback you may have about the services development process!

[What Makes a Crossplane Managed Service]: #what-makes-a-crossplane-managed-service
[managed resource]: concepts.md#managed-resource
[resource claim]: concepts.md#resource-claim
[resource class]: concepts.md#resource-class
[dynamic provisioning]: concepts.md#dynamic-and-static-provisioning
[`CloudMemorystoreInstance`]: https://github.com/crossplaneio/stack-gcp/blob/42ebb8b71/gcp/apis/cache/v1alpha2/cloudmemorystore_instance_types.go#L146
[`CloudMemorystoreInstanceClass`]: https://github.com/crossplaneio/stack-gcp/blob/42ebb8b71/gcp/apis/cache/v1alpha2/cloudmemorystore_instance_types.go#L237
[`Provider`]: https://github.com/crossplaneio/stack-gcp/blob/24ab7381b/gcp/apis/v1alpha2/types.go#L37
[`RedisCluster`]: https://github.com/crossplaneio/crossplane/blob/3c6cf4e/apis/cache/v1alpha1/rediscluster_types.go#L40
[`RedisClusterClass`]: https://github.com/crossplaneio/crossplane/blob/3c6cf4e/apis/cache/v1alpha1/rediscluster_types.go#L116
[watching the API server]: https://kubernetes.io/docs/reference/using-api/api-concepts/#efficient-detection-of-changes
[kubebuilder]: https://kubebuilder.io/
[controller-runtime]: https://github.com/kubernetes-sigs/controller-runtime
[crossplane-runtime]: https://github.com/crossplaneio/crossplane-runtime/
[golden path]: https://charity.wtf/2018/12/02/software-sprawl-the-golden-path-and-scaling-teams-with-agency/
[API Conventions]: https://github.com/kubernetes/community/blob/c6e1e89a/contributors/devel/sig-architecture/api-conventions.md
[kubebuilder book]: https://book.kubebuilder.io/
[Stacks quick start]: https://github.com/crossplaneio/crossplane-cli/blob/357d18e7b/README.md#quick-start-stacks
[resources]: https://kubebuilder.io/cronjob-tutorial/gvks.html#kinds-and-resources
[kinds]: https://kubebuilder.io/cronjob-tutorial/gvks.html#kinds-and-resources
[objects]: https://kubernetes.io/docs/concepts/#kubernetes-objects
[comment marker]: https://kubebuilder.io/reference/markers.html
[comment markers]: https://kubebuilder.io/reference/markers.html
[`resource.Managed`]: https://godoc.org/github.com/crossplaneio/crossplane-runtime/pkg/resource#Managed
[`resource.Claim`]: https://godoc.org/github.com/crossplaneio/crossplane-runtime/pkg/resource#Claim
[`resource.PortableClass`]: https://godoc.org/github.com/crossplaneio/crossplane-runtime/pkg/resource#PortableClass
[`resource.PortableClassList`]: https://godoc.org/github.com/crossplaneio/crossplane-runtime/pkg/resource#PortableClassList
[`resource.NonPortableClass`]: https://godoc.org/github.com/crossplaneio/crossplane-runtime/pkg/resource#NonPortableClass
[`resource.ManagedReconciler`]: https://godoc.org/github.com/crossplaneio/crossplane-runtime/pkg/resource#ManagedReconciler
[`resource.NewManagedReconciler`]: https://godoc.org/github.com/crossplaneio/crossplane-runtime/pkg/resource#NewManagedReconciler
[`resource.ClaimReconciler`]: https://godoc.org/github.com/crossplaneio/crossplane-runtime/pkg/resource#ClaimReconciler
[`resource.NewClaimReconciler`]: https://godoc.org/github.com/crossplaneio/crossplane-runtime/pkg/resource#NewClaimReconciler
[`resource.DefaultClassReconciler`]: https://godoc.org/github.com/crossplaneio/crossplane-runtime/pkg/resource#DefaultClassReconciler
[`resource.NewDefaultClassReconciler`]: https://godoc.org/github.com/crossplaneio/crossplane-runtime/pkg/resource#NewDefaultClassReconciler
[`resource.ExternalConnecter`]: https://godoc.org/github.com/crossplaneio/crossplane-runtime/pkg/resource#ExternalConnecter
[`resource.ExternalClient`]: https://godoc.org/github.com/crossplaneio/crossplane-runtime/pkg/resource#ExternalClient
[`resource.ManagedConfigurator`]: https://godoc.org/github.com/crossplaneio/crossplane-runtime/pkg/resource#ManagedConfigurator
[`ResourceSpec`]: https://godoc.org/github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1#ResourceSpec
[`ResourceStatus`]: https://godoc.org/github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1#ResourceStatus
[`ResourceClaimSpec`]: https://godoc.org/github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1#ResourceClaimSpec
[`ResourceClaimStatus`]: https://godoc.org/github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1#ResourceClaimStatus
[`NonPortableClassSpecTemplate`]: https://godoc.org/github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1#NonPortableClassSpecTemplate
[`PortableClass`]: https://godoc.org/github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1#PortableClass
['resource.ExternalConnecter`]: https://godoc.org/github.com/crossplaneio/crossplane-runtime/pkg/resource#ExternalConnecter
[opening a Crossplane issue]: https://github.com/crossplaneio/crossplane/issues/new/choose
[`GroupVersionKind`]: https://godoc.org/k8s.io/apimachinery/pkg/runtime/schema#GroupVersionKind
[`reconcile.Reconciler`]: https://godoc.org/sigs.k8s.io/controller-runtime/pkg/reconcile#Reconciler
[favor]: https://github.com/crossplaneio/crossplane/issues/452
[reach out]: https://github.com/crossplaneio/crossplane#contact
[#sig-services]: https://crossplane.slack.com/messages/sig-services
[crossplaneio org]: https://github.com/crossplaneio
