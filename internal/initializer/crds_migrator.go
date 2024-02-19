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

package initializer

import (
	"context"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// NewCoreCRDsMigrator returns a new *CoreCRDsMigrator.
func NewCoreCRDsMigrator(crdName, sourceVersion string) *CoreCRDsMigrator {
	c := &CoreCRDsMigrator{
		crdName:    crdName,
		oldVersion: sourceVersion,
	}
	return c
}

// CoreCRDsMigrator makes sure the CRDs are using the latest storage version.
type CoreCRDsMigrator struct {
	crdName    string
	oldVersion string
}

// Run applies all CRDs in the given directory.
func (c *CoreCRDsMigrator) Run(ctx context.Context, kube client.Client) error {
	var crd extv1.CustomResourceDefinition
	if err := kube.Get(ctx, client.ObjectKey{Name: c.crdName}, &crd); err != nil {
		if kerrors.IsNotFound(err) {
			// nothing to do
			return nil
		}
		return errors.Wrapf(err, "cannot get %s crd", c.crdName)
	}
	// no old version in the crd, nothing to do
	if !sets.NewString(crd.Status.StoredVersions...).Has(c.oldVersion) {
		return nil
	}
	// we need to patch all resources to the new storage version
	var storageVersion string
	for _, v := range crd.Spec.Versions {
		if v.Storage {
			storageVersion = v.Name
			break
		}
	}
	resources := unstructured.UnstructuredList{}
	resources.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   crd.Spec.Group,
		Version: storageVersion,
		Kind:    crd.Spec.Names.ListKind,
	})
	var continueToken string
	for {
		if err := kube.List(ctx, &resources,
			client.Limit(500),
			client.Continue(continueToken),
		); err != nil {
			return errors.Wrapf(err, "cannot list %s", resources.GroupVersionKind().String())
		}
		for i := range resources.Items {
			// apply empty patch for storage version upgrade
			res := resources.Items[i]
			if err := kube.Patch(ctx, &res, client.RawPatch(types.MergePatchType, []byte(`{}`))); err != nil {
				return errors.Wrapf(err, "cannot patch %s %q", crd.Spec.Names.Kind, res.GetName())
			}
		}
		continueToken = resources.GetContinue()
		if continueToken == "" {
			break
		}
	}

	origCrd := crd.DeepCopy()
	crd.Status.StoredVersions = []string{storageVersion}
	if err := kube.Status().Patch(ctx, &crd, client.MergeFrom(origCrd)); err != nil {
		return errors.Wrapf(err, "couldn't update %s crd", c.crdName)
	}

	// One more check just to be sure we actually updated the crd
	if err := kube.Get(ctx, client.ObjectKey{Name: c.crdName}, &crd); err != nil {
		return errors.Wrapf(err, "cannot get %s crd to check", c.crdName)
	}
	if len(crd.Status.StoredVersions) != 1 || crd.Status.StoredVersions[0] != storageVersion {
		return errors.Errorf("was expecting CRD %q to only have %s, got instead: %v", c.crdName, storageVersion, crd.Status.StoredVersions)
	}

	return nil
}
