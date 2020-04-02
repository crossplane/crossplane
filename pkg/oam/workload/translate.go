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

package workload

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/crossplane/crossplane-runtime/pkg/resource"

	oamv1alpha2 "github.com/crossplane/crossplane/apis/oam/v1alpha2"
	workloadv1alpha1 "github.com/crossplane/crossplane/apis/workload/v1alpha1"
)

const (
	errWrapInKubeApp = "unable to wrap objects in KubernetesApplication"
)

var (
	serviceKind       = reflect.TypeOf(corev1.Service{}).Name()
	serviceAPIVersion = corev1.SchemeGroupVersion.String()
)

// LabelKey is the label applied to translated workload objects.
const LabelKey = "workload.oam.crossplane.io"

// KubeAppWrapper wraps a set of translated objects in a KubernetesApplication.
func KubeAppWrapper(ctx context.Context, w resource.Workload, objs []resource.Object) ([]resource.Object, error) {
	if objs == nil {
		return nil, nil
	}

	app := &workloadv1alpha1.KubernetesApplication{}

	for _, o := range objs {
		karName := fmt.Sprintf("%s-%s", o.GetName(), strings.ToLower(o.GetObjectKind().GroupVersionKind().Kind))

		// NOTE(hasheddan): this updates the Deployment's Secret reference by
		// prepending the KubernetesApplicationResource name and returns the
		// Secrets that need to be added to the
		// KubernetesApplicationResourceTemplate.
		secrets := secretsForCWDeployment(w, o, karName)
		b, err := json.Marshal(o)
		if err != nil {
			return nil, errors.Wrap(err, errWrapInKubeApp)
		}

		kart := workloadv1alpha1.KubernetesApplicationResourceTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name: karName,
				Labels: map[string]string{
					LabelKey: string(w.GetUID()),
				},
			},
			Spec: workloadv1alpha1.KubernetesApplicationResourceSpec{
				Secrets:  secrets,
				Template: runtime.RawExtension{Raw: b},
			},
		}

		app.Spec.ResourceTemplates = append(app.Spec.ResourceTemplates, kart)
	}

	app.SetName(w.GetName())

	app.Spec.ResourceSelector = &metav1.LabelSelector{
		MatchLabels: map[string]string{
			LabelKey: string(w.GetUID()),
		},
	}

	return []resource.Object{app}, nil
}

// ServiceInjector adds a Service object for the first Port on the first
// Container for the first Deployment observed in a workload translation.
func ServiceInjector(ctx context.Context, w resource.Workload, objs []resource.Object) ([]resource.Object, error) {
	if objs == nil {
		return nil, nil
	}

	for _, o := range objs {
		d, ok := o.(*appsv1.Deployment)
		if !ok {
			continue
		}

		// We don't add a Service if there are no containers for the Deployment.
		// This should never happen in practice.
		if len(d.Spec.Template.Spec.Containers) < 1 {
			continue
		}

		s := &corev1.Service{
			TypeMeta: metav1.TypeMeta{
				Kind:       serviceKind,
				APIVersion: serviceAPIVersion,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      d.GetName(),
				Namespace: d.GetNamespace(),
				Labels: map[string]string{
					LabelKey: string(w.GetUID()),
				},
			},
			Spec: corev1.ServiceSpec{
				Selector: d.Spec.Selector.MatchLabels,
				Ports:    []corev1.ServicePort{},
				Type:     corev1.ServiceTypeLoadBalancer,
			},
		}

		// We only add a single Service for the Deployment, even if multiple
		// ports or no ports are defined on the first container. This is to
		// exclude the need for implementing garbage collection in the
		// short-term in the case that ports are modified after creation.
		if len(d.Spec.Template.Spec.Containers[0].Ports) > 0 {
			s.Spec.Ports = []corev1.ServicePort{
				{
					Name:       d.GetName(),
					Port:       d.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort,
					TargetPort: intstr.FromInt(int(d.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort)),
				},
			}
		}
		objs = append(objs, s)
		break
	}
	return objs, nil
}

// NOTE(hasheddan): secretsForCWDeployment is a temporary solution for adding
// Secrets used by the Deployment rendered from a ContainerizedWorkload to the
// KubernetesApplicationResourceTemplate it is wrapped in. It also updates the
// Deployment's Secret reference to the name that the
// KubernetesApplicationResource controller will propagate it with.
func secretsForCWDeployment(w resource.Workload, o resource.Object, prefix string) []corev1.LocalObjectReference {
	if _, ok := w.(*oamv1alpha2.ContainerizedWorkload); !ok {
		return nil
	}

	d, ok := o.(*appsv1.Deployment)
	if !ok {
		return nil
	}

	secretRefs := map[string]corev1.LocalObjectReference{}
	for _, c := range d.Spec.Template.Spec.Containers {
		for _, e := range c.Env {
			if e.ValueFrom != nil && e.ValueFrom.SecretKeyRef != nil {
				secretRefs[e.ValueFrom.SecretKeyRef.LocalObjectReference.Name] = e.ValueFrom.SecretKeyRef.LocalObjectReference

				// NOTE(hasheddan): we must update the name of the Secret in the
				// Deployment because the KubernetesApplication controller will
				// propagate the Secret to the remote cluster prefixed with the
				// KubernetesApplicationResource name.
				e.ValueFrom.SecretKeyRef.LocalObjectReference.Name = fmt.Sprintf("%s-%s", prefix, e.ValueFrom.SecretKeyRef.LocalObjectReference.Name)
			}
		}
	}

	secrets := make([]corev1.LocalObjectReference, len(secretRefs))
	count := 0
	for _, s := range secretRefs {
		secrets[count] = s
		count++
	}

	return secrets
}
