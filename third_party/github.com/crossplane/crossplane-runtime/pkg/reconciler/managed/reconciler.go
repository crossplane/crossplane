/*
Copyright 2019 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package managed

import (
	"context"
	"strings"
	"time"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/feature"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

const (
	// FinalizerName is the string that is used as finalizer on managed resource
	// objects.
	FinalizerName = "finalizer.managedresource.crossplane.io"

	reconcileGracePeriod = 30 * time.Second
	reconcileTimeout     = 1 * time.Minute

	defaultpollInterval = 1 * time.Minute
	defaultGracePeriod  = 30 * time.Second
)

// Error strings.
const (
	errFmtManagementPolicyNonDefault   = "`spec.managementPolicies` is set to a non-default value but the feature is not enabled: %s"
	errFmtManagementPolicyNotSupported = "`spec.managementPolicies` is set to a value(%s) which is not supported. Check docs for supported policies"

	errGetManaged               = "cannot get managed resource"
	errUpdateManagedAnnotations = "cannot update managed resource annotations"
	errCreateIncomplete         = "cannot determine creation result - remove the " + meta.AnnotationKeyExternalCreatePending + " annotation if it is safe to proceed"
	errReconcileConnect         = "connect failed"
	errReconcileObserve         = "observe failed"
	errReconcileCreate          = "create failed"
	errReconcileUpdate          = "update failed"
	errReconcileDelete          = "delete failed"

	errExternalResourceNotExist = "external resource does not exist"
)

// Event reasons.
const (
	reasonCannotConnect           event.Reason = "CannotConnectToProvider"
	reasonCannotDisconnect        event.Reason = "CannotDisconnectFromProvider"
	reasonCannotInitialize        event.Reason = "CannotInitializeManagedResource"
	reasonCannotResolveRefs       event.Reason = "CannotResolveResourceReferences"
	reasonCannotObserve           event.Reason = "CannotObserveExternalResource"
	reasonCannotCreate            event.Reason = "CannotCreateExternalResource"
	reasonCannotDelete            event.Reason = "CannotDeleteExternalResource"
	reasonCannotPublish           event.Reason = "CannotPublishConnectionDetails"
	reasonCannotUnpublish         event.Reason = "CannotUnpublishConnectionDetails"
	reasonCannotUpdate            event.Reason = "CannotUpdateExternalResource"
	reasonCannotUpdateManaged     event.Reason = "CannotUpdateManagedResource"
	reasonManagementPolicyInvalid event.Reason = "CannotUseInvalidManagementPolicy"

	reasonDeleted event.Reason = "DeletedExternalResource"
	reasonCreated event.Reason = "CreatedExternalResource"
	reasonUpdated event.Reason = "UpdatedExternalResource"
	reasonPending event.Reason = "PendingExternalResource"

	reasonReconciliationPaused event.Reason = "ReconciliationPaused"
)

// ControllerName returns the recommended name for controllers that use this
// package to reconcile a particular kind of managed resource.
func ControllerName(kind string) string {
	return "managed/" + strings.ToLower(kind)
}

// ManagementPoliciesChecker is used to perform checks on management policies
// to determine specific actions are allowed, or if they are the only allowed
// action.
type ManagementPoliciesChecker interface {
	// Validate validates the management policies.
	Validate() error
	// IsPaused returns true if the resource is paused based
	// on the management policy.
	IsPaused() bool

	// ShouldOnlyObserve returns true if only the Observe action is allowed.
	ShouldOnlyObserve() bool
	// ShouldCreate returns true if the Create action is allowed.
	ShouldCreate() bool
	// ShouldLateInitialize returns true if the LateInitialize action is
	// allowed.
	ShouldLateInitialize() bool
	// ShouldUpdate returns true if the Update action is allowed.
	ShouldUpdate() bool
	// ShouldDelete returns true if the Delete action is allowed.
	ShouldDelete() bool
}

// A CriticalAnnotationUpdater is used when it is critical that annotations must
// be updated before returning from the Reconcile loop.
type CriticalAnnotationUpdater interface {
	UpdateCriticalAnnotations(ctx context.Context, o client.Object) error
}

// A CriticalAnnotationUpdateFn may be used when it is critical that annotations
// must be updated before returning from the Reconcile loop.
type CriticalAnnotationUpdateFn func(ctx context.Context, o client.Object) error

// UpdateCriticalAnnotations of the supplied object.
func (fn CriticalAnnotationUpdateFn) UpdateCriticalAnnotations(ctx context.Context, o client.Object) error {
	return fn(ctx, o)
}

// ConnectionDetails created or updated during an operation on an external
// resource, for example usernames, passwords, endpoints, ports, etc.
type ConnectionDetails map[string][]byte

// A ConnectionPublisher manages the supplied ConnectionDetails for the
// supplied Managed resource. ManagedPublishers must handle the case in which
// the supplied ConnectionDetails are empty.
type ConnectionPublisher interface {
	// PublishConnection details for the supplied Managed resource. Publishing
	// must be additive; i.e. if details (a, b, c) are published, subsequently
	// publicing details (b, c, d) should update (b, c) but not remove a.
	PublishConnection(ctx context.Context, so resource.ConnectionSecretOwner, c ConnectionDetails) (published bool, err error)

	// UnpublishConnection details for the supplied Managed resource.
	UnpublishConnection(ctx context.Context, so resource.ConnectionSecretOwner, c ConnectionDetails) error
}

// ConnectionPublisherFns is the pluggable struct to produce objects with ConnectionPublisher interface.
type ConnectionPublisherFns struct {
	PublishConnectionFn   func(ctx context.Context, o resource.ConnectionSecretOwner, c ConnectionDetails) (bool, error)
	UnpublishConnectionFn func(ctx context.Context, o resource.ConnectionSecretOwner, c ConnectionDetails) error
}

// PublishConnection details for the supplied Managed resource.
func (fn ConnectionPublisherFns) PublishConnection(ctx context.Context, o resource.ConnectionSecretOwner, c ConnectionDetails) (bool, error) {
	return fn.PublishConnectionFn(ctx, o, c)
}

// UnpublishConnection details for the supplied Managed resource.
func (fn ConnectionPublisherFns) UnpublishConnection(ctx context.Context, o resource.ConnectionSecretOwner, c ConnectionDetails) error {
	return fn.UnpublishConnectionFn(ctx, o, c)
}

// A ConnectionDetailsFetcher fetches connection details for the supplied
// Connection Secret owner.
type ConnectionDetailsFetcher interface {
	FetchConnection(ctx context.Context, so resource.ConnectionSecretOwner) (ConnectionDetails, error)
}

// A Initializer establishes ownership of the supplied Managed resource.
// This typically involves the operations that are run before calling any
// ExternalClient methods.
type Initializer interface {
	Initialize(ctx context.Context, mg resource.Managed) error
}

// A InitializerChain chains multiple managed initializers.
type InitializerChain []Initializer

// Initialize calls each Initializer serially. It returns the first
// error it encounters, if any.
func (cc InitializerChain) Initialize(ctx context.Context, mg resource.Managed) error {
	for _, c := range cc {
		if err := c.Initialize(ctx, mg); err != nil {
			return err
		}
	}
	return nil
}

// A InitializerFn is a function that satisfies the Initializer
// interface.
type InitializerFn func(ctx context.Context, mg resource.Managed) error

// Initialize calls InitializerFn function.
func (m InitializerFn) Initialize(ctx context.Context, mg resource.Managed) error {
	return m(ctx, mg)
}

// A ReferenceResolver resolves references to other managed resources.
type ReferenceResolver interface {
	// ResolveReferences resolves all fields in the supplied managed resource
	// that are references to other managed resources by updating corresponding
	// fields, for example setting spec.network to the Network resource
	// specified by spec.networkRef.name.
	ResolveReferences(ctx context.Context, mg resource.Managed) error
}

// A ReferenceResolverFn is a function that satisfies the
// ReferenceResolver interface.
type ReferenceResolverFn func(context.Context, resource.Managed) error

// ResolveReferences calls ReferenceResolverFn function
func (m ReferenceResolverFn) ResolveReferences(ctx context.Context, mg resource.Managed) error {
	return m(ctx, mg)
}

// An ExternalConnecter produces a new ExternalClient given the supplied
// Managed resource.
type ExternalConnecter interface {
	// Connect to the provider specified by the supplied managed resource and
	// produce an ExternalClient.
	Connect(ctx context.Context, mg resource.Managed) (ExternalClient, error)
}

// An ExternalDisconnecter disconnects from a provider.
type ExternalDisconnecter interface {
	// Disconnect from the provider and close the ExternalClient.
	Disconnect(ctx context.Context) error
}

// A NopDisconnecter converts an ExternalConnecter into an
// ExternalConnectDisconnecter with a no-op Disconnect method.
type NopDisconnecter struct {
	c ExternalConnecter
}

// Connect calls the underlying ExternalConnecter's Connect method.
func (c *NopDisconnecter) Connect(ctx context.Context, mg resource.Managed) (ExternalClient, error) {
	return c.c.Connect(ctx, mg)
}

// Disconnect does nothing. It never returns an error.
func (c *NopDisconnecter) Disconnect(_ context.Context) error {
	return nil
}

// NewNopDisconnecter converts an ExternalConnecter into an
// ExternalConnectDisconnecter with a no-op Disconnect method.
func NewNopDisconnecter(c ExternalConnecter) ExternalConnectDisconnecter {
	return &NopDisconnecter{c}
}

// An ExternalConnectDisconnecter produces a new ExternalClient given the supplied
// Managed resource.
type ExternalConnectDisconnecter interface {
	ExternalConnecter
	ExternalDisconnecter
}

// An ExternalConnectorFn is a function that satisfies the ExternalConnecter
// interface.
type ExternalConnectorFn func(ctx context.Context, mg resource.Managed) (ExternalClient, error)

// Connect to the provider specified by the supplied managed resource and
// produce an ExternalClient.
func (ec ExternalConnectorFn) Connect(ctx context.Context, mg resource.Managed) (ExternalClient, error) {
	return ec(ctx, mg)
}

// An ExternalDisconnectorFn is a function that satisfies the ExternalConnecter
// interface.
type ExternalDisconnectorFn func(ctx context.Context) error

// Disconnect from provider and close the ExternalClient.
func (ed ExternalDisconnectorFn) Disconnect(ctx context.Context) error {
	return ed(ctx)
}

// ExternalConnectDisconnecterFns are functions that satisfy the
// ExternalConnectDisconnecter interface.
type ExternalConnectDisconnecterFns struct {
	ConnectFn    func(ctx context.Context, mg resource.Managed) (ExternalClient, error)
	DisconnectFn func(ctx context.Context) error
}

// Connect to the provider specified by the supplied managed resource and
// produce an ExternalClient.
func (fns ExternalConnectDisconnecterFns) Connect(ctx context.Context, mg resource.Managed) (ExternalClient, error) {
	return fns.ConnectFn(ctx, mg)
}

// Disconnect from the provider and close the ExternalClient.
func (fns ExternalConnectDisconnecterFns) Disconnect(ctx context.Context) error {
	return fns.DisconnectFn(ctx)
}

// An ExternalClient manages the lifecycle of an external resource.
// None of the calls here should be blocking. All of the calls should be
// idempotent. For example, Create call should not return AlreadyExists error
// if it's called again with the same parameters or Delete call should not
// return error if there is an ongoing deletion or resource does not exist.
type ExternalClient interface {
	// Observe the external resource the supplied Managed resource
	// represents, if any. Observe implementations must not modify the
	// external resource, but may update the supplied Managed resource to
	// reflect the state of the external resource. Status modifications are
	// automatically persisted unless ResourceLateInitialized is true - see
	// ResourceLateInitialized for more detail.
	Observe(ctx context.Context, mg resource.Managed) (ExternalObservation, error)

	// Create an external resource per the specifications of the supplied
	// Managed resource. Called when Observe reports that the associated
	// external resource does not exist. Create implementations may update
	// managed resource annotations, and those updates will be persisted.
	// All other updates will be discarded.
	Create(ctx context.Context, mg resource.Managed) (ExternalCreation, error)

	// Update the external resource represented by the supplied Managed
	// resource, if necessary. Called unless Observe reports that the
	// associated external resource is up to date.
	Update(ctx context.Context, mg resource.Managed) (ExternalUpdate, error)

	// Delete the external resource upon deletion of its associated Managed
	// resource. Called when the managed resource has been deleted.
	Delete(ctx context.Context, mg resource.Managed) error
}

// ExternalClientFns are a series of functions that satisfy the ExternalClient
// interface.
type ExternalClientFns struct {
	ObserveFn func(ctx context.Context, mg resource.Managed) (ExternalObservation, error)
	CreateFn  func(ctx context.Context, mg resource.Managed) (ExternalCreation, error)
	UpdateFn  func(ctx context.Context, mg resource.Managed) (ExternalUpdate, error)
	DeleteFn  func(ctx context.Context, mg resource.Managed) error
}

// Observe the external resource the supplied Managed resource represents, if
// any.
func (e ExternalClientFns) Observe(ctx context.Context, mg resource.Managed) (ExternalObservation, error) {
	return e.ObserveFn(ctx, mg)
}

// Create an external resource per the specifications of the supplied Managed
// resource.
func (e ExternalClientFns) Create(ctx context.Context, mg resource.Managed) (ExternalCreation, error) {
	return e.CreateFn(ctx, mg)
}

// Update the external resource represented by the supplied Managed resource, if
// necessary.
func (e ExternalClientFns) Update(ctx context.Context, mg resource.Managed) (ExternalUpdate, error) {
	return e.UpdateFn(ctx, mg)
}

// Delete the external resource upon deletion of its associated Managed
// resource.
func (e ExternalClientFns) Delete(ctx context.Context, mg resource.Managed) error {
	return e.DeleteFn(ctx, mg)
}

// A NopConnecter does nothing.
type NopConnecter struct{}

// Connect returns a NopClient. It never returns an error.
func (c *NopConnecter) Connect(_ context.Context, _ resource.Managed) (ExternalClient, error) {
	return &NopClient{}, nil
}

// A NopClient does nothing.
type NopClient struct{}

// Observe does nothing. It returns an empty ExternalObservation and no error.
func (c *NopClient) Observe(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
	return ExternalObservation{}, nil
}

// Create does nothing. It returns an empty ExternalCreation and no error.
func (c *NopClient) Create(_ context.Context, _ resource.Managed) (ExternalCreation, error) {
	return ExternalCreation{}, nil
}

// Update does nothing. It returns an empty ExternalUpdate and no error.
func (c *NopClient) Update(_ context.Context, _ resource.Managed) (ExternalUpdate, error) {
	return ExternalUpdate{}, nil
}

// Delete does nothing. It never returns an error.
func (c *NopClient) Delete(_ context.Context, _ resource.Managed) error { return nil }

// An ExternalObservation is the result of an observation of an external
// resource.
type ExternalObservation struct {
	// ResourceExists must be true if a corresponding external resource exists
	// for the managed resource. Typically this is proven by the presence of an
	// external resource of the expected kind whose unique identifier matches
	// the managed resource's external name. Crossplane uses this information to
	// determine whether it needs to create or delete the external resource.
	ResourceExists bool

	// ResourceUpToDate should be true if the corresponding external resource
	// appears to be up-to-date - i.e. updating the external resource to match
	// the desired state of the managed resource would be a no-op. Keep in mind
	// that often only a subset of external resource fields can be updated.
	// Crossplane uses this information to determine whether it needs to update
	// the external resource.
	ResourceUpToDate bool

	// ResourceLateInitialized should be true if the managed resource's spec was
	// updated during its observation. A Crossplane provider may update a
	// managed resource's spec fields after it is created or updated, as long as
	// the updates are limited to setting previously unset fields, and adding
	// keys to maps. Crossplane uses this information to determine whether
	// changes to the spec were made during observation that must be persisted.
	// Note that changes to the spec will be persisted before changes to the
	// status, and that pending changes to the status may be lost when the spec
	// is persisted. Status changes will be persisted by the first subsequent
	// observation that _does not_ late initialize the managed resource, so it
	// is important that Observe implementations do not late initialize the
	// resource every time they are called.
	ResourceLateInitialized bool

	// ConnectionDetails required to connect to this resource. These details
	// are a set that is collated throughout the managed resource's lifecycle -
	// i.e. returning new connection details will have no affect on old details
	// unless an existing key is overwritten. Crossplane may publish these
	// credentials to a store (e.g. a Secret).
	ConnectionDetails ConnectionDetails

	// Diff is a Debug level message that is sent to the reconciler when
	// there is a change in the observed Managed Resource. It is useful for
	// finding where the observed diverges from the desired state.
	// The string should be a cmp.Diff that details the difference.
	Diff string
}

// An ExternalCreation is the result of the creation of an external resource.
type ExternalCreation struct {
	// ConnectionDetails required to connect to this resource. These details
	// are a set that is collated throughout the managed resource's lifecycle -
	// i.e. returning new connection details will have no affect on old details
	// unless an existing key is overwritten. Crossplane may publish these
	// credentials to a store (e.g. a Secret).
	ConnectionDetails ConnectionDetails
}

// An ExternalUpdate is the result of an update to an external resource.
type ExternalUpdate struct {
	// ConnectionDetails required to connect to this resource. These details
	// are a set that is collated throughout the managed resource's lifecycle -
	// i.e. returning new connection details will have no affect on old details
	// unless an existing key is overwritten. Crossplane may publish these
	// credentials to a store (e.g. a Secret).
	ConnectionDetails ConnectionDetails
}

// A Reconciler reconciles managed resources by creating and managing the
// lifecycle of an external resource, i.e. a resource in an external system such
// as a cloud provider API. Each controller must watch the managed resource kind
// for which it is responsible.
type Reconciler struct {
	client     client.Client
	newManaged func() resource.Managed

	pollInterval        time.Duration
	timeout             time.Duration
	creationGracePeriod time.Duration

	features feature.Flags

	// The below structs embed the set of interfaces used to implement the
	// managed resource reconciler. We do this primarily for readability, so
	// that the reconciler logic reads r.external.Connect(),
	// r.managed.Delete(), etc.
	external mrExternal
	managed  mrManaged

	supportedManagementPolicies []sets.Set[xpv1.ManagementAction]

	log    logging.Logger
	record event.Recorder
}

type mrManaged struct {
	CriticalAnnotationUpdater
	ConnectionPublisher
	resource.Finalizer
	Initializer
	ReferenceResolver
}

func defaultMRManaged(m manager.Manager) mrManaged {
	return mrManaged{
		CriticalAnnotationUpdater: NewRetryingCriticalAnnotationUpdater(m.GetClient()),
		Finalizer:                 resource.NewAPIFinalizer(m.GetClient(), FinalizerName),
		Initializer:               NewNameAsExternalName(m.GetClient()),
		ReferenceResolver:         NewAPISimpleReferenceResolver(m.GetClient()),
		ConnectionPublisher: PublisherChain([]ConnectionPublisher{
			NewAPISecretPublisher(m.GetClient(), m.GetScheme()),
			&DisabledSecretStoreManager{},
		}),
	}
}

type mrExternal struct {
	ExternalConnectDisconnecter
}

func defaultMRExternal() mrExternal {
	return mrExternal{
		ExternalConnectDisconnecter: NewNopDisconnecter(&NopConnecter{}),
	}
}

// A ReconcilerOption configures a Reconciler.
type ReconcilerOption func(*Reconciler)

// WithTimeout specifies the timeout duration cumulatively for all the calls happen
// in the reconciliation function. In case the deadline exceeds, reconciler will
// still have some time to make the necessary calls to report the error such as
// status update.
func WithTimeout(duration time.Duration) ReconcilerOption {
	return func(r *Reconciler) {
		r.timeout = duration
	}
}

// WithPollInterval specifies how long the Reconciler should wait before queueing
// a new reconciliation after a successful reconcile. The Reconciler requeues
// after a specified duration when it is not actively waiting for an external
// operation, but wishes to check whether an existing external resource needs to
// be synced to its Crossplane Managed resource.
func WithPollInterval(after time.Duration) ReconcilerOption {
	return func(r *Reconciler) {
		r.pollInterval = after
	}
}

// WithCreationGracePeriod configures an optional period during which we will
// wait for the external API to report that a newly created external resource
// exists. This allows us to tolerate eventually consistent APIs that do not
// immediately report that newly created resources exist when queried. All
// resources have a 30 second grace period by default.
func WithCreationGracePeriod(d time.Duration) ReconcilerOption {
	return func(r *Reconciler) {
		r.creationGracePeriod = d
	}
}

// WithExternalConnecter specifies how the Reconciler should connect to the API
// used to sync and delete external resources.
func WithExternalConnecter(c ExternalConnecter) ReconcilerOption {
	return func(r *Reconciler) {
		r.external.ExternalConnectDisconnecter = NewNopDisconnecter(c)
	}
}

// WithExternalConnectDisconnecter specifies how the Reconciler should connect and disconnect to the API
// used to sync and delete external resources.
func WithExternalConnectDisconnecter(c ExternalConnectDisconnecter) ReconcilerOption {
	return func(r *Reconciler) {
		r.external.ExternalConnectDisconnecter = c
	}
}

// WithCriticalAnnotationUpdater specifies how the Reconciler should update a
// managed resource's critical annotations. Implementations typically contain
// some kind of retry logic to increase the likelihood that critical annotations
// (like non-deterministic external names) will be persisted.
func WithCriticalAnnotationUpdater(u CriticalAnnotationUpdater) ReconcilerOption {
	return func(r *Reconciler) {
		r.managed.CriticalAnnotationUpdater = u
	}
}

// WithConnectionPublishers specifies how the Reconciler should publish
// its connection details such as credentials and endpoints.
func WithConnectionPublishers(p ...ConnectionPublisher) ReconcilerOption {
	return func(r *Reconciler) {
		r.managed.ConnectionPublisher = PublisherChain(p)
	}
}

// WithInitializers specifies how the Reconciler should initialize a
// managed resource before calling any of the ExternalClient functions.
func WithInitializers(i ...Initializer) ReconcilerOption {
	return func(r *Reconciler) {
		r.managed.Initializer = InitializerChain(i)
	}
}

// WithFinalizer specifies how the Reconciler should add and remove
// finalizers to and from the managed resource.
func WithFinalizer(f resource.Finalizer) ReconcilerOption {
	return func(r *Reconciler) {
		r.managed.Finalizer = f
	}
}

// WithReferenceResolver specifies how the Reconciler should resolve any
// inter-resource references it encounters while reconciling managed resources.
func WithReferenceResolver(rr ReferenceResolver) ReconcilerOption {
	return func(r *Reconciler) {
		r.managed.ReferenceResolver = rr
	}
}

// WithLogger specifies how the Reconciler should log messages.
func WithLogger(l logging.Logger) ReconcilerOption {
	return func(r *Reconciler) {
		r.log = l
	}
}

// WithRecorder specifies how the Reconciler should record events.
func WithRecorder(er event.Recorder) ReconcilerOption {
	return func(r *Reconciler) {
		r.record = er
	}
}

// WithManagementPolicies enables support for management policies.
func WithManagementPolicies() ReconcilerOption {
	return func(r *Reconciler) {
		r.features.Enable(feature.EnableAlphaManagementPolicies)
	}
}

// WithReconcilerSupportedManagementPolicies configures which management policies are
// supported by the reconciler.
func WithReconcilerSupportedManagementPolicies(supported []sets.Set[xpv1.ManagementAction]) ReconcilerOption {
	return func(r *Reconciler) {
		r.supportedManagementPolicies = supported
	}
}

// NewReconciler returns a Reconciler that reconciles managed resources of the
// supplied ManagedKind with resources in an external system such as a cloud
// provider API. It panics if asked to reconcile a managed resource kind that is
// not registered with the supplied manager's runtime.Scheme. The returned
// Reconciler reconciles with a dummy, no-op 'external system' by default;
// callers should supply an ExternalConnector that returns an ExternalClient
// capable of managing resources in a real system.
func NewReconciler(m manager.Manager, of resource.ManagedKind, o ...ReconcilerOption) *Reconciler {
	nm := func() resource.Managed {
		return resource.MustCreateObject(schema.GroupVersionKind(of), m.GetScheme()).(resource.Managed)
	}

	// Panic early if we've been asked to reconcile a resource kind that has not
	// been registered with our controller manager's scheme.
	_ = nm()

	r := &Reconciler{
		client:                      m.GetClient(),
		newManaged:                  nm,
		pollInterval:                defaultpollInterval,
		creationGracePeriod:         defaultGracePeriod,
		timeout:                     reconcileTimeout,
		managed:                     defaultMRManaged(m),
		external:                    defaultMRExternal(),
		supportedManagementPolicies: defaultSupportedManagementPolicies(),
		log:                         logging.NewNopLogger(),
		record:                      event.NewNopRecorder(),
	}

	for _, ro := range o {
		ro(r)
	}

	return r
}

// Reconcile a managed resource with an external resource.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (result reconcile.Result, err error) { //nolint:gocyclo // See note below.
	// NOTE(negz): This method is a well over our cyclomatic complexity goal.
	// Be wary of adding additional complexity.

	log := r.log.WithValues("request", req)

	defer func() {
		if kerrors.IsConflict(errors.Cause(err)) {
			// conflict errors are transient in Kubernetes and completely normal.
			// The right reaction is to requeue, but not record the object as error'ing
			// or creating events.
			log.Debug("Transient conflict error", "error", err)
			result.Requeue = true
			err = nil
		}
	}()

	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(ctx, r.timeout+reconcileGracePeriod)
	defer cancel()

	// Govet linter has a check for lost cancel funcs but it's a false positive
	// for child contexts as because parent's cancel is called, so we skip it
	// for this line.
	externalCtx, _ := context.WithTimeout(ctx, r.timeout) //nolint:govet // See note above.

	managed := r.newManaged()
	if err := r.client.Get(ctx, req.NamespacedName, managed); err != nil {
		// There's no need to requeue if we no longer exist. Otherwise we'll be
		// requeued implicitly because we return an error.
		log.Debug("Cannot get managed resource", "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGetManaged)
	}

	record := r.record.WithAnnotations("external-name", meta.GetExternalName(managed))
	log = log.WithValues(
		"uid", managed.GetUID(),
		"version", managed.GetResourceVersion(),
		"external-name", meta.GetExternalName(managed),
	)

	managementPoliciesEnabled := r.features.Enabled(feature.EnableAlphaManagementPolicies)
	if managementPoliciesEnabled {
		log.WithValues("managementPolicies", managed.GetManagementPolicies())
	}

	// Create the management policy resolver which will assist us in determining
	// what actions to take on the managed resource based on the management
	// and deletion policies.
	policy := NewManagementPoliciesResolver(managementPoliciesEnabled, managed.GetManagementPolicies(), managed.GetDeletionPolicy(), WithSupportedManagementPolicies(r.supportedManagementPolicies))

	// Check if the resource has paused reconciliation based on the
	// annotation or the management policies.
	// Log, publish an event and update the SYNC status condition.
	if meta.IsPaused(managed) || policy.IsPaused() {
		log.Debug("Reconciliation is paused either through the `spec.managementPolicies` or the pause annotation", "annotation", meta.AnnotationKeyReconciliationPaused)
		record.Event(managed, event.Normal(reasonReconciliationPaused, "Reconciliation is paused either through the `spec.managementPolicies` or the pause annotation",
			"annotation", meta.AnnotationKeyReconciliationPaused))
		managed.SetConditions(xpv1.ReconcilePaused())
		// if the pause annotation is removed or the management policies changed, we will have a chance to reconcile
		// again and resume and if status update fails, we will reconcile again to retry to update the status
		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
	}

	// Check if the ManagementPolicies is set to a non-default value while the
	// feature is not enabled. This is a safety check to let users know that
	// they need to enable the feature flag before using the feature. For
	// example, we wouldn't want someone to set the policy to ObserveOnly but
	// not realize that the controller is still trying to reconcile
	// (and modify or delete) the resource since they forgot to enable the
	// feature flag. Also checks if the management policy is set to a value
	// that is not supported by the controller.
	if err := policy.Validate(); err != nil {
		log.Debug(err.Error())
		record.Event(managed, event.Warning(reasonManagementPolicyInvalid, err))
		managed.SetConditions(xpv1.ReconcileError(err))
		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
	}

	// If managed resource has a deletion timestamp and a deletion policy of
	// Orphan, we do not need to observe the external resource before attempting
	// to unpublish connection details and remove finalizer.
	if meta.WasDeleted(managed) && !policy.ShouldDelete() {
		log = log.WithValues("deletion-timestamp", managed.GetDeletionTimestamp())

		// Empty ConnectionDetails are passed to UnpublishConnection because we
		// have not retrieved them from the external resource. In practice we
		// currently only write connection details to a Secret, and we rely on
		// garbage collection to delete the entire secret, regardless of the
		// supplied connection details.
		if err := r.managed.UnpublishConnection(ctx, managed, ConnectionDetails{}); err != nil {
			// If this is the first time we encounter this issue we'll be
			// requeued implicitly when we update our status with the new error
			// condition. If not, we requeue explicitly, which will trigger
			// backoff.
			log.Debug("Cannot unpublish connection details", "error", err)
			record.Event(managed, event.Warning(reasonCannotUnpublish, err))
			managed.SetConditions(xpv1.Deleting(), xpv1.ReconcileError(err))
			return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
		}
		if err := r.managed.RemoveFinalizer(ctx, managed); err != nil {
			// If this is the first time we encounter this issue we'll be
			// requeued implicitly when we update our status with the new error
			// condition. If not, we requeue explicitly, which will trigger
			// backoff.
			log.Debug("Cannot remove managed resource finalizer", "error", err)
			managed.SetConditions(xpv1.Deleting(), xpv1.ReconcileError(err))
			return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
		}

		// We've successfully unpublished our managed resource's connection
		// details and removed our finalizer. If we assume we were the only
		// controller that added a finalizer to this resource then it should no
		// longer exist and thus there is no point trying to update its status.
		log.Debug("Successfully deleted managed resource")
		return reconcile.Result{Requeue: false}, nil
	}

	if err := r.managed.Initialize(ctx, managed); err != nil {
		// If this is the first time we encounter this issue we'll be requeued
		// implicitly when we update our status with the new error condition. If
		// not, we requeue explicitly, which will trigger backoff.
		log.Debug("Cannot initialize managed resource", "error", err)
		record.Event(managed, event.Warning(reasonCannotInitialize, err))
		managed.SetConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
	}

	// If we started but never completed creation of an external resource we
	// may have lost critical information. For example if we didn't persist
	// an updated external name we've leaked a resource. The safest thing to
	// do is to refuse to proceed.
	if meta.ExternalCreateIncomplete(managed) {
		log.Debug(errCreateIncomplete)
		record.Event(managed, event.Warning(reasonCannotInitialize, errors.New(errCreateIncomplete)))
		managed.SetConditions(xpv1.Creating(), xpv1.ReconcileError(errors.New(errCreateIncomplete)))
		return reconcile.Result{Requeue: false}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
	}

	// We resolve any references before observing our external resource because
	// in some rare examples we need a spec field to make the observe call, and
	// that spec field could be set by a reference.
	//
	// We do not resolve references when being deleted because it is likely that
	// the resources we reference are also being deleted, and would thus block
	// resolution due to being unready or non-existent. It is unlikely (but not
	// impossible) that we need to resolve a reference in order to process a
	// delete, and that reference is stale at delete time.
	if !meta.WasDeleted(managed) {
		if err := r.managed.ResolveReferences(ctx, managed); err != nil {
			// If any of our referenced resources are not yet ready (or if we
			// encountered an error resolving them) we want to try again. If
			// this is the first time we encounter this situation we'll be
			// requeued implicitly due to the status update. If not, we want
			// requeue explicitly, which will trigger backoff.
			log.Debug("Cannot resolve managed resource references", "error", err)
			record.Event(managed, event.Warning(reasonCannotResolveRefs, err))
			managed.SetConditions(xpv1.ReconcileError(err))
			return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
		}
	}

	external, err := r.external.Connect(externalCtx, managed)
	if err != nil {
		// We'll usually hit this case if our Provider or its secret are missing
		// or invalid. If this is first time we encounter this issue we'll be
		// requeued implicitly when we update our status with the new error
		// condition. If not, we requeue explicitly, which will trigger
		// backoff.
		log.Debug("Cannot connect to provider", "error", err)
		record.Event(managed, event.Warning(reasonCannotConnect, err))
		managed.SetConditions(xpv1.ReconcileError(errors.Wrap(err, errReconcileConnect)))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
	}
	defer func() {
		if err := r.external.Disconnect(ctx); err != nil {
			log.Debug("Cannot disconnect from provider", "error", err)
			record.Event(managed, event.Warning(reasonCannotDisconnect, err))
		}
	}()

	observation, err := external.Observe(externalCtx, managed)
	if err != nil {
		// We'll usually hit this case if our Provider credentials are invalid
		// or insufficient for observing the external resource type we're
		// concerned with. If this is the first time we encounter this issue
		// we'll be requeued implicitly when we update our status with the new
		// error condition. If not, we requeue explicitly, which will
		// trigger backoff.
		log.Debug("Cannot observe external resource", "error", err)
		record.Event(managed, event.Warning(reasonCannotObserve, err))
		managed.SetConditions(xpv1.ReconcileError(errors.Wrap(err, errReconcileObserve)))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
	}

	// In the observe-only mode, !observation.ResourceExists will be an error
	// case, and we will explicitly return this information to the user.
	if !observation.ResourceExists && policy.ShouldOnlyObserve() {
		record.Event(managed, event.Warning(reasonCannotObserve, errors.New(errExternalResourceNotExist)))
		managed.SetConditions(xpv1.ReconcileError(errors.Wrap(errors.New(errExternalResourceNotExist), errReconcileObserve)))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
	}

	// If this resource has a non-zero creation grace period we want to wait
	// for that period to expire before we trust that the resource really
	// doesn't exist. This is because some external APIs are eventually
	// consistent and may report that a recently created resource does not
	// exist.
	if !observation.ResourceExists && meta.ExternalCreateSucceededDuring(managed, r.creationGracePeriod) {
		log.Debug("Waiting for external resource existence to be confirmed")
		record.Event(managed, event.Normal(reasonPending, "Waiting for external resource existence to be confirmed"))
		return reconcile.Result{Requeue: true}, nil
	}

	if meta.WasDeleted(managed) {
		log = log.WithValues("deletion-timestamp", managed.GetDeletionTimestamp())

		if observation.ResourceExists && policy.ShouldDelete() {
			if err := external.Delete(externalCtx, managed); err != nil {
				// We'll hit this condition if we can't delete our external
				// resource, for example if our provider credentials don't have
				// access to delete it. If this is the first time we encounter
				// this issue we'll be requeued implicitly when we update our
				// status with the new error condition. If not, we want requeue
				// explicitly, which will trigger backoff.
				log.Debug("Cannot delete external resource", "error", err)
				record.Event(managed, event.Warning(reasonCannotDelete, err))
				managed.SetConditions(xpv1.Deleting(), xpv1.ReconcileError(errors.Wrap(err, errReconcileDelete)))
				return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
			}

			// We've successfully requested deletion of our external resource.
			// We queue another reconcile after a short wait rather than
			// immediately finalizing our delete in order to verify that the
			// external resource was actually deleted. If it no longer exists
			// we'll skip this block on the next reconcile and proceed to
			// unpublish and finalize. If it still exists we'll re-enter this
			// block and try again.
			log.Debug("Successfully requested deletion of external resource")
			record.Event(managed, event.Normal(reasonDeleted, "Successfully requested deletion of external resource"))
			managed.SetConditions(xpv1.Deleting(), xpv1.ReconcileSuccess())
			return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
		}
		if err := r.managed.UnpublishConnection(ctx, managed, observation.ConnectionDetails); err != nil {
			// If this is the first time we encounter this issue we'll be
			// requeued implicitly when we update our status with the new error
			// condition. If not, we requeue explicitly, which will trigger
			// backoff.
			log.Debug("Cannot unpublish connection details", "error", err)
			record.Event(managed, event.Warning(reasonCannotUnpublish, err))
			managed.SetConditions(xpv1.Deleting(), xpv1.ReconcileError(err))
			return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
		}
		if err := r.managed.RemoveFinalizer(ctx, managed); err != nil {
			// If this is the first time we encounter this issue we'll be
			// requeued implicitly when we update our status with the new error
			// condition. If not, we requeue explicitly, which will trigger
			// backoff.
			log.Debug("Cannot remove managed resource finalizer", "error", err)
			managed.SetConditions(xpv1.Deleting(), xpv1.ReconcileError(err))
			return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
		}

		// We've successfully deleted our external resource (if necessary) and
		// removed our finalizer. If we assume we were the only controller that
		// added a finalizer to this resource then it should no longer exist and
		// thus there is no point trying to update its status.
		log.Debug("Successfully deleted managed resource")
		return reconcile.Result{Requeue: false}, nil
	}

	if _, err := r.managed.PublishConnection(ctx, managed, observation.ConnectionDetails); err != nil {
		// If this is the first time we encounter this issue we'll be requeued
		// implicitly when we update our status with the new error condition. If
		// not, we requeue explicitly, which will trigger backoff.
		log.Debug("Cannot publish connection details", "error", err)
		record.Event(managed, event.Warning(reasonCannotPublish, err))
		managed.SetConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
	}

	if err := r.managed.AddFinalizer(ctx, managed); err != nil {
		// If this is the first time we encounter this issue we'll be requeued
		// implicitly when we update our status with the new error condition. If
		// not, we requeue explicitly, which will trigger backoff.
		log.Debug("Cannot add finalizer", "error", err)
		managed.SetConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
	}

	if !observation.ResourceExists && policy.ShouldCreate() {
		// We write this annotation for two reasons. Firstly, it helps
		// us to detect the case in which we fail to persist critical
		// information (like the external name) that may be set by the
		// subsequent external.Create call. Secondly, it guarantees that
		// we're operating on the latest version of our resource. We
		// don't use the CriticalAnnotationUpdater because we _want_ the
		// update to fail if we get a 409 due to a stale version.
		meta.SetExternalCreatePending(managed, time.Now())
		if err := r.client.Update(ctx, managed); err != nil {
			log.Debug(errUpdateManaged, "error", err)
			record.Event(managed, event.Warning(reasonCannotUpdateManaged, errors.Wrap(err, errUpdateManaged)))
			managed.SetConditions(xpv1.Creating(), xpv1.ReconcileError(errors.Wrap(err, errUpdateManaged)))
			return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
		}

		creation, err := external.Create(externalCtx, managed)
		if err != nil {
			// We'll hit this condition if we can't create our external
			// resource, for example if our provider credentials don't have
			// access to create it. If this is the first time we encounter this
			// issue we'll be requeued implicitly when we update our status with
			// the new error condition. If not, we requeue explicitly, which will trigger backoff.
			log.Debug("Cannot create external resource", "error", err)
			record.Event(managed, event.Warning(reasonCannotCreate, err))

			// We handle annotations specially here because it's
			// critical that they are persisted to the API server.
			// If we don't add the external-create-failed annotation
			// the reconciler will refuse to proceed, because it
			// won't know whether or not it created an external
			// resource.
			meta.SetExternalCreateFailed(managed, time.Now())
			if err := r.managed.UpdateCriticalAnnotations(ctx, managed); err != nil {
				log.Debug(errUpdateManagedAnnotations, "error", err)
				record.Event(managed, event.Warning(reasonCannotUpdateManaged, errors.Wrap(err, errUpdateManagedAnnotations)))

				// We only log and emit an event here rather
				// than setting a status condition and returning
				// early because presumably it's more useful to
				// set our status condition to the reason the
				// create failed.
			}

			managed.SetConditions(xpv1.Creating(), xpv1.ReconcileError(errors.Wrap(err, errReconcileCreate)))
			return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
		}

		// In some cases our external-name may be set by Create above.
		log = log.WithValues("external-name", meta.GetExternalName(managed))
		record = r.record.WithAnnotations("external-name", meta.GetExternalName(managed))

		// We handle annotations specially here because it's critical
		// that they are persisted to the API server. If we don't remove
		// add the external-create-succeeded annotation the reconciler
		// will refuse to proceed, because it won't know whether or not
		// it created an external resource. This is also important in
		// cases where we must record an external-name annotation set by
		// the Create call. Any other changes made during Create will be
		// reverted when annotations are updated; at the time of writing
		// Create implementations are advised not to alter status, but
		// we may revisit this in future.
		meta.SetExternalCreateSucceeded(managed, time.Now())
		if err := r.managed.UpdateCriticalAnnotations(ctx, managed); err != nil {
			log.Debug(errUpdateManagedAnnotations, "error", err)
			record.Event(managed, event.Warning(reasonCannotUpdateManaged, errors.Wrap(err, errUpdateManagedAnnotations)))
			managed.SetConditions(xpv1.Creating(), xpv1.ReconcileError(errors.Wrap(err, errUpdateManagedAnnotations)))
			return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
		}

		if _, err := r.managed.PublishConnection(ctx, managed, creation.ConnectionDetails); err != nil {
			// If this is the first time we encounter this issue we'll be
			// requeued implicitly when we update our status with the new error
			// condition. If not, we requeue explicitly, which will trigger backoff.
			log.Debug("Cannot publish connection details", "error", err)
			record.Event(managed, event.Warning(reasonCannotPublish, err))
			managed.SetConditions(xpv1.Creating(), xpv1.ReconcileError(err))
			return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
		}

		// We've successfully created our external resource. In many cases the
		// creation process takes a little time to finish. We requeue explicitly
		// order to observe the external resource to determine whether it's
		// ready for use.
		log.Debug("Successfully requested creation of external resource")
		record.Event(managed, event.Normal(reasonCreated, "Successfully requested creation of external resource"))
		managed.SetConditions(xpv1.Creating(), xpv1.ReconcileSuccess())
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
	}

	if observation.ResourceLateInitialized && policy.ShouldLateInitialize() {
		// Note that this update may reset any pending updates to the status of
		// the managed resource from when it was observed above. This is because
		// the API server replies to the update with its unchanged view of the
		// resource's status, which is subsequently deserialized into managed.
		// This is usually tolerable because the update will implicitly requeue
		// an immediate reconcile which should re-observe the external resource
		// and persist its status.
		if err := r.client.Update(ctx, managed); err != nil {
			log.Debug(errUpdateManaged, "error", err)
			record.Event(managed, event.Warning(reasonCannotUpdateManaged, err))
			managed.SetConditions(xpv1.ReconcileError(errors.Wrap(err, errUpdateManaged)))
			return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
		}
	}

	if observation.ResourceUpToDate {
		// We did not need to create, update, or delete our external resource.
		// Per the below issue nothing will notify us if and when the external
		// resource we manage changes, so we requeue a speculative reconcile
		// after the specified poll interval in order to observe it and react
		// accordingly.
		// https://github.com/crossplane/crossplane/issues/289
		log.Debug("External resource is up to date", "requeue-after", time.Now().Add(r.pollInterval))
		managed.SetConditions(xpv1.ReconcileSuccess())
		return reconcile.Result{RequeueAfter: r.pollInterval}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
	}

	if observation.Diff != "" {
		log.Debug("External resource differs from desired state", "diff", observation.Diff)
	}

	// skip the update if the management policy is set to ignore updates
	if !policy.ShouldUpdate() {
		log.Debug("Skipping update due to managementPolicies. Reconciliation succeeded", "requeue-after", time.Now().Add(r.pollInterval))
		managed.SetConditions(xpv1.ReconcileSuccess())
		return reconcile.Result{RequeueAfter: r.pollInterval}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
	}

	update, err := external.Update(externalCtx, managed)
	if err != nil {
		// We'll hit this condition if we can't update our external resource,
		// for example if our provider credentials don't have access to update
		// it. If this is the first time we encounter this issue we'll be
		// requeued implicitly when we update our status with the new error
		// condition. If not, we requeue explicitly, which will trigger backoff.
		log.Debug("Cannot update external resource")
		record.Event(managed, event.Warning(reasonCannotUpdate, err))
		managed.SetConditions(xpv1.ReconcileError(errors.Wrap(err, errReconcileUpdate)))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
	}

	if _, err := r.managed.PublishConnection(ctx, managed, update.ConnectionDetails); err != nil {
		// If this is the first time we encounter this issue we'll be requeued
		// implicitly when we update our status with the new error condition. If
		// not, we requeue explicitly, which will trigger backoff.
		log.Debug("Cannot publish connection details", "error", err)
		record.Event(managed, event.Warning(reasonCannotPublish, err))
		managed.SetConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
	}

	// We've successfully updated our external resource. Per the below issue
	// nothing will notify us if and when the external resource we manage
	// changes, so we requeue a speculative reconcile after the specified poll
	// interval in order to observe it and react accordingly.
	// https://github.com/crossplane/crossplane/issues/289
	log.Debug("Successfully requested update of external resource", "requeue-after", time.Now().Add(r.pollInterval))
	record.Event(managed, event.Normal(reasonUpdated, "Successfully requested update of external resource"))
	managed.SetConditions(xpv1.ReconcileSuccess())
	return reconcile.Result{RequeueAfter: r.pollInterval}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
}
