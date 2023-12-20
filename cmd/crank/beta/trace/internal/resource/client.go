/*
Copyright 2023 The Crossplane Authors.

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

package resource

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	xpmeta "github.com/crossplane/crossplane-runtime/pkg/meta"
	xpresource "github.com/crossplane/crossplane-runtime/pkg/resource"
	xpunstructured "github.com/crossplane/crossplane-runtime/pkg/resource/unstructured"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/claim"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
	pkgname "github.com/google/go-containerregistry/pkg/name"

	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	pkgv1beta1 "github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/internal/xpkg"
)

const (
	errFmtResourceTypeNotFound = "the server doesn't have a resource type %q"
)

// Client to get a Resource with all its children.
type Client struct {
	getConnectionSecrets bool
	dependencyOutput     DependencyOutput
	revisionOutput       RevisionOutput

	client  client.Client
	rmapper meta.RESTMapper
}

// ClientOption is a functional option for a Client.
type ClientOption func(*Client)

// WithConnectionSecrets is a functional option that sets the client to get secrets to the desired value.
func WithConnectionSecrets(v bool) ClientOption {
	return func(c *Client) {
		c.getConnectionSecrets = v
	}
}

func WithDependencyOutput(do DependencyOutput) ClientOption {
	return func(c *Client) {
		c.dependencyOutput = do
	}
}

func WithRevisionOutput(ro RevisionOutput) ClientOption {
	return func(c *Client) {
		c.revisionOutput = ro
	}
}

// NewClient returns a new Client.
func NewClient(in client.Client, rmapper meta.RESTMapper, opts ...ClientOption) (*Client, error) {
	uClient := xpunstructured.NewClient(in)

	c := &Client{
		client:  uClient,
		rmapper: rmapper,
	}

	for _, o := range opts {
		o(c)
	}

	return c, nil
}

// GetResourceTree returns the requested Resource and all its children.
func (kc *Client) GetResourceTree(ctx context.Context, rootRef *v1.ObjectReference) (*Resource, error) {
	root := kc.getResource(ctx, rootRef)
	// We should just surface any error getting the root resource immediately.
	if err := root.Error; err != nil {
		return nil, err
	}

	if IsPackageType(root.Unstructured.GroupVersionKind().GroupKind()) {
		// the root is a package type, get the lock file now
		lock := &v1beta1.Lock{}
		if err := kc.client.Get(ctx, types.NamespacedName{Name: "lock"}, lock); err != nil {
			return nil, err
		}

		return kc.getPackageTree(ctx, root, lock, map[string]struct{}{})
	}

	// Set up a FIFO queue to traverse the resource tree breadth first.
	queue := []*Resource{root}

	for len(queue) > 0 {
		// Pop the first element from the queue.
		res := queue[0]
		queue = queue[1:]

		refs := getResourceChildrenRefs(res, kc.getConnectionSecrets)

		for i := range refs {
			child := kc.getResource(ctx, &refs[i])

			res.Children = append(res.Children, child)
			queue = append(queue, child)
		}
	}

	return root, nil
}

func (kc *Client) getPackageTree(ctx context.Context, node *Resource, lock *v1beta1.Lock, uniqueDeps map[string]struct{}) (*Resource, error) {
	nodeGK := node.Unstructured.GroupVersionKind().GroupKind()
	var prl pkgv1.PackageRevisionList

	switch nodeGK {
	case pkgv1.ProviderGroupVersionKind.GroupKind():
		prl = &pkgv1.ProviderRevisionList{}
	case pkgv1.ConfigurationGroupVersionKind.GroupKind():
		prl = &pkgv1.ConfigurationRevisionList{}
	case pkgv1beta1.FunctionGroupVersionKind.GroupKind():
		prl = &pkgv1beta1.FunctionRevisionList{}
	default:
		return nil, errors.New(fmt.Sprintf("unknown package type %s", nodeGK))
	}

	if kc.revisionOutput != RevisionOutputNone {
		// retrieve revisions for this package to add them as children of the current node
		if err := kc.client.List(ctx, prl, client.MatchingLabels(map[string]string{pkgv1.LabelParentPackage: node.Unstructured.GetName()})); xpresource.IgnoreNotFound(err) != nil {
			return nil, err
		}
		prs := prl.GetRevisions()
		for i := range prs {
			pr := prs[i]
			childRevision := kc.getResource(ctx, xpmeta.ReferenceTo(pr, pr.GetObjectKind().GroupVersionKind()))

			// add the current revision as a child of the current node if the revision output says we should
			state, _ := fieldpath.Pave(childRevision.Unstructured.Object).GetString("spec.desiredState")
			isActive := pkgv1.PackageRevisionDesiredState(state) == pkgv1.PackageRevisionActive
			if kc.revisionOutput == RevisionOutputAll || (kc.revisionOutput == RevisionOutputActive && isActive) {
				node.Children = append(node.Children, childRevision)
			}
		}
	}

	if kc.dependencyOutput == DependencyOutputNone {
		// we're not supposed to show any dependencies, we can return now
		return node, nil
	}

	cr, err := fieldpath.Pave(node.Unstructured.Object).GetString("status.currentRevision")
	if err != nil || cr == "" {
		// we don't have a current package revision, so just return what we've found so far
		return node, nil
	}

	// find the lock file entry for the current revision
	var lp *pkgv1beta1.LockPackage
	for i := range lock.Packages {
		if cr == lock.Packages[i].Name {
			lp = &lock.Packages[i]
			break
		}
	}

	if lp == nil {
		// the current revision for this package isn't in the lock file yet,
		// just return what we've found so far
		return node, nil
	}

	// iterate over all dependencies of the package to get full references to them
	var depRefs []v1.ObjectReference
	for _, d := range lp.Dependencies {
		if kc.dependencyOutput == DependencyOutputUnique {
			if _, ok := uniqueDeps[d.Package]; ok {
				// we are supposed to only show unique dependencies, and we've seen this one already in the tree, skip it
				continue
			}
		}

		var name string
		var apiVersion string
		var pkgKind string
		var revision pkgv1.PackageRevision

		// figure out if the current dependency is a providers, configuration, or function
		switch d.Type {
		case pkgv1beta1.ProviderPackageType:
			pkgKind = pkgv1.ProviderKind
			apiVersion = pkgv1.ProviderGroupVersionKind.GroupVersion().String()
			revision = &pkgv1.ProviderRevision{}
		case pkgv1beta1.ConfigurationPackageType:
			pkgKind = pkgv1.ConfigurationKind
			apiVersion = pkgv1.ConfigurationGroupVersionKind.GroupVersion().String()
			revision = &pkgv1.ConfigurationRevision{}
		case pkgv1beta1.FunctionPackageType:
			pkgKind = pkgv1beta1.FunctionKind
			apiVersion = pkgv1beta1.FunctionGroupVersionKind.GroupVersion().String()
			revision = &pkgv1beta1.FunctionRevision{}
		default:
			return nil, errors.New(fmt.Sprintf("unknown package dependency type %s", d.Type))
		}

		// NOTE: everything in the lock file is basically a package revision
		// - pkgrev A
		//   - dependency: pkgrev B
		//   - dependency: pkgrev C
		// - pkgrev B
		// - pkgrev C

		// find the current dependency from all the packages in the lock file
		for _, p := range lock.Packages {
			if p.Source == d.Package {
				// current package source matches the package of the dependency, let's get the full object
				if err := kc.client.Get(ctx, types.NamespacedName{Name: p.Name}, revision); xpresource.IgnoreNotFound(err) != nil {
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

		if name == "" {
			// we didn't find a package to match the current dependency, which
			// can happen during initial installation when dependencies are
			// being discovered and fetched. We'd still like to show something
			// though, so try to make the package name pretty
			if pkgref, err := pkgname.ParseReference(d.Package); err == nil {
				name = xpkg.ToDNSLabel(pkgref.Context().RepositoryStr())
			} else {
				name = xpkg.ToDNSLabel(d.Package)
			}
		}

		depRefs = append(depRefs, v1.ObjectReference{
			APIVersion: apiVersion,
			Kind:       pkgKind,
			Name:       name,
		})

		// track this dependency in the unique dependency map
		uniqueDeps[d.Package] = struct{}{}
	}

	// traverse all the references to dependencies that we found to build the tree out with them too
	for i := range depRefs {
		child := kc.getResource(ctx, &depRefs[i])
		node.Children = append(node.Children, child)

		if _, err := kc.getPackageTree(ctx, child, lock, uniqueDeps); err != nil {
			return nil, err
		}
	}

	return node, nil
}

// getResource returns the requested Resource, setting any error as Resource.Error.
func (kc *Client) getResource(ctx context.Context, ref *v1.ObjectReference) *Resource {
	result := unstructured.Unstructured{}
	result.SetGroupVersionKind(ref.GroupVersionKind())

	err := kc.client.Get(ctx, xpmeta.NamespacedNameOf(ref), &result)

	if err != nil {
		// If the resource is not found, we still want to return a Resource
		// object with the name and namespace set, so that the caller can
		// still use it.
		result.SetName(ref.Name)
		result.SetNamespace(ref.Namespace)
	}
	return &Resource{Unstructured: result, Error: err}
}

// getResourceChildrenRefs returns the references to the children for the given
// Resource, assuming it's a Crossplane resource, XR or XRC.
func getResourceChildrenRefs(r *Resource, getConnectionSecrets bool) []v1.ObjectReference {
	obj := r.Unstructured
	// collect object references for the
	var refs []v1.ObjectReference

	switch obj.GroupVersionKind().GroupKind() {
	case schema.GroupKind{Group: "", Kind: "Secret"},
		v1alpha1.UsageGroupVersionKind.GroupKind(),
		v1alpha1.EnvironmentConfigGroupVersionKind.GroupKind():
		// nothing to do here, it's a resource we know not to have any reference
		return nil
	}

	if xrcNamespace := obj.GetNamespace(); xrcNamespace != "" {
		// This is an XRC, get the XR ref, we leave the connection secret
		// handling to the XR
		xrc := claim.Unstructured{Unstructured: obj}
		if ref := xrc.GetResourceReference(); ref != nil {
			refs = append(refs, v1.ObjectReference{
				APIVersion: ref.APIVersion,
				Kind:       ref.Kind,
				Name:       ref.Name,
				Namespace:  ref.Namespace,
			})
		}
		if getConnectionSecrets {
			xrcSecretRef := xrc.GetWriteConnectionSecretToReference()
			if xrcSecretRef != nil {
				ref := v1.ObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
					Name:       xrcSecretRef.Name,
					Namespace:  xrcNamespace,
				}
				refs = append(refs, ref)
			}
		}
		return refs
	}
	// This could be an XR or an MR
	xr := composite.Unstructured{Unstructured: obj}
	xrRefs := xr.GetResourceReferences()
	if len(xrRefs) == 0 {
		// This is an MR
		return refs
	}
	// This is an XR, get the Composed resources refs and the
	// connection secret if required
	refs = append(refs, xrRefs...)

	if !getConnectionSecrets {
		// We don't need the connection secret, so we can stop here
		return refs
	}
	xrSecretRef := xr.GetWriteConnectionSecretToReference()
	if xrSecretRef != nil {
		ref := v1.ObjectReference{
			APIVersion: "v1",
			Kind:       "Secret",
			Name:       xrSecretRef.Name,
			Namespace:  xrSecretRef.Namespace,
		}
		refs = append(refs, ref)
	}

	return refs
}

// MappingFor returns the RESTMapping for the given resource or kind argument.
// Copied over from cli-runtime pkg/resource Builder,
// https://github.com/kubernetes/cli-runtime/blob/9a91d944dd43186c52e0162e12b151b0e460354a/pkg/resource/builder.go#L768
func (kc *Client) MappingFor(resourceOrKindArg string) (*meta.RESTMapping, error) {
	// TODO(phisco): actually use the Builder.
	fullySpecifiedGVR, groupResource := schema.ParseResourceArg(resourceOrKindArg)
	gvk := schema.GroupVersionKind{}
	if fullySpecifiedGVR != nil {
		gvk, _ = kc.rmapper.KindFor(*fullySpecifiedGVR)
	}
	if gvk.Empty() {
		gvk, _ = kc.rmapper.KindFor(groupResource.WithVersion(""))
	}
	if !gvk.Empty() {
		return kc.rmapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	}
	fullySpecifiedGVK, groupKind := schema.ParseKindArg(resourceOrKindArg)
	if fullySpecifiedGVK == nil {
		gvk := groupKind.WithVersion("")
		fullySpecifiedGVK = &gvk
	}
	if !fullySpecifiedGVK.Empty() {
		if mapping, err := kc.rmapper.RESTMapping(fullySpecifiedGVK.GroupKind(), fullySpecifiedGVK.Version); err == nil {
			return mapping, nil
		}
	}
	mapping, err := kc.rmapper.RESTMapping(groupKind, gvk.Version)
	if err != nil {
		// if we error out here, it is because we could not match a resource or a kind
		// for the given argument. To maintain consistency with previous behavior,
		// announce that a resource type could not be found.
		// if the error is _not_ a *meta.NoKindMatchError, then we had trouble doing discovery,
		// so we should return the original error since it may help a user diagnose what is actually wrong
		if meta.IsNoMatchError(err) {
			return nil, fmt.Errorf(errFmtResourceTypeNotFound, groupResource.Resource)
		}
		return nil, err
	}
	return mapping, nil
}
