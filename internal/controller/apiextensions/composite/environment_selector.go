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

package composite

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
	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
)

const (
	errFmtReferenceEnvironmentConfig   = "failed to build reference at index %d"
	errFmtResolveLabelValue            = "failed to resolve value for label at index %d"
	errListEnvironmentConfigs          = "failed to list environments"
	errFmtSelectorNotEnoughResults     = "expected at least %d EnvironmentConfig(s) with matching labels, found: %d"
	errFmtInvalidEnvironmentSourceType = "invalid source type '%s'"
	errFmtInvalidLabelMatcherType      = "invalid label matcher type '%s'"
	errFmtUnknownSelectorMode          = "unknown mode '%s'"
	errFmtSortNotMatchingTypes         = "not matching types, got %[1]v (%[1]T), expected %[2]v"
	errFmtSortUnknownType              = "unexpected type %T"
	errFmtFoundMultipleInSingleMode    = "only 1 EnvironmentConfig can be selected in Single mode, found: %d"
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

	refs := make([]corev1.ObjectReference, 0, len(rev.Spec.Environment.EnvironmentConfigs))
	for i, src := range rev.Spec.Environment.EnvironmentConfigs {
		switch src.Type {
		case v1.EnvironmentSourceTypeReference:
			refs = append(
				refs,
				s.buildEnvironmentConfigRefFromRef(src.Ref),
			)
		case v1.EnvironmentSourceTypeSelector:
			ec, err := s.lookUpConfigs(ctx, cr, src.Selector.MatchLabels)
			if err != nil {
				return errors.Wrapf(err, errFmtReferenceEnvironmentConfig, i)
			}
			r, err := s.buildEnvironmentConfigRefFromSelector(ec, src.Selector)
			if err != nil {
				return errors.Wrapf(err, errFmtReferenceEnvironmentConfig, i)
			}
			refs = append(refs, r...)
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
	res := &v1alpha1.EnvironmentConfigList{}
	matchLabels := make(client.MatchingLabels, len(ml))
	for i, m := range ml {
		val, err := ResolveLabelValue(m, cr)
		if err != nil {
			if fieldpath.IsNotFound(err) && m.FromFieldPathIsOptional() {
				continue
			}
			return nil, errors.Wrapf(err, errFmtResolveLabelValue, i)
		}
		matchLabels[m.Key] = val
	}
	if len(matchLabels) == 0 {
		return res, nil
	}
	if err := s.kube.List(ctx, res, matchLabels); err != nil {
		return nil, errors.Wrap(err, errListEnvironmentConfigs)
	}
	return res, nil
}

func (s *APIEnvironmentSelector) buildEnvironmentConfigRefFromSelector(cl *v1alpha1.EnvironmentConfigList, selector *v1.EnvironmentSourceSelector) ([]corev1.ObjectReference, error) { //nolint:gocyclo // TODO: refactor
	ec := make([]v1alpha1.EnvironmentConfig, 0)

	if cl == nil {
		return []corev1.ObjectReference{}, nil
	}

	switch selector.Mode {
	case v1.EnvironmentSourceSelectorSingleMode:
		switch len(cl.Items) {
		case 1:
			ec = append(ec, cl.Items[0])
		default:
			return nil, errors.Errorf(errFmtFoundMultipleInSingleMode, len(cl.Items))
		}
	case v1.EnvironmentSourceSelectorMultiMode:

		if selector.MinMatch != nil && len(cl.Items) < int(*selector.MinMatch) {
			return nil, errors.Errorf(errFmtSelectorNotEnoughResults, *selector.MinMatch, len(cl.Items))
		}

		err := sortConfigs(cl.Items, selector.SortByFieldPath)
		if err != nil {
			return nil, err
		}

		if selector.MaxMatch != nil && len(cl.Items) > int(*selector.MaxMatch) {
			ec = append(ec, cl.Items[:*selector.MaxMatch]...)
			break
		}
		ec = append(ec, cl.Items...)

	default:
		// should never happen
		return nil, errors.Errorf(errFmtUnknownSelectorMode, selector.Mode)
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

func sortConfigs(ec []v1alpha1.EnvironmentConfig, f string) error { //nolint:gocyclo // TODO(phisco): refactor
	p := make([]struct {
		ec  v1alpha1.EnvironmentConfig
		val any
	}, len(ec))

	var valsKind reflect.Kind
	for i := 0; i < len(ec); i++ {
		m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&ec[i])
		if err != nil {
			return err
		}

		val, err := fieldpath.Pave(m).GetValue(f)
		if err != nil {
			return err
		}

		var vt reflect.Kind
		if val != nil {
			vt = reflect.TypeOf(val).Kind()
		}

		// check only vt1 as vt1 == vt2
		switch vt { //nolint:exhaustive // we only support these types
		case reflect.String, reflect.Int64, reflect.Float64:
			// ok
		default:
			return errors.Errorf(errFmtSortUnknownType, val)
		}

		if i == 0 {
			valsKind = vt
		} else if vt != valsKind {
			// compare with previous values' kind
			return errors.Errorf(errFmtSortNotMatchingTypes, val, valsKind)
		}

		p[i].ec = ec[i]
		p[i].val = val
	}

	var err error
	sort.Slice(p, func(i, j int) bool {
		vali, valj := p[i].val, p[j].val
		switch valsKind { //nolint:exhaustive // we only support these types
		case reflect.Float64:
			return vali.(float64) < valj.(float64)
		case reflect.Int64:
			return vali.(int64) < valj.(int64)
		case reflect.String:
			return vali.(string) < valj.(string)
		default:
			// should never happen
			err = errors.Errorf(errFmtSortUnknownType, valsKind)
			return false
		}
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
