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

package containerized

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/crossplane/crossplane-runtime/pkg/reconciler/oam/workload"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	oamv1alpha2 "github.com/crossplane/crossplane/apis/oam/v1alpha2"
)

var (
	cwName      = "test-name"
	cwNamespace = "test-namespace"
	cwUID       = "a-very-unique-identifier"
)

type deploymentModifier func(*appsv1.Deployment)

func dmWithOS(os string) deploymentModifier {
	return func(d *appsv1.Deployment) {
		if d.Spec.Template.Spec.NodeSelector == nil {
			d.Spec.Template.Spec.NodeSelector = map[string]string{}
		}
		d.Spec.Template.Spec.NodeSelector["beta.kubernetes.io/os"] = os
	}
}

func dmWithContainer(c corev1.Container) deploymentModifier {
	return func(d *appsv1.Deployment) {
		d.Spec.Template.Spec.Containers = append(d.Spec.Template.Spec.Containers, c)
	}
}

func deployment(mod ...deploymentModifier) *appsv1.Deployment {
	d := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       deploymentKind,
			APIVersion: deploymentAPIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cwName,
			Namespace: defaultNamespace,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					labelKey: cwUID,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						labelKey: cwUID,
					},
				},
			},
		},
	}

	for _, m := range mod {
		m(d)
	}

	return d
}

type cwModifier func(*oamv1alpha2.ContainerizedWorkload)

func cwWithOS(os string) cwModifier {
	return func(cw *oamv1alpha2.ContainerizedWorkload) {
		oamOS := oamv1alpha2.OperatingSystem(os)
		cw.Spec.OperatingSystem = &oamOS
	}
}

func cwWithContainer(c oamv1alpha2.Container) cwModifier {
	return func(cw *oamv1alpha2.ContainerizedWorkload) {
		cw.Spec.Containers = append(cw.Spec.Containers, c)
	}
}

func containerizedWorkload(mod ...cwModifier) *oamv1alpha2.ContainerizedWorkload {
	cw := &oamv1alpha2.ContainerizedWorkload{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cwName,
			Namespace: cwNamespace,
			UID:       types.UID(cwUID),
		},
	}

	for _, m := range mod {
		m(cw)
	}

	return cw
}

var _ workload.Translator = workload.TranslateFn(Translator)

func TestTranslator(t *testing.T) {

	envVarSecretVal := "nicesecretvalue"

	type args struct {
		w resource.Workload
	}

	type want struct {
		result []resource.Object
		err    error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ErrorWorkloadNotContainerizedWorkload": {
			reason: "Workload passed to translator that is not ContainerizedWorkload should return error.",
			args: args{
				w: &fake.Workload{},
			},
			want: want{err: errors.New(errNotContainerizedWorkload)},
		},
		"SuccessfulEmpty": {
			reason: "A ContainerizedWorkload should be successfully translated into a deployment.",
			args: args{
				w: containerizedWorkload(),
			},
			want: want{result: []resource.Object{deployment()}},
		},
		"SuccessfulOS": {
			reason: "A ContainerizedWorkload should be successfully translateddinto a deployment.",
			args: args{
				w: containerizedWorkload(cwWithOS("test")),
			},
			want: want{result: []resource.Object{deployment(dmWithOS("test"))}},
		},
		"SuccessfulContainers": {
			reason: "A ContainerizedWorkload should be successfully translated into a deployment.",
			args: args{
				w: containerizedWorkload(cwWithContainer(oamv1alpha2.Container{
					Name:      "cool-container",
					Image:     "cool/image:latest",
					Command:   []string{"run"},
					Arguments: []string{"--coolflag"},
					Ports: []oamv1alpha2.ContainerPort{
						{
							Name: "cool-port",
							Port: 8080,
						},
					},
					Resources: &oamv1alpha2.ContainerResources{
						Volumes: []oamv1alpha2.VolumeResource{
							{
								Name:      "cool-volume",
								MouthPath: "/my/cool/path",
							},
						},
					},
					Environment: []oamv1alpha2.ContainerEnvVar{
						{
							Name: "COOL_SECRET",
							FromSecret: &oamv1alpha2.SecretKeySelector{
								Name: "cool-secret",
								Key:  "secretdata",
							},
						},
						{
							Name:  "NICE_SECRET",
							Value: &envVarSecretVal,
						},
						// If both Value and FromSecret are defined, we use Value
						{
							Name:  "USE_VAL_SECRET",
							Value: &envVarSecretVal,
							FromSecret: &oamv1alpha2.SecretKeySelector{
								Name: "cool-secret",
								Key:  "secretdata",
							},
						},
						// If neither Value or FromSecret is define, we skip
						{
							Name: "USE_VAL_SECRET",
						},
					},
				})),
			},
			want: want{result: []resource.Object{deployment(dmWithContainer(corev1.Container{
				Name:    "cool-container",
				Image:   "cool/image:latest",
				Command: []string{"run"},
				Args:    []string{"--coolflag"},
				Ports: []corev1.ContainerPort{
					{
						Name:          "cool-port",
						ContainerPort: 8080,
					},
				},
				// CPU and Memory get initialized because we set them if any
				// part of OAM Container.Resources is present. They are not
				// pointer values, so we cannot tell if they were omitted or
				// explicitly set to zero-value.
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						"cpu":    {},
						"memory": {},
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "cool-volume",
						MountPath: "/my/cool/path",
					},
				},
				Env: []corev1.EnvVar{
					{
						Name: "COOL_SECRET",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								Key: "secretdata",
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "cool-secret",
								},
							},
						},
					},
					{
						Name:  "NICE_SECRET",
						Value: envVarSecretVal,
					},
					{
						Name:  "USE_VAL_SECRET",
						Value: envVarSecretVal,
					},
				},
			}))}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r, err := Translator(context.Background(), tc.args.w)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\nReason: %s\ncontainerizedWorkloadTranslator(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.result, r); diff != "" {
				t.Errorf("\nReason: %s\ncontainerizedWorkloadTranslator(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
