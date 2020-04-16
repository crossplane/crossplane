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

package definer

import (
	"context"

	"github.com/pkg/errors"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metaapi "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

const (
	errDeleteNonControlledCRD = "cannot delete a crd that the definer does not control"
	errGenerateCRD            = "cannot generate crd for given infrastructure definition"
)

// NewAPIInfrastructureClient returns a new APIInfrastructureClient.
func NewAPIInfrastructureClient(client client.Client) Client {
	return &APIInfrastructureClient{client: resource.ClientApplicator{
		Client:     client,
		Applicator: resource.NewAPIPatchingApplicator(client),
	}}
}

// APIInfrastructureClient manages the generated CRD.
type APIInfrastructureClient struct {
	client resource.ClientApplicator
}

// Get fetches the CRD.
func (m *APIInfrastructureClient) Get(ctx context.Context, definer Definer) (*v1beta1.CustomResourceDefinition, error) {
	crd := &v1beta1.CustomResourceDefinition{}
	return crd, m.client.Get(ctx, types.NamespacedName{Name: definer.GetCRDName()}, crd)
}

// Apply applies the CRD that the definer generates.
func (m *APIInfrastructureClient) Apply(ctx context.Context, definer Definer) error {
	generated, err := definer.GenerateCRD()
	if err != nil {
		return errors.Wrap(err, errGenerateCRD)
	}
	meta.AddOwnerReference(generated, meta.AsController(meta.ReferenceTo(definer, definer.GetObjectKind().GroupVersionKind())))
	return m.client.Apply(ctx, generated, resource.MustBeControllableBy(definer.GetUID()))
}

// Delete the generated CRD.
func (m *APIInfrastructureClient) Delete(ctx context.Context, definer Definer) error {
	crd := &v1beta1.CustomResourceDefinition{}
	err := m.client.Get(ctx, types.NamespacedName{Name: definer.GetCRDName()}, crd)
	if resource.IgnoreNotFound(err) != nil {
		return err
	}
	if kerrors.IsNotFound(err) {
		return nil
	}
	if !metav1.IsControlledBy(crd, definer) {
		return errors.New(errDeleteNonControlledCRD)
	}
	return resource.IgnoreNotFound(m.client.Delete(ctx, crd))
}

// DeleteInstances deletes all instances of the generated CRD in all namespaces
// and returns whether there are any remaining instances.
func (m *APIInfrastructureClient) DeleteInstances(ctx context.Context, definer Definer) (bool, error) {
	// Empty namespace option covers all namespaces in case CRD is namespace-scoped.
	// If it is cluster-scoped, it doesn't have any effect.
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(definer.GetCRDGroupVersionKind())
	err := m.client.List(ctx, list, client.InNamespace(""))
	switch {
	case metaapi.IsNoMatchError(err):
		return true, nil
	case err != nil:
		return false, err
	case len(list.Items) == 0:
		return true, nil
	}
	// NOTE(muvaf): When user deletes InfrastructureDefinition object the deletion
	// signal does not cascade to the owned resource until owner is gone. But
	// owner has its own finalizer that depends on having no instance of the CRD
	// because it cannot go away before stopping the controller.
	// So, we need to delete all instances of CRD manually here.
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(definer.GetCRDGroupVersionKind())
	return false, resource.Ignore(metaapi.IsNoMatchError, m.client.DeleteAllOf(ctx, obj, client.InNamespace("")))
}
