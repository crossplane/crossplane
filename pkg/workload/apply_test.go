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
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	workloadv1alpha1 "github.com/crossplane/crossplane/apis/workload/v1alpha1"
)

var (
	workloadName = "test-workload"
	workloadUID  = "a-very-unique-identifier"

	replicas      = int32(3)
	containerName = "test-container"
	portName      = "test-port"
)

var (
	deploymentKind       = reflect.TypeOf(appsv1.Deployment{}).Name()
	deploymentAPIVersion = appsv1.SchemeGroupVersion.String()
)

type deploymentModifier func(*appsv1.Deployment)

func dmWithContainerPorts(ports ...int32) deploymentModifier {
	return func(d *appsv1.Deployment) {
		p := []corev1.ContainerPort{}
		for _, port := range ports {
			p = append(p, corev1.ContainerPort{
				Name:          portName,
				ContainerPort: port,
			})
		}
		d.Spec.Template.Spec.Containers = append(d.Spec.Template.Spec.Containers, corev1.Container{
			Name:  containerName,
			Ports: p,
		})
	}
}

func dmWithReplicas(r *int32) deploymentModifier {
	return func(d *appsv1.Deployment) {
		d.Spec.Replicas = r
	}
}

func deployment(mod ...deploymentModifier) *appsv1.Deployment {
	d := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       deploymentKind,
			APIVersion: deploymentAPIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              workloadName,
			CreationTimestamp: metav1.NewTime(time.Date(0, 0, 0, 0, 0, 0, 0, time.Local)),
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"test-label": workloadUID,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					CreationTimestamp: metav1.NewTime(time.Date(0, 0, 0, 0, 0, 0, 0, time.Local)),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{},
				},
			},
		},
	}

	for _, m := range mod {
		m(d)
	}

	return d
}

type kubeAppModifier func(*workloadv1alpha1.KubernetesApplication)

func kaWithTemplate(name string, o runtime.Object) kubeAppModifier {
	return func(a *workloadv1alpha1.KubernetesApplication) {
		b, _ := json.Marshal(o)
		a.Spec.ResourceTemplates = append(a.Spec.ResourceTemplates, workloadv1alpha1.KubernetesApplicationResourceTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name:              name,
				CreationTimestamp: metav1.NewTime(time.Date(0, 0, 0, 0, 0, 0, 0, time.Local)),
			},
			Spec: workloadv1alpha1.KubernetesApplicationResourceSpec{
				Template: runtime.RawExtension{Raw: b},
			},
		})
	}
}

func kubeApp(mod ...kubeAppModifier) *workloadv1alpha1.KubernetesApplication {
	a := &workloadv1alpha1.KubernetesApplication{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cool-kapp",
		},
	}

	for _, m := range mod {
		m(a)
	}

	return a
}

var _ resource.ApplyOption = KubeAppApplyOption()

func TestKubeAppApplyOption(t *testing.T) {
	type args struct {
		c runtime.Object
		d runtime.Object
	}

	type want struct {
		o   runtime.Object
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NotAKubernetesApplication": {
			reason: "An error should be returned if the object is not a KubernetesApplication",
			args: args{
				c: &corev1.Namespace{},
				d: &corev1.Namespace{},
			},
			want: want{
				o:   &corev1.Namespace{},
				err: errors.New(errNotKubeApp),
			},
		},
		"PatchedNoOverwrite": {
			reason: "If existing and desired have the same name and kind of a template, non-array fields in templates should not be overwritten in patch",
			args: args{
				c: kubeApp(kaWithTemplate("cool-temp", deployment(dmWithReplicas(&replicas)))),
				d: kubeApp(kaWithTemplate("cool-temp", deployment())),
			},
			want: want{
				o: kubeApp(kaWithTemplate("cool-temp", deployment(dmWithReplicas(&replicas)))),
			},
		},
		"PatchedRemoveResource": {
			reason: "If existing and desired have different template names, the existing template should be overwritten by the desired",
			args: args{
				c: kubeApp(kaWithTemplate("cool-temp", deployment()), kaWithTemplate("nice-temp", deployment())),
				d: kubeApp(kaWithTemplate("cool-temp", deployment())),
			},
			want: want{
				o: kubeApp(kaWithTemplate("cool-temp", deployment())),
			},
		},
		"PatchedAddResource": {
			reason: "If existing and desired have different template names, the existing template should be overwritten by the desired",
			args: args{
				c: kubeApp(kaWithTemplate("cool-temp", deployment())),
				d: kubeApp(kaWithTemplate("cool-temp", deployment()), kaWithTemplate("nice-temp", deployment())),
			},
			want: want{
				o: kubeApp(kaWithTemplate("cool-temp", deployment()), kaWithTemplate("nice-temp", deployment())),
			},
		},
		"PatchedOverwrite": {
			reason: "If existing and desired have different template names, the existing template should be overwritten by the desired",
			args: args{
				c: kubeApp(kaWithTemplate("nice-temp", deployment())),
				d: kubeApp(kaWithTemplate("cool-temp", deployment())),
			},
			want: want{
				o: kubeApp(kaWithTemplate("cool-temp", deployment())),
			},
		},
		"PatchedPartialOverwrite": {
			reason: "If existing and desired have the same name and kind of a template, array fields in templates should be overwritten in patch",
			args: args{
				c: kubeApp(kaWithTemplate("cool-temp", deployment(dmWithReplicas(&replicas), dmWithContainerPorts(replicas)))),
				d: kubeApp(kaWithTemplate("cool-temp", deployment(dmWithReplicas(&replicas)))),
			},
			want: want{
				o: kubeApp(kaWithTemplate("cool-temp", deployment(dmWithReplicas(&replicas)))),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := KubeAppApplyOption()(context.Background(), tc.args.c, tc.args.d)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nKubeAppApplyOption(...): -want error, +got error\n%s\n", tc.reason, diff)
			}

			o, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(tc.want.o)
			d, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(tc.args.d)
			if diff := cmp.Diff(o, d); diff != "" {
				t.Errorf("\n%s\nKubeAppApplyOption(...): -want, +got\n%s\n", tc.reason, diff)
			}
		})
	}
}
