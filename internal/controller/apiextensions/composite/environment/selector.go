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
	"reflect"
	"sort"

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
	errFmtInvalidLabelMatcherType      = "invalid label matcher type '%s'"
	errFmtRequiredField                = "%s is required by type %s"
	errUnknownSelectorMode             = "unknown mode '%s'"
	errSortNotMatchingTypes            = "not matching types: %T : %T"
	errSortUnknownType                 = "unexpected type %T"
	errFoundMultipleInSingleMode       = "only 1 EnvironmentConfig can be selected in Single mode, found: %d"
)

// NewNoopEnvironmentSelector creates a new NoopEnvironmentSelector.
func NewNoopEnvironmentSelector() *NoopEnvironmentSelector {
	return &NoopEnvironmentSelector{}
}

// A NoopEnvironmentSelector always returns nil on Fetch().
type NoopEnvironmentSelector struct{}

// SelectEnvironment always returns nil.
func (s *NoopEnvironmentSelector) SelectEnvironment(_ context.Context, _ resource.Composite, _ *v1.CompositionRevision) error {
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
func (s *APIEnvironmentSelector) SelectEnvironment(ctx context.Context, cr resource.Composite, rev *v1.CompositionRevision) error {

	if !rev.Spec.Environment.ShouldResolve(cr.GetEnvironmentConfigReferences()) {
		return nil
	}

	refs := make([]corev1.ObjectReference, len(rev.Spec.Environment.EnvironmentConfigs))
	idx := 0
	for i, src := range rev.Spec.Environment.EnvironmentConfigs {
		switch src.Type {
		case v1.EnvironmentSourceTypeReference:
			refs = append(
				refs[:idx],
				s.buildEnvironmentConfigRefFromRef(src.Ref),
			)
			idx++
		case v1.EnvironmentSourceTypeSelector:

			ec, err := s.lookUpConfigs(ctx, cr, src.Selector.MatchLabels)
			if err != nil {
				return errors.Wrapf(err, errFmtReferenceEnvironmentConfig, i)
			}
			r, err := s.buildEnvironmentConfigRefFromSelector(ec, src.Selector)
			if err != nil {
				return errors.Wrapf(err, errFmtReferenceEnvironmentConfig, i)
			}
			refs = append(refs[:idx], r...)
			idx += len(r)
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

func (s *APIEnvironmentSelector) lookUpConfigs(ctx context.Context, cr resource.Composite, ml []v1.EnvironmentSourceSelectorLabelMatcher) (*v1alpha1.EnvironmentConfigList, error) {
	matchLabels := make(client.MatchingLabels, len(ml))
	for i, m := range ml {
		val, err := ResolveLabelValue(m, cr)
		if err != nil {
			return nil, errors.Wrapf(err, errFmtResolveLabelValue, i)
		}
		matchLabels[m.Key] = val
	}
	res := &v1alpha1.EnvironmentConfigList{}
	if err := s.kube.List(ctx, res, matchLabels); err != nil {
		return nil, errors.Wrap(err, errListEnvironmentConfigs)
	}
	return res, nil
}

func (s *APIEnvironmentSelector) buildEnvironmentConfigRefFromSelector(cl *v1alpha1.EnvironmentConfigList, selector *v1.EnvironmentSourceSelector) ([]corev1.ObjectReference, error) {

	ec := make([]v1alpha1.EnvironmentConfig, 0)

	switch {
	case len(cl.Items) == 0:
		return []corev1.ObjectReference{}, nil

	case selector.Mode == v1.EnvironmentSourceSelectorSingleMode:

		if len(cl.Items) != 1 {
			return []corev1.ObjectReference{}, errors.Errorf(errFoundMultipleInSingleMode, len(cl.Items))
		}
		ec = append(ec, cl.Items[0])

	case selector.Mode == v1.EnvironmentSourceSelectorMultiMode:
		err := sortConfigs(cl.Items, selector.SortByFieldPath)
		if err != nil {
			return []corev1.ObjectReference{}, err
		}

		if selector.MaxMatch == nil {
			ec = append(ec, cl.Items...)
		} else {
			ec = append(ec, cl.Items[:*selector.MaxMatch]...)
		}

	default:
		return []corev1.ObjectReference{}, errors.Errorf(errUnknownSelectorMode, selector.Mode)
	}

	envConfigs := make([]corev1.ObjectReference, len(ec))
	for i, v := range ec {
		envConfigs[i] = corev1.ObjectReference{
			Name:       v.Name,
			Kind:       v1alpha1.EnvironmentConfigKind,
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
		}
	}

	return envConfigs, nil
}

type sortPair struct {
	ec  v1alpha1.EnvironmentConfig
	obj map[string]interface{}
}

//nolint:gocyclo // tbd
func sortConfigs(ec []v1alpha1.EnvironmentConfig, f string) error {

	var err error

	p := make([]sortPair, len(ec))

	for i := 0; i < len(ec); i++ {
		m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&ec[i])
		if err != nil {
			return err
		}
		p[i] = sortPair{
			ec:  ec[i],
			obj: m,
		}
	}

	sort.Slice(p, func(i, j int) bool {
		if err != nil {
			return false
		}
		v1, e := fieldpath.Pave(p[i].obj).GetValue(f)
		if e != nil {
			err = e
			return false
		}
		v2, e := fieldpath.Pave(p[j].obj).GetValue(f)
		if err != nil {
			err = e
			return false
		}

		vt1 := reflect.TypeOf(v1).Kind()
		vt2 := reflect.TypeOf(v2).Kind()
		switch {
		case vt1 == reflect.Float64 && vt2 == reflect.Float64:
			return v1.(float64) < v2.(float64)
		case vt1 == reflect.Int64 && vt2 == reflect.Int64:
			return v1.(int64) < v2.(int64)
		case vt1 == reflect.String && vt2 == reflect.String:
			return v1.(string) < v2.(string)
		case vt1 != vt2:
			err = errors.Errorf(errSortNotMatchingTypes, v1, v2)
		default:
			err = errors.Errorf(errSortUnknownType, v1)
		}
		return false
	})

	if err != nil {
		return err
	}
	for i := 0; i < len(ec); i++ {
		ec[i] = p[i].ec
	}
	return nil
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
