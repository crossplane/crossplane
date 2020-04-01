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

package hosted

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"

	"github.com/crossplane/crossplane-runtime/pkg/test"
)

// url.Parse returns a slightly different error string in Go 1.14 than in prior versions.
func urlParseError(s string) error {
	_, err := url.Parse(s)
	return err
}

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

func TestGetHostPort(t *testing.T) {
	type args struct {
		urlHost string
	}

	type want struct {
		host string
		port string
		err  error
	}
	cases := map[string]struct {
		args
		want
	}{
		"Regular": {
			args: args{
				urlHost: "https://apiserver:6443",
			},
			want: want{
				host: "apiserver",
				port: "6443",
			},
		},
		"RegularWithIP": {
			args: args{
				urlHost: "https://111.222.111.222:6443",
			},
			want: want{
				host: "111.222.111.222",
				port: "6443",
			},
		},
		"NoPortHTTP": {
			args: args{
				urlHost: "http://apiserver",
			},
			want: want{
				host: "apiserver",
				port: "80",
			},
		},
		"NoPortHTTPS": {
			args: args{
				urlHost: "https://apiserver",
			},
			want: want{
				host: "apiserver",
				port: "443",
			},
		},
		"NoPortHTTPSWithIP": {
			args: args{
				urlHost: "https://111.222.111.222",
			},
			want: want{
				host: "111.222.111.222",
				port: "443",
			},
		},
		"InvalidURL": {
			args: args{
				urlHost: string(0x7f),
			},
			want: want{
				err: errors.Wrap(urlParseError(string(0x7f)), "cannot parse URL"),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			gotHost, gotPort, gotErr := getHostPort(tc.args.urlHost)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("getHostPort(...): -want error, +got error:\n %s", diff)
			}
			if diff := cmp.Diff(tc.want.host, gotHost); diff != "" {
				t.Errorf("getHostPort(...): -want host, +got result:\n %s", diff)
			}
			if diff := cmp.Diff(tc.want.port, gotPort); diff != "" {
				t.Errorf("getHostPort(...): -want port, +got result:\n %s", diff)
			}
		})
	}
}

func TestNewConfigForHost(t *testing.T) {
	type args struct {
		server                  string
		hostControllerNamespace string
	}

	type want struct {
		config *Config
		err    error
	}
	cases := map[string]struct {
		args
		want
	}{
		"RegularNonHosted": {
			args: args{
				server:                  "https://apiserver",
				hostControllerNamespace: "",
			},
			want: want{
				config: nil,
			},
		},
		"ErrorHostedMissingHost": {
			args: args{
				server:                  "",
				hostControllerNamespace: "test-controllers-ns",
			},
			want: want{
				config: nil,
			},
		},
		"ErrorHostedInvalidHost": {
			args: args{
				server:                  string(0x7f),
				hostControllerNamespace: "test-controllers-ns",
			},
			want: want{
				config: nil,
				err:    errors.Wrap(errors.Wrap(urlParseError(string(0x7f)), "cannot parse URL"), "cannot get host port from tenant kubeconfig"),
			},
		},
		"RegularHosted": {
			args: args{
				server:                  "https://apiserver:6443",
				hostControllerNamespace: "test-controllers-ns",
			},
			want: want{
				config: &Config{
					HostControllerNamespace: "test-controllers-ns",
					TenantAPIServiceHost:    "apiserver",
					TenantAPIServicePort:    "6443",
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, gotErr := NewConfigForHost(tc.args.hostControllerNamespace, tc.args.server)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("NewConfigForHost(...): -want error, +got error:\n %s", diff)
			}

			if diff := cmp.Diff(tc.want.config, got); diff != "" {
				t.Errorf("NewConfigForHost(...): -want result, +got result:\n %s", diff)
			}
		})
	}

}
