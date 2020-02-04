package hosted

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"

	"github.com/crossplaneio/crossplane-runtime/pkg/test"
)

func TestNewConfig(t *testing.T) {
	type args struct {
		hostControllerNamespace string
		tenantAPIServiceHost    string
		tenantAPIServicePort    string
	}
	type want struct {
		out *Config
		err error
	}
	cases := map[string]struct {
		args
		want
	}{
		"MissingControllerNamespace": {
			args: args{
				tenantAPIServiceHost: "test-apiserver",
				tenantAPIServicePort: "6443",
			},
			want: want{
				out: nil,
				err: errors.New(fmt.Sprintf(errMissingOption, "hostControllerNamespace")),
			},
		},
		"MissingHost": {
			args: args{
				hostControllerNamespace: "test-ns",
				tenantAPIServicePort:    "6443",
			},
			want: want{
				out: nil,
				err: errors.New(fmt.Sprintf(errMissingOption, "tenantAPIServiceHost")),
			},
		},
		"MissingPort": {
			args: args{
				hostControllerNamespace: "test-ns",
				tenantAPIServiceHost:    "test-apiserver",
			},
			want: want{
				out: nil,
				err: errors.New(fmt.Sprintf(errMissingOption, "tenantAPIServicePort")),
			},
		},
		"Success": {
			args: args{
				hostControllerNamespace: "test-ns",
				tenantAPIServiceHost:    "test-apiserver",
				tenantAPIServicePort:    "6443",
			},
			want: want{
				out: &Config{
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
			got, gotErr := NewConfig(tc.hostControllerNamespace, tc.tenantAPIServiceHost, tc.tenantAPIServicePort)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("NewConfig(...): -want error, +got error: %s", diff)
			}
			if diff := cmp.Diff(tc.want.out, got); diff != "" {
				t.Errorf("NewConfig(...): -want result, +got result: %s", diff)
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
			c := &Config{
				HostControllerNamespace: tc.hostControllerNamespace,
			}
			got := c.ObjectReferenceOnHost(tc.name, tc.namespace)
			if diff := cmp.Diff(tc.want.out, got); diff != "" {
				t.Errorf("ObjectReferenceOnHost(...): -want result, +got result: %s", diff)
			}
		})
	}
}
