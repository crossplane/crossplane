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

package requirement

import (
	"context"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

// Error strings.
const (
	errCreateComposite   = "cannot create composite resource"
	errUpdateRequirement = "cannot update resource requirement"
	errUpdateComposite   = "cannot update composite resource"
	errDeleteComposite   = "cannot delete composite resource"
	errBindConflict      = "cannot bind composite resource that references a different requirement"
)

// An APICompositeCreator creates resources by submitting them to a Kubernetes
// API server.
type APICompositeCreator struct {
	client client.Client
	typer  runtime.ObjectTyper
}

// NewAPICompositeCreator returns a new APICompositeCreator.
func NewAPICompositeCreator(c client.Client, t runtime.ObjectTyper) *APICompositeCreator {
	return &APICompositeCreator{client: c, typer: t}
}

// TODO(negz): We should render and patch a composite resource on each
// reconcile, rather than just creating it once.

// Create the supplied composite using the supplied requirement.
func (a *APICompositeCreator) Create(ctx context.Context, rq resource.Requirement, cp resource.Composite) error {

	cp.SetRequirementReference(meta.ReferenceTo(rq, resource.MustGetKind(rq, a.typer)))
	if err := a.client.Create(ctx, cp); err != nil {
		return errors.Wrap(err, errCreateComposite)
	}
	// Since we use GenerateName feature of ObjectMeta, final name of the
	// resource is calculated during the creation of the resource. So, we
	// can generate a complete reference only after the creation.
	cpr := meta.ReferenceTo(cp, resource.MustGetKind(cp, a.typer))
	rq.SetResourceReference(cpr)

	return errors.Wrap(a.client.Update(ctx, rq), errUpdateRequirement)
}

// An APICompositeDeleter deletes composite resources from the API server.
type APICompositeDeleter struct {
	client client.Client
}

// NewAPICompositeDeleter returns a new APICompositeDeleter.
func NewAPICompositeDeleter(c client.Client) *APICompositeDeleter {
	return &APICompositeDeleter{client: c}
}

// Delete the supplied composite resource from the API server.
func (a *APICompositeDeleter) Delete(ctx context.Context, _ resource.Requirement, cp resource.Composite) error {
	return errors.Wrap(resource.IgnoreNotFound(a.client.Delete(ctx, cp)), errDeleteComposite)
}

// An APIBinder binds requirements to composites by updating them in a
// Kubernetes API server. Note that APIBinder does not support objects that do
// not use the status subresource; such objects should use
// APIBinder.
type APIBinder struct {
	client client.Client
	typer  runtime.ObjectTyper
}

// NewAPIBinder returns a new APIBinder.
func NewAPIBinder(c client.Client, t runtime.ObjectTyper) *APIBinder {
	return &APIBinder{client: c, typer: t}
}

// Bind the supplied requirement to the supplied composite.
func (a *APIBinder) Bind(ctx context.Context, rq resource.Requirement, cp resource.Composite) error {
	existing := cp.GetRequirementReference()
	proposed := meta.ReferenceTo(rq, resource.MustGetKind(rq, a.typer))

	if existing != nil && (existing.Namespace != proposed.Namespace || existing.Name != proposed.Name) {
		return errors.New(errBindConflict)
	}

	cp.SetRequirementReference(proposed)
	if err := a.client.Update(ctx, cp); err != nil {
		return errors.Wrap(err, errUpdateComposite)
	}

	if meta.GetExternalName(cp) == "" {
		return nil
	}

	// Propagate back the final name of the composite resource to the requirement.
	meta.SetExternalName(rq, meta.GetExternalName(cp))
	return errors.Wrap(a.client.Update(ctx, rq), errUpdateRequirement)
}
