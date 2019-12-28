package host

import (
	"fmt"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"

	"github.com/crossplaneio/crossplane-runtime/pkg/test"
)

func TestNewConfig(t *testing.T) {
	type args struct {
		env map[string]string
	}
	type want struct {
		out *HostedConfig
		err error
	}
	cases := map[string]struct {
		args
		want
	}{
		"HostedModeDisabled": {
			args: args{
				env: nil,
			},
			want: want{
				out: nil,
				err: nil,
			},
		},
		"MissingControllerNamespace": {
			args: args{
				env: map[string]string{
					EnvTenantKubeconfig:            "path/to/test",
					envTenantKubernetesServiceHost: "test-apiserver",
					envTenantKubernetesServicePort: "6443",
				},
			},
			want: want{
				out: nil,
				err: errors.New(fmt.Sprintf(errMissingEnvVar, envControllerNamespace)),
			},
		},
		"MissingHost": {
			args: args{
				env: map[string]string{
					EnvTenantKubeconfig:            "path/to/test",
					envControllerNamespace:         "test-ns",
					envTenantKubernetesServicePort: "6443",
				},
			},
			want: want{
				out: nil,
				err: errors.New(fmt.Sprintf(errMissingEnvVar, envTenantKubernetesServiceHost)),
			},
		},
		"MissingPort": {
			args: args{
				env: map[string]string{
					EnvTenantKubeconfig:            "path/to/test",
					envControllerNamespace:         "test-ns",
					envTenantKubernetesServiceHost: "test-apiserver",
				},
			},
			want: want{
				out: nil,
				err: errors.New(fmt.Sprintf(errMissingEnvVar, envTenantKubernetesServicePort)),
			},
		},
		"Success": {
			args: args{
				env: map[string]string{
					EnvTenantKubeconfig:            "path/to/test",
					envControllerNamespace:         "test-ns",
					envTenantKubernetesServiceHost: "test-apiserver",
					envTenantKubernetesServicePort: "6443",
				},
			},
			want: want{
				out: &HostedConfig{
					HostControllerNamespace: "test-ns",
					TenantAPIServiceHost:    "test-apiserver",
					TenantAPIServicePort:    "6443",
				},
				err: nil,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			os.Clearenv()
			for k, v := range tc.env {
				os.Setenv(k, v)
			}
			got, gotErr := NewHostedConfig()
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("NewHostedConfig(...): -want error, +got error: %s", diff)
			}
			if diff := cmp.Diff(tc.want.out, got); diff != "" {
				t.Errorf("NewHostedConfig(...): -want result, +got result: %s", diff)
			}
		})
	}
}

func TestConfig_ObjectReferenceOnHost(t *testing.T) {
	type args struct {
		hostControllerNamespace string

		name      string
		namespace string
	}
	type want struct {
		out corev1.ObjectReference
	}
	cases := map[string]struct {
		args
		want
	}{
		"empty": {
			args: args{
				hostControllerNamespace: "controller-ns",
				name:                    "test-deployment",
				namespace:               "test-ns",
			},
			want: want{
				out: corev1.ObjectReference{
					Name:      "test-ns.test-deployment",
					Namespace: "controller-ns",
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &HostedConfig{
				HostControllerNamespace: tc.hostControllerNamespace,
			}
			got := c.ObjectReferenceOnHost(tc.name, tc.namespace)
			if diff := cmp.Diff(tc.want.out, got); diff != "" {
				t.Errorf("ObjectReferenceOnHost(...): -want result, +got result: %s", diff)
			}
		})
	}
}
