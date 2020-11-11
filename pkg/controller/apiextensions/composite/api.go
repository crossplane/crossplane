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

package composite

import (
	"context"
	"math/rand"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane/crossplane/apis/apiextensions/v1beta1"
)

// Error strings.
const (
	errApplySecret = "cannot apply connection secret"

	errNoCompatibleComposition  = "no compatible composition has been found"
	errListCompositions         = "cannot list compositions"
	errUpdateComposite          = "cannot update composite resource"
	errCompositionNotCompatible = "referenced composition is not compatible with this composite resource"
	errGetXRD                   = "cannot get composite resource definition"
)

// Event reasons.
const (
	reasonCompositionSelection event.Reason = "CompositionSelection"
)

// APIFilteredSecretPublisher publishes ConnectionDetails content after filtering
// it through a set of permitted keys.
type APIFilteredSecretPublisher struct {
	client resource.Applicator
	filter []string
}

// NewAPIFilteredSecretPublisher returns a ConnectionPublisher that only
// publishes connection secret keys that are included in the supplied filter.
func NewAPIFilteredSecretPublisher(c client.Client, filter []string) *APIFilteredSecretPublisher {
	return &APIFilteredSecretPublisher{client: resource.NewAPIPatchingApplicator(c), filter: filter}
}

// PublishConnection publishes the supplied ConnectionDetails to the Secret
// referenced in the resource.
func (a *APIFilteredSecretPublisher) PublishConnection(ctx context.Context, o resource.ConnectionSecretOwner, c managed.ConnectionDetails) (bool, error) {
	// This resource does not want to expose a connection secret.
	if o.GetWriteConnectionSecretToReference() == nil {
		return false, nil
	}

	s := resource.ConnectionSecretFor(o, o.GetObjectKind().GroupVersionKind())
	m := map[string]bool{}
	// TODO(muvaf): Should empty filter allow all keys?
	for _, key := range a.filter {
		m[key] = true
	}
	for key, val := range c {
		if _, ok := m[key]; ok {
			s.Data[key] = val
		}
	}

	err := a.client.Apply(ctx, s,
		resource.ConnectionSecretMustBeControllableBy(o.GetUID()),
		resource.AllowUpdateIf(func(current, desired runtime.Object) bool {
			// We consider the update to be a no-op and don't allow it if the
			// current and existing secret data are identical.
			return !cmp.Equal(current.(*corev1.Secret).Data, desired.(*corev1.Secret).Data)
		}),
	)
	if resource.IsNotAllowed(err) {
		// The update was not allowed because it was a no-op.
		return false, nil
	}
	if err != nil {
		return false, errors.Wrap(err, errApplySecret)
	}

	return true, nil
}

// UnpublishConnection is no-op since PublishConnection only creates resources
// that will be garbage collected by Kubernetes when the managed resource is
// deleted.
func (a *APIFilteredSecretPublisher) UnpublishConnection(_ context.Context, _ resource.ConnectionSecretOwner, _ managed.ConnectionDetails) error {
	return nil
}

// NewCompositionSelectorChain returns a new CompositionSelectorChain.
func NewCompositionSelectorChain(list ...CompositionSelector) *CompositionSelectorChain {
	return &CompositionSelectorChain{list: list}
}

// CompositionSelectorChain calls the given list of CompositionSelectors in order.
type CompositionSelectorChain struct {
	list []CompositionSelector
}

// SelectComposition calls all SelectComposition functions of CompositionSelectors
// in the list.
func (r *CompositionSelectorChain) SelectComposition(ctx context.Context, cp resource.Composite) error {
	for _, cs := range r.list {
		if err := cs.SelectComposition(ctx, cp); err != nil {
			return err
		}
	}
	return nil
}

// NewAPILabelSelectorResolver returns a SelectorResolver for composite resource.
func NewAPILabelSelectorResolver(c client.Client) *APILabelSelectorResolver {
	return &APILabelSelectorResolver{client: c}
}

// APILabelSelectorResolver is used to resolve the composition selector on the instance
// to composition reference.
type APILabelSelectorResolver struct {
	client client.Client
}

// SelectComposition resolves selector to a reference if it doesn't exist.
func (r *APILabelSelectorResolver) SelectComposition(ctx context.Context, cp resource.Composite) error {
	// TODO(muvaf): need to block the deletion of composition via finalizer once
	// it's selected since it's integral to this resource.
	// TODO(muvaf): We don't rely on UID in practice. It should not be there
	// because it will make confusion if the resource is backed up and restored
	// to another cluster
	if cp.GetCompositionReference() != nil {
		return nil
	}
	labels := map[string]string{}
	sel := cp.GetCompositionSelector()
	if sel != nil {
		labels = sel.MatchLabels
	}
	list := &v1beta1.CompositionList{}
	if err := r.client.List(ctx, list, client.MatchingLabels(labels)); err != nil {
		return errors.Wrap(err, errListCompositions)
	}

	candidates := make([]string, 0, len(list.Items))
	v, k := cp.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()

	for _, comp := range list.Items {
		if comp.Spec.CompositeTypeRef.APIVersion == v && comp.Spec.CompositeTypeRef.Kind == k {
			// This composition is compatible with our composite resource.
			candidates = append(candidates, comp.Name)
		}
	}

	if len(candidates) == 0 {
		return errors.New(errNoCompatibleComposition)
	}

	// We don't need this choice to be cryptographically random.
	random := rand.New(rand.NewSource(time.Now().UnixNano())) // nolint:gosec
	selected := candidates[random.Intn(len(candidates))]
	cp.SetCompositionReference(&corev1.ObjectReference{Name: selected})
	return errors.Wrap(r.client.Update(ctx, cp), errUpdateComposite)
}

// NewAPIDefaultCompositionSelector returns a APIDefaultCompositionSelector.
func NewAPIDefaultCompositionSelector(c client.Client, ref corev1.ObjectReference, r event.Recorder) *APIDefaultCompositionSelector {
	return &APIDefaultCompositionSelector{client: c, defRef: ref, recorder: r}
}

// APIDefaultCompositionSelector selects the default composition referenced in
// the definition of the resource if neither a reference nor selector is given
// in composite resource.
type APIDefaultCompositionSelector struct {
	client   client.Client
	defRef   corev1.ObjectReference
	recorder event.Recorder
}

// SelectComposition selects the default compositionif neither a reference nor
// selector is given in composite resource.
func (s *APIDefaultCompositionSelector) SelectComposition(ctx context.Context, cp resource.Composite) error {
	if cp.GetCompositionReference() != nil || cp.GetCompositionSelector() != nil {
		return nil
	}
	def := &v1beta1.CompositeResourceDefinition{}
	if err := s.client.Get(ctx, meta.NamespacedNameOf(&s.defRef), def); err != nil {
		return errors.Wrap(err, errGetXRD)
	}
	if def.Spec.DefaultCompositionRef == nil {
		return nil
	}
	cp.SetCompositionReference(&corev1.ObjectReference{Name: def.Spec.DefaultCompositionRef.Name})
	s.recorder.Event(cp, event.Normal(reasonCompositionSelection, "Default composition has been selected"))
	return nil
}

// NewEnforcedCompositionSelector returns a EnforcedCompositionSelector.
func NewEnforcedCompositionSelector(def v1beta1.CompositeResourceDefinition, r event.Recorder) *EnforcedCompositionSelector {
	return &EnforcedCompositionSelector{def: def, recorder: r}
}

// EnforcedCompositionSelector , if it's given, selects the enforced composition
// on the definition for all composite instances.
type EnforcedCompositionSelector struct {
	def      v1beta1.CompositeResourceDefinition
	recorder event.Recorder
}

// SelectComposition selects the enforced composition if it's given in definition.
func (s *EnforcedCompositionSelector) SelectComposition(_ context.Context, cp resource.Composite) error {
	// We don't need to fetch the CompositeResourceDefinition at every reconcile
	// because enforced composition ref is immutable as opposed to default
	// composition ref.
	if s.def.Spec.EnforcedCompositionRef == nil {
		return nil
	}
	// If the composition is already chosen, we don't need to check for compatibility
	// as its target type reference is immutable.
	if cp.GetCompositionReference() != nil && cp.GetCompositionReference().Name == s.def.Spec.EnforcedCompositionRef.Name {
		return nil
	}
	cp.SetCompositionReference(&corev1.ObjectReference{Name: s.def.Spec.EnforcedCompositionRef.Name})
	s.recorder.Event(cp, event.Normal(reasonCompositionSelection, "Enforced composition has been selected"))
	return nil
}

// NewConfiguratorChain returns a new *ConfiguratorChain.
func NewConfiguratorChain(l ...Configurator) *ConfiguratorChain {
	return &ConfiguratorChain{list: l}
}

// ConfiguratorChain executes the Configurators in given order.
type ConfiguratorChain struct {
	list []Configurator
}

// Configure calls Configure function of every Configurator in the list.
func (cc *ConfiguratorChain) Configure(ctx context.Context, cp resource.Composite, comp *v1beta1.Composition) error {
	for _, c := range cc.list {
		if err := c.Configure(ctx, cp, comp); err != nil {
			return err
		}
	}
	return nil
}

// NewAPIConfigurator returns a Configurator that configures a
// composite resource using its composition.
func NewAPIConfigurator(c client.Client) *APIConfigurator {
	return &APIConfigurator{client: c}
}

// An APIConfigurator configures a composite resource using its
// composition.
type APIConfigurator struct {
	client client.Client
}

// Configure any required fields that were omitted from the composite resource
// by copying them from its composition.
func (c *APIConfigurator) Configure(ctx context.Context, cp resource.Composite, comp *v1beta1.Composition) error {
	apiVersion, kind := cp.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()
	if comp.Spec.CompositeTypeRef.APIVersion != apiVersion || comp.Spec.CompositeTypeRef.Kind != kind {
		return errors.New(errCompositionNotCompatible)
	}

	if cp.GetWriteConnectionSecretToReference() != nil || comp.Spec.WriteConnectionSecretsToNamespace == nil {
		return nil
	}

	cp.SetWriteConnectionSecretToReference(&runtimev1alpha1.SecretReference{
		Name:      string(cp.GetUID()),
		Namespace: *comp.Spec.WriteConnectionSecretsToNamespace,
	})

	return errors.Wrap(c.client.Update(ctx, cp), errUpdateComposite)
}

// NewAPINamingConfigurator returns a Configurator that sets the root name prefixKu
// to its own name if it is not already set.
func NewAPINamingConfigurator(c client.Client) *APINamingConfigurator {
	return &APINamingConfigurator{client: c}
}

// An APINamingConfigurator sets the root name prefix to its own name if it is not
// already set.
type APINamingConfigurator struct {
	client client.Client
}

// Configure the supplied composite resource's root name prefix.
func (c *APINamingConfigurator) Configure(ctx context.Context, cp resource.Composite, _ *v1beta1.Composition) error {
	if cp.GetLabels()[LabelKeyNamePrefixForComposed] != "" {
		return nil
	}
	meta.AddLabels(cp, map[string]string{LabelKeyNamePrefixForComposed: cp.GetName()})
	return errors.Wrap(c.client.Update(ctx, cp), errUpdateComposite)
}
