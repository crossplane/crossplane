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

	pkgname "github.com/google/go-containerregistry/pkg/name"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	xpmeta "github.com/crossplane/crossplane-runtime/pkg/meta"
	xpresource "github.com/crossplane/crossplane-runtime/pkg/resource"
	xpunstructured "github.com/crossplane/crossplane-runtime/pkg/resource/unstructured"

	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/cmd/crank/beta/trace/internal/resource"
	"github.com/crossplane/crossplane/cmd/crank/beta/trace/internal/resource/xrm"
	"github.com/crossplane/crossplane/internal/xpkg"
)

// Client to get a Package with all its dependencies.
type Client struct {
	dependencyOutput DependencyOutput
	revisionOutput   RevisionOutput

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

// GetResourceTree returns the requested Resource and all its children.
func (kc *Client) GetResourceTree(ctx context.Context, root *resource.Resource) (*resource.Resource, error) {
	if !IsPackageType(root.Unstructured.GroupVersionKind().GroupKind()) {
		return nil, errors.Errorf("resource %s is not a package", root.Unstructured.GetName())
	}

	// the root is a package type, get the lock file now
	lock := &v1beta1.Lock{}
	if err := kc.client.Get(ctx, types.NamespacedName{Name: "lock"}, lock); err != nil {
		return nil, err
	}

	return kc.getPackageTree(ctx, root, lock, map[string]struct{}{})
}

func (kc *Client) setPackageRevisionChildren(ctx context.Context, node *resource.Resource) error {
	revisions, err := kc.getRevisions(ctx, node)
	if err != nil {
		return errors.Wrapf(err, "failed to get revisions for package %s", node.Unstructured.GetName())
	}

	for _, r := range revisions {
		// add the current revision as a child of the current node if the revision output says we should
		state, _ := fieldpath.Pave(r.Unstructured.Object).GetString("spec.desiredState")
		isActive := pkgv1.PackageRevisionDesiredState(state) == pkgv1.PackageRevisionActive
		if kc.revisionOutput == RevisionOutputAll || (kc.revisionOutput == RevisionOutputActive && isActive) {
			node.Children = append(node.Children, r)
		}
	}
	return nil
}

func getLockPackageForRevision(lock *v1beta1.Lock, revision string) *v1beta1.LockPackage {
	for i := range lock.Packages {
		if lock.Packages[i].Name == revision {
			return &lock.Packages[i]
		}
	}
	return nil
}

// getRevisions gets the revisions for the given package.
func (kc *Client) getRevisions(ctx context.Context, node *resource.Resource) ([]*resource.Resource, error) {
	nodeGK := node.Unstructured.GroupVersionKind().GroupKind()
	var prl pkgv1.PackageRevisionList

	switch nodeGK {
	case pkgv1.ProviderGroupVersionKind.GroupKind():
		prl = &pkgv1.ProviderRevisionList{}
	case pkgv1.ConfigurationGroupVersionKind.GroupKind():
		prl = &pkgv1.ConfigurationRevisionList{}
	case v1beta1.FunctionGroupVersionKind.GroupKind():
		prl = &v1beta1.FunctionRevisionList{}
	default:
		return nil, errors.Errorf("unknown package type %s", nodeGK)
	}

	if err := kc.client.List(ctx, prl, client.MatchingLabels(map[string]string{pkgv1.LabelParentPackage: node.Unstructured.GetName()})); xpresource.IgnoreNotFound(err) != nil {
		return nil, err
	}
	prs := prl.GetRevisions()
	resources := make([]*resource.Resource, 0, len(prs))
	for i := range prs {
		pr := prs[i]
		childRevision := xrm.GetResource(ctx, kc.client, xpmeta.ReferenceTo(pr, pr.GetObjectKind().GroupVersionKind()))
		resources = append(resources, childRevision)
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
	var name string
	pkgKind, apiVersion, revision, err := getPackageDetails(pkgType)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get package details for dependency %s", pkg)
	}

	// if we don't find a package to match the current dependency, which
	// can happen during initial installation when dependencies are
	// being discovered and fetched. We'd still like to show something
	// though, so try to make the package name pretty
	if pkgref, err := pkgname.ParseReference(pkg); err == nil {
		name = xpkg.ToDNSLabel(pkgref.Context().RepositoryStr())
	} else {
		name = xpkg.ToDNSLabel(pkg)
	}

	// NOTE: everything in the lock file is basically a package revision
	// - pkgrev A
	//   - dependency: pkgrev B
	//   - dependency: pkgrev C
	// - pkgrev B
	// - pkgrev C

	// find the current dependency from all the packages in the lock file
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

// getDependencies returns the dependencies for the given package resource.
func (kc *Client) getDependencies(ctx context.Context, node *resource.Resource, lock *v1beta1.Lock, uniqueDeps map[string]struct{}) ([]v1.ObjectReference, error) {
	cr, _ := fieldpath.Pave(node.Unstructured.Object).GetString("status.currentRevision")
	if cr == "" {
		// we don't have a current package revision, so just return empty deps
		return nil, nil
	}

	// find the lock file entry for the current revision
	lp := getLockPackageForRevision(lock, cr)
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

// setDependencyChildren gets and sets the dependencies for the given package
// as its children
func (kc *Client) setDependencyChildren(ctx context.Context, node *resource.Resource, lock *v1beta1.Lock, uniqueDeps map[string]struct{}) error {
	depRefs, err := kc.getDependencies(ctx, node, lock, uniqueDeps)
	if err != nil {
		return errors.Wrapf(err, "failed to get dependencies for package %s", node.Unstructured.GetName())
	}

	// traverse all the references to dependencies that we found to build the tree out with them too
	for i := range depRefs {
		child := xrm.GetResource(ctx, kc.client, &depRefs[i])
		node.Children = append(node.Children, child)

		if _, err := kc.getPackageTree(ctx, child, lock, uniqueDeps); err != nil {
			return errors.Wrapf(err, "failed to get package tree for dependency %s", child.Unstructured.GetName())
		}
	}
	return nil
}

// getPackageTree constructs the package resource tree for the given package.
func (kc *Client) getPackageTree(ctx context.Context, node *resource.Resource, lock *v1beta1.Lock, uniqueDeps map[string]struct{}) (*resource.Resource, error) {
	// get the revisions for the current package and add them as children
	if kc.revisionOutput != RevisionOutputNone {
		err := kc.setPackageRevisionChildren(ctx, node)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to set package revision children for package %s", node.Unstructured.GetName())
		}
	}

	// get the dependencies for the current package and add them as children
	if kc.dependencyOutput != DependencyOutputNone {
		err := kc.setDependencyChildren(ctx, node, lock, uniqueDeps)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to set package dependency children for package %s", node.Unstructured.GetName())
		}
	}

	return node, nil
}
