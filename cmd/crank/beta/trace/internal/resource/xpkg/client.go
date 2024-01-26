/*
Copyright 2024 The Crossplane Authors.

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

package xpkg

import (
	"context"
	"slices"

	pkgname "github.com/google/go-containerregistry/pkg/name"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	xpresource "github.com/crossplane/crossplane-runtime/pkg/resource"
	xpunstructured "github.com/crossplane/crossplane-runtime/pkg/resource/unstructured"

	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1alpha1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/cmd/crank/beta/trace/internal/resource"
	"github.com/crossplane/crossplane/internal/xpkg"
)

// Client to get a Package with all its dependencies.
type Client struct {
	dependencyOutput            DependencyOutput
	revisionOutput              RevisionOutput
	includePackageRuntimeConfig bool

	client client.Client
}

// ClientOption is a functional option for a Client.
type ClientOption func(*Client)

// WithDependencyOutput is a functional option that configures how the client should output dependencies.
func WithDependencyOutput(do DependencyOutput) ClientOption {
	return func(c *Client) {
		c.dependencyOutput = do
	}
}

// WithRevisionOutput is a functional option that configures how the client should output revisions.
func WithRevisionOutput(ro RevisionOutput) ClientOption {
	return func(c *Client) {
		c.revisionOutput = ro
	}
}

// WithPackageRuntimeConfigs is a functional option that configures if the client
// should include the package runtime config as a child.
func WithPackageRuntimeConfigs(v bool) ClientOption {
	return func(c *Client) {
		c.includePackageRuntimeConfig = v
	}
}

// NewClient returns a new Client.
func NewClient(in client.Client, opts ...ClientOption) (*Client, error) {
	uClient := xpunstructured.NewClient(in)

	c := &Client{
		client: uClient,
	}

	for _, o := range opts {
		o(c)
	}

	return c, nil
}

// GetResourceTree returns the requested package Resource and all its children.
func (kc *Client) GetResourceTree(ctx context.Context, root *resource.Resource) (*resource.Resource, error) {
	var err error
	if !IsPackageType(root.Unstructured.GroupVersionKind().GroupKind()) {
		return nil, errors.Errorf("resource %s is not a package", root.Unstructured.GetName())
	}

	// the root is a package type, get the lock file now
	lock := &v1beta1.Lock{}
	if err := kc.client.Get(ctx, types.NamespacedName{Name: "lock"}, lock); err != nil {
		return nil, err
	}

	// Set up a FIFO queue to traverse the resource tree breadth first.
	queue := []*resource.Resource{root}

	uniqueDeps := map[string]struct{}{}

	for len(queue) > 0 {
		// Pop the first element from the queue.
		res := queue[0]
		queue = queue[1:]

		if !IsPackageType(res.Unstructured.GroupVersionKind().GroupKind()) {
			return nil, errors.Errorf("resource %s is not a package: %s", res.Unstructured.GetName(), res.Unstructured.GroupVersionKind().GroupKind())
		}

		// Set the package runtime config as a child if we want to show it
		kc.setPackageRuntimeConfigChild(ctx, res)

		// Set the revisions for the current package and add them as children
		if err := kc.setChildrenRevisions(ctx, res); err != nil {
			return nil, errors.Wrapf(err, "failed to set package revision children for package %s", res.Unstructured.GetName())
		}

		refs := make([]v1.ObjectReference, 0)

		if kc.dependencyOutput != DependencyOutputNone {
			refs, err = kc.getPackageDeps(ctx, res, lock, uniqueDeps)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to get dependencies for package %s", res.Unstructured.GetName())
			}
		}

		for i := range refs {
			child := resource.GetResource(ctx, kc.client, &refs[i])

			res.Children = append(res.Children, child)
			queue = append(queue, child)
		}
	}

	return root, nil
}

func (kc *Client) setPackageRuntimeConfigChild(ctx context.Context, res *resource.Resource) {
	if !kc.includePackageRuntimeConfig {
		return
	}
	runtimeConfigRef := pkgv1.RuntimeConfigReference{}
	if err := fieldpath.Pave(res.Unstructured.Object).GetValueInto("spec.runtimeConfigRef", &runtimeConfigRef); err == nil {
		res.Children = append(res.Children, resource.GetResource(ctx, kc.client, &v1.ObjectReference{
			APIVersion: *runtimeConfigRef.APIVersion,
			Kind:       *runtimeConfigRef.Kind,
			Name:       runtimeConfigRef.Name,
		}))
	}
	// We try loading both as currently both are supported and if both are present they are merged.
	controllerConfigRef := pkgv1.ControllerConfigReference{}
	apiVersion, kind := v1alpha1.ControllerConfigGroupVersionKind.ToAPIVersionAndKind()
	if err := fieldpath.Pave(res.Unstructured.Object).GetValueInto("spec.controllerConfigRef", &runtimeConfigRef); err == nil {
		res.Children = append(res.Children, resource.GetResource(ctx, kc.client, &v1.ObjectReference{
			APIVersion: apiVersion,
			Kind:       kind,
			Name:       controllerConfigRef.Name,
		}))
	}
}

func (kc *Client) setChildrenRevisions(ctx context.Context, res *resource.Resource) (err error) {
	// Nothing to do if we don't want to show revisions
	if kc.revisionOutput == RevisionOutputNone {
		return nil
	}
	revisions, err := kc.getRevisions(ctx, res)
	if err != nil {
		return errors.Wrapf(err, "failed to get revisions for package %s", res.Unstructured.GetName())
	}

	// add the current revision as a child of the current node if the revision output says we should
	for _, r := range revisions {
		state, _ := fieldpath.Pave(r.Unstructured.Object).GetString("spec.desiredState")
		switch pkgv1.PackageRevisionDesiredState(state) {
		case pkgv1.PackageRevisionActive:
			res.Children = append(res.Children, r)
		case pkgv1.PackageRevisionInactive:
			if kc.revisionOutput == RevisionOutputAll {
				res.Children = append(res.Children, r)
			}
		}
	}
	return nil
}

// getRevisions gets the revisions for the given package.
func (kc *Client) getRevisions(ctx context.Context, xpkg *resource.Resource) ([]*resource.Resource, error) {
	revisions := &unstructured.UnstructuredList{}
	switch gvk := xpkg.Unstructured.GroupVersionKind(); gvk.GroupKind() {
	case pkgv1.ProviderGroupVersionKind.GroupKind():
		revisions.SetGroupVersionKind(pkgv1.ProviderRevisionGroupVersionKind)
	case pkgv1.ConfigurationGroupVersionKind.GroupKind():
		revisions.SetGroupVersionKind(pkgv1.ConfigurationRevisionGroupVersionKind)
	case v1beta1.FunctionGroupVersionKind.GroupKind():
		revisions.SetGroupVersionKind(v1beta1.FunctionRevisionGroupVersionKind)
	default:
		// If we didn't match any of the know types, we try to guess
		revisions.SetGroupVersionKind(gvk.GroupVersion().WithKind(gvk.Kind + "RevisionList"))
	}

	if err := kc.client.List(ctx, revisions, client.MatchingLabels(map[string]string{pkgv1.LabelParentPackage: xpkg.Unstructured.GetName()})); xpresource.IgnoreNotFound(err) != nil {
		return nil, err
	}
	// Sort the revisions by creation timestamp to have a stable output
	slices.SortFunc(revisions.Items, func(i, j unstructured.Unstructured) int {
		return i.GetCreationTimestamp().Compare(j.GetCreationTimestamp().Time)
	})
	resources := make([]*resource.Resource, 0, len(revisions.Items))
	for i := range revisions.Items {
		resources = append(resources, &resource.Resource{Unstructured: revisions.Items[i]})
	}
	return resources, nil
}

// getPackageDetails returns the package details for the given package type.
func getPackageDetails(t v1beta1.PackageType) (string, string, pkgv1.PackageRevision, error) {
	switch t {
	case v1beta1.ProviderPackageType:
		return pkgv1.ProviderKind, pkgv1.ProviderGroupVersionKind.GroupVersion().String(), &pkgv1.ProviderRevision{}, nil
	case v1beta1.ConfigurationPackageType:
		return pkgv1.ConfigurationKind, pkgv1.ConfigurationGroupVersionKind.GroupVersion().String(), &pkgv1.ConfigurationRevision{}, nil
	case v1beta1.FunctionPackageType:
		return v1beta1.FunctionKind, v1beta1.FunctionGroupVersionKind.GroupVersion().String(), &v1beta1.FunctionRevision{}, nil
	default:
		return "", "", nil, errors.Errorf("unknown package dependency type %s", t)
	}
}

// getDependencyRef returns the dependency reference for the given package,
// based on the lock file.
func (kc *Client) getDependencyRef(ctx context.Context, lock *v1beta1.Lock, pkgType v1beta1.PackageType, pkg string) (*v1.ObjectReference, error) {
	// if we don't find a package to match the current dependency, which
	// can happen during initial installation when dependencies are
	// being discovered and fetched. We'd still like to show something
	// though, so try to make the package name pretty
	name := xpkg.ToDNSLabel(pkg)
	if pkgref, err := pkgname.ParseReference(pkg); err == nil {
		name = xpkg.ToDNSLabel(pkgref.Context().RepositoryStr())
	}

	// NOTE: everything in the lock file is basically a package revision
	// - pkgrev A
	//   - dependency: pkgrev B
	//   - dependency: pkgrev C
	// - pkgrev B
	// - pkgrev C

	// find the current dependency from all the packages in the lock file
	pkgKind, apiVersion, revision, err := getPackageDetails(pkgType)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get package details for dependency %s", pkg)
	}

	for _, p := range lock.Packages {
		if p.Source == pkg {
			// current package source matches the package of the dependency, let's get the full object
			if err = kc.client.Get(ctx, types.NamespacedName{Name: p.Name}, revision); xpresource.IgnoreNotFound(err) != nil {
				return nil, err
			}

			// look for the owner of this package revision, that's its parent package
			for _, or := range revision.GetOwnerReferences() {
				if or.Kind == pkgKind && or.Controller != nil && *or.Controller {
					name = or.Name
					break
				}
			}
			break
		}
	}

	return &v1.ObjectReference{
		APIVersion: apiVersion,
		Kind:       pkgKind,
		Name:       name,
	}, nil
}

// getPackageDeps returns the dependencies for the given package resource.
func (kc *Client) getPackageDeps(ctx context.Context, node *resource.Resource, lock *v1beta1.Lock, uniqueDeps map[string]struct{}) ([]v1.ObjectReference, error) {
	cr, _ := fieldpath.Pave(node.Unstructured.Object).GetString("status.currentRevision")
	if cr == "" {
		// we don't have a current package revision, so just return empty deps
		return nil, nil
	}

	// find the lock file entry for the current revision
	var lp *v1beta1.LockPackage
	for i := range lock.Packages {
		if lock.Packages[i].Name == cr {
			lp = &lock.Packages[i]
			break
		}
	}
	if lp == nil {
		// the current revision for this package isn't in the lock file yet,
		// so just return empty deps
		return nil, nil
	}

	// iterate over all dependencies of the package to get full references to them
	depRefs := make([]v1.ObjectReference, 0)
	for _, d := range lp.Dependencies {
		if kc.dependencyOutput == DependencyOutputUnique {
			if _, ok := uniqueDeps[d.Package]; ok {
				// we are supposed to only show unique dependencies, and we've seen this one already in the tree, skip it
				continue
			}
		}

		dep, err := kc.getDependencyRef(ctx, lock, d.Type, d.Package)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get dependency ref %s", d.Package)
		}
		depRefs = append(depRefs, *dep)

		// track this dependency in the unique dependency map
		uniqueDeps[d.Package] = struct{}{}
	}
	return depRefs, nil
}
