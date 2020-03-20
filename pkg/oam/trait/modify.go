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

package trait

import (
	"context"
	"encoding/json"
	"reflect"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/pkg/reconciler/oam/trait"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	workloadv1alpha1 "github.com/crossplane/crossplane/apis/workload/v1alpha1"
)

const (
	errNotKubeApp           = "object passed to KubernetesApplication accessor is not KubernetesApplication"
	errNoDeploymentForTrait = "no deployment found for trait in KubernetesApplication"
)

var (
	deploymentKind = reflect.TypeOf(appsv1.Deployment{}).Name()
)

// DeploymentFromKubeAppAccessor finds deployments in a KubernetesApplication
// and applies the supplied modifier function to them.
func DeploymentFromKubeAppAccessor(ctx context.Context, obj runtime.Object, t resource.Trait, m trait.ModifyFn) error {
	a, ok := obj.(*workloadv1alpha1.KubernetesApplication)
	if !ok {
		return errors.New(errNotKubeApp)
	}

	for i, r := range a.Spec.ResourceTemplates {
		template := &unstructured.Unstructured{}
		if err := json.Unmarshal(r.Spec.Template.Raw, template); err != nil {
			return err
		}
		if template.GroupVersionKind().Kind == deploymentKind {
			d := &appsv1.Deployment{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(template.UnstructuredContent(), d); err != nil {
				return err
			}
			if err := m(ctx, d, t); err != nil {
				return err
			}
			deployment, err := json.Marshal(d)
			if err != nil {
				return err
			}
			a.Spec.ResourceTemplates[i].Spec.Template = runtime.RawExtension{Raw: deployment}
			return nil
		}
	}

	return errors.New(errNoDeploymentForTrait)
}
