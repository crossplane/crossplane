/*
Copyright 2024 The Crossplane Authors.

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

package deploymentruntime

import (
	"errors"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane/crossplane/apis/pkg/v1alpha1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
)

const (
	// default container name that XP uses
	runtimeContainerName = "package-runtime"

	errNilControllerConfig = "ControllerConfig is nil"
)

var timeNow = time.Now()

// controllerConfigToDeploymentRuntimeConfig converts a ControllerConfig to
// a DeploymentRuntimeConfig
func controllerConfigToDeploymentRuntimeConfig(cc *v1alpha1.ControllerConfig) (*v1beta1.DeploymentRuntimeConfig, error) {
	if cc == nil {
		return nil, errors.New(errNilControllerConfig)
	}
	dt := deploymentTemplateFromControllerConfig(cc)
	drc := newDeploymentRuntimeConfig(
		withName(cc.Name),
		// set the creation timestamp due to https://github.com/kubernetes/kubernetes/issues/109427
		// to be removed when fixed. k8s apply ignores this field
		withCreationTimestamp(metav1.NewTime(timeNow)),
		withServiceAccountTemplate(cc),
		withServiceTemplate(cc),
		withDeploymentTemplate(dt),
	)
	return drc, nil
}

func deploymentTemplateFromControllerConfig(cc *v1alpha1.ControllerConfig) *v1beta1.DeploymentTemplate { //nolint:gocyclo // Just a lot of if, then set field
	if cc == nil || !shouldCreateDeploymentTemplate(cc) {
		return nil
	}

	dt := &v1beta1.DeploymentTemplate{
		Spec: &appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{},
		},
	}

	// See code starting from https://github.com/crossplane/crossplane/blob/8c7fb7f2fa23ef5937a36260a80a87428f9d1f2b/internal/controller/pkg/revision/deployment.go#L213
	// for mapping fields from ControllerConfig to DeploymentRuntimeConfig
	if len(cc.Labels) > 0 || len(cc.Annotations) > 0 {
		dt.Metadata = &v1beta1.ObjectMeta{
			Labels:      cc.Labels,
			Annotations: cc.Annotations,
		}
	}

	// set the creation timestamp due to https://github.com/kubernetes/kubernetes/issues/109427
	// to be removed when fixed. k8s apply ignores this field
	if cc.CreationTimestamp.IsZero() || dt.Spec.Template.ObjectMeta.CreationTimestamp.IsZero() {
		dt.Spec.Template.ObjectMeta.CreationTimestamp = metav1.NewTime(timeNow)
	}

	if cc.Spec.Metadata != nil {
		dt.Spec.Template.Annotations = cc.Spec.Metadata.Annotations
	}
	if cc.Spec.Replicas != nil {
		dt.Spec.Replicas = cc.Spec.Replicas
	}
	if cc.Spec.NodeSelector != nil {
		dt.Spec.Template.Spec.NodeSelector = cc.Spec.NodeSelector
	}
	if cc.Spec.ServiceAccountName != nil {
		dt.Spec.Template.Spec.ServiceAccountName = *cc.Spec.ServiceAccountName
	}
	if cc.Spec.NodeName != nil {
		dt.Spec.Template.Spec.NodeName = *cc.Spec.NodeName
	}
	if len(cc.Spec.ImagePullSecrets) > 0 {
		dt.Spec.Template.Spec.ImagePullSecrets = cc.Spec.ImagePullSecrets
	}
	if cc.Spec.Affinity != nil {
		dt.Spec.Template.Spec.Affinity = cc.Spec.Affinity
	}
	if cc.Spec.PodSecurityContext != nil {
		dt.Spec.Template.Spec.SecurityContext = cc.Spec.PodSecurityContext
	}
	if len(cc.Spec.Tolerations) > 0 {
		dt.Spec.Template.Spec.Tolerations = cc.Spec.Tolerations
	}
	if cc.Spec.PriorityClassName != nil {
		dt.Spec.Template.Spec.PriorityClassName = *cc.Spec.PriorityClassName
	}
	if cc.Spec.RuntimeClassName != nil {
		dt.Spec.Template.Spec.RuntimeClassName = cc.Spec.RuntimeClassName
	}
	if len(cc.Spec.Volumes) > 0 {
		dt.Spec.Template.Spec.Volumes = append(dt.Spec.Template.Spec.Volumes, cc.Spec.Volumes...)
	}
	templateLabels := make(map[string]string)
	if cc.Spec.Metadata != nil {
		for k, v := range cc.Spec.Metadata.Labels {
			templateLabels[k] = v
		}
	}
	dt.Spec.Template.Labels = templateLabels

	if shouldCreateDeploymentTemplateContainer(cc) {
		c := containerFromControllerConfig(cc)
		dt.Spec.Template.Spec.Containers = append(dt.Spec.Template.Spec.Containers, *c)
	}

	return dt
}

func containerFromControllerConfig(cc *v1alpha1.ControllerConfig) *corev1.Container { //nolint:gocyclo // Just a lot of if, then set field
	if cc == nil || !shouldCreateDeploymentTemplateContainer(cc) {
		return nil
	}
	c := &corev1.Container{
		Name: runtimeContainerName,
	}

	if cc.Spec.Image != nil {
		c.Image = *cc.Spec.Image
	}
	if cc.Spec.ImagePullPolicy != nil {
		c.ImagePullPolicy = *cc.Spec.ImagePullPolicy
	}
	if len(cc.Spec.Ports) > 0 {
		c.Ports = cc.Spec.Ports
	}
	if cc.Spec.SecurityContext != nil {
		c.SecurityContext = cc.Spec.SecurityContext
	}
	if len(cc.Spec.Args) > 0 {
		c.Args = cc.Spec.Args
	}
	if len(cc.Spec.EnvFrom) > 0 {
		c.EnvFrom = cc.Spec.EnvFrom
	}
	if len(cc.Spec.Env) > 0 {
		c.Env = append(c.Env, cc.Spec.Env...)
	}
	if len(cc.Spec.VolumeMounts) > 0 {
		c.VolumeMounts =
			append(c.VolumeMounts, cc.Spec.VolumeMounts...)
	}
	if cc.Spec.ResourceRequirements != nil {
		c.Resources = *cc.Spec.ResourceRequirements.DeepCopy()
	}
	return c
}

func newDeploymentRuntimeConfig(options ...func(*v1beta1.DeploymentRuntimeConfig)) *v1beta1.DeploymentRuntimeConfig {
	drc := &v1beta1.DeploymentRuntimeConfig{}
	drc.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   v1beta1.Group,
		Kind:    v1beta1.DeploymentRuntimeConfigKind,
		Version: v1beta1.DeploymentRuntimeConfigGroupVersionKind.Version,
	})
	for _, o := range options {
		o(drc)
	}
	return drc
}

func withName(name string) func(*v1beta1.DeploymentRuntimeConfig) {
	return func(drc *v1beta1.DeploymentRuntimeConfig) {
		drc.ObjectMeta.Name = name
	}
}

func withCreationTimestamp(time metav1.Time) func(*v1beta1.DeploymentRuntimeConfig) {
	return func(drc *v1beta1.DeploymentRuntimeConfig) {
		drc.ObjectMeta.CreationTimestamp = time
	}
}

func withServiceAccountTemplate(cc *v1alpha1.ControllerConfig) func(*v1beta1.DeploymentRuntimeConfig) {
	return func(drc *v1beta1.DeploymentRuntimeConfig) {
		if cc != nil && (len(cc.Labels) > 0 || len(cc.Annotations) > 0 || cc.Spec.ServiceAccountName != nil) {
			drc.Spec.ServiceAccountTemplate = &v1beta1.ServiceAccountTemplate{
				Metadata: &v1beta1.ObjectMeta{
					Annotations: cc.Annotations,
					Labels:      cc.Labels,
					Name:        cc.Spec.ServiceAccountName,
				},
			}
		}
	}
}

func withServiceTemplate(cc *v1alpha1.ControllerConfig) func(*v1beta1.DeploymentRuntimeConfig) {
	return func(drc *v1beta1.DeploymentRuntimeConfig) {
		if cc != nil && (len(cc.Labels) > 0 || len(cc.Annotations) > 0) {
			drc.Spec.ServiceTemplate = &v1beta1.ServiceTemplate{
				Metadata: &v1beta1.ObjectMeta{
					Annotations: cc.Annotations,
					Labels:      cc.Labels,
				},
			}
		}
	}
}

func withDeploymentTemplate(dt *v1beta1.DeploymentTemplate) func(*v1beta1.DeploymentRuntimeConfig) {
	return func(drc *v1beta1.DeploymentRuntimeConfig) {
		if dt != nil {
			drc.Spec.DeploymentTemplate = dt
		}
	}
}

// shouldCreateDeploymentTemplate determines whether we should create a deployment
// template in the DeploymentRuntimeConfig
func shouldCreateDeploymentTemplate(cc *v1alpha1.ControllerConfig) bool { //nolint:gocyclo // There are a lot of triggers for this, but it's not complex
	return len(cc.Labels) > 0 ||
		len(cc.Annotations) > 0 ||
		cc.Spec.Metadata != nil ||
		cc.Spec.Replicas != nil ||
		cc.Spec.NodeSelector != nil ||
		cc.Spec.ServiceAccountName != nil ||
		cc.Spec.NodeName != nil ||
		cc.Spec.PodSecurityContext != nil ||
		len(cc.Spec.ImagePullSecrets) > 0 ||
		cc.Spec.Affinity != nil ||
		len(cc.Spec.Tolerations) > 0 ||
		cc.Spec.PriorityClassName != nil ||
		cc.Spec.RuntimeClassName != nil ||
		len(cc.Spec.Volumes) > 0 ||
		shouldCreateDeploymentTemplateContainer(cc)
}

// shouldCreateDeploymentTemplateContainer determines whether we should create a container
// entry in the DeploymentRuntimeConfig
func shouldCreateDeploymentTemplateContainer(cc *v1alpha1.ControllerConfig) bool {
	return cc.Spec.Image != nil ||
		cc.Spec.ImagePullPolicy != nil ||
		len(cc.Spec.Ports) > 0 ||
		cc.Spec.SecurityContext != nil ||
		cc.Spec.ResourceRequirements != nil ||
		len(cc.Spec.Args) > 0 ||
		len(cc.Spec.EnvFrom) > 0 ||
		len(cc.Spec.Env) > 0 ||
		len(cc.Spec.VolumeMounts) > 0
}
