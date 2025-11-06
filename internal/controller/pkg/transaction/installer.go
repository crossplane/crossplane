/*
Copyright 2025 The Crossplane Authors.

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

package transaction

import (
	"cmp"
	"context"
	"slices"
	"strconv"

	"github.com/google/go-containerregistry/pkg/name"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	pkgmetav1 "github.com/crossplane/crossplane/v2/apis/pkg/meta/v1"
	v1 "github.com/crossplane/crossplane/v2/apis/pkg/v1"
	"github.com/crossplane/crossplane/v2/apis/pkg/v1alpha1"
	"github.com/crossplane/crossplane/v2/internal/controller/pkg/revision"
	pkgruntime "github.com/crossplane/crossplane/v2/internal/controller/pkg/runtime"
	"github.com/crossplane/crossplane/v2/internal/initializer"
	"github.com/crossplane/crossplane/v2/internal/xpkg"
)

const (
	// WebhookPortName is the name of the webhook port.
	WebhookPortName = "webhook"
)

// Installer installs one aspect of a package (the Package resource, the
// PackageRevision, runtime components, or objects like CRDs).
type Installer interface {
	Install(ctx context.Context, tx *v1alpha1.Transaction, xp *xpkg.Package) error
}

// InstallerPipeline is a slice of Installers that are called in order.
type InstallerPipeline []Installer

// Install runs all installers in the pipeline in order.
func (p InstallerPipeline) Install(ctx context.Context, tx *v1alpha1.Transaction, xp *xpkg.Package) error {
	for _, installer := range p {
		if err := installer.Install(ctx, tx, xp); err != nil {
			return err
		}
	}
	return nil
}

// Establisher establishes control or ownership of a set of resources in the
// API server by checking that control or ownership can be established for all
// resources and then establishing it.
type Establisher interface {
	Establish(ctx context.Context, objects []runtime.Object, parent v1.PackageRevision, control bool) ([]xpv1.TypedReference, error)
}

// NopEstablisher does nothing.
type NopEstablisher struct{}

// NewNopEstablisher returns a new NopEstablisher.
func NewNopEstablisher() *NopEstablisher {
	return &NopEstablisher{}
}

// Establish does nothing.
func (*NopEstablisher) Establish(_ context.Context, _ []runtime.Object, _ v1.PackageRevision, _ bool) ([]xpv1.TypedReference, error) {
	return nil, nil
}

// PackageCreator creates or updates Package resources.
type PackageCreator struct {
	kube client.Client
}

// NewPackageCreator returns a new PackageCreator.
func NewPackageCreator(kube client.Client) *PackageCreator {
	return &PackageCreator{kube: kube}
}

// Install creates or updates the Package resource.
func (i *PackageCreator) Install(ctx context.Context, tx *v1alpha1.Transaction, xp *xpkg.Package) error {
	pkg, _, err := NewPackageAndRevision(xp)
	if err != nil {
		return err
	}

	// List packages to find pkgs one with matching source repository
	pkgs, _, err := NewPackageAndRevisionList(xp)
	if err != nil {
		return err
	}
	if err := i.kube.List(ctx, pkgs); err != nil {
		return errors.Wrap(err, "cannot list packages")
	}

	// Use existing package name if found, otherwise keep generated name
	if name := FindExistingPackage(pkgs, xp); name != "" {
		pkg.SetName(name)
	}

	_, err = ctrl.CreateOrUpdate(ctx, i.kube, pkg, func() error {
		pkg.SetSource(xpkg.BuildReference(xp.Source, xp.Version))

		// For new packages, generation is 0 on the client side but will be 1
		// after the API server creates them. For existing packages, use their
		// current generation.
		generation := pkg.GetGeneration()
		if generation == 0 {
			generation = 1
		}

		meta.AddLabels(pkg, map[string]string{
			v1alpha1.LabelTransactionName:       tx.GetName(),
			v1alpha1.LabelTransactionGeneration: strconv.FormatInt(generation, 10),
		})

		return nil
	})

	return errors.Wrap(err, "cannot create or update package")
}

// RevisionCreator creates or updates PackageRevision resources.
type RevisionCreator struct {
	kube client.Client
}

// NewRevisionCreator returns a new RevisionCreator.
func NewRevisionCreator(kube client.Client) *RevisionCreator {
	return &RevisionCreator{kube: kube}
}

// Install installs a PackageRevision by creating or updating it,
// deactivating other revisions, and garbage collecting old revisions.
//
// The package manager maintains at most one active revision per package at any
// time. When a new revision is installed, all other revisions are deactivated.
// This ensures a clean transition between package versions.
func (i *RevisionCreator) Install(ctx context.Context, tx *v1alpha1.Transaction, xp *xpkg.Package) error {
	pkg, rev, err := NewPackageAndRevision(xp)
	if err != nil {
		return err
	}

	// List packages to find pkgs one with matching source repository
	pkgs, revs, err := NewPackageAndRevisionList(xp)
	if err != nil {
		return err
	}
	if err := i.kube.List(ctx, pkgs); err != nil {
		return errors.Wrap(err, "cannot list packages")
	}

	// Use existing package name if found, otherwise keep generated name
	if name := FindExistingPackage(pkgs, xp); name != "" {
		pkg.SetName(name)
		rev.SetName(xpkg.FriendlyID(name, xp.DigestHex()))
	}

	if err := i.kube.Get(ctx, types.NamespacedName{Name: pkg.GetName()}, pkg); err != nil {
		return errors.Wrap(err, "cannot get package")
	}

	if err := i.kube.List(ctx, revs, client.MatchingLabels{v1.LabelParentPackage: pkg.GetName()}); err != nil {
		return errors.Wrap(err, "cannot list package revisions")
	}

	// Find the highest revision number to ensure our new revision gets a
	// higher number. Revision numbers are monotonically increasing.
	var maxRevision int64
	for _, r := range revs.GetRevisions() {
		if r.GetRevision() > maxRevision {
			maxRevision = r.GetRevision()
		}
	}

	// Deactivate all other revisions before activating the new one. This
	// ensures only one revision is active at a time, regardless of the
	// package's activation policy.
	for _, r := range revs.GetRevisions() {
		if r.GetName() == rev.GetName() {
			continue
		}
		if r.GetDesiredState() != v1.PackageRevisionActive {
			continue
		}
		r.SetDesiredState(v1.PackageRevisionInactive)
		if err := i.kube.Update(ctx, r); err != nil {
			return errors.Wrapf(err, "cannot deactivate revision %s", r.GetName())
		}
	}

	if _, err = ctrl.CreateOrUpdate(ctx, i.kube, rev, func() error {
		// The current revision should always be the highest numbered revision.
		// This ensures monotonically increasing revision numbers even when
		// reverting to an older package version.
		if rev.GetRevision() < maxRevision || maxRevision == 0 {
			rev.SetRevision(maxRevision + 1)
		}

		meta.AddOwnerReference(rev, meta.AsOwner(meta.TypedReferenceTo(pkg, pkg.GetObjectKind().GroupVersionKind())))
		meta.AddOwnerReference(rev, meta.AsOwner(meta.TypedReferenceTo(tx, tx.GetObjectKind().GroupVersionKind())))

		meta.AddLabels(rev, map[string]string{
			v1.LabelParentPackage:         pkg.GetName(),
			v1alpha1.LabelTransactionName: tx.GetName(),
		})

		// Propagate package configuration to revision.
		rev.SetSource(xpkg.BuildReference(xp.Source, xp.Version))
		rev.SetPackagePullPolicy(pkg.GetPackagePullPolicy())
		rev.SetPackagePullSecrets(pkg.GetPackagePullSecrets())
		rev.SetIgnoreCrossplaneConstraints(pkg.GetIgnoreCrossplaneConstraints())
		rev.SetSkipDependencyResolution(pkg.GetSkipDependencyResolution())
		rev.SetCommonLabels(pkg.GetCommonLabels())

		// Propagate runtime configuration for Provider and Function packages.
		if pwr, ok := pkg.(v1.PackageWithRuntime); ok {
			if prwr, ok := rev.(v1.PackageRevisionWithRuntime); ok {
				prwr.SetRuntimeConfigRef(pwr.GetRuntimeConfigRef())
				prwr.SetTLSServerSecretName(pwr.GetTLSServerSecretName())
				prwr.SetTLSClientSecretName(pwr.GetTLSClientSecretName())
			}
		}

		// Only activate if not already active and the package has automatic
		// activation policy. Manual activation policy means the user must
		// explicitly activate revisions.
		if rev.GetDesiredState() != v1.PackageRevisionActive && ptr.Deref(pkg.GetActivationPolicy(), v1.AutomaticActivation) == v1.AutomaticActivation {
			rev.SetDesiredState(v1.PackageRevisionActive)
		}

		return nil
	}); err != nil {
		return errors.Wrap(err, "cannot create or update package revision")
	}

	// Garbage collect old revisions if we exceed the history limit. The
	// default limit is 1, meaning only the current revision is kept. Setting
	// to 0 disables garbage collection. We check len > limit+1 because the
	// list includes the revision we just created/updated.
	limit := ptr.Deref(pkg.GetRevisionHistoryLimit(), 1)
	if limit == 0 || len(revs.GetRevisions()) <= int(limit)+1 {
		return nil
	}

	gc := revs.GetRevisions()
	slices.SortFunc(gc, func(a, b v1.PackageRevision) int {
		return cmp.Compare(a.GetRevision(), b.GetRevision())
	})

	err = resource.IgnoreNotFound(i.kube.Delete(ctx, gc[0]))
	return errors.Wrapf(err, "cannot garbage collect revision %s", gc[0].GetName())
}

// RuntimeBootstrapper creates runtime prerequisites for packages.
type RuntimeBootstrapper struct {
	kube      client.Client
	namespace string
}

// NewRuntimeBootstrapper returns a new RuntimeBootstrapper.
func NewRuntimeBootstrapper(kube client.Client, namespace string) *RuntimeBootstrapper {
	return &RuntimeBootstrapper{
		kube:      kube,
		namespace: namespace,
	}
}

// ObjectInstaller installs package objects (CRDs, XRDs, Compositions, webhooks, etc.).
type ObjectInstaller struct {
	kube    client.Client
	objects Establisher
}

// NewObjectInstaller returns a new ObjectInstaller.
func NewObjectInstaller(kube client.Client, e Establisher) *ObjectInstaller {
	return &ObjectInstaller{
		kube:    kube,
		objects: e,
	}
}

// Install installs the package's objects by establishing control of them.
func (i *ObjectInstaller) Install(ctx context.Context, tx *v1alpha1.Transaction, xp *xpkg.Package) error {
	_, rev, err := NewPackageAndRevision(xp)
	if err != nil {
		return err
	}

	// Fetch the revision from the API server to get its current status,
	// including the TLS secret names set by BootstrapRuntime.
	if err := i.kube.Get(ctx, types.NamespacedName{Name: rev.GetName()}, rev); err != nil {
		return errors.Wrap(err, "cannot get package revision")
	}

	// Label all objs with the transaction for traceability.
	objs := xp.GetObjects()
	for _, obj := range objs {
		if mo, ok := obj.(metav1.Object); ok {
			meta.AddLabels(mo, map[string]string{
				v1alpha1.LabelTransactionName: tx.GetName(),
			})
		}
	}

	// Establish control of all objects in the package. The Establisher handles
	// CRDs, webhooks, and other Kubernetes objects.
	_, err = i.objects.Establish(ctx, objs, rev, true)
	return errors.Wrap(err, "cannot establish control of package objects")
}

// RevisionStatusUpdater updates PackageRevision status conditions after
// installation completes.
type RevisionStatusUpdater struct {
	kube client.Client
}

// NewRevisionStatusUpdater returns a new RevisionStatusUpdater.
func NewRevisionStatusUpdater(kube client.Client) *RevisionStatusUpdater {
	return &RevisionStatusUpdater{kube: kube}
}

// Install sets the TypeRevisionHealthy condition on the PackageRevision after
// successful installation. This runs after ObjectInstaller has established
// control of all package objects (CRDs, XRDs, Compositions). The revision
// controller normally sets this condition, but since we don't run the revision
// controller in transaction mode, we set it here.
func (i *RevisionStatusUpdater) Install(ctx context.Context, _ *v1alpha1.Transaction, xp *xpkg.Package) error {
	pkg, rev, err := NewPackageAndRevision(xp)
	if err != nil {
		return err
	}

	// List packages to find existing one with matching source repository
	pkgList, _, err := NewPackageAndRevisionList(xp)
	if err != nil {
		return err
	}
	if err := i.kube.List(ctx, pkgList); err != nil {
		return errors.Wrap(err, "cannot list packages")
	}

	// Use existing package name if found, otherwise keep generated name
	if existingName := FindExistingPackage(pkgList, xp); existingName != "" {
		pkg.SetName(existingName)
		rev.SetName(xpkg.FriendlyID(existingName, xp.DigestHex()))
	}

	// Get the revision to update its status
	if err := i.kube.Get(ctx, types.NamespacedName{Name: rev.GetName()}, rev); err != nil {
		return errors.Wrap(err, "cannot get package revision")
	}

	// Set status fields from the fetched package
	rev.SetResolvedSource(xpkg.BuildReference(xp.ResolvedSource, xp.Version))
	rev.SetAppliedImageConfigRefs(AsImageConfigRefs(xp.AppliedImageConfigs)...)

	// Set RevisionHealthy condition after successfully establishing objects
	rev.SetConditions(v1.RevisionHealthy())

	return errors.Wrap(i.kube.Status().Update(ctx, rev), "cannot update package revision status")
}

// PackageStatusUpdater updates Package status fields after installation.
type PackageStatusUpdater struct {
	kube client.Client
}

// NewPackageStatusUpdater returns a new PackageStatusUpdater.
func NewPackageStatusUpdater(kube client.Client) *PackageStatusUpdater {
	return &PackageStatusUpdater{kube: kube}
}

// Install sets status fields on the Package that record facts about the
// installation (resolved source and applied image configs).
func (i *PackageStatusUpdater) Install(ctx context.Context, _ *v1alpha1.Transaction, xp *xpkg.Package) error {
	pkg, _, err := NewPackageAndRevision(xp)
	if err != nil {
		return err
	}

	// List packages to find existing one with matching source repository
	pkgList, _, err := NewPackageAndRevisionList(xp)
	if err != nil {
		return err
	}
	if err := i.kube.List(ctx, pkgList); err != nil {
		return errors.Wrap(err, "cannot list packages")
	}

	// Use existing package name if found, otherwise keep generated name
	if existingName := FindExistingPackage(pkgList, xp); existingName != "" {
		pkg.SetName(existingName)
	}

	// Get the package to update its status
	if err := i.kube.Get(ctx, types.NamespacedName{Name: pkg.GetName()}, pkg); err != nil {
		return errors.Wrap(err, "cannot get package")
	}

	// Set status fields from the fetched package
	pkg.SetResolvedSource(xpkg.BuildReference(xp.ResolvedSource, xp.Version))
	pkg.SetAppliedImageConfigRefs(AsImageConfigRefs(xp.AppliedImageConfigs)...)

	// Note: This does NOT set Package conditions. Package conditions
	// (Active, Healthy) are set by the manager-tx controller which
	// continuously monitors and derives them from PackageRevision state.
	// This is necessary because revision conditions are updated
	// asynchronously by other controllers (e.g., the runtime controller
	// sets RuntimeHealthy as the Deployment becomes ready).

	return errors.Wrap(i.kube.Status().Update(ctx, pkg), "cannot update package status")
}

// Install bootstraps runtime prerequisites for packages that need them
// (Providers and Functions). This is needed for the establisher to inject
// webhook configurations into CRDs. Configurations don't have runtimes so this
// is a no-op for them.
func (i *RuntimeBootstrapper) Install(ctx context.Context, _ *v1alpha1.Transaction, xp *xpkg.Package) error {
	_, rev, err := NewPackageAndRevision(xp)
	if err != nil {
		return err
	}

	// Get the revision we just created to have the full object
	if err := i.kube.Get(ctx, types.NamespacedName{Name: rev.GetName()}, rev); err != nil {
		return errors.Wrap(err, "cannot get package revision")
	}

	switch r := rev.(type) {
	case *v1.ProviderRevision:
		return i.BootstrapProviderRuntime(ctx, r)
	case *v1.FunctionRevision:
		return i.BootstrapFunctionRuntime(ctx, r)
	case *v1.ConfigurationRevision:
		// Configurations don't have runtimes
		return nil
	default:
		return errors.Errorf("unknown package revision type %T", rev)
	}
}

// BootstrapProviderRuntime creates runtime prerequisites (service and TLS secrets)
// for a ProviderRevision.
func (i *RuntimeBootstrapper) BootstrapProviderRuntime(ctx context.Context, pr *v1.ProviderRevision) error {
	if pr.GetDesiredState() != v1.PackageRevisionActive {
		return nil
	}

	pr.SetObservedTLSServerSecretName(pr.GetTLSServerSecretName())
	pr.SetObservedTLSClientSecretName(pr.GetTLSClientSecretName())

	builder := pkgruntime.NewDeploymentRuntimeBuilder(pr, i.namespace)

	// Create service. We only Create (not Update) - the runtime controller's
	// Pre hook handles updates via Applicator.Apply.
	svc := builder.Service(pkgruntime.ServiceWithAdditionalPorts([]corev1.ServicePort{
		{
			Name:       WebhookPortName,
			Protocol:   corev1.ProtocolTCP,
			Port:       revision.ServicePort,
			TargetPort: intstr.FromString(WebhookPortName),
		},
	}))
	if err := i.kube.Create(ctx, svc); err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrap(err, "cannot create service")
	}

	// Create TLS secrets
	secClient := builder.TLSClientSecret()
	secServer := builder.TLSServerSecret()

	if secClient == nil || secServer == nil {
		return errors.New("TLS secret names not set on provider revision")
	}

	if err := i.kube.Create(ctx, secClient); err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrap(err, "cannot create TLS client secret")
	}

	if err := i.kube.Create(ctx, secServer); err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrap(err, "cannot create TLS server secret")
	}

	// Generate TLS certificates (idempotent - safe to call multiple times)
	if err := initializer.NewTLSCertificateGenerator(
		secClient.Namespace,
		initializer.RootCACertSecretName,
		initializer.TLSCertificateGeneratorWithOwner(pr.GetOwnerReferences()),
		initializer.TLSCertificateGeneratorWithServerSecretName(secServer.GetName(), initializer.DNSNamesForService(svc.Name, svc.Namespace)),
		initializer.TLSCertificateGeneratorWithClientSecretName(secClient.GetName(), []string{pr.GetName()}),
	).Run(ctx, i.kube); err != nil {
		return errors.Wrapf(err, "cannot generate TLS certificates for %q", pr.GetLabels()[v1.LabelParentPackage])
	}

	// Update revision status to indicate secrets are ready
	if err := i.kube.Status().Update(ctx, pr); err != nil {
		return errors.Wrap(err, "cannot update provider revision status")
	}

	return nil
}

// BootstrapFunctionRuntime creates runtime prerequisites (service and TLS secrets)
// for a FunctionRevision.
func (i *RuntimeBootstrapper) BootstrapFunctionRuntime(ctx context.Context, fr *v1.FunctionRevision) error {
	if fr.GetDesiredState() != v1.PackageRevisionActive {
		return nil
	}

	fr.SetObservedTLSServerSecretName(fr.GetTLSServerSecretName())

	builder := pkgruntime.NewDeploymentRuntimeBuilder(fr, i.namespace)

	// Create service (headless for functions). We only Create (not Update) -
	// the runtime controller's Pre hook handles updates via Applicator.Apply.
	svc := builder.Service(
		pkgruntime.ServiceWithClusterIP(corev1.ClusterIPNone),
		pkgruntime.ServiceWithAdditionalPorts([]corev1.ServicePort{
			{
				Name:       pkgruntime.GRPCPortName,
				Protocol:   corev1.ProtocolTCP,
				Port:       pkgruntime.GRPCPort,
				TargetPort: intstr.FromString(pkgruntime.GRPCPortName),
			},
		}))
	if err := i.kube.Create(ctx, svc); err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrap(err, "cannot create service")
	}

	// Create TLS server secret (functions don't have client secret)
	secServer := builder.TLSServerSecret()

	if secServer == nil {
		return errors.New("TLS server secret name not set on function revision")
	}

	if err := i.kube.Create(ctx, secServer); err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrap(err, "cannot create TLS server secret")
	}

	// Generate TLS certificates (idempotent - safe to call multiple times)
	if err := initializer.NewTLSCertificateGenerator(
		secServer.Namespace,
		initializer.RootCACertSecretName,
		initializer.TLSCertificateGeneratorWithOwner(fr.GetOwnerReferences()),
		initializer.TLSCertificateGeneratorWithServerSecretName(secServer.GetName(), initializer.DNSNamesForService(svc.Name, svc.Namespace)),
	).Run(ctx, i.kube); err != nil {
		return errors.Wrapf(err, "cannot generate TLS certificates for %q", fr.GetLabels()[v1.LabelParentPackage])
	}

	// Update revision status to indicate secrets are ready
	if err := i.kube.Status().Update(ctx, fr); err != nil {
		return errors.Wrap(err, "cannot update function revision status")
	}

	return nil
}

// FindExistingPackage searches for an existing Package in the list that
// matches the source repository. Returns the package name if found, or empty
// string if not found or if an error occurs during parsing.
func FindExistingPackage(pkgList v1.PackageList, xp *xpkg.Package) string {
	// Parse the source to get the repository (without tag/digest)
	ref, err := name.ParseReference(xp.Source)
	if err != nil {
		return ""
	}
	sourceRepo := xpkg.ParsePackageSourceFromReference(ref)

	// Search for existing package with matching repository
	for _, p := range pkgList.GetPackages() {
		existingRef, err := name.ParseReference(p.GetSource())
		if err != nil {
			continue // Skip packages with invalid sources
		}
		if xpkg.ParsePackageSourceFromReference(existingRef) == sourceRepo {
			return p.GetName()
		}
	}

	return ""
}

// AsImageConfigRefs converts xpkg.ImageConfig to v1.ImageConfigRef.
func AsImageConfigRefs(configs []xpkg.ImageConfig) []v1.ImageConfigRef {
	if len(configs) == 0 {
		return nil
	}

	refs := make([]v1.ImageConfigRef, len(configs))
	for i, cfg := range configs {
		refs[i] = v1.ImageConfigRef{
			Name:   cfg.Name,
			Reason: v1.ImageConfigRefReason(cfg.Reason),
		}
	}
	return refs
}

// NewPackageAndRevision creates template Package and PackageRevision resources
// with name and source pre-filled based on the xpkg.Package metadata.
func NewPackageAndRevision(xp *xpkg.Package) (v1.Package, v1.PackageRevision, error) {
	var pkg v1.Package
	var rev v1.PackageRevision

	switch xp.GetMeta().(type) {
	case *pkgmetav1.Provider:
		pkg = &v1.Provider{}
		rev = &v1.ProviderRevision{}
	case *pkgmetav1.Configuration:
		pkg = &v1.Configuration{}
		rev = &v1.ConfigurationRevision{}
	case *pkgmetav1.Function:
		pkg = &v1.Function{}
		rev = &v1.FunctionRevision{}
	default:
		return nil, nil, errors.Errorf("unknown package type %T", xp.GetMeta())
	}

	// Parse the source to extract repository for naming
	ref, err := name.ParseReference(xp.Source)
	if err != nil {
		return nil, nil, errors.Wrap(err, "cannot parse package source")
	}

	name := xpkg.ToDNSLabel(ref.Context().RepositoryStr())
	pkg.SetName(name)
	rev.SetName(xpkg.FriendlyID(name, xp.DigestHex()))

	return pkg, rev, nil
}

// NewPackageAndRevisionList creates empty PackageList and PackageRevisionList
// suitable for listing packages and revisions of the appropriate type.
func NewPackageAndRevisionList(xp *xpkg.Package) (v1.PackageList, v1.PackageRevisionList, error) {
	var pkgList v1.PackageList
	var revList v1.PackageRevisionList

	switch xp.GetMeta().(type) {
	case *pkgmetav1.Provider:
		pkgList = &v1.ProviderList{}
		revList = &v1.ProviderRevisionList{}
	case *pkgmetav1.Configuration:
		pkgList = &v1.ConfigurationList{}
		revList = &v1.ConfigurationRevisionList{}
	case *pkgmetav1.Function:
		pkgList = &v1.FunctionList{}
		revList = &v1.FunctionRevisionList{}
	default:
		return nil, nil, errors.Errorf("unknown package type %T", xp.GetMeta())
	}

	return pkgList, revList, nil
}
