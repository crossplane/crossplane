/*
Copyright 2022 The Crossplane Authors.

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

package environment

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	v1alpha1 "github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
)

const (
	errFmtReferenceEnvironmentConfig   = "failed to build reference at index %d"
	errFmtResolveLabelValue            = "failed to resolve value for label at index %d"
	errListEnvironmentConfigs          = "failed to list environments"
	errListEnvironmentConfigsNoResult  = "no EnvironmentConfig found that matches labels"
	errFmtInvalidEnvironmentSourceType = "invalid source type '%s'"

	errFmtInvalidLabelMatcherType = "invalid label matcher type '%s'"
	errFmtRequiredField           = "%s is required by type %s"
)

// NewNoopEnvironmentSelector creates a new NoopEnvironmentSelector.
func NewNoopEnvironmentSelector() *NoopEnvironmentSelector {
	return &NoopEnvironmentSelector{}
}

// A NoopEnvironmentSelector always returns nil on Fetch().
type NoopEnvironmentSelector struct{}

// SelectEnvironment always returns nil.
func (s *NoopEnvironmentSelector) SelectEnvironment(_ context.Context, _ resource.Composite, _ *v1.Composition) error {
	return nil
}

// NewAPIEnvironmentSelector creates a new APIEnvironmentSelector
func NewAPIEnvironmentSelector(kube client.Client) *APIEnvironmentSelector {
	return &APIEnvironmentSelector{
		kube: kube,
	}
}

// APIEnvironmentSelector selects an environment using a kube client.
type APIEnvironmentSelector struct {
	kube client.Client
}

// SelectEnvironment for cr using the configuration defined in comp.
// The computed list of EnvironmentConfig references will be stored in cr.
func (s *APIEnvironmentSelector) SelectEnvironment(ctx context.Context, cr resource.Composite, comp *v1.Composition) error {
	// noop if EnvironmentConfig references are already computed
	if len(cr.GetEnvironmentConfigReferences()) > 0 {
		return nil
	}
	if comp.Spec.Environment == nil || comp.Spec.Environment.EnvironmentConfigs == nil {
		return nil
	}

	refs := make([]corev1.ObjectReference, len(comp.Spec.Environment.EnvironmentConfigs))
	for i, src := range comp.Spec.Environment.EnvironmentConfigs {
		switch src.Type {
		case v1.EnvironmentSourceTypeReference:
			refs[i] = s.buildEnvironmentConfigRefFromRef(src.Ref)
		case v1.EnvironmentSourceTypeSelector:
			r, err := s.buildEnvironmentConfigRefFromSelector(ctx, cr, src.Selector)
			if err != nil {
				return errors.Wrapf(err, errFmtReferenceEnvironmentConfig, i)
			}
			refs[i] = r
		default:
			return errors.Errorf(errFmtInvalidEnvironmentSourceType, string(src.Type))
		}
	}
	cr.SetEnvironmentConfigReferences(refs)
	return nil
}

func (s *APIEnvironmentSelector) buildEnvironmentConfigRefFromRef(ref *v1.EnvironmentSourceReference) corev1.ObjectReference {
	return corev1.ObjectReference{
		Name:       ref.Name,
		Kind:       v1alpha1.EnvironmentConfigKind,
		APIVersion: v1alpha1.SchemeGroupVersion.String(),
	}
}

func (s *APIEnvironmentSelector) buildEnvironmentConfigRefFromSelector(ctx context.Context, cr resource.Composite, selector *v1.EnvironmentSourceSelector) (corev1.ObjectReference, error) {
	matchLabels := make(client.MatchingLabels, len(selector.MatchLabels))
	for i, m := range selector.MatchLabels {
		val, err := ResolveLabelValue(m, cr)
		if err != nil {
			return corev1.ObjectReference{}, errors.Wrapf(err, errFmtResolveLabelValue, i)
		}
		matchLabels[m.Key] = val
	}
	res := &v1alpha1.EnvironmentConfigList{}
	if err := s.kube.List(ctx, res, matchLabels); err != nil {
		return corev1.ObjectReference{}, errors.Wrap(err, errListEnvironmentConfigs)
	}
	if len(res.Items) == 0 {
		return corev1.ObjectReference{}, errors.New(errListEnvironmentConfigsNoResult)
	}
	envConfig := res.Items[0]
	return corev1.ObjectReference{
		Name:       envConfig.Name,
		Kind:       v1alpha1.EnvironmentConfigKind,
		APIVersion: v1alpha1.SchemeGroupVersion.String(),
	}, nil
}

// ResolveLabelValue from a EnvironmentSourceSelectorLabelMatcher and an Object.
func ResolveLabelValue(m v1.EnvironmentSourceSelectorLabelMatcher, cp runtime.Object) (string, error) {
	switch m.Type {
	case v1.EnvironmentSourceSelectorLabelMatcherTypeValue:
		return resolveLabelValueFromLiteral(m)
	case v1.EnvironmentSourceSelectorLabelMatcherTypeFromCompositeFieldPath:
		return resolveLabelValueFromCompositeFieldPath(m, cp)
	}
	return "", errors.Errorf(errFmtInvalidLabelMatcherType, string(m.Type))
}

func resolveLabelValueFromLiteral(m v1.EnvironmentSourceSelectorLabelMatcher) (string, error) {
	if m.Value == nil {
		return "", errors.Errorf(errFmtRequiredField, "value", string(v1.EnvironmentSourceSelectorLabelMatcherTypeValue))
	}
	return *m.Value, nil
}

func resolveLabelValueFromCompositeFieldPath(m v1.EnvironmentSourceSelectorLabelMatcher, cp runtime.Object) (string, error) {
	if m.ValueFromFieldPath == nil {
		return "", errors.Errorf(errFmtRequiredField, "valueFromFieldPath", string(v1.EnvironmentSourceSelectorLabelMatcherTypeValue))
	}

	fromMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cp)
	if err != nil {
		return "", err
	}

	in, err := fieldpath.Pave(fromMap).GetValue(*m.ValueFromFieldPath)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%v", in), nil
}
