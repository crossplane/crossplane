package stacks

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplaneio/crossplane-runtime/pkg/test"
	"github.com/crossplaneio/crossplane/pkg/controller/stacks/hosted"
)

func Test_getHostPort(t *testing.T) {
	type args struct {
		urlHost string
	}

	type want struct {
		host      string
		port      string
		shouldErr bool
	}
	cases := map[string]struct {
		args
		want
	}{
		"regular": {
			args: args{
				urlHost: "https://apiserver:6443",
			},
			want: want{
				host:      "apiserver",
				port:      "6443",
				shouldErr: false,
			},
		},
		"regularWithIP": {
			args: args{
				urlHost: "https://111.222.111.222:6443",
			},
			want: want{
				host:      "111.222.111.222",
				port:      "6443",
				shouldErr: false,
			},
		},
		"noPortHTTP": {
			args: args{
				urlHost: "http://apiserver",
			},
			want: want{
				host:      "apiserver",
				port:      "80",
				shouldErr: false,
			},
		},
		"noPortHTTPS": {
			args: args{
				urlHost: "https://apiserver",
			},
			want: want{
				host:      "apiserver",
				port:      "443",
				shouldErr: false,
			},
		},
		"noPortHTTPSWithIP": {
			args: args{
				urlHost: "https://111.222.111.222",
			},
			want: want{
				host:      "111.222.111.222",
				port:      "443",
				shouldErr: false,
			},
		},
		"invalidURL": {
			args: args{
				urlHost: string(0x7f),
			},
			want: want{
				shouldErr: true,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			gotHost, gotPort, gotErr := getHostPort(tc.args.urlHost)
			if diff := cmp.Diff(tc.want.shouldErr, gotErr != nil, test.EquateErrors()); diff != "" {
				t.Fatalf("getHostPort(...): -want error, +got error: %s", diff)
			}
			if diff := cmp.Diff(tc.want.host, gotHost); diff != "" {
				t.Errorf("getHostPort(...): -want host, +got result: %s", diff)
			}
			if diff := cmp.Diff(tc.want.port, gotPort); diff != "" {
				t.Errorf("getHostPort(...): -want port, +got result: %s", diff)
			}
		})
	}
}

func Test_getSMOptions(t *testing.T) {
	type args struct {
		server                  string
		hostControllerNamespace string
	}

	type want struct {
		out       *MockSMReconciler
		shouldErr bool
	}
	cases := map[string]struct {
		args
		want
	}{
		"regularNonHosted": {
			args: args{
				server:                  "https://apiserver",
				hostControllerNamespace: "",
			},
			want: want{
				out:       &MockSMReconciler{},
				shouldErr: false,
			},
		},
		"errorHostedMissingHost": {
			args: args{
				server:                  "",
				hostControllerNamespace: "test-controllers-ns",
			},
			want: want{
				out:       &MockSMReconciler{},
				shouldErr: true,
			},
		},
		"errorHostedInvalidHost": {
			args: args{
				server:                  string(0x7f),
				hostControllerNamespace: "test-controllers-ns",
			},
			want: want{
				out:       &MockSMReconciler{},
				shouldErr: true,
			},
		},
		"regularHosted": {
			args: args{
				server:                  "https://apiserver:6443",
				hostControllerNamespace: "test-controllers-ns",
			},
			want: want{
				out: &MockSMReconciler{
					hostedConfig: &hosted.Config{
						HostControllerNamespace: "test-controllers-ns",
						TenantAPIServiceHost:    "apiserver",
						TenantAPIServicePort:    "6443",
					},
				},
				shouldErr: false,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			mGot := &MockSMReconciler{}

			got, gotErr := getSMOptions(tc.args.server, tc.args.hostControllerNamespace)
			if diff := cmp.Diff(tc.want.shouldErr, gotErr != nil, test.EquateErrors()); diff != "" {
				t.Fatalf("getSMOptions(...): -want error, +got error: %s", diff)
			}

			for _, o := range got {
				o(mGot)
			}

			if diff := cmp.Diff(tc.want.out.hostedConfig, mGot.hostedConfig); diff != "" {
				t.Errorf("getSMOptions(...): -want result, +got result: %s", diff)
			}
		})
	}

}

type MockSMReconciler struct {
	hostedConfig *hosted.Config
}

func (m *MockSMReconciler) SetHostedConfig(h *hosted.Config) {
	m.hostedConfig = h
}
