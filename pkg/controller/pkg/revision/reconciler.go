/*
Copyright 2020 The Crossplane Authors.

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

package revision

import (
	"context"
	"strings"
	"time"

	"github.com/pkg/errors"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/parser"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	apiextensionsv1alpha1 "github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
	pkgmeta "github.com/crossplane/crossplane/apis/pkg/meta/v1alpha1"
	"github.com/crossplane/crossplane/apis/pkg/v1alpha1"
)

const (
	reconcileTimeout = 1 * time.Minute

	shortWait = 30 * time.Second
	longWait  = 1 * time.Minute
)

const (
	errGetPackageRevision = "cannot get package revision"
	errUpdateStatus       = "cannot update package revision status"

	errInitParserBackend = "cannot initialize parser backend"
	errParseLogs         = "cannot parse pod logs"
	errNotOneMeta        = "cannot install package with multiple meta types"

	errPreHook  = "cannot run pre establish hook for package"
	errPostHook = "cannot run post establish hook for package"

	errEstablishControl = "cannot establish control of object"
)

// Event reasons.
const (
	reasonParse event.Reason = "ParsePackage"
	reasonLint  event.Reason = "LintPackage"
	reasonSync  event.Reason = "SyncPackage"
)

// ReconcilerOption is used to configure the Reconciler.
type ReconcilerOption func(*Reconciler)

// WithNewPackageRevisionFn determines the type of package being reconciled.
func WithNewPackageRevisionFn(f func() v1alpha1.PackageRevision) ReconcilerOption {
	return func(r *Reconciler) {
		r.newPackageRevision = f
	}
}

// WithLogger specifies how the Reconciler should log messages.
func WithLogger(log logging.Logger) ReconcilerOption {
	return func(r *Reconciler) {
		r.log = log
	}
}

// WithRecorder specifies how the Reconciler should record Kubernetes events.
func WithRecorder(er event.Recorder) ReconcilerOption {
	return func(r *Reconciler) {
		r.record = er
	}
}

// WithHooks specifies how the Reconciler should perform pre and post object
// establishment operations.
func WithHooks(h Hooks) ReconcilerOption {
	return func(r *Reconciler) {
		r.hook = h
	}
}

// WithEstablisher specifies how the Reconciler should establish package resources.
func WithEstablisher(e Establisher) ReconcilerOption {
	return func(r *Reconciler) {
		r.objects = e
	}
}

// WithParser specifies how the Reconciler should parse a package.
func WithParser(p parser.Parser) ReconcilerOption {
	return func(r *Reconciler) {
		r.parser = p
	}
}

// WithParserBackend specifies how the Reconciler should parse a package.
func WithParserBackend(p parser.Backend) ReconcilerOption {
	return func(r *Reconciler) {
		r.backend = p
	}
}

// WithLinter specifies how the Reconciler should parse a package.
func WithLinter(l Linter) ReconcilerOption {
	return func(r *Reconciler) {
		r.linter = l
	}
}

// BuildMetaScheme builds the default scheme used for identifying metadata in a
// Crossplane package.
func BuildMetaScheme() (*runtime.Scheme, error) {
	metaScheme := runtime.NewScheme()
	if err := pkgmeta.SchemeBuilder.AddToScheme(metaScheme); err != nil {
		return nil, err
	}
	return metaScheme, nil
}

// BuildObjectScheme builds the default scheme used for identifying objects in a
// Crossplane package.
func BuildObjectScheme() (*runtime.Scheme, error) {
	objScheme := runtime.NewScheme()
	if err := apiextensionsv1alpha1.AddToScheme(objScheme); err != nil {
		return nil, err
	}
	if err := v1beta1.AddToScheme(objScheme); err != nil {
		return nil, err
	}
	return objScheme, nil
}

// Reconciler reconciles packages.
type Reconciler struct {
	client       client.Client
	hook         Hooks
	objects      Establisher
	podLogClient kubernetes.Interface
	parser       parser.Parser
	linter       Linter
	backend      parser.Backend
	log          logging.Logger
	record       event.Recorder

	newPackageRevision func() v1alpha1.PackageRevision
}

// SetupProviderRevision adds a controller that reconciles ProviderRevisions.
func SetupProviderRevision(mgr ctrl.Manager, l logging.Logger, namespace string) error {
	name := "packages/" + strings.ToLower(v1alpha1.ProviderRevisionGroupKind)
	nr := func() v1alpha1.PackageRevision { return &v1alpha1.ProviderRevision{} }

	clientset, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return errors.Wrap(err, "failed to initialize host clientset with in cluster config")
	}

	metaScheme, err := BuildMetaScheme()
	if err != nil {
		return errors.New("cannot build meta scheme for package parser")
	}
	objScheme, err := BuildObjectScheme()
	if err != nil {
		return errors.New("cannot build object scheme for package parser")
	}

	r := NewReconciler(mgr,
		clientset,
		WithHooks(NewProviderHooks(resource.ClientApplicator{
			Client:     mgr.GetClient(),
			Applicator: resource.NewAPIPatchingApplicator(mgr.GetClient()),
		}, namespace)),
		WithNewPackageRevisionFn(nr),
		WithParser(parser.New(metaScheme, objScheme)),
		WithParserBackend(parser.NewPodLogBackend(parser.PodNamespace(namespace))),
		WithLinter(NewPackageLinter([]PackageLinterFn{OneMeta}, []ObjectLinterFn{IsProvider}, []ObjectLinterFn{IsCRD})),
		WithLogger(l.WithValues("controller", name)),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.ProviderRevision{}).
		Complete(r)
}

// SetupConfigurationRevision adds a controller that reconciles ConfigurationRevisions.
func SetupConfigurationRevision(mgr ctrl.Manager, l logging.Logger, namespace string) error {
	name := "packages/" + strings.ToLower(v1alpha1.ConfigurationRevisionGroupKind)
	nr := func() v1alpha1.PackageRevision { return &v1alpha1.ConfigurationRevision{} }

	clientset, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return errors.Wrap(err, "failed to initialize host clientset with in cluster config")
	}

	metaScheme, err := BuildMetaScheme()
	if err != nil {
		return errors.New("cannot build meta scheme for package parser")
	}
	objScheme, err := BuildObjectScheme()
	if err != nil {
		return errors.New("cannot build object scheme for package parser")
	}

	r := NewReconciler(mgr,
		clientset,
		WithHooks(NewConfigurationHooks()),
		WithNewPackageRevisionFn(nr),
		WithParser(parser.New(metaScheme, objScheme)),
		WithParserBackend(parser.NewPodLogBackend(parser.PodNamespace(namespace))),
		WithLinter(NewPackageLinter([]PackageLinterFn{OneMeta}, []ObjectLinterFn{IsConfiguration}, []ObjectLinterFn{Or(IsXRD, IsComposition)})),
		WithLogger(l.WithValues("controller", name)),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.ConfigurationRevision{}).
		Complete(r)
}

// NewReconciler creates a new package reconciler.
func NewReconciler(mgr ctrl.Manager, clientset kubernetes.Interface, opts ...ReconcilerOption) *Reconciler {
	r := &Reconciler{
		client:       mgr.GetClient(),
		hook:         NewNopHooks(),
		objects:      NewAPIEstablisher(mgr.GetClient()),
		podLogClient: clientset,
		parser:       parser.New(nil, nil),
		linter:       NewPackageLinter(nil, nil, nil),
		log:          logging.NewNopLogger(),
		record:       event.NewNopRecorder(),
	}

	for _, f := range opts {
		f(r)
	}

	return r
}

// Reconcile package revision.
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) { // nolint:gocyclo
	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(context.Background(), reconcileTimeout)
	defer cancel()

	pr := r.newPackageRevision()
	if err := r.client.Get(ctx, req.NamespacedName, pr); err != nil {
		// There's no need to requeue if we no longer exist. Otherwise we'll be
		// requeued implicitly because we return an error.
		log.Debug(errGetPackageRevision, "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGetPackageRevision)
	}

	log = log.WithValues(
		"uid", pr.GetUID(),
		"version", pr.GetResourceVersion(),
		"name", pr.GetName(),
	)

	// Initialize parser backend to obtain package contents.
	reader, err := r.backend.Init(ctx, parser.PodClient(r.podLogClient), parser.PodName(pr.GetInstallPod().Name))
	if err != nil {
		log.Debug(errInitParserBackend, "error", err)
		r.record.Event(pr, event.Warning(reasonParse, errors.Wrap(err, errInitParserBackend)))
		// Requeue after shortWait because we may be waiting for parent package
		// controller to recreate Pod.
		pr.SetConditions(v1alpha1.Unhealthy())
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, pr), errUpdateStatus)
	}

	// Parse package contents.
	pkg, err := r.parser.Parse(ctx, reader)
	if err != nil {
		log.Debug(errParseLogs, "error", err)
		r.record.Event(pr, event.Warning(reasonParse, errors.Wrap(err, errParseLogs)))
		pr.SetConditions(v1alpha1.Unhealthy())
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, pr), errUpdateStatus)
	}

	// Lint package.
	if err := r.linter.Lint(pkg); err != nil {
		r.record.Event(pr, event.Warning(reasonLint, err))
		// NOTE(hasheddan): a failed lint typically will require manual
		// intervention, but on the off chance that we read pod logs early,
		// which caused a linting failure, we will requeue after long wait.
		pr.SetConditions(v1alpha1.Unhealthy())
		return reconcile.Result{RequeueAfter: longWait}, errors.Wrap(r.client.Status().Update(ctx, pr), errUpdateStatus)
	}

	// NOTE(hasheddan): the linter should check this property already, but if a
	// consumer forgets to pass an option to guarantee one meta object, we check
	// here to avoid a potential panic on 0 index below.
	if len(pkg.GetMeta()) != 1 {
		r.record.Event(pr, event.Warning(reasonLint, errors.New(errNotOneMeta)))
		pr.SetConditions(v1alpha1.Unhealthy())
		return reconcile.Result{RequeueAfter: longWait}, errors.Wrap(r.client.Status().Update(ctx, pr), errUpdateStatus)
	}

	pkgMeta := pkg.GetMeta()[0]
	if err := r.hook.Pre(ctx, pkgMeta, pr); err != nil {
		log.Debug(errPreHook, "error", err)
		r.record.Event(pr, event.Warning(reasonSync, errors.Wrap(err, errPreHook)))
		pr.SetConditions(v1alpha1.Unhealthy())
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, pr), errUpdateStatus)
	}

	// Establish control or ownership of objects.
	refs, err := r.objects.Establish(ctx, pkg.GetObjects(), pr, pr.GetDesiredState() == v1alpha1.PackageRevisionActive)
	if err != nil {
		log.Debug(errEstablishControl, "error", err)
		r.record.Event(pr, event.Warning(reasonSync, errors.Wrap(err, errEstablishControl)))
		pr.SetConditions(v1alpha1.Unhealthy())
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, pr), errUpdateStatus)
	}

	// Update object list in package revision status with objects for which
	// ownership or control has been established.
	pr.SetObjects(refs)

	if err := r.hook.Post(ctx, pkgMeta, pr); err != nil {
		log.Debug(errPostHook, "error", err)
		r.record.Event(pr, event.Warning(reasonSync, errors.Wrap(err, errPostHook)))
		pr.SetConditions(v1alpha1.Unhealthy())
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, pr), errUpdateStatus)
	}

	r.record.Event(pr, event.Normal(reasonSync, "Successfully configured package revision"))
	pr.SetConditions(v1alpha1.Healthy())
	return reconcile.Result{RequeueAfter: longWait}, errors.Wrap(r.client.Status().Update(ctx, pr), errUpdateStatus)
}
