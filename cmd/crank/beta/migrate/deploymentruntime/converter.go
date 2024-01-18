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
	ErrNilControllerConfig = "ControllerConfig is nil"
)

var timeNow = time.Now()

func ControllerConfigToDeploymentRuntimeConfig(cc *v1alpha1.ControllerConfig) (*v1beta1.DeploymentRuntimeConfig, error) {
	if cc == nil {
		return nil, errors.New(ErrNilControllerConfig)
	}
	dt := NewDeploymentTemplateFromControllerConfig(cc)
	drc := NewDeploymentRuntimeConfig(
		WithName(cc.Name),
		WithCreationTimestamp(metav1.NewTime(timeNow)),
		WithServiceAccountTemplate(cc),
		WithServiceTemplate(cc),
		WithDeploymentTemplate(dt),
	)
	return drc, nil
}

func NewDeploymentTemplateFromControllerConfig(cc *v1alpha1.ControllerConfig) *v1beta1.DeploymentTemplate {
	if cc == nil || !CreateDeploymentTemplate(cc) {
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

	if CreateDeploymentTemplateContainer(cc) {
		c := NewContainerFromControllerConfig(cc)
		dt.Spec.Template.Spec.Containers = append(dt.Spec.Template.Spec.Containers, *c)
	}

	return dt
}

func NewContainerFromControllerConfig(cc *v1alpha1.ControllerConfig) *corev1.Container {
	if cc == nil || !CreateDeploymentTemplateContainer(cc) {
		return nil
	}
	c := &corev1.Container{
		Name: "package-runtime", // Default container name that XP uses
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

func NewDeploymentRuntimeConfig(options ...func(*v1beta1.DeploymentRuntimeConfig)) *v1beta1.DeploymentRuntimeConfig {
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

func WithName(name string) func(*v1beta1.DeploymentRuntimeConfig) {
	return func(drc *v1beta1.DeploymentRuntimeConfig) {
		drc.ObjectMeta.Name = name
	}
}

func WithCreationTimestamp(time metav1.Time) func(*v1beta1.DeploymentRuntimeConfig) {
	return func(drc *v1beta1.DeploymentRuntimeConfig) {
		drc.ObjectMeta.CreationTimestamp = time
	}
}
func WithServiceAccountTemplate(cc *v1alpha1.ControllerConfig) func(*v1beta1.DeploymentRuntimeConfig) {
	return func(drc *v1beta1.DeploymentRuntimeConfig) {
		if cc != nil && (len(cc.Labels) > 0 || len(cc.Annotations) > 0) {
			drc.Spec.ServiceAccountTemplate = &v1beta1.ServiceAccountTemplate{
				Metadata: &v1beta1.ObjectMeta{
					Annotations: cc.Annotations,
					Labels:      cc.Labels,
				},
			}
		}
	}
}

func WithServiceTemplate(cc *v1alpha1.ControllerConfig) func(*v1beta1.DeploymentRuntimeConfig) {
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

func WithDeploymentTemplate(dt *v1beta1.DeploymentTemplate) func(*v1beta1.DeploymentRuntimeConfig) {
	return func(drc *v1beta1.DeploymentRuntimeConfig) {
		if dt != nil {
			drc.Spec.DeploymentTemplate = dt
		}
	}
}

func NewDeploymentTemplate(options ...func(*v1beta1.DeploymentTemplate)) *v1beta1.DeploymentTemplate {
	d := &v1beta1.DeploymentTemplate{}
	for _, o := range options {
		o(d)
	}
	return d
}

// CreateDeploymentTemplate determines whether we should create a deployment
// template in the DeploymentRuntimeConfig
func CreateDeploymentTemplate(cc *v1alpha1.ControllerConfig) bool {
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
		CreateDeploymentTemplateContainer(cc)
}

// CreateDeploymentTemplateContainer determines whether we should create a container
// entry in the DeploymentRuntimeConfig
func CreateDeploymentTemplateContainer(cc *v1alpha1.ControllerConfig) bool {
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
