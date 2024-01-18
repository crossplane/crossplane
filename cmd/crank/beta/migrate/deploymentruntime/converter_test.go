package deploymentruntime

import (
	"errors"
	"testing"
	"time"

	"github.com/crossplane/crossplane/apis/pkg/v1alpha1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewDeploymentTemplateFromControllerConfig(t *testing.T) {
	replicas := int32(99)
	user := int64(33)
	saName := "sa-name"
	className := "className"
	image := "xpkg.upbound.io/crossplane/crossplane:latest"
	timeNow := metav1.NewTime(time.Now())

	type args struct {
		cc *v1alpha1.ControllerConfig
	}
	type want struct {
		dt *v1beta1.DeploymentTemplate
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NilControllerConfig": {
			reason: "Return a nil DeploymentTemplate",
			args: args{
				cc: nil,
			},
			want: want{
				dt: nil,
			},
		},
		"WithMultipleFields": {
			reason: "Fields Are correctly mapped",
			args: args{
				cc: &v1alpha1.ControllerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
						Labels: map[string]string{
							"key1": "value1",
							"key2": "value2",
						},
					},
					Spec: v1alpha1.ControllerConfigSpec{
						Args: []string{"- -d", "- --enable-management-policies"},
						Affinity: &corev1.Affinity{
							NodeAffinity: &corev1.NodeAffinity{
								RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
									NodeSelectorTerms: []corev1.NodeSelectorTerm{{
										MatchFields: []corev1.NodeSelectorRequirement{
											{Key: "xplane"},
										},
									},
									},
								},
							},
						},
						Image:              &image,
						ImagePullSecrets:   []corev1.LocalObjectReference{{Name: "my-secret"}},
						Replicas:           &replicas,
						NodeSelector:       map[string]string{"node-selector": "foo"},
						ServiceAccountName: &saName,
						NodeName:           &saName,
						PodSecurityContext: &corev1.PodSecurityContext{
							RunAsUser:  &user,
							RunAsGroup: &user,
						},
						PriorityClassName: &className,
						ResourceRequirements: &corev1.ResourceRequirements{
							Limits: map[corev1.ResourceName]resource.Quantity{
								"cpu":    *resource.NewMilliQuantity(5000, resource.DecimalSI),
								"memory": *resource.NewQuantity(10*1024*1024*1024, resource.BinarySI),
							},
							Requests: map[corev1.ResourceName]resource.Quantity{
								"cpu":    *resource.NewMilliQuantity(1500, resource.DecimalSI),
								"memory": *resource.NewQuantity(5*1024*1024*1024, resource.BinarySI),
							},
						},
						RuntimeClassName: &className,
						Tolerations:      []corev1.Toleration{{Key: "toleration-1"}},
						Volumes:          []corev1.Volume{{Name: "volume1"}, {Name: "volume2"}},
						VolumeMounts:     []corev1.VolumeMount{{Name: "mount1", MountPath: "/tmp"}, {Name: "mount2", MountPath: "/etc/ssl/certs"}},
					},
				},
			},
			want: want{
				dt: &v1beta1.DeploymentTemplate{
					Metadata: &v1beta1.ObjectMeta{
						Labels: map[string]string{"key1": "value1", "key2": "value2"},
					},
					Spec: &v1.DeploymentSpec{
						Replicas: &replicas,
						Selector: &metav1.LabelSelector{},
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								CreationTimestamp: timeNow,
								Labels:            map[string]string{},
							},
							Spec: corev1.PodSpec{
								Affinity: &corev1.Affinity{
									NodeAffinity: &corev1.NodeAffinity{
										RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
											NodeSelectorTerms: []corev1.NodeSelectorTerm{{
												MatchFields: []corev1.NodeSelectorRequirement{
													{Key: "xplane"},
												},
											},
											},
										},
									},
								},
								Containers: []corev1.Container{{
									Name:  "package-runtime",
									Args:  []string{"- -d", "- --enable-management-policies"},
									Image: image,
									Resources: corev1.ResourceRequirements{
										Limits: map[corev1.ResourceName]resource.Quantity{
											"cpu":    *resource.NewMilliQuantity(5000, resource.DecimalSI),
											"memory": *resource.NewQuantity(10*1024*1024*1024, resource.BinarySI),
										},
										Requests: map[corev1.ResourceName]resource.Quantity{
											"cpu":    *resource.NewMilliQuantity(1500, resource.DecimalSI),
											"memory": *resource.NewQuantity(5*1024*1024*1024, resource.BinarySI),
										},
									},
									VolumeMounts: []corev1.VolumeMount{{Name: "mount1", MountPath: "/tmp"}, {Name: "mount2", MountPath: "/etc/ssl/certs"}},
								},
								},

								ImagePullSecrets:   []corev1.LocalObjectReference{{Name: "my-secret"}},
								NodeSelector:       map[string]string{"node-selector": "foo"},
								ServiceAccountName: "sa-name",
								NodeName:           "sa-name",
								PriorityClassName:  className,
								RuntimeClassName:   &className,
								SecurityContext:    &corev1.PodSecurityContext{RunAsUser: &user, RunAsGroup: &user},
								Tolerations:        []corev1.Toleration{{Key: "toleration-1"}},
								Volumes:            []corev1.Volume{{Name: "volume1"}, {Name: "volume2"}},
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			dt := NewDeploymentTemplateFromControllerConfig(tc.args.cc)

			if diff := cmp.Diff(tc.want.dt, dt, cmpopts.EquateApproxTime(time.Second*2)); diff != "" {
				t.Errorf("%s\nControllerConfigToRuntimeDeploymentConfig(...): -want i, +got i:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestControllerConfigToRuntimeDeploymentConfig(t *testing.T) {
	timeNow := metav1.NewTime(time.Now())
	type args struct {
		cc *v1alpha1.ControllerConfig
	}
	type want struct {
		dr  *v1beta1.DeploymentRuntimeConfig
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NilControllerConfig": {
			reason: "Correctly return an error",
			args: args{
				cc: nil,
			},
			want: want{
				dr:  nil,
				err: errors.New(ErrNilControllerConfig),
			},
		},
		"WithName": {
			reason: "Name is correctly set",
			args: args{
				cc: &v1alpha1.ControllerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "test",
						CreationTimestamp: timeNow,
					},
				},
			},
			want: want{
				dr: &v1beta1.DeploymentRuntimeConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "test",
						CreationTimestamp: timeNow,
					},
					TypeMeta: metav1.TypeMeta{
						Kind:       v1beta1.DeploymentRuntimeConfigKind,
						APIVersion: v1beta1.Group + "/" + v1beta1.Version,
					},
				},
				err: nil,
			},
		},
		"WithLabelsAndAnnotations": {
			reason: "Correctly Set Labels and Annotations",
			args: args{
				cc: &v1alpha1.ControllerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "test",
						Annotations: map[string]string{"crossplane": "rocks"},
						Labels:      map[string]string{"a": "b"},
					},
				},
			},
			want: want{
				dr: &v1beta1.DeploymentRuntimeConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "test",
						CreationTimestamp: timeNow,
					},
					TypeMeta: metav1.TypeMeta{
						Kind:       v1beta1.DeploymentRuntimeConfigKind,
						APIVersion: v1beta1.Group + "/" + v1beta1.Version,
					},
					Spec: v1beta1.DeploymentRuntimeConfigSpec{
						ServiceTemplate: &v1beta1.ServiceTemplate{
							Metadata: &v1beta1.ObjectMeta{
								Annotations: map[string]string{"crossplane": "rocks"},
								Labels:      map[string]string{"a": "b"},
							},
						},

						ServiceAccountTemplate: &v1beta1.ServiceAccountTemplate{
							Metadata: &v1beta1.ObjectMeta{
								Annotations: map[string]string{"crossplane": "rocks"},
								Labels:      map[string]string{"a": "b"},
							},
						},
						DeploymentTemplate: &v1beta1.DeploymentTemplate{
							Metadata: &v1beta1.ObjectMeta{
								Annotations: map[string]string{"crossplane": "rocks"},
								Labels:      map[string]string{"a": "b"},
							},
							Spec: &v1.DeploymentSpec{
								Selector: &metav1.LabelSelector{},
								Template: corev1.PodTemplateSpec{
									ObjectMeta: metav1.ObjectMeta{
										Labels:            map[string]string{},
										CreationTimestamp: timeNow,
									},
								}},
						},
					},
				},
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			dr, err := ControllerConfigToDeploymentRuntimeConfig(tc.args.cc)
			if diff := cmp.Diff(tc.want.dr, dr, cmpopts.EquateApproxTime(time.Second*2)); diff != "" {
				t.Errorf("%s\nControllerConfigToRuntimeDeploymentConfig(...): -want i, +got i:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.err, err, EquateErrors()); diff != "" {
				t.Errorf("%s\nControllerConfigToRuntimeDeploymentConfig(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestNewContainerFromControllerConfig(t *testing.T) {
	image := "xpkg.upbound.io/crossplane/crossplane:latest"
	priv := false
	pullAlways := corev1.PullAlways

	type args struct {
		cc *v1alpha1.ControllerConfig
	}
	type want struct {
		c *corev1.Container
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{

		"NilControllerConfig": {
			reason: "Correctly return an empty container",
			args: args{
				cc: nil,
			},
			want: want{
				c: nil,
			},
		},
		"SetAllAvailableFields": {
			reason: "Correctly set all fields",
			args: args{
				cc: &v1alpha1.ControllerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
						Labels: map[string]string{
							"key1": "value1",
							"key2": "value2",
						},
					},
					Spec: v1alpha1.ControllerConfigSpec{
						Args:            []string{"- -d", "- --enable-management-policies"},
						Env:             []corev1.EnvVar{{Name: "ENV", Value: "unit-test"}, {Name: "X", Value: "y"}},
						EnvFrom:         []corev1.EnvFromSource{{Prefix: "XP_"}},
						Image:           &image,
						ImagePullPolicy: &pullAlways,
						Ports:           []corev1.ContainerPort{{Name: "metrics", HostPort: 8080, ContainerPort: 8888}},
						SecurityContext: &corev1.SecurityContext{Privileged: &priv},
						VolumeMounts:    []corev1.VolumeMount{{Name: "mount1", MountPath: "/tmp"}, {Name: "mount2", MountPath: "/etc/ssl/certs"}},
					},
				},
			},
			want: want{
				c: &corev1.Container{
					Name:            "package-runtime",
					Args:            []string{"- -d", "- --enable-management-policies"},
					Env:             []corev1.EnvVar{{Name: "ENV", Value: "unit-test"}, {Name: "X", Value: "y"}},
					EnvFrom:         []corev1.EnvFromSource{{Prefix: "XP_"}},
					Image:           image,
					ImagePullPolicy: pullAlways,
					Ports:           []corev1.ContainerPort{{Name: "metrics", HostPort: 8080, ContainerPort: 8888}},
					SecurityContext: &corev1.SecurityContext{Privileged: &priv},
					VolumeMounts:    []corev1.VolumeMount{{Name: "mount1", MountPath: "/tmp"}, {Name: "mount2", MountPath: "/etc/ssl/certs"}},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := NewContainerFromControllerConfig(tc.args.cc)
			if diff := cmp.Diff(tc.want.c, c, cmpopts.EquateApproxTime(time.Second*2)); diff != "" {
				t.Errorf("%s\nNewContainerFromControllerConfig(...): -want i, +got i:\n%s", tc.reason, diff)
			}

		})
	}
}

func EquateErrors() cmp.Option {
	return cmp.Comparer(func(a, b error) bool {
		if a == nil || b == nil {
			return a == nil && b == nil
		}
		return a.Error() == b.Error()
	})
}
