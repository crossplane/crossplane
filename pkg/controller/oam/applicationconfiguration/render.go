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

package applicationconfiguration

import (
	"context"
	"encoding/json"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"

	"github.com/crossplane/crossplane/apis/oam/v1alpha2"
)

// Render error strings.
const (
	errUnmarshalWorkload = "cannot unmarshal workload"
	errUnmarshalTrait    = "cannot unmarshal trait"
)

// Render error format strings.
const (
	errFmtGetComponent     = "cannot get component %q"
	errFmtResolveParams    = "cannot resolve parameter values for component %q"
	errFmtRenderWorkload   = "cannot render workload for component %q"
	errFmtRenderTrait      = "cannot render trait for component %q"
	errFmtSetParam         = "cannot set parameter %q"
	errFmtUnsupportedParam = "unsupported parameter %q"
	errFmtRequiredParam    = "required parameter %q not specified"
)

// A ComponentRenderer renders an ApplicationConfiguration's Components into
// workloads and traits.
type ComponentRenderer interface {
	Render(ctx context.Context, ac *v1alpha2.ApplicationConfiguration) ([]Workload, error)
}

// A ComponentRenderFn renders an ApplicationConfiguration's Components into
// workloads and traits.
type ComponentRenderFn func(ctx context.Context, ac *v1alpha2.ApplicationConfiguration) ([]Workload, error)

// Render an ApplicationConfiguration's Components into workloads and traits.
func (fn ComponentRenderFn) Render(ctx context.Context, ac *v1alpha2.ApplicationConfiguration) ([]Workload, error) {
	return fn(ctx, ac)
}

type components struct {
	client   client.Reader
	params   ParameterResolver
	workload ResourceRenderer
	trait    ResourceRenderer
}

func (r *components) Render(ctx context.Context, ac *v1alpha2.ApplicationConfiguration) ([]Workload, error) {
	workloads := make([]Workload, len(ac.Spec.Components))
	for i, acc := range ac.Spec.Components {

		c := &v1alpha2.Component{}
		nn := types.NamespacedName{Namespace: ac.GetNamespace(), Name: acc.ComponentName}
		if err := r.client.Get(ctx, nn, c); err != nil {
			return nil, errors.Wrapf(err, errFmtGetComponent, acc.ComponentName)
		}

		p, err := r.params.Resolve(c.Spec.Parameters, acc.ParameterValues)
		if err != nil {
			return nil, errors.Wrapf(err, errFmtResolveParams, acc.ComponentName)
		}

		w, err := r.workload.Render(c.Spec.Workload.Raw, p...)
		if err != nil {
			return nil, errors.Wrapf(err, errFmtRenderWorkload, acc.ComponentName)
		}

		ref := metav1.NewControllerRef(ac, v1alpha2.ApplicationConfigurationGroupVersionKind)
		w.SetOwnerReferences([]metav1.OwnerReference{*ref})
		w.SetNamespace(ac.GetNamespace())

		traits := make([]unstructured.Unstructured, len(acc.Traits))
		for i, ct := range acc.Traits {
			t, err := r.trait.Render(ct.Trait.Raw)
			if err != nil {
				return nil, errors.Wrapf(err, errFmtRenderTrait, acc.ComponentName)
			}

			t.SetOwnerReferences([]metav1.OwnerReference{*ref})
			t.SetNamespace(ac.GetNamespace())

			traits[i] = *t
		}

		workloads[i] = Workload{ComponentName: acc.ComponentName, Workload: w, Traits: traits}
	}
	return workloads, nil
}

// A ResourceRenderer renders a Kubernetes-compliant YAML resource into an
// Unstructured object, optionally setting the supplied parameters.
type ResourceRenderer interface {
	Render(data []byte, p ...Parameter) (*unstructured.Unstructured, error)
}

// A ResourceRenderFn renders a Kubernetes-compliant YAML resource into an
// Unstructured object, optionally setting the supplied parameters.
type ResourceRenderFn func(data []byte, p ...Parameter) (*unstructured.Unstructured, error)

// Render the supplied Kubernetes YAML resource.
func (fn ResourceRenderFn) Render(data []byte, p ...Parameter) (*unstructured.Unstructured, error) {
	return fn(data, p...)
}

func renderWorkload(data []byte, p ...Parameter) (*unstructured.Unstructured, error) {
	// TODO(negz): Is there a better decoder to use here?
	w := &fieldpath.Paved{}
	if err := json.Unmarshal(data, w); err != nil {
		return nil, errors.Wrap(err, errUnmarshalWorkload)
	}

	for _, param := range p {
		for _, path := range param.FieldPaths {
			// TODO(negz): Infer parameter type from workload OpenAPI schema.
			switch param.Value.Type {
			case intstr.String:
				if err := w.SetString(path, param.Value.StrVal); err != nil {
					return nil, errors.Wrapf(err, errFmtSetParam, param.Name)
				}
			case intstr.Int:
				if err := w.SetNumber(path, float64(param.Value.IntVal)); err != nil {
					return nil, errors.Wrapf(err, errFmtSetParam, param.Name)
				}
			}
		}
	}

	return &unstructured.Unstructured{Object: w.UnstructuredContent()}, nil
}

func renderTrait(data []byte, _ ...Parameter) (*unstructured.Unstructured, error) {
	// TODO(negz): Is there a better decoder to use here?
	u := &unstructured.Unstructured{}
	if err := json.Unmarshal(data, u); err != nil {
		return nil, errors.Wrap(err, errUnmarshalTrait)
	}
	return u, nil
}

// A Parameter may be used to set the supplied paths to the supplied value.
type Parameter struct {
	// Name of this parameter.
	Name string

	// Value of this parameter.
	Value intstr.IntOrString

	// FieldPaths that should be set to this parameter's value.
	FieldPaths []string
}

// A ParameterResolver resolves the parameters accepted by a component and the
// parameter values supplied to a component into configured parameters.
type ParameterResolver interface {
	Resolve([]v1alpha2.ComponentParameter, []v1alpha2.ComponentParameterValue) ([]Parameter, error)
}

// A ParameterResolveFn resolves the parameters accepted by a component and the
// parameter values supplied to a component into configured parameters.
type ParameterResolveFn func([]v1alpha2.ComponentParameter, []v1alpha2.ComponentParameterValue) ([]Parameter, error)

// Resolve the supplied parameters.
func (fn ParameterResolveFn) Resolve(cp []v1alpha2.ComponentParameter, cpv []v1alpha2.ComponentParameterValue) ([]Parameter, error) {
	return fn(cp, cpv)
}

func resolve(cp []v1alpha2.ComponentParameter, cpv []v1alpha2.ComponentParameterValue) ([]Parameter, error) {
	supported := make(map[string]bool)
	for _, v := range cp {
		supported[v.Name] = true
	}

	set := make(map[string]*Parameter)
	for _, v := range cpv {
		if !supported[v.Name] {
			return nil, errors.Errorf(errFmtUnsupportedParam, v.Name)
		}
		set[v.Name] = &Parameter{Name: v.Name, Value: v.Value}
	}

	for _, p := range cp {
		_, ok := set[p.Name]
		if !ok && p.Required != nil && *p.Required {
			// This parameter is required, but not set.
			return nil, errors.Errorf(errFmtRequiredParam, p.Name)
		}
		if !ok {
			// This parameter is not required, and not set.
			continue
		}

		set[p.Name].FieldPaths = p.FieldPaths
	}

	params := make([]Parameter, 0, len(set))
	for _, p := range set {
		params = append(params, *p)
	}

	return params, nil
}
